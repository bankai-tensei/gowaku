package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/platform"
)

// secureFakeWindow is a fake window that can do secure input.
type secureFakeWindow struct {
	*fakeWindow
	secure []bool
}

func (w *secureFakeWindow) SetSecureInput(secure bool) { w.secure = append(w.secure, secure) }

var (
	_ core.SecureInputHost       = windowHost{}
	_ platform.SecureInputWindow = (*secureFakeWindow)(nil)
)

func TestWindowHost_SecureInput(t *testing.T) {
	native := &secureFakeWindow{fakeWindow: &fakeWindow{}}
	h := windowHost{w: &window{native: native}}
	h.SetSecureInput(true)
	h.SetSecureInput(false)
	if len(native.secure) != 2 || !native.secure[0] || native.secure[1] {
		t.Errorf("secure input must reach the window, got %v", native.secure)
	}
	// A window without it is left alone.
	windowHost{w: &window{native: &fakeWindow{}}}.SetSecureInput(true)
}
