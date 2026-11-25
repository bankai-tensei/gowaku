//go:build darwin && arm64

package cocoa

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/evqueue"
)

// classSerial numbers the classes of each Platform. Their methods are closures
// over the Platform, as the window procedure of win32 is, and the runtime
// wants a new name for each set. It is the one mutable variable here.
var classSerial atomic.Uint32

// Platform is the AppKit implementation of platform.Platform.
type Platform struct {
	libs *libs
	app  objc.ID

	viewClass     objc.Class
	delegateClass objc.Class
	appDelegate   objc.ID

	windows  map[platform.WindowID]*Window
	byView   map[objc.ID]*Window
	byWindow map[objc.ID]*Window
	lastID   platform.WindowID
	queue    evqueue.Queue

	// woken is set by Wake, and wakePending while a wake event is posted.
	woken       atomic.Bool
	wakePending atomic.Bool
	wakeMu      sync.Mutex
	destroyed   bool

	modalHandler func()
	// inHandler is set while the modal handler runs, and dispatching while
	// AppKit is inside sendEvent, as in a live resize: WaitEvent must not pump
	// events of its own there.
	inHandler   bool
	dispatching int
	liveResize  int

	brightness   platform.Brightness
	reduceMotion bool

	// pending collects what interpretKeyEvents produces, so that the key
	// event goes in the queue first.
	pending     []platform.Event
	interpreted bool

	// menuOwner is the window whose menus the menu bar shows.
	menuOwner *Window

	// trays are the status items, numbered from lastTray, and notifications
	// the center of UserNotifications, if the application has a bundle.
	trays           []*tray
	lastTray        int
	notifications   objc.ID
	noNotifications bool
	notify          notifyState
}

var _ platform.Platform = (*Platform)(nil)

// New starts AppKit. Call it on the main goroutine.
func New() (*Platform, error) {
	l, err := loadLibs()
	if err != nil {
		return nil, err
	}
	p := &Platform{
		libs:     l,
		windows:  make(map[platform.WindowID]*Window),
		byView:   make(map[objc.ID]*Window),
		byWindow: make(map[objc.ID]*Window),
	}
	l.pool(func() { err = p.init() })
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Platform) init() error {
	p.app = class("NSApplication").Send(sel("sharedApplication"))
	if p.app == 0 {
		return errors.New("cocoa: no NSApplication")
	}
	if err := p.registerClasses(); err != nil {
		return err
	}
	p.appDelegate = objc.ID(p.delegateClass).Send(sel("new"))
	p.app.Send(sel("setDelegate:"), p.appDelegate)
	p.app.Send(sel("setActivationPolicy:"), int64(NSApplicationActivationPolicyRegular))
	p.app.Send(sel("setMainMenu:"), p.mainMenu(nil))
	// Running the application once finishes launching as AppKit expects,
	// with the Apple Events of the system wired; applicationDidFinishLaunching
	// stops it again at once. The event loop is WaitEvent from then on.
	running := class("NSRunningApplication").Send(sel("currentApplication"))
	if !objc.Send[bool](running, sel("isFinishedLaunching")) {
		p.app.Send(sel("run"))
		// run dispatched the event that stopped it.
		p.wakePending.Store(false)
	}
	p.brightness = p.currentBrightness()
	p.reduceMotion = p.currentReduceMotion()
	return nil
}

