//go:build darwin && arm64

package cocoa

import (
	"bytes"
	"image/png"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
)

// trayTagStride separates the tags of the menu items of the trays: tray n
// gives its items the tags from n*trayTagStride up.
const trayTagStride = 1 << 20

// tray is a status item in the menu bar, the tray of macOS.
type tray struct {
	p      *Platform
	serial int
	item   objc.ID
	on     func(platform.TrayEvent)
	// ids holds the IDs of the menu items by their place in the tags.
	ids []int
}

// CreateTray implements platform.Trays with an NSStatusItem. With menu items,
// a click opens the menu; without, it activates the tray.
func (p *Platform) CreateTray(opts platform.TrayOptions, on func(platform.TrayEvent)) (platform.Tray, error) {
	p.lastTray++
	t := &tray{p: p, serial: p.lastTray, on: on}
	p.libs.pool(func() {
		bar := class("NSStatusBar").Send(sel("systemStatusBar"))
		t.item = bar.Send(sel("statusItemWithLength:"), float64(-1)).Send(sel("retain")) // NSVariableStatusItemLength
	})
	if t.item == 0 {
		return nil, platform.ErrNoTray
	}
	p.trays = append(p.trays, t)
	t.Update(opts)
	return t, nil
}

// Update implements platform.Tray.
func (t *tray) Update(opts platform.TrayOptions) {
	if t.item == 0 {
		return
	}
	p := t.p
	p.libs.pool(func() {
		button := t.item.Send(sel("button"))
		if img := nsImage(opts); img != 0 {
			button.Send(sel("setImage:"), img)
			button.Send(sel("setTitle:"), nsString(""))
		} else {
			button.Send(sel("setTitle:"), nsString(opts.Tooltip))
		}
		button.Send(sel("setToolTip:"), nsString(opts.Tooltip))
		t.ids = t.ids[:0]
		if len(opts.Items) > 0 {
			menu := class("NSMenu").Send(sel("new")).Send(sel("autorelease"))
			t.addItems(menu, opts.Items)
			t.item.Send(sel("setMenu:"), menu)
			button.Send(sel("setAction:"), objc.SEL(0))
		} else {
			t.item.Send(sel("setMenu:"), objc.ID(0))
			button.Send(sel("setTarget:"), p.appDelegate)
			button.Send(sel("setAction:"), sel("gowakuTray:"))
			button.Send(sel("setTag:"), int64(t.serial*trayTagStride-1))
		}
	})
}

// addItems adds the items of a tray menu, tagged by their place in t.ids.
func (t *tray) addItems(menu objc.ID, items []platform.TrayItem) {
	menu.Send(sel("setAutoenablesItems:"), false)
	for i := range items {
		it := &items[i]
		if it.Separator {
			menu.Send(sel("addItem:"), class("NSMenuItem").Send(sel("separatorItem")))
			continue
		}
		item := class("NSMenuItem").Send(sel("alloc")).Send(sel("initWithTitle:action:keyEquivalent:"),
			nsString(menuTitle(it.Label)), sel("gowakuTray:"), nsString("")).Send(sel("autorelease"))
		item.Send(sel("setTarget:"), t.p.appDelegate)
		item.Send(sel("setTag:"), int64(t.serial*trayTagStride+len(t.ids)))
		t.ids = append(t.ids, it.ID)
		item.Send(sel("setEnabled:"), it.Enabled || len(it.Items) > 0)
		if it.Checkable && it.Checked {
			item.Send(sel("setState:"), int64(1))
		}
		if len(it.Items) > 0 {
			sub := class("NSMenu").Send(sel("new")).Send(sel("autorelease"))
			t.addItems(sub, it.Items)
			item.Send(sel("setSubmenu:"), sub)
		}
		menu.Send(sel("addItem:"), item)
	}
}

