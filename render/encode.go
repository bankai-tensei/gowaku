package render

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// Encode replaces the contents of b with the commands of dl for a surface of
// the given scale. It reuses the storage of b, so that it does not allocate once
// b has grown and the glyphs are in the atlas. Consecutive commands under the
// same clip and textures share one draw call.
//
// res can be nil if dl holds neither glyphs, images nor small paths. Otherwise
// call its Finish before handing b to the backend.
func Encode(dl *paint.DisplayList, res *Resources, scale float32, b *gpu.Batch) {
	clearBatch(b)
	if res != nil {
		res.atlas.BeginFrame()
		res.frame++
		res.layerScale = scale
		res.beginGroups()
	}
	res.encodeOps(dl, scale, b)
}

// encodeOps appends the commands of dl to b. A nil receiver can draw what needs
// no resources.
func (g *Resources) encodeOps(dl *paint.DisplayList, scale float32, b *gpu.Batch) {
	g.encodeRange(dl, 0, len(dl.Ops), scale, b)
}

// encodeRange appends the commands of dl from from up to to to b.
func (g *Resources) encodeRange(dl *paint.DisplayList, from, to int, scale float32, b *gpu.Batch) {
	firstDraw := len(b.Draws)
	blended := false
	for i := from; i < to; i++ {
		op := &dl.Ops[i]
		switch op.Kind {
		case paint.OpGroupPush:
			end, groupBlended := g.encodeGroup(dl, i, scale, b)
			blended = blended || groupBlended
			i = end
			continue
		case paint.OpGroupMask, paint.OpGroupPop:
			// Only a list cut short has them out of place.
			continue
		}
		if op.Blend != paint.BlendSrcOver {
			if op.Blend == paint.BlendDst {
				// It keeps what is below, which is drawing nothing.
				continue
			}
			blended = true
			if gpu.BlendMode(op.Blend).ReadsBackdrop() && (op.Kind != paint.OpPath || g != nil && atlasCanDraw(op, scale)) {
				beginBackdrop(b, op)
			}
		}
		switch op.Kind {
		case paint.OpGlyphs:
			g.encodeRun(op, scale, b)
		case paint.OpPath:
			encodePath(op, g, scale, b)
		case paint.OpImage:
			g.encodeImage(op, b)
		case paint.OpClipPush:
			encodeClipPush(op, b)
		case paint.OpClipPop:
			encodeClipPop(op, b)
		case paint.OpLayer:
			g.encodeLayer(op, scale, b)
		case paint.OpBackdrop:
			encodeBackdrop(op, scale, b)
		case paint.OpShader:
			g.encodeShader(op, dl, scale, b)
		default:
			addDraw(b, op, nil, nil)
			// The instance is built in place: it is large enough for the
			// copies to show.
			b.Instances = append(b.Instances, gpu.Instance{})
			in := &b.Instances[len(b.Instances)-1]
			g.fillInstance(in, op)
			if op.Kind == paint.OpRRect && op.Transform == (geom.Affine{}) {
				// A turned box has no pixel grid to snap to.
				snap(in, op, scale)
			}
		}
	}
	if blended {
		finishBlends(b, firstDraw, scale)
	}
}

// addDraw makes room for one more instance in the last draw call, or starts a
// new one if op needs a different clip or texture. Shapes do not read a texture,
// so they ride along with any.
func addDraw(b *gpu.Batch, op *paint.Op, texture, image gpu.Texture) {
	if n := len(b.Draws); n > 0 {
		d := &b.Draws[n-1]
		if d.Kind == gpu.DrawQuads && d.Blur == 0 && d.Shader == nil && d.Blend == gpu.BlendMode(op.Blend) && sameClip(d, op) &&
			fits(d.Texture, texture) && fits(d.Image, image) && (op.Blend == paint.BlendSrcOver || backdropFits(d, op)) {
			d.Count++
			if texture != nil {
				d.Texture = texture
			}
			if image != nil {
				d.Image = image
			}
			return
		}
	}
	b.Draws = append(b.Draws, gpu.DrawCall{
		First:     len(b.Instances),
		Count:     1,
		Texture:   texture,
		Image:     image,
		Clip:      op.Clip,
		Clipped:   op.Clipped,
		ClipLevel: op.ClipLevel,
		Blend:     gpu.BlendMode(op.Blend),
	})
	if op.Blend != paint.BlendSrcOver {
		b.Draws[len(b.Draws)-1].Bounds = opReach(op)
	}
}