// mainMenu builds the menu bar: the application menu with Hide and Quit, the
// menus of the application, and the Window menu.
func (p *Platform) mainMenu(menus []platform.MenuItem) objc.ID {
	name := goString(class("NSProcessInfo").Send(sel("processInfo")).Send(sel("processName")))
	bar := class("NSMenu").Send(sel("new")).Send(sel("autorelease"))
	item := func(menu objc.ID, title, action, key string, mods uint64) {
		it := class("NSMenuItem").Send(sel("alloc")).Send(sel("initWithTitle:action:keyEquivalent:"),
			nsString(title), sel(action), nsString(key)).Send(sel("autorelease"))
		if mods != 0 {
			it.Send(sel("setKeyEquivalentModifierMask:"), mods)
		}
		menu.Send(sel("addItem:"), it)
	}
	submenu := func(title string) objc.ID {
		top := class("NSMenuItem").Send(sel("new")).Send(sel("autorelease"))
		menu := class("NSMenu").Send(sel("alloc")).Send(sel("initWithTitle:"), nsString(title)).Send(sel("autorelease"))
		top.Send(sel("setSubmenu:"), menu)
		bar.Send(sel("addItem:"), top)
		return menu
	}
	appMenu := submenu(name)
	item(appMenu, "Hide "+name, "hide:", "h", 0)
	item(appMenu, "Hide Others", "hideOtherApplications:", "h", NSEventModifierFlagOption|NSEventModifierFlagCommand)
	item(appMenu, "Show All", "unhideAllApplications:", "", 0)
	appMenu.Send(sel("addItem:"), class("NSMenuItem").Send(sel("separatorItem")))
	item(appMenu, "Quit "+name, "terminate:", "q", 0)
	for i := range menus {
		m := &menus[i]
		if m.Separator {
			continue
		}
		p.addMenuItems(submenu(menuTitle(m.Label)), m.Items)
	}
	windowMenu := submenu("Window")
	item(windowMenu, "Minimize", "performMiniaturize:", "m", 0)
	item(windowMenu, "Zoom", "performZoom:", "", 0)
	item(windowMenu, "Close", "performClose:", "w", 0)
	p.app.Send(sel("setWindowsMenu:"), windowMenu)
	return bar
}

// currentBrightness reads the appearance of the application, which follows
// the system.
func (p *Platform) currentBrightness() platform.Brightness {
	appearance := p.app.Send(sel("effectiveAppearance"))
	names := nsArray(nsString("NSAppearanceNameAqua"), nsString("NSAppearanceNameDarkAqua"))
	best := goString(appearance.Send(sel("bestMatchFromAppearancesWithNames:"), names))
	if strings.Contains(best, "Dark") {
		return platform.BrightnessDark
	}
	return platform.BrightnessLight
}

func (p *Platform) currentReduceMotion() bool {
	workspace := class("NSWorkspace").Send(sel("sharedWorkspace"))
	return objc.Send[bool](workspace, sel("accessibilityDisplayShouldReduceMotion"))
}

// checkSettings reports changes of the brightness and of the settings.
func (p *Platform) checkSettings() {
	if b := p.currentBrightness(); b != p.brightness {
		p.brightness = b
		p.enqueue(platform.Event{Kind: platform.EventBrightness})
	}
	if r := p.currentReduceMotion(); r != p.reduceMotion {
		p.reduceMotion = r
		p.enqueue(platform.Event{Kind: platform.EventSettings})
	}
}

// Brightness implements platform.Platform.
func (p *Platform) Brightness() platform.Brightness { return p.brightness }

// ReduceMotion implements platform.Platform.
func (p *Platform) ReduceMotion() bool { return p.reduceMotion }

// Locale implements platform.Platform. The first preferred language of the
// user is already a BCP 47 tag, such as "ja-JP".
func (p *Platform) Locale() string {
	var tag string
	p.libs.pool(func() {
		languages := class("NSLocale").Send(sel("preferredLanguages"))
		if count(languages) > 0 {
			tag = goString(at(languages, 0))
		}
	})
	return tag
}

// SetModalHandler implements platform.Platform. AppKit holds the main thread
// while the user resizes a window; the handler runs on each step of it.
func (p *Platform) SetModalHandler(fn func()) { p.modalHandler = fn }

// runModalHandler runs the handler inside a live resize.
func (p *Platform) runModalHandler() {
	if p.liveResize == 0 || p.modalHandler == nil || p.inHandler {
		return
	}
	p.inHandler = true
	p.modalHandler()
	p.inHandler = false
}

