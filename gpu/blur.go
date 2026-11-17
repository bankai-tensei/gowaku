package gpu

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
)

// The thresholds of BlurFactor, in physical pixels of standard deviation.
const (
	// blurHalf is where a blur stops shrinking the picture by 2 and starts
	// shrinking it by 4.
	blurHalf = 8
	// blurQuarter is where it starts shrinking by 8.
	blurQuarter = 32
	// MaxBlurSigma bounds the standard deviation of a blur, in physical
	// pixels. Shrunk by 8, it reads MaxBlurRadius texels on each side.
	MaxBlurSigma = 128
	// MaxBlurRadius is the most texels on each side that a pass reads.
	MaxBlurRadius = 48
)

// BlurFactor returns how much a blur of standard deviation sigma, in physical
// pixels, shrinks the picture before it blurs: 2 below 8, 4 below 32 and 8 from
// there. The shrunk picture is blurred with a standard deviation from 1 to 16
// texels, never fewer than one so that the upscale stays smooth. The pixel
// shader of InstanceBackdrop and InstanceFiltered works out the same factor.
func BlurFactor(sigma float32) int {
	switch {
	case sigma < blurHalf:
		return 2
	case sigma < blurQuarter:
		return 4
	}
	return 8
}

// BlurRadius returns how many texels on each side a pass with the standard
// deviation sigma, in shrunk texels, reads: three standard deviations, at most
// MaxBlurRadius. The pixel shader works out the same radius.
func BlurRadius(sigma float32) int {
	return min(int(geom.Ceil32(3*sigma)), MaxBlurRadius)
}

// Blur is how a backend runs the blur of a draw call: copy Source, if it comes
// from the target, shrink it by Factor into a scratch target, blur that across
// into another and down back into the first, which the instance of the call then
// reads. See docs/ADR/0044-drawing-extensions.md.
type Blur struct {
	// Factor is what the picture shrinks by, and Sigma the standard deviation
	// of the blur in shrunk texels.
	Factor int
	Sigma  float32
	// Source is what the blur reads, in physical pixels of the target, or in
	// texels of the image of an InstanceFiltered. Reads past its edge repeat
	// the edge.
	Source image.Rectangle
	// Out is what each pass writes, in shrunk texels: Source shrunk, and one
	// texel more on every side for the bilinear filter of the instance. The
	// scratch targets must reach Out.Max.
	Out image.Rectangle
}

// PlanBlur returns the blur of c, which has one instance, in. width and height
// are the size of the target, or of the image for an InstanceFiltered. It
// reports false if the call draws nothing.
//
// A backdrop blur reads Bounds grown by three standard deviations, inside the
// clip and the target: what lies outside the clip, such as the header above a
// scrolled list, does not bleed in.
func PlanBlur(c *DrawCall, in *Instance, scale float32, width, height int) (Blur, bool) {
	sigma := min(c.Blur*scale, MaxBlurSigma)
	if !(sigma > 0) || c.Count != 1 {
		return Blur{}, false
	}
	b := Blur{Factor: BlurFactor(sigma)}
	b.Sigma = sigma / float32(b.Factor)
	bounds := image.Rect(0, 0, width, height)
	switch in.Kind.Base() {
	case InstanceBackdrop:
		r := c.Bounds
		if r.Size().Empty() {
			return Blur{}, false
		}
		grow := 3 * sigma / scale
		r = geom.Rect{X: r.X - grow, Y: r.Y - grow, W: r.W + 2*grow, H: r.H + 2*grow}
		if c.Clipped {
			r = r.Intersect(c.Clip)
		}
		if r.Size().Empty() {
			return Blur{}, false
		}
		left := int(geom.Floor32(r.X * scale))
		top := int(geom.Floor32(r.Y * scale))
		right := int(geom.Floor32((r.X+r.W)*scale)) + 1
		bottom := int(geom.Floor32((r.Y+r.H)*scale)) + 1
		b.Source = image.Rect(left, top, right, bottom).Intersect(bounds)
	case InstanceFiltered:
		texel := func(v float32, size int) int { return int(geom.Floor32(v*float32(size) + 0.5)) }
		b.Source = image.Rect(
			texel(in.GradientStart.X, width), texel(in.GradientStart.Y, height),
			texel(in.GradientEnd.X, width), texel(in.GradientEnd.Y, height),
		).Intersect(bounds)
	default:
		return Blur{}, false
	}
	if b.Source.Empty() {
		return Blur{}, false
	}
	f := b.Factor
	b.Out = image.Rect(
		max(b.Source.Min.X/f-1, 0), max(b.Source.Min.Y/f-1, 0),
		(b.Source.Max.X+f-1)/f+1, (b.Source.Max.Y+f-1)/f+1,
	)
	return b, true
}

// Passes writes the three passes of b, to be drawn in turn with the frame
// constants of the target: shrinking Source into a scratch target, blurring that
// across into a second one, and down back into the first. Each covers Out, and
// writes what it computes without blending (BlendSrc). The scratch targets are
// smaller than the target, but the viewport stays that of the target, so that a
// physical pixel of the target is a texel of the scratch target. fromImage makes
// the first pass read the image of the call instead of the backdrop.
func (b *Blur) Passes(out *[3]Instance, scale float32, fromImage bool) {
	r := geom.Rect{
		X: float32(b.Out.Min.X) / scale,
		Y: float32(b.Out.Min.Y) / scale,
		W: float32(b.Out.Dx()) / scale,
		H: float32(b.Out.Dy()) / scale,
	}
	source := [4]float32{
		float32(b.Source.Min.X), float32(b.Source.Min.Y),
		float32(b.Source.Max.X - 1), float32(b.Source.Max.Y - 1),
	}
	shrunk := [4]float32{
		float32(b.Out.Min.X), float32(b.Out.Min.Y),
		float32(b.Out.Max.X - 1), float32(b.Out.Max.Y - 1),
	}
	var from float32
	if fromImage {
		from = 1
	}
	out[0] = Instance{Rect: r, Radii: source, BorderWidth: float32(b.Factor), Blur: from, Kind: InstanceBlurDown}
	out[1] = Instance{Rect: r, Radii: shrunk, BorderWidth: b.Sigma, Kind: InstanceBlurX}
	out[2] = Instance{Rect: r, Radii: shrunk, BorderWidth: b.Sigma, Kind: InstanceBlurY}
}

// CountBlurs returns how many draw calls of b blur, for which a backend needs
// three more instances each.
func (b *Batch) CountBlurs() int {
	n := 0
	for i := range b.Draws {
		if b.Draws[i].Blur > 0 && b.Draws[i].Count > 0 {
			n++
		}
	}
	return n
}
