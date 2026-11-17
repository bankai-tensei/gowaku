package render

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// The passes of a filter. Every result is a target of the size of the picture
// of the group, holding premultiplied sRGB; a primitive that works in linearRGB
// converts what it reads and what it writes (gpu.EffectLinear). See
// docs/ADR/0114-gpu-layers-masks-filters.md.

const (
	// maxFilterSigma bounds the standard deviation of a blur, in pixels of
	// the picture, which bounds the texels a pass reads.
	maxFilterSigma = 256
	// maxMorphology bounds the radius of erode and dilate, in pixels, as the
	// CPU did.
	maxMorphology = 64
	// boxSigma is the standard deviation from which a blur is three box blurs
	// instead of a Gaussian, as in the Filter Effects specification.
	boxSigma = 2
)

// filterSource is an input of a primitive: a target and the flags that read it.
type filterSource struct {
	tex   *groupTexture
	alpha bool
}

// runFilter runs f over source, the picture of a group at r, with px taking the
// coordinates of the filter to the pixels of the picture. It returns the result,
// and releases source and every target but the result.
func (g *Resources) runFilter(f *paint.Filter, px geom.Affine, r groupRect, source *groupTexture) *groupTexture {
	n := len(f.Primitives)
	g.filterResults = g.filterResults[:0]
	g.filterLast = g.filterLast[:0]
	for range n {
		g.filterResults = append(g.filterResults, nil)
		g.filterLast = append(g.filterLast, -1)
	}
	// The last primitive that reads each result: a result can go once it
	// has been read for the last time. The result of the filter stays.
	for i := range n {
		p := &f.Primitives[i]
		g.noteUse(p.In, i)
		if p.Kind == paint.FilterComposite || p.Kind == paint.FilterBlend {
			g.noteUse(p.In2, i)
		}
		if p.Kind == paint.FilterMerge {
			for _, in := range p.Inputs {
				g.noteUse(in, i)
			}
		}
	}
	g.filterLast[n-1] = n
	sx, sy := axisScales(px)
	full := image.Rect(0, 0, r.width, r.height)
	for i := range n {
		p := &f.Primitives[i]
		out := g.takeGroupTexture(r.width, r.height)
		if out == nil {
			break
		}
		var sub image.Rectangle
		if p.Subregion != (geom.Rect{}) {
			b := px.ApplyRect(p.Subregion)
			sub = image.Rect(int(geom.Floor32(b.X)), int(geom.Floor32(b.Y)), int(geom.Ceil32(b.X+b.W)), int(geom.Ceil32(b.Y+b.H))).Intersect(full)
			if sub.Empty() {
				// Nothing of the result shows: it stays transparent.
				g.clearJob(out, r)
				g.filterResults[i] = out
				g.releaseRead(i)
				continue
			}
		}
		var flags uint32
		if !p.SRGB {
			flags = gpu.EffectLinear
		}
		in := g.filterInput(p.In, i, source)
		switch p.Kind {
		case paint.FilterBlur:
			g.blur(out, r, in, p.StdDev.X*sx, p.StdDev.Y*sy, flags, sub)
		case paint.FilterOffset:
			var e gpu.EffectParams
			e[0] = moved(px, p.Offset)
			g.effectPass(out, r, gpu.EffectCopy, flags|readFlags(in, 0), &e, texOf(in), nil, sub)
		case paint.FilterDropShadow:
			g.dropShadow(out, r, in, p, px, sx, sy, flags, sub)
		case paint.FilterFlood:
			var e gpu.EffectParams
			c := p.Color
			e[0] = [4]float32{c.R * c.A, c.G * c.A, c.B * c.A, c.A}
			g.effectPass(out, r, gpu.EffectFlood, 0, &e, nil, nil, sub)
		case paint.FilterColorMatrix:
			var e gpu.EffectParams
			m := &p.Matrix
			for row := range 4 {
				e[4+row] = [4]float32{m[row*5], m[row*5+1], m[row*5+2], m[row*5+3]}
				e[0][row] = m[row*5+4]
			}
			g.effectPass(out, r, gpu.EffectMatrix, flags|readFlags(in, 0), &e, texOf(in), nil, sub)
		case paint.FilterComponentTransfer:
			g.transfer(out, r, in, p, flags, sub)
		case paint.FilterComposite:
			in2 := g.filterInput(p.In2, i, source)
			var e gpu.EffectParams
			e[0][0] = float32(p.Operator)
			e[1] = p.K
			g.effectPass(out, r, gpu.EffectComposite, flags|readFlags(in, 0)|readFlags(in2, 1), &e, texOf(in), texOf(in2), sub)
		case paint.FilterBlend:
			in2 := g.filterInput(p.In2, i, source)
			var e gpu.EffectParams
			e[0][0] = float32(p.Mode)
			g.effectPass(out, r, gpu.EffectBlend, flags|readFlags(in, 0)|readFlags(in2, 1), &e, texOf(in), texOf(in2), sub)
		case paint.FilterMerge:
			g.merge(out, r, p.Inputs, i, source, flags, sub)
		case paint.FilterMorphology:
			rx := min(int(geom.Floor32(p.Radius.X*sx+0.5)), maxMorphology)
			ry := min(int(geom.Floor32(p.Radius.Y*sy+0.5)), maxMorphology)
			g.morphology(out, r, in, rx, ry, p.Dilate, flags, sub)
		}
		g.filterResults[i] = out
		g.releaseRead(i)
	}
	releaseGroupTexture(source)
	result := g.filterResults[n-1]
	clear(g.filterResults)
	return result
}

