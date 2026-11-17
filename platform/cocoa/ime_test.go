package cocoa

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestByteOffset_UTF16(t *testing.T) {
	s := "aあ😀b" // 1 + 3 + 4 + 1 bytes; 1 + 1 + 2 + 1 units
	for _, c := range []struct{ units, want int }{
		{0, 0}, {1, 1}, {2, 4}, {4, 8}, {5, 9}, {9, 9},
		{3, 8}, // 絵文字のサロゲートの間は次の字に寄せる
	} {
		if got := byteOffset(s, c.units); got != c.want {
			t.Errorf("byteOffset(%d) = %d, want %d", c.units, got, c.want)
		}
	}
	if got := utf16Len(s); got != 5 {
		t.Errorf("utf16Len = %d, want 5", got)
	}
	if got := utf16Offset(s, 8); got != 4 {
		t.Errorf("utf16Offset(8) = %d, want 4", got)
	}
}

func TestComposition_Clauses(t *testing.T) {
	t.Run("入力中は全体が未変換", func(t *testing.T) {
		got := composition("きょうは", []markedRun{{0, 4, false}}, 4)
		want := &input.Composition{Text: "きょうは", Cursor: 12, Clauses: []input.Clause{{Start: 0, End: 12, Kind: input.ClauseInput}}}
		if !sameComposition(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	t.Run("変換中は太い下線が注目文節", func(t *testing.T) {
		got := composition("今日は晴れ", []markedRun{{0, 3, true}, {3, 5, false}}, 3)
		want := &input.Composition{Text: "今日は晴れ", Cursor: 9, Clauses: []input.Clause{
			{Start: 0, End: 9, Kind: input.ClauseTarget}, {Start: 9, End: 15, Kind: input.ClauseConverted},
		}}
		if !sameComposition(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	t.Run("属性の無い文字列", func(t *testing.T) {
		got := composition("ka", nil, 2)
		want := &input.Composition{Text: "ka", Cursor: 2, Clauses: []input.Clause{{Start: 0, End: 2, Kind: input.ClauseInput}}}
		if !sameComposition(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func sameComposition(a, b *input.Composition) bool {
	return a.Text == b.Text && a.Cursor == b.Cursor && slices.Equal(a.Clauses, b.Clauses)
}

func TestValidUTF8_ReplacesLoneSurrogate(t *testing.T) {
	if got := validUTF8("a\u3042"); got != "a\u3042" {
		t.Errorf("valid text changed to %q", got)
	}
	if got := validUTF8("a\xed\xa0\x80b"); got != "a\uFFFD\uFFFD\uFFFDb" {
		t.Errorf("got %q", got)
	}
}
