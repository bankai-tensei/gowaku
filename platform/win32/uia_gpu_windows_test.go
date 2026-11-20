//go:build gpu

package win32

import (
	"errors"
	"math"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This drives the provider with the real UI Automation client, in process:
// the window thread pumps messages while a second OS thread asks questions,
// as a screen reader would. It shows a window, so it runs with the GPU tests,
// locally only.

// The client side of UIAutomationClient.h.
var (
	CLSID_CUIAutomation            = mustGUID("{ff48dba4-60ef-4201-aa87-54103eef594e}")
	IID_IUIAutomation              = mustGUID("{30cbe57d-d9d0-452a-ab13-7ac5ac4825ee}")
	IID_IUIAutomationInvokePattern = mustGUID("{fb377fbe-8ea6-46d5-9c73-6499642d3059}")
	IID_IUIAutomationRangeValue    = mustGUID("{59213f4f-7346-49e5-b120-80555987a148}")
	IID_IUIAutomationTextPattern   = mustGUID("{32eba289-3583-42c9-9c59-3b6d9a1e9b6a}")
)

const (
	iUIAutomation_ElementFromHandle          = 6  // IUIAutomation::ElementFromHandle
	iUIAutomation_CreateTrueCondition        = 21 // IUIAutomation::CreateTrueCondition
	iUIAutomationElement_FindAll             = 6  // IUIAutomationElement::FindAll
	iUIAutomationElement_GetCurrentPropValue = 10 // IUIAutomationElement::GetCurrentPropertyValue
	iUIAutomationElement_GetCurrentPatternAs = 14 // IUIAutomationElement::GetCurrentPatternAs
	iUIAutomationElement_CurrentControlType  = 21 // IUIAutomationElement::get_CurrentControlType
	iUIAutomationElement_CurrentName         = 23 // IUIAutomationElement::get_CurrentName
	iUIAutomationElementArray_Length         = 3  // IUIAutomationElementArray::get_Length
	iUIAutomationElementArray_GetElement     = 4  // IUIAutomationElementArray::GetElement
	iUIAutomationInvokePattern_Invoke        = 3  // IUIAutomationInvokePattern::Invoke
	iUIAutomationRangeValuePattern_SetValue  = 3  // IUIAutomationRangeValuePattern::SetValue
	iUIAutomationTextPattern_GetSelection    = 5  // IUIAutomationTextPattern::GetSelection
	iUIAutomationTextPattern_DocumentRange   = 7  // IUIAutomationTextPattern::get_DocumentRange
	iUIAutomationTextRangeArray_Length       = 3  // IUIAutomationTextRangeArray::get_Length
	iUIAutomationTextRangeArray_GetElement   = 4  // IUIAutomationTextRangeArray::GetElement
	iUIAutomationTextRange_ExpandToEnclosing = 6  // IUIAutomationTextRange::ExpandToEnclosingUnit
	iUIAutomationTextRange_GetText           = 12 // IUIAutomationTextRange::GetText
	iUIAutomationTextRange_Move              = 13 // IUIAutomationTextRange::Move

	TreeScope_Descendants = 4
	COINIT_MULTITHREADED  = 0x0
)

func uiaTestTree(saveLabel string, withCheckbox bool) *semantics.Node {
	text := "こんにちは world"
	root := &semantics.Node{Rect: geom.Rect{W: 400, H: 300}}
	root.Children = append(root.Children, &semantics.Node{
		ID: 1, Role: semantics.RoleButton, Label: saveLabel, State: semantics.Focusable,
		Actions: semantics.ActionTap | semantics.ActionFocus, Rect: geom.Rect{X: 10, Y: 10, W: 100, H: 30},
	})
	if withCheckbox {
		root.Children = append(root.Children, &semantics.Node{
			ID: 2, Role: semantics.RoleCheckbox, Label: "Done", State: semantics.Checked | semantics.Focusable,
			Actions: semantics.ActionTap, Rect: geom.Rect{X: 10, Y: 50, W: 100, H: 30},
		})
	}
	root.Children = append(root.Children,
		&semantics.Node{
			ID: 3, Role: semantics.RoleSlider, Label: "Volume", Value: "40",
			Range:   semantics.Range{Min: 0, Max: 100, Value: 40, Step: 5},
			Actions: semantics.ActionIncrease | semantics.ActionDecrease | semantics.ActionSetValue,
			Rect:    geom.Rect{X: 10, Y: 90, W: 200, H: 30},
		},
		&semantics.Node{
			ID: 4, Role: semantics.RoleTextField, Label: "Name", Value: text,
			State:   semantics.Focusable | semantics.Focused,
			Actions: semantics.ActionSetValue | semantics.ActionSetSelection,
			Text: &semantics.TextInfo{Text: text, SelectionStart: 5, SelectionEnd: 5,
				LineRects: []geom.Rect{{X: 10, Y: 130, W: 200, H: 24}}},
			Rect: geom.Rect{X: 10, Y: 130, W: 200, H: 24},
		})
	return root
}

func TestUIA_ClientSeesTreeAndActs(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	w, err := p.CreateWindow(platform.WindowOptions{Title: "gowaku uia test", Width: 400, Height: 300})
	if err != nil {
		t.Fatal(err)
	}
	a := w.Accessibility()
	if a == nil {
		t.Fatal("Accessibility() = nil")
	}
	hwnd := w.(*Window).hwnd

	var spoken *semantics.Node
	var diff semantics.Diff
	update := func(root *semantics.Node, focus semantics.ID) {
		semantics.Compare(spoken, root, 4, focus, &diff)
		spoken = root
		a.Update(root, focus, &diff)
	}

	ready := make(chan struct{})
	actions := make(chan platform.Event, 16)
	onUI := make(chan func())
	done := make(chan struct{})
	go func() {
		defer close(done)
		// The label changes and the checkbox goes away.
		second := func() { update(uiaTestTree("Saved", false), 4) }
		runClient(t, hwnd, ready, actions, onUI, second)
	}()
	deadline := time.Now().Add(60 * time.Second)
	for running, sawRedraw := true, false; running; {
		if time.Now().After(deadline) {
			t.Fatal("the client did not finish")
		}
		switch ev := p.WaitEvent(5 * time.Millisecond); ev.Kind {
		case platform.EventAccessibilityAction:
			actions <- ev
		case platform.EventRedraw:
			sawRedraw = true
		}
		if a.Listening() && spoken == nil {
			if !sawRedraw {
				continue
			}
			update(uiaTestTree("Save", true), 4)
			close(ready)
		}
		select {
		case fn := <-onUI:
			fn()
		case <-done:
			running = false
		default:
		}
	}
}

// runClient is the screen reader. It runs on an OS thread of its own in the
// multithreaded apartment; every call it makes is answered by the window
// thread, which keeps pumping.
func runClient(t *testing.T, hwnd HWND, ready <-chan struct{}, actions <-chan platform.Event, onUI chan<- func(), second func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, COINIT_MULTITHREADED); err != nil && !errors.Is(err, syscall.Errno(1)) {
		t.Errorf("CoInitializeEx: %v", err)
		return
	}
	defer windows.CoUninitialize()

	var uia *com.Unknown
	if hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&CLSID_CUIAutomation)), 0, CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&IID_IUIAutomation)), uintptr(unsafe.Pointer(&uia))); com.Failed(hr) {
		t.Errorf("CoCreateInstance(CUIAutomation): %#x", uint32(hr))
		return
	}
	defer uia.Release()

	var window *com.Unknown
	if hr := uia.Call(iUIAutomation_ElementFromHandle, uintptr(hwnd), uintptr(unsafe.Pointer(&window))); com.Failed(hr) {
		t.Errorf("ElementFromHandle: %#x", uint32(hr))
		return
	}
	defer window.Release()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Error("the window never started listening")
		return
	}

	// The title bar and its buttons come from the OS; the rest is gowaku's.
	var elements []*com.Unknown
	for _, e := range findAll(t, uia, window) {
		var v com.VARIANT
		e.Call(iUIAutomationElement_GetCurrentPropValue, UIA_FrameworkIdPropertyId, uintptr(unsafe.Pointer(&v)))
		if v.String() == uiaFrameworkID {
			elements = append(elements, e)
		} else {
			e.Release()
		}
		v.Clear()
	}
	defer func() {
		for _, e := range elements {
			e.Release()
		}
	}()
	want := []struct {
		name string
		typ  int32
	}{
		{"Save", UIA_ButtonControlTypeId},
		{"Done", UIA_CheckBoxControlTypeId},
		{"Volume", UIA_SliderControlTypeId},
		{"Name", UIA_EditControlTypeId},
	}
	if len(elements) != len(want) {
		for _, e := range elements {
			t.Logf("element %q type %d", elementName(e), elementType(e))
		}
		t.Errorf("found %d elements, want %d", len(elements), len(want))
		return
	}
	for i, e := range elements {
		name, typ := elementName(e), elementType(e)
		if name != want[i].name || typ != want[i].typ {
			t.Errorf("element %d: %q type %d, want %q type %d", i, name, typ, want[i].name, want[i].typ)
		}
	}
	save, done, volume, field := elements[0], elements[1], elements[2], elements[3]

	// Invoke comes back as an action event.
	invoke := pattern(t, save, UIA_InvokePatternId, &IID_IUIAutomationInvokePattern)
	if invoke != nil {
		if hr := invoke.Call(iUIAutomationInvokePattern_Invoke); com.Failed(hr) {
			t.Errorf("Invoke: %#x", uint32(hr))
		}
		invoke.Release()
		expectAction(t, actions, platform.Event{Node: 1, Action: semantics.ActionTap})
	}

	if v := property(t, done, UIA_ToggleToggleStatePropertyId); v.VT != com.VT_I4 || v.I4() != ToggleState_On {
		t.Errorf("ToggleState: type %d value %d, want On", v.VT, v.I4())
	}
	if v := property(t, volume, UIA_RangeValueValuePropertyId); v.VT != com.VT_R8 || v.R8() != 40 {
		t.Errorf("RangeValue: type %d value %v, want 40", v.VT, v.R8())
	}
	if v := property(t, field, UIA_ValueValuePropertyId); v.String() != "こんにちは world" {
		t.Errorf("Value: %q", v.String())
	}

	// RangeValue.SetValue takes a double, which reaches the provider through
	// its float thunk.
	if rv := pattern(t, volume, UIA_RangeValuePatternId, &IID_IUIAutomationRangeValue); rv != nil {
		vtbl := unsafe.Slice(*(**uintptr)(unsafe.Pointer(rv)), iUIAutomationRangeValuePattern_SetValue+1)
		caller, err := com.NewFloatCaller(vtbl[iUIAutomationRangeValuePattern_SetValue], 1)
		if err != nil {
			t.Error(err)
		} else if hr, _, _ := syscall.SyscallN(caller, rv.Addr(), uintptr(math.Float64bits(75))); com.Failed(hr) {
			t.Errorf("RangeValue.SetValue: %#x", uint32(hr))
		} else {
			expectAction(t, actions, platform.Event{Node: 3, Action: semantics.ActionSetValue, Number: 75})
		}
		rv.Release()
	}

	checkText(t, field)

	// A new tree, handed over on the window thread between messages.
	updated := make(chan struct{})
	onUI <- func() {
		second()
		close(updated)
	}
	<-updated
	if name := elementName(save); name != "Saved" {
		t.Errorf("name after the update: %q", name)
	}
	var b *uint16
	if hr := done.Call(iUIAutomationElement_CurrentName, uintptr(unsafe.Pointer(&b))); uint32(hr) != UIA_E_ELEMENTNOTAVAILABLE {
		t.Errorf("a removed element answered %#x, want UIA_E_ELEMENTNOTAVAILABLE", uint32(hr))
	}
	com.SysFreeString(b)
}

