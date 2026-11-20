package editor

// rowTable counts the rows on screen of every line of a Editor: one per line
// without wrapping, and as many as the line wraps into with it. Lines that were
// never laid out hold an estimate. A Fenwick tree over the counts finds the
// line at a row, and the row of a line, in O(log n), so that a document of
// 100,000 lines scrolls without adding up their heights.
//
// Without wrapping every line is one row, and the table is uniform: it holds
// only the number of lines, so that a file of a hundred million lines costs
// nothing here and an edit that adds lines copies nothing.
type rowTable struct {
	// uniform says that every line is one row, and n is then the number of
	// lines; rows, exact and tree are unused.
	uniform bool
	n       int
	rows    []int32
	// exact marks the counts that come from a layout.
	exact []bool
	// tree is the Fenwick tree over rows, 1-based.
	tree []int32
	// top is the largest power of two not above len(rows), for find.
	top int
}

// reset makes the table hold n lines whose counts estimate returns, or, when
// uniform, n lines of one row each.
func (t *rowTable) reset(n int, uniform bool, estimate func(i int) int32) {
	t.uniform, t.n = uniform, n
	if uniform {
		t.rows, t.exact, t.tree = nil, nil, nil
		return
	}
	t.rows = resize(t.rows, n)
	t.exact = resize(t.exact, n)
	for i := range t.rows {
		t.rows[i] = estimate(i)
		t.exact[i] = false
	}
	t.rebuild()
}

// splice follows an edit: removed lines from first on gave way to added
// lines, whose counts estimate returns.
func (t *rowTable) splice(first, removed, added int, estimate func(i int) int32) {
	if t.uniform {
		t.n += added - removed
		return
	}
	if removed == added {
		for i := first; i < first+added; i++ {
			t.set(i, estimate(i), false)
		}
		return
	}
	n := len(t.rows) - removed + added
	rows, exact := make([]int32, n), make([]bool, n)
	copy(rows, t.rows[:first])
	copy(exact, t.exact[:first])
	copy(rows[first+added:], t.rows[first+removed:])
	copy(exact[first+added:], t.exact[first+removed:])
	t.rows, t.exact = rows, exact
	for i := first; i < first+added; i++ {
		t.rows[i] = estimate(i)
	}
	t.rebuild()
}

// spliceAll follows edits that come from the last line to the first, as
// linesRewritten takes them, moving the counts once; estimate is asked for
// the lines as they are after all the edits.
func (t *rowTable) spliceAll(chs []lineChange, estimate func(i int) int32) {
	if len(chs) == 1 {
		t.splice(chs[0].first, chs[0].removed, chs[0].added, estimate)
		return
	}
	delta, same := 0, true
	for _, ch := range chs {
		delta += ch.added - ch.removed
		same = same && ch.added == ch.removed
	}
	if t.uniform {
		t.n += delta
		return
	}
	if same {
		for _, ch := range chs {
			for i := ch.first; i < ch.first+ch.added; i++ {
				t.set(i, estimate(i), false)
			}
		}
		return
	}
	n := len(t.rows) + delta
	rows, exact := make([]int32, n), make([]bool, n)
	from, to := 0, 0
	for k := len(chs) - 1; k >= 0; k-- {
		ch := chs[k]
		m := copy(rows[to:], t.rows[from:ch.first])
		copy(exact[to:], t.exact[from:ch.first])
		to += m
		for j := range ch.added {
			rows[to+j] = estimate(to + j)
		}
		to += ch.added
		from = ch.first + ch.removed
	}
	copy(rows[to:], t.rows[from:])
	copy(exact[to:], t.exact[from:])
	t.rows, t.exact = rows, exact
	t.rebuild()
}

func resize[T any](s []T, n int) []T {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]T, n)
}

// rebuild makes the tree from the counts in O(n).
func (t *rowTable) rebuild() {
	n := len(t.rows)
	t.tree = resize(t.tree, n+1)
	t.tree[0] = 0
	for i := 1; i <= n; i++ {
		t.tree[i] = t.rows[i-1]
	}
	for i := 1; i <= n; i++ {
		if j := i + i&-i; j <= n {
			t.tree[j] += t.tree[i]
		}
	}
	t.top = 1
	for t.top*2 <= n {
		t.top *= 2
	}
}

// set changes the count of line i, and returns how much it changed.
func (t *rowTable) set(i int, rows int32, exact bool) int32 {
	if t.uniform {
		return 0
	}
	t.exact[i] = exact
	d := rows - t.rows[i]
	if d == 0 {
		return 0
	}
	t.rows[i] = rows
	for j := i + 1; j < len(t.tree); j += j & -j {
		t.tree[j] += d
	}
	return d
}

// before returns the rows of the lines before line i.
func (t *rowTable) before(i int) int {
	if t.uniform {
		return max(min(i, t.n), 0)
	}
	s := int32(0)
	for j := min(i, len(t.rows)); j > 0; j -= j & -j {
		s += t.tree[j]
	}
	return int(s)
}

// total returns the rows of all lines.
func (t *rowTable) total() int { return t.before(t.count()) }

// count returns the number of lines.
func (t *rowTable) count() int {
	if t.uniform {
		return t.n
	}
	return len(t.rows)
}

// at returns the rows of line i.
func (t *rowTable) at(i int) int32 {
	if t.uniform {
		return 1
	}
	return t.rows[i]
}

// find returns the line that holds row, and the first row of that line. Rows
// past the end give the last line.
func (t *rowTable) find(row int) (line, first int) {
	n := t.count()
	if n == 0 {
		return 0, 0
	}
	if row < 0 {
		return 0, 0
	}
	if t.uniform {
		line = min(row, n-1)
		return line, line
	}
	pos, left := 0, int32(row)
	for step := t.top; step > 0; step /= 2 {
		if next := pos + step; next <= n && t.tree[next] <= left {
			pos = next
			left -= t.tree[next]
		}
	}
	if pos >= n {
		return n - 1, t.before(n - 1)
	}
	return pos, row - int(left)
}
