package text

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestStyleWith_ReplacesOneField(t *testing.T) {
	base := Style{Family: "Noto Sans JP", Size: 15, Weight: 600, LineHeight: 1.4, Color: geom.Color{R: 1, A: 1}, Align: AlignCenter}
	blue := geom.Color{B: 1, A: 1}
	cases := []struct {
		name string
		got  Style
		want func(Style) Style
	}{
		{"WithColor", base.WithColor(blue), func(s Style) Style { s.Color = blue; return s }},
		{"WithSize", base.WithSize(20), func(s Style) Style { s.Size, s.LineHeight = 20, 28.0/20; return s }},
		{"WithAlign", base.WithAlign(AlignEnd), func(s Style) Style { s.Align = AlignEnd; return s }},
		{"WithWeight", base.WithWeight(300), func(s Style) Style { s.Weight = 300; return s }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if want := c.want(base); c.got != want {
				t.Errorf("got %+v, want %+v", c.got, want)
			}
		})
	}
	if base.Color != (geom.Color{R: 1, A: 1}) || base.Size != 15 || base.Weight != 600 {
		t.Errorf("the receiver changed: %+v", base)
	}
}

func TestStyleWith_ZeroKeepsDefaults(t *testing.T) {
	s := Style{Size: 20, Weight: 700, Color: geom.Color{G: 1, A: 1}}.WithSize(0).WithWeight(0).WithColor(geom.Color{})
	if !s.SameLayout(Style{}) {
		t.Errorf("zero size and weight do not select the defaults: %+v", s.resolved())
	}
	if got := s.PaintColor(); got != (geom.Color{A: 1}) {
		t.Errorf("PaintColor = %v, want opaque black", got)
	}
}

func TestStyleWith_ColorKeepsLayout(t *testing.T) {
	s := Style{Size: 16}
	if !s.SameLayout(s.WithColor(geom.Color{R: 1, A: 1})) {
		t.Error("a new color changed the layout")
	}
	if s.SameLayout(s.WithSize(17)) {
		t.Error("a new size kept the layout")
	}
}

func TestStyleWith_NoAllocs(t *testing.T) {
	s := Style{Size: 16}
	var sink Style
	if n := testing.AllocsPerRun(100, func() { sink = s.WithColor(geom.Color{A: 1}).WithSize(12) }); n != 0 {
		t.Errorf("allocs = %v, want 0", n)
	}
	_ = sink
}

func TestStyleWith_SizeKeepsTheLineInPixels(t *testing.T) {
	cases := []struct {
		name       string
		from       Style
		size, line float32
	}{
		{"smaller keeps the line", Style{Size: 12, LineHeight: 18.0 / 12}, 11, 18},
		{"a little larger keeps the line", Style{Size: 13, LineHeight: 20.0 / 13}, 15, 20},
		{"much larger grows to an even line", Style{Size: 13, LineHeight: 20.0 / 13}, 24, 38},
		{"zero size is 14", Style{Size: 12, LineHeight: 18.0 / 12}, 0, 18},
		{"no line height keeps the default", Style{Size: 12}, 20, 30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := c.from.WithSize(c.size).resolved()
			if got := s.Size * s.LineHeight; got < c.line-1e-3 || got > c.line+1e-3 {
				t.Errorf("line = %v px, want %v", got, c.line)
			}
		})
	}
}

func TestStyleWith_DoesNotAllocateWithSize(t *testing.T) {
	s := Style{Size: 13, LineHeight: 20.0 / 13}
	if n := testing.AllocsPerRun(100, func() { s = s.WithSize(11).WithAlign(AlignEnd) }); n != 0 {
		t.Errorf("%v allocations", n)
	}
}

func TestStyle_LinePixels(t *testing.T) {
	if got := (Style{Size: 13, LineHeight: 20.0 / 13}).LinePixels(); got < 19.999 || got > 20.001 {
		t.Errorf("Body line = %v, want 20", got)
	}
	if got := (Style{}).LinePixels(); got != 21 {
		t.Errorf("zero style line = %v, want 21", got)
	}
}
