package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text"
)

// SelectableText is a render object whose text a field.SelectionArea above it
// selects and copies. RenderParagraph, the object of Text and RichText, is
// one; a widget that sets its own text, such as the paragraphs of the
// markdown module, implements it to take part in the selection of an area.
// The area finds the objects in the order they are painted, does its work at
// event time, and only calls SetSelection and ClearSelection on the objects
// it marks, so an object outside any area costs nothing more than the nil
// check its Paint makes. See docs/ADR/0089-selection-area.md and
// docs/ADR/0105-markdown-selection.md.
type SelectableText interface {
	core.RenderObject
	// Text returns the text that TextLayout sets: what a copy takes.
	Text() string
	// TextLayout returns the result of the last layout, or nil, which keeps
	// the object out of the selection: before its first layout, and for text
	// that is not meant to be selected, such as the number of a list item.
	TextLayout() *text.Layout
	// TextOrigin returns where the origin of TextLayout lies in the object:
	// zero unless the object moves its text, as to align it in a wider box.
	TextOrigin() geom.Offset
	// SetSelection highlights the bytes of Text from start to end, in either
	// order, in color; ClearSelection removes it, and Selection returns it
	// in order, or 0, 0. They behave as the methods of RenderParagraph.
	SetSelection(start, end int, color geom.Color)
	ClearSelection()
	Selection() (start, end int)
	// SelectionBreak returns what a copy puts between the text of the object
	// before this one and this one's: "\n" for RenderParagraph. The markdown
	// module uses it for blank lines between blocks and tabs between cells.
	SelectionBreak() string
}

var _ SelectableText = (*RenderParagraph)(nil)

// TextOrigin implements SelectableText: a paragraph sets its text at its
// origin.
func (r *RenderParagraph) TextOrigin() geom.Offset { return geom.Offset{} }

// SelectionBreak implements SelectableText: a copy puts a line break between
// paragraphs.
func (r *RenderParagraph) SelectionBreak() string { return "\n" }
