package win32

import (
	"unsafe"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The minimal Text pattern of a text field (ADR 0042 §4): the whole text, the
// selection and the caret, moved by character, word and line. The movement
// itself is in uiatext.go.

// textRange is the Go side of an ITextRangeProvider: a span of the text of a
// field, in UTF-16 code units. It names the field by ID, so it outlives
// trees, and its ends are clamped to the text of the latest one.
type textRange struct {
	b          *bridge
	id         semantics.ID
	start, end int
}

func (b *bridge) newRange(id semantics.ID, start, end int) *com.Unknown {
	return com.New(&textRange{b: b, id: id, start: start, end: end}, b.ifaces.textRange)
}

// lookupField returns the provider behind a Text pattern and its text.
func lookupField(this *com.Unknown) (*provider, *semantics.Node, fieldText, uintptr) {
	p, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return nil, nil, fieldText{}, hr
	}
	if n.Text == nil {
		return nil, nil, fieldText{}, UIA_E_ELEMENTNOTAVAILABLE
	}
	return p, n, newFieldText(n.Text), com.S_OK
}

// lookupRange returns the range behind this, its field and its clamped ends.
func lookupRange(this *com.Unknown) (*textRange, *semantics.Node, fieldText, uintptr) {
	r, ok := com.Get[*textRange](this)
	if !ok {
		return nil, nil, fieldText{}, UIA_E_ELEMENTNOTAVAILABLE
	}
	r.b.mu.Lock()
	defer r.b.mu.Unlock()
	n := r.b.snap.node(r.id)
	if r.b.closed || r.id == 0 || n == nil || n.Text == nil {
		return nil, nil, fieldText{}, UIA_E_ELEMENTNOTAVAILABLE
	}
	t := newFieldText(n.Text)
	r.start, r.end = t.clamp(r.start, r.end)
	return r, n, t, com.S_OK
}

// otherRange returns the range of the same field that a method received as an
// argument, or nil if it is another field's or not ours.
func otherRange(r *textRange, u *com.Unknown) *textRange {
	if !com.Owns(u, r.b.ifaces.textRange) {
		return nil
	}
	o, ok := com.Get[*textRange](u)
	if !ok || o.b != r.b || o.id != r.id {
		return nil
	}
	return o
}

func (r *textRange) endpoint(e uintptr) int {
	if int32(e) == TextPatternRangeEndpoint_End {
		return r.end
	}
	return r.start
}

func ordered(a, b int) (int, int) {
	if a > b {
		return b, a
	}
	return a, b
}

// ITextProvider and ITextProvider2

func textGetSelection(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, n, t, hr := lookupField(this)
	if hr != com.S_OK {
		return hr
	}
	start, end := t.clamp(ordered(n.Text.SelectionStart, n.Text.SelectionEnd))
	*ret = com.NewSafeArrayUnknown([]*com.Unknown{p.b.newRange(n.ID, start, end)})
	return com.S_OK
}

