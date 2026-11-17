package theme

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func nearColor(a, b geom.Color) bool {
	const eps = 1e-4
	return geom.Abs32(a.R-b.R) < eps && geom.Abs32(a.G-b.G) < eps && geom.Abs32(a.B-b.B) < eps && geom.Abs32(a.A-b.A) < eps
}

func TestHSV_KnownColors(t *testing.T) {
	cases := []struct {
		name string
		c    geom.Color
		want HSV
	}{
		{"赤", geom.Hex(0xff0000), HSV{H: 0, S: 1, V: 1, A: 1}},
		{"黄", geom.Hex(0xffff00), HSV{H: 60, S: 1, V: 1, A: 1}},
		{"緑", geom.Hex(0x00ff00), HSV{H: 120, S: 1, V: 1, A: 1}},
		{"シアン", geom.Hex(0x00ffff), HSV{H: 180, S: 1, V: 1, A: 1}},
		{"青", geom.Hex(0x0000ff), HSV{H: 240, S: 1, V: 1, A: 1}},
		{"マゼンタ", geom.Hex(0xff00ff), HSV{H: 300, S: 1, V: 1, A: 1}},
		{"白", geom.Hex(0xffffff), HSV{H: 0, S: 0, V: 1, A: 1}},
		{"黒", geom.Hex(0x000000), HSV{A: 1}},
		{"半透明の灰", geom.Color{R: 0.5, G: 0.5, B: 0.5, A: 0.25}, HSV{V: 0.5, A: 0.25}},
		{"ゼロ値", geom.Color{}, HSV{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HSVFromColor(tc.c)
			if geom.Abs32(got.H-tc.want.H) > 1e-3 || geom.Abs32(got.S-tc.want.S) > 1e-5 ||
				geom.Abs32(got.V-tc.want.V) > 1e-5 || got.A != tc.want.A {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
			if back := got.Color(); !nearColor(back, tc.c) {
				t.Errorf("round trip got %+v, want %+v", back, tc.c)
			}
		})
	}
}

func TestHSV_RoundTripsEvery8BitStep(t *testing.T) {
	for r := 0; r < 256; r += 15 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 51 {
				c := geom.Color{R: float32(r) / 255, G: float32(g) / 255, B: float32(b) / 255, A: 1}
				if back := HSVFromColor(c).Color(); !nearColor(back, c) {
					t.Fatalf("%v: got %+v", c, back)
				}
			}
		}
	}
}

func TestHSV_ColorWrapsAndClamps(t *testing.T) {
	if got := (HSV{H: 360 + 120, S: 1, V: 1, A: 1}).Color(); !nearColor(got, geom.Hex(0x00ff00)) {
		t.Errorf("hue past 360 wraps, got %+v", got)
	}
	if got := (HSV{H: -120, S: 1, V: 1, A: 1}).Color(); !nearColor(got, geom.Hex(0x0000ff)) {
		t.Errorf("negative hue wraps, got %+v", got)
	}
	if got := (HSV{H: 0, S: 2, V: 3, A: 4}).Color(); !nearColor(got, geom.Hex(0xff0000)) {
		t.Errorf("out of range clamps, got %+v", got)
	}
}

func TestHSVFromColorLike_KeepsHueAtTheEdges(t *testing.T) {
	like := HSV{H: 210, S: 0.6, V: 0.8, A: 1}
	black := HSVFromColorLike(geom.Hex(0), like)
	if black.H != 210 || black.S != 0.6 || black.V != 0 {
		t.Errorf("black keeps hue and saturation, got %+v", black)
	}
	gray := HSVFromColorLike(geom.Hex(0x808080), like)
	if gray.H != 210 || gray.S != 0 {
		t.Errorf("gray keeps the hue, got %+v", gray)
	}
	red := HSVFromColorLike(geom.Hex(0xff0000), like)
	if red.H != 0 || red.S != 1 {
		t.Errorf("a color decides its own hue, got %+v", red)
	}
}

func TestParseHex(t *testing.T) {
	cases := []struct {
		in   string
		want geom.Color
		ok   bool
	}{
		{"#fff", geom.Hex(0xffffff), true},
		{"#F80", geom.Hex(0xff8800), true},
		{"#f808", geom.Color{R: 1, G: 0x88 / 255.0, B: 0, A: 0x88 / 255.0}, true},
		{"#3A7BF0", geom.Hex(0x3a7bf0), true},
		{"#3a7bf080", geom.Color{R: 0x3a / 255.0, G: 0x7b / 255.0, B: 0xf0 / 255.0, A: 0x80 / 255.0}, true},
		{"3a7bf0", geom.Hex(0x3a7bf0), true},
		{"0x3A7BF0", geom.Hex(0x3a7bf0), true},
		{"  #3a7bf0\n", geom.Hex(0x3a7bf0), true},
		{`"#3a7bf0";`, geom.Hex(0x3a7bf0), true},
		{"'#abc',", geom.Hex(0xaabbcc), true},
		{"", geom.Color{}, false},
		{"#", geom.Color{}, false},
		{"#ff", geom.Color{}, false},
		{"#fffff", geom.Color{}, false},
		{"#fffffffff", geom.Color{}, false},
		{"#ggg", geom.Color{}, false},
		{"# fff", geom.Color{}, false},
		{"##fff", geom.Color{}, false},
		{"rgb(1,2,3)", geom.Color{}, false},
		{"＃fff", geom.Color{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := ParseHex(tc.in)
			if ok != tc.ok || ok && !nearColor(got, tc.want) {
				t.Errorf("got %+v %v, want %+v %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestFormatHex(t *testing.T) {
	cases := []struct {
		c    geom.Color
		want string
	}{
		{geom.Hex(0x3a7bf0), "#3A7BF0"},
		{geom.Color{R: 1, A: 0.5}, "#FF000080"},
		{geom.Color{}, "#00000000"},
		{geom.Color{R: 2, G: -1, B: 0.5, A: 1}, "#FF0080"},
		{geom.Color{R: 1, G: 1, B: 1, A: 0.999}, "#FFFFFF"},
	}
	for _, tc := range cases {
		if got := FormatHex(tc.c); got != tc.want {
			t.Errorf("%+v: got %q, want %q", tc.c, got, tc.want)
		}
		back, ok := ParseHex(FormatHex(tc.c))
		if !ok || FormatHex(back) != tc.want {
			t.Errorf("%q does not read back, got %+v", tc.want, back)
		}
	}
}
