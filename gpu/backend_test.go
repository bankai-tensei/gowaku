package gpu

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestPhysicalBounds(t *testing.T) {
	tests := []struct {
		name                     string
		r                        geom.Rect
		scale                    float32
		left, top, right, bottom int32
	}{
		{"等倍", geom.Rect{X: 1, Y: 2, W: 3, H: 4}, 1, 1, 2, 4, 6},
		{"150% の丸め", geom.Rect{X: 1, Y: 1, W: 1, H: 1}, 1.5, 2, 2, 3, 3},
		{"負の座標", geom.Rect{X: -1, Y: -1, W: 2, H: 2}, 1, -1, -1, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, tp, r, b := PhysicalBounds(tt.r, tt.scale)
			if l != tt.left || tp != tt.top || r != tt.right || b != tt.bottom {
				t.Errorf("got %d %d %d %d", l, tp, r, b)
			}
		})
	}
}

func TestPhysicalBounds_AdjacentRectsShareAnEdge(t *testing.T) {
	const scale = 1.25
	for x := float32(0); x < 50; x += 0.25 {
		_, _, right, _ := PhysicalBounds(geom.Rect{X: 0, W: x}, scale)
		left, _, _, _ := PhysicalBounds(geom.Rect{X: x, W: 10}, scale)
		if right != left {
			t.Fatalf("x=%v: edges %d and %d differ", x, right, left)
		}
	}
}

func TestBatch_Multisampled(t *testing.T) {
	tests := []struct {
		name  string
		draws []DrawCall
		want  bool
	}{
		{"空", nil, false},
		{"四角と文字だけ", []DrawCall{{Kind: DrawQuads}, {Kind: DrawQuads}}, false},
		{"パスの塗り", []DrawCall{{Kind: DrawQuads}, {Kind: DrawFill}}, true},
		{"形のあるクリップ", []DrawCall{{Kind: DrawClipPush}}, true},
		{"クリップの解除", []DrawCall{{Kind: DrawClipPop}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := Batch{Draws: tt.draws}
			if got := b.Multisampled(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