func fits(have, want gpu.Texture) bool { return want == nil || have == nil || have == want }

func sameClip(d *gpu.DrawCall, op *paint.Op) bool {
	return d.ClipLevel == op.ClipLevel && d.Clipped == op.Clipped && (!d.Clipped || d.Clip == op.Clip)
}

// instanceOf returns the instance of a shape. A nil receiver draws gradients of
// many stops with four of them.
func (g *Resources) instanceOf(op *paint.Op) gpu.Instance {
	var in gpu.Instance
	g.fillInstance(&in, op)
	return in
}

// fillInstance sets the fields of in, which must be zero, for op.
func (g *Resources) fillInstance(in *gpu.Instance, op *paint.Op) {
	in.Rect = op.RRect.Rect
	in.Radii = [4]float32{op.RRect.TopLeft, op.RRect.TopRight, op.RRect.BottomRight, op.RRect.BottomLeft}
	in.Color, in.BorderColor = op.Color, op.BorderColor
	in.BorderWidth, in.Blur = op.BorderWidth, op.Blur
	if op.Transform != (geom.Affine{}) {
		in.SetTransform(op.Transform)
	}
	if op.Kind == paint.OpShadow {
		in.Kind = gpu.InstanceShadow
	}
	if op.HasGradient {
		g.setGradient(in, &op.Gradient)
	}
}

// gradientModes maps the kinds of paint to the modes of the instance.
var gradientModes = [...]gpu.GradientMode{
	paint.GradientLinear: gpu.GradientLinear,
	paint.GradientRadial: gpu.GradientRadial,
	paint.GradientSweep:  gpu.GradientSweep,
	paint.GradientFocal:  gpu.GradientFocal,
}

// spreadModes maps the spreads of paint to the bits of the instance.
var spreadModes = [...]gpu.GradientMode{
	paint.SpreadPad:     0,
	paint.SpreadReflect: gpu.GradientReflect,
	paint.SpreadRepeat:  gpu.GradientRepeat,
}

// maxFocus is how far into its circle the focal point of a gradient may lie,
// as a share of the radius: on the circle the offsets would run to infinity.
const maxFocus = 0.99

// setGradient hands gr to in: its shape, its space and its stops.
func (g *Resources) setGradient(in *gpu.Instance, gr *paint.Gradient) {
	in.Gradient = gpu.GradientLinear
	if int(gr.Kind) < len(gradientModes) {
		in.Gradient = gradientModes[gr.Kind]
	}
	in.GradientStart, in.GradientEnd = gr.Start, gr.End
	if m := gr.Transform; !m.IsIdentity() {
		// The instance measures from the start on the shape, through the
		// inverse of the linear part, along the axis in the space of the
		// gradient. A flat transform has no inverse, and is left out.
		if inv, ok := m.Invert(); ok {
			start := m.Apply(gr.Start)
			in.GradientStart = start
			in.GradientEnd = start.Add(gr.End.Sub(gr.Start))
			in.GradientMatrix = [4]float32{inv.A, inv.B, inv.C, inv.D}
		}
	}
	if int(gr.Spread) < len(spreadModes) {
		in.Gradient |= spreadModes[gr.Spread]
	}
	var stops []paint.GradientStop
	if gr.MoreStops != nil {
		stops = *gr.MoreStops
	}
	if gr.Kind == paint.GradientFocal {
		g.setFocal(in, gr, stops)
		return
	}
	switch {
	case len(stops) == 0:
		setStops(in, gr)
		return
	case len(stops) <= gpu.MaxStops:
		setStopList(in, stops)
		return
	}
	if g != nil {
		if g.ramps == nil {
			g.ramps = newRampTable()
		}
		if row, ok := g.ramps.row(stops, g.frame); ok {
			in.Gradient |= gpu.GradientRamp
			in.StopOffsets = [4]float32{float32(row)}
			return
		}
	}
	setFewerStops(in, stops)
}

