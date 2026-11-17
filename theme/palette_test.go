package theme

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// upstreamCore returns a palette of CorePalette.of in upstream, whose neutral
// chroma of 4 differs from newCorePalette. "seed" is TonalPalette.fromInt.
func upstreamCore(t *testing.T, seed uint32, name string) TonalPalette {
	t.Helper()
	h := HCTFromColor(geom.Hex(seed))
	switch name {
	case "seed":
		return TonalPalette{Hue: h.Hue, Chroma: h.Chroma}
	case "a1":
		return TonalPalette{Hue: h.Hue, Chroma: max(48, h.Chroma)}
	case "a2":
		return TonalPalette{Hue: h.Hue, Chroma: 16}
	case "a3":
		return TonalPalette{Hue: h.Hue + 60, Chroma: 24}
	case "n1":
		return TonalPalette{Hue: h.Hue, Chroma: 4}
	}
	t.Fatalf("unknown palette %q", name)
	return TonalPalette{}
}

func TestTonalPalette_UpstreamVectors(t *testing.T) {
	for _, file := range []string{"tonal_palette.txt", "core_palette.txt"} {
		for _, row := range readVectors(t, file) {
			p := upstreamCore(t, parseRGB(t, row[0]), row[1])
			tone := parseFloat(t, row[2])
			if got, want := p.Tone(tone), geom.Hex(parseRGB(t, row[3])); got != want {
				t.Errorf("%s: seed %s palette %s tone %v: got %v, want %v", file, row[0], row[1], tone, got, want)
			}
		}
	}
}

func TestTonalPalette_Endpoints(t *testing.T) {
	for _, p := range []TonalPalette{{}, {Hue: 25, Chroma: 84}, {Hue: 300, Chroma: 200}} {
		if got := p.Tone(0); got != geom.Hex(0x000000) {
			t.Errorf("%+v tone 0: got %v", p, got)
		}
		if got := p.Tone(100); got != geom.Hex(0xffffff) {
			t.Errorf("%+v tone 100: got %v", p, got)
		}
	}
}

func TestTonalPalette_YellowTone99(t *testing.T) {
	tests := []struct {
		name    string
		hue     float64
		average bool
	}{
		{"黄の下限", 105, true},
		{"黄の中央", 115, true},
		{"黄の上限の手前", 124.9, true},
		{"一周しても黄", 115 + 360, true},
		{"黄の上限は含まない", 125, false},
		{"黄の下限の手前", 104.9, false},
		{"青", 280, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := TonalPalette{Hue: tt.hue, Chroma: 60}
			want := solveRGB(tt.hue, 60, 99)
			if tt.average {
				want = averageRGB(solveRGB(tt.hue, 60, 98), solveRGB(tt.hue, 60, 100))
			}
			if got := p.Tone(99); got != geom.Hex(want) {
				t.Errorf("got %v, want %06x", got, want)
			}
		})
	}
}

func TestAverageRGB(t *testing.T) {
	tests := []struct {
		name string
		a, b uint32
		want uint32
	}{
		{"同じ色", 0x123456, 0x123456, 0x123456},
		{"黒と白は切り上げ", 0x000000, 0xffffff, 0x808080},
		{"チャンネルは干渉しない", 0xff0000, 0x00ff00, 0x808000},
		{"端数は切り上げ", 0x000001, 0x000002, 0x000002},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := averageRGB(tt.a, tt.b); got != tt.want {
				t.Errorf("got %06x, want %06x", got, tt.want)
			}
		})
	}
}
