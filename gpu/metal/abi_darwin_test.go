//go:build darwin && arm64

package metal

import (
	"testing"
	"unsafe"

	"github.com/bankai-tensei/gowaku/gpu"
)

// TestABI_Sizes compares the structs with the C sizes of Metal.h, and
// gpu.Instance with struct Instance of sdf.metal, whose packed members follow
// the Go fields one by one.
func TestABI_Sizes(t *testing.T) {
	var in gpu.Instance
	for _, c := range []struct {
		name      string
		got, want uintptr
	}{
		{"MTLViewport", unsafe.Sizeof(MTLViewport{}), 48},
		{"MTLScissorRect", unsafe.Sizeof(MTLScissorRect{}), 32},
		{"MTLOrigin", unsafe.Sizeof(MTLOrigin{}), 24},
		{"MTLSize", unsafe.Sizeof(MTLSize{}), 24},
		{"MTLClearColor", unsafe.Sizeof(MTLClearColor{}), 32},
		{"frameConstants", unsafe.Sizeof(frameConstants{}), 16},
		{"Instance", unsafe.Sizeof(in), 200},
		{"Instance.gradient", unsafe.Offsetof(in.GradientStart), 80},
		{"Instance.params", unsafe.Offsetof(in.BorderWidth), 96},
		{"Instance.mode", unsafe.Offsetof(in.Kind), 104},
		{"Instance.midColor0", unsafe.Offsetof(in.MidColors), 112},
		{"Instance.stops", unsafe.Offsetof(in.StopOffsets), 144},
		{"Instance.transform", unsafe.Offsetof(in.Transform), 160},
		{"Instance.gradientMatrix", unsafe.Offsetof(in.GradientMatrix), 184},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
}