// setFocal hands in the stops of a focal gradient, which always come from a
// row of the ramps, and its focal point. Without a row it falls back to a
// radial gradient around its center.
func (g *Resources) setFocal(in *gpu.Instance, gr *paint.Gradient, stops []paint.GradientStop) {
	var few [gpu.MaxStops]paint.GradientStop
	switch {
	case len(stops) > 0:
	case gr.StopCount > 0:
		stops = gr.Stops[:gr.StopCount]
	default:
		few[0] = paint.GradientStop{Color: gr.StartColor}
		few[1] = paint.GradientStop{Offset: 1, Color: gr.EndColor}
		stops = few[:2]
	}
	if g != nil {
		if g.ramps == nil {
			g.ramps = newRampTable()
		}
		if row, ok := g.ramps.row(stops, g.frame); ok {
			f := gr.Focus.Sub(gr.Start)
			r := gr.End.Sub(gr.Start)
			radius := geom.Sqrt32(r.X*r.X + r.Y*r.Y)
			if d := geom.Sqrt32(f.X*f.X + f.Y*f.Y); d > maxFocus*radius {
				f = f.Scale(maxFocus * radius / d)
			}
			in.Gradient = gpu.GradientFocal | gpu.GradientRamp | in.Gradient.Spread()
			in.StopOffsets = [4]float32{float32(row), f.X, f.Y}
			return
		}
	}
	in.Gradient = gpu.GradientRadial | in.Gradient.Spread()
	if len(stops) <= gpu.MaxStops {
		setStopList(in, stops)
		return
	}
	setFewerStops(in, stops)
}

// A Gradient must fit the stops of an instance; this fails to compile if the
// two maxima ever differ.
const _ = uint(gpu.MaxStops-paint.MaxGradientStops) + uint(paint.MaxGradientStops-gpu.MaxStops)

// setStops hands the stops of g to in. The zero StopCount is the two colors
// of StartColor and EndColor at 0 and 1.
func setStops(in *gpu.Instance, g *paint.Gradient) {
	if g.StopCount > 0 {
		setStopList(in, g.Stops[:g.StopCount])
		return
	}
	colors := [gpu.MaxStops]geom.Color{g.StartColor, g.EndColor}
	offsets := [gpu.MaxStops]float32{0, 1}
	in.SetStops(&colors, &offsets, 2)
}

// setStopList hands in from 1 to gpu.MaxStops stops.
func setStopList(in *gpu.Instance, stops []paint.GradientStop) {
	var colors [gpu.MaxStops]geom.Color
	var offsets [gpu.MaxStops]float32
	for i, s := range stops {
		colors[i], offsets[i] = s.Color, s.Offset
	}
	in.SetStops(&colors, &offsets, len(stops))
}

// setFewerStops hands in the first and last of stops and two evenly spaced
// between them. It is what a gradient of many stops falls back to without a
// row of the ramps: without Resources, or past rampRows of them in one frame.
func setFewerStops(in *gpu.Instance, stops []paint.GradientStop) {
	var colors [gpu.MaxStops]geom.Color
	var offsets [gpu.MaxStops]float32
	last := len(stops) - 1
	for i := range gpu.MaxStops {
		s := stops[(i*last+1)/(gpu.MaxStops-1)]
		colors[i], offsets[i] = s.Color, s.Offset
	}
	in.SetStops(&colors, &offsets, gpu.MaxStops)
}
