package wayland

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestTextInputDone(t *testing.T) {
	tests := []struct {
		name      string
		composing bool
		preedit   string
		b, e      int32
		commit    string
		want      textInputUpdate
	}{
		{"打ち始め", false, "に", 3, 3, "",
			textInputUpdate{composition: &input.Composition{Text: "に", Cursor: 3, Clauses: []input.Clause{{Start: 0, End: 3, Kind: input.ClauseInput}}}}},
		{"続けて打つ", true, "にほ", 6, 6, "",
			textInputUpdate{composition: &input.Composition{Text: "にほ", Cursor: 6, Clauses: []input.Clause{{Start: 0, End: 6, Kind: input.ClauseInput}}}}},
		{"確定", true, "", 0, 0, "日本",
			textInputUpdate{endComposition: true, commit: "日本"}},
		{"消して空に", true, "", 0, 0, "",
			textInputUpdate{endComposition: true}},
		{"確定して次を打つ", true, "ご", 3, 3, "日本",
			textInputUpdate{endComposition: true, commit: "日本", composition: &input.Composition{Text: "ご", Cursor: 3, Clauses: []input.Clause{{Start: 0, End: 3, Kind: input.ClauseInput}}}}},
		{"変換中の文節", true, "日本語を", 6, 9, "",
			textInputUpdate{composition: &input.Composition{Text: "日本語を", Cursor: 6, Clauses: []input.Clause{
				{Start: 0, End: 6, Kind: input.ClauseConverted}, {Start: 6, End: 9, Kind: input.ClauseTarget}, {Start: 9, End: 12, Kind: input.ClauseConverted}}}}},
		{"カーソル非表示", false, "あ", -1, -1, "",
			textInputUpdate{composition: &input.Composition{Text: "あ", Cursor: 3, Clauses: []input.Clause{{Start: 0, End: 3, Kind: input.ClauseInput}}}}},
		{"文字の途中を指す", false, "あい", 4, 4, "",
			textInputUpdate{composition: &input.Composition{Text: "あい", Cursor: 3, Clauses: []input.Clause{{Start: 0, End: 6, Kind: input.ClauseInput}}}}},
		{"範囲外", false, "a", 9, 9, "",
			textInputUpdate{composition: &input.Composition{Text: "a", Cursor: 1, Clauses: []input.Clause{{Start: 0, End: 1, Kind: input.ClauseInput}}}}},
		{"入力メソッドを使わない確定", false, "", 0, 0, "x",
			textInputUpdate{commit: "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textInputDone(tt.composing, tt.preedit, tt.b, tt.e, tt.commit)
			if !sameUpdate(got, tt.want) {
				t.Errorf("got %+v %+v, want %+v %+v", got, got.composition, tt.want, tt.want.composition)
			}
		})
	}
}

func sameUpdate(a, b textInputUpdate) bool {
	if a.endComposition != b.endComposition || a.commit != b.commit || (a.composition == nil) != (b.composition == nil) {
		return false
	}
	if a.composition == nil {
		return true
	}
	x, y := a.composition, b.composition
	return x.Text == y.Text && x.Cursor == y.Cursor && slices.Equal(x.Clauses, y.Clauses)
}
