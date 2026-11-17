package soft

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// effectPass is one InstanceEffect at one pixel: render/effect.hlsl, function
// by function. Keep the two in step. See docs/ADR/0114-gpu-layers-masks-filters.md.
type effectPass struct {
	in0, in1 *Texture
	flags    uint32
	width    int
	height   int
}

// effect is shade of render/effect.hlsl, after main of the epilogue of
// gowaku-shader: opacity 1 and no turn, so that it is the color itself.
func (f *fragment) effect() (vec4, float32) {
	op, flags, p := f.in.Effect()
	e := effectPass{in0: f.picture, in1: f.atlas, flags: flags, width: int(p[3][0]), height: int(p[3][1])}
	// pos is the pixel center, and the instance lies at the origin at a scale
	// of 1.
	x, y := f.pixelX, f.pixelY
	return e.encode(e.run(x, y, op, &p)), 1
}

func effectToLinear(c float32) float32 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return float32(math.Pow(float64((c+0.055)/1.055), 2.4))
}

func effectToSRGB(c float32) float32 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*float32(math.Pow(float64(c), float64(float32(1.0)/2.4))) - 0.055
}

func (e *effectPass) linear() bool { return e.flags&gpu.EffectLinear != 0 }

func (e *effectPass) decode(c vec4) vec4 {
	if !e.linear() || c[3] <= 0 {
		return c
	}
	a := c[3]
	return vec4{
		effectToLinear(saturate(c[0]/a)) * a,
		effectToLinear(saturate(c[1]/a)) * a,
		effectToLinear(saturate(c[2]/a)) * a,
		a,
	}
}

func (e *effectPass) encode(c vec4) vec4 {
	if !e.linear() || c[3] <= 0 {
		return c
	}
	a := c[3]
	return vec4{
		effectToSRGB(saturate(c[0]/a)) * a,
		effectToSRGB(saturate(c[1]/a)) * a,
		effectToSRGB(saturate(c[2]/a)) * a,
		a,
	}
}

func (e *effectPass) outside(x, y int) bool {
	return x < 0 || y < 0 || x >= e.width || y >= e.height
}

// load is Load of t: zero outside the texture or without one.
func load(t *Texture, x, y int) vec4 {
	if t == nil || x < 0 || y < 0 || x >= t.width || y >= t.height {
		return vec4{}
	}
	return t.texel(x, y)
}

func (e *effectPass) read(t *Texture, x, y int, empty, alpha uint32) vec4 {
	if e.flags&empty != 0 || e.outside(x, y) {
		return vec4{}
	}
	c := load(t, x, y)
	if e.flags&alpha != 0 {
		return vec4{0, 0, 0, c[3]}
	}
	return e.decode(c)
}

func (e *effectPass) in0At(x, y int) vec4 {
	return e.read(e.in0, x, y, gpu.EffectEmpty0, gpu.EffectAlpha0)
}

func (e *effectPass) in1At(x, y int) vec4 {
	return e.read(e.in1, x, y, gpu.EffectEmpty1, gpu.EffectAlpha1)
}

// moved is effectMoved.
func (e *effectPass) moved(x, y int, dx, dy float32) vec4 {
	wx, wy := geom.Floor32(dx), geom.Floor32(dy)
	fx, fy := dx-wx, dy-wy
	sx, sy := x-int(wx), y-int(wy)
	top := lerp(e.in0At(sx, sy), e.in0At(sx-1, sy), fx)
	bottom := lerp(e.in0At(sx, sy-1), e.in0At(sx-1, sy-1), fx)
	return lerp(top, bottom, fy)
}

func unpremultiplied(c vec4) vec4 {
	if c[3] <= 0 {
		return vec4{}
	}
	return vec4{c[0] / c[3], c[1] / c[3], c[2] / c[3], c[3]}
}

// lookup is effectLookup.
func (e *effectPass) lookup(v float32, row, channel int) float32 {
	x := saturate(v) * 255
	i := min(geom.Floor32(x), 254)
	low := load(e.in1, int(i), row)[channel]
	high := load(e.in1, int(i)+1, row)[channel]
	return low + (x-i)*(high-low)
}

func compositeChannel(a, b, aa, ba float32, op gpu.CompositeOperator, k [4]float32) float32 {
	switch op {
	case gpu.CompositeIn:
		return a * ba
	case gpu.CompositeOut:
		return a * (1 - ba)
	case gpu.CompositeAtop:
		return a*ba + b*(1-aa)
	case gpu.CompositeXor:
		return a*(1-ba) + b*(1-aa)
	case gpu.CompositeLighter:
		return min(a+b, 1)
	case gpu.CompositeArithmetic:
		return saturate(k[0]*a*b + k[1]*a + k[2]*b + k[3])
	}
	return a + b*(1-aa)
}

