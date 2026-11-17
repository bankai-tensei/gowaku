// Command notes is a notebook application: a
// folder of plain text notes with search, tabs and windows, vertical and
// horizontal writing, ruby in the notation of Aozora Bunko, autosave, menus,
// a command palette and file drops.
//
//	notes                      opens the windows of the last session, or the notebook
//	notes -config DIR          keeps the settings and the session in DIR, as tests do
//	notes -demo                runs on a sample notebook in memory, saving nothing
//	notes -quit-after 5s       quits on its own
//	notes -shot out.png -scene palette [-dark]
//	                           draws a scene of the sample notebook and exits;
//	                           the scenes are notebook, search, editor, vertical,
//	                           find, palette, settings and prompt
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/examples/notes/store"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/theme"
)

func main() {
	config := flag.String("config", "", "the folder of the settings and the session; empty uses the usual one")
	demo := flag.Bool("demo", false, "run on a sample notebook in memory")
	shotPath := flag.String("shot", "", "draw -scene to this PNG file and exit")
	scene := flag.String("scene", "editor", "the scene of -shot")
	dark := flag.Bool("dark", false, "the dark theme (-shot only; windows follow the OS)")
	vk := flag.Bool("vulkan", false, "draw -shot with Vulkan instead of D3D11 (Windows)")
	quitAfter := flag.Duration("quit-after", 0, "quit after this long; 0 waits for the windows to close")
	flag.Parse()
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, gowaku.Quit)
	}

	var err error
	switch {
	case *shotPath != "":
		err = screenshot(*shotPath, *scene, *dark, *vk)
	case *demo:
		a, _, _ := newDemoApp()
		a.shell = gowakuShell{}
		a.now = time.Now
		err = run(a)
	default:
		var a *app
		a, err = openApp(*config)
		if err == nil {
			err = run(a)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}

// openApp opens the store in dir, or in the usual place.
func openApp(dir string) (*app, error) {
	if dir == "" {
		var err error
		if dir, err = store.DefaultDir(); err != nil {
			base, err := os.UserConfigDir()
			if err != nil {
				return nil, fmt.Errorf("notes: no folder for the settings: %w", err)
			}
			dir = filepath.Join(base, "gowaku-notes")
		}
	}
	st, err := store.Open(dir)
	if err != nil {
		return nil, err
	}
	d := disk{read: store.ReadDoc, write: store.WriteDoc}
	return newApp(st, d, gowakuShell{}, func(dir string) notebook { return store.Library{Dir: dir} }), nil
}

// run runs the windows of a until the last closes.
func run(a *app) error {
	first := a.restore()
	go func() {
		for range time.Tick(time.Second) {
			gowaku.Post(a.tick)
		}
	}()
	return gowaku.Run(gowaku.App{
		Title: first.windowTitle(), Width: windowWidth, Height: windowHeight,
		// The words of the library follow appLocale, not the OS.
		Locale:  appLocale,
		Home:    windowView{model: first},
		OnClose: func() { a.windowClosed(first) },
	})
}

// screenshot draws a scene of the sample notebook to path.
func screenshot(path, scene string, dark, vulkan bool) error {
	brightness := theme.BrightnessLight
	if dark {
		brightness = theme.BrightnessDark
	}
	a, sh, _ := newDemoApp()
	w := a.restore()
	h, err := shot.Open(windowView{model: w}, shot.Options{
		Width: windowWidth, Height: windowHeight, Theme: theme.Set{}.For(brightness), Vulkan: vulkan,
	})
	if err != nil {
		return err
	}
	defer h.Close()
	var frameErr error
	settle := func() {
		for start := time.Now(); time.Since(start) < 700*time.Millisecond; {
			sh.flush()
			if _, err := h.Frame(); err != nil && frameErr == nil {
				frameErr = err
			}
			h.Pump()
		}
	}
	if err := (stage{app: a, win: w, settle: settle}).play(scene); err != nil {
		return err
	}
	if frameErr != nil {
		return frameErr
	}
	return h.Screenshot(path)
}
