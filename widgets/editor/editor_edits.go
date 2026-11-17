package editor

import "slices"

// hunkContext is how many lines of the source around an edit SourceEdit keeps
// on each side, and savedLines how many lines an edit may replace for the text
// to keep them.
const (
	hunkContext = 2
	savedLines  = 4096
)

// keep notes the lines of the source around and inside an edit that replaces
// lines j of piece k to jz of piece kz, before the edit cuts them away, so
// that Controller.Edits can tell them even when the source can no longer read
// them as they were, as when the file under it was rewritten.
func (b *lineBuffer) keep(k, j, kz, jz int) {
	if b.src == nil {
		return
	}
	if b.saved == nil {
		b.saved = make(map[int]string)
	}
	save := func(p piece, from, to int) {
		from, to = max(from, 0), min(to, p.n)
		if !p.source || to-from > savedLines {
			return
		}
		for i := from; i < to; i++ {
			if _, ok := b.saved[p.start+i]; !ok {
				b.saved[p.start+i] = b.src.Line(p.start + i)
			}
		}
	}
	// The lines before the edit, in its piece or the one before.
	save(b.pieces[k], j-hunkContext, j)
	if j < hunkContext && k > 0 {
		p := b.pieces[k-1]
		save(p, p.n-(hunkContext-j), p.n)
	}
	// The lines the edit replaces.
	for i := k; i <= kz; i++ {
		from, to := 0, b.pieces[i].n
		if i == k {
			from = j
		}
		if i == kz {
			to = jz + 1
		}
		save(b.pieces[i], from, to)
	}
	// The lines after it, in its last piece or the one after.
	pz := b.pieces[kz]
	save(pz, jz+1, jz+1+hunkContext)
	if left := pz.n - (jz + 1); left < hunkContext && kz+1 < len(b.pieces) {
		save(b.pieces[kz+1], 0, hunkContext-left)
	}
}

// oldLine returns line i of the source as the text knew it: as kept by keep,
// or read from the source.
func (b *lineBuffer) oldLine(i int) string {
	if l, ok := b.saved[i]; ok {
		return l
	}
	return b.src.Line(i)
}

// oldLines returns lines from to to-1 of the source as the text knew them.
func (b *lineBuffer) oldLines(from, to int) []string {
	out := make([]string, 0, max(to-from, 0))
	for i := from; i < to; i++ {
		out = append(out, b.oldLine(i))
	}
	return out
}

// edits returns the edits of the text against its source, in order, each
// without the lines at its ends that it left as they were: an edit that
// replaced a line with itself and another, as typing a line and a break does,
// keeps only the new line, so that the lines that place it are as few as can
// be.
func (b *lineBuffer) edits() []SourceEdit {
	if b.src == nil {
		return nil
	}
	var out []SourceEdit
	add := func(from, to int, inserted []string) {
		big := to-from > savedLines
		for !big && from < to && len(inserted) > 0 && b.oldLine(to-1) == inserted[len(inserted)-1] {
			to--
			inserted = inserted[:len(inserted)-1]
		}
		for !big && from < to && len(inserted) > 0 && b.oldLine(from) == inserted[0] {
			from++
			inserted = inserted[1:]
		}
		e := SourceEdit{
			From: from, To: to, Inserted: slices.Clone(inserted),
			Before: b.oldLines(max(from-hunkContext, 0), from),
			After:  b.oldLines(to, min(to+hunkContext, b.src.Len())),
		}
		if !big {
			e.Removed = b.oldLines(from, to)
		}
		out = append(out, e)
	}
	cursor := 0
	var pending []string
	edited := false
	for _, p := range b.pieces {
		if !p.source {
			pending = append(pending, p.lines...)
			edited = true
			continue
		}
		if p.start > cursor || edited {
			add(cursor, p.start, pending)
		}
		cursor, pending, edited = p.start+p.n, nil, false
	}
	if n := b.src.Len(); cursor < n || edited {
		add(cursor, n, pending)
	}
	return out
}

