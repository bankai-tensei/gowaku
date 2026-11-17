package golden

import (
	"image"
	"image/color"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// detailSize is the width and height of detailed.
const detailSize = 256

// detailed returns a picture full of detail finer than a pixel once shrunk: a
// checkerboard of single texels, a colored zone plate, thin diagonal stripes
// and rows of dark bars like lines of small text. Shrinking it shows how each
// filter aliases.
func detailed() *paint.Image {
	const half = detailSize / 2
	m := image.NewNRGBA(image.Rect(0, 0, detailSize, detailSize))
	words := [...]int{5, 3, 7, 2, 4, 6, 3, 5, 2, 8, 4, 3}
	for y := range detailSize {
		for x := range detailSize {
			var c color.NRGBA
			switch {
			case x < half && y < half:
				// Single texels, black and white.
				v := uint8(255 * ((x + y) % 2))
				c = color.NRGBA{v, v, v, 255}
			case y < half:
				// A zone plate: rings that get finer away from the center,
				// in two colors.
				dx, dy := float32(x-half-half/2), float32(y-half/2)
				phase := (dx*dx + dy*dy) * 0.02
				v := 0.5 + 0.5*geom.Cos32(phase)
				c = color.NRGBA{uint8(40 + 200*v), uint8(60 + 120*(1-v)), uint8(200 - 150*v), 255}
			case x < half:
				// Stripes three texels apart, one texel wide.
				c = color.NRGBA{0xf4, 0xf4, 0xf0, 255}
				if (x+y)%3 == 0 {
					c = color.NRGBA{0x2f, 0x5f, 0xd0, 255}
				}
			default:
				// Lines of words: dark bars two texels high, with gaps.
				c = color.NRGBA{0xff, 0xff, 0xff, 255}
				if line := (y - half) % 5; line < 2 {
					at, i := x-half, (y-half)/5
					for at > 0 {
						at -= words[i%len(words)] + 1
						i++
					}
					if at < 0 {
						c = color.NRGBA{0x1a, 0x1a, 0x1a, 255}
					}
				}
			}
			m.SetNRGBA(x, y, c)
		}
	}
	return paint.NewImage(m)
}

// mipmaps draws detailed at a half, a third, a fifth and an eighth of its size,
// and turned at a little more than a quarter, in each filter quality: a row for
// each of nearest, linear and mipmapped.
func mipmaps() core.Widget {
	picture := detailed()
	qualities := [...]struct {
		name    string
		quality paint.FilterQuality
	}{{"Nearest", paint.FilterNearest}, {"Linear", paint.FilterLinear}, {"Mipmap", paint.FilterDefault}}
	rows := make([]core.Widget, 0, len(qualities))
	for _, q := range qualities {
		shrunk := func(divisor float32) core.Widget {
			return basic.Image{Source: picture, Width: detailSize / divisor, Height: detailSize / divisor, Quality: q.quality}
		}
		rows = append(rows, basic.Row{Spacing: 14, Cross: layout.CrossCenter, Children: []core.Widget{
			basic.SizedBox{Width: 64, Child: basic.Text{Content: q.name, Style: text.Style{Size: 13}}},
			shrunk(2), shrunk(3), shrunk(5), shrunk(8),
			basic.SizedBox{Width: 100, Height: 100, Child: basic.Center{
				Child: basic.Rotate(0.5, basic.Image{Source: picture, Width: 70, Height: 70, Quality: q.quality}),
			}},
		}})
	}
	return basic.Padding{Insets: geom.All(12), Child: basic.Column{Spacing: 10, Children: rows}}
}
