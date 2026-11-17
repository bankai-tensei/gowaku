package text

import (
	"slices"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestShaper_GraphemeBounds(t *testing.T) {
	s, err := NewShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, content string
		want          []int
	}{
		{"空", "", []int{0}},
		{"ASCII", "ab", []int{0, 1, 2}},
		{"かな", "あい", []int{0, 3, 6}},
		{"結合文字", "éx", []int{0, 3, 4}},
		{"ZWJ の絵文字", "👨‍👩x", []int{0, 11, 12}},
		{"国旗", "🇯🇵a", []int{0, 8, 9}},
		{"異体字セレクタ", "葛\U000E0100a", []int{0, 7, 8}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := s.GraphemeBounds(c.content, nil)
			if !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
	// dst is appended to.
	if got := s.GraphemeBounds("a", []int{9}); !slices.Equal(got, []int{9, 0, 1}) {
		t.Errorf("append: got %v", got)
	}
}
