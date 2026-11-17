package semantics

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestCompare(t *testing.T) {
	old := &Node{ID: 1, Children: []*Node{
		{ID: 2, Role: RoleButton, Label: "OK"},
		{ID: 3, Role: RoleText, Label: "gone"},
	}}
	new := &Node{ID: 1, Children: []*Node{
		{ID: 2, Role: RoleButton, Label: "Done", State: Focused},
		{ID: 4, Role: RoleCheckbox, Label: "new"},
	}}
	var d Diff
	Compare(old, new, 0, 2, &d)
	if len(d.Added) != 1 || d.Added[0].ID != 4 {
		t.Errorf("Added = %v, want node 4", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0] != 3 {
		t.Errorf("Removed = %v, want [3]", d.Removed)
	}
	if !d.FocusMoved {
		t.Error("FocusMoved = false")
	}
	want := map[ID]Change{1: ChangedChildren, 2: ChangedLabel | ChangedState}
	if len(d.Changed) != len(want) {
		t.Fatalf("Changed = %v", d.Changed)
	}
	for _, c := range d.Changed {
		if want[c.Node.ID] != c.What {
			t.Errorf("node %d changed %b, want %b", c.Node.ID, c.What, want[c.Node.ID])
		}
	}

	Compare(new, new, 2, 2, &d)
	if !d.Empty() {
		t.Errorf("same tree: %+v", d)
	}
	Compare(nil, new, 0, 0, &d)
	if len(d.Added) != 3 {
		t.Errorf("from nil: Added %d, want 3", len(d.Added))
	}
}

func TestCompare_PlaceInSet(t *testing.T) {
	a := &Node{ID: 1, Role: RoleTreeItem, Level: 2, PosInSet: 1, SetSize: 3}
	b := *a
	b.PosInSet = 2
	if compareNode(a, &b)&ChangedHint == 0 {
		t.Error("a new place among the siblings must count as a change")
	}
	if compareNode(a, a) != 0 {
		t.Error("the same node must not change")
	}
}

func TestCompareText(t *testing.T) {
	a := &Node{ID: 1, Text: &TextInfo{Text: "abc", SelectionStart: 1, SelectionEnd: 1}}
	b := &Node{ID: 1, Text: &TextInfo{Text: "abc", SelectionStart: 2, SelectionEnd: 2}}
	var d Diff
	Compare(a, b, 0, 0, &d)
	if len(d.Changed) != 1 || d.Changed[0].What != ChangedSelection {
		t.Errorf("caret move: %+v", d.Changed)
	}
	b.Text.LineRects = []geom.Rect{{W: 1}}
	Compare(a, b, 0, 0, &d)
	if d.Changed[0].What != ChangedSelection|ChangedRect {
		t.Errorf("line rect: %b", d.Changed[0].What)
	}
}

func TestComparePosition(t *testing.T) {
	a := &Node{ID: 1, Role: RoleRow, RowIndex: 2}
	b := &Node{ID: 1, Role: RoleRow, RowIndex: 3}
	var d Diff
	Compare(a, b, 0, 0, &d)
	if len(d.Changed) != 1 || d.Changed[0].What != ChangedPosition {
		t.Errorf("a row that moved: %+v", d.Changed)
	}
	Compare(&Node{ID: 1, RowCount: 10}, &Node{ID: 1, RowCount: 11}, 0, 0, &d)
	if len(d.Changed) != 1 || d.Changed[0].What != ChangedPosition {
		t.Errorf("a grid that grew: %+v", d.Changed)
	}
}

func TestCompare_Sort(t *testing.T) {
	a := &Node{ID: 1, Role: RoleColumnHeader, Sort: SortAscending}
	b := &Node{ID: 1, Role: RoleColumnHeader, Sort: SortDescending}
	if compareNode(a, b) != ChangedState {
		t.Errorf("a header sorted the other way: %b, want ChangedState", compareNode(a, b))
	}
	for s, want := range map[Sort]string{SortNone: "none", SortAscending: "ascending", SortDescending: "descending"} {
		if s.String() != want {
			t.Errorf("%d.String() = %q, want %q", s, s.String(), want)
		}
	}
}

func TestWords(t *testing.T) {
	cases := []struct {
		text  string
		words []string
	}{
		{"hello, world  ok", []string{"hello", ", ", "world  ", "ok"}},
		{"日本語のテキスト", []string{"日本語", "の", "テキスト"}},
		{"Go言語😀です", []string{"Go", "言語", "😀", "です"}},
	}
	for _, c := range cases {
		s := UTF16(c.text)
		var got []string
		start := 0
		for i := 1; i <= len(s); i++ {
			if IsWordStart(s, i) {
				got = append(got, string(utf16Decode(s[start:i])))
				start = i
			}
		}
		if len(got) != len(c.words) {
			t.Errorf("%q: words %q, want %q", c.text, got, c.words)
			continue
		}
		for i := range got {
			if got[i] != c.words[i] {
				t.Errorf("%q: words %q, want %q", c.text, got, c.words)
				break
			}
		}
	}
}

func TestLines(t *testing.T) {
	ti := &TextInfo{Text: "ab\ncd\nef", LineStarts: []int{3, 6}}
	if n := ti.Lines(); n != 3 {
		t.Fatalf("Lines = %d", n)
	}
	for off, want := range map[int]int{0: 0, 2: 0, 3: 1, 5: 1, 6: 2, 8: 2} {
		if got := ti.Line(off); got != want {
			t.Errorf("Line(%d) = %d, want %d", off, got, want)
		}
	}
	if s, e := ti.LineBounds(1, 8); s != 3 || e != 6 {
		t.Errorf("LineBounds(1) = %d, %d", s, e)
	}
	if s, e := ti.LineBounds(2, 8); s != 6 || e != 8 {
		t.Errorf("LineBounds(2) = %d, %d", s, e)
	}
}

func utf16Decode(s []uint16) []rune {
	var out []rune
	for i := 0; i < len(s); i++ {
		r := rune(s[i])
		if r >= 0xD800 && r < 0xDC00 && i+1 < len(s) {
			r = (r-0xD800)<<10 + (rune(s[i+1]) - 0xDC00) + 0x10000
			i++
		}
		out = append(out, r)
	}
	return out
}
