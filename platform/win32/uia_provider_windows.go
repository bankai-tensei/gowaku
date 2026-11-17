package win32

import (
	"math"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The methods of the element objects. UI Automation calls them through the
// shared vtables built in uia_windows.go, so their signatures are fixed by the
// headers: this first, then pointer-sized arguments, an HRESULT back.

// provider is the Go side of an element object: a node of a window, or its
// root for the ID zero.
type provider struct {
	b  *bridge
	id semantics.ID
}

// lookup returns the provider behind this and its node from the last tree.
// The node of the root is nil before the first Update. An element whose node
// is gone fails with UIA_E_ELEMENTNOTAVAILABLE.
func lookup(this *com.Unknown) (*provider, *semantics.Node, uintptr) {
	p, ok := com.Get[*provider](this)
	if !ok {
		return nil, nil, UIA_E_ELEMENTNOTAVAILABLE
	}
	p.b.mu.Lock()
	defer p.b.mu.Unlock()
	if p.b.closed || !p.b.snap.has(p.id) {
		return nil, nil, UIA_E_ELEMENTNOTAVAILABLE
	}
	return p, p.b.snap.node(p.id), com.S_OK
}

// lookupNode is lookup for methods that only nodes have, not the root.
func lookupNode(this *com.Unknown) (*provider, *semantics.Node, uintptr) {
	p, n, hr := lookup(this)
	if hr == com.S_OK && (p.id == 0 || n == nil) {
		return nil, nil, UIA_E_INVALIDOPERATION
	}
	return p, n, hr
}

// act queues action ev for the node behind this.
func act(this *com.Unknown, ev platform.Event) uintptr {
	p, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	if !enabled(n) {
		return UIA_E_ELEMENTNOTENABLED
	}
	ev.Node = n.ID
	p.b.post(ev)
	return com.S_OK
}

func setBool(ret *int32, b bool) {
	*ret = 0
	if b {
		*ret = 1
	}
}

// IRawElementProviderSimple

func simpleProviderOptions(_ *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = ProviderOptions_ServerSideProvider
	return com.S_OK
}

func simpleGetPatternProvider(this *com.Unknown, pattern uintptr, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, n, hr := lookup(this)
	if hr != com.S_OK {
		return hr
	}
	if p.id == 0 || n == nil {
		return com.S_OK
	}
	want := patternOf(int32(pattern))
	if want == 0 || patterns(n)&want == 0 {
		return com.S_OK
	}
	iface := p.b.ifaces.pattern(want)
	if int32(pattern) == UIA_TextPatternId {
		iface = p.b.ifaces.text
	}
	*ret = this.Query(iface)
	return com.S_OK
}

func simpleGetPropertyValue(this *com.Unknown, prop uintptr, ret *com.VARIANT) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	ret.SetEmpty()
	p, n, hr := lookup(this)
	if hr != com.S_OK {
		return hr
	}
	if p.id == 0 || n == nil {
		// The host window answers for the root.
		if int32(prop) == UIA_FrameworkIdPropertyId {
			ret.SetBSTR(uiaFrameworkID)
		}
		return com.S_OK
	}
	setVariant(ret, nodeProperty(n, int32(prop), p.b.focused(n.ID)))
	return com.S_OK
}

func simpleHostRawElementProvider(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, _, hr := lookup(this)
	if hr != com.S_OK || p.id != 0 {
		return hr
	}
	// ret receives a reference for UI Automation.
	r, _, _ := procUiaHostProviderFromHwnd.Call(uintptr(p.b.window.hwnd), uintptr(unsafe.Pointer(ret)))
	return r
}

// IRawElementProviderFragment

func fragmentNavigate(this *com.Unknown, dir uintptr, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, _, hr := lookup(this)
	if hr != com.S_OK {
		return hr
	}
	b := p.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if id, ok := b.snap.navigate(p.id, int32(dir)); ok {
		*ret = b.fragmentLocked(id)
	}
	return com.S_OK
}

func fragmentGetRuntimeID(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, _, hr := lookup(this)
	if hr != com.S_OK || p.id == 0 {
		// The root takes the runtime ID of its host window.
		return hr
	}
	rid := runtimeID(p.id)
	*ret = com.NewSafeArrayI4(rid[:])
	return com.S_OK
}

func fragmentBoundingRectangle(this *com.Unknown, ret *uiaRect) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = uiaRect{}
	p, n, hr := lookup(this)
	if hr != com.S_OK || p.id == 0 || n == nil {
		// An empty box makes UI Automation take the host window's.
		return hr
	}
	*ret = p.b.toScreen(n.Rect)
	return com.S_OK
}

func fragmentGetEmbeddedFragmentRoots(_ *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	return com.S_OK
}

func fragmentSetFocus(this *com.Unknown) uintptr {
	p, _, hr := lookup(this)
	if hr != com.S_OK || p.id == 0 {
		return hr
	}
	return act(this, platform.Event{Action: semantics.ActionFocus})
}

func fragmentFragmentRoot(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, _, hr := lookup(this)
	if hr != com.S_OK {
		return hr
	}
	b := p.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if root := b.providerLocked(0); root != nil {
		*ret = root.Query(b.ifaces.fragmentRoot)
	}
	return com.S_OK
}

// IRawElementProviderFragmentRoot

