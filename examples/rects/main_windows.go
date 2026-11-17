// Command rects is a sample of the SDF pipeline.
//
//	rects                              a gallery of decorations
//	rects -page elevation              the five MD3 elevation levels
//	rects -page stress -count 10000    moving rectangles, printing frame rates
//
// Add -immediate to the stress page to draw without waiting for vertical blank.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"math"
	"os"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/d3d11"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32"
	"github.com/bankai-tensei/gowaku/render"
)

// surfaceColor is the MD3 baseline light surface.
var surfaceColor = geom.Hex(0xfef7ff)

type options struct {
	page          string
	count         int
	rectWidth     float32
	rectHeight    float32
	shadows       bool
	width, height float64
	maxFrames     int
	pngPath       string
}

type app struct {
	options
	platform *win32.Platform
	device   *d3d11.Device
	window   platform.Window
	surface  gpu.Surface
	canvas   paint.Canvas
	batch    gpu.Batch
	start    time.Time

	open      bool
	dirty     bool
	minimized bool
	frames    int

	statsSince  time.Time
	statsFrames int
	cpuTime     time.Duration
}

func main() {
	var o options
	var width, height float64
	power := flag.String("gpu", "low", "GPU preference: low or high")
	immediate := flag.Bool("immediate", false, "do not wait for vertical blank")
	flag.StringVar(&o.page, "page", "gallery", "gallery, elevation or stress")
	flag.IntVar(&o.count, "count", 10000, "rectangles on the stress page")
	rectWidth := flag.Float64("rect-width", 0, "rectangle width on the stress page; 0 tiles the window")
	rectHeight := flag.Float64("rect-height", 0, "rectangle height on the stress page; 0 tiles the window")
	samples := flag.Uint("samples", 0, "MSAA level; 0 selects the default, 1 turns it off")
	flag.BoolVar(&o.shadows, "shadows", false, "give every stress rectangle a shadow")
	flag.Float64Var(&width, "width", 960, "client width in logical pixels")
	flag.Float64Var(&height, "height", 640, "client height in logical pixels")
	flag.IntVar(&o.maxFrames, "frames", 0, "quit after this many frames")
	flag.StringVar(&o.pngPath, "png", "", "save the last frame to this PNG file")
	flag.Parse()
	o.width, o.height = width, height
	o.rectWidth, o.rectHeight = float32(*rectWidth), float32(*rectHeight)

	opts := d3d11.Options{PresentImmediately: *immediate, Samples: uint32(*samples)}
	if *power == "high" {
		opts.Power = gpu.HighPerformance
	}
	if err := run(o, opts); err != nil {
		log.Fatal(err)
	}
}

func run(o options, opts d3d11.Options) error {
	p, err := win32.New()
	if err != nil {
		return err
	}
	defer p.Destroy()
	device, err := d3d11.New(opts)
	if err != nil {
		return err
	}
	defer device.Destroy()
	log.Printf("adapter: %s", device.AdapterName())

	w, err := p.CreateWindow(platform.WindowOptions{Title: "gowaku: rects", Width: float32(o.width), Height: float32(o.height)})
	if err != nil {
		return err
	}
	width, height := w.PhysicalSize()
	s, err := device.CreateSurface(w.NativeHandle(), width, height, w.Scale())
	if err != nil {
		return err
	}
	a := &app{
		options: o, platform: p, device: device, window: w, surface: s,
		start: time.Now(), statsSince: time.Now(), open: true, dirty: true,
	}
	p.SetModalHandler(a.pump)
	for a.open {
		timeout := time.Duration(-1)
		if a.animated() && !a.minimized {
			timeout = 0
		}
		a.handle(p.WaitEvent(timeout))
		a.pump()
	}
	return nil
}

func (a *app) animated() bool { return a.page == "stress" || a.maxFrames > 0 }

func (a *app) pump() {
	for {
		ev := a.platform.WaitEvent(0)
		if ev.Kind == platform.EventNone {
			break
		}
		a.handle(ev)
	}
	if a.open && (a.dirty || a.animated()) && !a.minimized {
		a.draw()
	}
}

func (a *app) handle(ev platform.Event) {
	switch ev.Kind {
	case platform.EventRedraw:
		a.dirty = true
	case platform.EventResize:
		a.minimized = ev.PhysicalWidth == 0 || ev.PhysicalHeight == 0
		if err := a.surface.Resize(ev.PhysicalWidth, ev.PhysicalHeight, ev.Scale); err != nil {
			log.Fatal(err)
		}
		a.dirty = true
	case platform.EventClose:
		a.close()
	}
}

func (a *app) close() {
	a.surface.Destroy()
	a.window.Close()
	a.open = false
}

func (a *app) logicalSize() geom.Size {
	w, h := a.window.PhysicalSize()
	return geom.Size{W: float32(w) / a.window.Scale(), H: float32(h) / a.window.Scale()}
}

