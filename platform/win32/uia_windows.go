package win32

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
	"github.com/bankai-tensei/gowaku/semantics"
)

// wmAccessibility carries actions that a reader asked for from a thread other
// than the window's, should UI Automation ever call from one.
const wmAccessibility = WM_USER + 2

var (
	uiautomationcore = windows.NewLazySystemDLL("UIAutomationCore.dll")

	procUiaClientsAreListening            = uiautomationcore.NewProc("UiaClientsAreListening")
	procUiaDisconnectProvider             = uiautomationcore.NewProc("UiaDisconnectProvider")
	procUiaGetReservedNotSupportedValue   = uiautomationcore.NewProc("UiaGetReservedNotSupportedValue")
	procUiaHostProviderFromHwnd           = uiautomationcore.NewProc("UiaHostProviderFromHwnd")
	procUiaRaiseAutomationEvent           = uiautomationcore.NewProc("UiaRaiseAutomationEvent")
	procUiaRaiseAutomationPropertyChanged = uiautomationcore.NewProc("UiaRaiseAutomationPropertyChangedEvent")
	procUiaRaiseStructureChangedEvent     = uiautomationcore.NewProc("UiaRaiseStructureChangedEvent")
	procUiaReturnRawElementProvider       = uiautomationcore.NewProc("UiaReturnRawElementProvider")
	procClientToScreen                    = user32.NewProc("ClientToScreen")
)

// The provider interfaces of UIAutomationCore.h.
var (
	IID_IRawElementProviderSimple       = mustGUID("{d6dd68d1-86fd-4332-8666-9abedea2d24c}")
	IID_IRawElementProviderFragment     = mustGUID("{f7063da8-8359-439c-9297-bbc5299a7d87}")
	IID_IRawElementProviderFragmentRoot = mustGUID("{620ce2a5-ab8f-40a9-86cb-de3c75599b58}")
	IID_IInvokeProvider                 = mustGUID("{54fcb24b-e18e-47a2-b4d3-eccbe77599a2}")
	IID_IToggleProvider                 = mustGUID("{56d00bd0-c4f4-433c-a836-1a52a57e0892}")
	IID_IValueProvider                  = mustGUID("{c7935180-6fb3-4201-b174-7df73adbf64a}")
	IID_IRangeValueProvider             = mustGUID("{36dc7aef-33e6-4691-afe1-2be7274b3d33}")
	IID_ISelectionItemProvider          = mustGUID("{2acad808-b2d4-452d-a407-91ff1ad167b2}")
	IID_IExpandCollapseProvider         = mustGUID("{d847d3a5-cab0-4a98-8c32-ecb45c59ad24}")
	IID_IScrollItemProvider             = mustGUID("{2360c714-4bf1-4b26-ba65-9b21316127eb}")
	IID_ITextProvider                   = mustGUID("{3589c92c-63f3-4367-99bb-ada653b77cf2}")
	IID_ITextProvider2                  = mustGUID("{0dc5e6ed-3e16-4bf1-8f9a-a979878bc195}")
	IID_ITextRangeProvider              = mustGUID("{5347ad7b-c355-46f8-aff5-909033582f63}")
	IID_ISelectionProvider              = mustGUID("{fb8b03af-3bdf-48d4-bd36-1a65793be168}")
	IID_IGridProvider                   = mustGUID("{b17d6187-0907-464b-a168-0ef17a1572b1}")
	IID_IGridItemProvider               = mustGUID("{d02541f1-fb81-4d64-ae32-f520f8a6dbd1}")
	IID_ITableProvider                  = mustGUID("{9c860395-97b3-490a-b52a-858cc22af166}")
	IID_ITableItemProvider              = mustGUID("{b9734fa6-771f-4d78-9c90-2517999349cd}")
)

// uiaInterfaces holds the provider interfaces implemented in Go.
type uiaInterfaces struct {
	simple, fragment, fragmentRoot, invoke, toggle, value, rangeValue *com.Interface
	selectionItem, expandCollapse, scrollItem, text, text2, textRange *com.Interface
	selection, grid, gridItem, table, tableItem                       *com.Interface
	// node and root are the interfaces of the two kinds of element object.
	node, root []*com.Interface
}

