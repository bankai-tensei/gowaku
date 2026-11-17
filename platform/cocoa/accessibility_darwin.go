//go:build darwin && arm64

package cocoa

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

// NSAccessibility: the content view answers for the root of the tree, which is
// the window, and an NSAccessibilityElement of a class of this Platform answers
// for each node below it. See docs/ADR/0085-accessibility-macos.md.

// bridgeSelector is a method of the content view that hands Window.Accessibility
// the bridge of the view. The bridges live in the state of the view class,
// which only its methods reach.
const bridgeSelector = "gowakuAccessibilityBridge:"

// NSAccessibilityPriorityLevel values, for announcements.
const (
	NSAccessibilityPriorityMedium = 50
	NSAccessibilityPriorityHigh   = 90
)

// axSymbols holds what AppKit exports for NSAccessibility: the functions that
// post notifications, and the names of the notifications, which are NSString
// constants that the framework keeps alive.
type axSymbols struct {
	post            uintptr // NSAccessibilityPostNotification
	postWithInfo    uintptr // NSAccessibilityPostNotificationWithUserInfo
	roleDescription uintptr // NSAccessibilityRoleDescription

	focusChanged            objc.ID
	valueChanged            objc.ID
	titleChanged            objc.ID
	layoutChanged           objc.ID
	selectedTextChanged     objc.ID
	selectedChildrenChanged objc.ID
	rowExpanded             objc.ID
	rowCollapsed            objc.ID
	destroyed               objc.ID
	announcement            objc.ID
	announcementKey         objc.ID
	priorityKey             objc.ID
}

// axLoaded resolves the symbols once per process, when a reader first asks.
var axLoaded = sync.OnceValues(loadAXSymbols)

func loadAXSymbols() (*axSymbols, error) {
	// AppKit is open already (openLibs); this returns the same handle.
	appKit, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return nil, fmt.Errorf("cocoa: open AppKit: %w", err)
	}
	s := &axSymbols{}
	for _, f := range []struct {
		name string
		to   *uintptr
	}{
		{"NSAccessibilityPostNotification", &s.post},
		{"NSAccessibilityPostNotificationWithUserInfo", &s.postWithInfo},
		{"NSAccessibilityRoleDescription", &s.roleDescription},
	} {
		if *f.to, err = purego.Dlsym(appKit, f.name); err != nil {
			return nil, fmt.Errorf("cocoa: missing %s: %w", f.name, err)
		}
	}
	for _, c := range []struct {
		name string
		to   *objc.ID
	}{
		{"NSAccessibilityFocusedUIElementChangedNotification", &s.focusChanged},
		{"NSAccessibilityValueChangedNotification", &s.valueChanged},
		{"NSAccessibilityTitleChangedNotification", &s.titleChanged},
		{"NSAccessibilityLayoutChangedNotification", &s.layoutChanged},
		{"NSAccessibilitySelectedTextChangedNotification", &s.selectedTextChanged},
		{"NSAccessibilitySelectedChildrenChangedNotification", &s.selectedChildrenChanged},
		{"NSAccessibilityRowExpandedNotification", &s.rowExpanded},
		{"NSAccessibilityRowCollapsedNotification", &s.rowCollapsed},
		{"NSAccessibilityUIElementDestroyedNotification", &s.destroyed},
		{"NSAccessibilityAnnouncementRequestedNotification", &s.announcement},
		{"NSAccessibilityAnnouncementKey", &s.announcementKey},
		{"NSAccessibilityPriorityKey", &s.priorityKey},
	} {
		sym, err := purego.Dlsym(appKit, c.name)
		if err != nil {
			return nil, fmt.Errorf("cocoa: missing %s: %w", c.name, err)
		}
		// The symbol is an NSString *const, as NSDefaultRunLoopMode is.
		*c.to = **(**objc.ID)(unsafe.Pointer(&sym))
	}
	return s, nil
}

