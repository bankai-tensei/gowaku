package basic

import (
	"math"

	"github.com/bankai-tensei/gowaku/layout"
)

// ScrollController reads and moves the offset of a Scroll from code. The
// offset counts from the start of the scroll range: from the top or the left,
// or with Scroll.Reverse from the bottom or the right. The zero value is
// ready; keep it where it outlives a build, such as in a State, and put it in
// Scroll.Controller.
//
// A jump asked for while no Scroll shows the controller waits for the next
// Scroll that does, so that a Scroll can open where the reader left it:
//
//	func (s *readerState) InitState() { s.scroll.JumpTo(s.saved) }
//
// A jump lands at once, as far as the content of the last layout reaches, and
// again after the next layout, which takes the content of a rebuild that has
// not run yet: JumpToEnd right after the SetState that added a line reaches
// that line.
//
// A controller belongs to one Scroll at a time. Its methods run on the UI
// goroutine.
type ScrollController struct {
	// view is the Scroll that shows the controller, or nil.
	view *scrollWidgetState
	// pending is a jump to offset that waits for a Scroll.
	pending bool
	offset  float64
}

// Offset returns how far the view is from the start of the scroll range. With
// no Scroll it returns the offset of a jump that waits for one, or where the
// last Scroll was when it let the controller go, which is what to save to
// open there again. While the wheel pulls the content past an end, it is
// below 0 or above MaxOffset.
func (c *ScrollController) Offset() float64 {
	if c.view == nil {
		return c.offset
	}
	return c.view.Offset.Value()
}

// MaxOffset returns the largest offset: how much longer the content is than
// the view, as of the last layout. It is 0 without a Scroll, before the first
// layout, and for content that fits.
func (c *ScrollController) MaxOffset() float64 {
	if c.view == nil {
		return 0
	}
	return c.view.Scroller.limit
}

// Position returns where the view is, or the zero position without a Scroll.
func (c *ScrollController) Position() ScrollPosition {
	if c.view == nil {
		return ScrollPosition{}
	}
	return c.view.Position()
}

// AtEnd reports whether a Scroll shows the controller and its view is at the
// end of the scroll range, as ScrollPosition.AtEnd tells.
func (c *ScrollController) AtEnd() bool { return c.view != nil && c.view.Position().AtEnd() }

// JumpTo moves the view at once to offset, ending any motion. An offset past
// either end goes to that end, once the Scroll has been laid out. The view
// moves again after the next layout, as the type tells. It does not count as
// the user scrolling.
func (c *ScrollController) JumpTo(offset float64) {
	offset = max(offset, 0)
	if c.view == nil {
		c.pending, c.offset = true, offset
		return
	}
	c.view.codeJump(offset)
}

// JumpToEnd moves the view at once to the end of the scroll range: the bottom,
// the right, or with Scroll.Reverse the top or the left.
func (c *ScrollController) JumpToEnd() { c.JumpTo(math.MaxFloat64) }

// codeJump moves the view to offset for code: now, within the content of the
// last layout, and again after the next layout.
func (s *ScrollState) codeJump(offset float64) {
	if s.Metrics != (layout.ScrollMetrics{}) {
		s.jumpTo(min(offset, s.Scroller.limit))
	} else {
		// The first layout brings an offset past the end back to it.
		s.jumpTo(offset)
	}
	s.replay, s.replayAt = true, offset
	if s.view != nil {
		s.view.MarkNeedsLayout()
	}
}

// attach makes c the controller of the Scroll, and takes a jump it holds.
func (s *scrollWidgetState) attach(c *ScrollController) {
	if s.controller == c {
		return
	}
	if s.controller != nil && s.controller.view == s {
		s.controller.view = nil
		s.controller.offset = s.Offset.Value()
	}
	s.controller = c
	if c == nil {
		return
	}
	if c.view != nil && c.view != s {
		c.view.controller = nil
	}
	c.view = s
	if c.pending {
		c.pending = false
		s.codeJump(c.offset)
	}
}

// atEndSlack is how near an end the view counts as there, for offsets that
// sums of fractional heights leave a little short.
const atEndSlack = 0.5

// ScrollPosition is where a scrolling view is, as OnScroll of Scroll and of
// collection.List tells it and their controllers read it.
type ScrollPosition struct {
	// Offset is how far the view is from the start of the scroll range, as
	// ScrollController.Offset counts it. While the wheel pulls the content
	// past an end, it is below 0 or above MaxOffset.
	Offset float64
	// MaxOffset is the largest offset: how much longer the content is than
	// the view, as of the last layout, or 0 for content that fits.
	MaxOffset float64
}

// AtStart reports whether the view is at the start of the scroll range,
// within half a pixel.
func (p ScrollPosition) AtStart() bool { return p.Offset <= atEndSlack }

// AtEnd reports whether the view is at the end of the scroll range, within
// half a pixel: at the bottom of a list or a Scroll, or with Scroll.Reverse at
// the top. Content that fits the view is at both ends.
func (p ScrollPosition) AtEnd() bool { return p.Offset >= p.MaxOffset-atEndSlack }
