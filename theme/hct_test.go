package theme

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func linearOf(rgb uint32) [3]float64 {
	return [3]float64{
		linearized(float64(rgb >> 16 & 0xff)),
		linearized(float64(rgb >> 8 & 0xff)),
		linearized(float64(rgb & 0xff)),
	}
}

func near(got, want, tolerance float64) bool { return math.Abs(got-want) <= tolerance }

func TestCam16_UpstreamVectors(t *testing.T) {
	for _, row := range readVectors(t, "cam16.txt") {
		t.Run(row[0], func(t *testing.T) {
			cam := cam16FromLinear(linearOf(parseRGB(t, row[0])))
			fields := []struct {
				name string
				got  float64
				want string
			}{
				{"j", cam.j, row[1]},
				{"chroma", cam.chroma, row[2]},
				{"hue", cam.hue, row[3]},
				{"m", cam.m, row[4]},
				{"s", cam.s, row[5]},
				{"q", cam.q, row[6]},
			}
			for _, f := range fields {
				if want := parseFloat(t, f.want); !near(f.got, want, 0.001) {
					t.Errorf("%s: got %v, want %v", f.name, f.got, want)
				}
			}
		})
	}
}

func TestLstar_UpstreamVectors(t *testing.T) {
	for _, row := range readVectors(t, "lstar.txt") {
		in, want := parseFloat(t, row[1]), parseFloat(t, row[2])
		var got float64
		switch row[0] {
		case "y_from_lstar":
			got = yFromLstar(in)
		case "lstar_from_y":
			got = lstarFromY(in)
		default:
			t.Fatalf("unknown function %q", row[0])
		}
		if !near(got, want, 1e-5) {
			t.Errorf("%s(%v): got %v, want %v", row[0], in, got, want)
		}
	}
}

func TestLstar_InversesAndContinuity(t *testing.T) {
	for i := range 1001 {
		v := float64(i) / 10
		if got := yFromLstar(lstarFromY(v)); !near(got, v, 1e-5) {
			t.Errorf("y %v: got %v", v, got)
		}
		if got := lstarFromY(yFromLstar(v)); !near(got, v, 1e-5) {
			t.Errorf("lstar %v: got %v", v, got)
		}
	}
	// The two branches of labInvF meet at L* 8.
	const delta = 1e-8
	if !near(yFromLstar(8-delta), yFromLstar(8), 1e-6) || !near(yFromLstar(8+delta), yFromLstar(8), 1e-6) {
		t.Error("yFromLstar is discontinuous at 8")
	}
}

func TestLinearized_RoundTrip(t *testing.T) {
	for c := range uint32(256) {
		if got := delinearized(linearized(float64(c))); got != c {
			t.Errorf("channel %d: got %d", c, got)
		}
	}
}

func TestRGBFromLstar_GrayRoundTrip(t *testing.T) {
	for c := range uint32(256) {
		rgb := c<<16 | c<<8 | c
		tone := HCTFromColor(geom.Hex(rgb)).Tone
		if got := rgbFromLstar(tone); got != rgb {
			t.Errorf("gray %06x: got %06x", rgb, got)
		}
	}
}

func TestCriticalPlane_UpstreamTable(t *testing.T) {
	rows := readVectors(t, "critical_planes.txt")
	if len(rows) != 255 {
		t.Fatalf("got %d planes, want 255", len(rows))
	}
	for i, row := range rows {
		want := parseFloat(t, row[0])
		if got := criticalPlane(i); !near(got, want, 1e-12) {
			t.Errorf("plane %d: got %v, want %v", i, got, want)
		}
	}
}

func TestHCT_RoundTrip(t *testing.T) {
	// Upstream's own grid uses 37. A step of 5 covers far more of the cube.
	step := 5
	if testing.Short() {
		step = 37
	}
	for r := 0; r < 255+step; r += step {
		for g := 0; g < 255+step; g += step {
			for b := 0; b < 255+step; b += step {
				rgb := uint32(min(r, 255))<<16 | uint32(min(g, 255))<<8 | uint32(min(b, 255))
				want := geom.Hex(rgb)
				if got := HCTFromColor(want).Color(); got != want {
					t.Errorf("%06x: got %v, want %v", rgb, got, want)
				}
			}
		}
	}
}

