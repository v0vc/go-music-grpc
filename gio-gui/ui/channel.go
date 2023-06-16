package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	lay "github.com/v0vc/go-music-grpc/gio-gui/layout"
)

// Channel selector state.
type Channel struct {
	widget.Clickable
	Image  CachedImage
	Active bool
}

type ChannelStyle struct {
	*Channel
	Image     Image
	Name      material.LabelStyle
	Summary   material.LabelStyle
	TimeStamp material.LabelStyle
	Indicator color.NRGBA
	Overlay   color.NRGBA
}

// ChannelConfig configures room item display.
type ChannelConfig struct {
	// Name of the room as raw text.
	Name string
	// Image of the room.
	Image image.Image
	// Content of the latest message as raw text.
	Content string
	// SentAt timestamp of the latest message.
	SentAt time.Time
}

// CreateChannel creates a style type that can lay out the data for a room.
func CreateChannel(th *material.Theme, interact *Channel, room *ChannelConfig) ChannelStyle {
	interact.Image.Cache(room.Image)
	return ChannelStyle{
		Channel:   interact,
		Name:      material.Label(th, unit.Sp(14), room.Name),
		Summary:   material.Label(th, unit.Sp(12), room.Content),
		TimeStamp: material.Label(th, unit.Sp(12), room.SentAt.Local().Format("15:04")),
		Image: Image{
			Image: widget.Image{
				Src: interact.Image.Op(),
				Fit: widget.Contain,
			},
			Radii:  unit.Dp(8),
			Height: unit.Dp(40),
			Width:  unit.Dp(40),
		},
		Indicator: th.ContrastBg,
		Overlay:   component.WithAlpha(th.Fg, 50),
	}
}

func (room ChannelStyle) Layout(gtx layout.Context) layout.Dimensions {
	var (
		surface = func(gtx layout.Context, w layout.Widget) layout.Dimensions { return w(gtx) }
		dims    layout.Dimensions
	)
	if room.Active {
		surface = lay.Background(room.Overlay).Layout
		defer func() {
			// Close-over the dimensions and layout the indicator atop everything
			// else.
			component.Rect{
				Size: image.Point{
					X: gtx.Dp(unit.Dp(3)),
					Y: dims.Size.Y,
				},
				Color: room.Indicator,
			}.Layout(gtx)
		}()
	}
	dims = surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &room.Clickable, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(
					gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return room.Image.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis: layout.Vertical,
						}.Layout(
							gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return room.Name.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return component.TruncatingLabelStyle(room.Summary).Layout(gtx)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return room.TimeStamp.Layout(gtx)
					}),
				)
			})
		})
	})
	return dims
}