func (a *app) draw() {
	began := time.Now()
	a.canvas.Reset()
	switch a.page {
	case "elevation":
		drawElevation(&a.canvas)
	case "stress":
		a.drawStress()
	default:
		drawGallery(&a.canvas)
	}
	render.Encode(a.canvas.DisplayList(), nil, 1, &a.batch)

	if err := a.device.BeginFrame(a.surface, surfaceColor); err != nil {
		log.Fatal(err)
	}
	if err := a.device.Draw(a.surface, &a.batch); err != nil {
		log.Fatal(err)
	}
	a.cpuTime += time.Since(began)
	a.frames++
	last := a.maxFrames > 0 && a.frames >= a.maxFrames
	if a.pngPath != "" && (last || a.maxFrames == 0 && a.frames == 1) {
		if err := a.savePNG(); err != nil {
			log.Fatal(err)
		}
	}
	if err := a.device.EndFrame(a.surface); err != nil {
		log.Fatal(err)
	}
	a.dirty = false
	a.report()
	if last {
		a.close()
	}
}

func (a *app) report() {
	a.statsFrames++
	elapsed := time.Since(a.statsSince)
	if elapsed < time.Second {
		return
	}
	fps := float64(a.statsFrames) / elapsed.Seconds()
	cpu := a.cpuTime.Seconds() * 1000 / float64(a.statsFrames)
	line := fmt.Sprintf("%.1f fps, cpu %.2f ms/frame, %d instances, %d draws", fps, cpu, len(a.batch.Instances), len(a.batch.Draws))
	log.Print(line)
	a.window.SetTitle("gowaku: " + line)
	a.statsSince, a.statsFrames, a.cpuTime = time.Now(), 0, 0
}

func (a *app) savePNG() error {
	img, err := a.device.ReadPixels(a.surface)
	if err != nil {
		return err
	}
	f, err := os.Create(a.pngPath)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close() //nolint:errcheck // the encoding error is the one worth reporting
		return err
	}
	return f.Close()
}

// drawStress moves count rectangles. With no explicit size they tile the
// window without overlap, which is what a dense UI looks like.
func (a *app) drawStress() {
	size := a.logicalSize()
	columns := int(math.Ceil(math.Sqrt(float64(a.count) * float64(size.W/size.H))))
	rows := (a.count + columns - 1) / columns
	cellW, cellH := size.W/float32(columns), size.H/float32(rows)
	width, height := a.rectWidth, a.rectHeight
	if width == 0 {
		width = cellW - 2
	}
	if height == 0 {
		height = cellH - 2
	}
	phase := float32(time.Since(a.start).Seconds())
	border := geom.Hex(0x6750a4)
	shadow := geom.Hex(0).WithAlpha(0.3)
	for i := range a.count {
		column, row := i%columns, i/columns
		sway := geom.Sin32(phase*2+float32(i)*0.01) * 2
		r := geom.Rect{X: float32(column)*cellW + 1 + sway, Y: float32(row)*cellH + 1, W: width, H: height}
		rr := geom.RRectAll(r, 3)
		if a.shadows {
			a.canvas.DrawShadow(rr, 6, shadow, geom.Offset{Y: 1})
		}
		a.canvas.DrawRRect(rr, paint.Paint{
			Stroke: &paint.Stroke{Width: 1, Color: border},
			Gradient: &paint.Gradient{
				Start: r.Origin(), End: geom.Offset{X: r.X + r.W, Y: r.Y + r.H},
				StartColor: geom.Hex(0xeaddff), EndColor: geom.Hex(0xb69df8),
			},
		})
	}
}

// layer is one CSS-style box shadow.
type layer struct {
	y, blur, spread, alpha float32
}

// elevations are the MD3 shadow tokens, each a key and an ambient layer.
var elevations = [...][2]layer{
	{{1, 2, 0, 0.3}, {1, 3, 1, 0.15}},
	{{1, 2, 0, 0.3}, {2, 6, 2, 0.15}},
	{{1, 3, 0, 0.3}, {4, 8, 3, 0.15}},
	{{2, 3, 0, 0.3}, {6, 10, 4, 0.15}},
	{{4, 4, 0, 0.3}, {8, 12, 6, 0.15}},
}

// drawCard draws a surface at the given elevation level, counted from 1.
func drawCard(c *paint.Canvas, r geom.Rect, radius float32, level int) {
	for _, l := range elevations[level-1] {
		spread := geom.Rect{X: r.X - l.spread, Y: r.Y - l.spread, W: r.W + 2*l.spread, H: r.H + 2*l.spread}
		c.DrawShadow(geom.RRectAll(spread, radius+l.spread), l.blur, geom.Hex(0).WithAlpha(l.alpha), geom.Offset{Y: l.y})
	}
	c.DrawRRect(geom.RRectAll(r, radius), paint.Paint{Color: surfaceColor})
}

