package basic

import (
	"slices"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// Text shows a string, broken into lines by the rules of Style. The zero value
// shows nothing and takes the height of one line.
//
// Style.MaxLines cuts the text short with an ellipsis, for the one-line
// preview in a list row or the two-line description on a card:
//
//	basic.Text{Content: mail.Body, Style: th.Text.Body.WithMaxLines(1)}
//
// A screen reader still reads the whole text. Text is read out as a text node;
// wrap a Text that only decorates, such as the initial in an avatar, in
// interact.ExcludeSemantics to leave it out.
//
// A text.Shaper must be available through core.Inherited. Run provides one.
type Text struct {
	core.StatelessBase
	Content string
	Style   text.Style
}

// Build implements core.StatelessWidget.
func (t Text) Build(ctx core.BuildContext) core.Widget {
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: Text needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	return paragraph{shaper: shaper, content: t.Content, style: kit.Themed(ctx, t.Style)}
}

// RichText shows spans of different looks, with ruby. Style applies to the
// whole paragraph, and each span can override it. Style.MaxLines and
// interact.ExcludeSemantics work as they do for Text.
//
// A screen reader reads the text of the spans joined, which is the base text
// of ruby without the ruby: 漢字 with the ruby かんじ reads 漢字. To have the
// reading heard, give interact.Semantics a Label of its own over the RichText
// in interact.ExcludeSemantics.
//
// A span with a Background, such as a match of a search, is filled behind its
// glyphs, line by line (text.Span.Background).
type RichText struct {
	core.StatelessBase
	Spans []text.Span
	Style text.Style
}

// Build implements core.StatelessWidget.
func (t RichText) Build(ctx core.BuildContext) core.Widget {
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: RichText needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	return paragraph{shaper: shaper, spans: t.Spans, style: kit.Themed(ctx, t.Style)}
}

// paragraph is Text or RichText with its Shaper found.
type paragraph struct {
	shaper  *text.Shaper
	content string
	spans   []text.Span
	style   text.Style
}

// Key implements core.Widget.
func (paragraph) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p paragraph) CreateRenderObject() core.RenderObject {
	return &RenderParagraph{shaper: p.shaper, content: p.content, spans: p.spans, style: p.style}
}

// UpdateRenderObject implements core.RenderWidget.
func (p paragraph) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*RenderParagraph)
	if r.content != p.content || !slices.Equal(r.spans, p.spans) || !r.style.SameLayout(p.style) {
		r.content, r.spans, r.style = p.content, p.spans, p.style
		r.shaped, r.laidOut = nil, nil
		r.MarkNeedsLayout()
		return
	}
	if r.style.PaintColor() != p.style.PaintColor() {
		r.style = p.style
		r.MarkNeedsPaint()
	}
}

// RenderParagraph lays out and paints a text. It lives here and not in layout
// because layout must not import text (CODING_STYLE §3.1). It keeps the shaped
// text across layouts, so that a new width only breaks the lines again.
type RenderParagraph struct {
	core.RenderBase
	shaper  *text.Shaper
	content string
	spans   []text.Span
	style   text.Style

	shaped  *text.Paragraph
	laidOut *text.Layout
	// width is the line length of laidOut: the width, or for vertical text
	// the height.
	width float32
	// selection is set only under a selection area (text_selection.go).
	selection *paragraphSelection
}

// Layout implements core.RenderObject.
func (r *RenderParagraph) Layout(c geom.Constraints) geom.Size {
	if r.shaped == nil {
		if r.spans != nil {
			r.shaped = r.shaper.ShapeSpans(r.spans, r.style)
		} else {
			r.shaped = r.shaper.Shape(r.content, r.style)
		}
	}
	// Vertical lines run down, so the height bounds them.
	length := c.MaxW
	if r.style.Orientation == text.Vertical {
		length = c.MaxH
	}
	if r.laidOut == nil || r.width != length {
		r.laidOut, r.width = r.shaped.Layout(length), length
	}
	return c.Constrain(r.laidOut.Size())
}

// Paint implements core.RenderObject.
func (r *RenderParagraph) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, b := range r.laidOut.Backgrounds() {
		cv.DrawRect(b.Rect.Translate(off), paint.Paint{Color: b.Color})
	}
	if r.selection != nil {
		r.paintSelection(cv, off)
	}
	color := r.style.PaintColor()
	runs := r.laidOut.Runs()
	for i := range runs {
		cv.DrawGlyphs(&runs[i], off, color)
	}
}

// Semantics implements core.RenderObject. The label is the whole text, also
// when MaxLines cut what is shown short.
func (r *RenderParagraph) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleText, Label: r.shaped.Text()}, r.Size())
	sb.Close()
}

// Text returns the text, with the spans of a RichText joined.
func (r *RenderParagraph) Text() string { return r.shaped.Text() }

// Style returns the style the text is set in.
func (r *RenderParagraph) Style() text.Style { return r.style }

// Overflow returns how far the set text runs past the box at the last layout,
// along the longer of the two directions, or zero: a word longer than the line,
// or a box too short for the lines. Tools such as gowakutest report it.
func (r *RenderParagraph) Overflow() float32 {
	need, got := r.laidOut.Size(), r.Size()
	return max(need.W-got.W, need.H-got.H, 0)
}

// TextLayout returns the result of the last layout, for caret and selection
// geometry.
func (r *RenderParagraph) TextLayout() *text.Layout { return r.laidOut }