// pattern returns the interface of a pattern.
func (u *uiaInterfaces) pattern(p patternSet) *com.Interface {
	switch p {
	case patternInvoke:
		return u.invoke
	case patternToggle:
		return u.toggle
	case patternValue:
		return u.value
	case patternRangeValue:
		return u.rangeValue
	case patternSelectionItem:
		return u.selectionItem
	case patternExpandCollapse:
		return u.expandCollapse
	case patternScrollItem:
		return u.scrollItem
	case patternText:
		return u.text2
	case patternSelection:
		return u.selection
	case patternGrid:
		return u.grid
	case patternGridItem:
		return u.gridItem
	case patternTable:
		return u.table
	case patternTableItem:
		return u.tableItem
	}
	return nil
}

// uiaSetup loads UIAutomationCore and builds the vtables, once per process,
// when a reader first asks about a window. Like the procedures, the result is
// written once and only read afterward.
var uiaSetup = sync.OnceValues(func() (*uiaInterfaces, error) {
	for _, p := range []*windows.LazyProc{
		procUiaClientsAreListening, procUiaDisconnectProvider, procUiaGetReservedNotSupportedValue,
		procUiaHostProviderFromHwnd, procUiaRaiseAutomationEvent, procUiaRaiseAutomationPropertyChanged,
		procUiaRaiseStructureChangedEvent, procUiaReturnRawElementProvider, procClientToScreen,
	} {
		if err := p.Find(); err != nil {
			return nil, fmt.Errorf("win32: load %s: %w", p.Name, err)
		}
	}
	return newUIAInterfaces()
})

