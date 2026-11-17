package gpu

import "image"

// ImageFilter selects how an InstanceImage reads its image. The instance keeps
// it in Gradient, as an image has no gradient; see SetImageFilter. The zero
// value is what layers and color glyphs, which have one level, use.
type ImageFilter uint32

// The image filters. See docs/ADR/0045-shaders-svg-animated-images.md.
const (
	// FilterLinear blends the four texels of level 0 around the pixel.
	FilterLinear ImageFilter = iota
	// FilterNearest takes the texel of level 0 under the pixel.
	FilterNearest
	// FilterMipmap blends the bilinear results of the two levels around the
	// level of detail: log2 of the most texels of level 0 that one physical
	// pixel spans in any direction, transform included. Where the image does
	// not shrink, it is FilterLinear. The image should be a
	// TextureRGBA8Mipmapped; with one level it reads level 0.
	FilterMipmap
)

// SetImageFilter stores f in an InstanceImage.
func (in *Instance) SetImageFilter(f ImageFilter) { in.Gradient = GradientMode(f) }

// ImageFilter returns the filter of an InstanceImage.
func (in *Instance) ImageFilter() ImageFilter { return ImageFilter(in.Gradient) }

// MipLevels returns how many levels the full mip chain of an image of the given
// size has: halving down to one texel on the longer side.
func MipLevels(width, height int) int {
	n := 1
	for s := max(width, height); s > 1; s >>= 1 {
		n++
	}
	return n
}

// MipSize returns the size of a level of the mip chain: half of the level above,
// rounded down, and never less than one texel. This is the rule of Direct3D and
// Vulkan.
func MipSize(width, height, level int) (w, h int) {
	return max(width>>level, 1), max(height>>level, 1)
}

// BuildMips returns levels 1 and up of the mip chain of a premultiplied RGBA8
// image, one after the other without gaps. pix holds level 0 with the given
// stride in bytes. A texel of a level is the mean of the texels of the level
// above that it covers, weighted by the area it covers of each: the 2×2 box of
// an even size, and three texels with partial weights across an odd one, so
// that no texel is left out. Every backend uploads the same levels, so they
// filter alike. See docs/ADR/0045-shaders-svg-animated-images.md.
func BuildMips(pix []byte, stride, width, height int) []byte {
	levels := MipLevels(width, height)
	total := 0
	for k := 1; k < levels; k++ {
		w, h := MipSize(width, height, k)
		total += w * h * 4
	}
	out := make([]byte, total)
	src, srcStride, sw, sh := pix, stride, width, height
	at := 0
	var xs, ys []mipTap
	var rows []float32
	for k := 1; k < levels; k++ {
		w, h := MipSize(width, height, k)
		dst := out[at : at+w*h*4]
		if sw == 2*w && sh == 2*h {
			halveEven(dst, w, h, src, srcStride)
		} else {
			xs = mipTaps(xs[:0], sw, w)
			ys = mipTaps(ys[:0], sh, h)
			rows = halve(dst, w, src, srcStride, xs, ys, rows)
		}
		src, srcStride, sw, sh = dst, w*4, w, h
		at += w * h * 4
	}
	return out
}

// MipRegion returns the whole of a level, for backends that upload one level
// at a time.
func MipRegion(width, height, level int) image.Rectangle {
	w, h := MipSize(width, height, level)
	return image.Rect(0, 0, w, h)
}

// mipTap says which texels of the level above make one texel of a level along
// one axis, and how much each weighs.
type mipTap struct {
	first  int
	count  int
	weight [4]float32
}

// mipTaps returns, for each of the dst texels along an axis, the src texels it
// covers. src is dst, twice dst or twice dst plus one.
func mipTaps(taps []mipTap, src, dst int) []mipTap {
	ratio := float32(src) / float32(dst)
	for i := range dst {
		start, end := float32(i)*ratio, float32(i+1)*ratio
		if i == dst-1 {
			end = float32(src)
		}
		t := mipTap{first: int(start)}
		for x := t.first; x < src && float32(x) < end && t.count < len(t.weight); x++ {
			lo, hi := max(float32(x), start), min(float32(x+1), end)
			t.weight[t.count] = (hi - lo) / ratio
			t.count++
		}
		taps = append(taps, t)
	}
	return taps
}

// halve writes the texels of dst, w wide, from src: first across, into rows,
// which it returns grown as needed for the next level, then down.
func halve(dst []byte, w int, src []byte, srcStride int, xs, ys []mipTap, rows []float32) []float32 {
	srcHeight := 0
	if n := len(ys); n > 0 {
		srcHeight = ys[n-1].first + ys[n-1].count
	}
	if need := srcHeight * w * 4; cap(rows) < need {
		rows = make([]float32, need)
	} else {
		rows = rows[:need]
	}
	for y := range srcHeight {
		line := src[y*srcStride:]
		out := rows[y*w*4:]
		for x, tx := range xs {
			var sum [4]float32
			for i := range tx.count {
				p := line[(tx.first+i)*4:]
				for c := range sum {
					sum[c] += tx.weight[i] * float32(p[c])
				}
			}
			copy(out[x*4:x*4+4], sum[:])
		}
	}
	for y, ty := range ys {
		out := dst[y*w*4:]
		for x := range w * 4 {
			var sum float32
			for j := range ty.count {
				sum += ty.weight[j] * rows[(ty.first+j)*w*4+x]
			}
			out[x] = byteOf(sum)
		}
	}
	return rows
}

// halveEven is halve for a level of exactly half the size in both directions:
// the mean of each 2×2 box, rounded to the nearest with halves up, which is what
// halve gives there, in integers.
func halveEven(dst []byte, w, h int, src []byte, srcStride int) {
	for y := range h {
		top, bottom := src[2*y*srcStride:], src[(2*y+1)*srcStride:]
		out := dst[y*w*4:]
		for x := range w {
			i := 8 * x
			for c := range 4 {
				sum := int(top[i+c]) + int(top[i+4+c]) + int(bottom[i+c]) + int(bottom[i+4+c])
				out[4*x+c] = uint8((sum + 2) >> 2)
			}
		}
	}
}

// byteOf rounds v, from 0 to 255, to the nearest byte.
func byteOf(v float32) uint8 {
	switch {
	case v >= 255:
		return 255
	case v > 0:
		return uint8(v + 0.5)
	}
	return 0
}