// nsImage makes the icon of a tray, 18 points high as the menu bar wants.
func nsImage(opts platform.TrayOptions) objc.ID {
	if opts.Icon == nil {
		return 0
	}
	var buf bytes.Buffer
	if png.Encode(&buf, opts.Icon) != nil {
		return 0
	}
	img := class("NSImage").Send(sel("alloc")).Send(sel("initWithData:"), nsData(buf.Bytes())).Send(sel("autorelease"))
	if img == 0 {
		return 0
	}
	b := opts.Icon.Bounds()
	const height = 18
	img.Send(sel("setSize:"), NSSize{float64(b.Dx()) * height / float64(max(b.Dy(), 1)), height})
	return img
}

// chooseTray reports the click of a status item or one of its menu items.
func (p *Platform) chooseTray(sender objc.ID) {
	tag := int(objc.Send[int64](sender, sel("tag")))
	for _, t := range p.trays {
		switch {
		case tag == t.serial*trayTagStride-1:
			t.on(platform.TrayEvent{Kind: platform.TrayActivate})
		case tag >= t.serial*trayTagStride && tag < t.serial*trayTagStride+len(t.ids):
			t.on(platform.TrayEvent{Kind: platform.TrayChoose, Item: t.ids[tag-t.serial*trayTagStride]})
		default:
			continue
		}
		p.postWake()
		return
	}
}

// Close implements platform.Tray.
func (t *tray) Close() {
	if t.item == 0 {
		return
	}
	p := t.p
	p.libs.pool(func() {
		class("NSStatusBar").Send(sel("systemStatusBar")).Send(sel("removeStatusItem:"), t.item)
		t.item.Send(sel("release"))
	})
	t.item = 0
	for i, other := range p.trays {
		if other == t {
			p.trays = append(p.trays[:i], p.trays[i+1:]...)
			break
		}
	}
}

// notifyState is what macOS said about the notifications of the
// application. It is shared with the completion handlers of
// UserNotifications, which run on a queue of their own.
type notifyState struct {
	mu sync.Mutex
	// known says that macOS answered, and granted what it answered. asking
	// says that a question is out.
	known, granted, asking bool
	// pending are the requests that Notify made before the first answer,
	// retained, and waiters the done functions of RequestNotifications.
	pending []objc.ID
	waiters []func(error)
}

// Notify implements platform.Trays with UNUserNotificationCenter, which only
// serves an application in a bundle: a bare executable gets
// ErrNoNotifications. The first call asks the user, as RequestNotifications
// does, and the notifications sent until the answer wait for it. Once the
// user has refused, Notify returns ErrNoNotifications, and asks macOS again,
// so that a change in System Settings applies to the next one.
func (p *Platform) Notify(title, body string) error {
	center := p.notificationCenter()
	if center == 0 {
		return platform.ErrNoNotifications
	}
	var request objc.ID
	p.libs.pool(func() {
		content := class("UNMutableNotificationContent").Send(sel("new")).Send(sel("autorelease"))
		content.Send(sel("setTitle:"), nsString(title))
		content.Send(sel("setBody:"), nsString(body))
		id := class("NSUUID").Send(sel("UUID")).Send(sel("UUIDString"))
		request = class("UNNotificationRequest").Send(sel("requestWithIdentifier:content:trigger:"), id, content, objc.ID(0))
		request.Send(sel("retain"))
	})
	n := &p.notify
	n.mu.Lock()
	switch {
	case n.known && n.granted:
		n.mu.Unlock()
		p.postRequest(center, request)
		return nil
	case n.known:
		n.mu.Unlock()
		request.Send(sel("release"))
		p.askNotifications(center)
		return platform.ErrNoNotifications
	}
	n.pending = append(n.pending, request)
	n.mu.Unlock()
	p.askNotifications(center)
	return nil
}

