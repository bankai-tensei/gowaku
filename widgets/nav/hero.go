package nav

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
)

// Hero marks a child that flies to the page a Navigator moves to: when a page
// comes or goes, each Hero on it whose Tag matches one on the page below lifts
// off at its place on one page and lands at its place on the other, crossfading
// from the one child to the other on the way, while both stay hidden. The
// flight follows the motion of the page, and a window that asks for less
// motion has none.
//
// Give the icon of a row in a list and the big icon of its detail page the same
// Tag, as the ID of the note. Tags must be comparable, and unique on a page.
// The flight measures both places on each frame, so a Hero inside a scrolled
// list lifts off from where it shows. It is drawn over the pages, clipped to
// the Navigator, and only paints: the pointer and the keyboard go to the pages
// as usual.
type Hero struct {
	Tag   core.Key
	Child core.Widget
}

// Key implements core.Widget.
func (Hero) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (h Hero) CreateRenderObject() core.RenderObject { return &renderHero{tag: h.Tag} }

// UpdateRenderObject implements core.RenderWidget.
func (h Hero) UpdateRenderObject(ro core.RenderObject) { ro.(*renderHero).tag = h.Tag }

// ChildWidget implements core.SingleChildWidget.
func (h Hero) ChildWidget() core.Widget { return h.Child }

// renderHero hides its child while it flies.
type renderHero struct {
	layout.RenderProxy
	tag core.Key
	// flying counts the flights the hero takes part in.
	flying int
}

func (r *renderHero) setFlying(on bool) {
	if on {
		r.flying++
	} else {
		r.flying = max(r.flying-1, 0)
	}
	r.MarkNeedsPaint()
}

// Paint implements core.RenderObject.
func (r *renderHero) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.flying > 0 {
		return
	}
	r.RenderProxy.Paint(cv, off)
}

// heroFlight is a Hero on its way between the page below and the one above.
type heroFlight struct {
	from, to *renderHero
}

// collectHeroes appends the heroes below ro that are not hidden, in the order
// of painting.
func collectHeroes(ro core.RenderObject, out []*renderHero) []*renderHero {
	for i := range ro.ChildCount() {
		c := ro.ChildAt(i)
		if h, ok := c.(core.Hider); ok && h.Hidden() {
			continue
		}
		if h, ok := c.(*renderHero); ok {
			for _, o := range out {
				if o.tag == h.tag {
					panic(fmt.Sprintf("gowaku: two Heroes on one page have the Tag %v; give each a tag of its own", h.tag))
				}
			}
			out = append(out, h)
		}
		out = collectHeroes(c, out)
	}
	return out
}

// below returns the page under e that is not on its way out, or nil.
func (s *navigatorState) below(e *navEntry) *navEntry {
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i] != e {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if !s.entries[j].exiting {
				return s.entries[j]
			}
		}
		return nil
	}
	return nil
}

// startFlights pairs the heroes of the page e with those of the page below,
// for the motion of e that starts.
func (s *navigatorState) startFlights(e *navEntry) {
	s.endFlights(e)
	if !e.animated() || e.slot == nil || s.render == nil || s.reduce() {
		return
	}
	under := s.below(e)
	if under == nil || under.slot == nil {
		return
	}
	from := collectHeroes(under.slot, nil)
	if len(from) == 0 {
		return
	}
	for _, to := range collectHeroes(e.slot, nil) {
		for _, f := range from {
			if f.tag == to.tag {
				e.flights = append(e.flights, heroFlight{from: f, to: to})
				f.setFlying(true)
				to.setFlying(true)
				break
			}
		}
	}
	s.render.MarkNeedsPaint()
}

// endFlights lands the heroes of e.
func (s *navigatorState) endFlights(e *navEntry) {
	if len(e.flights) == 0 {
		return
	}
	for _, f := range e.flights {
		f.from.setFlying(false)
		f.to.setFlying(false)
	}
	clear(e.flights)
	e.flights = e.flights[:0]
	if s.render != nil {
		s.render.MarkNeedsPaint()
	}
}

// heroRect returns the box of h relative to the Navigator.
func (s *navigatorState) heroRect(h *renderHero) geom.Rect {
	p, _ := core.GlobalToLocal(s.render, core.LocalToGlobal(h, geom.Offset{}))
	sz := h.Size()
	return geom.Rect{X: p.X, Y: p.Y, W: sz.W, H: sz.H}
}

// flightRect returns where a flight is at progress t, from the box of the hero
// below at 0 to that of the hero above at 1.
func flightRect(from, to geom.Rect, t float32) geom.Rect { return motion.LerpRect(from, to, t) }

// paintFlights draws the heroes in flight over the pages, clipped to the
// Navigator.
func (s *navigatorState) paintFlights(cv *paint.Canvas, off geom.Offset) {
	clipped := false
	for _, e := range s.entries {
		if len(e.flights) == 0 {
			continue
		}
		if !clipped {
			clipped = true
			sz := s.render.Size()
			cv.Save()
			cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
		}
		t := e.controller.Value()
		fade := geom.Clamp32(t, 0, 1)
		for _, f := range e.flights {
			if !f.from.Attached() || !f.to.Attached() {
				continue
			}
			box := flightRect(s.heroRect(f.from), s.heroRect(f.to), t).Translate(off)
			paintHeroChild(cv, f.from, box, 1-fade)
			paintHeroChild(cv, f.to, box, fade)
		}
	}
	if clipped {
		cv.Restore()
	}
}

// paintHeroChild paints the child of h fitted into box, centered and scaled
// alike on both axes, so that text keeps its shape, and faded to opacity.
func paintHeroChild(cv *paint.Canvas, h *renderHero, box geom.Rect, opacity float32) {
	child := h.ChildAt(0)
	if child == nil || opacity <= 0 {
		return
	}
	sz := child.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}
	scale := min(box.W/sz.W, box.H/sz.H)
	if scale <= 0 {
		return
	}
	cv.Save()
	cv.PushOpacity(opacity)
	cv.Translate(box.X+(box.W-sz.W*scale)/2, box.Y+(box.H-sz.H*scale)/2)
	cv.Scale(scale)
	child.Paint(cv, geom.Offset{})
	cv.Restore()
}