// noteUse records that primitive i reads in.
func (g *Resources) noteUse(in paint.FilterInput, i int) {
	if k := resultIndex(in, i); k >= 0 {
		g.filterLast[k] = max(g.filterLast[k], i)
	}
}

// resultIndex returns the primitive whose result in names for primitive i, or
// -1 for one of the other inputs.
func resultIndex(in paint.FilterInput, i int) int {
	switch {
	case in == paint.FilterPrevious || int(in) > i:
		return i - 1
	case in > 0:
		return int(in) - 1
	}
	return -1
}

// releaseRead releases the results that primitive i read for the last time.
func (g *Resources) releaseRead(i int) {
	for k := range i {
		if g.filterLast[k] == i {
			releaseGroupTexture(g.filterResults[k])
		}
	}
}

// filterInput resolves in for primitive i.
func (g *Resources) filterInput(in paint.FilterInput, i int, source *groupTexture) filterSource {
	switch in {
	case paint.FilterSourceGraphic:
		return filterSource{tex: source}
	case paint.FilterSourceAlpha:
		return filterSource{tex: source, alpha: true}
	}
	if k := resultIndex(in, i); k >= 0 {
		return filterSource{tex: g.filterResults[k]}
	}
	if in == paint.FilterPrevious || in > 0 {
		// The first primitive reads the picture.
		return filterSource{tex: source}
	}
	return filterSource{}
}

func texOf(s filterSource) gpu.Texture {
	if s.tex == nil {
		return nil
	}
	return s.tex.tex
}

// readFlags returns the flags that read s as input slot.
func readFlags(s filterSource, slot int) uint32 {
	empty, alpha := gpu.EffectEmpty0, gpu.EffectAlpha0
	if slot == 1 {
		empty, alpha = gpu.EffectEmpty1, gpu.EffectAlpha1
	}
	switch {
	case s.tex == nil:
		return empty
	case s.alpha:
		return alpha
	}
	return 0
}

// moved returns d, a length of the filter, in pixels, as the first two floats
// of a parameter.
func moved(px geom.Affine, d geom.Offset) [4]float32 {
	return [4]float32{px.A*d.X + px.C*d.Y, px.B*d.X + px.D*d.Y}
}

