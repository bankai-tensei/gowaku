package desktop

import (
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
)

func TestBrightnessOf(t *testing.T) {
	tests := []struct {
		name          string
		scheme, theme string
		want          platform.Brightness
	}{
		{"ダーク指定", "'prefer-dark'\n", "'Yaru'\n", platform.BrightnessDark},
		{"ライト指定はテーマ名に勝つ", "'prefer-light'\n", "'Adwaita-dark'\n", platform.BrightnessLight},
		{"既定はテーマ名で決まる", "'default'\n", "'Yaru-dark'\n", platform.BrightnessDark},
		{"color-scheme が無い環境", "", "'Adwaita-dark'\n", platform.BrightnessDark},
		{"何も分からない", "", "", platform.BrightnessLight},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BrightnessOf(tt.scheme, tt.theme); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
