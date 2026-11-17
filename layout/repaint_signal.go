package layout

import "slices"

// RepaintSignal tells the render objects attached to it to paint again, with no
// rebuild and no layout. It works like the repaint listenable of a custom painter: a State
// that only changes what a Painter draws, such as the bars of a spectrum or the
// hand of a clock, keeps one, hands its address to CustomPaint.Repaint, and
// calls Notify instead of SetState after it changes the values the painter
// reads.
//
// The zero value is ready to use, and a State usually holds it by value. A
// signal may drive several CustomPaints at once. Notify does not allocate, and
// like everything else in the tree it must run on the UI goroutine, as in a
// ticker or an event handler, and not from Paint. Data that another goroutine
// produces reaches the State through the App first, as for SetState.
type RepaintSignal struct {
	views []repaintView
}

// repaintView is a render object that listens to a RepaintSignal.
type repaintView interface {
	MarkNeedsPaint()
	Attached() bool
}

// Notify marks every attached render object as needing a paint. A render
// object that has left the tree is dropped, so a signal that outlives its
// CustomPaint, as in a State that stops showing it, holds nothing.
func (s *RepaintSignal) Notify() {
	kept := s.views[:0]
	for _, v := range s.views {
		if v.Attached() {
			v.MarkNeedsPaint()
			kept = append(kept, v)
		}
	}
	clear(s.views[len(kept):])
	s.views = kept
}

// attach adds v, unless it is there already.
func (s *RepaintSignal) attach(v repaintView) {
	if !slices.Contains(s.views, v) {
		s.views = append(s.views, v)
	}
}

// detach removes v, if it is there.
func (s *RepaintSignal) detach(v repaintView) {
	i := slices.Index(s.views, v)
	if i < 0 {
		return
	}
	last := len(s.views) - 1
	copy(s.views[i:], s.views[i+1:])
	s.views[last] = nil
	s.views = s.views[:last]
}

// Repaint returns the signal the object listens to, or nil.
func (r *RenderCustomPaint) Repaint() *RepaintSignal { return r.repaint }

// SetRepaint makes the object listen to s instead of the signal it had, and to
// none if s is nil. Each Notify of s then repaints the object, without a layout.
func (r *RenderCustomPaint) SetRepaint(s *RepaintSignal) {
	if r.repaint == s {
		return
	}
	if r.repaint != nil {
		r.repaint.detach(r)
	}
	r.repaint = s
	if s != nil {
		s.attach(r)
	}
}
