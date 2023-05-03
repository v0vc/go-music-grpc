package main

import (
	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget/material"
	page "github.com/v0vc/go-music-grpc/ui/pages"
	"github.com/v0vc/go-music-grpc/ui/pages/about"
	"github.com/v0vc/go-music-grpc/ui/pages/appbar"
	"github.com/v0vc/go-music-grpc/ui/pages/discloser"
	"github.com/v0vc/go-music-grpc/ui/pages/menu"
	"github.com/v0vc/go-music-grpc/ui/pages/navdrawer"
	"github.com/v0vc/go-music-grpc/ui/pages/sber"
	"github.com/v0vc/go-music-grpc/ui/pages/textfield"
	"log"
	"os"
)

func main() {
	/*flag.Parse()*/
	go func() {
		w := app.NewWindow()
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops

	router := page.NewRouter()
	router.Register(0, appbar.New(&router))
	router.Register(1, navdrawer.New(&router))
	router.Register(2, textfield.New(&router))
	router.Register(3, menu.New(&router))
	router.Register(4, discloser.New(&router))
	router.Register(5, about.New(&router))
	router.Register(6, sber.New(&router))

	for {
		select {
		case e := <-w.Events():
			switch e := e.(type) {
			case system.DestroyEvent:
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				router.Layout(gtx, th)
				e.Frame(gtx.Ops)
			}
		}
	}
}
