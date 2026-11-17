package control_test

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// spectrum shows bars that change 30 times a second.
type spectrum struct{ core.StatefulBase }

type spectrumState struct {
	core.StateBase
	bars    [32]float32
	repaint layout.RepaintSignal
	ticker  *core.Ticker
}

func (spectrum) CreateState() core.State { return &spectrumState{} }

func (s *spectrumState) InitState() {
	s.ticker = s.NewPeriodicTicker(time.Second/30, func(now time.Duration) {
		for i := range s.bars {
			s.bars[i] = float32((int(now/time.Millisecond)+i*37)%100) / 100
		}
		// Not SetState: only the picture changes, so nothing is built or laid
		// out again.
		s.repaint.Notify()
	})
	s.ticker.Start()
}

// Paint implements layout.Painter. It reads the bars on every paint.
func (s *spectrumState) Paint(cv *paint.Canvas, sz geom.Size) {
	w := sz.W / float32(len(s.bars))
	for i, v := range s.bars {
		h := sz.H * v
		r := geom.Rect{X: float32(i) * w, Y: sz.H - h, W: w - 2, H: h}
		cv.DrawRRect(geom.RRectAll(r, 2), paint.Paint{Color: geom.Hex(0x0a84ff)})
	}
}

func (s *spectrumState) Build(core.BuildContext) core.Widget {
	return basic.RepaintBoundary{Child: control.CustomPaint{
		Painter: s, Height: 80, Repaint: &s.repaint,
	}}
}

// A State that only changes what it paints notifies a RepaintSignal instead of
// calling SetState.
func ExampleCustomPaint_repaint() {
	var w core.Widget = spectrum{}
	_ = w
}
