//go:build gpu

package win32

import (
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// These open a real window and look at its input context. The second one
// types into it through the IME of the system, so it runs with the GPU tests,
// locally only.

var (
	imm32t                       = windows.NewLazySystemDLL("imm32.dll")
	procImmSetOpenStatusT        = imm32t.NewProc("ImmSetOpenStatus")
	procImmSetConversionStatusT  = imm32t.NewProc("ImmSetConversionStatus")
	procSendInputT               = user32t.NewProc("SendInput")
	procSetForegroundWindowT     = user32t.NewProc("SetForegroundWindow")
	procGetForegroundWindowT     = user32t.NewProc("GetForegroundWindow")
	procAttachThreadInputT       = user32t.NewProc("AttachThreadInput")
	procGetWindowThreadProcessId = user32t.NewProc("GetWindowThreadProcessId")
	procSetFocusT                = user32t.NewProc("SetFocus")
)

const (
	IME_CMODE_NATIVE    = 0x1
	IME_CMODE_FULLSHAPE = 0x8
	INPUT_KEYBOARD      = 1
	KEYEVENTF_KEYUP     = 0x2
)

// keyInput is INPUT with a KEYBDINPUT in its union, 40 bytes on 64-bit Windows.
type keyInput struct {
	kind  uint32
	_     uint32
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	_     uint32
	extra uintptr
	_     [8]byte
}

// hasContext reports whether the window has an input context, as an IME sees it.
func hasContext(w *Window) bool {
	himc, _, _ := syscall.SyscallN(procImmGetContext.Addr(), uintptr(w.hwnd))
	if himc == 0 {
		return false
	}
	_, _, _ = syscall.SyscallN(procImmReleaseContext.Addr(), uintptr(w.hwnd), himc)
	return true
}

func TestSecureInput_TakesTheInputContext(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "secure", Width: 300, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	steps := []struct {
		name string
		do   func()
		want bool
	}{
		{"new window", func() {}, false},
		{"IME on", func() { w.SetIMEEnabled(true) }, true},
		{"secure on", func() { w.SetSecureInput(true) }, false},
		{"IME on while secure", func() { w.SetIMEEnabled(true) }, false},
		{"secure on again", func() { w.SetSecureInput(true) }, false},
		{"secure off", func() { w.SetSecureInput(false) }, true},
		{"IME off", func() { w.SetIMEEnabled(false) }, false},
		{"secure on and off with the IME off", func() { w.SetSecureInput(true); w.SetSecureInput(false) }, false},
	}
	for _, s := range steps {
		s.do()
		if got := hasContext(w); got != s.want {
			t.Errorf("%s: input context %v, want %v", s.name, got, s.want)
		}
	}
}

func TestSecureInput_JapaneseIMEDoesNotCompose(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "secure typing", Width: 300, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	hwnd := uintptr(w.hwnd)
	// A window of a process without the foreground may not take it alone:
	// join the input of the foreground thread for a moment.
	fg, _, _ := procGetForegroundWindowT.Call()
	fgThread, _, _ := procGetWindowThreadProcessId.Call(fg, 0)
	self := uintptr(windows.GetCurrentThreadId())
	if fgThread != 0 && fgThread != self {
		procAttachThreadInputT.Call(self, fgThread, 1)
	}
	procSetWindowPosT.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x40) // HWND_TOPMOST
	procSetForegroundWindowT.Call(hwnd)
	procSetFocusT.Call(hwnd)
	if fgThread != 0 && fgThread != self {
		procAttachThreadInputT.Call(self, fgThread, 0)
	}
	defer procSetWindowPosT.Call(hwnd, ^uintptr(1), 0, 0, 0, 0, 0x1|0x2) // HWND_NOTOPMOST
	drain := func() (text string, compositions int) {
		deadline := time.Now().Add(700 * time.Millisecond)
		for time.Now().Before(deadline) {
			e := p.WaitEvent(50 * time.Millisecond)
			switch e.Kind {
			case platform.EventText:
				text += e.Text
			case platform.EventComposition:
				if e.Composition != nil {
					compositions++
				}
			}
		}
		return text, compositions
	}
	drain()
	if now, _, _ := procGetForegroundWindowT.Call(); now != hwnd {
		t.Skip("the window did not get the foreground, so the keys would go elsewhere")
	}
	press := func(keys string) {
		in := make([]keyInput, 0, 2*len(keys))
		for _, k := range keys {
			in = append(in, keyInput{kind: INPUT_KEYBOARD, vk: uint16(k)}, keyInput{kind: INPUT_KEYBOARD, vk: uint16(k), flags: KEYEVENTF_KEYUP})
		}
		if n, _, err := procSendInputT.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0])); int(n) != len(in) {
			t.Fatalf("SendInput: %v", err)
		}
	}
	openIME := func() bool {
		himc, _, _ := syscall.SyscallN(procImmGetContext.Addr(), hwnd)
		if himc == 0 {
			return false
		}
		procImmSetOpenStatusT.Call(himc, 1)
		procImmSetConversionStatusT.Call(himc, IME_CMODE_NATIVE|IME_CMODE_FULLSHAPE, 0)
		_, _, _ = syscall.SyscallN(procImmReleaseContext.Addr(), hwnd, himc)
		return true
	}

	// With the IME on in hiragana, KA composes か.
	w.SetIMEEnabled(true)
	if !openIME() {
		t.Fatal("no input context with the IME on")
	}
	press("KA")
	text, compositions := drain()
	w.CancelComposition()
	drain()
	t.Logf("IME on: text %q, %d compositions", text, compositions)
	if compositions == 0 {
		t.Skip("the IME did not compose here, so its absence below would prove nothing")
	}

	// With secure input, the same keys arrive as plain letters.
	w.SetSecureInput(true)
	w.SetIMEEnabled(true) // as a field that is not a password would ask
	openIME()
	press("KA")
	text, compositions = drain()
	t.Logf("secure: text %q, %d compositions", text, compositions)
	if compositions != 0 {
		t.Errorf("the IME composed %d times under secure input", compositions)
	}
	if text != "ka" {
		t.Errorf("typed %q, want %q", text, "ka")
	}
	w.SetSecureInput(false)
}
