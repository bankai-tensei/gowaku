package field

import (
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// Semantics implements core.RenderObject. The field tells a screen reader its
// text, selection and lines, so that the reader can follow the caret.
func (r *RenderEditable) Semantics(sb *core.SemanticsBuilder) {
	info := &semantics.TextInfo{
		Text:           r.Content,
		SelectionStart: kit.Utf16Len(r.Content[:r.Anchor]),
		SelectionEnd:   kit.Utf16Len(r.Content[:r.Caret]),
	}
	value := r.Content
	state := r.field.Extras.state
	if r.field.Obscure {
		// The reader gets the bullets, never the password. An obscured field
		// has one line, so the line offsets below need no mapping.
		value, info.SelectionStart, info.SelectionEnd = r.obscuredText()
		info.Text = value
		state |= semantics.Obscured
	}
	if r.field.Multiline {
		state |= semantics.Multiline
	}
	// A read-only field can be read and selected, a disabled one only read.
	actions := semantics.ActionSetValue | semantics.ActionSetSelection
	switch {
	case r.field.Disabled:
		state |= semantics.Disabled
		actions = 0
	case r.field.ReadOnly:
		state |= semantics.ReadOnly
		actions = semantics.ActionSetSelection
	}
	if r.laidOut != nil && !r.stale {
		// The reader gets the text as written, notation and all, as it gets
		// Value and as it sets it: the lines are those shown.
		l := r.shown()
		origin := r.textOrigin()
		// Under a Transform, a line is the upright box around the turned one.
		bounds := core.LocalRectToGlobal(r, geom.Rect{W: r.size.W, H: r.size.H})
		var rects []geom.Rect
		for i := range l.LineCount() {
			start, end := l.LineRange(i)
			if i > 0 {
				info.LineStarts = append(info.LineStarts, kit.Utf16Len(r.Content[:min(start, len(r.Content))]))
			}
			rects = l.SelectionRects(start, end, rects[:0])
			line := l.CaretRect(start)
			for _, rc := range rects {
				line = union(line, rc)
			}
			info.LineRects = append(info.LineRects, core.LocalRectToGlobal(r, line.Translate(origin)).Intersect(bounds))
		}
	}
	sb.Open(semantics.Node{
		Role:    semantics.RoleTextField,
		Label:   r.semanticsLabel(),
		Value:   value,
		Hint:    r.semanticsHint(),
		State:   state,
		Actions: actions,
		Text:    info,
	}, r.Size())
	sb.Close()
}

func (r *RenderEditable) semanticsLabel() string {
	if r.field.Label != "" {
		return r.field.Label
	}
	return r.field.Placeholder
}

// semanticsHint is the error of a FormItem around the field, or else the
// placeholder, when the label already names the field.
func (r *RenderEditable) semanticsHint() string {
	if r.field.Extras.err != "" {
		return r.field.Extras.err
	}
	if r.field.Label != "" {
		return r.field.Placeholder
	}
	return ""
}

// PerformSemanticsAction implements core.SemanticsActor. A screen reader can
// replace the text, as typing would, and select a part of it.
func (r *RenderEditable) PerformSemanticsAction(a semantics.Actions, arg core.SemanticsArg) bool {
	switch {
	case r.field.Disabled:
		return false
	case r.field.ReadOnly && a == semantics.ActionSetValue:
		return false
	}
	switch a {
	case semantics.ActionSetValue:
		r.replace(0, len(r.Content), arg.Text, kit.EditOther)
		return true
	case semantics.ActionSetSelection:
		if r.field.Obscure {
			r.Anchor = r.obscuredOffset(arg.SelectionStart)
			r.MoveTo(r.obscuredOffset(arg.SelectionEnd), true)
			return true
		}
		r.Anchor = kit.ByteOffset(r.Content, arg.SelectionStart)
		r.MoveTo(kit.ByteOffset(r.Content, arg.SelectionEnd), true)
		return true
	}
	return false
}

// union returns the smallest rectangle that holds a and b.
func union(a, b geom.Rect) geom.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return geom.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