func newUIAInterfaces() (*uiaInterfaces, error) {
	cb := syscall.NewCallback
	setValue, err := com.NewFloatThunk(cb(rangeSetValue), 1)
	if err != nil {
		return nil, err
	}
	fromPoint, err := com.NewFloatThunk(cb(rootElementProviderFromPoint), 2)
	if err != nil {
		return nil, err
	}
	rangeFromPoint, err := textRangeFromPointMethod()
	if err != nil {
		return nil, err
	}
	u := &uiaInterfaces{
		simple: com.NewInterface(IID_IRawElementProviderSimple,
			cb(simpleProviderOptions), cb(simpleGetPatternProvider), cb(simpleGetPropertyValue),
			cb(simpleHostRawElementProvider)),
		fragment: com.NewInterface(IID_IRawElementProviderFragment,
			cb(fragmentNavigate), cb(fragmentGetRuntimeID), cb(fragmentBoundingRectangle),
			cb(fragmentGetEmbeddedFragmentRoots), cb(fragmentSetFocus), cb(fragmentFragmentRoot)),
		fragmentRoot:   com.NewInterface(IID_IRawElementProviderFragmentRoot, fromPoint, cb(rootGetFocus)),
		invoke:         com.NewInterface(IID_IInvokeProvider, cb(invokeInvoke)),
		toggle:         com.NewInterface(IID_IToggleProvider, cb(toggleToggle), cb(toggleToggleState)),
		value:          com.NewInterface(IID_IValueProvider, cb(valueSetValue), cb(valueValue), cb(valueIsReadOnly)),
		selectionItem:  com.NewInterface(IID_ISelectionItemProvider, cb(selectionItemSelect), cb(selectionItemAddToSelection), cb(selectionItemRemoveFromSelection), cb(selectionItemIsSelected), cb(selectionItemSelectionContainer)),
		expandCollapse: com.NewInterface(IID_IExpandCollapseProvider, cb(expandCollapseExpand), cb(expandCollapseCollapse), cb(expandCollapseGetState)),
		scrollItem:     com.NewInterface(IID_IScrollItemProvider, cb(scrollItemScrollIntoView)),
		selection: com.NewInterface(IID_ISelectionProvider,
			cb(selectionGetSelection), cb(selectionCanSelectMultiple), cb(selectionIsSelectionRequired)),
		grid: com.NewInterface(IID_IGridProvider, cb(gridGetItem), cb(gridRowCount), cb(gridColumnCount)),
		gridItem: com.NewInterface(IID_IGridItemProvider,
			cb(gridItemRow), cb(gridItemColumn), cb(gridItemSpan), cb(gridItemSpan), cb(gridItemContainingGrid)),
		table: com.NewInterface(IID_ITableProvider,
			cb(tableGetRowHeaders), cb(tableGetColumnHeaders), cb(tableRowOrColumnMajor)),
		tableItem: com.NewInterface(IID_ITableItemProvider, cb(tableItemGetRowHeaderItems), cb(tableItemGetColumnHeaderItems)),
		rangeValue: com.NewInterface(IID_IRangeValueProvider, setValue,
			cb(rangeValue), cb(rangeIsReadOnly), cb(rangeMaximum), cb(rangeMinimum), cb(rangeLargeChange), cb(rangeSmallChange)),
	}
	// ITextProvider2 extends ITextProvider, so its vtable starts with the same
	// methods.
	text := make([]uintptr, 0, 8)
	text = append(text, cb(textGetSelection), cb(textGetVisibleRanges), cb(textRangeFromChild), rangeFromPoint,
		cb(textDocumentRange), cb(textSupportedTextSelection))
	u.text = com.NewInterface(IID_ITextProvider, text...)
	u.text2 = com.NewInterface(IID_ITextProvider2, append(text, cb(textRangeFromAnnotation), cb(textGetCaretRange))...)
	u.textRange = com.NewInterface(IID_ITextRangeProvider,
		cb(rangeClone), cb(rangeCompare), cb(rangeCompareEndpoints), cb(rangeExpandToEnclosingUnit),
		cb(rangeFindAttribute), cb(rangeFindText), cb(rangeGetAttributeValue), cb(rangeGetBoundingRectangles),
		cb(rangeGetEnclosingElement), cb(rangeGetText), cb(rangeMove), cb(rangeMoveEndpointByUnit),
		cb(rangeMoveEndpointByRange), cb(rangeSelect), cb(rangeAddToSelection), cb(rangeRemoveFromSelection),
		cb(rangeScrollIntoView), cb(rangeGetChildren))
	// IRawElementProviderSimple comes first: an element is handed to UI
	// Automation as that interface.
	u.node = []*com.Interface{u.simple, u.fragment, u.invoke, u.toggle, u.value, u.rangeValue,
		u.selectionItem, u.expandCollapse, u.scrollItem, u.text, u.text2,
		u.selection, u.grid, u.gridItem, u.table, u.tableItem}
	u.root = []*com.Interface{u.simple, u.fragment, u.fragmentRoot}
	return u, nil
}

// bridge is the UI Automation provider of a window, and its
// platform.Accessibility (ADR 0042 §4).
//
// UI Automation calls the providers on the window's thread, as they do not
// ask for COM threading; the mutex still guards the snapshot and the
// providers in case it calls from elsewhere. It is never held while calling
// UI Automation, which may call back.
type bridge struct {
	window *Window
	// thread is the UI thread, where events can go straight to the queue.
	thread    uint32
	listening atomic.Bool

	mu sync.Mutex
	// ifaces is set by the first question.
	ifaces *uiaInterfaces
	snap   snapshot
	focus  semantics.ID
	// providers holds one reference to the element object of each node that
	// UI Automation has seen, by ID; zero is the root. Identity stays stable
	// for as long as the node does.
	providers map[semantics.ID]*com.Unknown
	// pending holds actions that came from another thread.
	pending []platform.Event
	closed  bool
}

var _ platform.Accessibility = (*bridge)(nil)

func newBridge(w *Window) *bridge {
	return &bridge{window: w, thread: windows.GetCurrentThreadId()}
}

// Listening implements platform.Accessibility.
func (b *bridge) Listening() bool { return b.listening.Load() }

