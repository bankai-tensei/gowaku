package paint

import "github.com/bankai-tensei/gowaku/geom"

// layerMargin is how far, in logical pixels, a layer can reach past what its
// commands report. Glyphs can overhang the box of their run.
const layerMargin = 4

// Layer is a part of the picture that is recorded on its own, relative to its
// own origin. As long as its version stays, render keeps its pixels. See
// docs/ADR/0019-layers.md.
type Layer struct {
	canvas  Canvas
	version uint64
	bounds  geom.Rect
}

// Begin discards what the layer holds and returns the canvas to record it on.
// Call End when done.
func (l *Layer) Begin() *Canvas {
	l.canvas.Reset()
	l.canvas.layered = true
	return &l.canvas
}

// End completes the recording that Begin started.
func (l *Layer) End() {
	l.version++
	l.bounds = geom.Rect{}
	ops := l.canvas.list.Ops
	for i := range ops {
		l.bounds = union(l.bounds, reach(&ops[i]))
	}
}

// reach returns an estimate of where op draws. It serves to skip layers outside
// a clip: render measures the real extent of a layer from what it encodes.
func reach(op *Op) geom.Rect {
	r := op.RRect.Rect
	m := float32(layerMargin) + op.Blur*shadowReach
	r = geom.Rect{X: r.X - m, Y: r.Y - m, W: r.W + 2*m, H: r.H + 2*m}
	if op.Transform.IsIdentity() {
		return r
	}
	return op.Transform.ApplyRect(r)
}

func union(a, b geom.Rect) geom.Rect {
	if a.Size().Empty() {
		return b
	}
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return geom.Rect{X: x, Y: y, W: max(a.X+a.W, b.X+b.W) - x, H: max(a.Y+a.H, b.Y+b.H) - y}
}

// Version changes with every recording.
func (l *Layer) Version() uint64 { return l.version }

// Bounds returns an estimate of where the layer draws, relative to its origin.
func (l *Layer) Bounds() geom.Rect { return l.bounds }

// Empty reports whether the layer draws nothing.
func (l *Layer) Empty() bool { return len(l.canvas.list.Ops) == 0 }

// DisplayList returns what was recorded, relative to the origin of the layer.
func (l *Layer) DisplayList() *DisplayList { return &l.canvas.list }
