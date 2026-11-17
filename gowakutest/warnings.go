package gowakutest

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// WarningKind is what a Warning is about.
type WarningKind string

// The kinds of warnings.
const (
	// WarnOverflow is content that does not fit its box: children of a Row or
	// Column past its end, text longer or taller than its box, or any other
	// child laid out past an edge of its parent, as the segments of a
	// SegmentedControl given less room than they need. It is drawn over what
	// follows, or cut off. A parent that clips its children, such as the
	// viewport of a Scroll, and a child that Positioned places on purpose are
	// not reported.
	WarnOverflow WarningKind = "overflow"
	// WarnUnlabeled is a button, checkbox, slider or text field that tells
	// assistive technology nothing: no label, and no text inside.
	WarnUnlabeled WarningKind = "unlabeled"
	// WarnContrast is text whose color is too close to what is behind it to
	// read well: below 4.5:1, or 3:1 for large text, as WCAG 2 asks. The text
	// of a disabled control is not checked, as WCAG 1.4.3 exempts inactive
	// components: the control, or a node around it, has semantics.Disabled.
	WarnContrast WarningKind = "contrast"
	// WarnShader is a custom shader without a Fallback, which the CPU
	// renderer of gowakutest cannot draw: screenshots leave it out.
	WarnShader WarningKind = "shader"
)

// Warning is a likely problem of the current frame, found by Warnings.
type Warning struct {
	Kind WarningKind `json:"kind"`
	// Rect is where it is, in logical pixels from the top left of the window.
	Rect geom.Rect `json:"rect"`
	// Text is the text concerned, or the label of the nearest node, if any.
	Text string `json:"text,omitempty"`
	// Detail says what is wrong, in a sentence.
	Detail string `json:"detail"`
	// Amount is the overflow in logical pixels, or the contrast ratio.
	Amount float32 `json:"amount"`
}

func (w Warning) String() string {
	return fmt.Sprintf("%s at (%.0f, %.0f %.0fx%.0f): %s", w.Kind, w.Rect.X, w.Rect.Y, w.Rect.W, w.Rect.H, w.Detail)
}

