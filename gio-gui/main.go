package main

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
	"github.com/v0vc/go-music-grpc/gio-gui/pages/sber"
	"github.com/v0vc/go-music-grpc/gio-gui/pages/zvuk"
)

const (
	defaultLoadSize = 30
	defaultTheme    = "light"
)

func main() {
	/*flag.Parse()*/
	go func() {
		w := app.NewWindow( /*app.Title("Gio content client")*/ )
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func parseConf() *page.Config {
	conf := page.Config{Theme: defaultTheme, LoadSize: defaultLoadSize}
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("no .env file, use default values")
	} else {
		loadSize := os.Getenv("LoadSize")
		if loadSize != "" {
			loadSizeInt, err := strconv.Atoi(loadSize)
			if err == nil {
				conf.LoadSize = loadSizeInt
			} else {
				conf.LoadSize = defaultLoadSize
			}
		} else {
			conf.LoadSize = defaultLoadSize
		}
		themeEnv := os.Getenv("Theme")
		if themeEnv != "" {
			conf.Theme = themeEnv
		} else {
			conf.Theme = defaultTheme
		}
	}
	return &conf
}

func loop(w *app.Window) error {
	conf := parseConf()
	th := page.NewTheme(conf)
	var ops op.Ops

	router := page.NewRouter(w)
	sb := sber.New(&router)

	router.Register(0, zvuk.New(&router))
	/*router.Register(1, sb)
	router.Register(2, spotify.New(&router))
	router.Register(3, deezer.New(&router))
	router.Register(4, rutracker.New(&router))*/
	// router.Register(5, textfield.New(&router))
	// router.Register(6, appbar.New(&router))

	for {
		select {
		case e := <-w.Events():
			switch e := e.(type) {
			case system.DestroyEvent:
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				router.Layout(gtx, th, conf.LoadSize)
				e.Frame(gtx.Ops)
			}
		case pr := <-sb.ProgressIncrementer:
			if sb.Progress < 1 {
				sb.Progress += pr
				w.Invalidate()
			}
		}
	}
}
