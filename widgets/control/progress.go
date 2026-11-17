package control

import (
	"time"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
)

const progressRingDefault = 32

// ProgressBar shows how much of something is done, as a bar that fills from the
// left. It takes the width it is given. A new Value slides into place.
//
// With Indeterminate it shows work whose length is not known, such as a
// download without a size: a band crosses the bar again and again, and a
// screen reader hears a busy indicator with no value. The band moves only
// while the bar is seen, and stands in the middle when the window asks for
// less motion.
type ProgressBar struct {
	core.StatefulBase
	// Value is the done share, from 0 to 1. Indeterminate ignores it.
	Value float32
	// Indeterminate shows work of unknown length instead of Value.
	Indeterminate bool
	// Color fills the done part and Track the rest. The zero values select the
	// accent and the strong fill of the theme.
	Color, Track geom.Color
	// Label describes the bar to assistive technology. An indeterminate bar
	// without one says Loading in the language of the window.
	Label string
}

// CreateState implements core.StatefulWidget.
func (ProgressBar) CreateState() core.State { return &progressState{} }

// ProgressRing shows how much of something is done, as a ring that fills
// clockwise from twelve o'clock, with Child in its middle. A new Value slides
// into place.
//
// With Indeterminate it is a spinner, for work whose length is not known: an
// arc turns around the ring, and a screen reader hears a busy indicator with
// no value. The arc turns only while the ring is seen, and stands at twelve
// o'clock when the window asks for less motion.
type ProgressRing struct {
	core.StatefulBase
	// Value is the done share, from 0 to 1. Indeterminate ignores it.
	Value float32
	// Indeterminate makes the ring a spinner instead of showing Value.
	Indeterminate bool
	// Size is the diameter. Zero selects 32.
	Size float32
	// Thickness is the width of the ring. Zero selects a twelfth of Size.
	Thickness float32
	// Color fills the done part and Track the rest. The zero values select the
	// accent and the quiet fill of the theme.
	Color, Track geom.Color
	// Label describes the ring to assistive technology. An indeterminate
	// ring without one says Loading in the language of the window.
	Label string
	// Child sits in the middle of the ring, such as the time left.
	Child core.Widget
}

// CreateState implements core.StatefulWidget.
func (ProgressRing) CreateState() core.State { return &progressState{} }

// progressState animates the value of a ProgressBar or a ProgressRing.
type progressState struct {
	core.StateBase
	fill motion.SpringValue
}

func (s *progressState) value() float32 {
	switch w := s.Widget().(type) {
	case ProgressBar:
		return w.Value
	case ProgressRing:
		return w.Value
	}
	return 0
}

// progressSemantics returns the node of a bar or a ring: its share as a
// value, or, for work of unknown length, a busy indicator with no value,
// which the platforms announce as indeterminate.
func progressSemantics(ctx core.BuildContext, label string, value float32, indeterminate bool, child core.Widget) interact.Semantics {
	if indeterminate {
		if label == "" {
			label = basic.StringsOf(ctx).Loading
		}
		return interact.Semantics{Role: semantics.RoleProgress, Label: label, Child: child}
	}
	return interact.Semantics{Role: semantics.RoleProgress, Label: label, Value: kit.Percent(value), Range: kit.PercentRange(value), Child: child}
}

// InitState implements core.State.
func (s *progressState) InitState() {
	s.fill.OnChange = func() { kit.Rebuild(&s.StateBase) }
	s.fill.Attach(&s.StateBase, geom.Clamp32(s.value(), 0, 1))
}

// DidUpdateWidget implements core.State.
func (s *progressState) DidUpdateWidget(core.Widget) { s.fill.Set(geom.Clamp32(s.value(), 0, 1)) }

// Dispose implements core.State.
func (s *progressState) Dispose() { s.fill.Dispose() }

// Build implements core.State.
func (s *progressState) Build(ctx core.BuildContext) core.Widget {
	c := &basic.ThemeOf(ctx).Color
	pick := func(set, fallback geom.Color) geom.Color {
		if set == (geom.Color{}) {
			return fallback
		}
		return set
	}
	shown := geom.Clamp32(s.fill.Value(), 0, 1)
	still := basic.ReduceMotionOf(ctx)
	switch w := s.Widget().(type) {
	case ProgressBar:
		return progressSemantics(ctx, w.Label, w.Value, w.Indeterminate, progressBarPaint{
			value: shown, active: pick(w.Color, c.Accent), track: pick(w.Track, c.FillStrong),
			busy: w.Indeterminate, still: still,
		})
	case ProgressRing:
		d := w.Size
		if d <= 0 {
			d = progressRingDefault
		}
		var body core.Widget = progressRingPaint{
			value: shown, thickness: w.Thickness, active: pick(w.Color, c.Accent), track: pick(w.Track, c.Fill),
			busy: w.Indeterminate, still: still,
		}
		if w.Child != nil {
			body = basic.Stack{Children: []core.Widget{body, basic.Center{Child: w.Child}}}
		}
		return progressSemantics(ctx, w.Label, w.Value, w.Indeterminate, basic.SizedBox{Width: d, Height: d, Child: body})
	}
	return basic.SizedBox{}
}

// progressBarPaint is the widget of layout.RenderProgressBar.
type progressBarPaint struct {
	value         float32
	active, track geom.Color
	busy, still   bool
}

// Key implements core.Widget.
func (progressBarPaint) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p progressBarPaint) CreateRenderObject() core.RenderObject {
	return &layout.RenderProgressBar{Value: p.value, Active: p.active, Track: p.track, Busy: p.busy, Still: p.still}
}