func checkText(t *testing.T, field *com.Unknown) {
	t.Helper()
	text := pattern(t, field, UIA_TextPatternId, &IID_IUIAutomationTextPattern)
	if text == nil {
		return
	}
	defer text.Release()
	var doc *com.Unknown
	if hr := text.Call(iUIAutomationTextPattern_DocumentRange, uintptr(unsafe.Pointer(&doc))); com.Failed(hr) || doc == nil {
		t.Errorf("DocumentRange: %#x", uint32(hr))
		return
	}
	defer doc.Release()
	if got := rangeText(doc); got != "こんにちは world" {
		t.Errorf("document text %q", got)
	}
	if hr := doc.Call(iUIAutomationTextRange_ExpandToEnclosing, TextUnit_Word); com.Failed(hr) {
		t.Errorf("ExpandToEnclosingUnit: %#x", uint32(hr))
	}
	if got := rangeText(doc); got != "こんにちは " {
		t.Errorf("first word %q", got)
	}
	var moved int32
	if hr := doc.Call(iUIAutomationTextRange_Move, TextUnit_Word, 1, uintptr(unsafe.Pointer(&moved))); com.Failed(hr) || moved != 1 {
		t.Errorf("Move by a word: %#x, moved %d", uint32(hr), moved)
	}
	if got := rangeText(doc); got != "world" {
		t.Errorf("second word %q", got)
	}

	var sel *com.Unknown
	if hr := text.Call(iUIAutomationTextPattern_GetSelection, uintptr(unsafe.Pointer(&sel))); com.Failed(hr) || sel == nil {
		t.Errorf("GetSelection: %#x", uint32(hr))
		return
	}
	defer sel.Release()
	var n int32
	sel.Call(iUIAutomationTextRangeArray_Length, uintptr(unsafe.Pointer(&n)))
	if n != 1 {
		t.Errorf("selection has %d ranges", n)
		return
	}
	var caret *com.Unknown
	sel.Call(iUIAutomationTextRangeArray_GetElement, 0, uintptr(unsafe.Pointer(&caret)))
	defer caret.Release()
	if hr := caret.Call(iUIAutomationTextRange_Move, TextUnit_Character, ^uintptr(1), uintptr(unsafe.Pointer(&moved))); com.Failed(hr) || moved != -2 {
		t.Errorf("caret back two characters: %#x, moved %d", uint32(hr), moved)
	}
	if hr := caret.Call(iUIAutomationTextRange_ExpandToEnclosing, TextUnit_Character); com.Failed(hr) {
		t.Errorf("ExpandToEnclosingUnit: %#x", uint32(hr))
	}
	if got := rangeText(caret); got != "ち" {
		t.Errorf("character after the caret moved back by two: %q", got)
	}
}