func dot4(a, b [4]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3] }

// run is effect.
func (e *effectPass) run(x, y int, op gpu.EffectOp, p *gpu.EffectParams) vec4 {
	p0, p1 := p[0], p[1]
	switch op {
	case gpu.EffectCopy:
		return e.moved(x, y, p0[0], p0[1])
	case gpu.EffectFlood:
		return e.decode(vec4(p0))
	case gpu.EffectMatrix:
		u := unpremultiplied(e.in0At(x, y))
		var v vec4
		for k := range 4 {
			v[k] = saturate(dot4(p[4+k], u) + p0[k])
		}
		return vec4{v[0] * v[3], v[1] * v[3], v[2] * v[3], v[3]}
	case gpu.EffectTransfer:
		u := unpremultiplied(e.in0At(x, y))
		row := int(p0[0])
		var v vec4
		for k := range 4 {
			v[k] = saturate(e.lookup(u[k], row, k))
		}
		return vec4{v[0] * v[3], v[1] * v[3], v[2] * v[3], v[3]}
	case gpu.EffectComposite:
		a, b := e.in0At(x, y), e.in1At(x, y)
		oper := gpu.CompositeOperator(p0[0])
		var c vec4
		for k := range 4 {
			c[k] = compositeChannel(a[k], b[k], a[3], b[3], oper, p1)
		}
		if oper == gpu.CompositeArithmetic {
			for k := range 3 {
				c[k] = min(c[k], c[3])
			}
		}
		return c
	case gpu.EffectBlend:
		a, b := e.in0At(x, y), e.in1At(x, y)
		mode := gpu.BlendMode(p0[0])
		var s, d rgb
		for k := range 3 {
			if a[3] > 0 {
				s[k] = a[k] / a[3]
			}
			if b[3] > 0 {
				d[k] = b[k] / b[3]
			}
		}
		mixed := s
		if mode == gpu.BlendScreen {
			for k := range 3 {
				mixed[k] = s[k] + d[k] - s[k]*d[k]
			}
		} else if mode >= gpu.BlendMultiply {
			var ds, ss rgb
			for k := range 3 {
				ds[k], ss[k] = saturate(d[k]), saturate(s[k])
			}
			mixed = blendMix(ds, ss, mode)
		}
		var c vec4
		for k := range 3 {
			c[k] = (1-b[3])*a[k] + (1-a[3])*b[k] + a[3]*b[3]*mixed[k]
		}
		c[3] = a[3] + b[3] - a[3]*b[3]
		return c
	case gpu.EffectMorphology:
		ax, ay := int(p0[0]), int(p0[1])
		radius := int(p0[2])
		dilate := e.flags&gpu.EffectDilate != 0
		c := e.in0At(x, y)
		for k := -radius; k <= radius; k++ {
			sx, sy := x+ax*k, y+ay*k
			if e.outside(sx, sy) {
				continue
			}
			v := e.in0At(sx, sy)
			for i := range 4 {
				if dilate {
					c[i] = max(c[i], v[i])
				} else {
					c[i] = min(c[i], v[i])
				}
			}
		}
		return c
	case gpu.EffectGaussian:
		ax, ay := int(p0[0]), int(p0[1])
		falloff := -0.5 / (p0[2] * p0[2])
		radius := int(p0[3])
		var sum vec4
		var total float32
		for k := -radius; k <= radius; k++ {
			w := exp(float32(k*k) * falloff)
			sum = sum.add(e.in0At(x+ax*k, y+ay*k).times(w))
			total += w
		}
		return vec4{sum[0] / total, sum[1] / total, sum[2] / total, sum[3] / total}
	case gpu.EffectBoxes:
		ax, ay := int(p0[0]), int(p0[1])
		first := int(p1[3])
		d := [3]float32{p1[0], p1[1], p1[2]}
		span := int(p1[0]+p1[1]+p1[2]) - 2
		var sum vec4
		for m := range span {
			sum = sum.add(e.in0At(x+ax*(m-first), y+ay*(m-first)).times(gpu.BoxWeight(float32(m), d)))
		}
		return sum
	case gpu.EffectMask:
		c, m := load(e.in0, x, y), load(e.in1, x, y)
		if e.outside(x, y) {
			return vec4{}
		}
		v := m[3]
		if e.flags&gpu.EffectLuminance != 0 {
			v = m[0]*0.2125 + m[1]*0.7154 + m[2]*0.0721
		}
		return c.times(v)
	}
	// gpu.EffectShadow
	a := e.moved(x, y, p0[0], p0[1])[3]
	top := e.in1At(x, y)
	return top.add(e.decode(vec4(p1)).times(a * (1 - top[3])))
}