// Wake implements platform.Platform. It posts an event that ends the wait of
// nextEventMatchingMask, which is safe from any thread.
func (p *Platform) Wake() {
	p.woken.Store(true)
	p.postWake()
}

// enqueue queues an event from a callback of AppKit. Some, such as the quit of
// an Apple Event, arrive inside nextEventMatchingMask without an event of
// their own, so the wait is ended for them.
func (p *Platform) enqueue(e platform.Event) {
	p.queue.Push(e)
	if p.dispatching == 0 {
		p.postWake()
	}
}

// postWake posts the event that ends the wait of nextEventMatchingMask, once
// until it arrives. It is safe from any thread.
func (p *Platform) postWake() {
	if p.wakePending.Swap(true) {
		return
	}
	p.wakeMu.Lock()
	defer p.wakeMu.Unlock()
	if p.destroyed {
		return
	}
	p.libs.pool(func() {
		ev := class("NSEvent").Send(sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"),
			uint64(NSEventTypeApplicationDefined), NSPoint{}, uint64(0), float64(0), int64(0), objc.ID(0),
			int16(wakeSubtype), int64(0), int64(0))
		p.app.Send(sel("postEvent:atStart:"), ev, false)
	})
}

// WaitEvent implements platform.Platform.
func (p *Platform) WaitEvent(timeout time.Duration) platform.Event {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		if ev, ok := p.queue.Pop(); ok {
			return ev
		}
		if p.woken.Swap(false) {
			return platform.Event{Kind: platform.EventWake}
		}
		// Inside sendEvent AppKit runs its own loop; taking events from it
		// would reorder them.
		if p.inHandler || p.dispatching > 0 {
			return platform.Event{}
		}
		var wait time.Duration
		if timeout < 0 {
			wait = -1
		} else if timeout > 0 {
			if wait = time.Until(deadline); wait <= 0 {
				return platform.Event{}
			}
		}
		token := p.libs.push()
		got := p.dispatchOne(wait)
		p.libs.pop(token)
		if !got && timeout == 0 {
			return platform.Event{}
		}
	}
}

// dispatchOne takes the next event of AppKit, waiting up to wait (forever if
// negative), and sends it. It reports whether there was one.
func (p *Platform) dispatchOne(wait time.Duration) bool {
	var until objc.ID
	switch {
	case wait < 0:
		until = class("NSDate").Send(sel("distantFuture"))
	case wait == 0:
		until = class("NSDate").Send(sel("distantPast"))
	default:
		until = class("NSDate").Send(sel("dateWithTimeIntervalSinceNow:"), wait.Seconds())
	}
	ev := p.app.Send(sel("nextEventMatchingMask:untilDate:inMode:dequeue:"), ^uint64(0), until, p.libs.defaultRunLoopMode, true)
	if ev == 0 {
		return false
	}
	if objc.Send[uint64](ev, sel("type")) == NSEventTypeApplicationDefined &&
		objc.Send[int16](ev, sel("subtype")) == wakeSubtype {
		p.wakePending.Store(false)
		return true
	}
	p.dispatching++
	p.app.Send(sel("sendEvent:"), ev)
	p.dispatching--
	return true
}

// Destroy implements platform.Platform.
func (p *Platform) Destroy() {
	for _, w := range p.windows {
		w.Close()
	}
	for len(p.trays) > 0 {
		p.trays[len(p.trays)-1].Close()
	}
	p.wakeMu.Lock()
	p.destroyed = true
	p.wakeMu.Unlock()
	p.libs.pool(func() {
		p.app.Send(sel("setDelegate:"), objc.ID(0))
		if p.appDelegate != 0 {
			p.appDelegate.Send(sel("release"))
			p.appDelegate = 0
		}
	})
}

// nextID returns the ID of a new window.
func (p *Platform) nextID() platform.WindowID {
	p.lastID++
	return p.lastID
}

// className returns the name of a class of this Platform.
func className(base string, serial uint32) string {
	return fmt.Sprintf("Gowaku%s%d", base, serial)
}
