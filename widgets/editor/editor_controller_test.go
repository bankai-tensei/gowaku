package editor

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

func pos(line, col int) TextPosition { return TextPosition{Line: line, Column: col} }

func caretAt(c *Controller, p TextPosition) { c.SetSelection(TextRange{Start: p, End: p}) }

func TestTextEditorController_ZeroValue(t *testing.T) {
	var c Controller
	if c.Text() != "" || c.LineCount() != 1 || c.Line(0) != "" || c.Len() != 0 {
		t.Error("the zero controller must be an empty document")
	}
	c.Insert("a\nb")
	if c.Text() != "a\nb" || c.Selection().End != pos(1, 1) {
		t.Errorf("got %q, caret %v", c.Text(), c.Selection().End)
	}
	if c.Offset(pos(1, 1)) != 3 || c.PositionAt(2) != pos(1, 0) {
		t.Error("offsets and positions disagree")
	}
}

func TestTextEditorController_ReplaceMovesTheSelection(t *testing.T) {
	cases := []struct {
		name string
		sel  TextRange
		r    TextRange
		s    string
		want TextRange
	}{
		{"前の選択はそのまま", TextRange{pos(0, 0), pos(0, 1)}, TextRange{pos(1, 0), pos(1, 2)}, "X", TextRange{pos(0, 0), pos(0, 1)}},
		{"後ろの選択は動く", TextRange{pos(1, 3), pos(1, 4)}, TextRange{pos(1, 0), pos(1, 2)}, "X\nY", TextRange{pos(2, 2), pos(2, 3)}},
		{"重なる選択はキャレットに", TextRange{pos(1, 1), pos(1, 3)}, TextRange{pos(1, 0), pos(1, 2)}, "X", TextRange{pos(1, 1), pos(1, 1)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ctl Controller
			ctl.SetText("abcd\nefgh")
			ctl.SetSelection(c.sel)
			ctl.Replace(c.r, c.s)
			if got := ctl.Selection(); got != c.want {
				t.Errorf("selection %v, want %v (text %q)", got, c.want, ctl.Text())
			}
		})
	}
}

func TestTextEditorController_UndoMergesWords(t *testing.T) {
	var ctl Controller
	for _, s := range []string{"o", "n", "e", " ", "t", "w", "o"} {
		ctl.replace(ctl.cur().caret, ctl.cur().caret, s, kit.EditTyping)
	}
	ctl.Undo()
	if ctl.Text() != "one " {
		t.Errorf("one undo left %q", ctl.Text())
	}
	ctl.Undo()
	if ctl.Text() != "" {
		t.Errorf("two undos left %q", ctl.Text())
	}
	ctl.Redo()
	ctl.Redo()
	if ctl.Text() != "one two" || ctl.Selection().End != pos(0, 7) {
		t.Errorf("redo gave %q, caret %v", ctl.Text(), ctl.Selection().End)
	}
	if !ctl.CanUndo() || ctl.CanRedo() {
		t.Error("CanUndo and CanRedo are wrong")
	}
}

func TestTextEditorController_UndoRestoresTheSelection(t *testing.T) {
	var ctl Controller
	ctl.SetText("one\ntwo\nthree")
	ctl.SetSelection(TextRange{pos(0, 1), pos(2, 2)})
	ctl.Insert("X")
	if ctl.Text() != "oXree" {
		t.Fatalf("got %q", ctl.Text())
	}
	ctl.Undo()
	if ctl.Text() != "one\ntwo\nthree" || ctl.Selection() != (TextRange{pos(0, 1), pos(2, 2)}) {
		t.Errorf("undo gave %q, %v", ctl.Text(), ctl.Selection())
	}
}

func TestTextEditorController_FindOptions(t *testing.T) {
	text := "Go go gopher GO\nergo Gö gö"
	cases := []struct {
		name  string
		query string
		opts  FindOptions
		want  int
	}{
		{"大文字小文字を区別しない", "go", FindOptions{}, 5},
		{"区別する", "go", FindOptions{MatchCase: true}, 3},
		{"単語単位", "go", FindOptions{WholeWord: true}, 3},
		{"ASCII でない字", "GÖ", FindOptions{}, 2},
		{"正規表現", `g\w+`, FindOptions{Regexp: true, MatchCase: true}, 3},
		{"正規表現と単語単位", `go`, FindOptions{Regexp: true, WholeWord: true}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ctl Controller
			ctl.SetText(text)
			n, err := ctl.Find(c.query, c.opts)
			if err != nil || n != c.want {
				t.Errorf("%d matches (%v), want %d: %v", n, err, c.want, ctl.Matches())
			}
		})
	}
	var ctl Controller
	if _, err := ctl.Find("(", FindOptions{Regexp: true}); err == nil {
		t.Error("a bad pattern must return its error")
	}
}

func TestTextEditorController_FindNextWraps(t *testing.T) {
	var ctl Controller
	ctl.SetText("ab ab\nab")
	if _, err := ctl.Find("ab", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	caretAt(&ctl, pos(0, 1))
	var got []TextPosition
	for range 4 {
		ctl.FindNext()
		got = append(got, ctl.Selection().Start)
	}
	want := []TextPosition{pos(0, 3), pos(1, 0), pos(0, 0), pos(0, 3)}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FindNext went %v, want %v", got, want)
		}
	}
	if ctl.CurrentMatch() != 1 {
		t.Errorf("current match %d, want 1", ctl.CurrentMatch())
	}
	ctl.FindPrevious()
	ctl.FindPrevious()
	if ctl.Selection().Start != pos(1, 0) {
		t.Errorf("FindPrevious went to %v", ctl.Selection().Start)
	}
}

func TestTextEditorController_MatchesFollowEdits(t *testing.T) {
	var ctl Controller
	ctl.SetText("cat\ndog\ncat cat")
	if _, err := ctl.Find("cat", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	caretAt(&ctl, pos(1, 0))
	ctl.Insert("cat\n\n")
	want := []TextRange{
		{pos(0, 0), pos(0, 3)}, {pos(1, 0), pos(1, 3)},
		{pos(4, 0), pos(4, 3)}, {pos(4, 4), pos(4, 7)},
	}
	got := ctl.Matches()
	if len(got) != len(want) {
		t.Fatalf("matches %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("matches %v, want %v", got, want)
		}
	}
}

func TestTextEditorController_Replace(t *testing.T) {
	var ctl Controller
	ctl.SetText("a1 b22\nc333")
	if _, err := ctl.Find(`[a-z](\d+)`, FindOptions{Regexp: true}); err != nil {
		t.Fatal(err)
	}
	ctl.FindNext()
	if !ctl.ReplaceMatch("<$1>") || ctl.Text() != "<1> b22\nc333" {
		t.Errorf("ReplaceMatch gave %q", ctl.Text())
	}
	if ctl.SelectedText() != "b22" {
		t.Errorf("the next match is not selected: %q", ctl.SelectedText())
	}
	if n := ctl.ReplaceAll("#"); n != 2 || ctl.Text() != "<1> #\n#" {
		t.Errorf("ReplaceAll replaced %d: %q", n, ctl.Text())
	}
	ctl.Undo()
	if ctl.Text() != "<1> b22\nc333" {
		t.Errorf("one undo must take back ReplaceAll: %q", ctl.Text())
	}
	ctl.Undo()
	if ctl.Text() != "a1 b22\nc333" {
		t.Errorf("undo of ReplaceMatch: %q", ctl.Text())
	}
}