// UpdateRenderObject implements core.RenderWidget.
func (p progressBarPaint) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderProgressBar)
	if r.Value != p.value || r.Active != p.active || r.Track != p.track || r.Busy != p.busy || r.Still != p.still {
		r.Value, r.Active, r.Track, r.Busy, r.Still = p.value, p.active, p.track, p.busy, p.still
		r.MarkNeedsPaint()
	}
}

// progressRingPaint is the widget of layout.RenderProgressRing.
type progressRingPaint struct {
	value, thickness float32
	active, track    geom.Color
	busy, still      bool
}

// Key implements core.Widget.
func (progressRingPaint) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p progressRingPaint) CreateRenderObject() core.RenderObject {
	return &layout.RenderProgressRing{Value: p.value, Thickness: p.thickness, Active: p.active, Track: p.track, Busy: p.busy, Still: p.still}
}

// UpdateRenderObject implements core.RenderWidget.
func (p progressRingPaint) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderProgressRing)
	if r.Value != p.value || r.Thickness != p.thickness || r.Active != p.active || r.Track != p.track || r.Busy != p.busy || r.Still != p.still {
		r.Value, r.Thickness, r.Active, r.Track = p.value, p.thickness, p.active, p.track
		r.Busy, r.Still = p.busy, p.still
		r.MarkNeedsPaint()
	}
}

// CustomPaint draws with a layout.Painter, for shapes that no widget offers. It
// is Width by Height; a zero length takes all the space allowed. The painter
// runs again on every rebuild of the CustomPaint, since it may hide state.
//
// To draw text, such as the labels of a chart or the numbers of a ruler, look
// up the shaper in Build with core.DependOn[*text.Shaper](ctx) and lay the text
// out there, with Shaper.Shape(content, st).Layout(geom.Inf); the painter puts
// it at a point with paint.Canvas.DrawText, by its corner, middle or
// baseline (paint.TextAnchor). Labels the painter picks only while it paints
// come from a text.Labels kept in the State, which lays each text out once.
// Neither allocates while painting.
//
// A painter whose drawing changes far more often than anything else, such as
// a spectrum or a meter, should not rebuild to show it. Its State keeps a
// layout.RepaintSignal, passes its address as Repaint, and calls Notify after
// it changes what the painter reads: the painter runs again on the next frame,
// without a Build or a Layout. A RepaintBoundary around the CustomPaint keeps
// that paint from reaching the rest of the window.
//
// With a Shader, a custom shader made by cmd/gowaku-shader fills the box first,
// and the Painter, if any, draws over it. The shader reads Params, Textures and
// Time. Animate makes Time run on from where it starts, at the frame rate, in a
// repaint boundary of its own; the clock pauses while a fully transparent
// Opacity hides it (ADR 0027), and a window that asks for less motion holds it
// at Time. See docs/ADR/0045-shaders-svg-animated-images.md.
//
// What a painter draws is one picture to a screen reader. To make the items of
// a chart or a board accessible, put the CustomPaint in a basic.Stack and one
// basic.Positioned per item over it, at the item's rectangle, holding an
// interact.Semantics with the item's Role, Label, Value, State (Selected,
// Checked, Disabled) and OnTap, around a basic.SizedBox{}. The layers draw
// nothing and are hit only where a child is, so the pointer still reaches a
// GestureDetector around the CustomPaint. For the keyboard, put an
// interact.Focus{Enabled: true} inside each Semantics: Tab and Shift+Tab then
// move among the items in the order of the Stack (interact.FocusOrder changes
// it), core marks the one focused for the screen reader, and its OnShowFocus
// tells the State which item the painter should draw a focus ring around.
type CustomPaint struct {
	core.StatelessBase
	Painter       layout.Painter
	Width, Height float32
	// Repaint, if set, paints the CustomPaint again on each of its Notify. A
	// rebuild with another signal, or with none, stops listening to the old one.
	Repaint *layout.RepaintSignal
	// Shader, if set, fills the box before the Painter draws.
	Shader *paint.Shader
	// Params are the sixteen floats the shader reads as params[0] to params[3].
	Params paint.ShaderParams
	// Textures are the images the shader reads with sample0 and sample1,
	// stretched over the box.
	Textures [2]*paint.Image
	// Time is the time the shader sees, or where it starts with Animate.
	Time time.Duration
	// Animate runs the time of the shader.
	Animate bool
}

// Build implements core.StatelessWidget.
func (p CustomPaint) Build(ctx core.BuildContext) core.Widget {
	if p.Shader == nil || !p.Animate {
		return customPaintView{p: p}
	}
	return basic.RepaintBoundary{Child: customPaintView{p: p, still: basic.ReduceMotionOf(ctx)}}
}

// customPaintView is the render widget of CustomPaint. It holds the widget in
// a field: embedding it would bring its Build along.
type customPaintView struct {
	p     CustomPaint
	still bool
}

// Key implements core.Widget.
func (customPaintView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v customPaintView) CreateRenderObject() core.RenderObject {
	p := v.p
	r := &layout.RenderCustomPaint{Painter: p.Painter, Width: p.Width, Height: p.Height}
	r.SetRepaint(p.Repaint)
	r.SetShader(v.shader(), p.Animate, v.still)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v customPaintView) UpdateRenderObject(ro core.RenderObject) {
	p := v.p
	r := ro.(*layout.RenderCustomPaint)
	if r.Width != p.Width || r.Height != p.Height {
		r.Width, r.Height = p.Width, p.Height
		r.MarkNeedsLayout()
	}
	r.Painter = p.Painter
	r.SetRepaint(p.Repaint)
	r.SetShader(v.shader(), p.Animate, v.still)
	r.MarkNeedsPaint()
}

func (v customPaintView) shader() layout.ShaderPaint {
	p := &v.p
	return layout.ShaderPaint{Shader: p.Shader, Params: p.Params, Textures: p.Textures, Time: p.Time}
}