// Warnings checks the current frame for overflow, unlabeled controls, text of
// low contrast and custom shaders that screenshots leave out, and returns what
// it found, in the order of painting.
// It draws a screenshot to measure contrast, so it costs about as much as one.
//
// What the semantics tree leaves out because nobody can see or reach it is not
// checked, as Find does not find it: a hidden subtree, such as an Opacity of
// zero, and the page under an open dialog or sheet, which the scrim dims on
// purpose. The dialog itself, and what opens over it, are checked. Nor is
// what an ancestor clips away whole, such as a row scrolled out of the
// viewport of a Scroll, under a header that stays: the clips are those of the
// objects that implement core.Clipper, and the window. Text that is cut in
// part is measured against what shows of its background.
//
// The checks follow the transforms of the ancestors to where things are
// painted: the zoom and pan of an interact.InteractiveViewer, a Transform, and
// a Translate or Scale that moves only the picture (core.Transformer and
// core.PaintTransformer). Text under a zoom is measured where it shows, and
// as large as it shows, so text zoomed past 18.66 px counts as large; overflow
// is in pixels of the window. Text that a transform turns or slants is not
// measured for contrast, as its upright bounds hold what is around it.
func (tt *Tester) Warnings() []Warning {
	tt.frame()
	var out []Warning
	img := tt.Screenshot()
	root, _ := tt.tree.SemanticsTree()
	disabled := map[core.RenderObject]bool{}
	tt.disabledOwners(root, disabled)
	window := geom.Rect{W: tt.size.W, H: tt.size.H}
	// blocked is the way from the root to the modal that blocks, as the
	// semantics tree keeps to it: on that way, what is painted before the next
	// object is under the modal.
	var blocked []core.RenderObject
	for ro := tt.tree.BlockingSemantics(); ro != nil; ro = ro.Parent() {
		blocked = append(blocked, ro)
	}
	slices.Reverse(blocked)
	// walk visits ro, whose parent's coordinates m takes to the window; on is
	// the place of ro on the way to the modal, or -1 off it.
	// inactive says that ro is inside a disabled control.
	// clip is what the ancestors of ro let show, in the window.
	// away holds the objects that the clips hide whole, for unlabeled.
	away := map[core.RenderObject]bool{}
	var walk func(ro core.RenderObject, m geom.Affine, on int, inactive bool, clip geom.Rect)
	walk = func(ro core.RenderObject, m geom.Affine, on int, inactive bool, clip geom.Rect) {
		if h, ok := ro.(core.Hider); ok && h.Hidden() {
			return
		}
		inactive = inactive || disabled[ro]
		m = m.Mul(geom.Translation(ro.Offset()))
		rect := m.ApplyRect(geom.RectFromSize(ro.Size()))
		var childClip geom.Rect
		cuts := false
		if c, ok := ro.(core.Clipper); ok {
			childClip, cuts = c.ChildClip()
		}
		if outside(rect, clip) {
			away[ro] = true
		} else {
			out = tt.check(out, img, ro, m, rect, clip, inactive)
			if !cuts {
				out = childOverflow(out, ro, m, rect)
			}
		}
		if cuts {
			clip = clip.Intersect(m.ApplyRect(childClip))
		}
		if t, ok := ro.(core.Transformer); ok {
			if c := t.ChildTransform(); !c.IsIdentity() {
				m = m.Mul(c)
			}
		}
		if t, ok := ro.(core.PaintTransformer); ok {
			if c := t.PaintTransform(); !c.IsIdentity() {
				m = m.Mul(c)
			}
		}
		var next core.RenderObject
		if on >= 0 && on+1 < len(blocked) {
			next = blocked[on+1]
		}
		for i := range ro.ChildCount() {
			child := ro.ChildAt(i)
			switch {
			case next == nil:
				walk(child, m, -1, inactive, clip)
			case child == next:
				walk(child, m, on+1, inactive, clip)
				next = nil
			default:
				// Painted before the way to the modal: under it.
			}
		}
	}
	start := -1
	if len(blocked) > 0 {
		start = 0
	}
	walk(tt.tree.RootRenderObject(), geom.Identity(), start, false, window)
	return append(out, tt.unlabeled(root, away, nil)...)
}

// check appends the problems of ro, at rect in the window, to out. m takes
// the coordinates of ro to the window, through the transforms of its
// ancestors. clip is what of the window its ancestors let show, and rect
// overlaps it; inactive says that ro is inside a disabled control.
func (tt *Tester) check(out []Warning, img *image.NRGBA, ro core.RenderObject, m geom.Affine, rect, clip geom.Rect, inactive bool) []Warning {
	k := m.MinScale()
	switch r := ro.(type) {
	case *layout.RenderFlex:
		if o := r.Overflow() * k; o > 0.5 {
			what := "Row"
			if r.Axis == layout.Vertical {
				what = "Column"
			}
			out = append(out, Warning{Kind: WarnOverflow, Rect: rect, Amount: o,
				Detail: fmt.Sprintf("the children of a %s run %.1f px past its end", what, o)})
		}
	case *basic.RenderParagraph:
		if o := r.Overflow() * k; o > 0.5 {
			out = append(out, Warning{Kind: WarnOverflow, Rect: rect, Text: r.Text(), Amount: o,
				Detail: fmt.Sprintf("the text runs %.1f px past its box", o)})
		}
		if inactive || !m.IsScaleTranslate() {
			// A turned or slanted text covers only a part of its upright
			// bounds, and those hold what is around it: no measure.
			break
		}
		if w, ok := tt.contrast(img, r, k, rect, rect.Intersect(clip)); ok {
			out = append(out, w)
		}
	case *layout.RenderCustomPaint:
		if w, ok := shaderWarning(r.Shader().Shader, rect); ok {
			out = append(out, w)
		}
	case *layout.RenderShaderMask:
		if w, ok := shaderWarning(r.Shader, rect); ok {
			out = append(out, w)
		}
	}
	return out
}

