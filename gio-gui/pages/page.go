package page

import (
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
)

type Page interface {
	Actions() []component.AppBarAction
	Overflow() []component.OverflowAction
	Layout(gtx layout.Context, th *Theme, loadSize int, quality string) layout.Dimensions
	NavItem() component.NavItem
	ClickMainMenu(event component.AppBarEvent)
}

type Config struct {
	// theme to use {light,dark}.
	Theme string
	// loadSize specifies maximum number of items to load at a time.
	LoadSize int
	Quality  string
}

type Router struct {
	pages   map[interface{}]Page
	current interface{}
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
		pages:          make(map[interface{}]Page),
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

func (r *Router) Register(tag interface{}, p Page) {
	r.pages[tag] = p
	navItem := p.NavItem()
	navItem.Tag = tag
	if r.current == interface{}(nil) {
		r.current = tag
		r.AppBar.Title = navItem.Name
		r.AppBar.SetActions(p.Actions(), p.Overflow())
	}

	r.ModalNavDrawer.AddNavItem(navItem)
}

func (r *Router) SwitchTo(tag interface{}) {
	p, ok := r.pages[tag]
	if !ok {
		return
	}
	r.current = tag
	r.AppBar.Title = p.NavItem().Name
	r.AppBar.SetActions(p.Actions(), p.Overflow())
}

func (r *Router) Layout(gtx layout.Context, th *Theme, loadSize int, quality string) layout.Dimensions {
	for _, event := range r.AppBar.Events(gtx) {
		// switch event := event.(type) {
		switch event.(type) {
		case component.AppBarNavigationClicked:
			if r.NonModalDrawer {
				r.NavAnim.ToggleVisibility(gtx.Now)
			} else {
				r.ModalNavDrawer.Appear(gtx.Now)
				r.NavAnim.Disappear(gtx.Now)
			}
		case component.AppBarContextMenuDismissed:
			// log.Printf("Context menu dismissed: %v", event)
		case component.AppBarOverflowActionClicked:
			// log.Printf("Overflow action selected: %v", event)
			// r.pages[r.current].Overflow()
			r.pages[r.current].ClickMainMenu(event)
		}
	}
	if r.ModalNavDrawer.NavDestinationChanged() {
		r.SwitchTo(r.ModalNavDrawer.CurrentNavDestination())
	}
	// paint.Fill(gtx.Ops, th.Palette.Surface)
	content := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		origBg := th.Theme.Bg
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// gtx.Constraints.Max.X /= 3
				gtx.Constraints.Max.X = 375
				th.Theme.Bg = th.Palette.Surface

				return r.NavDrawer.Layout(gtx, th.Theme, &r.NavAnim)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				th.Theme.Bg = origBg
				return r.pages[r.current].Layout(gtx, th, loadSize, quality)
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