// axPlatform is the NSAccessibility state of a Platform. The methods of the
// content view and of the element class are closures over it.
type axPlatform struct {
	p *Platform
	// class is the element class, made by the first question; classErr
	// remembers that making it failed.
	class    objc.Class
	classErr error
	syms     *axSymbols
	// allowed decides isAccessibilitySelectorAllowed for the selectors that
	// not every node supports.
	allowed map[objc.SEL]func(*semantics.Node) bool

	bridges  map[objc.ID]*bridge // by content view
	elements map[objc.ID]axRef   // by element object
}

// axRef is what an element object stands for.
type axRef struct {
	b  *bridge
	id semantics.ID
}

// bridge is the NSAccessibility side of a window, and its
// platform.Accessibility (ADR 0042 §3). Everything runs on the main thread:
// AppKit asks from its run loop, inside WaitEvent.
type bridge struct {
	ax        *axPlatform
	w         *Window
	listening bool
	tree      axTree
	focus     semantics.ID
	// elements holds one reference to the element object of each node that
	// a reader has seen, by ID. The object stays the same for as long as the
	// node does.
	elements map[semantics.ID]objc.ID
}

var _ platform.Accessibility = (*bridge)(nil)

// accessibilityViewMethods returns the methods of NSAccessibility that the
// content view adds: the view answers for the root of the tree.
func (p *Platform) accessibilityViewMethods() []objc.MethodDef {
	ax := &axPlatform{
		p:        p,
		bridges:  make(map[objc.ID]*bridge),
		elements: make(map[objc.ID]axRef),
	}
	return []objc.MethodDef{
		{Cmd: sel(bridgeSelector), Fn: func(self objc.ID, _ objc.SEL, out unsafe.Pointer) {
			// out is a *(*bridge) on the heap, from Window.Accessibility,
			// valid for this call.
			*(**bridge)(out) = ax.bridgeFor(self)
		}},
		{Cmd: sel("accessibilityChildren"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			b := ax.bridgeFor(self)
			if b == nil || !b.listen() {
				return nsArray()
			}
			return b.children(0)
		}},
		{Cmd: sel("accessibilityHitTest:"), Fn: func(self objc.ID, cmd objc.SEL, point NSPoint) objc.ID {
			b := ax.bridgeFor(self)
			if b == nil || !b.listen() {
				return objc.SendSuper[objc.ID](self, cmd, point)
			}
			if id := b.tree.hitTest(b.fromScreen(point)); id != 0 {
				return b.element(id)
			}
			return objc.SendSuper[objc.ID](self, cmd, point)
		}},
		{Cmd: sel("accessibilityFocusedUIElement"), Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
			// Asking for the focus alone does not start the tree: AppKit
			// asks it of the first responder for more than readers.
			if b := ax.bridgeFor(self); b != nil && b.listening && b.tree.has(b.focus) {
				return b.element(b.focus)
			}
			return objc.SendSuper[objc.ID](self, cmd)
		}},
	}
}

// Accessibility implements platform.Window.
func (w *Window) Accessibility() platform.Accessibility {
	// out is on the heap, so that it stays put while the view writes it.
	out := new(*bridge)
	w.view.Send(sel(bridgeSelector), unsafe.Pointer(out))
	if *out == nil {
		return nil
	}
	return *out
}

// bridgeFor returns the bridge of a content view, made on first use, or nil
// for a view without a window. It forgets the bridges of closed windows, whose
// views may be gone and their addresses taken by new ones.
func (ax *axPlatform) bridgeFor(view objc.ID) *bridge {
	for v, b := range ax.bridges {
		if b.w.closed || ax.p.byView[v] != b.w {
			b.release()
			delete(ax.bridges, v)
		}
	}
	w := ax.p.byView[view]
	if w == nil {
		return nil
	}
	if b := ax.bridges[view]; b != nil {
		return b
	}
	b := &bridge{ax: ax, w: w, elements: make(map[semantics.ID]objc.ID)}
	ax.bridges[view] = b
	return b
}