// getObject answers WM_GETOBJECT for UiaRootObjectId. It reports false if UI
// Automation is unavailable, and the message goes to DefWindowProc.
func (b *bridge) getObject(wParam, lParam uintptr) (uintptr, bool) {
	ifaces, err := uiaSetup()
	if err != nil {
		return 0, false
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return 0, false
	}
	b.ifaces = ifaces
	root := b.providerLocked(0)
	b.mu.Unlock()
	if !b.listening.Swap(true) {
		// The application builds the tree only while someone listens.
		b.window.platform.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: b.window.id})
	}
	r, _, _ := procUiaReturnRawElementProvider.Call(uintptr(b.window.hwnd), wParam, lParam, root.Addr())
	return r, true
}

// providerLocked returns the element object of id, made on first use, with
// no reference added; nil if the tree lacks id. b.mu must be held.
func (b *bridge) providerLocked(id semantics.ID) *com.Unknown {
	if p := b.providers[id]; p != nil {
		return p
	}
	if b.closed || b.ifaces == nil || !b.snap.has(id) {
		return nil
	}
	ifaces := b.ifaces.node
	if id == 0 {
		ifaces = b.ifaces.root
	}
	p := com.New(&provider{b: b, id: id}, ifaces...)
	if b.providers == nil {
		b.providers = make(map[semantics.ID]*com.Unknown)
	}
	b.providers[id] = p
	return p
}

// simpleLocked returns IRawElementProviderSimple of id with a reference
// added for the caller, or nil.
func (b *bridge) simpleLocked(id semantics.ID) *com.Unknown {
	p := b.providerLocked(id)
	if p != nil {
		p.AddRef()
	}
	return p
}

// fragmentLocked returns IRawElementProviderFragment of id with a reference
// added for the caller, or nil.
func (b *bridge) fragmentLocked(id semantics.ID) *com.Unknown {
	if p := b.providerLocked(id); p != nil {
		return p.Query(b.ifaces.fragment)
	}
	return nil
}

// provider returns the element object of id with a reference added, or nil.
func (b *bridge) provider(id semantics.ID) *com.Unknown {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.simpleLocked(id)
}

// Update implements platform.Accessibility.
func (b *bridge) Update(root *semantics.Node, focus semantics.ID, d *semantics.Diff) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	old, oldFocus := b.snap, b.focus
	b.snap, b.focus = newSnapshot(root), focus
	var gone []*com.Unknown
	for id, p := range b.providers {
		if !b.snap.has(id) {
			gone = append(gone, p)
			delete(b.providers, id)
		}
	}
	ready := b.ifaces != nil
	b.mu.Unlock()
	if !ready {
		return
	}
	for _, p := range gone {
		// The reader learns that the element is gone, and every call that
		// still reaches it fails with UIA_E_ELEMENTNOTAVAILABLE.
		procUiaDisconnectProvider.Call(p.Addr())
		p.Release()
	}
	if r, _, _ := procUiaClientsAreListening.Call(); r == 0 || d == nil {
		return
	}
	b.raise(&old, oldFocus, focus, d)
}

