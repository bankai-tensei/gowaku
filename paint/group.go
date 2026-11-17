package paint

import "github.com/bankai-tensei/gowaku/geom"

// MaskMode says how the mask of a group is read.
type MaskMode uint8

// The mask modes, as mask-type in SVG.
const (
	// MaskNone means that the group has no mask.
	MaskNone MaskMode = iota
	// MaskAlpha multiplies the group by the alpha of the mask, which is how a
	// clip of any shape, drawn opaque, becomes a mask.
	MaskAlpha
	// MaskLuminance multiplies the group by the luminance of the mask:
	// 0.2125·R + 0.7154·G + 0.0721·B of its premultiplied color, as SVG
	// masks do.
	MaskLuminance
)

// GroupStyle says how PushGroup composites what the group draws.
//
// The group draws into a picture of its own, which starts transparent. When it
// ends, the picture goes through Filter, then is multiplied by the mask, then
// faded by the opacity that was in effect at PushGroup, and combined with what
// is below by Blend: the order of SVG and CSS. The zero value only isolates the
// group, which is what an opacity over overlapping shapes needs.
type GroupStyle struct {
	// Blend combines the picture with what is below. The zero value draws
	// source-over, or the mode of PushBlend.
	Blend BlendMode
	// Mask, unless it is MaskNone, multiplies the picture by what is drawn
	// between BeginMask and PopGroup.
	Mask MaskMode
	// Filter, if set, runs over the picture. It must stay unchanged until the
	// canvas is reset.
	Filter *Filter
	// Cache, if set, lets render keep the picture of the group on the GPU and
	// draw it again in later frames, instead of the content, for as long as
	// the version of Cache stays the same; see GroupCache.
	Cache *GroupCache
}

// GroupCache stands for the picture of a group from one frame to the next. Give
// each place that draws a group its own, and call Invalidate whenever what the
// group draws changes other than by moving: its content, its mask or its filter.
// Moving is a change of the translation in effect at PushGroup; content drawn at
// other coordinates under the same translation is a change, so translate to the
// place of the group before PushGroup and draw from there. Render reads the
// version when it draws the list, so a list kept from an earlier frame, whose
// layers were recorded again since, sees an Invalidate made with them.
// The opacity and the blend of the group, and its clip, apply when the picture
// is drawn, and do not count. The zero value is ready to use.
//
// The content of a cached group is recorded without the clip in effect, so that
// the picture holds all of it wherever it is drawn, and the picture lands on
// whole physical pixels, as the pixels of a Layer do. Render keeps the pictures
// within a budget of GPU memory, and drops those not drawn for a while. See
// docs/ADR/0115-cached-groups.md.
type GroupCache struct {
	version uint64
}

// Invalidate says that what the group draws has changed.
func (c *GroupCache) Invalidate() { c.version++ }

// Version changes with every Invalidate.
func (c *GroupCache) Version() uint64 { return c.version }

// GroupCall is what an OpGroupPush opens.
type GroupCall struct {
	Style GroupStyle
	// Mask is the index in DisplayList.Ops of the OpGroupMask that starts the
	// mask, or -1, and End that of the OpGroupPop.
	Mask, End int
}

// groupEntry is a group that is open on the canvas.
type groupEntry struct {
	// op is the index of its OpGroupPush, and call that of its GroupCall,
	// both -1 for a group that draws nothing.
	op, call int
	// depth is the length of the state stack inside the group.
	depth  int
	masked bool
}

// PushGroup starts a group: what is drawn until PopGroup composites as one
// picture, as style says. The opacity in effect now fades the group as a whole,
// and what the group draws starts at full opacity; the transform and clip
// carry on. PopGroup brings back the state of the moment, as Restore does. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
//
// Groups nest, also inside a Layer and in a layer that has to be drawn as part
// of the frame. They cost a picture on the GPU each, drawn anew every frame
// that draws them; a group inside a RepaintBoundary is drawn again only when
// the boundary is.
func (c *Canvas) PushGroup(style GroupStyle) {
	hidden := c.state.flat || c.state.opacity <= 0
	opacity, blend := c.state.opacity, c.blendOf(style.Blend)
	op := Op{
		Kind:      OpGroupPush,
		Blend:     blend,
		Color:     geom.Color{A: opacity},
		Origin:    c.state.translation,
		Scale:     c.state.scale,
		Transform: c.state.transform,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	}
	c.Save()
	e := groupEntry{op: -1, call: -1, depth: len(c.stack)}
	c.state.opacity, c.state.blend = 1, BlendSrcOver
	if style.Cache != nil {
		c.state.clip, c.state.clipped = geom.Rect{}, false
	}
	if hidden {
		c.state.flat = true
		c.groups = append(c.groups, e)
		return
	}
	e.call, e.op = len(c.list.Groups), len(c.list.Ops)
	c.list.Groups = append(c.list.Groups, GroupCall{Style: style, Mask: -1, End: -1})
	op.ShaderCall = int32(e.call)
	c.list.Ops = append(c.list.Ops, op)
	c.groups = append(c.groups, e)
}

// BeginMask ends what the innermost group draws and starts its mask, which is
// drawn with the transform and clip of PushGroup, at full opacity. It panics
// unless a group whose style has a mask is open and has not begun its mask.
func (c *Canvas) BeginMask() {
	if len(c.groups) == 0 {
		panic("gowaku: Canvas.BeginMask called without a group")
	}
	e := &c.groups[len(c.groups)-1]
	if e.masked {
		panic("gowaku: Canvas.BeginMask called twice for one group")
	}
	if e.call >= 0 && c.list.Groups[e.call].Style.Mask == MaskNone {
		panic("gowaku: Canvas.BeginMask called for a group without a mask")
	}
	e.masked = true
	c.closeGroupState(e)
	c.Save()
	c.state.opacity, c.state.blend = 1, BlendSrcOver
	if e.op < 0 {
		c.state.flat = true
		return
	}
	if c.list.Groups[e.call].Style.Cache != nil {
		c.state.clip, c.state.clipped = geom.Rect{}, false
	}
	c.list.Groups[e.call].Mask = len(c.list.Ops)
	c.list.Ops = append(c.list.Ops, Op{Kind: OpGroupMask, ShaderCall: int32(e.call), ClipLevel: c.state.level})
}

// PopGroup ends the innermost group and composites it. It panics without one.
// A group with a mask that never began draws nothing, as an empty mask hides
// everything.
func (c *Canvas) PopGroup() {
	if len(c.groups) == 0 {
		panic("gowaku: Canvas.PopGroup called without a group")
	}
	e := c.groups[len(c.groups)-1]
	c.groups = c.groups[:len(c.groups)-1]
	c.closeGroupState(&e)
	if e.op < 0 {
		return
	}
	c.list.Groups[e.call].End = len(c.list.Ops)
	c.list.Ops = append(c.list.Ops, Op{Kind: OpGroupPop, ShaderCall: int32(e.call), ClipLevel: c.state.level})
}

// closeGroupState brings back the state of the PushGroup of e, closing what was
// saved inside the group.
func (c *Canvas) closeGroupState(e *groupEntry) {
	for len(c.stack) > e.depth {
		c.restore()
	}
	c.restore()
}
