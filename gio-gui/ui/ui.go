package ui

import (
	"fmt"
	"image"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	slices2 "golang.org/x/exp/slices"

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
	zvArtistUrl     = "https://zvuk.com/artist/"
	zvReleaseUrl    = "https://zvuk.com/release/"
	youVideoUrl     = "https://www.youtube.com/watch?v="
	youChannelUrl   = "https://www.youtube.com/channel/"
	zvArtistRegex   = `^https://zvuk.com/artist/(\d+)$`
	zvReleaseRegex  = `^https://zvuk.com/release/(\d+)$`
	youVideoRegex   = "^(?:https?:)?(?:\\/\\/)?(?:youtu\\.be\\/|(?:www\\.|m\\.)?youtube\\.com\\/(?:watch|v|embed)(?:\\.php)?(?:\\?.*v=|\\/))([a-zA-Z0-9\\_-]{7,15})(?:[\\?&][a-zA-Z0-9\\_-]+=[a-zA-Z0-9\\_-]+)*(?:[&\\/\\#].*)?$"
	youChannelRegex = "^https?:\\/\\/(www\\.)?youtube\\.com\\/(channel\\/UC[\\w-]{21}[AQgw]|(c\\/|user\\/)?[\\w@-]+)$"
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
	// channel menu
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

	ChannelMenuTarget       *Room
	Invalidator             func()
	th                      *page.Theme
	SiteId                  uint32
	LoadSize                int
	ZvukQuality, YouQuality string
	RadioButtonsGroup       widget.Enum
}

// NewUI constructs a UI and populates it with data.
func NewUI(invalidator func(), theme *page.Theme, loadSize int, zvukQuality string, youQuality string, siteId uint32) *UI {
	var ui UI
	ui.th = theme
	ui.Invalidator = invalidator
	ui.SiteId = siteId
	ui.LoadSize = loadSize
	ui.ZvukQuality = zvukQuality
	ui.YouQuality = youQuality

	ui.MessageMenu = component.MenuState{
		Options: []func(gtx layout.Context) layout.Dimensions{
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.DownloadBtn, "Download")
				item.Icon = icon.DownloadIcon
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.CopyAlbBtn, "Copy Link")
				item.Icon = icon.CopyIcon
				return item.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				item := component.MenuItem(ui.th.Theme, &ui.CopyAlbArtistBtn, "Copy Author")
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
	rooms, _ := g.GetChannels(siteId)
	MapDto(&ui, rooms, nil, g)
	ui.Rooms.SelectAndFill(siteId, 0, nil, invalidator, ui.presentRow, false)
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
				Room:       r,
				RowTracker: rt,
				// SearchResponses: make(chan []list.Serial),
			}

			room.List.ScrollToEnd = room.RowTracker.ScrollToEnd
			room.List.Axis = layout.Vertical
			ui.Rooms.List = append(ui.Rooms.List, room)
		}
	}
}

func (ui *UI) MassDownload(siteId uint32) {
	curChannel := ui.Rooms.Active()
	if curChannel == nil || curChannel.Selected == nil || len(curChannel.Selected) == 0 {
		return
	}
	var quality string
	switch ui.SiteId {
	case 1:
		quality = ui.ZvukQuality
	case 4:
		quality = ui.YouQuality
	}
	go curChannel.DownloadAlbum(siteId, curChannel.Selected, quality)
}

func (ui *UI) SelectAll(value bool) {
	curChannel := ui.Rooms.Active()
	if curChannel != nil {
		curChannel.Selected = nil
		for _, data := range curChannel.RowTracker.Rows {
			switch el := data.(type) {
			case model.Message:
				elemState, ok := curChannel.ListState.GetState(data.Serial()).(*Row)
				if ok {
					elemState.Selected.Value = value
				}
				if value {
					switch ui.SiteId {
					case 1:
						curChannel.Selected = append(curChannel.Selected, el.AlbumId)
					case 4:
						curChannel.Selected = append(curChannel.Selected, el.ParentId[0]+";"+el.AlbumId+";"+el.Title)
					}
				}
			}
		}
	}
}

