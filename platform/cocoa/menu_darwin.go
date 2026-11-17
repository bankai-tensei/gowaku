//go:build darwin && arm64

package cocoa

import (
	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
)

// SetMenuBar implements platform.MenuBarWindow. The menus go between the
// application menu and the Window menu of the menu bar while w is the key
// window.
func (w *Window) SetMenuBar(items []platform.MenuItem, on func(id int)) {
	w.menu, w.onMenu = items, on
	p := w.p
	p.libs.pool(func() {
		if p.menuOwner == w || objc.Send[bool](w.window, sel("isKeyWindow")) {
			p.installMenu(w)
		}
	})
}

// installMenu shows the menus of w, or only those of every application for
// nil.
func (p *Platform) installMenu(w *Window) {
	p.menuOwner = w
	var items []platform.MenuItem
	if w != nil {
		items = w.menu
	}
	p.app.Send(sel("setMainMenu:"), p.mainMenu(items))
}

// addMenuItems adds items to menu. Chosen items reach chooseMenuItem through
// their tags.
func (p *Platform) addMenuItems(menu objc.ID, items []platform.MenuItem) {
	menu.Send(sel("setAutoenablesItems:"), false)
	for i := range items {
		it := &items[i]
		if it.Separator {
			menu.Send(sel("addItem:"), class("NSMenuItem").Send(sel("separatorItem")))
			continue
		}
		key, mask, ok := keyEquivalent(it.Key, it.Modifiers)
		if !ok || len(it.Items) > 0 {
			key, mask = "", 0
		}
		item := class("NSMenuItem").Send(sel("alloc")).Send(sel("initWithTitle:action:keyEquivalent:"),
			nsString(menuTitle(it.Label)), sel("gowakuMenu:"), nsString(key)).Send(sel("autorelease"))
		item.Send(sel("setKeyEquivalentModifierMask:"), mask)
		item.Send(sel("setTarget:"), p.appDelegate)
		item.Send(sel("setTag:"), int64(it.ID))
		item.Send(sel("setEnabled:"), it.Enabled)
		if it.Checkable && it.Checked {
			item.Send(sel("setState:"), int64(1)) // NSControlStateValueOn
		}
		if len(it.Items) > 0 {
			sub := class("NSMenu").Send(sel("alloc")).Send(sel("initWithTitle:"), nsString(menuTitle(it.Label))).Send(sel("autorelease"))
			p.addMenuItems(sub, it.Items)
			item.Send(sel("setSubmenu:"), sub)
			item.Send(sel("setAction:"), objc.SEL(0))
		}
		menu.Send(sel("addItem:"), item)
	}
}

// chooseMenuItem runs the item of the key window whose tag sender has.
func (p *Platform) chooseMenuItem(sender objc.ID) {
	w := p.menuOwner
	if w == nil || w.onMenu == nil {
		return
	}
	id := int(objc.Send[int64](sender, sel("tag")))
	if id >= 0 {
		w.onMenu(id)
		p.postWake()
	}
}
