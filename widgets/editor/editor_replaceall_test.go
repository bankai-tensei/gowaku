package editor

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// manyMatches returns n lines, each with one x between text that is kept.
func manyMatches(n int) string {
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("keep x keep")
	}
	return b.String()
}

// TestReplaceAll_ManyMatches replaces maxMatches matches with text that adds a
// line each, which cost a move of every line after each match one at a time.
func TestReplaceAll_ManyMatches(t *testing.T) {
	text := manyMatches(maxMatches)
	var ctl Controller
	ctl.SetText(text)
	if n, err := ctl.Find("x", FindOptions{}); n != maxMatches || err != nil {
		t.Fatalf("%d matches, %v", n, err)
	}
	start := time.Now()
	if n := ctl.ReplaceAll("y\nz"); n != maxMatches {
		t.Fatalf("ReplaceAll replaced %d", n)
	}
	took := time.Since(start)
	want := strings.ReplaceAll(text, "x", "y\nz")
	if ctl.Text() != want {
		t.Fatal("ReplaceAll: wrong text")
	}
	if took > 5*time.Second {
		t.Errorf("ReplaceAll of %d matches took %v", maxMatches, took)
	}
	ctl.Undo()
	if ctl.Text() != text {
		t.Fatal("one Undo must take back ReplaceAll")
	}
	ctl.Redo()
	if ctl.Text() != want {
		t.Fatal("Redo of ReplaceAll: wrong text")
	}
}

