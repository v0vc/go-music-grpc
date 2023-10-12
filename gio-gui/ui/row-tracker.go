package ui

import (
	"sort"
	"sync"

	"github.com/v0vc/go-music-grpc/gio-gui/gen"
	"github.com/v0vc/go-music-grpc/gio-gui/list"
	"github.com/v0vc/go-music-grpc/gio-gui/model"
)

// RowTracker is a stand-in for an application's data access logic.
// It stores a set of chat messages and can load them on request.
// It simulates network latency during the load operations for
// realism.
type RowTracker struct {
	// SimulateLatency is the maximum latency in milliseconds to
	// simulate on loads.
	// SimulateLatency int
	sync.Mutex
	Rows          []list.Element
	SerialToIndex map[list.Serial]int
	// Users         *model.Users
	Messages *model.Messages
	// Local         *model.User
	Generator *gen.Generator
	// MaxLoads specifies the number of elements a given load in either
	// direction can return.
	MaxLoads    int
	ScrollToEnd bool
}

// Send adds the message to the data model.
// This is analogous to interacting with the backend api.
/*func (rt *RowTracker) Send(user, content string) model.Message {
		u, ok := rt.RowTracker.Lookup(user)
		if !ok {
			return model.Message{}
		}
	msg := rt.Generator.GenNewMessage(u.Sender, content)
	msg := rt.Generator.GenNewMessage("Album", content)
	rt.Add(msg)
	return msg
}*/

// Add a list element as a row of data to track.
func (rt *RowTracker) Add(r list.Element) {
	rt.Lock()
	rt.Rows = append(rt.Rows, r)
	rt.reindex()
	rt.Unlock()
}

// Latest returns the latest element, or nil.
func (rt *RowTracker) Latest() list.Element {
	rt.Lock()
	final := len(rt.Rows) - 1
	// Unlock because index will lock again.
	rt.Unlock()
	return rt.Index(final)
}

// Index returns the element at the given index, or nil.
func (rt *RowTracker) Index(ii int) list.Element {
	rt.Lock()
	defer rt.Unlock()
	if len(rt.Rows) == 0 || len(rt.Rows) < ii {
		return nil
	}
	if ii < 0 {
		return rt.Rows[0]
	}
	return rt.Rows[ii]
}

// NewRow generates a new row.
/*func (rt *RowTracker) NewRow() list.Element {
	el := rt.Generator.GenNewMessage(rt.Messages.Random().Sender, "test new", len(rt.Rows))
	rt.Add(el)
	return el
}*/

// Load simulates loading chat history from a database or API. It
// sleeps for a random number of milliseconds and then returns
// some messages.
func (rt *RowTracker) Load(dir list.Direction, relativeTo list.Serial) (loaded []list.Element, more bool) {
	rt.Lock()
	defer rt.Unlock()
	defer func() {
		// Ensure the slice we return is backed by different memory than the underlying
		// RowTracker's slice, to avoid data races when the RowTracker sorts its storage.
		loaded = dupSlice(loaded)
	}()
	numRows := len(rt.Rows)
	if relativeTo == list.NoSerial {
		// If loading relative to nothing, likely the chat interface is empty.
		// We should load the most recent messages first in this case, regardless
		// of the direction parameter.

		if rt.ScrollToEnd {
			return rt.Rows[numRows-min(rt.MaxLoads, numRows):], numRows > rt.MaxLoads
		} else {
			var res int
			if numRows < rt.MaxLoads {
				res = numRows
			} else {
				res = rt.MaxLoads
			}
			return rt.Rows[:res], numRows > rt.MaxLoads
		}
	}
	idx := rt.SerialToIndex[relativeTo]
	if dir == list.After {
		end := min(numRows, idx+rt.MaxLoads)
		return rt.Rows[idx+1 : end], end < len(rt.Rows)-1
	}
	start := maximum(0, idx-rt.MaxLoads)
	return rt.Rows[start:idx], start > 0
}

// Delete removes the element with the provided serial from storage.
func (rt *RowTracker) Delete(serial list.Serial) {
	rt.Lock()
	defer rt.Unlock()
	idx := rt.SerialToIndex[serial]
	sliceRemove(&rt.Rows, idx)
	rt.reindex()
}

func (rt *RowTracker) reindex() {
	sort.Slice(rt.Rows, func(i, j int) bool {
		return rowLessThan(rt.Rows[i], rt.Rows[j])
	})
	rt.SerialToIndex = make(map[list.Serial]int)
	for i, row := range rt.Rows {
		rt.SerialToIndex[row.Serial()] = i
	}
}

// dupSlice returns a slice composed of the same elements in the same order,
// but backed by a different array.
func dupSlice(in []list.Element) []list.Element {
	out := make([]list.Element, len(in))
	copy(out, in)
	/*for i := range in {
		out[i] = in[i]
	}*/
	/*	sort.Slice(out, func(i, j int) bool {
		return out[j].Serial() > out[i].Serial()
	})*/
	return out
}

// sliceRemove takes the given index of a slice and swaps it with the final
// index in the slice, then shortens the slice by one element. This hides
// the element at index from the slice, though it does not erase its data.
func sliceRemove(s *[]list.Element, index int) {
	lastIndex := len(*s) - 1
	(*s)[index], (*s)[lastIndex] = (*s)[lastIndex], (*s)[index]
	*s = (*s)[:lastIndex]
}

func maximum(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
