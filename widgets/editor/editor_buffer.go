package editor

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
)

// TextPosition is a place in the text of a Editor: a line, from 0, and a
// byte offset into that line, which never counts its line break.
type TextPosition struct {
	Line, Column int
}

// Before reports whether p comes before q.
func (p TextPosition) Before(q TextPosition) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Column < q.Column
}

// TextRange is the text between two positions. As a selection, Start is the
// anchor that stays and End the caret that moves, so End can come first.
type TextRange struct {
	Start, End TextPosition
}

// Ordered returns r with Start before End.
func (r TextRange) Ordered() TextRange {
	if r.End.Before(r.Start) {
		return TextRange{Start: r.End, End: r.Start}
	}
	return r
}

// Empty reports whether r holds no text: a caret.
func (r TextRange) Empty() bool { return r.Start == r.End }

// lineBuffer holds the text of a Editor as pieces: runs of lines that are
// either held in memory or are lines of a LineSource, read when asked for. A
// text set with SetText is one piece in memory, an array of lines, because the
// view asks for whole lines every frame, which an array answers at once, and
// an edit costs the length of the lines it touches plus, when lines come or
// go, a move of 16 bytes per line after them. A source set with SetSource
// starts as one piece of the source; an edit cuts it around the lines it
// changes and puts those in a piece in memory, so that a file of gigabytes
// stays where it is but for the lines that were edited, and WriteTo writes the
// pieces in order. See docs/ADR/0153-editor-file-pieces.md and
// docs/ADR/0157-editor-line-source.md.
//
// Every line counts its length plus one for its break, the last line too, so
// that the bytes of a piece add up without knowing where it lies; the text is
// one byte shorter than the sum.
//
// The zero value is one empty line.
type lineBuffer struct {
	pieces []piece
	// lineEnd[k] and byteEnd[k] are the lines and bytes of pieces[:k+1].
	lineEnd []int
	byteEnd []int
	// src is the source the source pieces read, or nil, and saved the lines
	// of it that edits cut away and the lines around them, by their number in
	// the source (see keep).
	src   LineSource
	saved map[int]string
	// gen counts the edits, and joined is the whole text of generation
	// joinedGen.
	gen       uint64
	joined    string
	joinedGen uint64
}

// piece is a run of lines: lines of src from start, n of them, or the lines
// in memory.
type piece struct {
	source   bool
	start, n int
	lines    []string
	// bytes is the length of the lines plus one per line.
	bytes int
	// starts holds the offsets, from the start of the piece, of its first
	// lines in memory, as many as were needed since the last edit before them.
	starts []int
}

func memoryPiece(lines []string) piece {
	p := piece{lines: lines, n: len(lines)}
	for _, l := range lines {
		p.bytes += len(l) + 1
	}
	return p
}

// reset replaces the whole text with s, in memory.
func (b *lineBuffer) reset(s string) {
	b.src, b.saved = nil, nil
	b.pieces = append(b.pieces[:0], memoryPiece(splitLines(s)))
	b.sum(0)
	b.gen++
}

// resetSource makes the text the lines of src known so far.
func (b *lineBuffer) resetSource(src LineSource) {
	b.src, b.saved = src, nil
	b.pieces = b.pieces[:0]
	if n := src.Len(); n > 0 {
		b.pieces = append(b.pieces, b.sourcePiece(0, n))
	} else {
		b.pieces = append(b.pieces, memoryPiece([]string{""}))
	}
	b.sum(0)
	b.gen++
}

// sourcePiece returns the piece of lines start to start+n of the source.
func (b *lineBuffer) sourcePiece(start, n int) piece {
	return piece{source: true, start: start, n: n, bytes: int(b.src.Offset(start+n)) - int(b.src.Offset(start))}
}

// sum recounts lineEnd and byteEnd from piece k on.
func (b *lineBuffer) sum(k int) {
	n := len(b.pieces)
	// The sums before k stay, so the slices grow keeping them.
	b.lineEnd = slices.Grow(b.lineEnd[:min(k, len(b.lineEnd))], n-k)[:n]
	b.byteEnd = slices.Grow(b.byteEnd[:min(k, len(b.byteEnd))], n-k)[:n]
	lines, bytes := 0, 0
	if k > 0 {
		lines, bytes = b.lineEnd[k-1], b.byteEnd[k-1]
	}
	for ; k < n; k++ {
		lines += b.pieces[k].n
		bytes += b.pieces[k].bytes
		b.lineEnd[k], b.byteEnd[k] = lines, bytes
	}
}

