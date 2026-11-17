package text

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"golang.org/x/image/font/gofont/goregular"
)

func TestStartupFaces_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", startupFile())
	want := []startupFace{
		{location: fontscan.Location{File: `C:\Windows\Fonts\YuGothR.ttc`, Index: 1}, size: 13835976, modTime: 1765314095426031600},
		{location: fontscan.Location{File: "/usr/share/fonts/a b.otf", Instance: 2}, size: 1, modTime: -5},
	}
	if err := writeStartupFaces(path, want); err != nil {
		t.Fatal(err)
	}
	if got := readStartupFaces(path); !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestStartupFaces_BrokenListIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), startupFile())
	if err := os.WriteFile(path, []byte("a\t0\t0\tx\t1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readStartupFaces(path); got != nil {
		t.Errorf("got %+v", got)
	}
	if got := readStartupFaces(filepath.Join(t.TempDir(), "none")); got != nil {
		t.Errorf("got %+v", got)
	}
}

func TestPreload_LeavesOutChangedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	same, ok := statFace(fontscan.Location{File: path})
	if !ok {
		t.Fatal("cannot stat the font")
	}
	changed := same
	changed.size++
	missing := startupFace{location: fontscan.Location{File: path + ".gone"}}
	got := preload([]startupFace{changed, same, missing})()
	if len(got) != 1 || got[0].location != same.location || got[0].face == nil {
		t.Fatalf("got %+v", got)
	}
	if n := got[0].face.Upem(); n != 2048 {
		t.Errorf("upem %d", n)
	}
}

// TestSaveStartupFaces_NextShaperResolvesToTheSameFonts saves the fonts of a
// shaped text, and checks that the next Shaper finds them preloaded and shapes
// as one that read them itself.
func TestSaveStartupFaces_NextShaperResolvesToTheSameFonts(t *testing.T) {
	systemShaper(t)
	dir, err := os.UserCacheDir()
	if err != nil {
		t.Skip(err)
	}
	cache := filepath.Join(dir, "gowaku")
	list := filepath.Join(t.TempDir(), startupFile())
	const sample = "aあ亜"
	first, err := newSystemShaper(cache, list)
	if err != nil {
		t.Fatal(err)
	}
	want := first.Shape(sample, Style{Size: 16}).Layout(1000)
	first.Shape("太字", Style{Size: 16, Weight: 700})
	first.SaveStartupFaces()
	saved := readStartupFaces(list)
	if len(saved) < 2 {
		t.Fatalf("saved %+v", saved)
	}

	next, err := newSystemShaper(cache, list)
	if err != nil {
		t.Fatal(err)
	}
	got := next.Shape(sample, Style{Size: 16}).Layout(1000)
	if got.Size() != want.Size() {
		t.Errorf("size %v, want %v", got.Size(), want.Size())
	}
	for _, f := range next.faces {
		if !next.adopted(f.Font) {
			t.Errorf("%v was read again", next.fonts.FontLocation(f.Font))
		}
	}
	if len(next.faces) == 0 {
		t.Error("no faces")
	}
}

// adopted reports whether f is one of the fonts that newSystemShaper read ahead.
func (s *Shaper) adopted(f *font.Font) bool {
	for _, face := range s.adoptedFaces {
		if face.Font == f {
			return true
		}
	}
	return false
}

func TestStartupFile_IsAFileName(t *testing.T) {
	name := startupFile()
	if filepath.Base(name) != name || !strings.HasPrefix(name, "startup_faces_v1_text") {
		t.Errorf("got %q", name)
	}
}
