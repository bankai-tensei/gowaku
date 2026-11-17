package theme

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

type brand struct{ Highlight geom.Color }

type spacing struct{ Gutter float32 }

type named interface{ name() string }

type labeled struct{ s string }

func (l labeled) name() string { return l.s }

func TestExtension_MissingGivesZeroAndFalse(t *testing.T) {
	th := New(geom.Hex(defaultAccent), BrightnessLight)
	if v, ok := Extension[brand](th); ok || v != (brand{}) {
		t.Errorf("got %+v, %v", v, ok)
	}
	if v, ok := Extension[brand](nil); ok || v != (brand{}) {
		t.Errorf("nil theme: got %+v, %v", v, ok)
	}
	fallback := brand{Highlight: geom.Hex(0x123456)}
	if got := ExtensionOr(th, fallback); got != fallback {
		t.Errorf("ExtensionOr gave %+v", got)
	}
}

func TestWithExtension_CopiesAndReplacesByType(t *testing.T) {
	base := New(geom.Hex(defaultAccent), BrightnessLight)
	a := WithExtension(base, brand{Highlight: geom.Hex(0xff0000)})
	a = WithExtension(a, spacing{Gutter: 12})
	b := WithExtension(a, brand{Highlight: geom.Hex(0x00ff00)})

	if _, ok := Extension[brand](base); ok {
		t.Error("the original theme changed")
	}
	if v, _ := Extension[brand](a); v.Highlight != geom.Hex(0xff0000) {
		t.Errorf("a lost its value: %+v", v)
	}
	if v, _ := Extension[brand](b); v.Highlight != geom.Hex(0x00ff00) {
		t.Errorf("b did not replace the value: %+v", v)
	}
	if v, ok := Extension[spacing](b); !ok || v.Gutter != 12 {
		t.Errorf("b lost the other extension: %+v, %v", v, ok)
	}
	if len(b.extensions) != 2 {
		t.Errorf("b holds %d values, want 2", len(b.extensions))
	}
	if b.Color != base.Color || b.Brightness != base.Brightness {
		t.Error("the standard tokens were not copied")
	}
}

func TestExtension_InterfaceFindsAnImplementer(t *testing.T) {
	th := WithExtension(Default().Light, labeled{s: "x"})
	n, ok := Extension[named](th)
	if !ok || n.name() != "x" {
		t.Errorf("got %v, %v", n, ok)
	}
	if _, ok := Extension[*labeled](th); ok {
		t.Error("a pointer type matched a value")
	}
}

func TestWithExtensionPair_GivesEachBrightnessItsValue(t *testing.T) {
	light, dark := brand{Highlight: geom.Hex(0xffd60a)}, brand{Highlight: geom.Hex(0x806b05)}
	s := WithExtensionPair(Set{Dark: New(geom.Hex(defaultAccent), BrightnessDark)}, light, dark)
	if got, _ := Extension[brand](s.For(BrightnessLight)); got != light {
		t.Errorf("light got %+v", got)
	}
	if got, _ := Extension[brand](s.For(BrightnessDark)); got != dark {
		t.Errorf("dark got %+v", got)
	}
	if s.Light == nil {
		t.Error("the missing light theme was not filled in")
	}
	if _, ok := Extension[brand](Default().Light); ok {
		t.Error("the default theme changed")
	}
}

func TestWithExtension_NilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	WithExtension[named](Default().Light, nil)
}

func BenchmarkExtension_Lookup(b *testing.B) {
	th := Default().Light
	th = WithExtension(th, spacing{Gutter: 8})
	th = WithExtension(th, labeled{s: "x"})
	th = WithExtension(th, brand{Highlight: geom.Hex(0xffd60a)})
	b.ReportAllocs()
	var sink geom.Color
	for b.Loop() {
		v, _ := Extension[brand](th)
		sink = v.Highlight
		sink = ExtensionOr(th, brand{Highlight: sink}).Highlight
	}
	_ = sink
}