// ensure gives the zero buffer its one empty line, before an edit.
func (b *lineBuffer) ensure() {
	if len(b.pieces) == 0 {
		b.pieces = append(b.pieces, memoryPiece([]string{""}))
		b.sum(0)
	}
}

// splitLines cuts s at its line breaks, of which it takes \n, \r\n and a lone
// \r. It returns one line at least.
func splitLines(s string) []string {
	return strings.Split(normalizeBreaks(s), "\n")
}

// normalizeBreaks returns s with \n in place of every \r\n and lone \r, which
// is how the buffer holds it.
func normalizeBreaks(s string) string {
	if strings.IndexByte(s, '\r') >= 0 {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
	}
	return s
}

// lineCount returns the number of lines, at least one.
func (b *lineBuffer) lineCount() int {
	if len(b.pieces) == 0 {
		return 1
	}
	return b.lineEnd[len(b.lineEnd)-1]
}

// locate returns the piece that holds line i and the index of the line in it.
func (b *lineBuffer) locate(i int) (k, j int) {
	if len(b.pieces) == 1 {
		return 0, i
	}
	k = sort.SearchInts(b.lineEnd, i+1)
	k = min(k, len(b.pieces)-1)
	return k, i - b.lineEnd[k] + b.pieces[k].n
}

// line returns line i without its line break.
func (b *lineBuffer) line(i int) string {
	if len(b.pieces) == 0 {
		return ""
	}
	k, j := b.locate(i)
	p := &b.pieces[k]
	if p.source {
		return b.src.Line(p.start + j)
	}
	return p.lines[j]
}

// text returns the whole text, joined with \n. It is made once per edit; for
// a file it reads the whole file.
func (b *lineBuffer) text() string {
	if b.joinedGen != b.gen || b.joined == "" && b.gen == 0 {
		if len(b.pieces) == 1 && !b.pieces[0].source {
			b.joined = strings.Join(b.pieces[0].lines, "\n")
		} else {
			var sb strings.Builder
			sb.Grow(max(b.length(), 0))
			n := b.lineCount()
			for i := range n {
				if i > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(b.line(i))
			}
			b.joined = sb.String()
		}
		b.joinedGen = b.gen
	}
	return b.joined
}

// clamp moves p into the text.
func (b *lineBuffer) clamp(p TextPosition) TextPosition {
	n := b.lineCount()
	switch {
	case p.Line < 0:
		return TextPosition{}
	case p.Line >= n:
		return TextPosition{Line: n - 1, Column: len(b.line(n - 1))}
	}
	l := b.line(p.Line)
	p.Column = max(min(p.Column, len(l)), 0)
	// Stay at the start of a character.
	for p.Column > 0 && p.Column < len(l) && !startsRune(l[p.Column]) {
		p.Column--
	}
	return p
}

func startsRune(c byte) bool { return c&0xC0 != 0x80 }

// end returns the position after the last character.
func (b *lineBuffer) end() TextPosition {
	n := b.lineCount()
	return TextPosition{Line: n - 1, Column: len(b.line(n - 1))}
}

// pieceStart returns the offset of piece k in the whole text.
func (b *lineBuffer) pieceStart(k int) int {
	if k == 0 {
		return 0
	}
	return b.byteEnd[k-1]
}

// within returns the offset of line j of piece k from the start of the piece.
func (b *lineBuffer) within(k, j int) int {
	p := &b.pieces[k]
	if p.source {
		return int(b.src.Offset(p.start+j)) - int(b.src.Offset(p.start))
	}
	for len(p.starts) <= j {
		m := len(p.starts)
		if m == 0 {
			p.starts = append(p.starts, 0)
			continue
		}
		p.starts = append(p.starts, p.starts[m-1]+len(p.lines[m-1])+1)
	}
	return p.starts[j]
}

// lineStart returns the offset of line i in the whole text.
func (b *lineBuffer) lineStart(i int) int {
	if len(b.pieces) == 0 {
		return 0
	}
	k, j := b.locate(i)
	return b.pieceStart(k) + b.within(k, j)
}

// offset returns p as a byte offset into the whole text.
func (b *lineBuffer) offset(p TextPosition) int { return b.lineStart(p.Line) + p.Column }

// length returns the length of the whole text in bytes.
func (b *lineBuffer) length() int {
	if len(b.pieces) == 0 {
		return 0
	}
	return b.byteEnd[len(b.byteEnd)-1] - 1
}

