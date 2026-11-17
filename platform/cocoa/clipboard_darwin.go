//go:build darwin && arm64

package cocoa

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
)

// The pasteboard types, by their uniform type identifiers.
const (
	typeString = "public.utf8-plain-text"
	typeHTML   = "public.html"
	typePNG    = "public.png"
)

func generalPasteboard() objc.ID {
	return class("NSPasteboard").Send(sel("generalPasteboard"))
}

// ClipboardText implements platform.Window.
func (w *Window) ClipboardText() string {
	var s string
	w.p.libs.pool(func() {
		s = goString(generalPasteboard().Send(sel("stringForType:"), nsString(typeString)))
	})
	return s
}

// SetClipboardText implements platform.Window.
func (w *Window) SetClipboardText(s string) {
	w.WriteClipboard(platform.ClipboardContent{Text: s})
}

// ClipboardKinds implements platform.Clipboard.
func (w *Window) ClipboardKinds() platform.ClipboardKinds {
	var kinds platform.ClipboardKinds
	w.p.libs.pool(func() {
		types := generalPasteboard().Send(sel("types"))
		for i := range count(types) {
			switch goString(at(types, i)) {
			case typeString:
				kinds |= platform.ClipboardText
			case typeHTML:
				kinds |= platform.ClipboardHTML
			case typePNG:
				kinds |= platform.ClipboardImage
			case typeFileURL:
				kinds |= platform.ClipboardFiles
			}
		}
	})
	return kinds
}

// ReadClipboard implements platform.Clipboard.
func (w *Window) ReadClipboard(kinds platform.ClipboardKinds) platform.ClipboardContent {
	var c platform.ClipboardContent
	w.p.libs.pool(func() {
		board := generalPasteboard()
		if kinds&platform.ClipboardText != 0 {
			c.Text = goString(board.Send(sel("stringForType:"), nsString(typeString)))
		}
		if kinds&platform.ClipboardHTML != 0 {
			c.HTML = goString(board.Send(sel("stringForType:"), nsString(typeHTML)))
		}
		if kinds&platform.ClipboardImage != 0 {
			c.Image = decodePNG(dataBytes(board.Send(sel("dataForType:"), nsString(typePNG))))
		}
		if kinds&platform.ClipboardFiles != 0 {
			c.Files = readFiles(board)
		}
	})
	return c
}

// WriteClipboard implements platform.Clipboard.
func (w *Window) WriteClipboard(c platform.ClipboardContent) {
	w.p.libs.pool(func() {
		board := generalPasteboard()
		board.Send(sel("clearContents"))
		if c.Text != "" {
			board.Send(sel("setString:forType:"), nsString(c.Text), nsString(typeString))
		}
		if c.HTML != "" {
			board.Send(sel("setString:forType:"), nsString(c.HTML), nsString(typeHTML))
		}
		if c.Image != nil {
			var buf bytes.Buffer
			if err := png.Encode(&buf, c.Image); err == nil {
				board.Send(sel("setData:forType:"), nsData(buf.Bytes()), nsString(typePNG))
			}
		}
		if len(c.Files) > 0 {
			// One item per file, each a file URL.
			urls := make([]objc.ID, len(c.Files))
			for i, f := range c.Files {
				urls[i] = class("NSURL").Send(sel("fileURLWithPath:"), nsString(f))
			}
			board.Send(sel("writeObjects:"), nsArray(urls...))
		}
	})
}

// readFiles returns the paths of the file URLs of a pasteboard, one per item.
func readFiles(board objc.ID) []string {
	var files []string
	urls := board.Send(sel("readObjectsForClasses:options:"), nsArray(class("NSURL")), objc.ID(0))
	for i := range count(urls) {
		u := at(urls, i)
		if !objc.Send[bool](u, sel("isFileURL")) {
			continue
		}
		// A file reference URL (file:///.file/id=...) resolves to its path.
		if path := u.Send(sel("filePathURL")); path != 0 {
			files = append(files, goString(path.Send(sel("path"))))
		}
	}
	return files
}

// dataBytes copies the bytes of an NSData.
func dataBytes(data objc.ID) []byte {
	if data == 0 {
		return nil
	}
	n := int(objc.Send[uint64](data, sel("length")))
	if n == 0 {
		return nil
	}
	p := objc.Send[*byte](data, sel("bytes"))
	// bytes points to n bytes that the NSData keeps.
	return bytes.Clone(unsafe.Slice(p, n))
}

// nsData returns an autoreleased NSData with a copy of b.
func nsData(b []byte) objc.ID {
	if len(b) == 0 {
		return class("NSData").Send(sel("data"))
	}
	// dataWithBytes copies the bytes before it returns.
	return class("NSData").Send(sel("dataWithBytes:length:"), unsafe.Pointer(&b[0]), uint64(len(b)))
}

func decodePNG(b []byte) *image.NRGBA {
	if len(b) == 0 {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}
