package soft

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// vec4 is a color, premultiplied unless a function says otherwise.
type vec4 [4]float32

func (a vec4) add(b vec4) vec4 {
	return vec4{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
}

func (a vec4) times(s float32) vec4 {
	return vec4{a[0] * s, a[1] * s, a[2] * s, a[3] * s}
}

// stored converts a to what a render target takes in.
func (a vec4) stored() [4]uint8 {
	return [4]uint8{unorm8(a[0]), unorm8(a[1]), unorm8(a[2]), unorm8(a[3])}
}

func lerp(a, b vec4, t float32) vec4 {
	return vec4{
		a[0] + t*(b[0]-a[0]),
		a[1] + t*(b[1]-a[1]),
		a[2] + t*(b[2]-a[2]),
		a[3] + t*(b[3]-a[3]),
	}
}

// saturate clamps v to 0..1, and turns NaN into 0 as the GPU does.
func saturate(v float32) float32 {
	switch {
	case v >= 1:
		return 1
	case v > 0:
		return v
	}
	return 0
}

func sign(v float32) float32 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// exp is what the HLSL intrinsic compiles to.
func exp(v float32) float32 { return float32(math.Exp2(float64(v * 1.442695))) }

// round is the HLSL intrinsic, which rounds halves to even.
func round(v float32) float32 { return float32(math.RoundToEven(float64(v))) }

// fragment is the input of the pixel shader. It is a port of
// gpu/shaders/sdf.ps.hlsl, function by function: keep the two in step.
type fragment struct {
	in    *gpu.Instance
	scale float32
	// atlas and picture are the textures of the draw call, and ramps those of
	// the batch, which can be nil.
	atlas, picture, ramps *Texture
	// filtered is the blurred picture of a draw call that blurs.
	filtered *Texture
	// logical is the interpolated varying.
	logical geom.Offset
	// pixelX and pixelY are floor(SV_Position).
	pixelX, pixelY int
	// shader is the custom shader of the draw call, and input and samplers
	// what custom hands its fallback.
	shader   *gpu.Shader
	input    gpu.ShaderInput
	samplers [2]shaderSampler
}

// shade is shade in the pixel shader: the color of the shape with the
// coverage of its edge applied, and that coverage.
func (f *fragment) shade() (vec4, float32) {
	switch f.in.Kind.Base() {
	case gpu.InstanceShadow:
		return f.shadow()
	case gpu.InstanceGlyph:
		return f.glyph()
	case gpu.InstanceCover:
		return f.fill(), 1
	case gpu.InstanceImage:
		return f.image()
	case gpu.InstanceBackdrop:
		return f.backdropBlur()
	case gpu.InstanceFiltered:
		return f.filteredImage()
	case gpu.InstanceShader:
		return f.custom()
	case gpu.InstanceEffect:
		return f.effect()
	}
	return f.rrect()
}

// premultiply turns a color of an instance into a shader color.
func premultiply(c geom.Color) vec4 {
	return vec4{c.R * c.A, c.G * c.A, c.B * c.A, c.A}
}

func roundedBoxDistance(p, halfSize geom.Offset, radii [4]float32) float32 {
	top, bottom := radii[0], radii[3]
	if p.X > 0 {
		top, bottom = radii[1], radii[2]
	}
	radius := top
	if p.Y > 0 {
		radius = bottom
	}
	radius = min(radius, min(halfSize.X, halfSize.Y))
	qx := geom.Abs32(p.X) - halfSize.X + radius
	qy := geom.Abs32(p.Y) - halfSize.Y + radius
	outX, outY := max(qx, 0), max(qy, 0)
	return min(max(qx, qy), 0) + geom.Sqrt32(outX*outX+outY*outY) - radius
}

func (f *fragment) fill() vec4 {
	in := f.in
	shape := in.Gradient.Shape()
	if shape == gpu.GradientNone {
		return premultiply(in.Color)
	}
	axisX, axisY := in.GradientEnd.X-in.GradientStart.X, in.GradientEnd.Y-in.GradientStart.Y
	px, py := f.logical.X-in.GradientStart.X, f.logical.Y-in.GradientStart.Y
	if m := &in.GradientMatrix; *m != [4]float32{} {
		px, py = m[0]*px+m[2]*py, m[1]*px+m[3]*py
	}
	lengthSquared := max(axisX*axisX+axisY*axisY, 1e-6)
	var t float32
	switch shape {
	case gpu.GradientSweep:
		t = sweep(px, py, axisX, axisY)
	case gpu.GradientLinear:
		t = (px*axisX + py*axisY) / lengthSquared
	case gpu.GradientFocal:
		t = gpu.FocalOffset(geom.Offset{X: px, Y: py}, geom.Offset{X: axisX, Y: axisY},
			geom.Offset{X: in.StopOffsets[1], Y: in.StopOffsets[2]})
	default:
		t = geom.Sqrt32((px*px + py*py) / lengthSquared)
	}
	t = gpu.SpreadOffset(t, in.Gradient)
	if in.Gradient&gpu.GradientRamp != 0 {
		return f.rampTexels(t, int(in.StopOffsets[0]))
	}
	return f.ramp(t)
}

// sweep is the turn of p clockwise from the axis, from 0 to 1.
func sweep(px, py, axisX, axisY float32) float32 {
	angle := float32(math.Atan2(float64(axisX*py-axisY*px), float64(px*axisX+py*axisY)))
	t := angle * 0.15915494
	if t < 0 {
		return t + 1
	}
	return t
}

// rampTexels reads t from row of the ramps: the two texels around it, blended
// linearly.
func (f *fragment) rampTexels(t float32, row int) vec4 {
	if f.ramps == nil || row < 0 || row >= f.ramps.height {
		return vec4{}
	}
	x := saturate(t) * (gpu.RampWidth - 1)
	i := min(geom.Floor32(x), gpu.RampWidth-2)
	return lerp(f.ramps.texel(int(i), row), f.ramps.texel(int(i)+1, row), x-i)
}

// stopWeight is how far t has come from one stop to the next.
func stopWeight(t, from, to float32) float32 {
	return saturate((t - from) / max(to-from, 1e-6))
}

// ramp blends the four stops in turn. Each lerp holds until t reaches the
// stop before it, so the stops of one segment decide its color.
func (f *fragment) ramp(t float32) vec4 {
	in := f.in
	o := &in.StopOffsets
	c := premultiply(in.Color)
	c = lerp(c, premultiply(in.MidColors[0]), stopWeight(t, o[0], o[1]))
	c = lerp(c, premultiply(in.MidColors[1]), stopWeight(t, o[1], o[2]))
	return lerp(c, premultiply(in.EndColor), stopWeight(t, o[2], o[3]))
}

// centered returns the position of the fragment and the half size of the rectangle,
// relative to the center of the rectangle.
func (f *fragment) centered() (p, halfSize geom.Offset) {
	r := f.in.Rect
	halfSize = geom.Offset{X: r.W * 0.5, Y: r.H * 0.5}
	p = geom.Offset{X: f.logical.X - r.X - halfSize.X, Y: f.logical.Y - r.Y - halfSize.Y}
	return p, halfSize
}

// roundedBoxNormal is the outward direction in which roundedBoxDistance grows.
func roundedBoxNormal(p, halfSize geom.Offset, radii [4]float32) geom.Offset {
	top, bottom := radii[0], radii[3]
	if p.X > 0 {
		top, bottom = radii[1], radii[2]
	}
	radius := top
	if p.Y > 0 {
		radius = bottom
	}
	radius = min(radius, min(halfSize.X, halfSize.Y))
	qx := geom.Abs32(p.X) - halfSize.X + radius
	qy := geom.Abs32(p.Y) - halfSize.Y + radius
	n := geom.Offset{Y: 1}
	if qx > qy {
		n = geom.Offset{X: 1}
	}
	if qx > 0 && qy > 0 {
		length := geom.Sqrt32(qx*qx + qy*qy)
		n = geom.Offset{X: qx / length, Y: qy / length}
	}
	if p.X < 0 {
		n.X = -n.X
	}
	if p.Y < 0 {
		n.Y = -n.Y
	}
	return n
}

// turned reports whether the instance has a transform.
func (f *fragment) turned() bool { return f.in.Transform != [6]float32{} }

// edgeStep is how much a distance along n changes over one physical pixel of
// the surface.
func (f *fragment) edgeStep(n geom.Offset) float32 {
	t := &f.in.Transform
	det := t[0]*t[3] - t[1]*t[2]
	x := n.X*t[3] - n.Y*t[1]
	y := -n.X*t[2] + n.Y*t[0]
	return max(geom.Sqrt32(x*x+y*y)/(geom.Abs32(det)*f.scale), 1e-12)
}

func (f *fragment) rrect() (vec4, float32) {
	p, halfSize := f.centered()
	distance := roundedBoxDistance(p, halfSize, f.in.Radii)
	outer := saturate(0.5 - distance*f.scale)
	inner := saturate(0.5 - (distance+f.in.BorderWidth)*f.scale)
	if f.turned() {
		step := f.edgeStep(roundedBoxNormal(p, halfSize, f.in.Radii))
		outer = saturate(0.5 - distance/step)
		inner = saturate(0.5 - (distance+f.in.BorderWidth)/step)
	}
	return f.fill().times(inner).add(premultiply(f.in.BorderColor).times(outer - inner)), outer
}

func gaussian(x, sigma float32) float32 {
	return exp(-(x*x)/(2*sigma*sigma)) / (2.5066282746 * sigma)
}

// erf follows Abramowitz and Stegun 7.1.27.
func erf(x float32) float32 {
	s := sign(x)
	a := geom.Abs32(x)
	d := 1 + (0.278393+(0.230389+0.078108*(a*a))*a)*a
	d *= d
	return s - s/(d*d)
}

func shadowSlice(x, y, sigma, radius float32, halfSize geom.Offset) float32 {
	delta := min(halfSize.Y-radius-geom.Abs32(y), 0)
	curved := halfSize.X - radius + geom.Sqrt32(max(0, radius*radius-delta*delta))
	inverse := 0.70710678 / sigma
	low := 0.5 + 0.5*erf((x-curved)*inverse)
	high := 0.5 + 0.5*erf((x+curved)*inverse)
	return high - low
}

func shadowCoverage(p, halfSize geom.Offset, radius, sigma float32) float32 {
	low := p.Y - halfSize.Y
	high := p.Y + halfSize.Y
	start := min(max(-3*sigma, low), high)
	end := min(max(3*sigma, low), high)
	step := (end - start) / 6
	y := start + step*0.5
	var value float32
	for range 6 {
		value += shadowSlice(p.X, p.Y-y, sigma, radius, halfSize) * gaussian(y, sigma) * step
		y += step
	}
	return value
}

func (f *fragment) shadow() (vec4, float32) {
	p, halfSize := f.centered()
	radii := &f.in.Radii
	radius := max(max(radii[0], radii[1]), max(radii[2], radii[3]))
	radius = min(radius, min(halfSize.X, halfSize.Y))
	sigma := f.in.Blur * 0.5
	coverage := shadowCoverage(p, halfSize, radius, sigma)
	return premultiply(f.in.Color).times(coverage), coverage
}

// glyphTexel reads the atlas at (x, y), and nothing outside the glyph.
func (f *fragment) glyphTexel(x, y, firstX, firstY, width, height int) float32 {
	if x < firstX || y < firstY || x >= firstX+width || y >= firstY+height {
		return 0
	}
	return f.atlas.load(x, y)
}

// turnedGlyph filters the four texels around the fragment.
func (f *fragment) turnedGlyph() float32 {
	in := f.in
	firstX, firstY := int(in.Radii[0]), int(in.Radii[1])
	width, height := int(round(in.Rect.W*f.scale)), int(round(in.Rect.H*f.scale))
	atX := in.Radii[0] + (f.logical.X-in.Rect.X)*f.scale - 0.5
	atY := in.Radii[1] + (f.logical.Y-in.Rect.Y)*f.scale - 0.5
	baseX, baseY := geom.Floor32(atX), geom.Floor32(atY)
	fx, fy := atX-baseX, atY-baseY
	x, y := int(baseX), int(baseY)
	t := func(dx, dy int) float32 { return f.glyphTexel(x+dx, y+dy, firstX, firstY, width, height) }
	top := t(0, 0) + fx*(t(1, 0)-t(0, 0))
	bottom := t(0, 1) + fx*(t(1, 1)-t(0, 1))
	return top + fy*(bottom-top)
}

func (f *fragment) glyph() (vec4, float32) {
	in := f.in
	if f.turned() {
		coverage := f.turnedGlyph()
		return premultiply(in.Color).times(coverage), coverage
	}
	originX, originY := int(round(in.Rect.X*f.scale)), int(round(in.Rect.Y*f.scale))
	x := int(in.Radii[0]) + f.pixelX - originX
	y := int(in.Radii[1]) + f.pixelY - originY
	coverage := f.atlas.load(x, y)
	return premultiply(in.Color).times(coverage), coverage
}

func (f *fragment) image() (vec4, float32) {
	in := f.in
	tx := (f.logical.X - in.Rect.X) / in.Rect.W
	ty := (f.logical.Y - in.Rect.Y) / in.Rect.H
	coverage := float32(1)
	if f.turned() {
		tx, ty = saturate(tx), saturate(ty)
		p, halfSize := f.centered()
		var square [4]float32
		distance := roundedBoxDistance(p, halfSize, square)
		coverage = saturate(0.5 - distance/f.edgeStep(roundedBoxNormal(p, halfSize, square)))
	}
	u := in.GradientStart.X + tx*(in.GradientEnd.X-in.GradientStart.X)
	v := in.GradientStart.Y + ty*(in.GradientEnd.Y-in.GradientStart.Y)
	var c vec4
	switch in.ImageFilter() {
	case gpu.FilterNearest:
		c = f.picture.nearestTexel(u, v)
	case gpu.FilterMipmap:
		c = f.mipmapped(u, v)
	default:
		c = f.picture.sample(u, v)
	}
	return c.times(in.Color.A * coverage), coverage
}

// nearestBias is NEAREST_BIAS: it moves a position on the edge between two
// texels onto the texel after it.
const nearestBias = 1.0 / 256

// minLevelOfDetail is MIN_LOD, below which a mipmapped image is not shrunk.
const minLevelOfDetail = 1.0 / 256

// nearestTexel reads the texel of level 0 under (u, v).
func (t *Texture) nearestTexel(u, v float32) vec4 {
	if t == nil || t.width == 0 {
		return vec4{}
	}
	x := int(geom.Floor32(u*float32(t.width) + nearestBias))
	y := int(geom.Floor32(v*float32(t.height) + nearestBias))
	return t.texel(min(max(x, 0), t.width-1), min(max(y, 0), t.height-1))
}

// bilinearLevel blends the four texels of a level around (u, v) with exact
// weights.
func (t *Texture) bilinearLevel(u, v float32, level int) vec4 {
	width, height := t.levelSize(level)
	atX, atY := u*float32(width)-0.5, v*float32(height)-0.5
	baseX, baseY := geom.Floor32(atX), geom.Floor32(atY)
	fx, fy := atX-baseX, atY-baseY
	x, y := int(baseX), int(baseY)
	texel := func(dx, dy int) vec4 {
		return t.levelTexel(level, min(max(x+dx, 0), width-1), min(max(y+dy, 0), height-1))
	}
	top := lerp(texel(0, 0), texel(1, 0), fx)
	bottom := lerp(texel(0, 1), texel(1, 1), fx)
	return lerp(top, bottom, fy)
}

// levelOfDetail is log2 of how many texels of level 0 one physical pixel spans
// in the direction where it spans the most.
func (f *fragment) levelOfDetail(width, height int) float32 {
	in := f.in
	texelsX := (in.GradientEnd.X - in.GradientStart.X) * float32(width) / in.Rect.W
	texelsY := (in.GradientEnd.Y - in.GradientStart.Y) * float32(height) / in.Rect.H
	var stretch float32
	if f.turned() {
		t := &in.Transform
		det := max(geom.Abs32(t[0]*t[3]-t[1]*t[2]), 1e-12)
		m := [4]float32{texelsX * t[3] / det, -texelsX * t[2] / det, -texelsY * t[1] / det, texelsY * t[0] / det}
		sum := m[0]*m[0] + m[1]*m[1] + m[2]*m[2] + m[3]*m[3]
		mdet := m[0]*m[3] - m[1]*m[2]
		stretch = geom.Sqrt32((sum + geom.Sqrt32(max(sum*sum-4*mdet*mdet, 0))) * 0.5)
	} else {
		stretch = max(geom.Abs32(texelsX), geom.Abs32(texelsY))
	}
	return float32(math.Log2(float64(stretch / f.scale)))
}

// mipmapped blends the two levels around the level of detail, each read
// bilinearly. Where the image does not shrink, it reads as FilterLinear does.
func (f *fragment) mipmapped(u, v float32) vec4 {
	t := f.picture
	if t == nil || t.width == 0 {
		return vec4{}
	}
	lod := f.levelOfDetail(t.width, t.height)
	if !(lod > minLevelOfDetail) {
		return t.sample(u, v)
	}
	lod = min(lod, float32(t.levels()-1))
	low := geom.Floor32(lod)
	c := t.bilinearLevel(u, v, int(low))
	if w := lod - low; w > 0 {
		c = lerp(c, t.bilinearLevel(u, v, int(low)+1), w)
	}
	return c
}
