package ui

import (
	"image"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
)

func BenchmarkKitchen(b *testing.B) {
	const scale = 2
	sz := image.Point{X: 800 * scale, Y: 600 * scale}
	w, err := headless.NewWindow(sz.X, sz.Y)
	if err != nil {
		b.Error(err)
	}
	ui := NewUI(func() {}, page.NewTheme(&page.Config{Theme: "light", LoadSize: 10, ZvukQuality: "mid"}), 0, "mid", 0)
	gtx := layout.Context{
		Ops: new(op.Ops),
		Metric: unit.Metric{
			PxPerDp: scale,
			PxPerSp: scale,
		},
		Constraints: layout.Exact(sz),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gtx.Ops.Reset()
		ui.Layout(gtx)
		er := w.Frame(gtx.Ops)
		if er != nil {
			return
		}
	}
}
