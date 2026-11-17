//go:build gpu

package golden

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/d3d11"
	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/platform/win32"
)

// TestGolden_GPU holds D3D11 and Vulkan to the pictures of the CPU backend.
// Vulkan runs on the fast GPU, because the Windows driver of Iris Xe damages what
// lies on a shadow. See docs/ADR/0013-vulkan-backend.md.
func TestGolden_GPU(t *testing.T) {
	p, err := win32.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Destroy)
	d, err := d3d11.New(d3d11.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Destroy)
	v, err := vulkan.New(vulkan.Options{Power: gpu.HighPerformance})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Destroy)
	checkOnWindows(t, p, "d3d11", d)
	checkOnWindows(t, p, "vulkan", v)
}
