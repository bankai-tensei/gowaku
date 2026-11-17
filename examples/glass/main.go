// Command glass shows a glass dialog, Dialog{Glass: true}, over a colorful list
// that keeps scrolling behind it. See docs/ADR/0044-drawing-extensions.md.
//
//	glass                         opens the window; the button opens the dialog
//	glass -dark                   the same in the dark theme
//	glass -png out.png            draws one frame with the dialog open and exits
//	glass -bench 10s              measures frames with the dialog open, without
//	                              waiting for vertical blank, and prints the
//	                              CPU and GPU time of a frame
//	glass -bench 10s -vulkan -gpu high
//	                              the same with Vulkan on the fast GPU
//
// The measuring window is 1280 by 720 logical pixels, 1920 by 1080 physical at a
// scale of 150%. The dialog is as wide as a dialog gets, 560 logical pixels.
package main

import (
	"flag"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	rowHeight = 64
	rowCount  = 24
	// speed is how fast the list scrolls, in logical pixels per second.
	speed = 90
)

// palette holds the colors of the rows, two per row for a gradient.
var palette = [][2]uint32{
	{0xff6b6b, 0xffa94d}, {0xffd43b, 0x69db7c}, {0x38d9a9, 0x4dabf7}, {0x748ffc, 0xda77f2},
	{0xf783ac, 0xff8787}, {0x63e6be, 0x74c0fc}, {0xffc078, 0xffe066}, {0x9775fa, 0x5c7cfa},
}

func row(i int) core.Widget {
	c := palette[i%len(palette)]
	return basic.RepaintBoundary{Child: basic.Padding{Insets: geom.Symmetric(16, 6), Child: basic.Container{
		Height: rowHeight - 12, Radius: 14, Padding: geom.Symmetric(16, 0),
		Gradient: &paint.Gradient{
			End:        geom.Offset{X: 1200, Y: 40},
			StartColor: geom.Hex(c[0]), EndColor: geom.Hex(c[1]),
		},
		Child: basic.Row{Cross: layout.CrossCenter, Spacing: 12, Children: []core.Widget{
			basic.Container{Width: 28, Height: 28, Radius: 14, Color: geom.Hex(0xffffff).WithAlpha(0.8)},
			basic.Expanded{Child: basic.Text{
				Content: fmt.Sprintf("%d 行目 Row number %d of a list that keeps moving", i%rowCount+1, i%rowCount+1),
				Style:   text.Style{Size: 15, Color: geom.Hex(0x14161a)},
			}},
		}},
	}}}
}

// drift is the list behind the dialog. It scrolls by itself, without a pointer,
// so that every frame has a new backdrop to blur.
type drift struct {
	core.StatefulBase
}

type driftState struct {
	core.StateBase
	ticker *core.Ticker
	now    time.Duration
	rows   []core.Widget
}

func (drift) CreateState() core.State { return &driftState{} }

func (s *driftState) InitState() {
	// Two rounds of rows, so that the first comes back where it started.
	for i := range 2 * rowCount {
		s.rows = append(s.rows, row(i))
	}
	s.ticker = s.NewTicker(func(now time.Duration) {
		s.SetState(func() { s.now = now })
	})
	s.ticker.Start()
}

func (s *driftState) Dispose() { s.ticker.Stop() }

func (s *driftState) Build(core.BuildContext) core.Widget {
	cycle := float32(rowCount * rowHeight)
	y := float32(s.now.Seconds()) * speed
	y -= geom.Floor32(y/cycle) * cycle
	return conveyor{y: y, child: basic.Column{Size: layout.MainMin, Children: s.rows}}
}

// conveyor shows its child at its natural height, moved up by y, as a list that
// scrolls by itself.
type conveyor struct {
	y     float32
	child core.Widget
}

func (conveyor) Key() core.Key { return nil }

func (c conveyor) CreateRenderObject() core.RenderObject { return &renderConveyor{y: c.y} }

func (c conveyor) UpdateRenderObject(ro core.RenderObject) {
	if r := ro.(*renderConveyor); r.y != c.y {
		r.y = c.y
		r.MarkNeedsPaint()
	}
}

func (c conveyor) ChildWidget() core.Widget { return c.child }

type renderConveyor struct {
	layout.RenderProxy
	y float32
}

func (r *renderConveyor) Layout(c geom.Constraints) geom.Size {
	if r.ChildCount() > 0 {
		core.LayoutChild(r.ChildAt(0), geom.Constraints{MinW: c.MaxW, MaxW: c.MaxW, MaxH: geom.Inf})
	}
	return c.Biggest()
}

func (r *renderConveyor) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	r.RenderProxy.Paint(cv, off.Add(geom.Offset{Y: -r.y}))
	cv.Restore()
}

// page is the list with a bar on top that opens the dialog.
type page struct {
	core.StatelessBase
	// ctx receives the context below the Overlay, for the measurements.
	ctx *core.BuildContext
	// opaque opens the dialog without glass, to compare.
	opaque bool
}

func (p page) Build(ctx core.BuildContext) core.Widget {
	if p.ctx != nil {
		*p.ctx = ctx
	}
	return basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
		drift{},
		basic.Positioned{Right: 24, Bottom: 24, AnchorRight: true, AnchorBottom: true, Child: control.Button{
			Label: "Glass dialog", Style: control.ButtonPrimary, OnPress: func() { showDialog(ctx, !p.opaque) },
		}},
	}}
}

