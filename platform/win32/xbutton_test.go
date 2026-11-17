package win32

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestXButtonOf(t *testing.T) {
	const MK_XBUTTON1, MK_LBUTTON = 0x0020, 0x0001
	for _, tt := range []struct {
		wParam uintptr
		want   input.Button
		ok     bool
	}{
		{XBUTTON1 << 16, input.ButtonBack, true},
		{XBUTTON2 << 16, input.ButtonForward, true},
		// The low word holds the keys and buttons that are down.
		{XBUTTON1<<16 | MK_XBUTTON1 | MK_LBUTTON, input.ButtonBack, true},
		{0, 0, false},
		{3 << 16, 0, false},
	} {
		got, ok := xbuttonOf(tt.wParam)
		if got != tt.want || ok != tt.ok {
			t.Errorf("xbuttonOf(%#x) = %v, %v; want %v, %v", tt.wParam, got, ok, tt.want, tt.ok)
		}
	}
}