// ready makes the element class and loads the symbols, on the first question.
// It reports false, once in the log, if either fails.
func (ax *axPlatform) ready() bool {
	if ax.class != 0 {
		return true
	}
	if ax.classErr != nil {
		return false
	}
	syms, err := axLoaded()
	if err == nil {
		ax.syms = syms
		ax.class, err = objc.RegisterClass(className("Element", classSerial.Add(1)),
			objc.GetClass("NSAccessibilityElement"), nil, nil, ax.elementMethods())
	}
	if err != nil {
		ax.classErr = err
		slog.Debug("cocoa: no NSAccessibility", "err", err)
		return false
	}
	ax.allowed = ax.allowedSelectors()
	return true
}

// listen marks the bridge as listened to, and queues a redraw on the first
// question so that a tree follows (platform.Accessibility). It reports false
// if NSAccessibility cannot be served.
func (b *bridge) listen() bool {
	if !b.ax.ready() {
		return false
	}
	if !b.listening {
		b.listening = true
		b.ax.p.enqueue(platform.Event{Kind: platform.EventRedraw, Window: b.w.id})
	}
	return true
}

// Listening implements platform.Accessibility.
func (b *bridge) Listening() bool { return b.listening }

// Update implements platform.Accessibility.
func (b *bridge) Update(root *semantics.Node, focus semantics.ID, d *semantics.Diff) {
	if b.w.closed {
		b.release()
		return
	}
	before := b.tree
	b.tree, b.focus = newAXTree(root), focus
	if len(b.elements) == 0 && (!b.listening || d == nil) {
		return
	}
	b.ax.p.libs.pool(func() {
		for id, el := range b.elements {
			if !b.tree.has(id) {
				b.post(el, b.ax.syms.destroyed)
				b.drop(id, el)
			}
		}
		if !b.listening || d == nil {
			return
		}
		for _, n := range axNotes(&before, &b.tree, d, focus) {
			b.notify(n)
		}
	})
}

// notify posts a notification, for the elements a reader has seen; the focus
// always, as the reader follows it.
func (b *bridge) notify(n axNote) {
	s := b.ax.syms
	switch n.kind {
	case noteLayout:
		b.post(b.w.window, s.layoutChanged)
		return
	case noteFocus:
		b.post(b.element(n.id), s.focusChanged)
		return
	case noteAnnounce:
		priority := int64(NSAccessibilityPriorityMedium)
		if n.assertive {
			priority = NSAccessibilityPriorityHigh
		}
		keys := []objc.ID{s.announcementKey, s.priorityKey}
		values := []objc.ID{nsString(n.text), class("NSNumber").Send(sel("numberWithInteger:"), priority)}
		// The dictionary copies the keys and retains the values before the
		// call returns.
		info := class("NSDictionary").Send(sel("dictionaryWithObjects:forKeys:count:"),
			unsafe.Pointer(&values[0]), unsafe.Pointer(&keys[0]), uint64(len(keys)))
		purego.SyscallN(s.postWithInfo, uintptr(b.ax.p.app), uintptr(s.announcement), uintptr(info))
		return
	}
	el := b.elements[n.id]
	if el == 0 {
		return
	}
	switch n.kind {
	case noteValue:
		b.post(el, s.valueChanged)
	case noteTitle:
		b.post(el, s.titleChanged)
	case noteSelectedText:
		b.post(el, s.selectedTextChanged)
	case noteSelectedChildren:
		b.post(el, s.selectedChildrenChanged)
	case noteRowExpanded:
		b.post(el, s.rowExpanded)
	case noteRowCollapsed:
		b.post(el, s.rowCollapsed)
	}
}

func (b *bridge) post(element, name objc.ID) {
	if element != 0 && name != 0 {
		purego.SyscallN(b.ax.syms.post, uintptr(element), uintptr(name))
	}
}

// drop forgets the element object of id.
func (b *bridge) drop(id semantics.ID, el objc.ID) {
	delete(b.elements, id)
	delete(b.ax.elements, el)
	el.Send(sel("release"))
}

// release lets go of every element object, for a window that closed.
func (b *bridge) release() {
	if len(b.elements) == 0 {
		return
	}
	b.ax.p.libs.pool(func() {
		for id, el := range b.elements {
			b.post(el, b.ax.syms.destroyed)
			b.drop(id, el)
		}
	})
	b.listening = false
}

