package kit

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// Itoa spells n in decimal.
func Itoa(n int) string { return strconv.Itoa(n) }

// Pad2 spells n in decimal with at least two digits.
func Pad2(n int) string {
	if n < 10 {
		return "0" + Itoa(n)
	}
	return Itoa(n)
}

// FiniteOr returns v, or fallback where v is unbounded.
func FiniteOr(v, fallback float32) float32 {
	if v >= geom.Inf {
		return fallback
	}
	return v
}

// PercentRange is the number of a progress indicator, from 0 to 100.
func PercentRange(v float32) semantics.Range {
	// Rounded to a hundredth of a percent: 0.3 as a float32 is 30.0000019 in a
	// float64, which a reader would say.
	return semantics.Range{Max: 100, Value: math.Round(float64(geom.Clamp32(v, 0, 1))*10000) / 100}
}

// Percent spells a share for assistive technology.
func Percent(v float32) string {
	return strconv.Itoa(int(geom.Clamp32(v, 0, 1)*100+0.5)) + "%"
}

// Utf16Len returns the length of s in UTF-16 code units.
func Utf16Len(s string) int {
	n := 0
	for _, c := range s {
		n += utf16.RuneLen(c)
	}
	return n
}

// ByteOffset returns the offset in s of the UTF-16 offset u, kept to the start
// of a character and within s.
func ByteOffset(s string, u int) int {
	n := 0
	for i, c := range s {
		if n >= u {
			return i
		}
		n += utf16.RuneLen(c)
	}
	return len(s)
}

// Themed fills in what a style leaves open from the theme, if there is one: the
// body text in ink.
func Themed(ctx core.BuildContext, s text.Style) text.Style {
	th, ok := core.DependOn[*theme.Theme](ctx)
	if !ok {
		return s
	}
	if s.Color == (geom.Color{}) {
		s.Color = th.Color.Ink
	}
	if s.Size == 0 {
		s.Size = th.Text.Body.Size
		if s.LineHeight == 0 {
			s.LineHeight = th.Text.Body.LineHeight
		}
	}
	return s
}

// TextWidth returns the width of s on one line.
func TextWidth(shaper *text.Shaper, s string, st text.Style) float32 {
	return shaper.Shape(s, st).Layout(geom.Inf).Size().W
}

// CrossPath returns a × with arms of length arm and width width, in the middle
// of a square of side box.
func CrossPath(box, arm, width float32) *paint.Path {
	lo, hi := (box-arm)/2, (box+arm)/2
	var p paint.Path
	p.MoveTo(lo, lo)
	p.LineTo(hi, hi)
	p.MoveTo(hi, lo)
	p.LineTo(lo, hi)
	return p.Stroke(paint.StrokeStyle{Width: width, Cap: paint.CapRound})
}

// GlobalRect returns the box of ro in the window.
func GlobalRect(ro core.RenderObject) geom.Rect {
	sz := ro.Size()
	return core.LocalRectToGlobal(ro, geom.Rect{W: sz.W, H: sz.H})
}

// FocusOf returns what has the focus in the tree of ctx, or nil.
func FocusOf(ctx core.BuildContext) core.FocusTarget {
	if f, ok := ctx.RenderObject().(interface{ Focused() core.FocusTarget }); ok {
		return f.Focused()
	}
	return nil
}

// IsContextMenuKey reports whether e asks for a context menu from the
// keyboard: the menu key, or Shift+F10.
func IsContextMenuKey(e *core.KeyEvent) bool {
	return e.Key == input.KeyContextMenu || (e.Key == input.KeyF10 && e.Modifiers == input.ModShift)
}

// FocusFrom returns what has the focus in the tree of ro, or nil.
func FocusFrom(ro core.RenderObject) core.RenderObject {
	if f, ok := ro.(interface{ Focused() core.FocusTarget }); ok {
		if t := f.Focused(); t != nil {
			return t
		}
	}
	return nil
}

// FirstFocusable returns the first object at or below ro that takes the focus
// now, or nil.
func FirstFocusable(ro core.RenderObject) core.FocusTarget {
	if ro == nil {
		return nil
	}
	if f, ok := ro.(core.FocusTarget); ok && f.CanFocus() {
		return f
	}
	for i := range ro.ChildCount() {
		if f := FirstFocusable(ro.ChildAt(i)); f != nil {
			return f
		}
	}
	return nil
}

// OverlayRect turns a rectangle in the window into one in the overlay whose
// render object is overlay.
func OverlayRect(overlay core.RenderObject, r geom.Rect) geom.Rect {
	if overlay != nil {
		at := core.LocalToGlobal(overlay, geom.Offset{})
		r.X, r.Y = r.X-at.X, r.Y-at.Y
	}
	return r
}

// PopupRows returns how many rows of a list of choices fit beside anchor in
// the overlay whose render object is overlay, on the side with more room,
// from three up to PopupMaxRows.
func PopupRows(overlay core.RenderObject, anchor geom.Rect) int {
	if overlay == nil {
		return PopupMaxRows
	}
	below := overlay.Size().H - anchor.Y - anchor.H
	room := max(below, anchor.Y) - MenuGap - MenuEdge - 2*MenuInset
	return min(max(int(room/MenuItemHeight), 3), PopupMaxRows)
}

// Mnemonic is a label with its & taken out.
type Mnemonic struct {
	// Text is what shows, and Key the letter after the &, or KeyNone.
	Text string
	Key  input.KeyCode
}

// ParseMnemonic takes the & out of label.
func ParseMnemonic(label string) Mnemonic {
	if !strings.Contains(label, "&") {
		return Mnemonic{Text: label}
	}
	var b strings.Builder
	var key input.KeyCode
	for i := 0; i < len(label); i++ {
		c := label[i]
		if c != '&' {
			b.WriteByte(c)
			continue
		}
		if i+1 < len(label) && label[i+1] == '&' {
			b.WriteByte('&')
			i++
			continue
		}
		if i+1 < len(label) && key == input.KeyNone {
			key = LetterKey(label[i+1])
		}
	}
	return Mnemonic{Text: b.String(), Key: key}
}

// LetterKey returns the key of a letter or digit, or KeyNone.
func LetterKey(c byte) input.KeyCode {
	switch {
	case c >= 'a' && c <= 'z':
		return input.KeyCode(c - 'a' + 'A')
	case c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return input.KeyCode(c)
	}
	return input.KeyNone
}
