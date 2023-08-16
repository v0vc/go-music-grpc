package zvuk

import (
	"sync"

	"gioui.org/layout"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
	"github.com/v0vc/go-music-grpc/gio-gui/ui"
)

type Page struct {
	// widget.List
	*page.Router
}

// New constructs a Page with the provided router.
func New(router *page.Router) *Page {
	return &Page{
		Router: router,
	}
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Zvuk",
		Icon: icon.MusicIcon,
	}
}

var singleInstance *ui.UI

var lock = &sync.Mutex{}

func getInstance(invalidator func(), th *page.Theme) *ui.UI {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		singleInstance = ui.NewUI(invalidator, th)
	}
	return singleInstance
}

func (p *Page) Layout(gtx layout.Context, th *page.Theme) layout.Dimensions {
	ui := getInstance(p.Router.Invalidate, th)
	return ui.Layout(gtx)
}
