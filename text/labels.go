package text

import "github.com/bankai-tensei/gowaku/geom"

// maxLabels bounds a Labels: past it, the next new label starts it afresh.
const maxLabels = 256

// Labels lays out short texts on one line each and keeps them, for a
// layout.Painter that draws labels it knows only while it paints, such as the
// numbers of a ruler or the ticks of a chart that depend on the size. Keep it
// in the State of the widget, set Shaper (core.DependOn[*text.Shaper](ctx))
// and Style in Build, and hand the painter a pointer to it; the painter draws
// Get(content) with paint.Canvas.DrawText.
//
// Get shapes a text the first time it sees it, which allocates, and returns
// the same Layout after that, which does not. A painter that draws the same
// labels frame after frame therefore allocates nothing. For a number it writes
// itself, it appends the digits to a []byte it keeps (strconv.AppendInt) and
// calls GetBytes, which looks the bytes up without making a string. A label
// known in Build is laid out there just as well, with
// Shaper.Shape(content, st).Layout(geom.Inf).
//
// Labels keeps at most 256 texts and forgets them all when a new one comes
// past that, or when Shaper or Style changes. A Layout holds no color, so a
// change of Style.Color alone keeps them: DrawText takes the color. Like
// everything of a Shaper, Labels belongs to the UI goroutine. The zero value
// has no Shaper, and Get panics.
type Labels struct {
	Shaper *Shaper
	Style  Style

	// shaper and style are what the kept layouts were made with.
	shaper *Shaper
	style  Style
	kept   map[string]*Layout
}

// Get returns content laid out at an infinite length, so that each block
// takes one line.
func (ls *Labels) Get(content string) *Layout {
	ls.check()
	if l, ok := ls.kept[content]; ok {
		return l
	}
	return ls.add(content)
}

// GetBytes is Get for text in a byte slice, which it does not keep.
func (ls *Labels) GetBytes(content []byte) *Layout {
	ls.check()
	if l, ok := ls.kept[string(content)]; ok {
		return l
	}
	return ls.add(string(content))
}

// check forgets the kept layouts when Shaper or Style has changed.
func (ls *Labels) check() {
	if ls.Shaper == nil {
		panic("gowaku: text.Labels.Get without a Shaper; set Labels.Shaper in Build")
	}
	st := ls.Style
	st.Color = geom.Color{}
	if ls.shaper != ls.Shaper || ls.style != st {
		clear(ls.kept)
		ls.shaper, ls.style = ls.Shaper, st
	}
}

func (ls *Labels) add(content string) *Layout {
	if ls.kept == nil {
		ls.kept = make(map[string]*Layout)
	} else if len(ls.kept) >= maxLabels {
		clear(ls.kept)
	}
	l := ls.Shaper.Shape(content, ls.style).Layout(geom.Inf)
	ls.kept[content] = l
	return l
}
