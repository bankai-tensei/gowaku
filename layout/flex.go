package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// Axis is a direction of layout.
type Axis uint8

// The axes.
const (
	Horizontal Axis = iota
	Vertical
)

// MainAxisAlignment places children along the axis of a flex.
type MainAxisAlignment uint8

// The main axis alignments.
const (
	MainStart MainAxisAlignment = iota
	MainEnd
	MainCenter
	MainSpaceBetween
	MainSpaceAround
	MainSpaceEvenly
)

// CrossAxisAlignment places children across the axis of a flex.
type CrossAxisAlignment uint8

// The cross axis alignments.
const (
	CrossStart CrossAxisAlignment = iota
	CrossEnd
	CrossCenter
	// CrossStretch makes every child as long as the flex across its axis: as
	// tall as a Row, as wide as a Column. That length must be bounded, so a Row
	// with CrossStretch inside a vertical Scroll panics unless a SizedBox gives
	// it a height.
	CrossStretch
)

// The panics of a flex whose room is unbounded, by its Axis. They are constants
// so that Layout does not build them.
var (
	stretchUnbounded = [...]string{
		Horizontal: "gowaku: Row with CrossStretch has an unbounded height, as inside a vertical Scroll; give it a height with a SizedBox, or align its children another way",
		Vertical:   "gowaku: Column with CrossStretch has an unbounded width, as inside a horizontal Scroll; give it a width with a SizedBox, or align its children another way",
	}
	flexUnbounded = [...]string{
		Horizontal: "gowaku: Expanded or Flexible in a Row with an unbounded width, as inside a horizontal Scroll; give the Row a width with a SizedBox",
		Vertical:   "gowaku: Expanded or Flexible in a Column with an unbounded height, as inside a vertical Scroll; give the Column a height with a SizedBox",
	}
)

// MainAxisSize decides how long a flex is.
type MainAxisSize uint8

// The main axis sizes.
const (
	// MainMax takes all the space allowed.
	MainMax MainAxisSize = iota
	// MainMin shrinks to the children.
	MainMin
)

// FlexFit decides whether a flexible child must fill its share.
type FlexFit uint8

// The flex fits.
const (
	FitLoose FlexFit = iota
	FitTight
)

// RenderFlexible marks its child as sharing the free space of a RenderFlex.
// A plain render object stands in for parent data on the child, which would need
// an untyped slot on every render object.
type RenderFlexible struct {
	RenderProxy
	// Flex is the weight of the share. Zero counts as one.
	Flex int
	Fit  FlexFit
}

func (r *RenderFlexible) weight() float32 {
	if r.Flex <= 0 {
		return 1
	}
	return float32(r.Flex)
}

// RenderFlex lays out its children in a line.
type RenderFlex struct {
	core.RenderBase
	Axis     Axis
	Main     MainAxisAlignment
	Cross    CrossAxisAlignment
	MainSize MainAxisSize
	Spacing  float32
	children []core.RenderObject
	// overflow is how far the children ran past the end at the last layout.
	overflow float32
}

// Overflow returns how far the children ran past the end of the flex along its
// axis at the last layout, or zero. Children that do not fit are a layout bug:
// they are drawn over what follows. Tools such as gowakutest report it.
func (r *RenderFlex) Overflow() float32 { return r.overflow }

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderFlex) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderFlex) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderFlex) ChildAt(i int) core.RenderObject { return r.children[i] }

// mainOf and crossOf read a size along and across the axis.
func (r *RenderFlex) mainOf(s geom.Size) float32 {
	if r.Axis == Horizontal {
		return s.W
	}
	return s.H
}

func (r *RenderFlex) crossOf(s geom.Size) float32 {
	if r.Axis == Horizontal {
		return s.H
	}
	return s.W
}

// bounds builds constraints from ranges along and across the axis.
func (r *RenderFlex) bounds(minMain, maxMain, minCross, maxCross float32) Constraints {
	if r.Axis == Horizontal {
		return Constraints{MinW: minMain, MaxW: maxMain, MinH: minCross, MaxH: maxCross}
	}
	return Constraints{MinW: minCross, MaxW: maxCross, MinH: minMain, MaxH: maxMain}
}

func (r *RenderFlex) sizeOf(mainLength, crossLength float32) geom.Size {
	if r.Axis == Horizontal {
		return geom.Size{W: mainLength, H: crossLength}
	}
	return geom.Size{W: crossLength, H: mainLength}
}

// Layout implements core.RenderObject. Fixed children go first, and flexible
// ones share what is left.
func (r *RenderFlex) Layout(c Constraints) geom.Size {
	limit := c.Biggest()
	maxMain, maxCross := r.mainOf(limit), r.crossOf(limit)
	var minCross float32
	if r.Cross == CrossStretch {
		if maxCross >= geom.Inf {
			panic(stretchUnbounded[r.Axis])
		}
		minCross = maxCross
	}

	var used, crossSize, totalFlex float32
	for _, child := range r.children {
		if f, ok := child.(*RenderFlexible); ok {
			totalFlex += f.weight()
			continue
		}
		sz := core.LayoutChild(child, r.bounds(0, geom.Inf, minCross, maxCross))
		used += r.mainOf(sz)
		crossSize = max(crossSize, r.crossOf(sz))
	}
	gaps := r.Spacing * float32(max(len(r.children)-1, 0))

	if totalFlex > 0 {
		if maxMain >= geom.Inf {
			panic(flexUnbounded[r.Axis])
		}
		perFlex := max(maxMain-used-gaps, 0) / totalFlex
		for _, child := range r.children {
			f, ok := child.(*RenderFlexible)
			if !ok {
				continue
			}
			share := perFlex * f.weight()
			var minMain float32
			if f.Fit == FitTight {
				minMain = share
			}
			sz := core.LayoutChild(child, r.bounds(minMain, share, minCross, maxCross))
			used += r.mainOf(sz)
			crossSize = max(crossSize, r.crossOf(sz))
		}
	}

	content := used + gaps
	mainSize := content
	if r.MainSize == MainMax && maxMain < geom.Inf {
		mainSize = maxMain
	}
	own := c.Constrain(r.sizeOf(mainSize, crossSize))
	r.overflow = max(content-r.mainOf(own), 0)
	r.place(r.mainOf(own), r.crossOf(own), content)
	return own
}

func (r *RenderFlex) place(mainSize, crossSize, content float32) {
	free := max(mainSize-content, 0)
	var lead, between float32
	n := float32(len(r.children))
	switch r.Main {
	case MainEnd:
		lead = free
	case MainCenter:
		lead = free / 2
	case MainSpaceBetween:
		if n > 1 {
			between = free / (n - 1)
		}
	case MainSpaceAround:
		if n > 0 {
			between = free / n
			lead = between / 2
		}
	case MainSpaceEvenly:
		between = free / (n + 1)
		lead = between
	}
	at := lead
	for _, child := range r.children {
		sz := child.Size()
		var across float32
		switch r.Cross {
		case CrossEnd:
			across = crossSize - r.crossOf(sz)
		case CrossCenter:
			across = (crossSize - r.crossOf(sz)) / 2
		}
		if r.Axis == Horizontal {
			child.SetOffset(geom.Offset{X: at, Y: across})
		} else {
			child.SetOffset(geom.Offset{X: across, Y: at})
		}
		at += r.mainOf(sz) + r.Spacing + between
	}
}

// Paint implements core.RenderObject.
func (r *RenderFlex) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
