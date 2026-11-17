package win32

import (
	"bytes"
	"image/png"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/clip"
)

// The clipboard beyond text: HTML Format, PNG and CF_DIB for images, CF_HDROP
// for files. See docs/ADR/0049-clipboard.md.

var (
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
)

// The standard clipboard formats that gowaku reads and writes beyond text.
const (
	CF_DIB   = 8
	CF_HDROP = 15
	CF_DIBV5 = 17
)

// formats are the registered clipboard formats, found on first use.
type formats struct {
	html, png uintptr
}

func (p *Platform) clipFormats() formats {
	if p.formats.html == 0 {
		register := func(name string) uintptr {
			s, err := syscall.UTF16PtrFromString(name)
			if err != nil {
				return 0
			}
			r, _, _ := syscall.SyscallN(procRegisterClipboardFormatW.Addr(), uintptr(unsafe.Pointer(s)))
			return r
		}
		p.formats = formats{html: register("HTML Format"), png: register("PNG")}
	}
	return p.formats
}

func available(format uintptr) bool {
	r, _, _ := syscall.SyscallN(procIsClipboardFormatAvailable.Addr(), format)
	return r != 0
}

// ClipboardKinds implements platform.Clipboard.
func (w *Window) ClipboardKinds() platform.ClipboardKinds {
	f := w.platform.clipFormats()
	var k platform.ClipboardKinds
	if available(CF_UNICODETEXT) {
		k |= platform.ClipboardText
	}
	if available(f.html) {
		k |= platform.ClipboardHTML
	}
	if available(f.png) || available(CF_DIB) || available(CF_DIBV5) {
		k |= platform.ClipboardImage
	}
	if available(CF_HDROP) {
		k |= platform.ClipboardFiles
	}
	return k
}

// clipboardBytes copies the data of format, or returns nil. The clipboard must
// be open.
func clipboardBytes(format uintptr) []byte {
	h, _, _ := syscall.SyscallN(procGetClipboardData.Addr(), format)
	if h == 0 {
		return nil
	}
	return globalCopy(h)
}

// globalCopy copies the bytes of a global memory block, or returns nil.
func globalCopy(h uintptr) []byte {
	p, _, _ := syscall.SyscallN(procGlobalLock.Addr(), h)
	if p == 0 {
		return nil
	}
	defer func() { _, _, _ = syscall.SyscallN(procGlobalUnlock.Addr(), h) }()
	size, _, _ := syscall.SyscallN(procGlobalSize.Addr(), h)
	// Reading through &p turns the integer into a pointer without a
	// uintptr-to-pointer conversion. The block stays valid until GlobalUnlock,
	// and is copied before.
	return bytes.Clone(unsafe.Slice(*(**byte)(unsafe.Pointer(&p)), int(size)))
}

// ReadClipboard implements platform.Clipboard.
func (w *Window) ReadClipboard(kinds platform.ClipboardKinds) platform.ClipboardContent {
	var c platform.ClipboardContent
	if kinds&platform.ClipboardText != 0 {
		c.Text = w.ClipboardText()
	}
	if kinds&^platform.ClipboardText == 0 {
		return c
	}
	f := w.platform.clipFormats()
	if r, _, _ := syscall.SyscallN(procOpenClipboard.Addr(), uintptr(w.hwnd)); r == 0 {
		return c
	}
	defer func() { _, _, _ = syscall.SyscallN(procCloseClipboard.Addr()) }()
	if kinds&platform.ClipboardHTML != 0 {
		if b := clipboardBytes(f.html); b != nil {
			c.HTML = clip.DecodeHTMLFormat(b)
		}
	}
	if kinds&platform.ClipboardImage != 0 {
		// PNG keeps the alpha that browsers and Office put there.
		if b := clipboardBytes(f.png); b != nil {
			c.Image = clip.DecodePNG(b)
		}
		if c.Image == nil {
			if b := clipboardBytes(CF_DIB); b != nil {
				c.Image = clip.DecodeDIB(b)
			}
		}
	}
	if kinds&platform.ClipboardFiles != 0 {
		if b := clipboardBytes(CF_HDROP); b != nil {
			c.Files = clip.DecodeDropFiles(b)
		}
	}
	return c
}

// globalBytes copies data into a movable global block for SetClipboardData,
// or returns zero.
func globalBytes(data []byte) uintptr {
	h, _, _ := syscall.SyscallN(procGlobalAlloc.Addr(), GMEM_MOVEABLE, uintptr(max(len(data), 1)))
	if h == 0 {
		return 0
	}
	p, _, _ := syscall.SyscallN(procGlobalLock.Addr(), h)
	if p == 0 {
		_, _, _ = syscall.SyscallN(procGlobalFree.Addr(), h)
		return 0
	}
	// See clipboardBytes for the conversion. The block is ours until it is
	// handed to the clipboard.
	copy(unsafe.Slice(*(**byte)(unsafe.Pointer(&p)), len(data)), data)
	_, _, _ = syscall.SyscallN(procGlobalUnlock.Addr(), h)
	return h
}

// put hands data to the clipboard as format, which must be open and emptied.
func put(format uintptr, data []byte) {
	h := globalBytes(data)
	if h == 0 {
		return
	}
	// On success the clipboard owns the block.
	if r, _, _ := syscall.SyscallN(procSetClipboardData.Addr(), format, h); r == 0 {
		_, _, _ = syscall.SyscallN(procGlobalFree.Addr(), h)
	}
}

// utf16Decode turns UTF-16 code units into runes.
func utf16Decode(units []uint16) []rune { return utf16.Decode(units) }

// utf16Bytes encodes s as UTF-16 with a terminating zero.
func utf16Bytes(s string) []byte {
	units := append(utf16.Encode([]rune(s)), 0)
	out := make([]byte, 2*len(units))
	for i, u := range units {
		out[2*i], out[2*i+1] = byte(u), byte(u>>8)
	}
	return out
}

// WriteClipboard implements platform.Clipboard.
func (w *Window) WriteClipboard(c platform.ClipboardContent) {
	f := w.platform.clipFormats()
	var pngData []byte
	if c.Image != nil {
		var b bytes.Buffer
		if png.Encode(&b, c.Image) == nil {
			pngData = b.Bytes()
		}
	}
	if r, _, _ := syscall.SyscallN(procOpenClipboard.Addr(), uintptr(w.hwnd)); r == 0 {
		return
	}
	defer func() { _, _, _ = syscall.SyscallN(procCloseClipboard.Addr()) }()
	_, _, _ = syscall.SyscallN(procEmptyClipboard.Addr())
	if c.Text != "" {
		put(CF_UNICODETEXT, utf16Bytes(c.Text))
	}
	if c.HTML != "" {
		put(f.html, clip.EncodeHTMLFormat(c.HTML))
	}
	if c.Image != nil {
		if pngData != nil {
			put(f.png, pngData)
		}
		// CF_DIB for the programs that know no PNG; Windows makes the other
		// bitmap formats from it.
		put(CF_DIB, clip.EncodeDIB(c.Image))
	}
	if len(c.Files) > 0 {
		put(CF_HDROP, clip.EncodeDropFiles(c.Files))
	}
}