// TestReplaceAll_ManyMatchesInASource replaces matches all over a file, which
// cut its piece around each of them one at a time.
func TestReplaceAll_ManyMatchesInASource(t *testing.T) {
	text := manyMatches(maxMatches)
	var ctl Controller
	postNow(&ctl)
	ctl.SetSource(newMemSource(text))
	if _, err := ctl.Find("x", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFind(t, &ctl)
	start := time.Now()
	if n := ctl.ReplaceAll("yy"); n != maxMatches {
		t.Fatalf("ReplaceAll replaced %d", n)
	}
	took := time.Since(start)
	waitFind(t, &ctl)
	want := strings.ReplaceAll(text, "x", "yy")
	if ctl.Text() != want {
		t.Fatal("ReplaceAll: wrong text")
	}
	if took > 5*time.Second {
		t.Errorf("ReplaceAll of %d matches took %v", maxMatches, took)
	}
	ctl.Undo()
	if ctl.Text() != text {
		t.Fatal("one Undo must take back ReplaceAll")
	}
}

// replaceOneByOne replaces the matches of c one at a time from the last, as
// ReplaceAll once did, for a reference.
func replaceOneByOne(c *Controller, with func(k int) string) {
	matches := append([]TextRange(nil), c.find.matches...)
	query := c.find.query
	c.find.query = ""
	for k := len(matches) - 1; k >= 0; k-- {
		c.splice(matches[k].Start, matches[k].End, with(k))
	}
	c.find.query = query
	c.refind()
}

// TestReplaceAll_AsOneByOne compares ReplaceAll with replacing each match by
// itself: the text, the pieces of a source and its edits, and where the
// selections of other views go.
func TestReplaceAll_AsOneByOne(t *testing.T) {
	texts := []string{
		"a x b x\nx\n\nccc x x x\nlast x",
		strings.Repeat("one x two\nnothing\nnothing\nnothing\nnothing\nnothing\nx x\n", 50),
	}
	opts := []struct {
		query string
		opts  FindOptions
		with  string
	}{
		{"x", FindOptions{}, "Y"},
		{"x", FindOptions{}, ""},
		{"x", FindOptions{}, "1\n2\r\n3"},
		{`(x)`, FindOptions{Regexp: true}, "<$1>\n"},
		{`\bx\b`, FindOptions{Regexp: true}, "[$0]"},
	}
	for _, text := range texts {
		for _, src := range []bool{false, true} {
			for _, o := range opts {
				set := func(c *Controller) {
					if src {
						c.SetSource(newMemSource(text))
					} else {
						c.SetText(text)
					}
					postNow(c)
					if _, err := c.Find(o.query, o.opts); err != nil {
						t.Fatal(err)
					}
					waitFind(t, c)
				}
				var got, want Controller
				set(&got)
				set(&want)
				with := want.find.expand(&want.buf, want.find.matches, o.with)
				replaceOneByOne(&want, func(k int) string {
					if with != nil {
						return with[k]
					}
					return o.with
				})
				got.ReplaceAll(o.with)
				waitFind(t, &got)
				waitFind(t, &want)
				name := o.query + " -> " + o.with
				if got.Text() != want.Text() {
					t.Fatalf("%s: %q, want %q", name, got.Text(), want.Text())
				}
				if len(got.buf.pieces) != len(want.buf.pieces) {
					t.Errorf("%s: %d pieces, want %d", name, len(got.buf.pieces), len(want.buf.pieces))
				}
				if g, w := len(got.Edits()), len(want.Edits()); g != w {
					t.Errorf("%s: %d edits, want %d", name, g, w)
				}
				sameMatches(t, got.Matches(), want.Matches())
				got.Undo()
				if got.Text() != text {
					t.Fatalf("%s: undo %q", name, got.Text())
				}
			}
		}
	}
}

// TestReplaced_MovesPositionsAsEachReplace moves every position of a text
// through the matches as shiftEdited moves it through each replace.
func TestReplaced_MovesPositionsAsEachReplace(t *testing.T) {
	text := "x ab x\nxx\n\nc x x"
	for _, with := range []string{"", "Y", "YYY", "1\n22", "\n"} {
		var ctl Controller
		ctl.SetText(text)
		ctl.Find("x", FindOptions{}) //nolint:errcheck // plain
		matches := append([]TextRange(nil), ctl.find.matches...)
		_, _, ends := ctl.replaceLines(matches, nil, with)
		var positions []TextPosition
		for i, l := range strings.Split(text, "\n") {
			for col := 0; col <= len(l); col++ {
				positions = append(positions, TextPosition{i, col})
			}
		}
		for _, p := range positions {
			q := p
			var b lineBuffer
			b.reset(text)
			for k := len(matches) - 1; k >= 0; k-- {
				end, _ := b.replace(matches[k].Start, matches[k].End, with)
				q = shiftEdited(q, matches[k].Start, matches[k].End, end)
			}
			if got := moveReplaced(p, matches, ends); got != q {
				t.Errorf("with %q: %v moved to %v, want %v", with, p, got, q)
			}
		}
	}
}

// TestReplaceAll_OtherViewsAsOneByOne compares what another view keeps after
// ReplaceAll with what it keeps after each match is replaced by itself: its
// selection, where it scrolled and the rows it counts for each line.
func TestReplaceAll_OtherViewsAsOneByOne(t *testing.T) {
	long := strings.Repeat("words that wrap ", 8)
	var b strings.Builder
	for i := range 400 {
		switch i % 3 {
		case 0:
			b.WriteString("x " + long + " x\n")
		case 1:
			b.WriteString("nothing here\n")
		default:
			b.WriteString(long + "\n")
		}
	}
	text := b.String()
	for _, wrap := range []bool{false, true} {
		for _, with := range []string{"Y", "1\n2", ""} {
			run := func(all bool) (*View, ScrollPosition, []int32) {
				var ctl Controller
				var left, right View
				ctl.SetText(text)
				right.SetScroll(ScrollPosition{Line: 200, Offset: 1})
				h := newSplit(t, &ctl, &left, &right, Editor{Wrap: wrap})
				h.focus(&left)
				right.SetSelection(TextRange{Start: pos(201, 3), End: pos(300, 1)})
				if _, err := ctl.Find("x", FindOptions{}); err != nil {
					t.Fatal(err)
				}
				if all {
					ctl.ReplaceAll(with)
				} else {
					replaceOneByOne(&ctl, func(int) string { return with })
				}
				// The rows as the edits left them, before a layout counts some.
				rows := slices.Clone(right.r.rows.rows)
				h.frame()
				return &right, right.Scroll(), rows
			}
			got, gotScroll, gotRows := run(true)
			want, wantScroll, wantRows := run(false)
			name := map[bool]string{false: "nowrap", true: "wrap"}[wrap] + " " + with
			if got.Selection() != want.Selection() {
				t.Errorf("%s: selection %v, want %v", name, got.Selection(), want.Selection())
			}
			if gotScroll != wantScroll {
				t.Errorf("%s: scrolled to %+v, want %+v", name, gotScroll, wantScroll)
			}
			if !slices.Equal(gotRows, wantRows) || !slices.Equal(got.r.rows.rows, want.r.rows.rows) || got.r.rows.n != want.r.rows.n {
				t.Errorf("%s: the rows differ", name)
			}
		}
	}
}

// TestReplaceAll_ManyMatchesWrapped replaces maxMatches matches with lines in
// an editor that wraps, whose rows were counted again for each.
func TestReplaceAll_ManyMatchesWrapped(t *testing.T) {
	var ctl Controller
	var v View
	ctl.SetText(manyMatches(maxMatches))
	h := newEditor(t, &ctl, Editor{View: &v, Wrap: true})
	if n, err := ctl.Find("x", FindOptions{}); n != maxMatches || err != nil {
		t.Fatalf("%d matches, %v", n, err)
	}
	start := time.Now()
	ctl.ReplaceAll("y\nz")
	h.frame()
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("ReplaceAll of %d matches took %v", maxMatches, took)
	}
	if n := ctl.LineCount(); n != 2*maxMatches {
		t.Errorf("%d lines", n)
	}
}
