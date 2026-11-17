package wayland

import (
	"log/slog"
	"math"
	"os"
	"runtime"
	"strconv"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The constants of logic.go must match the generated ones.
var _ = [1]int{}[shapeEResize-wpCursorShapeDeviceV1ShapeEResize+shapeSWResize-wpCursorShapeDeviceV1ShapeSwResize+
	shapeText-wpCursorShapeDeviceV1ShapeText+shapePointer-wpCursorShapeDeviceV1ShapePointer+
	shapeColResize-wpCursorShapeDeviceV1ShapeColResize+shapeRowResize-wpCursorShapeDeviceV1ShapeRowResize]

// defaultCursorSize is the size of cursors in logical pixels when
// $XCURSOR_SIZE does not say.
const defaultCursorSize = 24

// cursorLib holds the functions of libwayland-cursor.
type cursorLib struct {
	themeLoad, themeDestroy, themeGetCursor, imageGetBuffer uintptr
}

var openCursorLib = sync.OnceValues(func() (cursorLib, error) {
	var l cursorLib
	lib, err := purego.Dlopen("libwayland-cursor.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return l, err
	}
	for _, s := range []struct {
		name string
		proc *uintptr
	}{
		{"wl_cursor_theme_load", &l.themeLoad}, {"wl_cursor_theme_destroy", &l.themeDestroy},
		{"wl_cursor_theme_get_cursor", &l.themeGetCursor}, {"wl_cursor_image_get_buffer", &l.imageGetBuffer},
	} {
		if *s.proc, err = purego.Dlsym(lib, s.name); err != nil {
			return l, err
		}
	}
	return l, nil
})

// wlCursor is struct wl_cursor, and wlCursorImage struct wl_cursor_image.
type (
	wlCursor struct {
		imageCount uint32
		images     uintptr
		name       uintptr
	}
	wlCursorImage struct {
		width, height, hotspotX, hotspotY, delay uint32
	}
)

// cursorState sets the cursor over the windows of a seat.
type cursorState struct {
	p      *Platform
	device wpCursorShapeDeviceV1

	// What the fallback without cursor-shape-v1 needs: a theme loaded at a
	// whole scale, and a surface that shows the image.
	lib        cursorLib
	theme      uintptr
	themeScale int32
	surface    wlSurface

	shape  cursorShape
	serial uint32
	name   [64]byte
	call   [8]uintptr
}

func (cs *cursorState) init(p *Platform) {
	cs.p = p
	s := &p.seat
	if p.cursorShapes != 0 {
		cs.device = p.cursorShapes.getPointer(p.c, s.pointer)
		return
	}
	lib, err := openCursorLib()
	if err != nil || p.shm == 0 {
		slog.Debug("wayland: no cursor theme", "err", err)
		return
	}
	cs.lib = lib
	cs.surface = p.compositor.createSurface(p.c)
}

func (cs *cursorState) release() {
	if cs.p == nil {
		return
	}
	c := cs.p.c
	if cs.device != 0 {
		cs.device.destroy(c)
	}
	if cs.surface != 0 {
		cs.surface.destroy(c)
	}
	if cs.theme != 0 {
		cs.hot(cs.lib.themeDestroy, cs.theme)
	}
	*cs = cursorState{}
}

func (cs *cursorState) hot(fn uintptr, args ...uintptr) uintptr {
	n := copy(cs.call[:], args)
	r, _, _ := purego.SyscallN(fn, cs.call[:n]...)
	return r
}

// set shows shape over the window under the pointer. A new enter of the pointer
// needs it again, even for the same shape.
func (cs *cursorState) set(shape cursorShape) {
	if cs.p == nil {
		return
	}
	s := &cs.p.seat
	if s.pointer == 0 || (shape == cs.shape && s.enterSerial == cs.serial) {
		return
	}
	cs.shape, cs.serial = shape, s.enterSerial
	if cs.device != 0 {
		cs.device.setShape(cs.p.c, s.enterSerial, uint32(shape))
		return
	}
	if cs.surface != 0 {
		cs.setImage(shape)
	}
}

// setImage shows a cursor of the theme.
func (cs *cursorState) setImage(shape cursorShape) {
	c, s := cs.p.c, &cs.p.seat
	scale := int32(1)
	if w := s.pointerFocus; w != nil {
		scale = int32(math.Ceil(w.scale))
	}
	if cs.theme == 0 || scale != cs.themeScale {
		if cs.theme != 0 {
			cs.hot(cs.lib.themeDestroy, cs.theme)
		}
		cs.themeScale = scale
		cs.theme = cs.loadTheme(scale)
		if cs.theme == 0 {
			return
		}
	}
	cursor := cs.find(cursorName(shape))
	if cursor == 0 {
		cursor = cs.find(legacyCursorName(shape))
	}
	if cursor == 0 {
		return
	}
	cur := (*wlCursor)(cptr(cursor))
	if cur.imageCount == 0 {
		return
	}
	imagePtr := *(*uintptr)(cptr(cur.images))
	image := (*wlCursorImage)(cptr(imagePtr))
	buffer := cs.hot(cs.lib.imageGetBuffer, imagePtr)
	if buffer == 0 {
		return
	}
	s.pointer.setCursor(c, s.enterSerial, cs.surface, int32(image.hotspotX)/scale, int32(image.hotspotY)/scale)
	cs.surface.attach(c, wlBuffer(buffer), 0, 0)
	if cs.p.compositorVersion >= wlSurfaceSetBufferScaleSince {
		cs.surface.setBufferScale(c, scale)
	}
	cs.surface.damage(c, 0, 0, int32(image.width), int32(image.height))
	cs.surface.commit(c)
}

func (cs *cursorState) loadTheme(scale int32) uintptr {
	size := defaultCursorSize
	if v, err := strconv.Atoi(os.Getenv("XCURSOR_SIZE")); err == nil && v > 0 {
		size = v
	}
	var name uintptr
	var keep []byte
	if v := os.Getenv("XCURSOR_THEME"); v != "" {
		keep = append([]byte(v), 0)
		name = uintptr(unsafe.Pointer(&keep[0]))
	}
	theme := cs.hot(cs.lib.themeLoad, name, uintptr(size*int(scale)), uintptr(cs.p.shm))
	runtime.KeepAlive(keep)
	if theme == 0 {
		slog.Debug("wayland: load cursor theme failed")
	}
	return theme
}

func (cs *cursorState) find(name string) uintptr {
	n := copy(cs.name[:len(cs.name)-1], name)
	cs.name[n] = 0
	return cs.hot(cs.lib.themeGetCursor, cs.theme, uintptr(unsafe.Pointer(&cs.name[0])))
}