// raise tells the readers what changed (ADR 0042 §4).
func (b *bridge) raise(old *snapshot, oldFocus, focus semantics.ID, d *semantics.Diff) {
	for _, c := range d.Changed {
		n, before := c.Node, old.node(c.Node.ID)
		id := n.ID
		if c.What&semantics.ChangedChildren != 0 {
			b.raiseStructure(id)
		}
		if before == nil || id == 0 {
			continue
		}
		p := patterns(n)
		if c.What&semantics.ChangedLabel != 0 {
			b.raiseProperty(id, UIA_NamePropertyId, propText(nodeName(before)), propText(nodeName(n)))
		}
		if p&patternValue != 0 && c.What&(semantics.ChangedValue|semantics.ChangedText) != 0 {
			b.raiseProperty(id, UIA_ValueValuePropertyId, propText(nodeValue(before)), propText(nodeValue(n)))
		}
		if p&patternRangeValue != 0 && before.Range.Value != n.Range.Value {
			b.raiseRange(id, before.Range.Value, n.Range.Value)
		}
		if c.What&semantics.ChangedState != 0 {
			b.raiseState(id, before, n, p)
		}
		if p&patternText != 0 {
			if c.What&semantics.ChangedText != 0 {
				b.raiseEvent(id, UIA_Text_TextChangedEventId)
			}
			if c.What&semantics.ChangedSelection != 0 {
				b.raiseEvent(id, UIA_Text_TextSelectionChangedEventId)
			}
		}
		if n.Live != semantics.LiveOff && c.What&(semantics.ChangedLabel|semantics.ChangedValue|semantics.ChangedText) != 0 {
			b.raiseEvent(id, UIA_LiveRegionChangedEventId)
		}
	}
	// A live region speaks when it changes, not when it appears, as in ARIA:
	// otherwise every window would read its status lines on opening.
	if d.FocusMoved {
		if oldFocus != 0 && b.has(oldFocus) {
			b.raiseProperty(oldFocus, UIA_HasKeyboardFocusPropertyId, propFlag(true), propFlag(false))
		}
		if focus != 0 {
			b.raiseProperty(focus, UIA_HasKeyboardFocusPropertyId, propFlag(false), propFlag(true))
			b.raiseEvent(focus, UIA_AutomationFocusChangedEventId)
		}
	}
}

func (b *bridge) raiseState(id semantics.ID, before, n *semantics.Node, p patternSet) {
	if p&patternToggle != 0 && toggleState(before) != toggleState(n) {
		b.raiseProperty(id, UIA_ToggleToggleStatePropertyId, propInt(toggleState(before)), propInt(toggleState(n)))
	}
	if p&patternExpandCollapse != 0 && expandCollapseState(before) != expandCollapseState(n) {
		b.raiseProperty(id, UIA_ExpandCollapseExpandCollapseStatePropertyId,
			propInt(expandCollapseState(before)), propInt(expandCollapseState(n)))
	}
	if was, is := sortStatus(before), sortStatus(n); was != is {
		b.raiseProperty(id, UIA_ItemStatusPropertyId, propText(was), propText(is))
	}
	if enabled(before) != enabled(n) {
		b.raiseProperty(id, UIA_IsEnabledPropertyId, propFlag(enabled(before)), propFlag(enabled(n)))
	}
	wasSelected, selected := before.State&semantics.Selected != 0, n.State&semantics.Selected != 0
	if p&patternSelectionItem != 0 && wasSelected != selected {
		b.raiseProperty(id, UIA_SelectionItemIsSelectedPropertyId, propFlag(wasSelected), propFlag(selected))
		if selected {
			b.raiseEvent(id, UIA_SelectionItem_ElementSelectedEventId)
		}
	}
}

func (b *bridge) has(id semantics.ID) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.snap.has(id)
}

func (b *bridge) raiseEvent(id semantics.ID, event int32) {
	if p := b.provider(id); p != nil {
		procUiaRaiseAutomationEvent.Call(p.Addr(), uintptr(event))
		p.Release()
	}
}

func (b *bridge) raiseProperty(id semantics.ID, prop int32, before, after propValue) {
	p := b.provider(id)
	if p == nil {
		return
	}
	var o, n com.VARIANT
	setVariant(&o, before)
	setVariant(&n, after)
	// The VARIANT arguments are passed by value, which both the x64 and the
	// arm64 conventions turn into a pointer to a copy that the caller owns.
	procUiaRaiseAutomationPropertyChanged.Call(p.Addr(), uintptr(prop),
		uintptr(unsafe.Pointer(&o)), uintptr(unsafe.Pointer(&n)))
	o.Clear()
	n.Clear()
	p.Release()
}

func (b *bridge) raiseRange(id semantics.ID, before, after float64) {
	p := b.provider(id)
	if p == nil {
		return
	}
	var o, n com.VARIANT
	o.SetR8(before)
	n.SetR8(after)
	procUiaRaiseAutomationPropertyChanged.Call(p.Addr(), UIA_RangeValueValuePropertyId,
		uintptr(unsafe.Pointer(&o)), uintptr(unsafe.Pointer(&n)))
	p.Release()
}

