package ui

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	lay "github.com/v0vc/go-music-grpc/gio-gui/layout"
)

// Row holds persistent state for a single row of a chat.
type Row struct {
	// ContextArea holds the clicks state for the right-click context menu.
	component.ContextArea
	// Image contains the cached image op for the message.
	Avatar CachedImage
	// Message
	widget.Clickable

	Active   bool
	Selected widget.Bool
}

type RowStyle struct {
	*Row
	Image     Image
	Name      material.LabelStyle
	Summary   material.LabelStyle
	TimeStamp material.LabelStyle
	Type      material.LabelStyle
	Selected  material.CheckBoxStyle
	Indicator color.NRGBA
	Overlay   color.NRGBA
	// Menu configures the right-click context menu for this message.
	Menu component.MenuStyle
}

type RowConfig struct {
	// Name of the room as raw text.
	Title string
	// Image of the room.
	Avatar image.Image
	// Content of the latest message as raw text.
	Content string
	Type    string
	// SentAt timestamp of the latest message.
	SentAt time.Time
}

func NewRow(th *material.Theme, interact *Row, menu *component.MenuState, msg *RowConfig) RowStyle {
	interact.Avatar.Cache(msg.Avatar)
	return RowStyle{
		Row:       interact,
		Name:      material.Label(th, unit.Sp(14), msg.Title),
		Summary:   material.Label(th, unit.Sp(12), msg.Content),
		Type:      material.Label(th, unit.Sp(12), msg.Type),
		TimeStamp: material.Label(th, unit.Sp(12), strconv.Itoa(msg.SentAt.Local().Year())),
		Selected:  material.CheckBox(th, &interact.Selected, ""),
		Image: Image{
			Image: widget.Image{
				Src: interact.Avatar.Op(),
				Fit: widget.Contain,
			},
			Radii:  unit.Dp(8),
			Height: unit.Dp(40),
			Width:  unit.Dp(40),
		},
		Indicator: th.ContrastBg,
		Overlay:   component.WithAlpha(th.Fg, 50),
		Menu:      component.Menu(th, menu),
	}
}

func (row RowStyle) Layout(gtx layout.Context) layout.Dimensions {
	var (
		surface = func(gtx layout.Context, w layout.Widget) layout.Dimensions { return w(gtx) }
		dims    layout.Dimensions
	)
	if row.Active {
		surface = lay.Background(row.Overlay).Layout

		defer func() {
			// Close-over the dimensions and layout the indicator atop everything
			// else.
			component.Rect{
				Size: image.Point{
					X: gtx.Dp(unit.Dp(3)),
					Y: dims.Size.Y,
				},
				Color: row.Indicator,
			}.Layout(gtx)
		}()
	}
	dims = surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &row.Clickable, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(
					gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return row.Image.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis: layout.Vertical,
						}.Layout(
							gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return row.Name.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return row.Summary.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return row.Type.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(15)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return row.TimeStamp.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return row.Selected.Layout(gtx)
					}),
				)
			})
		})
	})
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return dims
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return row.Row.ContextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return row.Menu.Layout(gtx)
			})
		}),
	)
	// return dims
}