// rootElementProviderFromPoint receives its two doubles as bits through a
// float thunk.
func rootElementProviderFromPoint(this *com.Unknown, x, y uintptr, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, _, hr := lookup(this)
	if hr != com.S_OK {
		return hr
	}
	b := p.b
	pt := b.fromScreen(math.Float64frombits(uint64(x)), math.Float64frombits(uint64(y)))
	b.mu.Lock()
	defer b.mu.Unlock()
	// Nil means the root itself.
	if id := b.snap.hitTest(pt); id != 0 {
		*ret = b.fragmentLocked(id)
	}
	return com.S_OK
}

func rootGetFocus(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, _, hr := lookup(this)
	if hr != com.S_OK {
		return hr
	}
	b := p.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.focus != 0 && b.snap.has(b.focus) {
		*ret = b.fragmentLocked(b.focus)
	}
	return com.S_OK
}

// IInvokeProvider

func invokeInvoke(this *com.Unknown) uintptr {
	return act(this, platform.Event{Action: semantics.ActionTap})
}

// IToggleProvider

func toggleToggle(this *com.Unknown) uintptr {
	return act(this, platform.Event{Action: semantics.ActionTap})
}

func toggleToggleState(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = toggleState(n)
	return com.S_OK
}

// IValueProvider

func valueSetValue(this *com.Unknown, value *uint16) uintptr {
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	if enabled(n) && valueReadOnly(n) {
		return UIA_E_INVALIDOPERATION
	}
	return act(this, platform.Event{Action: semantics.ActionSetValue, Text: windows.UTF16PtrToString(value)})
}

func valueValue(this *com.Unknown, ret **uint16) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = com.SysAllocString(nodeValue(n))
	return com.S_OK
}

func valueIsReadOnly(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	setBool(ret, valueReadOnly(n))
	return com.S_OK
}

// IRangeValueProvider

// rangeSetValue receives its double as bits through a float thunk.
func rangeSetValue(this *com.Unknown, bits uintptr) uintptr {
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	if enabled(n) && rangeReadOnly(n) {
		return UIA_E_INVALIDOPERATION
	}
	v := math.Float64frombits(uint64(bits))
	if math.IsNaN(v) || v < n.Range.Min || v > n.Range.Max {
		return com.E_INVALIDARG
	}
	return act(this, platform.Event{Action: semantics.ActionSetValue, Number: v})
}

// rangeGet answers the getters of IRangeValueProvider.
func rangeGet(this *com.Unknown, ret *float64, get func(semantics.Range) float64) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = get(n.Range)
	return com.S_OK
}

func rangeValue(this *com.Unknown, ret *float64) uintptr {
	return rangeGet(this, ret, func(r semantics.Range) float64 { return r.Value })
}

func rangeMaximum(this *com.Unknown, ret *float64) uintptr {
	return rangeGet(this, ret, func(r semantics.Range) float64 { return r.Max })
}

func rangeMinimum(this *com.Unknown, ret *float64) uintptr {
	return rangeGet(this, ret, func(r semantics.Range) float64 { return r.Min })
}

func rangeLargeChange(this *com.Unknown, ret *float64) uintptr {
	return rangeGet(this, ret, largeChange)
}

func rangeSmallChange(this *com.Unknown, ret *float64) uintptr {
	return rangeGet(this, ret, smallChange)
}

func rangeIsReadOnly(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	setBool(ret, rangeReadOnly(n))
	return com.S_OK
}

// ISelectionItemProvider

func selected(n *semantics.Node) bool { return n.State&semantics.Selected != 0 }

func selectionItemSelect(this *com.Unknown) uintptr {
	_, n, hr := lookupNode(this)
	if hr != com.S_OK || selected(n) {
		return hr
	}
	return act(this, platform.Event{Action: semantics.ActionTap})
}

// selectionItemAddToSelection succeeds only for what is selected already: a
// group holds one choice.
func selectionItemAddToSelection(this *com.Unknown) uintptr {
	_, n, hr := lookupNode(this)
	if hr != com.S_OK || selected(n) {
		return hr
	}
	return UIA_E_INVALIDOPERATION
}

func selectionItemRemoveFromSelection(this *com.Unknown) uintptr {
	_, n, hr := lookupNode(this)
	if hr != com.S_OK || !selected(n) {
		return hr
	}
	return UIA_E_INVALIDOPERATION
}

func selectionItemIsSelected(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	setBool(ret, selected(n))
	return com.S_OK
}

func selectionItemSelectionContainer(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, _, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	b := p.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if id, ok := b.snap.selectionContainer(p.id); ok {
		*ret = b.simpleLocked(id)
	}
	return com.S_OK
}

// IExpandCollapseProvider

// expandOrCollapse asks for action, or for a tap where the node only has
// that, as a disclosure button does.
func expandOrCollapse(this *com.Unknown, action semantics.Actions, from semantics.State) uintptr {
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	switch {
	case n.Actions&action != 0:
		return act(this, platform.Event{Action: action})
	case n.Actions&semantics.ActionTap != 0 && n.State&from != 0:
		return act(this, platform.Event{Action: semantics.ActionTap})
	case n.State&from == 0:
		// Already there.
		return com.S_OK
	}
	return UIA_E_INVALIDOPERATION
}

func expandCollapseExpand(this *com.Unknown) uintptr {
	return expandOrCollapse(this, semantics.ActionExpand, semantics.Collapsed)
}

func expandCollapseCollapse(this *com.Unknown) uintptr {
	return expandOrCollapse(this, semantics.ActionCollapse, semantics.Expanded)
}

func expandCollapseGetState(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = expandCollapseState(n)
	return com.S_OK
}

// IScrollItemProvider

func scrollItemScrollIntoView(this *com.Unknown) uintptr {
	return act(this, platform.Event{Action: semantics.ActionScrollIntoView})
}