// rebase makes the text the lines of src with the edits placed in it: edit
// p.Edit of edits, which replaced its lines From to To-1 of the old source,
// replaces as many lines of src from line p.At on. Edits placed out of order,
// overlapping an earlier one or past the end of src are left out, as are those
// not placed.
func (b *lineBuffer) rebase(src LineSource, edits []SourceEdit, placed []PlacedEdit) {
	b.src, b.saved = src, nil
	b.pieces = b.pieces[:0]
	cursor := 0
	n := src.Len()
	for _, p := range placed {
		if p.Edit < 0 || p.Edit >= len(edits) {
			continue
		}
		e := edits[p.Edit]
		removed := e.To - e.From
		if p.At < cursor || p.At+removed > n {
			continue
		}
		if p.At > cursor {
			b.pieces = append(b.pieces, b.sourcePiece(cursor, p.At-cursor))
		}
		if len(e.Inserted) > 0 {
			b.pieces = append(b.pieces, memoryPiece(slices.Clone(e.Inserted)))
		}
		cursor = p.At + removed
	}
	if cursor < n {
		b.pieces = append(b.pieces, b.sourcePiece(cursor, n-cursor))
	}
	if len(b.pieces) == 0 {
		b.pieces = append(b.pieces, memoryPiece([]string{""}))
	}
	b.sum(0)
	b.gen++
}

// lineMap returns where line i of the text goes when rebase places edits in
// a source of n lines: a line of the source moves as far as the placed edit
// before it moved, a line an edit put in goes to the same line of the edit
// where it is placed, or where the lines it replaced come back when it is not
// placed. Call it before rebase.
func (b *lineBuffer) lineMap(edits []SourceEdit, placed []PlacedEdit, n int) func(i int) int {
	type kept struct{ from, to, at, inserted int }
	var ok []kept
	cursor := 0
	for _, p := range placed {
		if p.Edit < 0 || p.Edit >= len(edits) {
			continue
		}
		e := edits[p.Edit]
		if p.At < cursor || p.At+e.To-e.From > n {
			continue
		}
		ok = append(ok, kept{from: e.From, to: e.To, at: p.At, inserted: len(e.Inserted)})
		cursor = p.At + e.To - e.From
	}
	// source returns the line of the new text of line s of the old source:
	// with before set, where an edit that starts at s puts its lines.
	source := func(s int, before bool) int {
		drift, grown := 0, 0
		for _, k := range ok {
			if s < k.from {
				break
			}
			if before && s == k.from || s < k.to {
				// A line the edit replaced: where its lines are put.
				return k.at + grown
			}
			drift = k.at - k.from
			grown += k.inserted - (k.to - k.from)
		}
		return s + drift + grown
	}
	return func(i int) int {
		if b.src == nil || len(b.pieces) == 0 {
			return i
		}
		k, j := b.locate(min(max(i, 0), b.lineCount()-1))
		if p := b.pieces[k]; p.source {
			return source(p.start+j, false)
		}
		// A line put in by an edit: from the source line the run of edited
		// lines starts at.
		base, run := 0, 0
		for q := k - 1; q >= 0; q-- {
			if b.pieces[q].source {
				base, run = b.pieces[q].start+b.pieces[q].n, b.lineEnd[q]
				break
			}
		}
		return source(base, true) + (b.lineEnd[k] - b.pieces[k].n + j - run)
	}
}

// replaceSource makes the text the lines of src, which hold the text as it is,
// as after it was saved to the file src reads. It reports false, and changes
// nothing, when src does not hold as many lines and bytes as the text.
func (b *lineBuffer) replaceSource(src LineSource) bool {
	n := b.lineCount()
	if src.Len() != n || src.Offset(n) != int64(b.length())+1 {
		return false
	}
	b.src, b.saved = src, nil
	b.pieces = append(b.pieces[:0], b.sourcePiece(0, n))
	b.sum(0)
	return true
}
