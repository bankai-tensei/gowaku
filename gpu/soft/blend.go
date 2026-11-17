package soft

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// output runs the pixel shader for the fragment and returns what it hands the
// output merger: the shape, shaped for the blend mode of the draw call. It is a
// port of main and blendOutput in gpu/shaders/sdf.ps.hlsl.
func (c *canvas) output(f *fragment, st *quadState) [4]uint8 {
	color, coverage := f.shade()
	switch mode := st.blend; {
	case mode == gpu.BlendSrcOver:
	case mode == gpu.BlendClear:
		color = vec4{0, 0, 0, coverage}
	case mode == gpu.BlendDstIn || mode == gpu.BlendModulate:
		color = vec4{coverage - color[0], coverage - color[1], coverage - color[2], coverage - color[3]}
	case mode.ReadsBackdrop():
		color = readBackdrop(color, coverage, mode, c.backdropAt(f.pixelX, f.pixelY))
	}
	return color.stored()
}

// merge applies the fixed-function blend of mode to sample i. Like the GPUs, it
// takes in src rounded to 8 bits.
func (c *canvas) merge(i int, src [4]uint8, mode gpu.BlendMode) {
	if mode == gpu.BlendSrcOver {
		c.blend(i, src)
		return
	}
	p := c.color[i*4 : i*4+4]
	srcFactor, dstFactor := mode.Factors()
	for k := range p {
		s, d := int(src[k]), int(p[k])
		v := s*factor(srcFactor, src, p, k) + d*factor(dstFactor, src, p, k)
		p[k] = uint8(min((v+127)/255, 255))
	}
}

// factor returns f for channel k, from 0 to 255. In the alpha channel a factor
// of color is that of alpha.
func factor(f gpu.BlendFactor, src [4]uint8, dst []uint8, k int) int {
	switch f {
	case gpu.FactorOne:
		return 255
	case gpu.FactorSrcColor:
		return int(src[k])
	case gpu.FactorInvSrcColor:
		return 255 - int(src[k])
	case gpu.FactorSrcAlpha:
		return int(src[3])
	case gpu.FactorInvSrcAlpha:
		return 255 - int(src[3])
	case gpu.FactorDstAlpha:
		return int(dst[3])
	case gpu.FactorInvDstAlpha:
		return 255 - int(dst[3])
	}
	return 0
}

// copyBackdrop copies r of the canvas into the backdrop, as the GPU backends
// copy the target before a draw call that reads it. A multisampled canvas is
// resolved on the way, rounding as resolve does.
func (c *canvas) copyBackdrop(r image.Rectangle) {
	if n := c.width * c.height * 4; len(c.backdrop) != n {
		c.backdrop = make([]uint8, n)
	}
	samples := len(c.positions)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			first := c.sampleIndex(x, y) * 4
			out := c.backdrop[(y*c.width+x)*4:]
			for k := range 4 {
				sum := (samples - 1) / 2
				for s := range samples {
					sum += int(c.color[first+s*4+k])
				}
				out[k] = uint8(sum / samples)
			}
		}
	}
}

// backdropAt returns the backdrop at a pixel, premultiplied.
func (c *canvas) backdropAt(x, y int) vec4 {
	if len(c.backdrop) == 0 || x < 0 || y < 0 || x >= c.width || y >= c.height {
		return vec4{}
	}
	p := c.backdrop[(y*c.width+x)*4:]
	return vec4{float32(p[0]) / 255, float32(p[1]) / 255, float32(p[2]) / 255, float32(p[3]) / 255}
}

// readBackdrop composes color, which has coverage applied, with the backdrop d.
// It is a port of readBackdrop in gpu/shaders/sdf.ps.hlsl.
func readBackdrop(color vec4, coverage float32, mode gpu.BlendMode, d vec4) vec4 {
	rest := d.times(1 - coverage)
	switch mode {
	case gpu.BlendSrc:
		return color.add(rest)
	case gpu.BlendSrcIn:
		return color.times(d[3]).add(rest)
	case gpu.BlendSrcOut:
		return color.times(1 - d[3]).add(rest)
	case gpu.BlendDstAtop:
		return color.times(1 - d[3]).add(d.times(color[3])).add(rest)
	}
	if color[3] <= 0 {
		return vec4{}
	}
	var s, b rgb
	for k := range 3 {
		s[k] = saturate(color[k] / color[3])
		if d[3] > 0 {
			b[k] = saturate(d[k] / d[3])
		}
	}
	mixed := blendMix(b, s, mode)
	var out vec4
	for k := range 3 {
		out[k] = ((1-d[3])*s[k] + d[3]*saturate(mixed[k])) * color[3]
	}
	out[3] = color[3]
	return out
}

