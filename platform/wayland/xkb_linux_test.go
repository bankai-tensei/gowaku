package wayland

import (
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/keysym"
)

type xkbRuleNames struct {
	rules, model, layout, variant, options *byte
}

func TestKeyboardKeySym_UsesNumLockAndShift(t *testing.T) {
	k, err := newKeyboard()
	if err != nil {
		t.Skipf("xkbcommon is unavailable: %v", err)
	}
	t.Cleanup(k.destroy)

	lib, err := openXKB()
	if err != nil {
		t.Skipf("xkbcommon is unavailable: %v", err)
	}
	var keymapNewFromNames, keymapKeyByName, keymapModGetIndex uintptr
	for _, s := range []struct {
		name string
		proc *uintptr
	}{
		{"xkb_keymap_new_from_names", &keymapNewFromNames},
		{"xkb_keymap_key_by_name", &keymapKeyByName},
		{"xkb_keymap_mod_get_index", &keymapModGetIndex},
	} {
		if *s.proc, err = purego.Dlsym(lib, s.name); err != nil {
			t.Fatalf("find %s: %v", s.name, err)
		}
	}

	names := xkbRuleNames{rules: cstr("evdev\x00"), model: cstr("pc105\x00"), layout: cstr("us\x00")}
	keymap := k.hot(keymapNewFromNames, k.context, uintptr(unsafe.Pointer(&names)), xkbKeymapCompileNoFlags)
	runtime.KeepAlive(names)
	if keymap == 0 {
		t.Fatal("compile the US XKB keymap")
	}
	state := k.hot(k.x.stateNew, keymap)
	if state == 0 {
		k.hot(k.x.keymapUnref, keymap)
		t.Fatal("create XKB state")
	}
	k.release()
	k.keymap, k.state = keymap, state

	keycode := func(name string) uint32 {
		t.Helper()
		nameWithNUL := name + "\x00"
		keyName := cstr(nameWithNUL)
		code := uint32(k.hot(keymapKeyByName, keymap, uintptr(unsafe.Pointer(keyName))))
		runtime.KeepAlive(nameWithNUL)
		if code < xkbKeycodeOffset || code == ^uint32(0) {
			t.Fatalf("no XKB key named %q", name)
		}
		return code - xkbKeycodeOffset
	}
	modifier := func(name string) uint32 {
		t.Helper()
		nameWithNUL := name + "\x00"
		modName := cstr(nameWithNUL)
		index := uint32(k.hot(keymapModGetIndex, keymap, uintptr(unsafe.Pointer(modName))))
		runtime.KeepAlive(nameWithNUL)
		if index >= 32 {
			t.Fatalf("no XKB modifier named %q", name)
		}
		return index
	}

	kp4, kpDecimal, q := keycode("KP4"), keycode("KPDL"), keycode("AD01")
	shiftMask, numLockMask := uint32(1)<<modifier("Shift"), uint32(1)<<modifier("NumLock")
	for _, tt := range []struct {
		name      string
		depressed uint32
		locked    uint32
		key       uint32
		want      uint64
	}{
		{"Num Lock off", 0, 0, kp4, keysym.XK_KP_Left},
		{"Num Lock on", 0, numLockMask, kp4, keysym.XK_KP_0 + 4},
		{"Shift inverts Num Lock", shiftMask, numLockMask, kp4, keysym.XK_KP_Left},
		{"Shift without Num Lock", shiftMask, 0, kp4, keysym.XK_KP_Left},
		{"keypad decimal", 0, numLockMask, kpDecimal, keysym.XK_KP_Decimal},
		{"ordinary key ignores Shift", shiftMask, 0, q, 'q'},
	} {
		t.Run(tt.name, func(t *testing.T) {
			k.updateMask(tt.depressed, 0, tt.locked, 0)
			if got := k.keySym(tt.key); got != tt.want {
				t.Errorf("keySym(%d) = %#x, want %#x", tt.key, got, tt.want)
			}
		})
	}

	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	oldTimeNow := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = oldTimeNow })
	k.updateMask(0, 0, numLockMask, 0)
	p := &Platform{}
	s := &seat{
		p: p, kb: k, keyboardFocus: &Window{id: 1},
		repeatRate: 10, repeatDelay: 50,
	}
	s.wlKeyboardKey(1, 100, kp4, keyPressed)
	s.repeatDue(now.Add(50 * time.Millisecond))
	s.wlKeyboardKey(1, 200, kp4, 0)

	var events []platform.Event
	for {
		ev, ok := p.queue.Pop()
		if !ok {
			break
		}
		events = append(events, ev)
	}
	if len(events) != 5 {
		t.Fatalf("got %d key events, want key-down, text, repeat key-down, text, key-up: %+v", len(events), events)
	}
	if events[0].Kind != platform.EventKeyDown || events[0].Key != input.Key4 || events[0].Repeat {
		t.Errorf("initial key event = %+v, want non-repeat key 4 down", events[0])
	}
	if events[1].Kind != platform.EventText || events[1].Text != "4" {
		t.Errorf("initial text event = %+v, want text 4", events[1])
	}
	if events[2].Kind != platform.EventKeyDown || events[2].Key != input.Key4 || !events[2].Repeat {
		t.Errorf("repeat key event = %+v, want repeat key 4 down", events[2])
	}
	if events[3].Kind != platform.EventText || events[3].Text != "4" {
		t.Errorf("repeat text event = %+v, want text 4", events[3])
	}
	if events[4].Kind != platform.EventKeyUp || events[4].Key != input.Key4 {
		t.Errorf("release event = %+v, want key 4 up", events[4])
	}
}
