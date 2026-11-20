package wayland

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/input"
)

// Values of xkbcommon.h and xkbcommon-compose.h.
const (
	xkbKeymapFormatTextV1   = 1
	xkbStateModsEffective   = 1<<0 | 1<<1 | 1<<2
	xkbComposeCompileNone   = 0
	xkbComposeStateNone     = 0
	xkbComposeFeedAccepted  = 1
	xkbComposeNothing       = 0
	xkbComposeComposing     = 1
	xkbComposeComposed      = 2
	xkbComposeCancelled     = 3
	xkbKeycodeOffset        = 8
	maxKeyText              = 64
	keymapSizeLimit         = 1 << 24
	xkbConsumedModeXkb      = 0
	xkbComposeTableNoFlags  = 0
	xkbComposeStateNoFlags  = 0
	xkbContextNoFlags       = 0
	xkbKeymapCompileNoFlags = 0
)

// xkbLib holds the functions of libxkbcommon that gowaku calls.
type xkbLib struct {
	contextNew, contextUnref, keymapNewFromString, keymapUnref                uintptr
	keymapKeyRepeats, keymapKeyGetSymsByLevel, stateNew, stateUnref           uintptr
	stateUpdateMask, stateKeyGetUTF8, stateKeyGetLayout, stateModNameIsActive uintptr
	stateKeyGetOneSym                                                         uintptr
	// The compose functions are optional: without them dead keys do nothing.
	composeTableNew, composeTableUnref, composeStateNew, composeStateUnref     uintptr
	composeStateFeed, composeStateGetStatus, composeStateGetUTF8, composeReset uintptr
}

var openXKB = sync.OnceValues(func() (uintptr, error) {
	lib, err := purego.Dlopen("libxkbcommon.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return 0, fmt.Errorf("wayland: load libxkbcommon.so.0: %w", err)
	}
	return lib, nil
})

func (x *xkbLib) load() error {
	lib, err := openXKB()
	if err != nil {
		return err
	}
	for _, s := range []struct {
		name string
		proc *uintptr
	}{
		{"xkb_context_new", &x.contextNew}, {"xkb_context_unref", &x.contextUnref},
		{"xkb_keymap_new_from_string", &x.keymapNewFromString}, {"xkb_keymap_unref", &x.keymapUnref},
		{"xkb_keymap_key_repeats", &x.keymapKeyRepeats}, {"xkb_keymap_key_get_syms_by_level", &x.keymapKeyGetSymsByLevel},
		{"xkb_state_new", &x.stateNew}, {"xkb_state_unref", &x.stateUnref},
		{"xkb_state_update_mask", &x.stateUpdateMask}, {"xkb_state_key_get_utf8", &x.stateKeyGetUTF8},
		{"xkb_state_key_get_layout", &x.stateKeyGetLayout}, {"xkb_state_mod_name_is_active", &x.stateModNameIsActive},
		{"xkb_state_key_get_one_sym", &x.stateKeyGetOneSym},
	} {
		if *s.proc, err = purego.Dlsym(lib, s.name); err != nil {
			return fmt.Errorf("wayland: find %s: %w", s.name, err)
		}
	}
	for _, s := range []struct {
		name string
		proc *uintptr
	}{
		{"xkb_compose_table_new_from_locale", &x.composeTableNew}, {"xkb_compose_table_unref", &x.composeTableUnref},
		{"xkb_compose_state_new", &x.composeStateNew}, {"xkb_compose_state_unref", &x.composeStateUnref},
		{"xkb_compose_state_feed", &x.composeStateFeed}, {"xkb_compose_state_get_status", &x.composeStateGetStatus},
		{"xkb_compose_state_get_utf8", &x.composeStateGetUTF8}, {"xkb_compose_state_reset", &x.composeReset},
	} {
		if *s.proc, err = purego.Dlsym(lib, s.name); err != nil {
			slog.Debug("wayland: no compose support", "missing", s.name)
			x.composeTableNew = 0
			break
		}
	}
	return nil
}

// keyboard turns the key codes of wl_keyboard into keys and text with the
// keymap that the compositor sends.
type keyboard struct {
	x       xkbLib
	context uintptr
	keymap  uintptr
	state   uintptr
	compose uintptr
	table   uintptr

	// modShift and the others are the names that xkb_state_mod_name_is_active
	// takes, as C strings.
	modShift, modControl, modAlt *byte
	text                         [maxKeyText]byte
	syms                         uintptr
	call                         [8]uintptr
}

func newKeyboard() (*keyboard, error) {
	k := &keyboard{
		modShift: cstr("Shift\x00"), modControl: cstr("Control\x00"), modAlt: cstr("Mod1\x00"),
	}
	if err := k.x.load(); err != nil {
		return nil, err
	}
	k.context = k.hot(k.x.contextNew, xkbContextNoFlags)
	if k.context == 0 {
		return nil, fmt.Errorf("wayland: xkb_context_new failed")
	}
	if k.x.composeTableNew != 0 {
		// The string is built here, so that nothing else keeps it alive
		// during the call.
		name := cstr(locale() + "\x00")
		k.table = k.hot(k.x.composeTableNew, k.context, uintptr(unsafe.Pointer(name)), xkbComposeCompileNone)
		runtime.KeepAlive(name)
		if k.table != 0 {
			k.compose = k.hot(k.x.composeStateNew, k.table, xkbComposeStateNoFlags)
		}
	}
	return k, nil
}

