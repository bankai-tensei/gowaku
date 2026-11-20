package editor

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// TestIndexASCIIFold_AgreesWithLowering compares the search that folds ASCII
// case in place with the lowered copy it replaced, on random lines of a small
// alphabet so that near misses and overlaps are common.
func TestIndexASCIIFold_AgreesWithLowering(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	const alphabet = "aAbB-1 "
	word := func(n int) string {
		var b strings.Builder
		for range n {
			b.WriteByte(alphabet[r.IntN(len(alphabet))])
		}
		return b.String()
	}
	for range 20000 {
		s := word(r.IntN(24))
		q := strings.ToLower(word(1 + r.IntN(4)))
		if got, want := indexASCIIFold(s, q), strings.Index(strings.ToLower(s), q); got != want {
			t.Fatalf("indexASCIIFold(%q, %q) = %d, want %d", s, q, got, want)
		}
	}
}

func TestFind_FoldsCaseInALogWithoutCopyingLines(t *testing.T) {
	var ctl Controller
	ctl.SetText("INFO ok\nError: disk\nwarn\nERROR again error\n")
	n, err := ctl.Find("error", FindOptions{})
	if err != nil || n != 3 {
		t.Fatalf("Find = %d, %v; want 3 matches", n, err)
	}
	want := []TextRange{
		{Start: TextPosition{1, 0}, End: TextPosition{1, 5}},
		{Start: TextPosition{3, 0}, End: TextPosition{3, 5}},
		{Start: TextPosition{3, 12}, End: TextPosition{3, 17}},
	}
	for k, m := range ctl.find.matches {
		if m != want[k] {
			t.Errorf("match %d = %v, want %v", k, m, want[k])
		}
	}
}

// BenchmarkFind_FoldCase1M searches a log of a million ASCII lines without
// regard to case, as Ctrl+F does by default, for a word on one line in a
// thousand. The text is over bigSearch, so that Controller.Find hands it to
// a goroutine of its own; the benchmark measures the search itself.
func BenchmarkFind_FoldCase1M(b *testing.B) {
	var ctl Controller
	ctl.SetText(probeLogForBench(1_000_000))
	var f finder
	if err := f.set("error", FindOptions{}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		f.all(&ctl.buf)
		if len(f.matches) != 1000 {
			b.Fatalf("%d matches, want 1000", len(f.matches))
		}
	}
}

func probeLogForBench(n int) string {
	var b strings.Builder
	for i := range n {
		lvl := "INFO "
		if i%1000 == 0 {
			lvl = "ERROR"
		}
		b.WriteString("2026-11-17T10:00:00.123Z ")
		b.WriteString(lvl)
		b.WriteString(" server request handled path=/api/v1/items status=200 dur=12ms\n")
	}
	return b.String()
}
