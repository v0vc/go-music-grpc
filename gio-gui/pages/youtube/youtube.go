package youtube

import (
	"flag"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget/material"
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
		Name: "YouTube",
		Icon: icon.VisibilityIcon,
	}
}

var singleInstance *ui.UI

var lock = &sync.Mutex{}

func getInstance(invalidator func()) *ui.UI {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		singleInstance = ui.NewUI(invalidator, config)
	}
	return singleInstance
}

var config ui.Config

func init() {
	flag.StringVar(&config.Theme, "theme", "light", "theme to use {light,dark}")
	// flag.IntVar(&config.Latency, "latency", 1000, "maximum latency (in millis) to simulate")
	flag.IntVar(&config.LoadSize, "load-size", 30, "number of items to load at a time")
	// flag.IntVar(&config.BufferSize, "buffer-size", 30, "number of elements to hold in memory before compacting")

	flag.Parse()
}

func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	ui := getInstance(p.Router.Invalidate)
	return ui.Layout(gtx)
}
