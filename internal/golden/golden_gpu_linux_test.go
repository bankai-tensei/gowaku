//go:build gpu

package golden

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/internal/linuxwin"
)

// TestGolden_GPU holds Vulkan to the pictures of the CPU backend. In CI that
// means lavapipe under Xvfb. With WAYLAND_DISPLAY set it draws to Wayland.
func TestGolden_GPU(t *testing.T) {
	p, err := linuxwin.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Destroy)
	v, err := vulkan.New(vulkan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Destroy)
	checkOnWindows(t, p, "vulkan", v)
}
