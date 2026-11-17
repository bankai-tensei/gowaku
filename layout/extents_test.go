package layout

import (
	"strings"
	"testing"
)

// mixedHeight is a header, then sections of a title and nine rows.
func mixedHeight(i int) float32 {
	switch {
	case i == 0:
		return 32
	case i%10 == 1:
		return 24
	default:
		return 44
	}
}

func TestExtentTable_OffsetsAndIndex(t *testing.T) {
	var tab ExtentTable
	tab.Reset(25, mixedHeight)
	if tab.Len() != 25 {
		t.Fatalf("Len = %d", tab.Len())
	}
	var top float64
	for i := range 25 {
		if got := tab.Offset(i); got != top {
			t.Errorf("Offset(%d) = %v, want %v", i, got, top)
		}
		if got := tab.Extent(i); got != mixedHeight(i) {
			t.Errorf("Extent(%d) = %v, want %v", i, got, mixedHeight(i))
		}
		// The top, the middle and the last point of a row all belong to it.
		for _, y := range []float64{top, top + float64(mixedHeight(i))/2, top + float64(mixedHeight(i)) - 0.5} {
			if got := tab.IndexAt(y); got != i {
				t.Errorf("IndexAt(%v) = %d, want %d", y, got, i)
			}
		}
		top += float64(mixedHeight(i))
	}
	if tab.Total() != top || tab.Offset(25) != top {
		t.Errorf("Total = %v, want %v", tab.Total(), top)
	}
	for _, c := range []struct {
		y    float64
		want int
	}{{-100, 0}, {0, 0}, {top, 24}, {top + 1000, 24}} {
		if got := tab.IndexAt(c.y); got != c.want {
			t.Errorf("IndexAt(%v) = %d, want %d", c.y, got, c.want)
		}
	}
}

func TestExtentTable_ZeroHeightRows(t *testing.T) {
	var tab ExtentTable
	tab.Reset(5, func(i int) float32 { return float32(i%2) * 10 }) // 0 10 0 10 0
	if tab.Total() != 20 {
		t.Errorf("Total = %v", tab.Total())
	}
	// A hidden row covers nothing; the next one with a height is found.
	if got := tab.IndexAt(0); got != 1 {
		t.Errorf("IndexAt(0) = %d, want 1", got)
	}
	if got := tab.IndexAt(10); got != 3 {
		t.Errorf("IndexAt(10) = %d, want 3", got)
	}
}

func TestExtentTable_Empty(t *testing.T) {
	var tab ExtentTable
	if tab.Len() != 0 || tab.Total() != 0 || tab.IndexAt(50) != 0 {
		t.Error("the zero table must be empty")
	}
	tab.Reset(0, mixedHeight)
	if tab.Len() != 0 || tab.Total() != 0 || tab.IndexAt(50) != 0 {
		t.Error("a table of no rows must be empty")
	}
}

func TestExtentTable_ResetChangesVersionAndReusesMemory(t *testing.T) {
	var tab ExtentTable
	tab.Reset(10000, mixedHeight)
	v := tab.Version()
	allocs := testing.AllocsPerRun(10, func() { tab.Reset(5000, mixedHeight) })
	if allocs != 0 {
		t.Errorf("a smaller reset allocated %v times", allocs)
	}
	if tab.Version() == v {
		t.Error("Reset must change the version")
	}
	if tab.Len() != 5000 {
		t.Errorf("Len = %d", tab.Len())
	}
}

func TestExtentTable_NegativeHeightPanics(t *testing.T) {
	defer func() {
		got, _ := recover().(string)
		if !strings.Contains(got, "gowaku: ListView.ItemExtentOf returned a negative") {
			t.Errorf("got panic %q", got)
		}
	}()
	var tab ExtentTable
	tab.Reset(3, func(i int) float32 { return float32(1 - i) })
}

func TestExtentTable_ManyRowsKeepPrecision(t *testing.T) {
	var tab ExtentTable
	const n = 1_000_000
	tab.Reset(n, func(int) float32 { return 0.1 })
	// A float32 running sum would drift by thousands of pixels here.
	if got, want := tab.Total(), float64(n*0.1); got < want-1 || got > want+1 {
		t.Errorf("Total = %v, want about %v", got, want)
	}
}

func BenchmarkExtentTable_IndexAt_10k(b *testing.B) {
	var tab ExtentTable
	tab.Reset(10000, mixedHeight)
	total := tab.Total()
	y := float64(0)
	b.ReportAllocs()
	for b.Loop() {
		y += 37
		if y > total {
			y = 0
		}
		_ = tab.IndexAt(y)
	}
}

// The zero value is an empty table: every method the doc gives a meaning on
// an empty table answers without a Reset, and without allocating.
func TestExtentTable_ZeroValue(t *testing.T) {
	var tab ExtentTable
	if tab.Len() != 0 || tab.Total() != 0 {
		t.Errorf("Len = %d, Total = %v, want 0, 0", tab.Len(), tab.Total())
	}
	if got := tab.Offset(tab.Len()); got != tab.Total() {
		t.Errorf("Offset(Len()) = %v, want Total() = %v", got, tab.Total())
	}
	if got := tab.IndexAt(10); got != 0 {
		t.Errorf("IndexAt(10) = %d, want 0", got)
	}
	if n := testing.AllocsPerRun(100, func() { _ = tab.Offset(0) }); n != 0 {
		t.Errorf("Offset allocates %v times", n)
	}
	// An index outside 0..Len is still a contract violation.
	for name, f := range map[string]func(){
		"Offset(1)":  func() { tab.Offset(1) },
		"Offset(-1)": func() { tab.Offset(-1) },
		"Extent(0)":  func() { tab.Extent(0) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s on an empty table did not panic", name)
				}
			}()
			f()
		}()
	}
	// A table reset to no rows behaves the same as the zero value.
	tab.Reset(0, nil)
	if got := tab.Offset(0); got != 0 {
		t.Errorf("after Reset(0): Offset(0) = %v, want 0", got)
	}
}
