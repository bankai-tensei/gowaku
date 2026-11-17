//go:build darwin && arm64

package cocoa

import (
	"bytes"
	"image/png"
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

// Notify implements platform.Trays with UNUserNotificationCenter, which only
// serves an application in a bundle: a bare executable gets
// ErrNoNotifications.
func (p *Platform) Notify(title, body string) error {
	center := p.notificationCenter()
	if center == 0 {
		return platform.ErrNoNotifications
	}
	p.libs.pool(func() {
		content := class("UNMutableNotificationContent").Send(sel("new")).Send(sel("autorelease"))
		content.Send(sel("setTitle:"), nsString(title))
		content.Send(sel("setBody:"), nsString(body))
		id := class("NSUUID").Send(sel("UUID")).Send(sel("UUIDString"))
		request := class("UNNotificationRequest").Send(sel("requestWithIdentifier:content:trigger:"), id, content, objc.ID(0))
		center.Send(sel("addNotificationRequest:withCompletionHandler:"), request, objc.ID(0))
	})
	return nil
}

// notificationCenter returns the current UNUserNotificationCenter, asking for
// permission the first time, or zero outside a bundle.
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
		ask := objc.NewBlock(func(objc.Block, bool, objc.ID) {})
		const alertAndSound = 1<<2 | 1<<1 // UNAuthorizationOptionAlert | UNAuthorizationOptionSound
		center.Send(sel("requestAuthorizationWithOptions:completionHandler:"), uint64(alertAndSound), ask)
		ask.Release()
		p.notifications = center.Send(sel("retain"))
	})
	p.noNotifications = p.notifications == 0
	return p.notifications
}

// notificationClicked reports a click on a notification to the first tray,
// and calls the completion handler that AppKit passes, a block of its own.
func (p *Platform) notificationClicked(completion objc.Block) {
	if len(p.trays) > 0 {
		p.trays[0].on(platform.TrayEvent{Kind: platform.TrayNotificationClick})
		p.postWake()
	}
	if completion != 0 {
		// The invoke function of a block lies after its isa, flags and
		// reserved fields (Block_layout of the Blocks ABI).
		invoke := *(*uintptr)(unsafe.Add(*(*unsafe.Pointer)(unsafe.Pointer(&completion)), 16))
		purego.SyscallN(invoke, uintptr(completion))
	}
}
