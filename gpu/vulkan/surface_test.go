//go:build linux || windows

package vulkan

import "testing"

func TestCompositeAlpha(t *testing.T) {
	const (
		opaque  = VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR
		pre     = VK_COMPOSITE_ALPHA_PRE_MULTIPLIED_BIT_KHR
		post    = VK_COMPOSITE_ALPHA_POST_MULTIPLIED_BIT_KHR
		inherit = VK_COMPOSITE_ALPHA_INHERIT_BIT_KHR
	)
	tests := []struct {
		name        string
		supported   uint32
		transparent bool
		want        uint32
	}{
		{"不透明: 不透明がある", opaque | pre, false, opaque},
		{"不透明: 不透明が無い（X11 の ARGB visual）", pre | inherit, false, pre},
		{"透過: 乗算済みがある", opaque | pre, true, pre},
		{"透過: 継承だけ", opaque | inherit, true, inherit},
		{"透過: 不透明だけ（Windows の多くのドライバ）", opaque, true, opaque},
		{"透過: 乗算前だけ", post, true, post},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compositeAlpha(tt.supported, tt.transparent); got != tt.want {
				t.Errorf("got %#x, want %#x", got, tt.want)
			}
		})
	}
}