// blurPass makes the pass that blurs in along axis with the standard deviation
// sigma, in pixels, into out. A sigma too small to blur copies.
func (g *Resources) blurPass(out *groupTexture, r groupRect, in filterSource, axis geom.Offset, sigma float32, flags uint32, sub image.Rectangle) {
	sigma = min(sigma, maxFilterSigma)
	var e gpu.EffectParams
	e[0] = [4]float32{axis.X, axis.Y}
	flags |= readFlags(in, 0)
	if sigma < boxSigma {
		radius := geom.Ceil32(3 * sigma)
		if radius < 1 {
			g.effectPass(out, r, gpu.EffectCopy, flags, &e, texOf(in), nil, sub)
			return
		}
		e[0][2], e[0][3] = sigma, radius
		g.effectPass(out, r, gpu.EffectGaussian, flags, &e, texOf(in), nil, sub)
		return
	}
	widths, before := gpu.BoxSizes(sigma)
	e[1] = [4]float32{float32(widths[0]), float32(widths[1]), float32(widths[2]), float32(before)}
	g.effectPass(out, r, gpu.EffectBoxes, flags, &e, texOf(in), nil, sub)
}

// blur blurs in by sx and sy into out: across into a target of its own, then
// down into out, or in one pass if only one axis blurs.
func (g *Resources) blur(out *groupTexture, r groupRect, in filterSource, sx, sy float32, flags uint32, sub image.Rectangle) {
	sx, sy = max(sx, 0), max(sy, 0)
	switch {
	case sx > 0 && sy > 0:
		across := g.takeGroupTexture(r.width, r.height)
		if across == nil {
			return
		}
		g.blurPass(across, r, in, geom.Offset{X: 1}, sx, flags, image.Rectangle{})
		g.blurPass(out, r, filterSource{tex: across}, geom.Offset{Y: 1}, sy, flags, sub)
		releaseGroupTexture(across)
	case sx > 0:
		g.blurPass(out, r, in, geom.Offset{X: 1}, sx, flags, sub)
	default:
		g.blurPass(out, r, in, geom.Offset{Y: 1}, sy, flags, sub)
	}
}

// dropShadow draws in over its alpha blurred, moved and colored, into out.
func (g *Resources) dropShadow(out *groupTexture, r groupRect, in filterSource, p *paint.FilterPrimitive, px geom.Affine,
	sx, sy float32, flags uint32, sub image.Rectangle) {
	alpha := filterSource{tex: in.tex, alpha: true}
	var blurred *groupTexture
	if p.StdDev.X*sx > 0 || p.StdDev.Y*sy > 0 {
		if blurred = g.takeGroupTexture(r.width, r.height); blurred == nil {
			return
		}
		// Alpha alone is the same in either space.
		g.blur(blurred, r, alpha, p.StdDev.X*sx, p.StdDev.Y*sy, 0, image.Rectangle{})
		alpha = filterSource{tex: blurred}
	}
	var e gpu.EffectParams
	e[0] = moved(px, p.Offset)
	c := p.Color
	e[1] = [4]float32{c.R * c.A, c.G * c.A, c.B * c.A, c.A}
	g.effectPass(out, r, gpu.EffectShadow, flags|readFlags(alpha, 0)|readFlags(in, 1), &e, texOf(alpha), texOf(in), sub)
	releaseGroupTexture(blurred)
}

