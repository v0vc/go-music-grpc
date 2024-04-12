package ui

import (
	"fmt"
	"image"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/async"
	"github.com/v0vc/go-music-grpc/gio-gui/gen"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
	lay "github.com/v0vc/go-music-grpc/gio-gui/layout"
	"github.com/v0vc/go-music-grpc/gio-gui/list"
	"github.com/v0vc/go-music-grpc/gio-gui/model"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
)

const (
	artistRegexString = `^https://zvuk.com/artist/(\d+)$`
	artistUrl         = "https://zvuk.com/artist/"
	releaseUrl        = "https://zvuk.com/release/"
)

var (
	// SidebarMaxWidth specifies how large the sidebar should be on
	// desktop layouts.
	SidebarMaxWidth = unit.Dp(250)
	// Breakpoint at which to switch from desktop to mobile layout.
	Breakpoint = unit.Dp(600)
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
	// Back button navigates out of a room.
	Back widget.Clickable
	// InsideRoom if we are currently in the room view.
	// Used to decide when to render the sidebar on small viewports.
	InsideRoom bool
	// room menu
	SyncBtn, DownloadChannelBtn, CopyChannelBtn, DeleteBtn widget.Clickable
	// message menu
	CopyAlbBtn, CopyAlbArtistBtn, DownloadBtn widget.Clickable
	// MessageMenu is the context menu available on messages.
	MessageMenu component.MenuState
	// ChannelMenu is the context menu available on channel.
	ChannelMenu component.MenuState
	// ContextMenuTarget tracks the message state on which the context
	// menu is currently acting.
	ContextMenuTarget *model.Message

	ChannelMenuTarget *Room
	Invalidator       func()
	th                *page.Theme
	SiteId            uint32
	LoadSize          int
}

// NewUI constructs a UI and populates it with data.
func NewUI(invalidator func(), theme *page.Theme, loadSize int, siteId uint32) *UI {
	var ui UI
	ui.th = theme
	ui.Invalidator = invalidator
	ui.SiteId = siteId
	ui.LoadSize = loadSize

	ui.MessageMenu = component.MenuState{
		Options: []func(gtx layout.Context) layout.Dimensions{
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.DownloadBtn, "Download")
				item.Icon = icon.DownloadIcon
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.CopyAlbBtn, "Copy Album")
				item.Icon = icon.CopyIcon
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.CopyAlbArtistBtn, "Copy Artist")
				item.Icon = icon.CopyIcon
				return item.Layout(gtx)
			},
		},
	}
	ui.ChannelMenu = component.MenuState{
		Options: []func(gtx layout.Context) layout.Dimensions{
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.SyncBtn, "Sync")
				item.Icon = icon.SyncIcon
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.DeleteBtn, "Delete")
				item.Icon = icon.DeleteIcon
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.DownloadChannelBtn, "Download")
				item.Icon = icon.DownloadIcon
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.CopyChannelBtn, "Copy")
				item.Icon = icon.CopyIcon
				return item.Layout(gtx)
			},
		},
	}

	g := &gen.Generator{}

	// Generate most of the model data.
	rooms, err := g.GetChannels(siteId)
	MapDto(&ui, rooms, nil, g)
	ui.Rooms.SelectAndFill(siteId, 0, nil, invalidator, ui.presentChatRow, err)
	return &ui
}

func MapDto(ui *UI, channels *model.Rooms, albums *model.Messages, g *gen.Generator) {
	for _, r := range channels.List() {
		ch := ui.Rooms.GetChannelById(r.Id)
		if ch != nil {
			ch.Lock()
			curCount, _ := strconv.Atoi(ch.Room.Count)
			ch.Room.Count = strconv.Itoa(curCount + len(albums.GetList()))
			ch.Unlock()
		} else {
			rt := &RowTracker{
				SerialToIndex: make(map[list.Serial]int),
				Generator:     g,
				Messages:      albums,
				MaxLoads:      ui.LoadSize,
				ScrollToEnd:   false,
			}
			room := &Room{
				Room:            r,
				RowTracker:      rt,
				SearchResponses: make(chan []list.Serial),
			}

			room.List.ScrollToEnd = room.RowTracker.ScrollToEnd
			room.List.Axis = layout.Vertical
			ui.Rooms.List = append(ui.Rooms.List, room)
		}
	}
}

func (ui *UI) AddChannel(siteId uint32, artistUrl string) {
	g := &gen.Generator{}
	ch := ui.Rooms.GetBaseChannel()
	if ch == nil {
		return
	}
	artistId := findArtistId(artistUrl)
	if artistId == "" {
		ch.Content = "invalid url"
		return
	} else {
		ch.Content = fmt.Sprintf("working: %v", artistId)
	}
	start := time.Now()
	channels, albums, artTitle, err := g.AddChannel(siteId, artistId)
	if err != nil {
		ch.Content = err.Error()
		return
	} else {
		ch.Content = fmt.Sprintf("%v (%v)", artTitle, time.Since(start))
	}

	MapDto(ui, channels, albums, g)
	ui.Rooms.SelectAndFill(siteId, len(ui.Rooms.List)-1, albums.GetList(), ui.Invalidator, ui.presentChatRow, nil)
}

// Layout the application UI.
func (ui *UI) Layout(gtx layout.Context) layout.Dimensions {
	return ui.Loader.Frame(gtx, ui.layout)
}

