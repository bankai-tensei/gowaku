package win32

import (
	"syscall"
	"testing"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// usLayout and jisLayout are what MapVirtualKey returns for the OEM keys on
// kbdus.dll and kbd106.dll, the US and the Japanese (106/109) layouts.
var (
	usLayout = map[uintptr]rune{
		VK_OEM_1: ';', VK_OEM_PLUS: '=', VK_OEM_COMMA: ',', VK_OEM_MINUS: '-', VK_OEM_PERIOD: '.',
		VK_OEM_2: '/', VK_OEM_3: '`', VK_OEM_4: '[', VK_OEM_5: '\\', VK_OEM_6: ']', VK_OEM_7: '\'',
	}
	jisLayout = map[uintptr]rune{
		VK_OEM_1: ':', VK_OEM_PLUS: ';', VK_OEM_COMMA: ',', VK_OEM_MINUS: '-', VK_OEM_PERIOD: '.',
		VK_OEM_2: '/', VK_OEM_3: '@', VK_OEM_4: '[', VK_OEM_5: '\\', VK_OEM_6: ']', VK_OEM_7: '^',
		VK_OEM_102: '\\',
	}
)

func TestKeyCodeOf_PunctuationFollowsLayout(t *testing.T) {
	for _, c := range []struct {
		name   string
		layout map[uintptr]rune
		vk     uintptr
		want   input.KeyCode
	}{
		{"US の \\", usLayout, VK_OEM_5, input.KeyBackslash},
		{"US の [", usLayout, VK_OEM_4, input.KeyBracketLeft},
		{"US の ]", usLayout, VK_OEM_6, input.KeyBracketRight},
		{"US の ;", usLayout, VK_OEM_1, input.KeySemicolon},
		{"US の '", usLayout, VK_OEM_7, input.KeyQuote},
		{"US の `", usLayout, VK_OEM_3, input.KeyBackquote},
		{"US の =", usLayout, VK_OEM_PLUS, input.KeyEqual},
		{"US の ,", usLayout, VK_OEM_COMMA, input.KeyComma},
		{"JIS の @", jisLayout, VK_OEM_3, input.KeyAt},
		{"JIS の [", jisLayout, VK_OEM_4, input.KeyBracketLeft},
		{"JIS の ]", jisLayout, VK_OEM_6, input.KeyBracketRight},
		{"JIS の :", jisLayout, VK_OEM_1, input.KeyColon},
		{"JIS の ;（+ のキー）", jisLayout, VK_OEM_PLUS, input.KeySemicolon},
		{"JIS の ^", jisLayout, VK_OEM_7, input.KeyCaret},
		{"JIS の ¥ は \\", jisLayout, VK_OEM_5, input.KeyBackslash},
		{"JIS の ろ は \\", jisLayout, VK_OEM_102, input.KeyBackslash},
		{"JIS の -", jisLayout, VK_OEM_MINUS, input.KeyMinus},
		{"文字の無いキー", usLayout, VK_OEM_102, input.KeyNone},
		{"テンキーの +", usLayout, VK_ADD, input.KeyPlus},
		{"テンキーの *", jisLayout, VK_MULTIPLY, input.KeyAsterisk},
		{"文字は配列に依らない", jisLayout, 'S', input.KeyS},
		{"F5", jisLayout, VK_F1 + 4, input.KeyF5},
	} {
		t.Run(c.name, func(t *testing.T) {
			typed := func(vk uintptr) rune { return c.layout[vk] }
			if got := keyCodeOf(c.vk, typed); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestKeyCodeOf_NonASCII covers a German layout, whose OEM keys type umlauts and
// a dead ^.
func TestKeyCodeOf_NonASCII(t *testing.T) {
	german := map[uintptr]rune{VK_OEM_1: 'ü', VK_OEM_5: '^', VK_OEM_PLUS: '+', VK_OEM_2: '#', VK_OEM_102: '<'}
	typed := func(vk uintptr) rune { return german[vk] }
	for vk, want := range map[uintptr]input.KeyCode{
		VK_OEM_1: input.KeyNone, VK_OEM_5: input.KeyCaret, VK_OEM_PLUS: input.KeyPlus,
		VK_OEM_2: input.KeyHash, VK_OEM_102: input.KeyLess,
	} {
		if got := keyCodeOf(vk, typed); got != want {
			t.Errorf("vk %#x: got %q, want %q", vk, got, want)
		}
	}
}

// TestKeyCodeOf_InstalledLayouts asks the layouts of this machine for US and
// Japanese, if any are installed, and checks them against the tables above. A
// Japanese IME on US hardware uses the US layout.
func TestKeyCodeOf_InstalledLayouts(t *testing.T) {
	user32 := syscall.NewLazyDLL("user32.dll")
	list := user32.NewProc("GetKeyboardLayoutList")
	mapEx := user32.NewProc("MapVirtualKeyExW")
	n, _, _ := syscall.SyscallN(list.Addr(), 0, 0)
	if n == 0 {
		t.Skip("no keyboard layouts")
	}
	layouts := make([]uintptr, n)
	n, _, _ = syscall.SyscallN(list.Addr(), n, uintptr(unsafe.Pointer(&layouts[0])))
	checked := 0
	for _, hkl := range layouts[:n] {
		lang := hkl & 0xFFFF
		if lang != 0x0409 && lang != 0x0411 {
			continue
		}
		typed := func(vk uintptr) rune {
			r, _, _ := syscall.SyscallN(mapEx.Addr(), vk, MAPVK_VK_TO_CHAR, hkl)
			return rune(r & 0x7FFFFFFF)
		}
		want := usLayout
		if typed(VK_OEM_3) == '@' {
			want = jisLayout
		}
		for vk, r := range want {
			if got := typed(vk); got != r {
				t.Errorf("layout %#x, vk %#x: types %q, want %q", hkl, vk, got, r)
			}
		}
		for _, vk := range []uintptr{VK_OEM_5} {
			if got := keyCodeOf(vk, typed); got != input.KeyBackslash {
				t.Errorf("layout %#x: Ctrl+\\ is %q", hkl, got)
			}
		}
		at := keyCodeOf(VK_OEM_3, typed)
		if want := map[bool]input.KeyCode{true: input.KeyAt, false: input.KeyBackquote}[typed(VK_OEM_3) == '@']; at != want {
			t.Errorf("layout %#x: VK_OEM_3 is %q, want %q", hkl, at, want)
		}
		t.Logf("layout %#x: VK_OEM_3 is %q", hkl, at)
		checked++
	}
	if checked == 0 {
		t.Skip("no US or Japanese layout installed")
	}
}

// TestKeyEvent_Backslash sends a WM_KEYDOWN of VK_OEM_5 through the translation
// with the layout of this thread, where it types a backslash on both US and
// Japanese layouts.
func TestKeyEvent_Backslash(t *testing.T) {
	if r := layoutChar(VK_OEM_5); r != '\\' {
		t.Skipf("the layout of this machine types %q on VK_OEM_5", r)
	}
	p := &Platform{}
	w := &Window{id: 1}
	p.keyEvent(w, platform.EventKeyDown, VK_OEM_5, 0)
	ev, ok := p.queue.Pop()
	if !ok || ev.Kind != platform.EventKeyDown || ev.Key != input.KeyBackslash {
		t.Fatalf("got %+v, %v; want a key down of KeyBackslash", ev, ok)
	}
}
