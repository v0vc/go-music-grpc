package ui

import (
	"image"
	"time"

	"gioui.org/op/clip"
	"gioui.org/op/paint"

	page "github.com/v0vc/go-music-grpc/gio-gui/pages"

	"github.com/v0vc/go-music-grpc/gio-gui/icon"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/async"
	"github.com/v0vc/go-music-grpc/gio-gui/gen"
	lay "github.com/v0vc/go-music-grpc/gio-gui/layout"
	"github.com/v0vc/go-music-grpc/gio-gui/list"
	"github.com/v0vc/go-music-grpc/gio-gui/model"
)

var (
	// SidebarMaxWidth specifies how large the sidebar should be on
	// desktop layouts.
	SidebarMaxWidth = unit.Dp(250)
	// Breakpoint at which to switch from desktop to mobile layout.
	Breakpoint = unit.Dp(600)
	// LoadSize specifies maximum number of items to load at a time.
	LoadSize = 30
)

// UI manages the state for the entire application's UI.
type UI struct {
	// Loader loads resources asynchronously.
	// Deallocates stale resources.
	// Stale is defined as "not being scheduled frequently".
	async.Loader
	// Rooms is the root of the data, containing messages chunked by
	// room.
	// It also contains interact state, rather than maintaining two
	// separate lists for the model and state.
	Rooms Rooms
	// RowTracker *model.Messages
	// RoomList for the sidebar.
	RoomList widget.List
	// Modal can show widgets atop the rest of the gio-gui.
	Modal component.ModalState
	// Back button navigates out of a room.
	Back widget.Clickable
	// InsideRoom if we are currently in the room view.
	// Used to decide when to render the sidebar on small viewports.
	InsideRoom bool
	// AddBtn holds click state for a button that adds a new message to
	// the current room.
	AddBtn widget.Clickable
	// DeleteBtn holds click state for a button that removes a message
	// from the current room.
	DeleteBtn widget.Clickable

	DownloadBtn widget.Clickable
	// MessageMenu is the context menu available on messages.
	MessageMenu component.MenuState
	// ContextMenuTarget tracks the message state on which the context
	// menu is currently acting.
	ContextMenuTarget *model.Message

	Invalidator func()
	th          *page.Theme
}

// NewUI constructs a UI and populates it with data.
func NewUI(invalidator func(), theme *page.Theme) *UI {
	var ui UI

	ui.th = theme

	ui.Invalidator = invalidator
	ui.Modal.VisibilityAnimation.Duration = time.Millisecond * 250

	ui.MessageMenu = component.MenuState{
		Options: []func(gtx layout.Context) layout.Dimensions{
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.DeleteBtn, "Delete")
				item.Icon = icon.DeleteIcon
				item.Hint = component.MenuHintText(ui.th.Theme, "Test")
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.DownloadBtn, "Download")
				item.Icon = icon.DownloadIcon
				return item.Layout(gtx)
			},
		},
	}

	g := &gen.Generator{}

	// Generate most of the model data.
	rooms := g.GetChannels(1)
	for _, r := range rooms.List() {
		mess := model.Messages{}
		rt := &RowTracker{
			SerialToIndex: make(map[list.Serial]int),
			Generator:     g,
			Messages:      &mess,
			MaxLoads:      LoadSize,
			ScrollToEnd:   false,
		}
		room := &Room{
			Room:       r,
			RowTracker: rt,
		}
		room.List.ScrollToEnd = room.RowTracker.ScrollToEnd
		room.List.Axis = layout.Vertical
		ui.Rooms.List = append(ui.Rooms.List, room)
	}

	// spin up a bunch of async actors to send messages to rooms.
	/*	for _, u := range users.List() {
		u := u
		if u.Name == local.Name {
			continue
		}
		go func() {
			for {
				var (
					respond = time.Second * time.Duration(1)
					compose = time.Second * time.Duration(1)
					room    = ui.Rooms.Random()
				)
				func() {
					time.Sleep(respond)
					// room.SetComposing(u.Name, true)
					time.Sleep(compose)
					// room.SetComposing(u.Name, false)
					room.Send(u.Name, lorem.Paragraph(1, 4))
				}()
			}
		}()
	}*/

	/*	for ii := range ui.Rooms.List {
		ui.Rooms.List[ii].List.ScrollToEnd = false
		ui.Rooms.List[ii].List.Axis = layout.Vertical
		//ui.Rooms.List[ii].List.Position.First = 0
		//ui.Rooms.List[ii].List.Position.Offset = 0
	}*/
	ui.Rooms.SelectAndFill(0, invalidator, ui.presentChatRow)

	return &ui
}