// element returns the object that stands for node id: the content view for
// the root, an element object made on first use for the others, zero for a
// node the tree lacks.
func (b *bridge) element(id semantics.ID) objc.ID {
	if id == 0 {
		return b.w.view
	}
	if el := b.elements[id]; el != 0 {
		return el
	}
	if !b.tree.has(id) || b.ax.class == 0 {
		return 0
	}
	el := objc.ID(b.ax.class).Send(sel("new"))
	b.elements[id] = el
	b.ax.elements[el] = axRef{b: b, id: id}
	return el
}

// array returns an NSArray of the objects of ids.
func (b *bridge) array(ids []semantics.ID) objc.ID {
	objects := make([]objc.ID, 0, len(ids))
	for _, id := range ids {
		if el := b.element(id); el != 0 {
			objects = append(objects, el)
		}
	}
	return nsArray(objects...)
}

// children returns an NSArray of the objects of the children of id.
func (b *bridge) children(id semantics.ID) objc.ID {
	n := b.tree.node(id)
	if n == nil {
		return nsArray()
	}
	ids := make([]semantics.ID, len(n.Children))
	for i, c := range n.Children {
		ids[i] = c.ID
	}
	return b.array(ids)
}

// toScreen converts a box in logical pixels of the window, which are the
// points of the flipped content view, to the coordinates of the screen.
func (b *bridge) toScreen(r geom.Rect) NSRect {
	local := NSRect{
		Origin: NSPoint{float64(r.X), float64(r.Y)},
		Size:   NSSize{float64(r.W), float64(r.H)},
	}
	inWindow := objc.Send[NSRect](b.w.view, sel("convertRect:toView:"), local, objc.ID(0))
	return objc.Send[NSRect](b.w.window, sel("convertRectToScreen:"), inWindow)
}

// fromScreen converts a point of the screen to logical pixels of the window.
func (b *bridge) fromScreen(p NSPoint) geom.Offset {
	inWindow := objc.Send[NSPoint](b.w.window, sel("convertPointFromScreen:"), p)
	local := objc.Send[NSPoint](b.w.view, sel("convertPoint:fromView:"), inWindow, objc.ID(0))
	return geom.Offset{X: float32(local.X), Y: float32(local.Y)}
}

// act queues action ev for node id, unless the node is disabled.
func (b *bridge) act(n *semantics.Node, ev platform.Event) bool {
	if !axEnabled(n) {
		return false
	}
	ev.Kind, ev.Window, ev.Node = platform.EventAccessibilityAction, b.w.id, n.ID
	b.ax.p.enqueue(ev)
	return true
}

// lookup returns the bridge and node behind an element object, or nil for one
// whose node is gone.
func (ax *axPlatform) lookup(el objc.ID) (*bridge, *semantics.Node) {
	ref, ok := ax.elements[el]
	if !ok || ref.b.w.closed {
		return nil, nil
	}
	n := ref.b.tree.node(ref.id)
	if n == nil {
		return nil, nil
	}
	return ref.b, n
}

func nsNumber(v float64) objc.ID { return class("NSNumber").Send(sel("numberWithDouble:"), v) }

func nsInteger(v int64) objc.ID { return class("NSNumber").Send(sel("numberWithInteger:"), v) }

// valueObject returns the AXValue of n as an NSNumber or an NSString, or nil.
func valueObject(n *semantics.Node) objc.ID {
	v := axNodeValue(n)
	switch v.kind {
	case axValueInt:
		return nsInteger(int64(v.number))
	case axValueNumber:
		return nsNumber(v.number)
	case axValueString:
		return nsString(v.text)
	}
	return 0
}

