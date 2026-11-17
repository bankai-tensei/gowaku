package shaders

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// SceneWidth and SceneHeight are the size of Scene in logical pixels.
const (
	SceneWidth  = 880
	SceneHeight = 520
)

// auroraParams run from a deep blue to a violet, with rings of about 31
// logical pixels that move at 2.4 radians a second.
func auroraParams() paint.ShaderParams {
	return paint.ShaderParams{
		0.13, 0.30, 0.72, 1,
		0.62, 0.22, 0.66, 1,
		0.2, 2.4, 0.35, 0,
	}
}

// shimmerParams pass once every two seconds, in a slanted band of a fifth of
// the width, white at 85%.
func shimmerParams() paint.ShaderParams {
	return paint.ShaderParams{
		0.5, 0.2, 0.35, 0,
		1, 1, 1, 0.85,
	}
}

// rippleParams are rings from the middle, about 21 logical pixels apart, that
// move a pixel up to push logical pixels.
func rippleParams(push float32) paint.ShaderParams {
	return paint.ShaderParams{
		0.5, 0.5, push, 0.3,
		3, 0, 0, 0,
	}
}

// Picture returns the image that the ripple bends: a grid of colored squares
// with diagonal stripes, which shows every bend, 240 by 160 pixels.
func Picture() *paint.Image {
	const w, h, cell = 240, 160, 20
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			cx, cy := x/cell, y/cell
			hue := float64(cx*3+cy*5) * 0.45
			r := 0.5 + 0.45*math.Cos(hue)
			g := 0.5 + 0.45*math.Cos(hue-2.1)
			b := 0.5 + 0.45*math.Cos(hue+2.1)
			if (x+y)/6%2 == 0 {
				r, g, b = r*0.8, g*0.8, b*0.8
			}
			if x%cell == 0 || y%cell == 0 {
				r, g, b = 0.12, 0.13, 0.16
			}
			m.Set(x, y, color.RGBA{R: uint8(255 * r), G: uint8(255 * g), B: uint8(255 * b), A: 255})
		}
	}
	return paint.NewImage(m)
}

// Scene is the picture of examples/shader at time t: an aurora that
// CustomPaint draws, a turned copy of it, a shimmer over text and a ripple
// over a picture and over text, both by ShaderMask. With animate set, the
// shaders run on from t.
func Scene(t time.Duration, animate bool, picture *paint.Image) core.Widget {
	white := geom.Color{R: 1, G: 1, B: 1, A: 1}
	title := basic.Padding{Insets: geom.All(24), Child: basic.Column{Spacing: 6, Children: []core.Widget{
		basic.Text{Content: "Custom shaders", Style: text.Style{Size: 34, Weight: 700, Color: white}},
		basic.Text{Content: "One HLSL source, compiled for Direct3D 11, Vulkan and Metal by gowaku-shader.",
			Style: text.Style{Size: 15, Color: white}},
	}}}
	aurora := basic.Stack{Children: []core.Widget{
		control.CustomPaint{Shader: Aurora, Params: auroraParams(), Width: 832, Height: 170, Time: t, Animate: animate},
		title,
	}}
	shimmer := basic.ShaderMask{Shader: Shimmer, Params: shimmerParams(), Time: t, Animate: animate,
		Child: basic.Text{Content: "gowaku", Style: text.Style{Size: 72, Weight: 800, Color: geom.Hex(0x2f5fd0)}}}
	ripple := basic.ShaderMask{Shader: Ripple, Params: rippleParams(5), Time: t, Animate: animate,
		Child: basic.Image{Source: picture, Width: 240, Height: 160}}
	rippleText := basic.ShaderMask{Shader: Ripple, Params: rippleParams(1.5), Time: t, Animate: animate,
		Child: basic.Container{Color: geom.Hex(0xf2efe6), Radius: 10, Padding: geom.All(12), Child: basic.Text{
			Content: "Ripples bend\nwhat the mask\ndraws: text too.", Style: text.Style{Size: 18, Weight: 600, Color: geom.Hex(0x1d2430)}}}}
	turned := basic.Transform{Transform: geom.Rotation(-0.25), Origin: layout.AlignCenter, Child: control.CustomPaint{
		Shader: Aurora, Params: auroraParams(), Width: 150, Height: 96, Time: t, Animate: animate}}
	return basic.Padding{Insets: geom.All(24), Child: basic.Column{Spacing: 24, Children: []core.Widget{
		aurora,
		basic.Row{Spacing: 28, Children: []core.Widget{
			basic.Column{Spacing: 20, Children: []core.Widget{shimmer, rippleText}},
			ripple,
			basic.Padding{Insets: geom.Insets{Top: 30, Left: 10}, Child: turned},
		}},
	}}}
}
