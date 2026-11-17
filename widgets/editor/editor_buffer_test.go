package editor

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func TestLineBuffer_SplitsEveryLineBreak(t *testing.T) {
	var b lineBuffer
	b.reset("a\r\nb\rc\nd")
	if b.lineCount() != 4 || b.text() != "a\nb\nc\nd" {
		t.Errorf("got %d lines, %q", b.lineCount(), b.text())
	}
	var empty lineBuffer
	if empty.lineCount() != 1 || empty.line(0) != "" || empty.text() != "" || empty.length() != 0 {
		t.Error("the zero buffer must be one empty line")
	}
}

func TestLineBuffer_ReplaceAcrossLines(t *testing.T) {
	cases := []struct {
		name    string
		a, z    TextPosition
		s, want string
		end     TextPosition
		ch      lineChange
	}{
		{"1 行の中", TextPosition{1, 1}, TextPosition{1, 2}, "X", "abc\ndXf\nghi", TextPosition{1, 2}, lineChange{1, 1, 1}},
		{"行をつなぐ", TextPosition{0, 3}, TextPosition{1, 0}, "", "abcdef\nghi", TextPosition{0, 3}, lineChange{0, 2, 1}},
		{"行を分ける", TextPosition{1, 1}, TextPosition{1, 1}, "\n", "abc\nd\nef\nghi", TextPosition{2, 0}, lineChange{1, 1, 2}},
		{"複数行を複数行に", TextPosition{0, 1}, TextPosition{2, 1}, "1\n2\n3", "a1\n2\n3hi", TextPosition{2, 1}, lineChange{0, 3, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var b lineBuffer
			b.reset("abc\ndef\nghi")
			end, ch := b.replace(c.a, c.z, c.s)
			if got := b.text(); got != c.want {
				t.Errorf("text %q, want %q", got, c.want)
			}
			if end != c.end || ch != c.ch {
				t.Errorf("end %v change %v, want %v %v", end, ch, c.end, c.ch)
			}
		})
	}
}

func TestLineBuffer_OffsetsFollowEdits(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	var b lineBuffer
	b.reset(numbered(200))
	for range 300 {
		text := b.text()
		off := rng.IntN(len(text) + 1)
		for off > 0 && off < len(text) && !startsRune(text[off]) {
			off--
		}
		p := b.position(off)
		if got := b.offset(p); got != off {
			t.Fatalf("offset %d became %v and %d", off, p, got)
		}
		ins := []string{"x", "\n", "ab\ncd", ""}[rng.IntN(4)]
		z := b.position(min(off+rng.IntN(5), len(text)))
		end, _ := b.replace(p, z, ins)
		want := text[:off] + ins + text[b2o(text, z):]
		if b.text() != want {
			t.Fatalf("text diverged after replacing at %v", p)
		}
		if b.offset(end) != off+len(ins) {
			t.Fatalf("end %v is at %d, want %d", end, b.offset(end), off+len(ins))
		}
	}
}

// b2o returns the offset of p in text.
func b2o(text string, p TextPosition) int {
	lines := strings.Split(text, "\n")
	off := 0
	for i := range p.Line {
		off += len(lines[i]) + 1
	}
	return off + p.Column
}

func TestLineBuffer_ClampKeepsToCharacters(t *testing.T) {
	var b lineBuffer
	b.reset("aé\nb")
	cases := []struct{ in, want TextPosition }{
		{TextPosition{0, 2}, TextPosition{0, 1}},
		{TextPosition{0, 9}, TextPosition{0, 3}},
		{TextPosition{5, 0}, TextPosition{1, 1}},
		{TextPosition{-1, 3}, TextPosition{0, 0}},
	}
	for _, c := range cases {
		if got := b.clamp(c.in); got != c.want {
			t.Errorf("clamp(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRowTable_FindsLinesAndRows(t *testing.T) {
	var rt rowTable
	counts := []int32{1, 3, 1, 2, 5, 1}
	rt.reset(len(counts), false, func(i int) int32 { return counts[i] })
	row := 0
	for i, n := range counts {
		if got := rt.before(i); got != row {
			t.Errorf("before(%d) = %d, want %d", i, got, row)
		}
		for k := range int(n) {
			line, first := rt.find(row + k)
			if line != i || first != row {
				t.Errorf("find(%d) = %d, %d; want %d, %d", row+k, line, first, i, row)
			}
		}
		row += int(n)
	}
	if rt.total() != row {
		t.Errorf("total %d, want %d", rt.total(), row)
	}
	if line, _ := rt.find(1000); line != len(counts)-1 {
		t.Errorf("a row past the end found line %d", line)
	}
	rt.set(1, 1, true)
	if rt.before(2) != 2 || rt.total() != row-2 {
		t.Errorf("set did not update the sums: %d %d", rt.before(2), rt.total())
	}
	rt.splice(1, 2, 3, func(int) int32 { return 2 })
	if len(rt.rows) != 7 || rt.before(4) != 1+6 {
		t.Errorf("splice: %v, before(4) = %d", rt.rows, rt.before(4))
	}
}

func TestRowTable_UniformHoldsOnlyTheCount(t *testing.T) {
	var rt rowTable
	rt.reset(100_000_000, true, nil)
	if rt.rows != nil || rt.total() != 100_000_000 || rt.before(42) != 42 || rt.at(7) != 1 {
		t.Fatalf("uniform table: total %d, before(42) %d", rt.total(), rt.before(42))
	}
	if line, first := rt.find(99_999_999); line != 99_999_999 || first != line {
		t.Errorf("find = %d, %d", line, first)
	}
	if line, _ := rt.find(1 << 40); line != 99_999_999 {
		t.Errorf("a row past the end finds line %d", line)
	}
	rt.splice(10, 3, 1, nil)
	if rt.count() != 99_999_998 {
		t.Errorf("after a splice: %d lines", rt.count())
	}
}
