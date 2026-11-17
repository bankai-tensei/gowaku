package markdown

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// pngFile returns a PNG of w by h pixels.
func pngFile(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		m.Pix[i] = 0xff
	}
	m.Set(0, 0, color.NRGBA{R: 0xff, A: 0xff})
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// countingFS counts the files opened.
type countingFS struct {
	fs.FS
	opened *int
}

func (c countingFS) Open(name string) (fs.File, error) {
	*c.opened++
	return c.FS.Open(name)
}

func pictures(t *testing.T) fstest.MapFS {
	return fstest.MapFS{
		"logo.png":     {Data: pngFile(t, 200, 80)},
		"img/wide.png": {Data: pngFile(t, 1600, 400)},
		"notes.txt":    {Data: []byte("no picture")},
	}
}

func TestMarkdown_ImagesAreReadFromTheFileSystem(t *testing.T) {
	tt := gowakutest.New(t, Markdown{Source: "Before\n\n![the logo](logo.png)\n\nAfter", Images: pictures(t)},
		gowakutest.Options{Width: 600, Height: 400})
	n, ok := tt.Find("the logo")
	if !ok || n.Role != semantics.RoleImage {
		t.Fatalf("no image named by its alternative text\n%s", tt.Dump())
	}
	if n.Rect.W != 200 || n.Rect.H != 80 {
		t.Errorf("the picture is %v by %v, want its own 200 by 80", n.Rect.W, n.Rect.H)
	}
}

func TestMarkdown_ImageShrinksToTheWidthThereIs(t *testing.T) {
	for _, src := range []string{"img/wide.png", "./img/wide.png", "/img/wide.png", "img/../img/wide.png"} {
		tt := gowakutest.New(t, Markdown{Source: "![wide](" + src + ")", Images: pictures(t)},
			gowakutest.Options{Width: 400, Height: 400})
		n, ok := tt.Find("wide")
		if !ok {
			t.Fatalf("%s: no image\n%s", src, tt.Dump())
		}
		if n.Rect.W != 400 || n.Rect.H != 100 {
			t.Errorf("%s: the picture is %v by %v, want 400 by 100, its proportions kept", src, n.Rect.W, n.Rect.H)
		}
	}
}

func TestMarkdown_ImageThatCannotBeReadShowsItsText(t *testing.T) {
	sources := []string{
		"missing.png",
		"notes.txt",
		"../logo.png",
		"https://example.com/logo.png",
		"//example.com/logo.png",
		"file:///logo.png",
		"data:image/png;base64,AAAA",
		"logo.png?size=2",
		`img\wide.png`,
	}
	for _, src := range sources {
		opened := 0
		tt := gowakutest.New(t, Markdown{Source: "![the words](" + src + ")", Images: countingFS{pictures(t), &opened}},
			gowakutest.Options{Width: 600, Height: 400})
		if len(tt.FindAll("the words")) == 0 {
			t.Errorf("%s: the alternative text is gone\n%s", src, tt.Dump())
		}
		if n, ok := tt.Find("the words"); ok && n.Rect.H > 60 {
			t.Errorf("%s: a picture %v high shows where the text belongs", src, n.Rect.H)
		}
		if src != "missing.png" && src != "notes.txt" && opened != 0 {
			t.Errorf("%s: opened %d files, want none", src, opened)
		}
	}
}

func TestMarkdown_ImageIsReadOnce(t *testing.T) {
	opened := 0
	files := countingFS{pictures(t), &opened}
	var hs *picturesState
	tt := gowakutest.New(t, picturesHost{files: files, self: &hs}, gowakutest.Options{})
	for range 3 {
		hs.SetState(func() { hs.count++ })
		tt.Pump()
	}
	// The one that is there, and the one that is not: each tried once.
	if opened != 2 {
		t.Errorf("opened %d files over four builds, want 2", opened)
	}
	// Another document reads its pictures anew, the same file included.
	hs.SetState(func() { hs.source = "![again](logo.png)" })
	tt.Pump()
	if opened != 3 {
		t.Errorf("opened %d files after the document changed, want 3", opened)
	}
}

// picturesHost is a stateful parent that rebuilds a Markdown with pictures.
type picturesHost struct {
	core.StatefulBase
	files fs.FS
	self  **picturesState
}

type picturesState struct {
	core.StateOf[picturesHost]
	count int
	// source takes the place of the document of three pictures, if set.
	source string
}

func (picturesHost) CreateState() core.State { return &picturesState{} }

func (s *picturesState) InitState() { *s.Widget().self = s }

func (s *picturesState) Build(core.BuildContext) core.Widget {
	source := s.source
	if source == "" {
		source = "![a](logo.png)\n\n![b](logo.png)\n\n![c](missing.png)"
	}
	return Markdown{Source: source, Images: s.Widget().files}
}

func TestMarkdown_ImageProviderComesBeforeImages(t *testing.T) {
	opened := 0
	tt := gowakutest.New(t, Markdown{
		Source: "![mine](logo.png)\n\n![theirs](img/wide.png)",
		Images: countingFS{pictures(t), &opened},
		ImageProvider: func(src, _ string) core.Widget {
			if src == "logo.png" {
				return basic.SizedBox{Width: 40, Height: 30}
			}
			return nil
		},
	}, gowakutest.Options{Width: 400, Height: 400})
	if opened != 1 {
		t.Errorf("opened %d files, want only the one the provider left", opened)
	}
	if n, ok := tt.Find("theirs"); !ok || n.Rect.W != 400 {
		t.Errorf("the image the provider left is not read from Images\n%s", tt.Dump())
	}
}

func TestMarkdown_WithoutImagesNothingIsRead(t *testing.T) {
	tt := gowakutest.New(t, Markdown{Source: "![the words](logo.png)"}, gowakutest.Options{})
	if len(tt.FindAll("the words")) == 0 {
		t.Errorf("the alternative text is gone\n%s", tt.Dump())
	}
}

func TestImagePath(t *testing.T) {
	cases := []struct {
		src, want string
		ok        bool
	}{
		{"logo.png", "logo.png", true},
		{"./a/b.png", "a/b.png", true},
		{"/a/b.png", "a/b.png", true},
		{"a/../b.png", "b.png", true},
		{"", "", false},
		{".", "", false},
		{"../b.png", "", false},
		{"a/../../b.png", "", false},
		{"http://x/b.png", "", false},
		{"//x/b.png", "", false},
		{"b.png#frag", "", false},
	}
	for _, c := range cases {
		if got, ok := imagePath(c.src); got != c.want || ok != c.ok {
			t.Errorf("imagePath(%q) = %q, %v; want %q, %v", c.src, got, ok, c.want, c.ok)
		}
	}
}