func showDialog(ctx core.BuildContext, glass bool) {
	overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
		return overlay.Dialog{
			Title: "Glass dialog",
			Message: "The list keeps scrolling behind this card. It shows through, blurred, " +
				"while the text stays easy to read. 背景はぼかして透かし、文字は読みやすいままにする。",
			Actions: []core.Widget{
				control.Button{Label: "Cancel", OnPress: dismiss},
				control.Button{Label: "OK", Style: control.ButtonPrimary, OnPress: dismiss},
			},
			Width: 560,
			Glass: glass,
		}
	})
}

// percentile returns the value below which a share q of sorted lies.
func percentile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[min(int(q*float64(len(sorted))), len(sorted)-1)]
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// open starts a harness with the dialog open and settled.
func open(opts shot.Options, opaque bool) (*shot.Harness, error) {
	var ctx core.BuildContext
	h, err := shot.Open(page{ctx: &ctx, opaque: opaque}, opts)
	if err != nil {
		return nil, err
	}
	if _, err := h.Frame(); err != nil {
		h.Close()
		return nil, err
	}
	showDialog(ctx, !opaque)
	// The dialog takes 280 ms to come in.
	for start := time.Now(); time.Since(start) < 600*time.Millisecond; {
		if _, err := h.Frame(); err != nil {
			h.Close()
			return nil, err
		}
		h.Pump()
	}
	return h, nil
}

// bench draws frames for length and prints what a frame cost.
func bench(opts shot.Options, length time.Duration, opaque bool) error {
	opts.Immediate = true
	h, err := open(opts, opaque)
	if err != nil {
		return err
	}
	defer h.Close()
	if err := h.EnableGPUTiming(); err != nil {
		return err
	}
	width, height := h.PhysicalSize()
	backend := "d3d11"
	if opts.Vulkan {
		backend = "vulkan"
	}
	dialog := "glass"
	if opaque {
		dialog = "opaque"
	}
	fmt.Printf("%s on %s, %dx%d physical, %s dialog\n", backend, h.AdapterName, width, height, dialog)
	var cpu, encode, wait, wall, gpuTimes []time.Duration
	start := time.Now()
	last := shot.Now()
	for time.Since(start) < length {
		ft, err := h.TimedFrame()
		if err != nil {
			return err
		}
		h.Pump()
		now := shot.Now()
		// BeginFrame and EndFrame wait for the GPU and the swap chain; the
		// work of the CPU is the rest.
		cpu = append(cpu, ft.Encode+ft.Draw)
		encode = append(encode, ft.Encode)
		wait = append(wait, ft.Begin+ft.Present)
		wall = append(wall, now-last)
		last = now
		gpuTimes = h.GPUFrameTimes(gpuTimes)
	}
	total := time.Since(start)
	// The last frames finish on the GPU after the loop.
	for range 3 {
		if _, err := h.Frame(); err != nil {
			return err
		}
	}
	gpuTimes = h.GPUFrameTimes(gpuTimes)
	for _, s := range [][]time.Duration{cpu, encode, wait, wall, gpuTimes} {
		slices.Sort(s)
	}
	line := func(name string, s []time.Duration) {
		fmt.Printf("%-28s p50 %6.3f ms  p95 %6.3f ms  max %6.3f ms\n", name, ms(percentile(s, 0.5)), ms(percentile(s, 0.95)), ms(s[len(s)-1]))
	}
	fmt.Printf("%d frames in %.1fs, %.0f fps without vsync\n", len(cpu), total.Seconds(), float64(len(cpu))/total.Seconds())
	line("cpu (encode + draw)", cpu)
	line("  encode (tree to batch)", encode)
	line("  waits (begin + present)", wait)
	line(fmt.Sprintf("gpu (%d frames)", len(gpuTimes)), gpuTimes)
	line("frame interval", wall)
	return nil
}

func screenshot(opts shot.Options, path string, opaque bool) error {
	h, err := open(opts, opaque)
	if err != nil {
		return err
	}
	defer h.Close()
	return h.Screenshot(path)
}

func main() {
	pngPath := flag.String("png", "", "write one frame with the dialog open to this file and exit")
	length := flag.Duration("bench", 0, "measure frames with the dialog open for this long")
	dark := flag.Bool("dark", false, "use the dark theme")
	vk := flag.Bool("vulkan", false, "use Vulkan instead of D3D11 (Windows)")
	power := flag.String("gpu", "low", "low or high: the GPU of a hybrid machine")
	width := flag.Float64("w", 1280, "width of the window in logical pixels")
	height := flag.Float64("h", 720, "height of the window in logical pixels")
	glass := flag.Bool("glass", true, "open the glass dialog; false opens the opaque one, to compare")
	flag.Parse()

	brightness := theme.BrightnessLight
	mode := gowaku.ThemeLight
	if *dark {
		brightness, mode = theme.BrightnessDark, gowaku.ThemeDark
	}
	opts := shot.Options{
		Width: float32(*width), Height: float32(*height),
		Theme:  theme.Set{}.For(brightness),
		Vulkan: *vk,
	}
	if *power == "high" {
		opts.Power = gpu.HighPerformance
	}
	var err error
	switch {
	case *length > 0:
		err = bench(opts, *length, !*glass)
	case *pngPath != "":
		err = screenshot(opts, *pngPath, !*glass)
	default:
		err = gowaku.Run(gowaku.App{Title: "gowaku: glass", Width: 900, Height: 640, ThemeMode: mode, Home: page{opaque: !*glass}})
	}
	if err != nil {
		log.Fatal(err)
	}
}
