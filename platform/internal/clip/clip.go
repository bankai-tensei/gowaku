// Package clip converts clipboard data between platform.ClipboardContent and
// the formats of the OS: the MIME types of X11 and Wayland, and the HTML Format
// and device-independent bitmaps of Windows. It does no I/O, so that every
// conversion is tested on any system. See docs/ADR/0049-clipboard.md.
package clip

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/bankai-tensei/gowaku/platform"
)

// The MIME types of each kind, in order of preference when reading.
var (
	TextTypes  = []string{"text/plain;charset=utf-8", "UTF8_STRING", "text/plain", "TEXT", "STRING"}
	HTMLTypes  = []string{"text/html"}
	ImageTypes = []string{"image/png"}
	// FileTypes lists those of files. Nautilus and the other GNOME programs
	// paste files only from their own type.
	FileTypes = []string{"text/uri-list", GnomeFiles}
)

// GnomeFiles is the type in which GNOME programs copy files.
const GnomeFiles = "x-special/gnome-copied-files"

// MaxImage bounds the pixels of an image read from the clipboard, which comes
// from another program.
const MaxImage = 1 << 26

// Kinds returns the kinds of data that the MIME types offer.
func Kinds(types []string) platform.ClipboardKinds {
	var k platform.ClipboardKinds
	has := func(list []string) bool {
		for _, t := range list {
			for _, u := range types {
				if t == u {
					return true
				}
			}
		}
		return false
	}
	if has(TextTypes) {
		k |= platform.ClipboardText
	}
	if has(HTMLTypes) {
		k |= platform.ClipboardHTML
	}
	if has(ImageTypes) {
		k |= platform.ClipboardImage
	}
	if has(FileTypes) {
		k |= platform.ClipboardFiles
	}
	return k
}

// Best returns the first of want that types offers, or "".
func Best(types, want []string) string {
	for _, w := range want {
		for _, t := range types {
			if t == w {
				return w
			}
		}
	}
	return ""
}

// Offer is one MIME type with its data.
type Offer struct {
	Type string
	Data []byte
}

// Encode returns the MIME types in which c goes on the clipboard, the best of
// each kind first. An image that does not encode is left out.
func Encode(c *platform.ClipboardContent) []Offer {
	var out []Offer
	if c.Text != "" {
		for _, t := range TextTypes {
			out = append(out, Offer{t, []byte(c.Text)})
		}
	}
	if c.HTML != "" {
		out = append(out, Offer{"text/html", []byte(c.HTML)})
	}
	if c.Image != nil {
		var b bytes.Buffer
		if png.Encode(&b, c.Image) == nil {
			out = append(out, Offer{"image/png", b.Bytes()})
		}
	}
	if len(c.Files) > 0 {
		uris := FileURIs(c.Files)
		out = append(out,
			Offer{"text/uri-list", []byte(strings.Join(uris, "\r\n") + "\r\n")},
			Offer{GnomeFiles, []byte("copy\n" + strings.Join(uris, "\n"))},
		)
	}
	return out
}

// Decode turns the data of MIME type t into the matching field of c.
func Decode(c *platform.ClipboardContent, t string, data []byte) {
	switch {
	case contains(TextTypes, t):
		c.Text = string(data)
	case contains(HTMLTypes, t):
		c.HTML = DecodeHTMLText(data)
	case contains(ImageTypes, t):
		c.Image = DecodePNG(data)
	case t == GnomeFiles:
		lines := strings.Split(string(data), "\n")
		if len(lines) > 0 && (lines[0] == "copy" || lines[0] == "cut") {
			lines = lines[1:]
		}
		c.Files = ParseURIList(strings.Join(lines, "\n"))
	case contains(FileTypes, t):
		c.Files = ParseURIList(string(data))
	}
}

func contains(list []string, s string) bool {
	for _, t := range list {
		if t == s {
			return true
		}
	}
	return false
}

// DecodePNG reads a PNG into an NRGBA image, or returns nil.
func DecodePNG(data []byte) *image.NRGBA {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxImage {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return toNRGBA(img)
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Rect, img, b.Min, draw.Src)
	return out
}

// DecodeHTMLText takes a fragment out of HTML that a browser put on the
// clipboard: Chrome and Firefox send a whole document on Linux, sometimes in
// UTF-16 with a byte order mark.
func DecodeHTMLText(data []byte) string {
	s := decodeUTF16(data)
	if i := strings.Index(s, "<!--StartFragment-->"); i >= 0 {
		if j := strings.Index(s[i:], "<!--EndFragment-->"); j >= 0 {
			return s[i+len("<!--StartFragment-->") : i+j]
		}
	}
	lower := strings.ToLower(s)
	if i := strings.Index(lower, "<body"); i >= 0 {
		if k := strings.IndexByte(lower[i:], '>'); k >= 0 {
			start := i + k + 1
			end := strings.LastIndex(lower, "</body>")
			if end < start {
				end = len(s)
			}
			return strings.TrimSpace(s[start:end])
		}
	}
	return strings.TrimRight(s, "\x00")
}

// decodeUTF16 reads text that starts with the byte order mark of UTF-16, and
// leaves other text as it is.
func decodeUTF16(data []byte) string {
	if len(data) < 2 || len(data)%2 != 0 {
		return string(data)
	}
	var little bool
	switch {
	case data[0] == 0xFF && data[1] == 0xFE:
		little = true
	case data[0] == 0xFE && data[1] == 0xFF:
	default:
		return string(data)
	}
	units := make([]uint16, 0, len(data)/2-1)
	for i := 2; i+1 < len(data); i += 2 {
		if little {
			units = append(units, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	return utf16String(units)
}

// FileURIs turns absolute paths into file URIs.
func FileURIs(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = filepath.ToSlash(p)
		if !strings.HasPrefix(p, "/") {
			// A Windows path such as C:/x becomes file:///C:/x.
			p = "/" + p
		}
		out = append(out, (&url.URL{Scheme: "file", Path: p}).String())
	}
	return out
}

// ParseURIList reads text/uri-list: the local paths of its file URIs, skipping
// comments and the URIs of other places.
func ParseURIList(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || !strings.HasPrefix(u.Path, "/") {
			continue
		}
		out = append(out, u.Path)
	}
	return out
}
