package input

// PointerDevice is what moves a pointer. See
// docs/ADR/0052-pinch-touch-and-pen.md.
type PointerDevice uint8

// The pointer devices. The zero value is a mouse or a touchpad.
const (
	DeviceMouse PointerDevice = iota
	// DeviceTouch is a finger on a touch screen.
	DeviceTouch
	// DevicePen is a stylus, which has a pressure.
	DevicePen
)