// merge draws inputs over each other into out, pair by pair, so that they
// composite in the space of the primitive.
func (g *Resources) merge(out *groupTexture, r groupRect, inputs []paint.FilterInput, i int, source *groupTexture, flags uint32, sub image.Rectangle) {
	var e gpu.EffectParams
	switch len(inputs) {
	case 0:
		g.clearJob(out, r)
		return
	case 1:
		in := g.filterInput(inputs[0], i, source)
		g.effectPass(out, r, gpu.EffectCopy, flags|readFlags(in, 0), &e, texOf(in), nil, sub)
		return
	}
	below := g.filterInput(inputs[0], i, source)
	var held *groupTexture
	for k := 1; k < len(inputs); k++ {
		above := g.filterInput(inputs[k], i, source)
		target := out
		if k < len(inputs)-1 {
			if target = g.takeGroupTexture(r.width, r.height); target == nil {
				return
			}
		}
		e[0][0] = float32(gpu.CompositeOver)
		s := sub
		if target != out {
			s = image.Rectangle{}
		}
		g.effectPass(target, r, gpu.EffectComposite, flags|readFlags(above, 0)|readFlags(below, 1), &e, texOf(above), texOf(below), s)
		releaseGroupTexture(held)
		held = nil
		if target != out {
			held = target
		}
		below = filterSource{tex: target}
	}
}

// morphology erodes or dilates in by rx and ry pixels into out.
func (g *Resources) morphology(out *groupTexture, r groupRect, in filterSource, rx, ry int, dilate bool, flags uint32, sub image.Rectangle) {
	if dilate {
		flags |= gpu.EffectDilate
	}
	var e gpu.EffectParams
	switch {
	case rx <= 0 && ry <= 0:
		g.effectPass(out, r, gpu.EffectCopy, flags|readFlags(in, 0), &e, texOf(in), nil, sub)
	case rx > 0 && ry > 0:
		across := g.takeGroupTexture(r.width, r.height)
		if across == nil {
			return
		}
		e[0] = [4]float32{1, 0, float32(rx)}
		g.effectPass(across, r, gpu.EffectMorphology, flags|readFlags(in, 0), &e, texOf(in), nil, image.Rectangle{})
		e[0] = [4]float32{0, 1, float32(ry)}
		g.effectPass(out, r, gpu.EffectMorphology, flags, &e, across.tex, nil, sub)
		releaseGroupTexture(across)
	case rx > 0:
		e[0] = [4]float32{1, 0, float32(rx)}
		g.effectPass(out, r, gpu.EffectMorphology, flags|readFlags(in, 0), &e, texOf(in), nil, sub)
	default:
		e[0] = [4]float32{0, 1, float32(ry)}
		g.effectPass(out, r, gpu.EffectMorphology, flags|readFlags(in, 0), &e, texOf(in), nil, sub)
	}
}

// transfer maps in through the functions of p into out, by a table in a row of
// the ramps.
func (g *Resources) transfer(out *groupTexture, r groupRect, in filterSource, p *paint.FilterPrimitive, flags uint32, sub image.Rectangle) {
	for i := range gpu.RampWidth {
		c := float32(i) / (gpu.RampWidth - 1)
		for ch := range 4 {
			g.transferTable[i*4+ch] = unorm8(p.Transfer[ch].Apply(c))
		}
	}
	if g.ramps == nil {
		g.ramps = newRampTable()
	}
	row, ok := g.ramps.table(&g.transferTable, g.frame)
	if !ok {
		g.clearJob(out, r)
		return
	}
	var e gpu.EffectParams
	e[0][0] = float32(row)
	g.effectPass(out, r, gpu.EffectTransfer, flags|readFlags(in, 0), &e, texOf(in), rampsTexture{}, sub)
}

// clearJob makes the job that leaves out transparent.
func (g *Resources) clearJob(out *groupTexture, r groupRect) {
	b := g.newJob(out, 1)
	b.Clears = append(b.Clears, image.Rect(0, 0, r.width, r.height))
}

// unorm8 stores v as a channel of a render target does: clamped, rounded.
func unorm8(v float32) uint8 {
	switch {
	case v >= 1:
		return 255
	case v > 0:
		return uint8(v*255 + 0.5)
	}
	return 0
}

// rampsTexture stands in for the texture of the ramps, which Finish creates only
// after Encode.
type rampsTexture struct{}

// Destroy implements gpu.Texture.
func (rampsTexture) Destroy() {}
