//go:build gpu

package vulkan

import (
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32"
)

func newPlatform(t *testing.T) platform.Platform {
	t.Helper()
	p, err := win32.New()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func foreignHandle() platform.NativeHandle { return platform.X11Handle{} }