// Layout the application UI.
func (ui *UI) Layout(gtx layout.Context) layout.Dimensions {
	return ui.Loader.Frame(gtx, ui.layout)
}

func (ui *UI) layout(gtx layout.Context) layout.Dimensions {
	small := gtx.Constraints.Max.X < gtx.Dp(Breakpoint)
	for ii := range ui.Rooms.List {
		r := ui.Rooms.List[ii]
		if r.Interact.Clicked() {
			// ui.Rooms.Select(ii)
			ui.Rooms.SelectAndFill(ii, ui.Invalidator, ui.presentChatRow)
			ui.InsideRoom = true
			break
		}
	}
	if ui.Back.Clicked() {
		ui.InsideRoom = false
	}
	paint.FillShape(gtx.Ops, ui.th.Palette.BgSecondary, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
	if small {
		if !ui.InsideRoom {
			return ui.layoutRoomList(gtx)
		}
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(
			gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTopbar(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min = gtx.Constraints.Max
						return ui.layoutChat(gtx)
					}),
				)
			}),
		)
	}
	return layout.Flex{
		Axis: layout.Horizontal,
	}.Layout(
		gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(SidebarMaxWidth)
			gtx.Constraints.Min = gtx.Constraints.Constrain(gtx.Constraints.Min)
			return ui.layoutRoomList(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return ui.layoutChat(gtx)
				}),
			)
		}),
	)
}

// layoutChat lays out the chat interface with associated controls.
func (ui *UI) layoutChat(gtx layout.Context) layout.Dimensions {
	room := ui.Rooms.Active()
	listStyle := material.List(ui.th.Theme, &room.List)
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return listStyle.Layout(gtx,
				room.ListState.UpdatedLen(&room.List.List),
				room.ListState.Layout,
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return lay.Background(ui.th.Palette.BgSecondary).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if ui.AddBtn.Clicked() {
					active := ui.Rooms.Active()
					// active.SendLocal(active.Editor.Text())
					active.RunSearch(active.Editor.Text())
					active.Editor.SetText("")
				}
				if ui.DeleteBtn.Clicked() {
					serial := ui.ContextMenuTarget.Serial()
					ui.Rooms.Active().DeleteRow(serial)
				}
				return layout.Inset{
					Bottom: unit.Dp(8),
					Top:    unit.Dp(8),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gutter := lay.Gutter()
					gutter.RightWidth = gutter.RightWidth + listStyle.ScrollbarStyle.Width()
					return gutter.Layout(gtx,
						nil,
						func(gtx layout.Context) layout.Dimensions {
							return ui.layoutEditor(gtx)
						},
						material.IconButton(ui.th.Theme, &ui.AddBtn, icon.Search, "Search").Layout,
					)
				})
			})
		}),
	)
}

