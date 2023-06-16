package appbar

import (
	"image/color"

	"gioui.org/unit"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
	lay "github.com/v0vc/go-music-grpc/gio-gui/layout"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
)

// Page holds the state for a page demonstrating the features of
// the AppBar component.
type Page struct {
	heartBtn, plusBtn, contextBtn          widget.Clickable
	exampleOverflowState, red, green, blue widget.Clickable
	bottomBar, customNavIcon               widget.Bool
	favorite                               bool
	widget.List
	*page.Router
}

// New constructs a Page with the provided router.
func New(router *page.Router) *Page {
	return &Page{
		Router: router,
	}
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{
		{
			OverflowAction: component.OverflowAction{
				Name: "Favorite",
				Tag:  &p.heartBtn,
			},
			Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
				if p.heartBtn.Clicked() {
					p.favorite = !p.favorite
				}
				btn := component.SimpleIconButton(bg, fg, &p.heartBtn, icon.HeartIcon)
				btn.Background = bg
				if p.favorite {
					btn.Color = color.NRGBA{R: 200, A: 255}
				} else {
					btn.Color = fg
				}
				return btn.Layout(gtx)
			},
		},
		component.SimpleIconAction(&p.plusBtn, icon.PlusIcon,
			component.OverflowAction{
				Name: "Create",
				Tag:  &p.plusBtn,
			},
		),
	}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{
		{
			Name: "Example 1",
			Tag:  &p.exampleOverflowState,
		},
		{
			Name: "Example 2",
			Tag:  &p.exampleOverflowState,
		},
	}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "App Bar Features",
		Icon: icon.HomeIcon,
	}
}

func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	p.List.Axis = layout.Vertical
	return material.List(th, &p.List).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{
			Alignment: layout.Middle,
			Axis:      layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Body1(th, `The app bar widget provides a consistent interface element for triggering navigation`).Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return lay.DetailRow{}.Layout(gtx, material.Body1(th, "Contextual App Bar").Layout, func(gtx layout.Context) layout.Dimensions {
					if p.contextBtn.Clicked() {
						p.Router.AppBar.SetContextualActions(
							[]component.AppBarAction{
								component.SimpleIconAction(&p.red, icon.HeartIcon,
									component.OverflowAction{
										Name: "House",
										Tag:  &p.red,
									},
								),
							},
							[]component.OverflowAction{
								{
									Name: "foo",
									Tag:  &p.blue,
								},
								{
									Name: "bar",
									Tag:  &p.green,
								},
							},
						)
						p.Router.AppBar.ToggleContextual(gtx.Now, "Contextual Title")
					}
					return material.Button(th, &p.contextBtn, "Trigger").Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return lay.DetailRow{}.Layout(gtx,
					material.Body1(th, "Bottom App Bar").Layout,
					func(gtx layout.Context) layout.Dimensions {
						if p.bottomBar.Changed() {
							if p.bottomBar.Value {
								p.Router.ModalNavDrawer.Anchor = component.Bottom
								p.Router.AppBar.Anchor = component.Bottom
							} else {
								p.Router.ModalNavDrawer.Anchor = component.Top
								p.Router.AppBar.Anchor = component.Top
							}
							p.Router.BottomBar = p.bottomBar.Value
						}

						return material.Switch(th, &p.bottomBar, "Use Bottom App Bar").Layout(gtx)
					})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return lay.DetailRow{}.Layout(gtx,
					material.Body1(th, "Custom Navigation Icon").Layout,
					func(gtx layout.Context) layout.Dimensions {
						if p.customNavIcon.Changed() {
							if p.customNavIcon.Value {
								p.Router.AppBar.NavigationIcon = icon.HomeIcon
							} else {
								p.Router.AppBar.NavigationIcon = icon.MenuIcon
							}
						}
						return material.Switch(th, &p.customNavIcon, "Use Custom Navigation Icon").Layout(gtx)
					})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return lay.DetailRow{}.Layout(gtx,
					material.Body1(th, "Animated Resize").Layout,
					material.Body2(th, "Resize the width of your screen to see app bar actions collapse into or emerge from the overflow menu (as size permits).").Layout,
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return lay.DetailRow{}.Layout(gtx,
					material.Body1(th, "Custom Action Buttons").Layout,
					material.Body2(th, "Click the heart action to see custom button behavior.").Layout)
			}),
		)
	})
}