// drawElevation draws the elevations, to compare with the same shadows in a
// browser.
func drawElevation(c *paint.Canvas) {
	for i := range elevations {
		drawCard(c, geom.Rect{X: 40 + float32(i)*180, Y: 60, W: 140, H: 100}, 12, i+1)
	}
}

func drawGallery(c *paint.Canvas) {
	purple := geom.Hex(0x6750a4)
	lavender := geom.Hex(0xeaddff)
	cell := func(column, row int) geom.Rect {
		return geom.Rect{X: 30 + float32(column)*150, Y: 30 + float32(row)*120, W: 130, H: 90}
	}

	// Corners.
	c.DrawRRect(geom.RRect{Rect: cell(0, 0)}, paint.Paint{Color: purple})
	c.DrawRRect(geom.RRectAll(cell(1, 0), 16), paint.Paint{Color: purple})
	c.DrawRRect(geom.RRectAll(cell(2, 0), 100), paint.Paint{Color: purple})
	c.DrawRRect(geom.RRect{Rect: cell(3, 0), TopLeft: 40, BottomRight: 40}, paint.Paint{Color: purple})
	c.DrawRRect(geom.RRect{Rect: cell(4, 0), TopRight: 60, BottomLeft: 8}, paint.Paint{Color: purple})

	// Borders.
	c.DrawRRect(geom.RRectAll(cell(0, 1), 12), paint.Paint{Color: lavender, Stroke: &paint.Stroke{Width: 1, Color: purple}})
	c.DrawRRect(geom.RRectAll(cell(1, 1), 12), paint.Paint{Color: lavender, Stroke: &paint.Stroke{Width: 6, Color: purple}})
	c.DrawRRect(geom.RRectAll(cell(2, 1), 45), paint.Paint{Stroke: &paint.Stroke{Width: 3, Color: purple}})
	c.DrawRRect(geom.RRect{Rect: cell(3, 1)}, paint.Paint{Stroke: &paint.Stroke{Width: 2, Color: purple}})
	c.DrawRRect(geom.RRectAll(cell(4, 1), 20), paint.Paint{Color: purple.WithAlpha(0.3), Stroke: &paint.Stroke{Width: 4, Color: purple.WithAlpha(0.6)}})

	// Gradients.
	r := cell(0, 2)
	c.DrawRRect(geom.RRectAll(r, 12), paint.Paint{Gradient: &paint.Gradient{
		Start: r.Origin(), End: geom.Offset{X: r.X + r.W, Y: r.Y}, StartColor: purple, EndColor: lavender,
	}})
	r = cell(1, 2)
	c.DrawRRect(geom.RRectAll(r, 12), paint.Paint{Gradient: &paint.Gradient{
		Start: r.Origin(), End: geom.Offset{X: r.X + r.W, Y: r.Y + r.H}, StartColor: geom.Hex(0xef5350), EndColor: geom.Hex(0x42a5f5),
	}})
	r = cell(2, 2)
	c.DrawRRect(geom.RRectAll(r, 45), paint.Paint{Gradient: &paint.Gradient{
		Kind: paint.GradientRadial, Start: r.Center(), End: geom.Offset{X: r.X + r.W/2, Y: r.Y},
		StartColor: lavender, EndColor: purple,
	}})
	r = cell(3, 2)
	c.DrawRRect(geom.RRectAll(r, 12), paint.Paint{
		Stroke: &paint.Stroke{Width: 2, Color: purple},
		Gradient: &paint.Gradient{
			Start: r.Origin(), End: geom.Offset{X: r.X, Y: r.Y + r.H}, StartColor: purple, EndColor: purple.WithAlpha(0),
		},
	})

	// Clip and opacity.
	r = cell(4, 2)
	c.Save()
	c.ClipRect(r)
	for i := range 6 {
		c.DrawRRect(geom.RRectAll(geom.Rect{X: r.X - 20 + float32(i)*36, Y: r.Y - 20 + float32(i)*16, W: 60, H: 60}, 30), paint.Paint{Color: purple.WithAlpha(0.5)})
	}
	c.Restore()

	// Elevation.
	for i := range elevations {
		r := cell(i, 3)
		drawCard(c, geom.Rect{X: r.X, Y: r.Y + 10, W: r.W, H: r.H - 20}, 12, i+1)
	}

	c.Save()
	c.PushOpacity(0.4)
	r = geom.Rect{X: 790, Y: 30, W: 130, H: 450}
	c.DrawShadow(geom.RRectAll(r, 16), 16, geom.Hex(0).WithAlpha(0.5), geom.Offset{Y: 4})
	c.DrawRRect(geom.RRectAll(r, 16), paint.Paint{Color: purple, Stroke: &paint.Stroke{Width: 4, Color: geom.Hex(0x000000)}})
	c.Restore()
}
