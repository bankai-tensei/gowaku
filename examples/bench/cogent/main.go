// Command cogent runs the scenarios of the benchmark on Cogent Core. See
// ../README.md.
package main

import (
	"fmt"
	"image"
	"image/color"
	"runtime"
	"time"

	"cogentcore.org/core/colors"
	"cogentcore.org/core/core"
	"cogentcore.org/core/events"
	"cogentcore.org/core/icons"
	"cogentcore.org/core/styles"
	"cogentcore.org/core/styles/units"
	"cogentcore.org/core/tree"

	"github.com/bankai-tensei/gowaku/examples/bench/frames"
)

// rec is here because Render has no other way to reach it.
var rec *frames.Recorder

// rowList is a List whose rows are an icon, a text and a button. Like a
// List, it keeps widgets for the visible rows only.
type rowList struct{ core.List }

// Init implements tree.Node.
func (l *rowList) Init() {
	l.List.Init()
	l.SetReadOnly(true)
	l.ShowIndexes = false
}

// Render counts a frame: scrolling the list renders it again.
func (l *rowList) Render() {
	l.List.Render()
	rec.Mark()
}

// MakeRow implements core.Lister.
func (l *rowList) MakeRow(p *tree.Plan, i int) {
	tree.AddAt(p, fmt.Sprintf("row-%d", i), func(w *core.Frame) {
		w.Styler(func(s *styles.Style) {
			s.Direction = styles.Row
			s.Align.Items = styles.Center
			s.Gap.X.Dp(12)
			s.Padding.Set(units.Dp(4), units.Dp(16))
			s.Min.Y.Dp(52)
			s.Grow.Set(1, 0)
		})
		core.NewIcon(w).SetIcon(icons.Star)
		label := core.NewText(w)
		label.Styler(func(s *styles.Style) { s.Grow.Set(1, 0) })
		core.NewButton(w).SetText("開く")
		w.Updater(func() {
			si, _, _ := l.SliceIndex(i)
			label.SetText(frames.Label(si))
		})
	})
}

// box is one of the frames of the animate scenario, which counts frames by
// the first of them.
type box struct {
	core.Frame
	counts bool
}

// Render counts a frame.
func (b *box) Render() {
	b.Frame.Render()
	if b.counts {
		rec.Mark()
	}
}

// ease is the curve of the other programs.
func ease(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// mix returns the color of the boxes at t.
func mix(t float32) color.RGBA {
	lerp := func(a, b float32) uint8 { return uint8(a + (b-a)*t) }
	return color.RGBA{R: lerp(0x2f, 0xd0), G: lerp(0x5f, 0x50), B: lerp(0xd0, 0x2f), A: 0xff}
}

// animate builds the boxes. Cogent Core has no implicit animation: a function
// that runs on every tick restyles them by hand.
func animate(b *core.Body) {
	var t float32
	start := time.Now()
	grid := core.NewFrame(b)
	grid.Styler(func(s *styles.Style) {
		s.Display = styles.Grid
		s.Columns = frames.BoxesSide
		s.Gap.Zero()
	})
	first := true
	for range frames.Boxes {
		cell := core.NewFrame(grid)
		cell.Styler(func(s *styles.Style) {
			s.Min.Set(units.Dp(64))
			s.Max.Set(units.Dp(64))
			s.CenterAll()
			s.Padding.Zero()
		})
		w := tree.New[box](cell)
		w.counts = first
		first = false
		w.Styler(func(s *styles.Style) {
			size := 24 + 32*t
			s.Min.Set(units.Dp(size))
			s.Max.Set(units.Dp(size))
			s.Border.Radius.Set(units.Dp(8))
			s.Background = colors.Uniform(mix(t))
			s.BoxShadow = []styles.Shadow{{
				OffsetY: units.Dp(2), Blur: units.Dp(2 + 14*t), Color: colors.Uniform(colors.Black),
			}}
		})
	}
	grid.Animate(func(*core.Animation) {
		t = ease(frames.Wave(time.Since(start)))
		grid.Update()
	})
}

// scroll moves the list by itself: the wheel of Cogent Core acts where the real
// mouse is, which the harness leaves alone. The list scrolls by whole rows, which
// is how it works under the wheel as well.
func scroll(l *rowList) {
	const rowHeight = 52
	start := time.Now()
	l.Animate(func(*core.Animation) {
		row := int(time.Since(start).Seconds() * frames.ScrollSpeed / rowHeight)
		if row != l.StartIndex {
			l.StartIndex = row
			l.Update()
		}
	})
}

func main() {
	f := frames.Parse()
	rec = frames.New(f)
	b := core.NewBody("bench")
	switch f.Scenario {
	case "animate":
		animate(b)
	case "startup":
		core.NewText(b).SetText("こんにちは")
	default:
		items := make([]int, frames.Rows)
		l := tree.New[rowList](b)
		l.SetSlice(&items)
		l.Styler(func(s *styles.Style) { s.Grow.Set(1, 1) })
		if f.Scenario == "scroll" {
			scroll(l)
		}
	}
	b.OnShow(func(events.Event) {
		rec.Ready()
		// On macOS the window would take the size of its content, at least 60%
		// of the screen, and no harness moves it. gowaku and Gio open at 800x700
		// points; so does this. On Windows the harness sizes every window.
		if runtime.GOOS == "darwin" {
			b.Scene.RenderWindow().SystemWindow.SetWinSize(image.Pt(800, 700))
		}
	})
	b.RunMainWindow()
}