func findAll(t *testing.T, uia, root *com.Unknown) []*com.Unknown {
	t.Helper()
	var cond, arr *com.Unknown
	if hr := uia.Call(iUIAutomation_CreateTrueCondition, uintptr(unsafe.Pointer(&cond))); com.Failed(hr) {
		t.Errorf("CreateTrueCondition: %#x", uint32(hr))
		return nil
	}
	defer cond.Release()
	if hr := root.Call(iUIAutomationElement_FindAll, TreeScope_Descendants, cond.Addr(), uintptr(unsafe.Pointer(&arr))); com.Failed(hr) || arr == nil {
		t.Errorf("FindAll: %#x", uint32(hr))
		return nil
	}
	defer arr.Release()
	var n int32
	arr.Call(iUIAutomationElementArray_Length, uintptr(unsafe.Pointer(&n)))
	out := make([]*com.Unknown, n)
	for i := range out {
		arr.Call(iUIAutomationElementArray_GetElement, uintptr(i), uintptr(unsafe.Pointer(&out[i])))
	}
	return out
}

func elementName(e *com.Unknown) string {
	var b *uint16
	if hr := e.Call(iUIAutomationElement_CurrentName, uintptr(unsafe.Pointer(&b))); com.Failed(hr) {
		return ""
	}
	defer com.SysFreeString(b)
	return com.BSTRToString(b)
}