// childOverflow appends a warning to out if a child of ro, at rect in the
// window through the transform m, is laid out past an edge of ro: a box that placed its children
// without looking at its own width, as a row of segments wider than the room
// it was given. A Row or Column says so on its own, and an object that clips
// its children shows only what fits, so neither is asked; nor is a child of a
// Stack that Positioned places, on purpose, anywhere. The warning names the
// edge that the farthest child crosses.
func childOverflow(out []Warning, ro core.RenderObject, m geom.Affine, rect geom.Rect) []Warning {
	box := ro.Size()
	if _, ok := ro.(*layout.RenderFlex); ok || box.W <= 0 || box.H <= 0 {
		return out
	}
	var most float32
	edge := ""
	for i := range ro.ChildCount() {
		child := ro.ChildAt(i)
		if h, ok := child.(core.Hider); ok && h.Hidden() {
			continue
		}
		if _, ok := child.(*layout.RenderPositioned); ok {
			// Placed by hand, as a badge that hangs over a corner.
			continue
		}
		sz := child.Size()
		if sz.W <= 0 || sz.H <= 0 {
			continue
		}
		off := child.Offset()
		for _, e := range [...]struct {
			name string
			by   float32
		}{
			{"left", -off.X},
			{"top", -off.Y},
			{"right", off.X + sz.W - box.W},
			{"bottom", off.Y + sz.H - box.H},
		} {
			if e.by > most {
				most, edge = e.by, e.name
			}
		}
	}
	most *= m.MinScale()
	if most <= 0.5 {
		return out
	}
	return append(out, Warning{Kind: WarnOverflow, Rect: rect, Amount: most,
		Detail: fmt.Sprintf("a child of %s runs %.1f px past its %s edge", typeName(ro), most, edge)})
}

// typeName is the name of the type of ro without its pointer, as
// layout.RenderSegmented.
func typeName(ro core.RenderObject) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", ro), "*")
}

// outside reports whether rect lies wholly outside clip. An object of no size
// is outside only if its place is.
func outside(rect, clip geom.Rect) bool {
	if rect.W > 0 && rect.H > 0 {
		return rect.Intersect(clip).Size().Empty()
	}
	return clip.Size().Empty() || rect.X < clip.X || rect.Y < clip.Y ||
		rect.X > clip.X+clip.W || rect.Y > clip.Y+clip.H
}

// disabledOwners puts in set the objects that opened the disabled nodes at
// and below n, of the last SemanticsTree. The nodes below a disabled one need
// not be visited: the walk of Warnings carries the state down.
func (tt *Tester) disabledOwners(n *Node, set map[core.RenderObject]bool) {
	if n.State&semantics.Disabled != 0 {
		if ro := tt.tree.SemanticsOwner(n.ID); ro != nil {
			set[ro] = true
			return
		}
	}
	for _, c := range n.Children {
		tt.disabledOwners(c, set)
	}
}

// shaderWarning reports s, drawn at rect, if it has no Fallback.
func shaderWarning(s *paint.Shader, rect geom.Rect) (Warning, bool) {
	if s == nil || s.Fallback != nil {
		return Warning{}, false
	}
	return Warning{Kind: WarnShader, Rect: rect, Text: s.Name,
		Detail: fmt.Sprintf("the shader %q has no Fallback, so the CPU renderer draws nothing for it", s.Name)}, true
}

// unlabeled finds the controls below n that say nothing, but for those whose
// objects the clips hide whole.
func (tt *Tester) unlabeled(n *Node, away map[core.RenderObject]bool, out []Warning) []Warning {
	switch n.Role {
	case semantics.RoleButton, semantics.RoleCheckbox, semantics.RoleSlider, semantics.RoleTextField:
		if n.Label == "" && !saysSomething(n) && !away[tt.tree.SemanticsOwner(n.ID)] {
			out = append(out, Warning{Kind: WarnUnlabeled, Rect: n.Rect,
				Detail: fmt.Sprintf("a %s has no label and no text, so a screen reader cannot name it", strings.ToLower(n.Role.String()))})
		}
	}
	for _, c := range n.Children {
		out = tt.unlabeled(c, away, out)
	}
	return out
}

