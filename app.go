package gowaku

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// tickWhileHidden is the pace of tickers in a minimized window.
const tickWhileHidden = 16 * time.Millisecond

// currentApp is the running application, for Post. See the package comment.
var currentApp atomic.Pointer[app]

// app is the state of Run. Everything but the post queue belongs to the UI
// goroutine.
type app struct {
	platform platform.Platform
	backend  gpu.Backend
	windows  map[platform.WindowID]*window
	// closing holds the windows that reap will close.
	closing []*window
	// shaper and glyphs serve every window.
	shaper *text.Shaper
	glyphs *render.Resources

	themes     theme.Set
	mode       ThemeMode
	brightness platform.Brightness
	// reduceMotion is what the OS says about animation.
	reduceMotion bool
	// locale is the locale of the windows: fixedLocale when the application
	// gives one, else what the OS says. strings is App.Strings.
	locale      language.Tag
	fixedLocale bool
	strings     []*basic.Strings
	// appID is App.AppID, which every window gets.
	appID string

	// start is the zero of the time that tickers see.
	start time.Time

	// trays are the icons of ShowTray, which go when Run ends.
	trays []*appTray

	// inspector serves the Inspector, with -tags gowakudebug only.
	inspector inspectorState

	mu    sync.Mutex
	posts []func()
	// running is the half of the double buffer that the UI goroutine drains.
	running []func()
}

// run is the event loop (see docs/ARCHITECTURE.md). It knows the platform and GPU only by
// their interfaces, so that tests can drive it with fakes.
func run(p platform.Platform, b gpu.Backend, shaper *text.Shaper, a App) error {
	return runWith(p, b, shaper, a, nil)
}

// runWith is run with the window of a already made by start, or made here if
// first is nil.
func runWith(p platform.Platform, b gpu.Backend, shaper *text.Shaper, a App, first *firstWindow) error {
	ap := &app{
		platform: p,
		backend:  b,
		windows:  make(map[platform.WindowID]*window),
		shaper:   shaper,
		glyphs:   render.NewResources(text.NewAtlas(shaper), b),
		themes:   a.Theme,
		mode:     a.ThemeMode,
		start:    time.Now(),
		locale:   a.Locale,
		strings:  a.Strings,
		appID:    a.AppID,
	}
	ap.brightness = p.Brightness()
	ap.reduceMotion = p.ReduceMotion()
	ap.fixedLocale = a.Locale != language.Und
	if !ap.fixedLocale {
		ap.locale = parseLocale(p.Locale())
	}
	defer ap.glyphs.Destroy()
	// Before the first window: its tree mounts on creation, and InitState of Home
	// may already call Post.
	currentApp.Store(ap)
	defer currentApp.Store(nil)
	opts := a.windowOptions()
	if first != nil {
		ap.attach(opts, a.Home, nil, first.native, first.surface, first.width, first.height, first.scale)
	} else if _, err := ap.open(opts, a.Home); err != nil {
		return err
	}

	if inspecting {
		ap.inspectStart()
	}
	p.SetModalHandler(ap.pump)
	// The first frame must not depend on the OS asking for one.
	ap.pump()
	// The fonts of the first frame load in parallel at the next start.
	shaper.SaveStartupFaces()
	for len(ap.windows) > 0 {
		// Nothing wakes the loop but events, so an idle application sleeps
		// here and neither CPU nor GPU does any work. While something animates,
		// presenting a frame is what paces the loop.
		ap.handle(p.WaitEvent(ap.waitTime()))
		ap.pump()
	}
	p.SetModalHandler(nil)
	for len(ap.trays) > 0 {
		ap.trays[len(ap.trays)-1].Close()
	}
	return nil
}

// waitTime returns how long the loop may sleep.
func (ap *app) waitTime() time.Duration {
	wait := -1 * time.Second
	sooner := func(d time.Duration) {
		if wait < 0 || d < wait {
			wait = d
		}
	}
	now := time.Since(ap.start)
	for _, w := range ap.windows {
		if w.tree.Due(now) {
			if !w.minimized {
				return 0
			}
			// Nothing presents, so a minimized animation needs a pace of its own.
			sooner(tickWhileHidden)
		} else if wake, ok := w.tree.NextWake(); ok {
			sooner(wake - now)
		}
	}
	if inspecting {
		if d, ok := ap.inspectWake(); ok {
			sooner(d)
		}
	}
	return wait
}