// RequestNotifications implements platform.Trays. macOS asks the user only
// the first time; later it answers at once with what the user chose, in
// System Settings as well.
func (p *Platform) RequestNotifications(done func(error)) {
	center := p.notificationCenter()
	if center == 0 {
		done(platform.ErrNoNotifications)
		return
	}
	n := &p.notify
	n.mu.Lock()
	n.waiters = append(n.waiters, done)
	n.mu.Unlock()
	p.askNotifications(center)
}

// askNotifications asks macOS for leave to show notifications, unless a
// question is out already.
func (p *Platform) askNotifications(center objc.ID) {
	n := &p.notify
	n.mu.Lock()
	if n.asking {
		n.mu.Unlock()
		return
	}
	n.asking = true
	n.mu.Unlock()
	answer := objc.NewBlock(func(_ objc.Block, granted bool, _ objc.ID) {
		n.mu.Lock()
		n.known, n.granted, n.asking = true, granted, false
		pending, waiters := n.pending, n.waiters
		n.pending, n.waiters = nil, nil
		n.mu.Unlock()
		for _, r := range pending {
			if granted {
				p.postRequest(center, r)
			} else {
				r.Send(sel("release"))
			}
		}
		var err error
		if !granted {
			err = platform.ErrNoNotifications
		}
		for _, done := range waiters {
			done(err)
		}
	})
	const alertAndSound = 1<<2 | 1<<1 // UNAuthorizationOptionAlert | UNAuthorizationOptionSound
	p.libs.pool(func() {
		center.Send(sel("requestAuthorizationWithOptions:completionHandler:"), uint64(alertAndSound), answer)
	})
	// The center keeps a copy of the block.
	answer.Release()
}

// postRequest hands a retained request to the center, which may be called from any
// thread, and releases it.
func (p *Platform) postRequest(center, request objc.ID) {
	center.Send(sel("addNotificationRequest:withCompletionHandler:"), request, objc.ID(0))
	request.Send(sel("release"))
}

// notificationCenter returns the current UNUserNotificationCenter, or zero
// outside a bundle. The application delegate is its delegate.
func (p *Platform) notificationCenter() objc.ID {
	if p.notifications != 0 || p.noNotifications {
		return p.notifications
	}
	p.libs.pool(func() {
		bundle := class("NSBundle").Send(sel("mainBundle"))
		if bundle.Send(sel("bundleIdentifier")) == 0 {
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/UserNotifications.framework/UserNotifications",
			purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
			return
		}
		center := class("UNUserNotificationCenter").Send(sel("currentNotificationCenter"))
		if center == 0 {
			return
		}
		center.Send(sel("setDelegate:"), p.appDelegate)
		p.notifications = center.Send(sel("retain"))
	})
	p.noNotifications = p.notifications == 0
	return p.notifications
}

// presentNotification answers whether a notification shows while the
// application is in front: as a banner, in the list and with its sound, as
// it does when the application is not, and as on Windows and Linux.
func presentNotification(completion objc.Block) {
	const banner = 1<<4 | 1<<3 | 1<<1 // UNNotificationPresentationOptionBanner | List | Sound
	callBlock(completion, banner)
}

// notificationClicked reports a click on a notification to the first tray,
// and calls the completion handler that AppKit passes, a block of its own.
func (p *Platform) notificationClicked(completion objc.Block) {
	if len(p.trays) > 0 {
		p.trays[0].on(platform.TrayEvent{Kind: platform.TrayNotificationClick})
		p.postWake()
	}
	callBlock(completion)
}

// callBlock calls a block of AppKit with integer arguments. Its invoke
// function lies after its isa, flags and reserved fields (Block_layout of the
// Blocks ABI).
func callBlock(b objc.Block, args ...uintptr) {
	if b == 0 {
		return
	}
	invoke := *(*uintptr)(unsafe.Add(*(*unsafe.Pointer)(unsafe.Pointer(&b)), 16))
	purego.SyscallN(invoke, append([]uintptr{uintptr(b)}, args...)...)
}
