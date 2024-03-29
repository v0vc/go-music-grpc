package list

import (
	"image"
	"runtime"
	"strconv"
	"testing"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type actuallyStatefulElement struct {
	serial string
}

func (a actuallyStatefulElement) Serial() Serial {
	return Serial(a.serial)
}

func mkStateUpdate(elements []Element, synth Synthesizer) stateUpdate {
	var su stateUpdate
	su.Synthesis = Synthesize(elements, synth)
	return su
}

func TestManager(t *testing.T) {
	// create a fake rendering context
	var ops op.Ops
	gtx := app.NewContext(&ops, app.FrameEvent{
		Now: time.Now(),
		Metric: unit.Metric{
			PxPerDp: 1,
			PxPerSp: 1,
		},
		Size: image.Pt(1000, 1000),
	})

	var list layout.List
	allocationCounter := 0
	presenterCounter := 0

	synth := func(a, b, c Element) []Element { return []Element{b} }

	m := NewManager(6, Hooks{
		Allocator: func(e Element) interface{} {
			allocationCounter++
			switch e.(type) {
			case actuallyStatefulElement:
				// Just allocate something, doesn't matter what.
				return actuallyStatefulElement{}
			}
			return nil
		},
		Presenter: func(e Element, state interface{}) layout.Widget {
			presenterCounter++
			switch e.(type) {
			case actuallyStatefulElement:
				// Trigger a panic if the wrong state type was provided.
				_ = state.(actuallyStatefulElement)
			}
			return layout.Spacer{
				Width:  unit.Dp(5),
				Height: unit.Dp(5),
			}.Layout
		},
		Loader: func(dir Direction, relativeTo Serial) ([]Element, bool) {
			return nil, false
		},
		Invalidator: func() {},
		Comparator:  func(a, b Element) bool { return true },
		Synthesizer: synth,
	})
	// Shut down the existing background processing for this manager.
	close(m.requests)

	// Replace the background processing channels with channels we can control
	// from within the test.
	requests := make(chan interface{}, 1)
	updates := make(chan []stateUpdate, 1)
	m.requests = requests
	m.stateUpdates = updates

	var persistentElements []Element

	type testcase struct {
		name                  string
		expectingRequest      bool
		expectedRequest       loadRequest
		sendUpdate            bool
		update                stateUpdate
		expectedAllocations   int
		expectedPresentations int
		stateSize             int
	}

	for _, tc := range []testcase{
		{
			name:       "load initial elements",
			sendUpdate: true,
			update: func() stateUpdate {
				// Send an update to provide a few elements to work with.
				persistentElements = testElements[0:3]
				return mkStateUpdate(persistentElements, synth)
			}(),
			expectedAllocations:   3,
			expectedPresentations: 3,
			expectingRequest:      true,
			expectedRequest: loadRequest{
				Direction: Before,
			},
			stateSize: 3,
		},
		{
			name:       "load stateless elements (shouldn't allocate, should present)",
			sendUpdate: true,
			update: func() stateUpdate {
				// Send an update to provide a few elements to work with.
				persistentElements = append(persistentElements,
					testElement{
						synthCount: 1,
						serial:     string(NoSerial),
					},
					testElement{
						synthCount: 1,
						serial:     string(NoSerial),
					})
				return mkStateUpdate(persistentElements, synth)
			}(),
			expectedAllocations:   0,
			expectedPresentations: 5,
			expectingRequest:      true,
			expectedRequest: loadRequest{
				Direction: Before,
			},
			stateSize: 3,
		},
		{
			name:       "load a truly stateful element",
			sendUpdate: true,
			update: func() stateUpdate {
				// Send an update to provide a few elements to work with.
				persistentElements = append(persistentElements, actuallyStatefulElement{
					serial: "serial",
				})
				return mkStateUpdate(persistentElements, synth)
			}(),
			expectedAllocations:   1,
			expectedPresentations: 6,
			expectingRequest:      true,
			expectedRequest: loadRequest{
				Direction: Before,
			},
			stateSize: 4,
		},
		{
			name:       "compact the stateful element",
			sendUpdate: true,
			update: func() stateUpdate {
				// Send an update to provide a few elements to work with.
				persistentElements = persistentElements[:len(persistentElements)-1]
				su := mkStateUpdate(persistentElements, synth)
				su.CompactedSerials = []Serial{"serial"}
				return su
			}(),
			expectedAllocations:   0,
			expectedPresentations: 5,
			expectingRequest:      true,
			expectedRequest: loadRequest{
				Direction: Before,
			},
			stateSize: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Send a state update if configured.
			if tc.sendUpdate {
				updates <- []stateUpdate{tc.update}
			}

			// Lay out the managed list.
			list.Layout(gtx, m.UpdatedLen(&list), m.Layout)

			// Ensure the hooks were invoked the expected number of times.
			if allocationCounter != tc.expectedAllocations {
				t.Errorf("expected allocator to be called %d times, was called %d", tc.expectedAllocations, allocationCounter)
			}
			if presenterCounter != tc.expectedPresentations {
				t.Errorf("expected presenter to be called %d times, was called %d", tc.expectedPresentations, presenterCounter)
			}
			presenterCounter = 0
			allocationCounter = 0

			// Check the loadRequest, if any.
			var request loadRequest
			select {
			case req := <-requests:
				request = req.(loadRequest)
				if !tc.expectingRequest {
					t.Errorf("did not expect load request %v", request)
				} else if tc.expectedRequest.Direction != request.Direction {
					t.Errorf("expected loadRequest %v, got %v", tc.expectedRequest, request)
				}
			default:
			}
			if tc.stateSize != len(m.elementState) {
				t.Errorf("expected %d states allocated, got %d", tc.stateSize, len(m.elementState))
			}
		})
	}
}