func textGetVisibleRanges(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, n, t, hr := lookupField(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = com.NewSafeArrayUnknown([]*com.Unknown{p.b.newRange(n.ID, 0, len(t.s))})
	return com.S_OK
}

// textRangeFromChild fails: a field holds no elements.
func textRangeFromChild(_, _ *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	return com.E_INVALIDARG
}

// rangeFromPoint answers ITextProvider::RangeFromPoint for a point in physical
// pixels of the screen: an empty range at the start of the nearest line,
// since the field does not tell where in a line an offset is.
func rangeFromPoint(this *com.Unknown, x, y float64, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, n, t, hr := lookupField(this)
	if hr != com.S_OK {
		return hr
	}
	off := t.lineAt(p.b.fromScreen(x, y).Y)
	*ret = p.b.newRange(n.ID, off, off)
	return com.S_OK
}

func textDocumentRange(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, n, t, hr := lookupField(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = p.b.newRange(n.ID, 0, len(t.s))
	return com.S_OK
}

func textSupportedTextSelection(_ *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = SupportedTextSelection_Single
	return com.S_OK
}

func textRangeFromAnnotation(_, _ *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	return com.E_NOTIMPL
}

// textGetCaretRange returns an empty range at the moving end of the
// selection. The caret is active while the field has the keyboard focus.
func textGetCaretRange(this *com.Unknown, active *int32, ret **com.Unknown) uintptr {
	if ret == nil || active == nil {
		return com.E_POINTER
	}
	*ret = nil
	p, n, t, hr := lookupField(this)
	if hr != com.S_OK {
		return hr
	}
	setBool(active, p.b.focused(n.ID))
	caret, _ := t.clamp(n.Text.SelectionEnd, n.Text.SelectionEnd)
	*ret = p.b.newRange(n.ID, caret, caret)
	return com.S_OK
}

// ITextRangeProvider

func rangeClone(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	r, _, _, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = r.b.newRange(r.id, r.start, r.end)
	return com.S_OK
}

func rangeCompare(this, other *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 0
	r, _, _, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	o := otherRange(r, other)
	setBool(ret, o != nil && o.start == r.start && o.end == r.end)
	return com.S_OK
}

func rangeCompareEndpoints(this *com.Unknown, endpoint uintptr, target *com.Unknown, targetEndpoint uintptr, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 0
	r, _, _, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	o := otherRange(r, target)
	if o == nil {
		return com.E_INVALIDARG
	}
	*ret = int32(r.endpoint(endpoint) - o.endpoint(targetEndpoint))
	return com.S_OK
}

func rangeExpandToEnclosingUnit(this *com.Unknown, unit uintptr) uintptr {
	r, _, t, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	r.b.mu.Lock()
	r.start, r.end = t.expand(int32(unit), r.start)
	r.b.mu.Unlock()
	return com.S_OK
}

// rangeFindAttribute finds nothing: the text has no attributes. The VARIANT
// comes by reference, as larger arguments do.
func rangeFindAttribute(_ *com.Unknown, _ uintptr, _ *com.VARIANT, _ uintptr, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	return com.S_OK
}

func rangeFindText(_ *com.Unknown, _ *uint16, _, _ uintptr, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	return com.S_OK
}

func rangeGetAttributeValue(this *com.Unknown, _ uintptr, ret *com.VARIANT) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	ret.SetEmpty()
	if _, _, _, hr := lookupRange(this); hr != com.S_OK {
		return hr
	}
	var unsupported *com.Unknown
	// The reserved value is a static object of UI Automation; reference
	// counting does not apply to it.
	if hr, _, _ := procUiaGetReservedNotSupportedValue.Call(addrOf(&unsupported)); com.Failed(hr) {
		return hr
	}
	ret.SetUnknown(unsupported)
	return com.S_OK
}

func rangeGetBoundingRectangles(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	r, _, t, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	rects := t.lineRects(r.start, r.end)
	values := make([]float64, 0, 4*len(rects))
	for _, rect := range rects {
		s := r.b.toScreen(rect)
		values = append(values, s.left, s.top, s.width, s.height)
	}
	*ret = com.NewSafeArrayR8(values)
	return com.S_OK
}

func rangeGetEnclosingElement(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	r, _, _, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = r.b.provider(r.id)
	return com.S_OK
}

func rangeGetText(this *com.Unknown, maxLength uintptr, ret **uint16) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	r, _, t, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	*ret = com.SysAllocString(t.text(r.start, r.end, int(int32(maxLength))))
	return com.S_OK
}

func rangeMove(this *com.Unknown, unit, count uintptr, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 0
	r, _, t, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	r.b.mu.Lock()
	var moved int
	r.start, r.end, moved = t.move(int32(unit), int(int32(count)), r.start, r.end)
	r.b.mu.Unlock()
	*ret = int32(moved)
	return com.S_OK
}

func rangeMoveEndpointByUnit(this *com.Unknown, endpoint, unit, count uintptr, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 0
	r, _, t, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	r.b.mu.Lock()
	var moved int
	r.start, r.end, moved = t.moveEndpoint(int32(endpoint), int32(unit), int(int32(count)), r.start, r.end)
	r.b.mu.Unlock()
	*ret = int32(moved)
	return com.S_OK
}

func rangeMoveEndpointByRange(this *com.Unknown, endpoint uintptr, target *com.Unknown, targetEndpoint uintptr) uintptr {
	r, _, _, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	o := otherRange(r, target)
	if o == nil {
		return com.E_INVALIDARG
	}
	r.b.mu.Lock()
	r.start, r.end = setEndpoint(int32(endpoint), o.endpoint(targetEndpoint), r.start, r.end)
	r.b.mu.Unlock()
	return com.S_OK
}

func rangeSelect(this *com.Unknown) uintptr {
	r, n, _, hr := lookupRange(this)
	if hr != com.S_OK {
		return hr
	}
	if n.Actions&semantics.ActionSetSelection == 0 {
		return UIA_E_INVALIDOPERATION
	}
	if !enabled(n) {
		return UIA_E_ELEMENTNOTENABLED
	}
	r.b.post(platform.Event{Node: r.id, Action: semantics.ActionSetSelection,
		SelectionStart: r.start, SelectionEnd: r.end})
	return com.S_OK
}

// rangeAddToSelection fails: a field has a single selection.
func rangeAddToSelection(*com.Unknown) uintptr { return UIA_E_INVALIDOPERATION }

func rangeRemoveFromSelection(*com.Unknown) uintptr { return UIA_E_INVALIDOPERATION }

func rangeScrollIntoView(this *com.Unknown, _ uintptr) uintptr {
	_, _, _, hr := lookupRange(this)
	return hr
}

func rangeGetChildren(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	if _, _, _, hr := lookupRange(this); hr != com.S_OK {
		return hr
	}
	*ret = com.NewSafeArrayUnknown(nil)
	return com.S_OK
}

// addrOf returns the address of an out parameter for a syscall, which writes
// it during the call only.
func addrOf(p **com.Unknown) uintptr { return uintptr(unsafe.Pointer(p)) }
