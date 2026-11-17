package clip

import (
	"encoding/binary"
	"fmt"
	"image"
	"strconv"
	"strings"
	"unicode/utf16"
)

// The formats of Windows live in a file without the _windows suffix, so that
// they are tested on every system.

// htmlHeader is the header of the HTML Format of Windows, with room for the
// offsets, which have a fixed width so that the header has a known length.
const htmlHeader = "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"

// EncodeHTMLFormat wraps a fragment in the HTML Format of Windows: a header of
// byte offsets, then a document with the fragment between markers. It is UTF-8.
func EncodeHTMLFormat(fragment string) []byte {
	const (
		before = "<html><body>\r\n<!--StartFragment-->"
		after  = "<!--EndFragment-->\r\n</body></html>"
	)
	head := len(fmt.Sprintf(htmlHeader, 0, 0, 0, 0))
	startHTML := head
	startFragment := startHTML + len(before)
	endFragment := startFragment + len(fragment)
	endHTML := endFragment + len(after)
	return []byte(fmt.Sprintf(htmlHeader, startHTML, endHTML, startFragment, endFragment) + before + fragment + after)
}

// DecodeHTMLFormat takes the fragment out of the HTML Format of Windows. It
// falls back to the markers when the offsets are missing or wrong.
func DecodeHTMLFormat(data []byte) string {
	s := strings.TrimRight(string(data), "\x00")
	start, end := htmlOffset(s, "StartFragment:"), htmlOffset(s, "EndFragment:")
	if start >= 0 && end >= start && end <= len(s) {
		return s[start:end]
	}
	return DecodeHTMLText([]byte(s))
}

func htmlOffset(s, key string) int {
	i := strings.Index(s, key)
	if i < 0 {
		return -1
	}
	rest := s[i+len(key):]
	j := strings.IndexAny(rest, "\r\n")
	if j < 0 {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:j]))
	if err != nil {
		return -1
	}
	return n
}

// The fields of BITMAPINFOHEADER that matter.
const (
	bitmapHeaderSize = 40
	biRGB            = 0
	biBitfields      = 3
)

// EncodeDIB writes img as the data of CF_DIB: a BITMAPINFOHEADER of 32 bits a
// pixel, then the rows from the bottom up, in BGRA with straight alpha.
func EncodeDIB(img *image.NRGBA) []byte {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	out := make([]byte, bitmapHeaderSize+4*w*h)
	le := binary.LittleEndian
	le.PutUint32(out[0:], bitmapHeaderSize)
	le.PutUint32(out[4:], uint32(int32(w)))
	le.PutUint32(out[8:], uint32(int32(h)))
	le.PutUint16(out[12:], 1)
	le.PutUint16(out[14:], 32)
	le.PutUint32(out[16:], biRGB)
	le.PutUint32(out[20:], uint32(4*w*h))
	px := out[bitmapHeaderSize:]
	for y := range h {
		src := img.Pix[y*img.Stride : y*img.Stride+4*w]
		dst := px[(h-1-y)*4*w:]
		for x := range w {
			dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = src[4*x+2], src[4*x+1], src[4*x], src[4*x+3]
		}
	}
	return out
}

// DecodeDIB reads the data of CF_DIB or CF_DIBV5 of 24 or 32 bits a pixel,
// uncompressed or with the usual bit fields, or returns nil. A 32-bit bitmap
// whose alpha is all zero is taken as opaque, as most programs mean it.
func DecodeDIB(data []byte) *image.NRGBA {
	le := binary.LittleEndian
	if len(data) < bitmapHeaderSize {
		return nil
	}
	size := int(le.Uint32(data[0:]))
	w, h := int(int32(le.Uint32(data[4:]))), int(int32(le.Uint32(data[8:])))
	bits, compression := int(le.Uint16(data[14:])), le.Uint32(data[16:])
	if size < bitmapHeaderSize || size > len(data) || w <= 0 || h == 0 {
		return nil
	}
	topDown := h < 0
	if topDown {
		h = -h
	}
	if w*h > MaxImage || (bits != 24 && bits != 32) || (compression != biRGB && compression != biBitfields) {
		return nil
	}
	offset := size
	if compression == biBitfields && size == bitmapHeaderSize {
		// Three masks follow the old header.
		offset += 12
	}
	stride := (w*bits/8 + 3) &^ 3
	if offset+stride*h > len(data) {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	anyAlpha := false
	for y := range h {
		row := y
		if !topDown {
			row = h - 1 - y
		}
		src := data[offset+row*stride:]
		dst := img.Pix[y*img.Stride:]
		for x := range w {
			s := src[x*bits/8:]
			a := byte(255)
			if bits == 32 {
				a = s[3]
				anyAlpha = anyAlpha || a != 0
			}
			dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = s[2], s[1], s[0], a
		}
	}
	if bits == 32 && !anyAlpha {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	return img
}

// dropFilesSize is the size of DROPFILES, after which the paths come.
const dropFilesSize = 20

// EncodeDropFiles writes the data of CF_HDROP: DROPFILES, then the paths in
// UTF-16, each ended by a zero, and one more zero after the last.
func EncodeDropFiles(paths []string) []byte {
	units := make([]uint16, 0, 64)
	for _, p := range paths {
		units = append(units, utf16.Encode([]rune(p))...)
		units = append(units, 0)
	}
	units = append(units, 0)
	out := make([]byte, dropFilesSize+2*len(units))
	le := binary.LittleEndian
	le.PutUint32(out[0:], dropFilesSize) // pFiles
	le.PutUint32(out[16:], 1)            // fWide
	for i, u := range units {
		le.PutUint16(out[dropFilesSize+2*i:], u)
	}
	return out
}

// DecodeDropFiles reads the data of CF_HDROP.
func DecodeDropFiles(data []byte) []string {
	le := binary.LittleEndian
	if len(data) < dropFilesSize {
		return nil
	}
	start, wide := int(le.Uint32(data[0:])), le.Uint32(data[16:]) != 0
	if start < dropFilesSize || start > len(data) {
		return nil
	}
	var out []string
	rest := data[start:]
	if !wide {
		for _, p := range strings.Split(string(rest), "\x00") {
			if p == "" {
				break
			}
			out = append(out, p)
		}
		return out
	}
	var cur []uint16
	for i := 0; i+1 < len(rest); i += 2 {
		u := le.Uint16(rest[i:])
		if u != 0 {
			cur = append(cur, u)
			continue
		}
		if len(cur) == 0 {
			break
		}
		out = append(out, utf16String(cur))
		cur = cur[:0]
	}
	return out
}

func utf16String(units []uint16) string { return string(utf16.Decode(units)) }