// raiseStructure says that the children of id changed. The runtime ID is
// that of the parent; the root's comes from its host window, so it has none.
func (b *bridge) raiseStructure(id semantics.ID) {
	p := b.provider(id)
	if p == nil {
		return
	}
	if id == 0 {
		procUiaRaiseStructureChangedEvent.Call(p.Addr(), StructureChangeType_ChildrenInvalidated, 0, 0)
	} else {
		rid := runtimeID(id)
		// rid is only read during the call.
		procUiaRaiseStructureChangedEvent.Call(p.Addr(), StructureChangeType_ChildrenInvalidated,
			uintptr(unsafe.Pointer(&rid[0])), uintptr(len(rid)))
	}
	p.Release()
}

// destroy cuts the window off from UI Automation, when it is destroyed.
func (b *bridge) destroy() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	gone := make([]*com.Unknown, 0, len(b.providers))
	for _, p := range b.providers {
		gone = append(gone, p)
	}
	b.providers, b.snap, b.pending = nil, snapshot{}, nil
	ready := b.ifaces != nil
	b.mu.Unlock()
	if !ready {
		return
	}
	procUiaReturnRawElementProvider.Call(uintptr(b.window.hwnd), 0, 0, 0)
	// Only the providers of this window: UiaDisconnectAllProviders would cut
	// off the other windows too (ADR 0042 §4).
	for _, p := range gone {
		procUiaDisconnectProvider.Call(p.Addr())
		p.Release()
	}
}

// post queues an action that a reader asked for.
func (b *bridge) post(ev platform.Event) {
	ev.Kind = platform.EventAccessibilityAction
	ev.Window = b.window.id
	if windows.GetCurrentThreadId() == b.thread {
		b.window.platform.queue.Push(ev)
		return
	}
	b.mu.Lock()
	b.pending = append(b.pending, ev)
	hwnd := b.window.hwnd
	b.mu.Unlock()
	procPostMessageW.Call(uintptr(hwnd), wmAccessibility, 0, 0)
}

// drain moves the actions from other threads to the queue, on the UI thread.
func (b *bridge) drain() {
	b.mu.Lock()
	pending := b.pending
	b.pending = nil
	b.mu.Unlock()
	for i := range pending {
		b.window.platform.queue.Push(pending[i])
	}
}

// focused reports whether id has the keyboard focus.
func (b *bridge) focused(id semantics.ID) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return id != 0 && b.focus == id
}

// uiaRect is UiaRect.
type uiaRect struct {
	left, top, width, height float64
}

// clientOrigin returns where the client area starts on the screen, in
// physical pixels, and the scale.
func (b *bridge) clientOrigin() (x, y, scale float64) {
	var pt POINT
	// pt is only used during the call.
	procClientToScreen.Call(uintptr(b.window.hwnd), uintptr(unsafe.Pointer(&pt)))
	return float64(pt.x), float64(pt.y), float64(b.window.Scale())
}

// toScreen turns a box in logical pixels of the window into physical pixels
// of the screen, which UI Automation speaks.
func (b *bridge) toScreen(r geom.Rect) uiaRect {
	x, y, s := b.clientOrigin()
	return uiaRect{x + float64(r.X)*s, y + float64(r.Y)*s, float64(r.W) * s, float64(r.H) * s}
}

// fromScreen turns a point in physical pixels of the screen into logical
// pixels of the window.
func (b *bridge) fromScreen(px, py float64) geom.Offset {
	x, y, s := b.clientOrigin()
	return geom.Offset{X: float32((px - x) / s), Y: float32((py - y) / s)}
}

func setVariant(v *com.VARIANT, p propValue) {
	switch p.kind {
	case propI4:
		v.SetI4(p.i)
	case propBool:
		v.SetBool(p.b)
	case propString:
		v.SetBSTR(p.s)
	default:
		v.SetEmpty()
	}
}
