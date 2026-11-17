package clip

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
)

func sample() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	img.SetNRGBA(2, 1, color.NRGBA{0, 0, 255, 128})
	img.SetNRGBA(1, 1, color.NRGBA{10, 20, 30, 0})
	return img
}

func TestEncodeDecode_RoundTripsEveryKind(t *testing.T) {
	in := platform.ClipboardContent{
		Text: "和文 text", HTML: "<b>太字</b>", Image: sample(),
		Files: []string{"/home/a/メモ.txt", "/tmp/with space"},
	}
	offers := Encode(&in)
	var types []string
	for _, o := range offers {
		types = append(types, o.Type)
	}
	if k := Kinds(types); k != in.Kinds() {
		t.Errorf("kinds %b, want %b", k, in.Kinds())
	}
	var out platform.ClipboardContent
	for _, want := range [][]string{TextTypes, HTMLTypes, ImageTypes, {"text/uri-list"}} {
		best := Best(types, want)
		for _, o := range offers {
			if o.Type == best {
				Decode(&out, o.Type, o.Data)
			}
		}
	}
	if out.Text != in.Text || out.HTML != in.HTML || !slices.Equal(out.Files, in.Files) {
		t.Errorf("got %q %q %q", out.Text, out.HTML, out.Files)
	}
	if out.Image == nil || !slices.Equal(out.Image.Pix, in.Image.Pix) {
		t.Error("the image must come back as it went")
	}
	var gnome platform.ClipboardContent
	for _, o := range offers {
		if o.Type == GnomeFiles {
			Decode(&gnome, o.Type, o.Data)
		}
	}
	if !slices.Equal(gnome.Files, in.Files) {
		t.Errorf("GNOME files %q", gnome.Files)
	}
}

func TestDecodeHTMLText_BrowserDocuments(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"markers", []byte("<html><body><!--StartFragment--><i>x</i><!--EndFragment--></body></html>"), "<i>x</i>"},
		{"body", []byte("<meta charset='utf-8'><html><BODY class=a>\n<p>y</p>\n</body></html>"), "<p>y</p>"},
		{"fragment", []byte("<span>z</span>"), "<span>z</span>"},
		{"UTF-16LE", []byte{0xFF, 0xFE, '<', 0, 'b', 0, '>', 0, 0x8C, 0x54, '<', 0, '/', 0, 'b', 0, '>', 0}, "<b>和</b>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecodeHTMLText(tt.data); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTMLFormat_OffsetsPointAtTheFragment(t *testing.T) {
	data := EncodeHTMLFormat("<b>和文</b>")
	if got := DecodeHTMLFormat(data); got != "<b>和文</b>" {
		t.Errorf("got %q", got)
	}
	// What Chrome puts on the clipboard, shortened: offsets of ten digits.
	doc := "<html>\r\n<body>\r\n<!--StartFragment--><i>italic</i>x<!--EndFragment-->\r\n</body>\r\n</html>"
	head := len(fmt.Sprintf(htmlHeader, 0, 0, 0, 0))
	start := head + strings.Index(doc, "<i>")
	end := head + strings.Index(doc, "<!--EndFragment-->")
	chrome := fmt.Sprintf(htmlHeader, head, head+len(doc), start, end) + doc
	if got := DecodeHTMLFormat([]byte(chrome)); got != "<i>italic</i>x" {
		t.Errorf("Chrome: got %q", got)
	}
}

func TestDIB_RoundTripAndOpaqueZeroAlpha(t *testing.T) {
	in := sample()
	out := DecodeDIB(EncodeDIB(in))
	if out == nil || !slices.Equal(out.Pix, in.Pix) {
		t.Fatalf("round trip: %v", out)
	}
	opaque := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	copy(opaque.Pix, []byte{1, 2, 3, 0, 4, 5, 6, 0})
	got := DecodeDIB(EncodeDIB(opaque))
	if got.Pix[3] != 255 || got.Pix[7] != 255 {
		t.Error("a bitmap without alpha must be opaque")
	}
	if DecodeDIB([]byte{1, 2, 3}) != nil || DecodeDIB(make([]byte, 40)) != nil {
		t.Error("broken data must give nil")
	}
}

func TestDropFiles_RoundTrip(t *testing.T) {
	paths := []string{`C:\Users\a\メモ.txt`, `D:\x y\z`}
	if got := DecodeDropFiles(EncodeDropFiles(paths)); !slices.Equal(got, paths) {
		t.Errorf("got %q", got)
	}
}

func TestParseURIList_SkipsCommentsAndRemote(t *testing.T) {
	got := ParseURIList("# comment\r\nfile:///a/b%20c\r\nhttp://x/y\r\nfile://localhost/d\r\nfile://other/e\r\n")
	if !slices.Equal(got, []string{"/a/b c", "/d"}) {
		t.Errorf("got %q", got)
	}
}

func FuzzDecodeDIB(f *testing.F) {
	f.Add(EncodeDIB(sample()))
	f.Fuzz(func(_ *testing.T, data []byte) { DecodeDIB(data) })
}

func FuzzDecodeDropFiles(f *testing.F) {
	f.Add(EncodeDropFiles([]string{"a", "b"}))
	f.Fuzz(func(_ *testing.T, data []byte) { DecodeDropFiles(data) })
}