// theme returns the theme of the moment.
func (ap *app) theme() *theme.Theme { return ap.themes.For(ap.effectiveBrightness()) }

// effectiveBrightness applies the mode of the application to that of the OS.
func (ap *app) effectiveBrightness() theme.Brightness {
	switch ap.mode {
	case ThemeLight:
		return theme.BrightnessLight
	case ThemeDark:
		return theme.BrightnessDark
	}
	return osBrightness(ap.brightness)
}

func osBrightness(b platform.Brightness) theme.Brightness {
	if b == platform.BrightnessDark {
		return theme.BrightnessDark
	}
	return theme.BrightnessLight
}

// followOS picks up a change of the mode of the OS.
func (ap *app) followOS() {
	b := ap.platform.Brightness()
	if b == ap.brightness {
		return
	}
	ap.brightness = b
	for _, w := range ap.windows {
		w.applyTheme()
		w.root.refresh()
	}
}

// followSettings picks up a change of the OS settings other than brightness.
func (ap *app) followSettings() {
	r := ap.platform.ReduceMotion()
	locale := ap.locale
	if !ap.fixedLocale {
		locale = parseLocale(ap.platform.Locale())
	}
	if r == ap.reduceMotion && locale == ap.locale {
		return
	}
	ap.reduceMotion, ap.locale = r, locale
	for _, w := range ap.windows {
		w.root.refresh()
	}
}

// parseLocale reads the BCP 47 tag of the platform. What it cannot read is the
// root locale, which the widgets read as English.
func parseLocale(s string) language.Tag {
	tag, err := language.Parse(s)
	if err != nil {
		return language.Und
	}
	return tag
}

// pump handles everything pending and draws what changed. It is the body of the
// event loop, and also runs inside OS modal loops. See
// docs/ADR/0001-win32-modal-loop.md.
func (ap *app) pump() {
	for {
		ev := ap.platform.WaitEvent(0)
		if ev.Kind == platform.EventNone {
			break
		}
		ap.handle(ev)
	}
	ap.runPosts()
	ap.reap()
	now := time.Since(ap.start)
	for _, w := range ap.windows {
		if w.tree.Due(now) {
			w.tree.Tick(now)
		}
		if w.tree.NeedsFrame() && !w.minimized {
			ap.frame(w)
		}
	}
	if inspecting {
		ap.inspectPumped()
	}
	// A ticker or a frame may have asked as well.
	ap.reap()
}

func (ap *app) post(fn func()) {
	ap.mu.Lock()
	ap.posts = append(ap.posts, fn)
	ap.mu.Unlock()
	ap.platform.Wake()
}

func (ap *app) runPosts() {
	ap.mu.Lock()
	ap.posts, ap.running = ap.running[:0], ap.posts
	ap.mu.Unlock()
	for i, fn := range ap.running {
		ap.running[i] = nil
		fn()
	}
}

// Post schedules fn to run on the UI goroutine, which is the only place where
// SetState may be called. It is safe to call from any goroutine. Outside Run,
// fn is dropped, so that a worker can outlive the application. Work for one
// State goes through core.StateBase.Poster instead, which drops it once the
// State has left the tree and works under gowakutest as well. To run code
// on the UI goroutine after a delay, as to hide a message, a State uses
// core.StateBase.After instead, which follows the clock of the window and of
// gowakutest.
func Post(fn func()) {
	if ap := currentApp.Load(); ap != nil {
		ap.post(fn)
	}
}

// Quit closes every window, which ends Run. It asks no window: OnCloseRequest
// does not run, but OnClose does. An application that asks about unsaved work
// before it quits does so first, then calls Quit. It is safe to call from any
// goroutine.
func Quit() {
	Post(func() {
		ap := currentApp.Load()
		for _, w := range ap.windows {
			ap.requestClose(w)
		}
	})
}