// saysSomething reports whether a node below n has a label.
func saysSomething(n *Node) bool {
	for _, c := range n.Children {
		if c.Label != "" || saysSomething(c) {
			return true
		}
	}
	return false
}

// contrast measures text against what is behind it: the most common color in
// vis, the part of its box rect that shows, once the pixels near the color of
// the text are left out, since the glyphs, and their smoothed edges, cover a
// part of the box. Colors are counted in coarse steps, so that a gradient
// still has a most common one. zoom is the scale of the transforms of the
// ancestors, by which the text shows larger, or smaller, than its size.
func (tt *Tester) contrast(img *image.NRGBA, r *basic.RenderParagraph, zoom float32, rect, vis geom.Rect) (Warning, bool) {
	if vis.W < 1 || vis.H < 1 || r.Text() == "" {
		return Warning{}, false
	}
	style := r.Style()
	ink := style.PaintColor()
	if ink.A <= 0 {
		return Warning{}, false
	}
	s := tt.scale
	bounds := image.Rect(int(vis.X*s), int(vis.Y*s), int(math.Ceil(float64((vis.X+vis.W)*s))), int(math.Ceil(float64((vis.Y+vis.H)*s)))).Intersect(img.Rect)
	inkRGB := [3]float64{float64(ink.R) * 255, float64(ink.G) * 255, float64(ink.B) * 255}
	type bucket struct {
		n   int
		sum [3]float64
	}
	buckets := map[[3]uint8]*bucket{}
	var best *bucket
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			px := [3]float64{float64(c.R), float64(c.G), float64(c.B)}
			if distance(px, inkRGB) < 48 {
				continue
			}
			k := [3]uint8{c.R >> 4, c.G >> 4, c.B >> 4}
			b := buckets[k]
			if b == nil {
				b = &bucket{}
				buckets[k] = b
			}
			b.n++
			for i := range px {
				b.sum[i] += px[i]
			}
			if best == nil || b.n > best.n {
				best = b
			}
		}
	}
	if best == nil {
		// Nothing but the color of the text: a box filled by its own glyphs.
		return Warning{}, false
	}
	back := [3]float64{best.sum[0] / float64(best.n) / 255, best.sum[1] / float64(best.n) / 255, best.sum[2] / float64(best.n) / 255}
	a := float64(ink.A)
	fore := [3]float64{
		float64(ink.R)*a + back[0]*(1-a),
		float64(ink.G)*a + back[1]*(1-a),
		float64(ink.B)*a + back[2]*(1-a),
	}
	ratio := contrastRatio(fore, back)
	need := 4.5
	size := style.Size
	if size <= 0 {
		size = 14
	}
	size *= zoom
	if size >= 18.66 || size >= 14 && style.Weight >= 700 {
		need = 3
	}
	if ratio >= need {
		return Warning{}, false
	}
	return Warning{Kind: WarnContrast, Rect: rect, Text: r.Text(), Amount: float32(ratio), Detail: contrastDetail(ratio, need)}, true
}

// contrastDetail says that ratio is below need. The ratio is floored to two
// decimals, so that one just short of the need never prints as the need
// itself, as in 4.50:1 below 4.5:1.
func contrastDetail(ratio, need float64) string {
	return fmt.Sprintf("the text has a contrast of %.2f:1 against its background, below %.1f:1", math.Floor(ratio*100)/100, need)
}

// distance is how far apart two colors are, in steps of 0..255 summed over
// the channels.
func distance(a, b [3]float64) float64 {
	return math.Abs(a[0]-b[0]) + math.Abs(a[1]-b[1]) + math.Abs(a[2]-b[2])
}

// contrastRatio is the WCAG 2 contrast of two sRGB colors with channels in 0..1.
func contrastRatio(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c [3]float64) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
}
