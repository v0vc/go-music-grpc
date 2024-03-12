package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// CachedImage is a cacheable image operation.
type CachedImage struct {
	op paint.ImageOp
	ch bool
}

// Reload tells the CachedImage to repopulate the cache.
func (img *CachedImage) Reload() {
	img.ch = true
}

// Cache the image if it is not already.
// First call will compute the image operation, subsequent calls will noop.
// When reloaded, cache will re-populated on next invocation.
func (img *CachedImage) Cache(src image.Image) *CachedImage {
	if img == nil || src == nil {
		return img
	}
	if img.op == (paint.ImageOp{}) || img.changed() {
		img.op = paint.NewImageOp(src)
	}
	return img
}

// Op returns the concrete image operation.
func (img CachedImage) Op() paint.ImageOp {
	return img.op
}

// changed reports whether the underlying image has changed and therefore
// should be cached again.
func (img *CachedImage) changed() bool {
	defer func() { img.ch = false }()
	return img.ch
}

// Image lays out an image with optionally rounded corners.
type Image struct {
	widget.Image
	widget.Clickable
	// Radii specifies the amount of rounding.
	Radii unit.Dp
	// Width and Height specify respective dimensions.
	// If left empty, dimensions will be unconstrained.
	Width, Height unit.Dp
}

// Layout the image.
func (img Image) Layout(gtx layout.Context) layout.Dimensions {
	if img.Width > 0 {
		gtx.Constraints.Max.X = gtx.Constraints.Constrain(image.Pt(gtx.Dp(img.Width), 0)).X
	}
	if img.Height > 0 {
		gtx.Constraints.Max.Y = gtx.Constraints.Constrain(image.Pt(0, gtx.Dp(img.Height))).Y
	}
	if img.Image.Src == (paint.ImageOp{}) {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	macro := op.Record(gtx.Ops)
	dims := img.Image.Layout(gtx)
	call := macro.Stop()
	r := gtx.Dp(img.Radii)
	defer clip.RRect{
		Rect: image.Rectangle{Max: dims.Size},
		NE:   r, NW: r, SE: r, SW: r,
	}.Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}