func TestHCT_GamutMapPrimaries(t *testing.T) {
	// gamutMap_* of upstream's hct_test.dart.
	tests := []struct {
		name string
		rgb  uint32
	}{
		{"赤", 0xff0000},
		{"緑", 0x00ff00},
		{"青", 0x0000ff},
		{"白", 0xffffff},
		{"黒", 0x000000},
		{"中間の灰色", 0x777777},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := geom.Hex(tt.rgb)
			if got := HCTFromColor(want).Color(); got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

// onBoundary reports whether a color lies on a face of the RGB cube.
func onBoundary(c geom.Color) bool {
	s := c.Std()
	for _, v := range [3]uint8{s.R, s.G, s.B} {
		if v == 0 || v == 255 {
			return true
		}
	}
	return false
}

// The sweep and its limits are those of "HCT returns a sufficiently close color"
// in upstream's hct_test.dart.
func TestHCT_SolvesCloseToRequest(t *testing.T) {
	for hue := 15.0; hue < 360; hue += 30 {
		for chroma := 0.0; chroma <= 100; chroma += 10 {
			for tone := 20.0; tone <= 80; tone += 10 {
				request := HCT{Hue: hue, Chroma: chroma, Tone: tone}
				c := request.Color()
				got := HCTFromColor(c)
				if chroma > 0 && !near(got.Hue, hue, 4) {
					t.Errorf("%+v: hue %v", request, got.Hue)
				}
				if got.Chroma < 0 || got.Chroma > chroma+2.5 {
					t.Errorf("%+v: chroma %v", request, got.Chroma)
				}
				if got.Chroma < chroma-2.5 && !onBoundary(c) {
					t.Errorf("%+v: reduced chroma %v, but %v is not on the gamut boundary", request, got.Chroma, c)
				}
				if !near(got.Tone, tone, 0.5) {
					t.Errorf("%+v: tone %v", request, got.Tone)
				}
			}
		}
	}
}

func TestHCT_Boundaries(t *testing.T) {
	white, black := geom.Hex(0xffffff), geom.Hex(0x000000)
	tests := []struct {
		name string
		hct  HCT
		want geom.Color
	}{
		{"ゼロ値は黒", HCT{}, black},
		{"トーン 0 は色相によらず黒", HCT{Hue: 120, Chroma: 80, Tone: 0}, black},
		{"トーン 100 は色相によらず白", HCT{Hue: 120, Chroma: 80, Tone: 100}, white},
		{"トーンが範囲の上を超える", HCT{Hue: 120, Chroma: 80, Tone: 150}, white},
		{"トーンが負", HCT{Hue: 120, Chroma: 80, Tone: -5}, black},
		{"彩度が負なら灰色", HCT{Hue: 120, Chroma: -5, Tone: 100}, white},
		{"色相が 360 以上でも同じ色", HCT{Hue: 480, Chroma: 40, Tone: 50}, HCT{Hue: 120, Chroma: 40, Tone: 50}.Color()},
		{"色相が負でも同じ色", HCT{Hue: -240, Chroma: 40, Tone: 50}, HCT{Hue: 120, Chroma: 40, Tone: 50}.Color()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hct.Color(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHCT_ExtremeChromaStaysInGamut(t *testing.T) {
	for hue := 0.0; hue < 360; hue += 7 {
		for tone := 1.0; tone < 100; tone += 7 {
			request := HCT{Hue: hue, Chroma: 1000, Tone: tone}
			c := request.Color()
			if !onBoundary(c) {
				t.Errorf("%+v: %v is not on the gamut boundary", request, c)
			}
			if got := HCTFromColor(c).Tone; !near(got, tone, 0.6) {
				t.Errorf("%+v: tone %v", request, got)
			}
		}
	}
}

func TestHCT_NaNDoesNotPanic(t *testing.T) {
	nan := math.NaN()
	for _, h := range []HCT{{Hue: nan, Chroma: 40, Tone: 50}, {Hue: 120, Chroma: nan, Tone: 50}, {Hue: 120, Chroma: 40, Tone: nan}} {
		if got := h.Color(); got.A != 1 {
			t.Errorf("%+v: alpha %v", h, got.A)
		}
	}
}

func TestHCTFromColor_IgnoresAlphaAndClamps(t *testing.T) {
	want := HCTFromColor(geom.Hex(0x6750a4))
	if got := HCTFromColor(geom.Hex(0x6750a4).WithAlpha(0.25)); got != want {
		t.Errorf("alpha: got %+v, want %+v", got, want)
	}
	if got, want := HCTFromColor(geom.RGBA(2, -1, 0, 1)), HCTFromColor(geom.Hex(0xff0000)); got != want {
		t.Errorf("clamp: got %+v, want %+v", got, want)
	}
}

func TestHCTFromColor_KeepsFractionalChannels(t *testing.T) {
	lo := HCTFromColor(geom.RGBA(0.5, 0.5, 0.5, 1)).Tone
	hi := HCTFromColor(geom.RGBA(0.501, 0.501, 0.501, 1)).Tone
	if lo >= hi {
		t.Errorf("tones %v and %v are not ordered", lo, hi)
	}
}
