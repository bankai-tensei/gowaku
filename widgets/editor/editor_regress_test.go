package editor

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/text"
)

func TestTextEditorController_UndoOfCRLFKeepsTheRest(t *testing.T) {
	for _, s := range []string{"a\r\nb", "a\rb", "\r\n\r\n", "a\r\n"} {
		var c Controller
		c.SetText("suffix")
		c.Insert(s)
		after := c.Text()
		c.Undo()
		if c.Text() != "suffix" {
			t.Errorf("insert %q, undo: got %q", s, c.Text())
		}
		c.Redo()
		if c.Text() != after {
			t.Errorf("insert %q, redo: got %q, want %q", s, c.Text(), after)
		}
		c.Undo()
		if c.Text() != "suffix" {
			t.Errorf("insert %q, second undo: got %q", s, c.Text())
		}
	}
}

func TestTextEditorController_ZeroValuePositionAt(t *testing.T) {
	var c Controller
	for _, off := range []int{-1, 0, 1, 100} {
		if got := c.PositionAt(off); got != pos(0, 0) {
			t.Errorf("PositionAt(%d) = %v", off, got)
		}
	}
}

func TestTextEditorController_RegexpReplaceKeepsContext(t *testing.T) {
	tests := []struct {
		name, text, query, with string
		one, all                string
	}{
		{"not a word boundary", "ba", `\Ba`, "X", "bX", "bX"},
		{"word boundary", "ab b", `\bb`, "X", "ab X", "ab X"},
		{"start of line", "aa\naa", `^a`, "X", "Xa\naa", "Xa\nXa"},
		{"end of line", "aa\naa", `a$`, "X", "aX\naa", "aX\naX"},
		{"groups", "k=v k=w", `(\w)=(\w)\b`, "$2=$1", "v=k k=w", "v=k w=k"},
		{"what follows a match changes", "a1a1", `a\d`, "$0$0", "a1a1a1", "a1a1a1a1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Controller
			c.SetText(tt.text)
			if _, err := c.Find(tt.query, FindOptions{Regexp: true}); err != nil {
				t.Fatal(err)
			}
			if !c.FindNext() {
				t.Fatal("no match")
			}
			if !c.ReplaceMatch(tt.with) || c.Text() != tt.one {
				t.Errorf("ReplaceMatch: got %q, want %q", c.Text(), tt.one)
			}
			c.SetText(tt.text)
			c.Find(tt.query, FindOptions{Regexp: true}) //nolint:errcheck // compiled above
			c.ReplaceAll(tt.with)
			if c.Text() != tt.all {
				t.Errorf("ReplaceAll: got %q, want %q", c.Text(), tt.all)
			}
			c.Undo()
			if c.Text() != tt.text {
				t.Errorf("undo of ReplaceAll: got %q, want %q", c.Text(), tt.text)
			}
		})
	}
}

// walk presses key until the caret stops or steps run out, and returns the
// carets it passed.
func (h *editorHarness) walk(key input.KeyCode, steps int) []TextPosition {
	var seen []TextPosition
	for range steps {
		before := h.ctl.Selection().End
		h.Key(key, 0)
		if h.ctl.Selection().End == before {
			break
		}
		seen = append(seen, h.ctl.Selection().End)
	}
	return seen
}

func TestTextEditor_ArrowsFollowTheScreenInRTLLines(t *testing.T) {
	var ctl Controller
	// Each Hebrew letter is two bytes.
	ctl.SetText("אב\nגד")
	h := newEditor(t, &ctl, Editor{})
	if h.view().lineLayout(0).Direction() != text.DirectionRTL {
		t.Skip("no font for Hebrew")
	}
	// The start of a right-to-left line is its right end: left goes inward
	// and on to the next line, which reads on from the left end.
	ctl.SetSelection(TextRange{})
	got := h.walk(input.KeyLeft, 10)
	want := []TextPosition{pos(0, 2), pos(0, 4), pos(1, 0), pos(1, 2), pos(1, 4)}
	if !slices.Equal(got, want) {
		t.Errorf("left from the start: %v, want %v", got, want)
	}
	got = h.walk(input.KeyRight, 10)
	want = []TextPosition{pos(1, 2), pos(1, 0), pos(0, 4), pos(0, 2), pos(0, 0)}
	if !slices.Equal(got, want) {
		t.Errorf("right from the end: %v, want %v", got, want)
	}
}

func TestTextEditor_ArrowsCrossMixedLines(t *testing.T) {
	// A line that ends against its direction, or lies between lines of the
	// other direction, must not hold the caret: the arrow that reads on reaches
	// the end of the text and the other one comes back to the start.
	tests := []struct {
		content string
		rtl     bool
	}{
		{"ab\nאב\ncd", false},
		{"abc אב\nx", false},
		{"x\nאב abc\nx", false},
		{"x\nאב\nגד\nx", false},
		{"אב\n\nגד", true},
		{"אב\nabc\nגד", true},
	}
	for _, tt := range tests {
		var ctl Controller
		ctl.SetText(tt.content)
		h := newEditor(t, &ctl, Editor{})
		if h.view().lineLayout(0).Direction() == text.DirectionRTL != tt.rtl {
			t.Skip("no font for Hebrew")
		}
		on, back := input.KeyRight, input.KeyLeft
		if tt.rtl {
			on, back = back, on
		}
		last := ctl.LineCount() - 1
		end := pos(last, len(ctl.Line(last)))
		ctl.SetSelection(TextRange{})
		h.walk(on, 40)
		if got := ctl.Selection().End; got != end {
			t.Errorf("%q: reading on ended at %v, want %v", tt.content, got, end)
		}
		h.walk(back, 40)
		if got := ctl.Selection().End; got != pos(0, 0) {
			t.Errorf("%q: going back ended at %v, want 0:0", tt.content, got)
		}
	}
}