// rgb is a color that is not premultiplied, without alpha.
type rgb [3]float32

// blendMix is B(Cb, Cs) of the W3C Compositing and Blending Level 1 formulas.
func blendMix(b, s rgb, mode gpu.BlendMode) rgb {
	each := func(fn func(b, s float32) float32) rgb {
		return rgb{fn(b[0], s[0]), fn(b[1], s[1]), fn(b[2], s[2])}
	}
	switch mode {
	case gpu.BlendMultiply:
		return each(func(b, s float32) float32 { return b * s })
	case gpu.BlendOverlay:
		return each(func(b, s float32) float32 { return multiplyOrScreen(s, b) })
	case gpu.BlendDarken:
		return each(func(b, s float32) float32 { return min(b, s) })
	case gpu.BlendLighten:
		return each(func(b, s float32) float32 { return max(b, s) })
	case gpu.BlendColorDodge:
		return each(colorDodge)
	case gpu.BlendColorBurn:
		return each(colorBurn)
	case gpu.BlendHardLight:
		return each(multiplyOrScreen)
	case gpu.BlendSoftLight:
		return each(softLight)
	case gpu.BlendDifference:
		return each(func(b, s float32) float32 {
			if b > s {
				return b - s
			}
			return s - b
		})
	case gpu.BlendExclusion:
		return each(func(b, s float32) float32 { return b + s - 2*b*s })
	case gpu.BlendHue:
		return setLuminosity(setSaturation(s, saturation(b)), luminosity(b))
	case gpu.BlendSaturation:
		return setLuminosity(setSaturation(b, saturation(s)), luminosity(b))
	case gpu.BlendColor:
		return setLuminosity(s, luminosity(b))
	}
	return setLuminosity(b, luminosity(s))
}

// multiplyOrScreen is HardLight: b multiplied by 2s up to the middle, and
// screened by 2s - 1 above it.
func multiplyOrScreen(b, s float32) float32 {
	if s <= 0.5 {
		return b * 2 * s
	}
	t := 2*s - 1
	return b + t - b*t
}

func colorDodge(b, s float32) float32 {
	switch {
	case b <= 0:
		return 0
	case s >= 1:
		return 1
	}
	return min(1, b/(1-s))
}

func colorBurn(b, s float32) float32 {
	switch {
	case b >= 1:
		return 1
	case s <= 0:
		return 0
	}
	return 1 - min(1, (1-b)/s)
}

func softLight(b, s float32) float32 {
	if s <= 0.5 {
		return b - (1-2*s)*b*(1-b)
	}
	var d float32
	if b <= 0.25 {
		d = ((16*b-12)*b + 4) * b
	} else {
		d = geom.Sqrt32(b)
	}
	return b + (2*s-1)*(d-b)
}

func luminosity(c rgb) float32 { return 0.3*c[0] + 0.59*c[1] + 0.11*c[2] }

func clipColor(c rgb) rgb {
	l := luminosity(c)
	n := min(c[0], c[1], c[2])
	x := max(c[0], c[1], c[2])
	if n < 0 {
		for k := range c {
			c[k] = l + (c[k]-l)*l/(l-n)
		}
	}
	if x > 1 {
		for k := range c {
			c[k] = l + (c[k]-l)*(1-l)/(x-l)
		}
	}
	return c
}

func setLuminosity(c rgb, l float32) rgb {
	d := l - luminosity(c)
	return clipColor(rgb{c[0] + d, c[1] + d, c[2] + d})
}

func saturation(c rgb) float32 { return max(c[0], c[1], c[2]) - min(c[0], c[1], c[2]) }

// setSaturation stretches c to saturation s, which moves its largest channel to
// s and its smallest to 0.
func setSaturation(c rgb, s float32) rgb {
	n := min(c[0], c[1], c[2])
	x := max(c[0], c[1], c[2])
	if x <= n {
		return rgb{}
	}
	for k := range c {
		c[k] = (c[k] - n) * s / (x - n)
	}
	return c
}
