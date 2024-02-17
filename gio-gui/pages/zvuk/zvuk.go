package zvuk

import (
	"image/color"
	"sync"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
	lay "github.com/v0vc/go-music-grpc/gio-gui/layout"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
	"github.com/v0vc/go-music-grpc/gio-gui/ui"
)

type Page struct {
	// widget.List
	*page.Router
	addBtn, insertBtn widget.Clickable
	editor            widget.Editor
	th                *page.Theme
}

const siteId = 1

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
				gutter := lay.Gutter()
				gutter.RightWidth = gutter.RightWidth - unit.Dp(60)
				if p.insertBtn.Clicked(gtx) {
					if p.editor.Text() != "" {
						go p.Router.AppBar.StopContextual(gtx.Now)
						go singleInstance.AddChannel(siteId, p.editor.Text())
						p.editor.SetText("")
					}
				}
				return gutter.Layout(gtx,
					nil,
					func(gtx layout.Context) layout.Dimensions {
						p.editor.SingleLine = true
						p.editor.MaxLen = 128
						// p.editor.Focus()
						p.editor.InputHint = key.HintURL
						return material.Editor(thh, &p.editor, "url").Layout(gtx)
					},
					material.IconButton(thh, &p.insertBtn, icon.SaveIcon, "Save").Layout,
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
					p.Router.AppBar.ToggleContextual(gtx.Now, "Add artist link:")
				}
				return component.SimpleIconButton(bg, fg, &p.addBtn, icon.PlusIcon).Layout(gtx)
			},
		},
	}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
	/*return []component.OverflowAction{
		{
			Name: "Add",
			Tag:  &p.addBtn,
		},
		{
			Name: "Delete",
			Tag:  &p.removeState,
		},
	}*/
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Zvuk",
		Icon: icon.MusicIcon,
	}
}

var singleInstance *ui.UI

var lock = &sync.Mutex{}

func getInstance(invalidator func(), th *page.Theme, loadSize int) *ui.UI {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		singleInstance = ui.NewUI(invalidator, th, loadSize, siteId)
	}
	return singleInstance
}

func (p *Page) Layout(gtx layout.Context, th *page.Theme, loadSize int) layout.Dimensions {
	ui := getInstance(p.Router.Invalidate, th, loadSize)
	p.th = th
	return ui.Layout(gtx)
}
