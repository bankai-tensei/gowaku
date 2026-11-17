package layout

import "testing"

// The tables below run past 2^24 pixels, where a float32 holds only every
// other whole pixel and further down only every 32nd (ADR 0150).

func TestExtentTable_RowsFarDownSitOnTheirPixels(t *testing.T) {
	const n = 3_000_000
	var tab ExtentTable
	tab.Reset(n, func(i int) float32 { return float32(100 + i%3) })
	// Rows 3k, 3k+1 and 3k+2 are 100, 101 and 102 high: 303 a triple.
	for _, i := range []int{n - 1, n - 300, 1_500_001} {
		want := float64(i/3)*303 + []float64{0, 100, 201}[i%3]
		if got := tab.Offset(i); got != want {
			t.Errorf("Offset(%d) = %v, want %v", i, got, want)
		}
		for _, y := range []float64{want, want + 0.5, want + float64(tab.Extent(i)) - 0.5} {
			if got := tab.IndexAt(y); got != i {
				t.Errorf("IndexAt(%v) = %d, want %d", y, got, i)
			}
		}
	}
	if got, want := tab.Total(), float64(n/3)*303; got != want {
		t.Errorf("Total = %v, want %v", got, want)
	}
}

func TestMeasuredExtents_RowsFarDownSitOnTheirPixels(t *testing.T) {
	const n = 3_000_000
	var m MeasuredExtents
	m.Resize(n)
	m.SetEstimate(100, false)
	m.Set(n-2, 140)
	i := n - 1
	want := float64(n-2)*100 + 140
	if got := m.Offset(i); got != want {
		t.Fatalf("Offset(%d) = %v, want %v", i, got, want)
	}
	for _, y := range []float64{want, want + 0.5, want + 99.5} {
		if got := m.IndexAt(y); got != i {
			t.Errorf("IndexAt(%v) = %d, want %d", y, got, i)
		}
	}
	if got := m.IndexAt(want - 0.5); got != n-2 {
		t.Errorf("IndexAt(%v) = %d, want %d", want-0.5, got, n-2)
	}
}

func TestScrollOffset_HoldsAPixelFarDown(t *testing.T) {
	var s ScrollOffset
	base := float64(400_000_000)
	s.Set(base)
	s.Set(base + 1)
	if got := s.Value() - base; got != 1 {
		t.Errorf("a move of 1 at %v keeps %v", base, got)
	}
}