func (ui *UI) AddChannel(siteId uint32, url string) {
	g := &gen.Generator{}
	ch := ui.Rooms.GetBaseChannel()
	if ch == nil {
		return
	}
	var artistId string
	switch siteId {
	case 1:
		// автор со сберзвука
		artistId = findArtistId(url, true)
		if artistId == "" {
			releaseId := findArtistId(url, false)
			if releaseId != "" {
				go g.DownloadAlbum(siteId, []string{releaseId}, "mid")
				ch.Content = "download: " + releaseId
				return
			}
		}
	case 2:
		// автор со спотика
	case 3:
		// автор с дизера
	case 4:
		// автор с ютуба
		artistId = findYoutubeId(url)
	}
	if artistId == "" {
		ch.Content = "invalid url"
		return
	} else {
		ch.Content = fmt.Sprintf("In work: %v", artistId)
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
	ui.Rooms.SelectAndFill(siteId, len(ui.Rooms.List)-1, albums.GetList(), ui.Invalidator, ui.presentRow, false)
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
			ui.Rooms.SelectAndFill(ui.SiteId, ii, nil, ui.Invalidator, ui.presentRow, false)
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
				if ui.SiteId == 4 {
					if ui.RadioButtonsGroup.Update(gtx) {
						vid := make([]model.Message, 0)
						for _, i := range room.RowTracker.Rows {
							vid = append(vid, i.(model.Message))
						}
						switch ui.RadioButtonsGroup.Value {
						case "Date":
							sort.Slice(vid, func(i, j int) bool {
								return vid[i].SentAt.After(vid[j].SentAt)
							})
						case "Views":
							sort.Slice(vid, func(i, j int) bool {
								return vid[i].Views > vid[j].Views
							})
						case "Likes":
							sort.Slice(vid, func(i, j int) bool {
								return vid[i].Likes > vid[j].Likes
							})
						case "Quality":
							sort.Slice(vid, func(i, j int) bool {
								return vid[i].Quality > vid[j].Quality
							})
						default:
							sort.Slice(vid, func(i, j int) bool {
								return vid[i].SentAt.Before(vid[j].SentAt)
							})
						}

						resp := make([]list.Serial, 0)
						for _, i := range room.RowTracker.Rows {
							resp = append(resp, i.Serial())
						}
						room.RowTracker.DeleteAll()
						room.ListState.Modify(nil, nil, resp)

						res := make([]list.Element, 0)
						for i, j := range vid {
							j.SerialID = fmt.Sprintf("%05d", i+1)
							res = append(res, j)
						}
						room.RowTracker.AddAll(res)
					}
					return layout.Inset{
						Bottom: unit.Dp(8),
						Top:    unit.Dp(8),
						Left:   unit.Dp(8),
						Right:  unit.Dp(8),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(
							gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutEditor(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
							layout.Rigid(material.RadioButton(ui.th.Theme, &ui.RadioButtonsGroup, "Date", "Date").Layout),
							layout.Rigid(material.RadioButton(ui.th.Theme, &ui.RadioButtonsGroup, "Views", "Views").Layout),
							layout.Rigid(material.RadioButton(ui.th.Theme, &ui.RadioButtonsGroup, "Likes", "Likes").Layout),
							layout.Rigid(material.RadioButton(ui.th.Theme, &ui.RadioButtonsGroup, "Quality", "Quality").Layout),
						)
					})
				} else {
					return layout.Inset{
						Bottom: unit.Dp(8),
						Top:    unit.Dp(8),
						Left:   unit.Dp(8),
						Right:  unit.Dp(8),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutEditor(gtx)
					})
				}
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
					go channel.SyncArtist(&ui.Rooms, ui.SiteId)
				}
				if ui.DownloadChannelBtn.Clicked(gtx) {
					channel := ui.ChannelMenuTarget
					var quality string
					switch ui.SiteId {
					case 1:
						quality = ui.ZvukQuality
					case 4:
						quality = ui.YouQuality
					}
					if channel.Loaded {
						var albumIds []string
						for _, i := range channel.RowTracker.Rows {
							alb := i.(model.Message)
							switch ui.SiteId {
							case 1:
								albumIds = append(albumIds, alb.AlbumId)
							case 4:
								albumIds = append(albumIds, alb.ParentId[0]+";"+alb.AlbumId+";"+alb.Title)
							}
						}
						go channel.DownloadAlbum(ui.SiteId, albumIds, quality)
					} else {
						go channel.DownloadArtist(ui.SiteId, channel.Id, quality)
					}
				}
				if ui.CopyChannelBtn.Clicked(gtx) && !ui.ChannelMenuTarget.IsBase {
					switch ui.SiteId {
					case 1:
						gtx.Execute(clipboard.WriteCmd{
							Data: io.NopCloser(strings.NewReader(zvArtistUrl + ui.ChannelMenuTarget.Id)),
						})
					case 4:
						gtx.Execute(clipboard.WriteCmd{
							Data: io.NopCloser(strings.NewReader(youChannelUrl + ui.ChannelMenuTarget.Id)),
						})
					}
				}
				if ui.DeleteBtn.Clicked(gtx) {
					if ui.ChannelMenuTarget.IsBase {
						// Delete на -=NEW=- сделаем очистку статусу синка
						for _, ch := range ui.Rooms.List {
							ch.Room.Count = ""
							ch.Room.Selected = nil
						}
						go ui.ChannelMenuTarget.ClearSync(ui.SiteId)
					} else {
						var ids []string
						for _, data := range ui.ChannelMenuTarget.RowTracker.Rows {
							switch el := data.(type) {
							case model.Message:
								ids = append(ids, el.AlbumId)
							}
						}
						curCount, _ := strconv.Atoi(ui.ChannelMenuTarget.Room.Count)
						ind := slices.Index(ui.Rooms.List, ui.ChannelMenuTarget)
						if ui.ChannelMenuTarget.Interact.Active {
							ui.Rooms.SelectAndFill(ui.SiteId, ind-1, nil, ui.Invalidator, ui.presentRow, false)
						}
						ui.Rooms.List = ui.Rooms.DeleteChannel(ind, ui.SiteId)
						ch := ui.Rooms.GetBaseChannel()
						if ch != nil {
							curBaseCount, _ := strconv.Atoi(ch.Room.Count)
							if curBaseCount >= curCount {
								if curBaseCount-curCount == 0 {
									ch.Room.Count = ""
								} else {
									ch.Room.Count = strconv.Itoa(curBaseCount - curCount)
								}
							}
							for _, data := range ch.RowTracker.Rows {
								switch el := data.(type) {
								case model.Message:
									if slices.Contains(ids, el.AlbumId) {
										ch.DeleteRow(el.Serial())
									}
								}
							}
						}
					}
				}
				r := ui.Rooms.Index(ii)
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
				for {
					event, ok := editor.Update(gtx)
					if !ok {
						break
					}
					if _, ok := event.(widget.ChangeEvent); ok {
						go active.RunSearch(editor.Text())
						if editor.Text() == "" {
							ui.Rooms.Active().Loaded = false
							ui.Rooms.SelectAndFill(ui.SiteId, ui.Rooms.active, nil, ui.Invalidator, ui.presentRow, false)
						}
						break
					}
				}
				editor.Submit = true
				editor.SingleLine = true
				return material.Editor(ui.th.Theme, editor, "Search").Layout(gtx)
			})
		})
	})
}

