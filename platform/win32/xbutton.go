package win32

import "github.com/bankai-tensei/gowaku/input"

// The buttons of GET_XBUTTON_WPARAM. They keep their header names
// (CODING_STYLE §8).
const (
	XBUTTON1 = 0x0001
	XBUTTON2 = 0x0002
)

// xbuttonOf maps the wParam of WM_XBUTTONDOWN and WM_XBUTTONUP, whose high word
// names the thumb button: XBUTTON1 goes back and XBUTTON2 forward. ok is false
// for anything else.
func xbuttonOf(wParam uintptr) (b input.Button, ok bool) {
	switch wParam >> 16 & 0xFFFF {
	case XBUTTON1:
		return input.ButtonBack, true
	case XBUTTON2:
		return input.ButtonForward, true
	}
	return 0, false
}