// TestManagerPreftch tests that load requests are only issued when the index
// falls into the prefetch zones, and that any requests are for the proper
// direction.
func TestManagerPrefetch(t *testing.T) {
	for _, tc := range []struct {
		// name of test case.
		name string
		// prefetch percentage.
		prefetch float32
		// elements to allocate in the list.
		elements int
		// index to scroll to.
		index int
		// expcted request, nil if we are not expecting anything.
		expect *loadRequest
	}{
		{
			name:     "index outside prefetch zone: no prefetch",
			prefetch: 0.2,
			elements: 10,
			index:    5,
			expect:   nil,
		},
		{
			// This case used to not prefetch, but Gio's list now lays out one element
			// above and below the visible viewport in order to implement proper focus
			// navigation to off-screen elements.
			name:     "index on prefetch boundary: prefetch due to list impl",
			prefetch: 0.2,
			elements: 10,
			index:    2,
			expect:   &loadRequest{Direction: Before},
		},
		{
			name:     "index on one after prefetch boundary: no prefetch",
			prefetch: 0.2,
			elements: 10,
			index:    3,
			expect:   nil,
		},
		{
			name:     "index on prefetch boundary: no prefetch",
			prefetch: 0.2,
			elements: 10,
			index:    7,
			expect:   nil,
		},
		{
			name:     "index inside 'before' zone: prefetch before",
			prefetch: 0.2,
			elements: 10,
			index:    1,
			expect:   &loadRequest{Direction: Before},
		},
		{
			name:     "index inside 'before' zone: prefetch before",
			prefetch: 0.2,
			elements: 10,
			index:    0,
			expect:   &loadRequest{Direction: Before},
		},
		{
			name:     "index inside 'after' zone: prefetch after",
			prefetch: 0.2,
			elements: 10,
			index:    8,
			expect:   &loadRequest{Direction: After},
		},
		{
			name:     "index inside 'after' zone: prefetch after",
			prefetch: 0.2,
			elements: 10,
			index:    9,
			expect:   &loadRequest{Direction: After},
		},
		{
			name:     "index greater than bounds: load after",
			prefetch: 0.2,
			elements: 10,
			index:    10, // zero-index means the last index would be '9'
			expect:   &loadRequest{Direction: After},
		},
		{
			name:     "index less than bounds: load before",
			prefetch: 0.2,
			elements: 10,
			index:    -1,
			expect:   &loadRequest{Direction: Before},
		},
		{
			name:     "zeroed prefetch should default",
			prefetch: 0.0,
			elements: 10,
			index:    1,
			expect:   &loadRequest{Direction: Before},
		},
		{
			name:     "too few elements: load after",
			prefetch: 0.15,
			elements: 2,
			index:    1, // indexf 0.5
			expect:   &loadRequest{Direction: After},
		},
		{
			name:     "too few elements: load after",
			prefetch: 0.15,
			elements: 6,
			index:    3, // indexf 0.6
			expect:   &loadRequest{Direction: After},
		},
		{
			name:     "just enough elements: no load",
			prefetch: 0.15,
			elements: 7,
			index:    3, // indexf 0.6
			expect:   nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ops op.Ops
				gtx = app.NewContext(&ops, app.FrameEvent{
					Now: time.Now(),
					Metric: unit.Metric{
						PxPerDp: 1,
						PxPerSp: 1,
					},
					Size: image.Pt(1, 1),
				})
				list         layout.List
				requests     = make(chan interface{}, 1)
				stateUpdates = make(chan []stateUpdate, 1)
				viewports    = make(chan viewport, 1)
			)

			// Manually allocate list manager.
			// Constructor doesn't help with testing.
			m := Manager{
				hooks:        testHooks,
				Prefetch:     tc.prefetch,
				requests:     requests,
				stateUpdates: stateUpdates,
				elementState: make(map[Serial]interface{}),
				elements: func() Synthesis {
					elements := make([]Element, tc.elements)
					for ii := 0; ii < tc.elements; ii++ {
						elements[ii] = &actuallyStatefulElement{
							serial: strconv.Itoa(ii),
						}
					}
					return Synthesize(elements, func(_, x, _ Element) []Element {
						return []Element{x}
					})
				}(),
				viewports: viewports,
			}

			// Set the index position to render the list at.
			list.Position = layout.Position{
				BeforeEnd: true,
				First:     tc.index,
				Count:     1,
			}

			// Lay out the managed list.
			list.Layout(gtx, m.UpdatedLen(&list), m.Layout)

			select {
			case req := <-requests:
				request := req.(loadRequest)
				if tc.expect == nil {
					t.Errorf("did not expect load request %v", request)
				}
				if tc.expect != nil && tc.expect.Direction != request.Direction {
					t.Errorf("got %v, want %v", request, tc.expect)
				}
			default:
			}
		})
	}
}

