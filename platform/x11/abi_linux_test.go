package x11

import (
	"testing"
	"unsafe"
)

// The numbers come from a C program built against Xlib.h (CODING_STYLE §8).
func TestABI_Layouts(t *testing.T) {
	var key XKeyEvent
	var crossing XCrossingEvent
	var focus XFocusChangeEvent
	var configure XConfigureEvent
	var client XClientMessageEvent
	var property XPropertyEvent
	var request XSelectionRequestEvent
	var selection XSelectionEvent
	var clearing XSelectionClearEvent
	var attrs XSetWindowAttributes
	var cookie XGenericEventCookie
	var pinch XIGesturePinchEvent
	tests := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"XEvent", unsafe.Sizeof(XEvent{}), 192},
		{"XGenericEventCookie", unsafe.Sizeof(cookie), 56},
		{"XGenericEventCookie.data", unsafe.Offsetof(cookie.Data), 48},
		{"XIEventMask", unsafe.Sizeof(XIEventMask{}), 16},
		{"XIGesturePinchEvent", unsafe.Sizeof(pinch), 208},
		{"XIGesturePinchEvent.event", unsafe.Offsetof(pinch.Event), 72},
		{"XIGesturePinchEvent.event_x", unsafe.Offsetof(pinch.EventX), 104},
		{"XIGesturePinchEvent.scale", unsafe.Offsetof(pinch.Scale), 152},
		{"XIGesturePinchEvent.mods", unsafe.Offsetof(pinch.Mods), 172},
		{"XSetWindowAttributes", unsafe.Sizeof(attrs), 112},
		{"XVisualInfo", unsafe.Sizeof(XVisualInfo{}), 64},
		{"XVisualInfo.red_mask", unsafe.Offsetof(XVisualInfo{}.RedMask), 32},
		{"XKeyEvent", unsafe.Sizeof(key), 96},
		{"XCrossingEvent", unsafe.Sizeof(crossing), 104},
		{"XFocusChangeEvent", unsafe.Sizeof(focus), 48},
		{"XConfigureEvent", unsafe.Sizeof(configure), 88},
		{"XClientMessageEvent", unsafe.Sizeof(client), 96},
		{"XPropertyEvent", unsafe.Sizeof(property), 64},
		{"XSelectionRequestEvent", unsafe.Sizeof(request), 80},
		{"XSelectionEvent", unsafe.Sizeof(selection), 72},
		{"XSelectionClearEvent", unsafe.Sizeof(clearing), 56},
		{"XKeyEvent.window", unsafe.Offsetof(key.Window), 32},
		{"XKeyEvent.time", unsafe.Offsetof(key.Time), 56},
		{"XKeyEvent.x", unsafe.Offsetof(key.X), 64},
		{"XKeyEvent.y", unsafe.Offsetof(key.Y), 68},
		{"XKeyEvent.state", unsafe.Offsetof(key.State), 80},
		{"XKeyEvent.keycode", unsafe.Offsetof(key.Keycode), 84},
		{"XCrossingEvent.mode", unsafe.Offsetof(crossing.Mode), 80},
		{"XCrossingEvent.detail", unsafe.Offsetof(crossing.Detail), 84},
		{"XFocusChangeEvent.mode", unsafe.Offsetof(focus.Mode), 40},
		{"XFocusChangeEvent.detail", unsafe.Offsetof(focus.Detail), 44},
		{"XConfigureEvent.window", unsafe.Offsetof(configure.Target), 40},
		{"XConfigureEvent.width", unsafe.Offsetof(configure.Width), 56},
		{"XConfigureEvent.height", unsafe.Offsetof(configure.Height), 60},
		{"XClientMessageEvent.message_type", unsafe.Offsetof(client.MessageType), 40},
		{"XClientMessageEvent.format", unsafe.Offsetof(client.Format), 48},
		{"XClientMessageEvent.data", unsafe.Offsetof(client.Data), 56},
		{"XPropertyEvent.atom", unsafe.Offsetof(property.Atom), 40},
		{"XSelectionRequestEvent.requestor", unsafe.Offsetof(request.Requestor), 40},
		{"XSelectionRequestEvent.selection", unsafe.Offsetof(request.Selection), 48},
		{"XSelectionRequestEvent.target", unsafe.Offsetof(request.Target), 56},
		{"XSelectionRequestEvent.property", unsafe.Offsetof(request.Property), 64},
		{"XSelectionRequestEvent.time", unsafe.Offsetof(request.Time), 72},
		{"XSelectionEvent.selection", unsafe.Offsetof(selection.Selection), 40},
		{"XSelectionEvent.target", unsafe.Offsetof(selection.Target), 48},
		{"XSelectionEvent.property", unsafe.Offsetof(selection.Property), 56},
		{"XSelectionEvent.time", unsafe.Offsetof(selection.Time), 64},
		{"XSelectionClearEvent.selection", unsafe.Offsetof(clearing.Selection), 40},
		{"XSetWindowAttributes.bit_gravity", unsafe.Offsetof(attrs.BitGravity), 32},
		{"XSetWindowAttributes.event_mask", unsafe.Offsetof(attrs.EventMask), 72},
		{"XSetWindowAttributes.colormap", unsafe.Offsetof(attrs.Colormap), 96},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: Go %d, C %d", tt.name, tt.got, tt.want)
		}
	}
}
