package golden

import (
	"bytes"
	_ "embed" // the animation fixtures
	"image"
	"image/color"
	"image/gif"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// The animations that libwebp and ffmpeg made, copied from paint/testdata/anim
// (see the README there).
var (
	//go:embed testdata/anim/movie.webp
	movieWebP []byte
	//go:embed testdata/anim/movie.png
	movieAPNG []byte
)

// midFrame is the frame of each animation that the scene shows. Its picture
// depends on the disposals and blends of the frames before it.
const midFrame = 3

// disposalGIF returns a GIF of four frames on a 12x12 canvas: a dark frame,
// a square that disposes to the previous picture, one that disposes to the
// background, and a last small frame with a transparent pixel.
func disposalGIF() []byte {
	pal := color.Palette{color.RGBA{}, color.RGBA{0x20, 0x30, 0x40, 255}, color.RGBA{0xe8, 0x50, 0x30, 255}, color.RGBA{0x30, 0xb0, 0x60, 255}}
	frame := func(r image.Rectangle, index uint8) *image.Paletted {
		m := image.NewPaletted(r, pal)
		for i := range m.Pix {
			m.Pix[i] = index
		}
		return m
	}
	last := frame(image.Rect(4, 4, 8, 8), 3)
	last.SetColorIndex(5, 5, 0)
	g := &gif.GIF{
		Image: []*image.Paletted{
			frame(image.Rect(0, 0, 12, 12), 1),
			frame(image.Rect(0, 0, 6, 6), 2),
			frame(image.Rect(6, 6, 12, 12), 2),
			last,
		},
		Delay:    []int{10, 10, 10, 10},
		Disposal: []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalBackground, gif.DisposalNone},
		Config:   image.Config{ColorModel: pal, Width: 12, Height: 12},
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// fromFrame returns an animation that starts at frame i of the file data.
func fromFrame(data []byte, i int) *paint.Animation {
	a, err := paint.DecodeAnimation(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return &paint.Animation{Frames: a.Frames[i:], Delays: a.Delays[i:], Loops: 1}
}

// animated draws frame midFrame of a GIF, an APNG and an animated WebP file
// through AnimatedImage, enlarged without smoothing, and the WebP one shrunk.
func animated() core.Widget {
	files := [...]struct {
		name string
		anim *paint.Animation
	}{
		{"GIF", fromFrame(disposalGIF(), midFrame)},
		{"APNG", fromFrame(movieAPNG, midFrame)},
		{"WebP", fromFrame(movieWebP, midFrame)},
	}
	cells := make([]core.Widget, 0, len(files)+1)
	for _, f := range files {
		w, _ := f.anim.Size()
		cells = append(cells, basic.Column{Spacing: 6, Children: []core.Widget{
			basic.Text{Content: f.name, Style: text.Style{Size: 13}},
			basic.Container{Color: geom.Hex(0xd8dce4), Child: basic.AnimatedImage{
				Source: f.anim, Width: float32(w) * 4, Quality: paint.FilterNearest,
			}},
		}})
	}
	cells = append(cells, basic.Container{Color: geom.Hex(0xd8dce4), Child: basic.AnimatedImage{
		Source: files[2].anim, Width: 20, Fit: layout.FitContain,
	}})
	return basic.Padding{Insets: geom.All(12), Child: basic.Row{Spacing: 16, Cross: layout.CrossEnd, Children: cells}}
}
