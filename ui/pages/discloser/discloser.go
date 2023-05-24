package discloser

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/ui/icon"
	page "github.com/v0vc/go-music-grpc/ui/pages"
)

// TreeNode is a simple tree implementation that holds both
// display data and the state for Discloser widgets. In
// practice, you'll often want to separate the state from
// the data being presented.
/*type TreeNode struct {
	Text     string
	Children []TreeNode
	component.DiscloserState
}*/

// Page holds the state for a page demonstrating the features of
// the AppBar component.
type Page struct {
	//TreeNode
	customDiscloserState component.DiscloserState
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
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Disclosers",
		Icon: icon.VisibilityIcon,
	}
}

// LayoutTreeNode recursively lays out a tree of widgets described by
// TreeNodes.
/*func (p *Page) LayoutTreeNode(gtx layout.Context, th *material.Theme, tn *TreeNode) layout.Dimensions {
	if len(tn.Children) == 0 {
		return layout.UniformInset(unit.Dp(2)).Layout(gtx,
			material.Body1(th, tn.Text).Layout)
	}
	children := make([]layout.FlexChild, 0, len(tn.Children))
	for i := range tn.Children {
		child := &tn.Children[i]
		children = append(children, layout.Rigid(
			func(gtx layout.Context) layout.Dimensions {
				return p.LayoutTreeNode(gtx, th, child)
			}))
	}
	return component.SimpleDiscloser(th, &tn.DiscloserState).Layout(gtx,
		material.Body1(th, tn.Text).Layout,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
}*/

// LayoutCustomDiscloser demonstrates how to create a custom control for
// a discloser.
func (p *Page) LayoutCustomDiscloser(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return component.Discloser(th, &p.customDiscloserState).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			var l material.LabelStyle
			l = material.Body1(th, "+")
			if p.customDiscloserState.Visible() {
				l.Text = "-"
			}
			l.Font.Variant = "Mono"
			return layout.UniformInset(unit.Dp(2)).Layout(gtx, l.Layout)
		},
		material.Body2(th, "Add artist").Layout,
		material.Body1(th, "This control only took 9 lines of code.").Layout,
	)
}

func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	p.List.Axis = layout.Vertical
	return material.List(th, &p.List).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			/*			if index == 0 {
						return p.LayoutTreeNode(gtx, th, &p.TreeNode)
					}*/
			return p.LayoutCustomDiscloser(gtx, th)
		})
	})
}
