package input

import "testing"

func TestComposition_Caret(t *testing.T) {
	const kana = "へんかん"
	cases := []struct {
		name string
		c    Composition
		want int
	}{
		{"typing: at the end", Composition{Text: kana, Cursor: len(kana)}, len(kana)},
		{"typing: moved back with the arrows", Composition{Text: kana, Cursor: 6}, 6},
		{"typing, with a clause for the input", Composition{Text: kana, Cursor: 3, Clauses: []Clause{{0, len(kana), ClauseInput}}}, 3},
		// On conversion an IME reports the start of the target clause.
		{"converting one clause", Composition{Text: "変換", Cursor: 0, Clauses: []Clause{{0, 6, ClauseTarget}}}, 6},
		{"converting the second clause", Composition{Text: "今日は晴れ", Cursor: 9, Clauses: []Clause{{0, 9, ClauseConverted}, {9, 15, ClauseTarget}}}, 15},
		{"converted, no target", Composition{Text: "変換", Cursor: 3, Clauses: []Clause{{0, 6, ClauseConverted}}}, 3},
		{"a cursor past the text", Composition{Text: "ab", Cursor: 9}, 2},
		{"a negative cursor", Composition{Text: "ab", Cursor: -1}, 0},
		{"empty", Composition{}, 0},
	}
	for _, c := range cases {
		if got := c.c.Caret(); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