// dupSlice returns a slice composed of the same elements in the same order,
// but backed by a different array.
func dupSlice(in []Element) []Element {
	out := make([]Element, len(in))
	copy(out, in)
	return out
}

func TestManagerViewportOnRemoval(t *testing.T) {
	// create a fake rendering context
	var ops op.Ops
	gtx := app.NewContext(&ops, app.FrameEvent{
		Now: time.Now(),
		Metric: unit.Metric{
			PxPerDp: 1,
			PxPerSp: 1,
		},
		// Make the viewport small enough that only a few list elements
		// fit on screen at a time.
		Size: image.Pt(10, 10),
	})

	var list layout.List
	synth := func(a, b, c Element) []Element { return []Element{b} }

	m := NewManager(6, Hooks{
		Allocator: func(e Element) interface{} { return nil },
		Presenter: func(e Element, state interface{}) layout.Widget {
			return layout.Spacer{
				Width:  unit.Dp(5),
				Height: unit.Dp(5),
			}.Layout
		},
		Loader:      func(dir Direction, relativeTo Serial) ([]Element, bool) { return nil, false },
		Invalidator: func() {},
		Comparator:  func(a, b Element) bool { return true },
		Synthesizer: synth,
	})
	// Shut down the existing background processing for this manager.
	close(m.requests)

	// Replace the background processing channels with channels we can control
	// from within the test.
	updates := make(chan []stateUpdate, 1)
	m.requests = nil
	m.stateUpdates = updates

	var persistentElements []Element

	type testcase struct {
		name                           string
		sendUpdate                     bool
		update                         stateUpdate
		startFirstIndex, endFirstIndex int
	}

	for _, tc := range []testcase{
		{
			name:       "load initial elements",
			sendUpdate: true,
			update: func() stateUpdate {
				// Send an update to provide a few elements to work with.
				return mkStateUpdate(dupSlice(testElements), synth)
			}(),
			startFirstIndex: 0,
			endFirstIndex:   0,
		},
		{
			name:       "remove first visible element",
			sendUpdate: true,
			update: func() stateUpdate {
				persistentElements = dupSlice(append(testElements[0:2], testElements[3:]...))
				return mkStateUpdate(persistentElements, synth)
			}(),
			startFirstIndex: 2,
			endFirstIndex:   1,
		},
		{
			name:       "replace all elements, inserting many non-serial elements",
			sendUpdate: true,
			update: func() stateUpdate {
				persistentElements = dupSlice([]Element{
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
					testElements[len(testElements)-1],
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
				})
				return mkStateUpdate(persistentElements, synth)
			}(),
			startFirstIndex: 0,
			endFirstIndex:   0,
		},
		{
			name:       "remove the one stateful element",
			sendUpdate: true,
			update: func() stateUpdate {
				persistentElements = dupSlice([]Element{
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
					testElement{
						serial:     string(NoSerial),
						synthCount: 1,
					},
				})
				return mkStateUpdate(persistentElements, synth)
			}(),
			startFirstIndex: 2,
			endFirstIndex:   0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Send a state update if configured.
			if tc.sendUpdate {
				updates <- []stateUpdate{tc.update}
			}

			// Lay out the managed list.
			list.Position.First = tc.startFirstIndex
			list.Layout(gtx, m.UpdatedLen(&list), m.Layout)

			// Ensure that the viewport is in the right place afterward.
			if list.Position.First != tc.endFirstIndex {
				t.Errorf("Expected list.Position.First to be %d after layout, got %d",
					tc.endFirstIndex, list.Position.First)
			}
		})
	}
}