// layoutTopbar lays out a context bar that contains a "back" button and
// room title for context.
func (ui *UI) layoutTopbar(gtx layout.Context) layout.Dimensions {
	room := ui.Rooms.Active()
	return layout.Stack{}.Layout(
		gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return component.Rect{
				Size: image.Point{
					X: gtx.Constraints.Max.X,
					Y: gtx.Constraints.Min.Y,
				},
				Color: ui.th.Palette.Surface,
			}.Layout(gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(
				gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(ui.th.Theme, &ui.Back, icon.NavBack, "Back")
					btn.Color = ui.th.Fg
					btn.Background = ui.th.Palette.Surface
					return btn.Layout(gtx)
				}),
				/*layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return chatwidget.Image{
						Image: widget.Image{
							Src: room.Interact.Image.Op(),
						},
						Width:  unit.Dp(24),
						Height: unit.Dp(24),
						Radii:  unit.Dp(3),
					}.Layout(gtx)
				}),*/
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Label(ui.th.Theme, unit.Sp(14), room.Name).Layout(gtx)
				}),
			)
		}),
	)
}

// layoutRoomList lays out a list of rooms that can be clicked to view
// the messages in that room.
func (ui *UI) layoutRoomList(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(
		gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return component.Rect{
				Size: image.Point{
					X: gtx.Constraints.Min.X,
					Y: gtx.Constraints.Max.Y,
				},
				Color: ui.th.Palette.Surface,
			}.Layout(gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			ui.RoomList.Axis = layout.Vertical
			gtx.Constraints.Min = gtx.Constraints.Max
			return material.List(ui.th.Theme, &ui.RoomList).Layout(gtx, len(ui.Rooms.List), func(gtx layout.Context, ii int) layout.Dimensions {
				r := ui.Rooms.Index(ii)
				latest := r.Latest()
				return CreateChannel(ui.th.Theme, &r.Interact, &ChannelConfig{
					Name:    r.Room.Name,
					Image:   r.Room.Image,
					Content: latest.Content,
					SentAt:  latest.SentAt,
				}).Layout(gtx)
			})
		}),
	)
}

// layoutEditor lays out the message editor.
func (ui *UI) layoutEditor(gtx layout.Context) layout.Dimensions {
	return lay.Rounded(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return lay.Background(ui.th.Palette.Surface).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				active := ui.Rooms.Active()
				editor := &active.Editor
				for _, e := range editor.Events() {
					switch e.(type) {
					case widget.SubmitEvent:
						// active.SendLocal(editor.Text())
						active.RunSearch(editor.Text())
						editor.SetText("")
					}
				}
				editor.Submit = true
				editor.SingleLine = true
				return material.Editor(ui.th.Theme, editor, "Search").Layout(gtx)
			})
		})
	})
}

// presentChatRow returns a widget closure that can layout the given chat item.
// `data` contains managed data for this chat item, `state` contains UI defined
// interactive state.
func (ui *UI) presentChatRow(data list.Element, state interface{}) layout.Widget {
	switch el := data.(type) {
	case model.Message:
		elemState, ok := state.(*Row)
		if !ok {
			return func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
		}
		return func(gtx layout.Context) layout.Dimensions {
			/*			if state.Clicked() {
						ui.Modal.Show(gtx.Now, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(25)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return widget.Image{
									Src:      state.Avatar.Op(),
									Fit:      widget.ScaleDown,
									Position: layout.Center,
								}.Layout(gtx)
							})
						})
					}*/
			if elemState.ContextArea.Active() {
				// If the right-click context area for this message is activated,
				// inform the UI that this message is the target of any action
				// taken within that menu.
				ui.ContextMenuTarget = &el
			}
			return ui.row(el, elemState)(gtx)
		}
	case model.DateBoundary:
		// return DateSeparator(th.Theme, data.Date).Layout
		return func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	case model.UnreadBoundary:
		return UnreadSeparator(ui.th.Theme).Layout
	default:
		return func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
}

// row returns RowStyle
func (ui *UI) row(data model.Message, state *Row) layout.Widget {
	msg := NewRow(ui.th.Theme, state, &ui.MessageMenu, &RowConfig{
		Sender:  data.Sender,
		Content: data.Content,
		SentAt:  data.SentAt,
		Avatar:  data.Avatar,
	})

	return msg.Layout
}
