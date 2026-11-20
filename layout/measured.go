package layout

import "math/bits"

// unmeasured marks a row of MeasuredExtents whose height is not known.
const unmeasured = -1

// MeasuredExtents holds the heights of the rows of a list that learns them by
// laying the rows out, as rows of wrapping text do. A row that has not been
// laid out yet counts as the estimate, which is either a fixed height or the
// mean of the rows measured so far.
//
// The heights sit in Fenwick trees, so that the top of a row, the row at an
// offset, a new measurement and a forgotten one each cost O(log n), whatever
// the number of rows, and none of them allocates. It keeps 16 bytes per row.
//
// The zero value is an empty table. See docs/ADR/0078-measured-list-extents.md.
type MeasuredExtents struct {
	// extents[i] is the measured height of row i, or unmeasured.
	extents []float32
	// sums and counts are Fenwick trees, from 1, of the measured heights and
	// of the number of measured rows.
	sums   []float64
	counts []int32
	// total and measured are the sum and number of all the measured rows.
	total    float64
	measured int
	// estimate is the height of a row not measured yet, and average makes it
	// follow the mean of the measured rows once there are some.
	estimate float32
	average  bool
}

// Resize sets the number of rows to n. The rows below both lengths keep their
// heights; new rows are not measured. Rows come and go at the end, so adding
// k rows costs O(k log n) and dropping any number costs O(log n): a list of a
// hundred million rows that grows by one does not walk them all. Memory grows
// by doubling, so that a list that grows a row at a time does not copy itself
// each time.
func (m *MeasuredExtents) Resize(n int) {
	n = max(n, 0)
	old := len(m.extents)
	if n == old {
		return
	}
	if n < old {
		// A node of the trees holds only rows at or before it, so the first
		// n+1 nodes stay right; the totals become the sums of the rows kept.
		m.extents = m.extents[:n]
		m.sums = m.sums[:n+1]
		m.counts = m.counts[:n+1]
		m.total, m.measured = m.prefix(n)
		return
	}
	if cap(m.extents) < n {
		c := max(n, 2*cap(m.extents))
		extents := make([]float32, n, c)
		sums := make([]float64, n+1, c+1)
		counts := make([]int32, n+1, c+1)
		copy(extents, m.extents)
		copy(sums, m.sums)
		copy(counts, m.counts)
		m.extents, m.sums, m.counts = extents, sums, counts
	} else {
		m.extents = m.extents[:n]
		m.sums = m.sums[:n+1]
		m.counts = m.counts[:n+1]
	}
	for k := old + 1; k <= n; k++ {
		m.extents[k-1] = unmeasured
		// Node k holds the rows after k-k&-k up to k. Row k is not measured,
		// so the node is the sum of the nodes below it in that span.
		var sum float64
		var count int32
		for j := k - 1; j > k-k&-k; j -= j & -j {
			sum += m.sums[j]
			count += m.counts[j]
		}
		m.sums[k], m.counts[k] = sum, count
	}
}

// prefix returns the sum and the number of the measured heights of the rows
// before row i.
func (m *MeasuredExtents) prefix(i int) (float64, int) {
	var sum float64
	var count int32
	for k := i; k > 0; k -= k & -k {
		sum += m.sums[k]
		count += m.counts[k]
	}
	return sum, int(count)
}

// SetEstimate sets the height of a row not measured yet. With average set, the
// estimate is the mean of the measured rows, and e only holds until a row is
// measured.
func (m *MeasuredExtents) SetEstimate(e float32, average bool) {
	m.estimate, m.average = max(e, 0), average
}

// Estimate returns the height that a row not measured yet counts as.
func (m *MeasuredExtents) Estimate() float32 {
	if m.average && m.measured > 0 {
		return float32(m.total / float64(m.measured))
	}
	return m.estimate
}

// Len returns the number of rows.
func (m *MeasuredExtents) Len() int { return len(m.extents) }

// Measured reports whether row i has a measured height.
func (m *MeasuredExtents) Measured(i int) bool { return m.extents[i] >= 0 }

// MeasuredCount returns the number of rows with a measured height.
func (m *MeasuredExtents) MeasuredCount() int { return m.measured }

// Set records that row i is h high. It panics if h is negative or NaN.
func (m *MeasuredExtents) Set(i int, h float32) {
	if !(h >= 0) {
		panic("gowaku: a ListView row measured a negative or NaN height")
	}
	old := m.extents[i]
	if old == h {
		return
	}
	dsum, dcount := float64(h), int32(1)
	if old >= 0 {
		dsum, dcount = float64(h)-float64(old), 0
	}
	m.extents[i] = h
	m.add(i, dsum, dcount)
}

// Forget makes row i unmeasured again, so that it counts as the estimate
// until it is laid out.
func (m *MeasuredExtents) Forget(i int) {
	old := m.extents[i]
	if old < 0 {
		return
	}
	m.extents[i] = unmeasured
	m.add(i, -float64(old), -1)
}

// ForgetAll makes every row unmeasured, as when the width of the rows changes.
func (m *MeasuredExtents) ForgetAll() {
	if m.measured == 0 {
		return
	}
	for i := range m.extents {
		m.extents[i] = unmeasured
	}
	clear(m.sums)
	clear(m.counts)
	m.total, m.measured = 0, 0
}

func (m *MeasuredExtents) add(i int, dsum float64, dcount int32) {
	m.total += dsum
	m.measured += int(dcount)
	for k := i + 1; k < len(m.sums); k += k & -k {
		m.sums[k] += dsum
		m.counts[k] += dcount
	}
}

// Extent returns the height of row i: measured, or the estimate.
func (m *MeasuredExtents) Extent(i int) float32 {
	if e := m.extents[i]; e >= 0 {
		return e
	}
	return m.Estimate()
}

// Offset returns the top of row i, for 0 <= i <= Len; Offset(Len()) is Total.
func (m *MeasuredExtents) Offset(i int) float64 {
	sum, count := m.prefix(i)
	return sum + float64(i-count)*float64(m.Estimate())
}

// Total returns the height of all the rows, the unmeasured ones at the
// estimate.
func (m *MeasuredExtents) Total() float64 {
	return m.total + float64(len(m.extents)-m.measured)*float64(m.Estimate())
}

// IndexAt returns the row that covers y: the last row whose top is at or
// above y, clamped to the rows that exist. An empty table returns 0.
func (m *MeasuredExtents) IndexAt(y float64) int {
	n := len(m.extents)
	if n == 0 {
		return 0
	}
	est := float64(m.Estimate())
	// Walk down the tree, taking each span of rows that ends at or above y.
	// At step s the node pos+s holds exactly the s rows after pos.
	pos := 0
	var acc float64
	for step := 1 << (bits.Len(uint(n)) - 1); step > 0; step >>= 1 {
		k := pos + step
		if k > n {
			continue
		}
		span := m.sums[k] + float64(step-int(m.counts[k]))*est
		if acc+span <= y {
			pos = k
			acc += span
		}
	}
	return min(pos, n-1)
}
