package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// wrapTolerance keeps a child that fits exactly in a run from wrapping because
// of rounding in the sum of float32 lengths.
const wrapTolerance = 1.0 / 1024

// RenderWrap lays out its children in runs along Axis, and starts a new run
// when the next child does not fit. Runs stack across the axis: rows downwards
// for Horizontal, columns rightwards for Vertical.
//
// Children may be as long as the Wrap along the axis, and as long as they like
// across it. If the length along the axis is unbounded, nothing wraps and the
// children form a single run.
type RenderWrap struct {
	core.RenderBase
	Axis Axis
	// Main places the children of each run along the axis.
	Main MainAxisAlignment
	// Cross places each child within the length of its run across the axis.
	// CrossStretch makes it as long as the longest child of the run.
	Cross CrossAxisAlignment
	// RunAlignment places the runs across the axis, where the Wrap is longer
	// than they need, such as under tight constraints.
	RunAlignment MainAxisAlignment
	// MainSize decides the length along the axis. MainMax, the zero value,
	// takes all that is allowed, as Row and Column do; MainMin shrinks to the
	// longest run. An unbounded length always shrinks.
	MainSize MainAxisSize
	// Spacing is the gap between children in a run, and RunSpacing the gap
	// between runs.
	Spacing    float32
	RunSpacing float32
	children   []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderWrap) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderWrap) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderWrap) ChildAt(i int) core.RenderObject { return r.children[i] }

func (r *RenderWrap) mainOf(s geom.Size) float32 {
	if r.Axis == Horizontal {
		return s.W
	}
	return s.H
}

func (r *RenderWrap) crossOf(s geom.Size) float32 {
	if r.Axis == Horizontal {
		return s.H
	}
	return s.W
}

func (r *RenderWrap) sizeOf(mainLength, crossLength float32) geom.Size {
	if r.Axis == Horizontal {
		return geom.Size{W: mainLength, H: crossLength}
	}
	return geom.Size{W: crossLength, H: mainLength}
}

func (r *RenderWrap) offsetOf(along, across float32) geom.Offset {
	if r.Axis == Horizontal {
		return geom.Offset{X: along, Y: across}
	}
	return geom.Offset{X: across, Y: along}
}

// run returns the end of the run that starts at child start, and the lengths
// of the run along and across the axis. It reads the sizes of the children, so
// Layout can find the same runs twice without storing them.
func (r *RenderWrap) run(start int, limit float32) (end int, mainLength, crossLength float32) {
	end = start
	for end < len(r.children) {
		sz := r.children[end].Size()
		m := r.mainOf(sz)
		next := m
		if end > start {
			next = mainLength + r.Spacing + m
		}
		if end > start && next > limit+wrapTolerance {
			break
		}
		mainLength = next
		crossLength = max(crossLength, r.crossOf(sz))
		end++
	}
	return end, mainLength, crossLength
}

// Layout implements core.RenderObject.
func (r *RenderWrap) Layout(c Constraints) geom.Size {
	limit := r.mainOf(c.Biggest())
	inner := Constraints{MaxW: c.MaxW, MaxH: geom.Inf}
	if r.Axis == Vertical {
		inner = Constraints{MaxW: geom.Inf, MaxH: c.MaxH}
	}
	for _, child := range r.children {
		core.LayoutChild(child, inner)
	}

	var longest, content float32
	runs := 0
	for start := 0; start < len(r.children); runs++ {
		end, m, x := r.run(start, limit)
		longest = max(longest, m)
		content += x
		start = end
	}
	content += r.RunSpacing * float32(max(runs-1, 0))

	mainLength := longest
	if r.MainSize == MainMax && limit < geom.Inf {
		mainLength = limit
	}
	own := c.Constrain(r.sizeOf(mainLength, content))
	r.place(limit, r.mainOf(own), r.crossOf(own), content, runs)
	return own
}

// place puts the runs across the axis and the children of each run along it.
// The runs break at limit, as in Layout, and are aligned within mainLength.
func (r *RenderWrap) place(limit, mainLength, crossLength, content float32, runs int) {
	across, runGap := distribute(r.RunAlignment, max(crossLength-content, 0), runs)
	for start := 0; start < len(r.children); {
		end, m, x := r.run(start, limit)
		along, gap := distribute(r.Main, max(mainLength-m, 0), end-start)
		for _, child := range r.children[start:end] {
			if r.Cross == CrossStretch && r.crossOf(child.Size()) != x {
				core.LayoutChild(child, geom.Tight(r.sizeOf(r.mainOf(child.Size()), x)))
			}
			sz := child.Size()
			var shift float32
			switch r.Cross {
			case CrossEnd:
				shift = x - r.crossOf(sz)
			case CrossCenter:
				shift = (x - r.crossOf(sz)) / 2
			}
			child.SetOffset(r.offsetOf(along, across+shift))
			along += r.mainOf(sz) + r.Spacing + gap
		}
		across += x + r.RunSpacing + runGap
		start = end
	}
}

// distribute returns where the first of n items starts and the extra gap after
// each, for free room left over by alignment a.
func distribute(a MainAxisAlignment, free float32, n int) (lead, between float32) {
	count := float32(n)
	switch a {
	case MainEnd:
		lead = free
	case MainCenter:
		lead = free / 2
	case MainSpaceBetween:
		if n > 1 {
			between = free / (count - 1)
		}
	case MainSpaceAround:
		if n > 0 {
			between = free / count
			lead = between / 2
		}
	case MainSpaceEvenly:
		between = free / (count + 1)
		lead = between
	}
	return lead, between
}

// Paint implements core.RenderObject.
func (r *RenderWrap) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