func elementType(e *com.Unknown) int32 {
	var typ int32
	e.Call(iUIAutomationElement_CurrentControlType, uintptr(unsafe.Pointer(&typ)))
	return typ
}

// property returns a property of e. Its BSTR is freed by the next call of the
// test, which is fine for comparing right away.
func property(t *testing.T, e *com.Unknown, prop int32) *com.VARIANT {
	t.Helper()
	v := new(com.VARIANT)
	if hr := e.Call(iUIAutomationElement_GetCurrentPropValue, uintptr(prop), uintptr(unsafe.Pointer(v))); com.Failed(hr) {
		t.Errorf("property %d: %#x", prop, uint32(hr))
	}
	t.Cleanup(v.Clear)
	return v
}

func pattern(t *testing.T, e *com.Unknown, id int32, iid *windows.GUID) *com.Unknown {
	t.Helper()
	var p *com.Unknown
	if hr := e.Call(iUIAutomationElement_GetCurrentPatternAs, uintptr(id), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&p))); com.Failed(hr) || p == nil {
		t.Errorf("pattern %d: %#x", id, uint32(hr))
		return nil
	}
	return p
}

func rangeText(r *com.Unknown) string {
	var b *uint16
	if hr := r.Call(iUIAutomationTextRange_GetText, ^uintptr(0), uintptr(unsafe.Pointer(&b))); com.Failed(hr) {
		return "<error>"
	}
	defer com.SysFreeString(b)
	return com.BSTRToString(b)
}

// expectAction waits for an action like want. Other actions may come first:
// UI Automation focuses an element before it invokes it, for one.
func expectAction(t *testing.T, actions <-chan platform.Event, want platform.Event) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-actions:
			if ev.Node == want.Node && ev.Action == want.Action && ev.Number == want.Number && ev.Window != 0 {
				return
			}
			t.Logf("other action first: node %d action %b", ev.Node, ev.Action)
		case <-timeout:
			t.Errorf("no action %b for node %d", want.Action, want.Node)
			return
		}
	}
}