// presentRow returns a widget closure that can layout the given chat item.
// `data` contains managed data for this chat item, `state` contains UI defined
// interactive state.
func (ui *UI) presentRow(data list.Element, state interface{}) layout.Widget {
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
						Data: io.NopCloser(strings.NewReader(zvReleaseUrl + ui.ContextMenuTarget.AlbumId)),
					})
				case 4:
					gtx.Execute(clipboard.WriteCmd{
						Data: io.NopCloser(strings.NewReader(youVideoUrl + ui.ContextMenuTarget.AlbumId)),
					})
				}
			}
			if ui.CopyAlbArtistBtn.Clicked(gtx) {
				switch ui.SiteId {
				case 1:
					var sb []string
					for _, artId := range ui.ContextMenuTarget.ParentId {
						sb = append(sb, zvArtistUrl+artId)
					}

					gtx.Execute(clipboard.WriteCmd{
						Data: io.NopCloser(strings.NewReader(strings.Join(sb, ", "))),
					})
				case 4:
					gtx.Execute(clipboard.WriteCmd{
						Data: io.NopCloser(strings.NewReader(youChannelUrl + ui.ContextMenuTarget.ParentId[0])),
					})
				}
			}
			if ui.DownloadBtn.Clicked(gtx) {
				active := ui.Rooms.Active()
				if active != nil {
					switch ui.SiteId {
					case 1:
						go active.DownloadAlbum(ui.SiteId, []string{ui.ContextMenuTarget.AlbumId}, ui.ZvukQuality)
					case 4:
						go active.DownloadAlbum(ui.SiteId, []string{active.Id + ";" + ui.ContextMenuTarget.AlbumId + ";" + ui.ContextMenuTarget.Title}, ui.YouQuality)
					}
				}
			}
			if elemState.Selected.Update(gtx) {
				active := ui.Rooms.Active()
				if active != nil {
					if elemState.Selected.Value {
						if !slices2.Contains(active.Selected, el.AlbumId) {
							active.Selected = append(active.Selected, el.AlbumId)
						}
					} else {
						for i, v := range active.Selected {
							if v == el.AlbumId {
								active.Selected = append(active.Selected[:i], active.Selected[i+1:]...)
								break
							}
						}
					}
				}
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
		TypeId:  data.TypeId,
		Type:    data.GetStringType(),
		SentAt:  data.SentAt,
		Avatar:  data.Avatar,
	})
	return msg.Layout
}

func findArtistId(url string, isArtist bool) string {
	var resId []string
	if isArtist {
		resId = regexp.MustCompile(zvArtistRegex).FindStringSubmatch(url)
	} else {
		resId = regexp.MustCompile(zvReleaseRegex).FindStringSubmatch(url)
	}
	if resId == nil {
		return ""
	}
	return resId[1]
}

func findYoutubeId(url string) string {
	resId := regexp.MustCompile(youVideoRegex).FindStringSubmatch(url)
	if resId == nil {
		resId = regexp.MustCompile(youChannelRegex).FindStringSubmatch(url)
		if len(resId) >= 3 {
			if strings.HasPrefix(resId[2], "channel") {
				return strings.Split(resId[2], "/")[1]
			}
			if strings.HasPrefix(resId[2], "user") {
				res := strings.Split(resId[2], "/")[1]
				if !strings.HasPrefix(res, "@") {
					res = "@" + res
				}
				return res
			}
			res := resId[2]
			if !strings.HasPrefix(res, "@") {
				res = "@" + res
			}
			return res
		}
		return ""
	}
	if len(resId) >= 2 {
		return resId[1]
	}
	return ""
}