// elementMethods returns the methods of the element class: NSAccessibility,
// answered from the node of each element.
func (ax *axPlatform) elementMethods() []objc.MethodDef {
	// str, flag and num are getters that answer from the node, and nothing
	// for an element whose node is gone.
	str := func(get func(*semantics.Node) string) func(objc.ID, objc.SEL) objc.ID {
		return func(self objc.ID, _ objc.SEL) objc.ID {
			if _, n := ax.lookup(self); n != nil {
				if s := get(n); s != "" {
					return nsString(s)
				}
			}
			return 0
		}
	}
	flag := func(get func(*semantics.Node) bool) func(objc.ID, objc.SEL) bool {
		return func(self objc.ID, _ objc.SEL) bool {
			_, n := ax.lookup(self)
			return n != nil && get(n)
		}
	}
	// text answers the questions about the text of a field.
	text := func(self objc.ID) (*bridge, *semantics.Node, []uint16) {
		b, n := ax.lookup(self)
		if n == nil || n.Text == nil {
			return nil, nil, nil
		}
		return b, n, semantics.UTF16(n.Text.Text)
	}
	return []objc.MethodDef{
		{Cmd: sel("isAccessibilityElement"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("accessibilityRole"), Fn: str(func(n *semantics.Node) string {
			role, _ := axRole(n)
			return role
		})},
		{Cmd: sel("accessibilitySubrole"), Fn: str(func(n *semantics.Node) string {
			_, subrole := axRole(n)
			return subrole
		})},
		{Cmd: sel("accessibilityRoleDescription"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			_, n := ax.lookup(self)
			if n == nil {
				return 0
			}
			role, subrole := axRole(n)
			var sub objc.ID
			if subrole != "" {
				sub = nsString(subrole)
			}
			r, _, _ := purego.SyscallN(ax.syms.roleDescription, uintptr(nsString(role)), uintptr(sub))
			if d := objc.ID(r); d != 0 && objc.Send[uint64](d, sel("length")) > 0 {
				return d
			}
			if role == axRoleHeading {
				return nsString("heading")
			}
			return 0
		}},
		{Cmd: sel("accessibilityLabel"), Fn: str(axLabel)},
		{Cmd: sel("accessibilityHelp"), Fn: str(func(n *semantics.Node) string { return n.Hint })},
		{Cmd: sel("accessibilityIdentifier"), Fn: str(func(n *semantics.Node) string {
			return "gowaku-" + strconv.FormatUint(uint64(n.ID), 10)
		})},
		{Cmd: sel("accessibilityValue"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if _, n := ax.lookup(self); n != nil {
				return valueObject(n)
			}
			return 0
		}},
		{Cmd: sel("accessibilityMinValue"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if _, n := ax.lookup(self); n != nil && axHasRange(n) {
				return nsNumber(n.Range.Min)
			}
			return 0
		}},
		{Cmd: sel("accessibilityMaxValue"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if _, n := ax.lookup(self); n != nil && axHasRange(n) {
				return nsNumber(n.Range.Max)
			}
			return 0
		}},
		{Cmd: sel("accessibilityFrame"), Fn: func(self objc.ID, _ objc.SEL) NSRect {
			if b, n := ax.lookup(self); n != nil {
				return b.toScreen(n.Rect)
			}
			return NSRect{}
		}},
		{Cmd: sel("accessibilityParent"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			ref, ok := ax.elements[self]
			if b, n := ax.lookup(self); n != nil && ok {
				return b.element(b.tree.parents[ref.id])
			}
			return 0
		}},
		{Cmd: sel("accessibilityChildren"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if b, n := ax.lookup(self); n != nil {
				return b.children(n.ID)
			}
			return nsArray()
		}},
		{Cmd: sel("accessibilityWindow"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if b, _ := ax.lookup(self); b != nil {
				return b.w.window
			}
			return 0
		}},
		{Cmd: sel("accessibilityTopLevelUIElement"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if b, _ := ax.lookup(self); b != nil {
				return b.w.window
			}
			return 0
		}},
		{Cmd: sel("isAccessibilityEnabled"), Fn: flag(axEnabled)},
		{Cmd: sel("isAccessibilityFocused"), Fn: func(self objc.ID, _ objc.SEL) bool {
			b, n := ax.lookup(self)
			return n != nil && b.focus == n.ID
		}},
		{Cmd: sel("setAccessibilityFocused:"), Fn: func(self objc.ID, _ objc.SEL, focused bool) {
			if b, n := ax.lookup(self); n != nil && focused && b.focus != n.ID {
				b.act(n, platform.Event{Action: semantics.ActionFocus})
			}
		}},
		{Cmd: sel("isAccessibilityRequired"), Fn: flag(func(n *semantics.Node) bool { return n.State&semantics.Required != 0 })},
		{Cmd: sel("isAccessibilitySelected"), Fn: flag(func(n *semantics.Node) bool { return n.State&semantics.Selected != 0 })},
		{Cmd: sel("setAccessibilitySelected:"), Fn: func(self objc.ID, _ objc.SEL, selected bool) {
			// A group holds one choice: selecting taps, and nothing deselects.
			if b, n := ax.lookup(self); n != nil && selected && n.State&semantics.Selected == 0 {
				b.act(n, platform.Event{Action: semantics.ActionTap})
			}
		}},
		{Cmd: sel("accessibilitySelectedChildren"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if b, n := ax.lookup(self); n != nil {
				return b.array(b.tree.selection(n.ID))
			}
			return 0
		}},
		{Cmd: sel("isAccessibilityExpanded"), Fn: flag(func(n *semantics.Node) bool { return n.State&semantics.Expanded != 0 })},
		{Cmd: sel("setAccessibilityExpanded:"), Fn: func(self objc.ID, _ objc.SEL, open bool) {
			if b, n := ax.lookup(self); n != nil {
				if action, ok := axExpandAction(n, open); ok {
					b.act(n, platform.Event{Action: action})
				}
			}
		}},
		{Cmd: sel("isAccessibilityDisclosed"), Fn: flag(func(n *semantics.Node) bool { return n.State&semantics.Expanded != 0 })},
		{Cmd: sel("setAccessibilityDisclosed:"), Fn: func(self objc.ID, _ objc.SEL, open bool) {
			if b, n := ax.lookup(self); n != nil {
				if action, ok := axExpandAction(n, open); ok {
					b.act(n, platform.Event{Action: action})
				}
			}
		}},
		{Cmd: sel("accessibilityDisclosureLevel"), Fn: func(self objc.ID, _ objc.SEL) int64 {
			if _, n := ax.lookup(self); n != nil && n.Level > 0 {
				return int64(n.Level - 1)
			}
			return 0
		}},
		{Cmd: sel("accessibilitySortDirection"), Fn: func(self objc.ID, _ objc.SEL) int64 {
			if _, n := ax.lookup(self); n != nil {
				return axSortDirection(n)
			}
			return axSortUnknown
		}},
		{Cmd: sel("accessibilityDisclosedByRow"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if b, n := ax.lookup(self); n != nil {
				if id := b.tree.disclosedBy(n.ID); id != 0 {
					return b.element(id)
				}
			}
			return 0
		}},
		{Cmd: sel("accessibilityDisclosedRows"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if b, n := ax.lookup(self); n != nil {
				return b.array(b.tree.disclosed(n.ID))
			}
			return 0
		}},
		{Cmd: sel("accessibilityRows"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if b, n := ax.lookup(self); n != nil {
				return b.array(b.tree.rows(n.ID))
			}
			return 0
		}},

		// Actions
		{Cmd: sel("accessibilityPerformPress"), Fn: func(self objc.ID, _ objc.SEL) bool {
			b, n := ax.lookup(self)
			return n != nil && n.Actions&semantics.ActionTap != 0 && b.act(n, platform.Event{Action: semantics.ActionTap})
		}},
		{Cmd: sel("accessibilityPerformIncrement"), Fn: func(self objc.ID, _ objc.SEL) bool {
			b, n := ax.lookup(self)
			return n != nil && n.Actions&semantics.ActionIncrease != 0 && b.act(n, platform.Event{Action: semantics.ActionIncrease})
		}},
		{Cmd: sel("accessibilityPerformDecrement"), Fn: func(self objc.ID, _ objc.SEL) bool {
			b, n := ax.lookup(self)
			return n != nil && n.Actions&semantics.ActionDecrease != 0 && b.act(n, platform.Event{Action: semantics.ActionDecrease})
		}},
		{Cmd: sel("setAccessibilityValue:"), Fn: func(self objc.ID, _ objc.SEL, value objc.ID) {
			b, n := ax.lookup(self)
			if n == nil || value == 0 || !axCanSetValue(n) {
				return
			}
			if axHasRange(n) {
				v := objc.Send[float64](value, sel("doubleValue"))
				if action, ok := axSetRangeAction(n, v); ok {
					b.act(n, platform.Event{Action: action, Number: v})
				}
				return
			}
			if n.Actions&semantics.ActionSetValue != 0 {
				b.act(n, platform.Event{Action: semantics.ActionSetValue, Text: objectText(value)})
			}
		}},

		// The text of a field, in UTF-16 code units as NSString counts them.
		{Cmd: sel("accessibilityNumberOfCharacters"), Fn: func(self objc.ID, _ objc.SEL) int64 {
			_, _, s := text(self)
			return int64(len(s))
		}},
		{Cmd: sel("accessibilitySelectedText"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			_, n, s := text(self)
			if n == nil {
				return 0
			}
			start, length := axSelection(n.Text, len(s))
			return nsString(axSubstring(s, start, start+length))
		}},
		{Cmd: sel("accessibilitySelectedTextRange"), Fn: func(self objc.ID, _ objc.SEL) NSRange {
			_, n, s := text(self)
			if n == nil {
				return NSRange{}
			}
			start, length := axSelection(n.Text, len(s))
			return NSRange{uint64(start), uint64(length)}
		}},
		{Cmd: sel("setAccessibilitySelectedTextRange:"), Fn: func(self objc.ID, _ objc.SEL, r NSRange) {
			b, n, s := text(self)
			if n == nil || n.Actions&semantics.ActionSetSelection == 0 || r.Location > uint64(len(s)) {
				return
			}
			start, end := axClamp(int(r.Location), int(min(r.Length, uint64(len(s)))), len(s))
			b.act(n, platform.Event{Action: semantics.ActionSetSelection, SelectionStart: start, SelectionEnd: end})
		}},
		{Cmd: sel("accessibilityVisibleCharacterRange"), Fn: func(self objc.ID, _ objc.SEL) NSRange {
			_, _, s := text(self)
			return NSRange{0, uint64(len(s))}
		}},
		{Cmd: sel("accessibilityInsertionPointLineNumber"), Fn: func(self objc.ID, _ objc.SEL) int64 {
			_, n, _ := text(self)
			if n == nil {
				return 0
			}
			return int64(n.Text.Line(n.Text.SelectionEnd))
		}},
		{Cmd: sel("accessibilityLineForIndex:"), Fn: func(self objc.ID, _ objc.SEL, i int64) int64 {
			_, n, _ := text(self)
			if n == nil {
				return 0
			}
			return int64(n.Text.Line(int(i)))
		}},
		{Cmd: sel("accessibilityRangeForLine:"), Fn: func(self objc.ID, _ objc.SEL, line int64) NSRange {
			_, n, s := text(self)
			if n == nil || line < 0 || int(line) >= n.Text.Lines() {
				return NSRange{NSNotFound, 0}
			}
			start, end := n.Text.LineBounds(int(line), len(s))
			return NSRange{uint64(start), uint64(max(end-start, 0))}
		}},
		{Cmd: sel("accessibilityStringForRange:"), Fn: func(self objc.ID, _ objc.SEL, r NSRange) objc.ID {
			_, n, s := text(self)
			if n == nil || r.Location > uint64(len(s)) {
				return 0
			}
			start, end := axClamp(int(r.Location), int(min(r.Length, uint64(len(s)))), len(s))
			return nsString(axSubstring(s, start, end))
		}},
		{Cmd: sel("accessibilityFrameForRange:"), Fn: func(self objc.ID, _ objc.SEL, r NSRange) NSRect {
			b, n, s := text(self)
			if n == nil || r.Location > uint64(len(s)) {
				return NSRect{}
			}
			start, end := axClamp(int(r.Location), int(min(r.Length, uint64(len(s)))), len(s))
			return b.toScreen(axRangeRect(n, start, end-start))
		}},

		{Cmd: sel("isAccessibilitySelectorAllowed:"), Fn: func(self objc.ID, cmd objc.SEL, s objc.SEL) bool {
			allow, ok := ax.allowed[s]
			if !ok {
				return objc.SendSuper[bool](self, cmd, s)
			}
			_, n := ax.lookup(self)
			return n != nil && allow(n)
		}},
	}
}

// allowedSelectors decides which nodes offer the methods that not every node
// supports. NSAccessibilityElement lists an attribute or an action for every
// method the class has, unless isAccessibilitySelectorAllowed says no.
func (ax *axPlatform) allowedSelectors() map[objc.SEL]func(*semantics.Node) bool {
	action := func(a semantics.Actions) func(*semantics.Node) bool {
		return func(n *semantics.Node) bool { return n.Actions&a != 0 }
	}
	field := func(n *semantics.Node) bool { return n.Text != nil }
	ranged := axHasRange
	expandableOnly := func(n *semantics.Node) bool { return axExpandable(n) && n.Role != semantics.RoleTreeItem }
	treeItem := func(n *semantics.Node) bool { return n.Role == semantics.RoleTreeItem }
	tabular := func(n *semantics.Node) bool { return n.Role == semantics.RoleGrid || n.Role == semantics.RoleTree }
	m := map[objc.SEL]func(*semantics.Node) bool{
		sel("accessibilityPerformPress"):     action(semantics.ActionTap),
		sel("accessibilityPerformIncrement"): action(semantics.ActionIncrease),
		sel("accessibilityPerformDecrement"): action(semantics.ActionDecrease),
		sel("setAccessibilityValue:"):        axCanSetValue,
		sel("setAccessibilityFocused:"): func(n *semantics.Node) bool {
			return n.State&semantics.Focusable != 0 || n.Actions&semantics.ActionFocus != 0
		},
		sel("accessibilityMinValue"):         ranged,
		sel("accessibilityMaxValue"):         ranged,
		sel("isAccessibilitySelected"):       axIsSelectionItem,
		sel("setAccessibilitySelected:"):     func(n *semantics.Node) bool { return axIsSelectionItem(n) && n.Actions&semantics.ActionTap != 0 },
		sel("accessibilitySelectedChildren"): axIsSelectionContainer,
		sel("isAccessibilityExpanded"):       expandableOnly,
		sel("setAccessibilityExpanded:"):     expandableOnly,
		sel("isAccessibilityDisclosed"):      treeItem,
		sel("setAccessibilityDisclosed:"):    func(n *semantics.Node) bool { return treeItem(n) && axExpandable(n) },
		sel("accessibilityDisclosureLevel"):  treeItem,
		sel("accessibilityDisclosedByRow"):   treeItem,
		sel("accessibilityDisclosedRows"):    treeItem,
		sel("accessibilityRows"):             tabular,
		sel("accessibilitySortDirection"):    func(n *semantics.Node) bool { return n.Role == semantics.RoleColumnHeader },
		sel("isAccessibilityRequired"): func(n *semantics.Node) bool {
			return n.Role == semantics.RoleTextField || n.State&semantics.Required != 0
		},
		sel("setAccessibilitySelectedTextRange:"): func(n *semantics.Node) bool { return field(n) && n.Actions&semantics.ActionSetSelection != 0 },
	}
	for _, name := range []string{
		"accessibilityNumberOfCharacters", "accessibilitySelectedText", "accessibilitySelectedTextRange",
		"accessibilityVisibleCharacterRange", "accessibilityInsertionPointLineNumber", "accessibilityLineForIndex:",
		"accessibilityRangeForLine:", "accessibilityStringForRange:", "accessibilityFrameForRange:",
	} {
		m[sel(name)] = field
	}
	return m
}

// objectText returns the text of an NSString or an NSAttributedString, or the
// description of another object, such as an NSNumber.
func objectText(value objc.ID) string {
	if objc.Send[bool](value, sel("isKindOfClass:"), objc.GetClass("NSString")) ||
		objc.Send[bool](value, sel("isKindOfClass:"), objc.GetClass("NSAttributedString")) {
		return stringOf(value)
	}
	return goString(value.Send(sel("description")))
}