// locale returns the locale that compose sequences follow, as setlocale would.
func locale() string {
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return "C"
}

// hot calls fn through the buffer of k, which keeps the calls of each key
// from allocating. A uintptr does not keep its object alive or in place, so
// that pointers among args must lead to memory that outlives the call by
// itself: fields of k, static strings, or an object the caller keeps alive
// with runtime.KeepAlive.
func (k *keyboard) hot(fn uintptr, args ...uintptr) uintptr {
	n := copy(k.call[:], args)
	r, _, _ := purego.SyscallN(fn, k.call[:n]...)
	return r
}

// setKeymap reads the keymap that the compositor shares through fd.
func (k *keyboard) setKeymap(fd int32, size uint32) {
	defer unix.Close(int(fd)) //nolint:errcheck // nothing to do about it
	if size == 0 || size > keymapSizeLimit {
		return
	}
	data, err := unix.Mmap(int(fd), 0, int(size), unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		slog.Debug("wayland: map keymap", "err", err)
		return
	}
	defer unix.Munmap(data) //nolint:errcheck // nothing to do about it
	// The text ends in NUL, which xkb_keymap_new_from_string needs.
	keymap := k.hot(k.x.keymapNewFromString, k.context, uintptr(unsafe.Pointer(&data[0])), xkbKeymapFormatTextV1, xkbKeymapCompileNoFlags)
	if keymap == 0 {
		slog.Debug("wayland: compile keymap failed")
		return
	}
	state := k.hot(k.x.stateNew, keymap)
	if state == 0 {
		k.hot(k.x.keymapUnref, keymap)
		return
	}
	k.release()
	k.keymap, k.state = keymap, state
}

func (k *keyboard) release() {
	if k.state != 0 {
		k.hot(k.x.stateUnref, k.state)
		k.state = 0
	}
	if k.keymap != 0 {
		k.hot(k.x.keymapUnref, k.keymap)
		k.keymap = 0
	}
}

func (k *keyboard) destroy() {
	k.release()
	if k.compose != 0 {
		k.hot(k.x.composeStateUnref, k.compose)
	}
	if k.table != 0 {
		k.hot(k.x.composeTableUnref, k.table)
	}
	if k.context != 0 {
		k.hot(k.x.contextUnref, k.context)
	}
}

func (k *keyboard) updateMask(depressed, latched, locked, group uint32) {
	if k.state == 0 {
		return
	}
	k.hot(k.x.stateUpdateMask, k.state, uintptr(depressed), uintptr(latched), uintptr(locked), 0, 0, uintptr(group))
}

// modifiers returns the modifiers in effect.
func (k *keyboard) modifiers() input.Modifiers {
	if k.state == 0 {
		return 0
	}
	var m input.Modifiers
	if k.active(k.modShift) {
		m |= input.ModShift
	}
	if k.active(k.modControl) {
		m |= input.ModControl
	}
	if k.active(k.modAlt) {
		m |= input.ModAlt
	}
	return m
}

func (k *keyboard) active(name *byte) bool {
	return int32(k.hot(k.x.stateModNameIsActive, k.state, uintptr(unsafe.Pointer(name)), xkbStateModsEffective)) > 0
}

// plainSym returns the keysym of an evdev key code without modifiers, in the
// layout in effect, so that Ctrl+Shift+Z stays a Z.
func (k *keyboard) plainSym(key uint32) uint64 {
	if k.state == 0 {
		return 0
	}
	code := uintptr(key + xkbKeycodeOffset)
	layout := k.hot(k.x.stateKeyGetLayout, k.state, code)
	n := int32(k.hot(k.x.keymapKeyGetSymsByLevel, k.keymap, code, layout, 0, uintptr(unsafe.Pointer(&k.syms))))
	if n <= 0 || k.syms == 0 {
		return 0
	}
	return uint64(*(*uint32)(cptr(k.syms)))
}

// repeats reports whether the key repeats while held.
func (k *keyboard) repeats(key uint32) bool {
	return k.keymap != 0 && k.hot(k.x.keymapKeyRepeats, k.keymap, uintptr(key+xkbKeycodeOffset)) != 0
}

// keyText returns the text of a key press, through compose sequences. The result
// is only valid until the next call.
func (k *keyboard) keyText(key uint32) []byte {
	if k.state == 0 {
		return nil
	}
	code := uintptr(key + xkbKeycodeOffset)
	if k.compose != 0 {
		sym := k.hot(k.x.stateKeyGetOneSym, k.state, code)
		if int32(k.hot(k.x.composeStateFeed, k.compose, sym)) == xkbComposeFeedAccepted {
			switch k.hot(k.x.composeStateGetStatus, k.compose) {
			case xkbComposeComposing:
				return nil
			case xkbComposeComposed:
				n := int32(k.hot(k.x.composeStateGetUTF8, k.compose, uintptr(unsafe.Pointer(&k.text[0])), uintptr(len(k.text))))
				k.hot(k.x.composeReset, k.compose)
				return k.text[:min(int(max(n, 0)), len(k.text)-1)]
			case xkbComposeCancelled:
				k.hot(k.x.composeReset, k.compose)
				return nil
			}
		}
	}
	n := int32(k.hot(k.x.stateKeyGetUTF8, k.state, code, uintptr(unsafe.Pointer(&k.text[0])), uintptr(len(k.text))))
	return k.text[:min(int(max(n, 0)), len(k.text)-1)]
}
