package text

import (
	"testing"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/geom"
)

// TestFamily_ListSkipsWhatIsMissing: a Family of several names takes the first
// that the fonts have, so a list of the monospaced families of every OS works
// on each of them.
func TestFamily_ListSkipsWhatIsMissing(t *testing.T) {
	s, err := NewShaper(goregular.TTF, gomono.TTF)
	if err != nil {
		t.Fatal(err)
	}
	width := func(content, family string) float32 {
		return s.Shape(content, Style{Family: family, Size: 20}).Layout(geom.Inf).Size().W
	}
	for _, family := range []string{"Go Mono", "Cascadia Mono, Go Mono", " Menlo ,Go Mono, Go"} {
		if i, w := width("iiii", family), width("WWWW", family); i != w {
			t.Errorf("%q: iiii is %v wide and WWWW %v, want a monospaced face", family, i, w)
		}
	}
	if i, w := width("iiii", "Cascadia Mono, Go"), width("WWWW", "Cascadia Mono, Go"); i == w {
		t.Error("the proportional Go took the place of the list")
	}
}
