package main

import (
	"log"
	"os"
	"strconv"

	"gioui.org/io/system"

	"github.com/v0vc/go-music-grpc/gio-gui/pages/youtube"

	"gioui.org/app"
	"gioui.org/op"
	"github.com/joho/godotenv"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
	"github.com/v0vc/go-music-grpc/gio-gui/pages/zvuk"
)

const (
	defaultLoadSize          = 30
	defaultTheme             = "light"
	defaultZvukQuality       = "mid"
	defaultYouVideoQuality   = "best"
	defaultYouVideoHqQuality = "bestvideo+bestaudio"
	defaultYouAudioQuality   = "bestaudio"
)

func main() {
	/*flag.Parse()*/
	go func() {
		w := new(app.Window)
		w.Perform(system.ActionCenter)
		/*w.Option(
			app.Title("Gio CMT v0.0.1"),
		)*/
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func parseConf() *page.Config {
	conf := page.Config{Theme: defaultTheme, LoadSize: defaultLoadSize, ZvukQuality: defaultZvukQuality}
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("no .env file, use default values")
	} else {
		loadSize := os.Getenv("LoadSize")
		if loadSize != "" {
			loadSizeInt, er := strconv.Atoi(loadSize)
			if er == nil {
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
		zvukQuality := os.Getenv("ZvukQuality")
		if zvukQuality != "" {
			conf.ZvukQuality = zvukQuality
		} else {
			conf.ZvukQuality = defaultZvukQuality
		}
		youVideoQuality := os.Getenv("YouVideoQuality")
		if youVideoQuality != "" {
			conf.YouVideoQuality = youVideoQuality
		} else {
			conf.YouVideoQuality = defaultYouVideoQuality
		}
		youVideoHqQuality := os.Getenv("YouVideoHqQuality")
		if youVideoHqQuality != "" {
			conf.YouVideoHqQuality = youVideoHqQuality
		} else {
			conf.YouVideoHqQuality = defaultYouVideoHqQuality
		}
		youAudioQuality := os.Getenv("YouAudioQuality")
		if youAudioQuality != "" {
			conf.YouAudioQuality = youAudioQuality
		} else {
			conf.YouAudioQuality = defaultYouAudioQuality
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
	router.Register(1, youtube.New(&router))

	for {
		// detect the type of the event.
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			router.Layout(gtx, th, conf)
			e.Frame(gtx.Ops)
		}
	}
}