// testHooks allocates default hooks for testing.
var testHooks = Hooks{
	Allocator: func(e Element) interface{} {
		switch e.(type) {
		case actuallyStatefulElement:
			// Just allocate something, doesn't matter what.
			return actuallyStatefulElement{}
		}
		return nil
	},
	Presenter: func(e Element, state interface{}) layout.Widget {
		switch e.(type) {
		case actuallyStatefulElement:
			// Trigger a panic if the wrong state type was provided.
			_ = state.(actuallyStatefulElement)
		}
		return layout.Spacer{
			Width:  unit.Dp(5),
			Height: unit.Dp(5),
		}.Layout
	},
	Loader: func(dir Direction, relativeTo Serial) ([]Element, bool) {
		return nil, false
	},
	Invalidator: func() {},
	Comparator:  func(a, b Element) bool { return true },
	Synthesizer: func(a, b, c Element) []Element { return nil },
}

// TestManagerGC ensures that the Manager cleans up its async goroutine
// when it is garbage collected.
func TestManagerGC(t *testing.T) {
	targetGoroutine := "git.sr.ht/~gioverse/chat/list.asyncProcess.func1"

	if goroutineRunning(targetGoroutine) {
		t.Skip("Another list manager is executing concurrently, cannot test cleanup.")
	}

	mgr := NewManager(10, DefaultHooks(nil, nil))
	_ = mgr // Pretend to use the variable so that it isn't "unused"
	timeout := time.NewTicker(time.Second)

	for !goroutineRunning(targetGoroutine) {
		select {
		case <-timeout.C:
			t.Fatalf("timed out waiting for async goroutine to launch")
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	// mgr = nil
	// Garbage collect twice, once to run the finalizer and once to
	// actually destroy the manager.
	runtime.GC()
	runtime.GC()
	if goroutineRunning(targetGoroutine) {
		t.Errorf("Destroying a list manager did not clean up background goroutine.")
	}
}

// TestManagerGC ensures that the Manager cleans up its async goroutine
// when it is garbage collected.
func TestManagerModifyAfterShutdown(t *testing.T) {
	mgr := NewManager(10, DefaultHooks(nil, nil))
	mgr.Shutdown()
	mgr.Modify(nil, nil, nil) // Should not panic.
}

// goroutineRunning returns whether a goroutine is currently executing
// within the provided function name. It only checks the first 100
// goroutines, and it does not differentiate between a goroutine
// currently executing the target function and a goroutine that
// is in a deeper function with the target higher in the call
// stack.
func goroutineRunning(name string) bool {
	var grs [100]runtime.StackRecord
	n, _ := runtime.GoroutineProfile(grs[:])
	active := grs[:n]

	for _, gr := range active {
		frames := runtime.CallersFrames(gr.Stack())
	frameLoop:
		for {
			frame, more := frames.Next()
			if frame.Func.Name() == name {
				return true
			}
			if !more {
				break frameLoop
			}
		}
	}
	return false
}
