//go:build gpu

package vulkan

import (
	"testing"

	"github.com/bankai-tensei/gowaku/internal/linuxwin"
	"github.com/bankai-tensei/gowaku/platform"
)

func newPlatform(t *testing.T) platform.Platform {
	t.Helper()
	p, err := linuxwin.Open()
	if err != nil {
		t.Skip(err)
	}
	return p
}

func foreignHandle() platform.NativeHandle { return platform.Win32Handle{} }
