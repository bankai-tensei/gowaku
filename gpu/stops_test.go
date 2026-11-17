package gpu

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestInstance_SetStops(t *testing.T) {
	a, b, c, d := geom.Hex(0x110000), geom.Hex(0x002200), geom.Hex(0x000033), geom.Hex(0x444444)
	colors := [MaxStops]geom.Color{a, b, c, d}
	offsets := [MaxStops]float32{0.1, 0.4, 0.7, 0.9}
	tests := []struct {
		name    string
		n       int
		colors  [MaxStops]geom.Color
		offsets [MaxStops]float32
	}{
		{"1 つ", 1, [MaxStops]geom.Color{a, a, a, a}, [MaxStops]float32{0.1, 0.1, 0.1, 0.1}},
		{"2 つは先頭を繰り返す", 2, [MaxStops]geom.Color{a, a, a, b}, [MaxStops]float32{0.1, 0.1, 0.1, 0.4}},
		{"3 つ", 3, [MaxStops]geom.Color{a, a, b, c}, [MaxStops]float32{0.1, 0.1, 0.4, 0.7}},
		{"4 つ", 4, colors, offsets},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in Instance
			in.SetStops(&colors, &offsets, tt.n)
			got := [MaxStops]geom.Color{in.Color, in.MidColors[0], in.MidColors[1], in.EndColor}
			if got != tt.colors {
				t.Errorf("colors: got %v, want %v", got, tt.colors)
			}
			if in.StopOffsets != tt.offsets {
				t.Errorf("offsets: got %v, want %v", in.StopOffsets, tt.offsets)
			}
		})
	}
}

func TestInstance_SetStopsPanicsOnBadCount(t *testing.T) {
	for _, n := range []int{0, MaxStops + 1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%d stops: no panic", n)
				}
			}()
			var in Instance
			var colors [MaxStops]geom.Color
			var offsets [MaxStops]float32
			in.SetStops(&colors, &offsets, n)
		}()
	}
}
