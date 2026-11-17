package layout

// ExtentTable holds the heights of the rows of a list whose rows differ in
// height, as the offset of the top of each row from the top of the first. It
// finds the row at an offset by binary search, so a scroll through any number
// of rows costs O(log n) and does not allocate. It keeps 8 bytes per row: the
// offsets are float64, so that the rows far down a list of millions still sit
// on whole pixels.
//
// The zero value is an empty table. See docs/ADR/0034-varying-list-extents.md.
type ExtentTable struct {
	// offsets[i] is the top of row i, and offsets[n] the bottom of the last.
	offsets []float64
	version uint64
}

// Reset reads the heights of n rows from extent, reusing the memory of the
// table when it is large enough. It panics if a height is negative.
func (t *ExtentTable) Reset(n int, extent func(i int) float32) {
	if cap(t.offsets) < n+1 {
		t.offsets = make([]float64, n+1)
	}
	t.offsets = t.offsets[:n+1]
	var top float64
	for i := range n {
		t.offsets[i] = top
		e := extent(i)
		if !(e >= 0) {
			panic("gowaku: ListView.ItemExtentOf returned a negative or NaN height")
		}
		top += float64(e)
	}
	t.offsets[n] = top
	t.version++
}

// Version changes on each Reset, so that a render object can tell that the
// heights may have changed.
func (t *ExtentTable) Version() uint64 { return t.version }

// Len returns the number of rows.
func (t *ExtentTable) Len() int { return max(len(t.offsets)-1, 0) }

// Total returns the height of all the rows.
func (t *ExtentTable) Total() float64 {
	if len(t.offsets) == 0 {
		return 0
	}
	return t.offsets[len(t.offsets)-1]
}

// Offset returns the top of row i, for 0 <= i <= Len; Offset(Len()) is Total.
func (t *ExtentTable) Offset(i int) float64 {
	if i == 0 && len(t.offsets) == 0 {
		return 0 // the zero value has no offsets, and its Total is 0
	}
	return t.offsets[i]
}

// Extent returns the height of row i.
func (t *ExtentTable) Extent(i int) float32 { return float32(t.offsets[i+1] - t.offsets[i]) }

// IndexAt returns the row that covers y: the last row whose top is at or
// above y, clamped to the rows that exist. An empty table returns 0.
func (t *ExtentTable) IndexAt(y float64) int {
	n := t.Len()
	if n == 0 {
		return 0
	}
	// Find the first row whose top is below y; the row before it covers y.
	lo, hi := 0, n
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if t.offsets[mid] <= y {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return max(lo-1, 0)
}
