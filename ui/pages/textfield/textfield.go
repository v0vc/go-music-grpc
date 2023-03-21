package textfield

import (
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	alo "github.com/v0vc/go-music-grpc/ui/applayout"
	"github.com/v0vc/go-music-grpc/ui/icon"
	page "github.com/v0vc/go-music-grpc/ui/pages"
	"image/color"
	"unicode"
)

// Page holds the state for a page demonstrating the features of
// the TextField component.
type Page struct {
	inputAlignment                                               layout.Alignment
	inputAlignmentEnum                                           widget.Enum
	nameInput, addressInput, priceInput, tweetInput, numberInput component.TextField
	widget.List
	*page.Router
}

// New constructs a Page with the provided router.
func New(router *page.Router) *Page {
	return &Page{
		Router: router,
	}
}

/*var _ page.Page = &Page{}*/

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Text Field Features",
		Icon: icon.EditIcon,
	}
}

func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	p.List.Axis = layout.Vertical
	return material.List(th, &p.List).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(
			gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				p.nameInput.Alignment = p.inputAlignment
				return p.nameInput.Layout(gtx, th, "Name")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return alo.DefaultInset.Layout(gtx, material.Body2(th, "Responds to hover events.").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				p.addressInput.Alignment = p.inputAlignment
				return p.addressInput.Layout(gtx, th, "Address")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return alo.DefaultInset.Layout(gtx, material.Body2(th, "Label animates properly when you click to select the text field.").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				p.priceInput.Prefix = func(gtx layout.Context) layout.Dimensions {
					th := *th
					th.Palette.Fg = color.NRGBA{R: 100, G: 100, B: 100, A: 255}
					return material.Label(&th, th.TextSize, "$").Layout(gtx)
				}
				p.priceInput.Suffix = func(gtx layout.Context) layout.Dimensions {
					th := *th
					th.Palette.Fg = color.NRGBA{R: 100, G: 100, B: 100, A: 255}
					return material.Label(&th, th.TextSize, ".00").Layout(gtx)
				}
				p.priceInput.SingleLine = true
				p.priceInput.Alignment = p.inputAlignment
				return p.priceInput.Layout(gtx, th, "Price")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return alo.DefaultInset.Layout(gtx, material.Body2(th, "Can have prefix and suffix elements.").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if err := func() string {
					for _, r := range p.numberInput.Text() {
						if !unicode.IsDigit(r) {
							return "Must contain only digits"
						}
					}
					return ""
				}(); err != "" {
					p.numberInput.SetError(err)
				} else {
					p.numberInput.ClearError()
				}
				p.numberInput.SingleLine = true
				p.numberInput.Alignment = p.inputAlignment
				return p.numberInput.Layout(gtx, th, "Number")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return alo.DefaultInset.Layout(gtx, material.Body2(th, "Can be validated.").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.tweetInput.TextTooLong() {
					p.tweetInput.SetError("Too many characters")
				} else {
					p.tweetInput.ClearError()
				}
				p.tweetInput.CharLimit = 128
				p.tweetInput.Helper = "Tweets have a limited character count"
				p.tweetInput.Alignment = p.inputAlignment
				return p.tweetInput.Layout(gtx, th, "Tweet")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return alo.DefaultInset.Layout(gtx, material.Body2(th, "Can have a character counter and help text.").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.inputAlignmentEnum.Changed() {
					switch p.inputAlignmentEnum.Value {
					case layout.Start.String():
						p.inputAlignment = layout.Start
					case layout.Middle.String():
						p.inputAlignment = layout.Middle
					case layout.End.String():
						p.inputAlignment = layout.End
					default:
						p.inputAlignment = layout.Start
					}
					op.InvalidateOp{}.Add(gtx.Ops)
				}
				return alo.DefaultInset.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis: layout.Vertical,
						}.Layout(
							gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Body2(th, "Text Alignment").Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{
									Axis: layout.Vertical,
								}.Layout(
									gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return material.RadioButton(
											th,
											&p.inputAlignmentEnum,
											layout.Start.String(),
											"Start",
										).Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return material.RadioButton(
											th,
											&p.inputAlignmentEnum,
											layout.Middle.String(),
											"Middle",
										).Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return material.RadioButton(
											th,
											&p.inputAlignmentEnum,
											layout.End.String(),
											"End",
										).Layout(gtx)
									}),
								)
							}),
						)
					},
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return alo.DefaultInset.Layout(gtx, material.Body2(th, "This text field implementation was contributed by Jack Mordaunt. Thanks Jack!").Layout)
			}),
		)
	})
}
