package page

import (
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
)

type Page interface {
	Actions() []component.AppBarAction
	Overflow() []component.OverflowAction
	Layout(gtx layout.Context, th *Theme, conf *Config) layout.Dimensions
	NavItem() component.NavItem
	ClickMainMenu(event component.AppBarEvent)
	HandleKeyboard(key.Name)
}

type Config struct {
	// theme to use {light,dark}.
	Theme string
	// loadSize specifies maximum number of items to load at a time.
	LoadSize int
	// {flac, high, mid}
	ZvukQuality string
	// yt-dlp params
	YouVideoQuality, YouVideoHqQuality, YouAudioQuality string
}

type Router struct {
	pages   map[any]Page
	current any
	*component.ModalNavDrawer
	NavAnim component.VisibilityAnimation
	*component.AppBar
	*component.ModalLayer
	NonModalDrawer, BottomBar bool
	*app.Window
}

func NewRouter(w *app.Window) Router {
	modal := component.NewModal()

	nav := component.NewNav("grpc-music", "v0.0.1")
	modalNav := component.ModalNavFrom(&nav, modal)

	bar := component.NewAppBar(modal)
	bar.NavigationIcon = icon.MenuIcon

	na := component.VisibilityAnimation{
		State:    component.Invisible,
		Duration: time.Millisecond * 250,
	}
	return Router{
		pages:          make(map[any]Page),
		current:        nil,
		ModalNavDrawer: modalNav,
		NavAnim:        na,
		AppBar:         bar,
		ModalLayer:     modal,
		NonModalDrawer: true,
		BottomBar:      false,
		Window:         w,
	}
}

var topLevelKeyFilters = []event.Filter{
	key.Filter{Name: "A", Required: key.ModCtrl},
	key.Filter{Name: key.NameUpArrow},
	key.Filter{Name: key.NameDownArrow},
	key.Filter{Name: key.NamePageUp},
	key.Filter{Name: key.NamePageDown},
	key.Filter{Name: key.NameHome},
	key.Filter{Name: key.NameEnd},
}

func (r *Router) Register(tag any, p Page) {
	r.pages[tag] = p
	navItem := p.NavItem()
	navItem.Tag = tag
	if r.current == any(nil) {
		r.current = tag
		r.Title = navItem.Name
		r.SetActions(p.Actions(), p.Overflow())
	}

	r.AddNavItem(navItem)
}

func (r *Router) SwitchTo(tag any) {
	p, ok := r.pages[tag]
	if !ok {
		return
	}
	r.current = tag
	r.Title = p.NavItem().Name
	r.SetActions(p.Actions(), p.Overflow())
}

func (r *Router) Layout(gtx layout.Context, th *Theme, conf *Config) layout.Dimensions {
	for {
		ev, ok := gtx.Event(topLevelKeyFilters...)
		if !ok {
			break
		}
		if ke, yes := ev.(key.Event); yes {
			r.handleKeyEvent(gtx, ke)
		}
	}
	for _, ev := range r.Events(gtx) {
		// switch event := event.(type) {
		switch ev.(type) {
		case component.AppBarNavigationClicked:
			if r.NonModalDrawer {
				r.NavAnim.ToggleVisibility(gtx.Now)
			} else {
				r.Appear(gtx.Now)
				r.NavAnim.Disappear(gtx.Now)
			}
		case component.AppBarContextMenuDismissed:
			// log.Printf("Context menu dismissed: %v", event)
		case component.AppBarOverflowActionClicked:
			// log.Printf("Overflow action selected: %v", event)
			// r.pages[r.current].Overflow()
			r.pages[r.current].ClickMainMenu(ev)
		}
	}
	if r.NavDestinationChanged() {
		r.SwitchTo(r.CurrentNavDestination())
	}
	// paint.Fill(gtx.Ops, th.Palette.Surface)
	content := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		origBg := th.Bg
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// gtx.Constraints.Max.X /= 3
				gtx.Constraints.Max.X = 375
				th.Bg = th.Palette.Surface

				return r.NavDrawer.Layout(gtx, th.Theme, &r.NavAnim)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				th.Bg = origBg
				return r.pages[r.current].Layout(gtx, th, conf)
			}),
		)
	})
	bar := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return r.AppBar.Layout(gtx, th.Theme, "Menu", "Actions")
	})
	flex := layout.Flex{Axis: layout.Vertical}
	if r.BottomBar {
		flex.Layout(gtx, content, bar)
	} else {
		flex.Layout(gtx, bar, content)
	}

	r.ModalLayer.Layout(gtx, th.Theme)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (r *Router) handleKeyEvent(gtx layout.Context, e key.Event) {
	if e.State != key.Press {
		return
	}
	r.pages[r.current].HandleKeyboard(e.Name)
}