func (ui *UI) layout(gtx layout.Context) layout.Dimensions {
	small := gtx.Constraints.Max.X < gtx.Dp(Breakpoint)

	for ii := range ui.Rooms.List {
		r := ui.Rooms.List[ii]
		if r.Interact.Clicked(gtx) {
			// ui.Rooms.Select(ii)
			ui.Rooms.SelectAndFill(ui.SiteId, ii, nil, ui.Invalidator, ui.presentChatRow, nil)
			ui.InsideRoom = true
			break
		}
	}

	if ui.Back.Clicked(gtx) {
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
				// inset := layout.UniformInset(unit.Dp(8))
				return layout.Inset{
					Bottom: unit.Dp(8),
					Top:    unit.Dp(8),
					Left:   unit.Dp(8),
					Right:  unit.Dp(8),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutEditor(gtx)
					/*gutter := lay.Gutter()
					gutter.RightWidth = gutter.RightWidth + listStyle.ScrollbarStyle.Width()
					return gutter.Layout(gtx,
						nil,
						func(gtx layout.Context) layout.Dimensions {
							return ui.layoutEditor(gtx)
						},
						material.IconButton(ui.th.Theme, &ui.AddBtn, icon.Search, "Search").Layout,
					)*/
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
				if ui.SyncBtn.Clicked(gtx) {
					channel := ui.ChannelMenuTarget
					channel.Content = "In progress..."
					go channel.SyncArtist(&ui.Rooms, ui.SiteId)
				}
				if ui.DownloadChannelBtn.Clicked(gtx) {
					channel := ui.ChannelMenuTarget
					if channel.Loaded {
						var albumIds []string
						for i := range channel.RowTracker.Rows {
							alb := channel.RowTracker.Rows[i].(model.Message)
							albumIds = append(albumIds, alb.Status)
						}
						go channel.DownloadAlbum(ui.SiteId, albumIds, "mid")
					} else {
						go channel.DownloadArtist(ui.SiteId, channel.Id, "mid")
					}
				}
				if ui.CopyChannelBtn.Clicked(gtx) && !ui.ChannelMenuTarget.IsBase {
					switch ui.SiteId {
					case 1:
						gtx.Execute(clipboard.WriteCmd{
							Data: io.NopCloser(strings.NewReader(artistUrl + ui.ChannelMenuTarget.Id)),
						})
					}
				}
				if ui.DeleteBtn.Clicked(gtx) {
					if ui.ChannelMenuTarget.IsBase {
						// Delete на -=NEW=- сделаем очистку статусу синка
						for _, ch := range ui.Rooms.List {
							ch.Room.Count = ""
						}
						go ui.ChannelMenuTarget.ClearSync(ui.SiteId)
					} else {
						ind := slices.Index(ui.Rooms.List, ui.ChannelMenuTarget)
						if ui.ChannelMenuTarget.Interact.Active {
							ui.Rooms.SelectAndFill(ui.SiteId, ind-1, nil, ui.Invalidator, ui.presentChatRow, nil)
						}
						ui.Rooms.List = ui.Rooms.DeleteChannel(ind, ui.SiteId)
					}
				}
				r := ui.Rooms.Index(ii)
				// latest := r.Latest()
				if r.Interact.ContextArea.Active() {
					ui.ChannelMenuTarget = r
				}
				return CreateChannel(ui.th.Theme, &r.Interact, &ui.ChannelMenu, &ChannelConfig{
					Name:    r.Room.Name,
					Image:   r.Room.Image,
					Content: r.Room.Content,
					Count:   r.Count,
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
				/*for _, e := range editor.Events() {
					if _, ok := e.(widget.ChangeEvent); ok {
						active.RunSearch(editor.Text())
						break
					}
				}*/
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
			if elemState.ContextArea.Active() {
				// If the right-click context area for this message is activated,
				// inform the UI that this message is the target of any action
				// taken within that menu.
				ui.ContextMenuTarget = &el
			}
			if ui.CopyAlbBtn.Clicked(gtx) {
				switch ui.SiteId {
				case 1:
					gtx.Execute(clipboard.WriteCmd{
						Data: io.NopCloser(strings.NewReader(releaseUrl + ui.ContextMenuTarget.Status)),
					})
				}
			}
			if ui.CopyAlbArtistBtn.Clicked(gtx) {
				switch ui.SiteId {
				case 1:
					var sb []string
					for _, artId := range ui.ContextMenuTarget.ParentId {
						sb = append(sb, artistUrl+artId)
					}

					gtx.Execute(clipboard.WriteCmd{
						Data: io.NopCloser(strings.NewReader(strings.Join(sb, ", "))),
					})
				}
			}
			if ui.DownloadBtn.Clicked(gtx) {
				active := ui.Rooms.Active()
				go active.DownloadAlbum(ui.SiteId, []string{ui.ContextMenuTarget.Status}, "mid")
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

// row returns RowStyle.
func (ui *UI) row(data model.Message, state *Row) layout.Widget {
	msg := NewRow(ui.th.Theme, state, &ui.MessageMenu, &RowConfig{
		Title:   data.Title,
		Content: data.Content,
		Type:    data.Type,
		SentAt:  data.SentAt,
		Avatar:  data.Avatar,
	})

	return msg.Layout
}

func findArtistId(url string) string {
	matchArtist := regexp.MustCompile(artistRegexString).FindStringSubmatch(url)
	if matchArtist == nil {
		return ""
	}
	return matchArtist[1]
}