// position returns the position of the byte offset off.
func (b *lineBuffer) position(off int) TextPosition {
	if off <= 0 || len(b.pieces) == 0 {
		return TextPosition{}
	}
	if off >= b.length() {
		return b.end()
	}
	k := sort.SearchInts(b.byteEnd, off+1)
	k = min(k, len(b.pieces)-1)
	rel := off - b.pieceStart(k)
	p := &b.pieces[k]
	first := b.lineEnd[k] - p.n
	var j int
	if p.source {
		base := int(b.src.Offset(p.start))
		j = b.src.LineAt(int64(base+rel)) - p.start
	} else {
		// Extend the starts until they pass rel, then search them.
		for len(p.starts) < p.n && (len(p.starts) == 0 || p.starts[len(p.starts)-1] <= rel) {
			b.within(k, len(p.starts))
		}
		j = sort.SearchInts(p.starts, rel+1) - 1
	}
	j = min(max(j, 0), p.n-1)
	return b.clamp(TextPosition{Line: first + j, Column: rel - b.within(k, j)})
}

// slice returns the text from a to z, which come in order.
func (b *lineBuffer) slice(a, z TextPosition) string {
	if a.Line == z.Line {
		return b.line(a.Line)[a.Column:z.Column]
	}
	var sb strings.Builder
	sb.WriteString(b.line(a.Line)[a.Column:])
	for i := a.Line + 1; i < z.Line; i++ {
		sb.WriteByte('\n')
		sb.WriteString(b.line(i))
	}
	sb.WriteByte('\n')
	sb.WriteString(b.line(z.Line)[:z.Column])
	return sb.String()
}

// lineChange says which lines an edit replaced: removed lines from first on
// gave way to added lines.
type lineChange struct {
	first, removed, added int
}

// replace puts s in place of the text from a to z, which come in order, and
// returns the position after s.
func (b *lineBuffer) replace(a, z TextPosition, s string) (TextPosition, lineChange) {
	b.ensure()
	prefix := b.line(a.Line)[:a.Column]
	suffix := b.line(z.Line)[z.Column:]
	ch := lineChange{first: a.Line, removed: z.Line - a.Line + 1}
	var end TextPosition
	var parts []string
	if !strings.ContainsAny(s, "\r\n") {
		parts = []string{prefix + s + suffix}
		end = TextPosition{Line: a.Line, Column: len(prefix) + len(s)}
	} else {
		parts = splitLines(s)
		last := len(parts) - 1
		end = TextPosition{Line: a.Line + last, Column: len(parts[last])}
		parts[0] = prefix + parts[0]
		parts[last] += suffix
	}
	ch.added = len(parts)
	b.splice(ch.first, ch.removed, parts)
	b.gen++
	return end, ch
}

// splice puts lines in place of removed lines from first on.
func (b *lineBuffer) splice(first, removed int, lines []string) {
	k, j := b.locate(first)
	kz, jz := b.locate(first + removed - 1)
	if p := &b.pieces[k]; k == kz && !p.source {
		// Within one piece in memory, as all edits of a text set by SetText.
		old := 0
		for _, l := range p.lines[j : j+removed] {
			old += len(l) + 1
		}
		p.lines = slices.Replace(p.lines, j, j+removed, lines...)
		p.n = len(p.lines)
		p.bytes -= old
		for _, l := range lines {
			p.bytes += len(l) + 1
		}
		p.starts = p.starts[:min(len(p.starts), j+1)]
		b.sum(k)
		return
	}
	b.keep(k, j, kz, jz)
	var mid []piece
	if j > 0 {
		mid = append(mid, b.head(k, j))
	}
	mid = append(mid, memoryPiece(slices.Clone(lines)))
	if pz := b.pieces[kz]; jz+1 < pz.n {
		mid = append(mid, b.tail(kz, jz+1))
	}
	b.pieces = slices.Replace(b.pieces, k, kz+1, mid...)
	// Pieces in memory side by side become one, so that typing on line
	// after line of a file does not make a piece for each.
	from := max(k-1, 0)
	for i := min(k+len(mid), len(b.pieces)-1); i > from; i-- {
		if l, r := &b.pieces[i-1], &b.pieces[i]; !l.source && !r.source {
			l.lines = append(slices.Clip(l.lines), r.lines...)
			l.n, l.bytes = len(l.lines), l.bytes+r.bytes
			b.pieces = slices.Delete(b.pieces, i, i+1)
		}
	}
	b.sum(from)
}

