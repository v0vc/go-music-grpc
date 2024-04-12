package main

import (
	"log"
	"os"
	"strconv"

	"gioui.org/app"
	"gioui.org/op"
	"github.com/joho/godotenv"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
	"github.com/v0vc/go-music-grpc/gio-gui/pages/zvuk"
)

const (
	defaultLoadSize = 30
	defaultTheme    = "light"
)

func main() {
	/*flag.Parse()*/
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Gio CMT v1.0.0"),
		)
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
	var ops op.Ops

	conf := parseConf()
	th := page.NewTheme(conf)
	router := page.NewRouter(w)
	router.Register(0, zvuk.New(&router))
	// router.Register(1, sber.New(&router))
	// router.Register(2, spotify.New(&router))
	// router.Register(3, deezer.New(&router))
	// router.Register(4, rutracker.New(&router))*/
	// router.Register(5, textfield.New(&router))
	// router.Register(6, appbar.New(&router))

	for {
		// detect the type of the event.
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			router.Layout(gtx, th, conf.LoadSize)
			e.Frame(gtx.Ops)
		}
	}
}
