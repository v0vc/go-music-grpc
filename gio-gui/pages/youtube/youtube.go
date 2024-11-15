package youtube

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"

	"gioui.org/io/clipboard"
	"gioui.org/io/key"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
	"github.com/v0vc/go-music-grpc/gio-gui/ui"
)

type Page struct {
	// widget.List
	*page.Router
	addBtn, insertBtn, pasteBtn widget.Clickable
	editor                      widget.Editor
	th                          *page.Theme
	contextMenu                 component.MenuState
	contextArea                 component.ContextArea
}

const (
	siteId = 4
	newUrl = "url"
)

// New constructs a Page with the provided router.
func New(router *page.Router) *Page {
	return &Page{
		Router: router,
	}
}

func (p *Page) addActions() []component.AppBarAction {
	return []component.AppBarAction{
		{
			OverflowAction: component.OverflowAction{
				Name: "AddLink",
				Tag:  &p.editor,
			},
			Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
				thh := material.NewTheme()
				thh.Palette.Fg = p.th.Palette.BgSecondary
				p.editor.SingleLine = true
				p.editor.MaxLen = 128
				// p.editor.Focus()
				p.editor.InputHint = key.HintURL
				p.contextMenu = component.MenuState{
					Options: []func(gtx layout.Context) layout.Dimensions{
						func(gtx layout.Context) layout.Dimensions {
							item := component.MenuItem(p.th.Theme, &p.pasteBtn, "Paste")
							item.Icon = icon.PasteIcon
							return item.Layout(gtx)
						},
					},
				}
				if p.insertBtn.Clicked(gtx) {
					if p.editor.Text() != "" {
						go p.Router.AppBar.StopContextual(gtx.Now)
						go singleInstance.AddChannel(siteId, strings.TrimSpace(p.editor.Text()))
						p.editor.SetText(newUrl)
					}
				}
				if p.pasteBtn.Clicked(gtx) {
					p.editor.SetText("")
					gtx.Execute(clipboard.ReadCmd{
						Tag: &p.editor,
					})
				}
				return layout.Flex{
					Alignment: layout.Middle,
				}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{}.Layout(gtx,
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								return material.Editor(thh, &p.editor, newUrl).Layout(gtx)
							}),
							layout.Expanded(func(gtx layout.Context) layout.Dimensions {
								return p.contextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min = image.Point{}
									return component.Menu(p.th.Theme, &p.contextMenu).Layout(gtx)
								})
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.IconButton(thh, &p.insertBtn, icon.SaveIcon, "Save").Layout(gtx)
					}),
				)
			},
		},
	}
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{
		{
			OverflowAction: component.OverflowAction{
				Name: "Add",
				Tag:  &p.addBtn,
			},
			Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
				if p.addBtn.Clicked(gtx) {
					p.Router.AppBar.SetContextualActions(
						p.addActions(),
						[]component.OverflowAction{},
					)
					p.Router.AppBar.ToggleContextual(gtx.Now, "Add video link: ")
				}
				return component.SimpleIconButton(bg, fg, &p.addBtn, icon.PlusIcon).Layout(gtx)
			},
		},
	}
}

func (p *Page) Overflow() []component.OverflowAction {
	// return []component.OverflowAction{}
	return []component.OverflowAction{
		{
			Name: "Download",
			Tag:  0,
		},
		{
			Name: "Select All",
			Tag:  1,
		},
		{
			Name: "Unselect",
			Tag:  2,
		},
	}
}

func (p *Page) ClickMainMenu(event component.AppBarEvent) {
	res := strings.Split(fmt.Sprint(event), " ")
	switch res[len(res)-1] {
	case "0":
		if singleInstance != nil {
			go singleInstance.MassDownload(siteId)
		}
	case "1":
		if singleInstance != nil {
			go singleInstance.SelectAll(true)
		}
	case "2":
		if singleInstance != nil {
			go singleInstance.SelectAll(false)
		}
	}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Youtube",
		Icon: icon.YoutubeIcon,
	}
}

var singleInstance *ui.UI

var lock = &sync.Mutex{}

func getInstance(invalidator func(), th *page.Theme, loadSize int, quality string) *ui.UI {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		singleInstance = ui.NewUI(invalidator, th, loadSize, quality, siteId)
	}
	return singleInstance
}

func (p *Page) Layout(gtx layout.Context, th *page.Theme, loadSize int, quality string) layout.Dimensions {
	mainUi := getInstance(p.Router.Invalidate, th, loadSize, quality)
	p.th = th
	return mainUi.Layout(gtx)
}