// head returns the first j lines of piece k as a piece of their own.
func (b *lineBuffer) head(k, j int) piece {
	p := b.pieces[k]
	if p.source {
		return b.sourcePiece(p.start, j)
	}
	return memoryPiece(slices.Clip(p.lines[:j]))
}

// tail returns the lines of piece k from j on as a piece of their own.
func (b *lineBuffer) tail(k, j int) piece {
	p := b.pieces[k]
	if p.source {
		return b.sourcePiece(p.start+j, p.n-j)
	}
	return memoryPiece(slices.Clip(p.lines[j:]))
}

// grow follows the lines of the source from line from on, which are new or
// changed, at its end, as a source that is being read learns its lines. It
// reports the change, or false when nothing changed.
func (b *lineBuffer) grow(from int) (lineChange, bool) {
	if b.src == nil {
		return lineChange{}, false
	}
	to := b.src.Len()
	if most := b.sourceLines(); to < most {
		panic(fmt.Sprintf("gowaku: the LineSource of an editor went from %d lines to %d; a source must not lose lines (show a changed source with SetSource or Rebase)", most, to))
	}
	n := b.lineCount()
	last := len(b.pieces) - 1
	p := &b.pieces[last]
	var ch lineChange
	switch {
	case p.source && p.start <= from && from <= p.start+p.n:
		end := p.start + p.n
		if to <= from && end <= from {
			return lineChange{}, false
		}
		p.n = to - p.start
		p.bytes = int(b.src.Offset(p.start+p.n) - b.src.Offset(p.start))
		if changed := end - from; changed > 0 {
			// Lines at the end changed, as a last line an append went on.
			ch = lineChange{first: n - changed, removed: changed, added: to - from}
		} else {
			ch = lineChange{first: n - 1, removed: 1, added: 1 + to - end}
		}
	case !p.source && p.n == 1 && p.lines[0] == "" && last == 0 && from == 0:
		// The empty line of a source that had no lines yet.
		if to == 0 {
			return lineChange{}, false
		}
		b.pieces[0] = b.sourcePiece(0, to)
		ch = lineChange{first: 0, removed: 1, added: to}
	default:
		if to <= from {
			return lineChange{}, false
		}
		b.pieces = append(b.pieces, b.sourcePiece(from, to-from))
		ch = lineChange{first: n - 1, removed: 1, added: 1 + to - from}
	}
	b.sum(last)
	b.gen++
	return ch, true
}

// sourceLines returns how many lines of the source the pieces reach.
func (b *lineBuffer) sourceLines() int {
	most := 0
	for _, p := range b.pieces {
		if p.source {
			most = max(most, p.start+p.n)
		}
	}
	return most
}

// writeTo writes the text to w: the lines of the source as the source writes
// them, when it is a LineWriter, and the other lines with nl after each but
// the last.
func (b *lineBuffer) writeTo(w io.Writer, nl string) error {
	bw := bufio.NewWriterSize(w, 1<<16)
	lw, _ := b.src.(LineWriter)
	last := len(b.pieces) - 1
	put := func(s string) {
		if _, err := bw.WriteString(s); err != nil {
			panic(writeError{err})
		}
	}
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				we, ok := r.(writeError)
				if !ok {
					panic(r)
				}
				err = we.err
			}
		}()
		for k, p := range b.pieces {
			end := p.start + p.n
			switch {
			case p.source && lw != nil:
				srcEnd := end == b.src.Len()
				if k == last && !srcEnd {
					// The text ends inside the source: its last line without
					// the break the source has after it.
					if err := lw.WriteLines(bw, p.start, end-1); err != nil {
						return err
					}
					put(b.src.Line(end - 1))
					continue
				}
				if err := lw.WriteLines(bw, p.start, end); err != nil {
					return err
				}
				if k < last && srcEnd {
					// The last line of the source, which has no break, and
					// more text after it.
					put(nl)
				}
			default:
				for j := range p.n {
					if p.source {
						put(b.src.Line(p.start + j))
					} else {
						put(p.lines[j])
					}
					if k < last || j < p.n-1 {
						put(nl)
					}
				}
			}
		}
		return nil
	}()
	if err != nil {
		return err
	}
	return bw.Flush()
}

// writeError carries an error of writeTo out of its writes.
type writeError struct{ err error }
