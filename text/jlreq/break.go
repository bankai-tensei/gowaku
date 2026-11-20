package jlreq

// Strictness says how many characters the line start prohibition covers, like
// the strict, normal and loose of CSS line-break.
type Strictness uint8

// The strictness levels. The zero value is what JLReq 3.1.7 describes.
const (
	// Strict keeps every prohibited character from starting a line.
	Strict Strictness = iota
	// Normal lets small kana and the prolonged sound mark start a line.
	Normal
	// Loose also lets iteration marks, hyphens and wave dashes start a line.
	Loose
)

// Kinsoku selects how a line resolves a character that must not start the next
// line.
type Kinsoku uint8

// The kinsoku modes.
const (
	// Oidashi moves the end of the line to the next line.
	Oidashi Kinsoku = iota
	// Oikomi squeezes the line to take the character in, and falls back to
	// Oidashi if the line has too little to give.
	Oikomi
)

// Punctuation selects what happens to the blank halves of full-width
// punctuation.
type Punctuation uint8

// The punctuation modes.
const (
	// PunctuationSqueeze removes the blanks that JLReq 3.1.5 removes.
	PunctuationSqueeze Punctuation = iota
	// PunctuationFull keeps every mark on its full em box.
	PunctuationFull
)

// WesternSpacing selects the space between Japanese and Western text.
type WesternSpacing uint8

// The Western spacing modes.
const (
	// WesternQuarterEm adds a quarter em, as JLReq 3.2.5 describes.
	WesternQuarterEm WesternSpacing = iota
	// WesternNone adds nothing.
	WesternNone
)

// Options selects the rules. The zero value is the default composition.
type Options struct {
	Strictness     Strictness
	Kinsoku        Kinsoku
	Punctuation    Punctuation
	WesternSpacing WesternSpacing
	// Hanging lets a comma or full stop stick out past the end of the line.
	Hanging bool
	// Justify stretches every line but the last to the full width (JLReq
	// 3.8). Western word spaces open up first, then the gaps between Japanese
	// characters.
	Justify bool

	// Indent is the space before the first line, and HangingIndent the space
	// before every other line.
	Indent, HangingIndent float32
	// TabStop is the distance between tab stops, measured from the start edge
	// of the line. A unit marked Tab reaches to the next stop. Zero leaves
	// tabs at their own advance.
	TabStop float32
}

// Unit is what a line cannot split: one grapheme cluster, or a group of them
// that has to stay together, as the base of group ruby does.
type Unit struct {
	// Rune is the first rune of the cluster. It decides how the unit meets
	// the one before it.
	Rune rune
	// Last is the last rune of a group whose ends differ in class, as the
	// base of 「AI時代」 does. It decides how the unit meets the one after
	// it. Zero means Rune.
	Last rune
	// Advance is the width of the cluster as shaped.
	Advance float32
	// Em is the font size of the cluster.
	Em float32
	// FullWidth means that the glyph sits on an em box, so that a bracket has
	// the blank half that squeezing removes. Proportional fonts have none.
	FullWidth bool
	// BreakAfter is the UAX #14 opportunity after the cluster. It decides
	// wherever a Western character is involved.
	BreakAfter bool
	// Tab marks a tab, which reaches to the next tab stop.
	Tab bool
	// Ideographic makes the unit follow the rules of a kanji whatever its
	// Rune, as digits set horizontally in vertical text do.
	Ideographic bool
	// Hyphen is the width of a hyphen if a word may break after the unit,
	// and zero otherwise.
	Hyphen float32
	// StartExtra is added to the advance when the unit starts a line, and
	// EndExtra when it ends one. Ruby longer than its base reaches over the
	// characters beside it, and must not reach past the head or the end of
	// the line (JLReq 3.3.6): there the base takes the room itself.
	StartExtra, EndExtra float32
}

// last returns the rune that decides how u meets the unit after it.
func (u *Unit) last() rune {
	if u.Last != 0 {
		return u.Last
	}
	return u.Rune
}

// Line is one line of the result.
type Line struct {
	// Start and End delimit the units of the line. End is exclusive and
	// includes trailing spaces.
	Start, End int
	// Width is the extent of the line without trailing spaces and hanging
	// punctuation, measured from the start edge: it includes the indent.
	Width float32
	// Hang is how far hanging punctuation sticks out past Width.
	Hang float32
	// Hyphen is set if the line ends inside a word, which then needs a hyphen.
	// Width includes it.
	Hyphen bool
}

// Breaker holds the buffers of Break, to reuse them between calls.
type Breaker struct {
	// Lines is the result of the last Break.
	Lines []Line
	// X is the position of each unit from the start edge of its line, indent
	// included.
	X []float32
	// Advance is the extent of each unit on its line: its own advance, or for
	// a tab the distance to the next stop.
	Advance []float32

	// class is the class of the start of each unit, and endClass that of its
	// end.
	class    []Class
	endClass []Class
	before   []float32 // space before each unit, negative when trimmed
	after    []float32 // space after each unit, negative when trimmed
}

// Break splits units into lines no wider than maxWidth, and stores the lines
// and positions in b. A maxWidth of zero or less breaks wherever it may.
func (b *Breaker) Break(units []Unit, maxWidth float32, opts Options) {
	b.Lines = b.Lines[:0]
	b.prepare(units, opts)
	maxWidth = max(maxWidth, 0)
	for start := 0; start < len(units); {
		indent := opts.HangingIndent
		if start == 0 {
			indent = opts.Indent
		}
		end, line := b.nextLine(units, start, indent, maxWidth, opts)
		if opts.Justify && end < len(units) {
			b.justify(units, &line, maxWidth)
		}
		b.Lines = append(b.Lines, line)
		start = end
	}
}

// MinWidth returns the width of the widest piece of units that no line break
// opportunity splits: the narrowest width at which Break never has to break
// inside a word. Hyphenation points are not counted as opportunities, and
// trailing spaces do not count. It uses the buffers of b and does not
// allocate once they are large enough.
func (b *Breaker) MinWidth(units []Unit, opts Options) float32 {
	if len(units) == 0 {
		return 0
	}
	b.prepare(units, opts)
	var widest float32
	x := opts.Indent + b.leading(units, 0, opts)
	start := 0
	for i := range units {
		if i > start {
			x += b.before[i]
		}
		right := x + b.advanceAt(units, i, x, opts)
		if i == start {
			right += units[i].StartExtra
		}
		if b.class[i] != ClassSpace {
			widest = max(widest, right+units[i].EndExtra)
		}
		x = right + b.after[i]
		if i+1 < len(units) && b.canBreak(units, i, opts) {
			start = i + 1
			x = opts.HangingIndent + b.leading(units, start, opts)
		}
	}
	return widest
}

// prepare classifies the units and works out the spacing that does not depend
// on where the lines end. Space between two units goes into the first one's
// after, so that it disappears by itself when a line ends there.
func (b *Breaker) prepare(units []Unit, opts Options) {
	n := len(units)
	b.class = resize(b.class, n)
	b.endClass = resize(b.endClass, n)
	b.before = resize(b.before, n)
	b.after = resize(b.after, n)
	b.X = resize(b.X, n)
	b.Advance = resize(b.Advance, n)
	for i := range units {
		u := &units[i]
		b.class[i] = Classify(u.Rune)
		b.endClass[i] = b.class[i]
		if u.Last != 0 {
			b.endClass[i] = Classify(u.Last)
		}
		if u.Ideographic {
			b.class[i], b.endClass[i] = ClassIdeograph, ClassIdeograph
		}
		b.before[i], b.after[i] = 0, 0
		b.Advance[i] = units[i].Advance
	}
	for i := 0; i+1 < n; i++ {
		if opts.Punctuation == PunctuationSqueeze {
			b.squeeze(units, i)
		}
		if opts.WesternSpacing == WesternQuarterEm {
			b.westernSpace(units, i)
		}
	}
}

func resize[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	return s[:n]
}

// squeeze removes half an em where the blanks of two adjacent punctuation marks
// meet (JLReq 3.1.5). A closing mark before another closing mark or an opening
// bracket gives up its right half, and the second of two opening brackets gives
// up its left half.
func (b *Breaker) squeeze(units []Unit, i int) {
	a, c := b.endClass[i], b.class[i+1]
	if !units[i].FullWidth || !units[i+1].FullWidth {
		return
	}
	switch {
	case endsWithBlank(a) && (endsWithBlank(c) || c == ClassOpening):
		b.after[i] = -units[i].Em / 2
	case a == ClassOpening && c == ClassOpening:
		b.before[i+1] = -units[i+1].Em / 2
	}
}

// westernSpace adds a quarter em where Japanese and Western text meet (JLReq
// 3.2.5). Punctuation brings its own space, and a real space is enough: the
// ideographic space U+3000 is classed with ideographs for line breaking, but
// it is a space too and takes no quarter em next to Western text.
func (b *Breaker) westernSpace(units []Unit, i int) {
	a, c := b.endClass[i], b.class[i+1]
	if units[i].last() == '　' || units[i+1].Rune == '　' {
		return
	}
	switch {
	case isJapanese(a) && c == ClassWestern:
		b.after[i] += units[i].Em / 4
	case a == ClassWestern && isJapanese(c):
		b.after[i] += units[i+1].Em / 4
	}
}

// leading returns the position of unit i when it starts a line. An opening
// bracket loses its blank half, so that the ink lines up with the text below
// (JLReq 3.1.5).
func (b *Breaker) leading(units []Unit, i int, opts Options) float32 {
	if opts.Punctuation == PunctuationSqueeze && b.class[i] == ClassOpening && units[i].FullWidth {
		return -units[i].Em / 2
	}
	return 0
}

// nextLine finds where the line starting at unit start ends. It returns the
// first unit of the next line and the finished line.
func (b *Breaker) nextLine(units []Unit, start int, indent, maxWidth float32, opts Options) (int, Line) {
	x := indent + b.leading(units, start, opts)
	lastBreak, lastHyphen := -1, -1
	i := start
	for ; i < len(units); i++ {
		if i > start {
			x += b.before[i]
		}
		right := x + b.advanceAt(units, i, x, opts)
		if i == start {
			right += units[i].StartExtra
			b.Advance[i] += units[i].StartExtra
		}
		// A unit that needs more room at the end of a line only stays if it
		// has that room, which the next unit would take anyway.
		if right+units[i].EndExtra > maxWidth && i > start && b.class[i] != ClassSpace {
			break
		}
		b.X[i] = x
		x = right + b.after[i]
		if i+1 < len(units) && b.canBreak(units, i, opts) {
			lastBreak = i + 1
		}
		if h := units[i].Hyphen; h > 0 && i+1 < len(units) && right+h <= maxWidth {
			lastHyphen = i + 1
		}
	}
	if i == len(units) {
		return i, b.finish(units, start, i, 0)
	}
	if hang, ok := b.tryHang(units, i, x, maxWidth, opts); ok {
		return i + 1, b.finish(units, start, i+1, hang)
	}
	if end, ok := b.tryOikomi(units, start, i, indent, maxWidth, opts); ok {
		return end, b.finish(units, start, end, 0)
	}
	if lastHyphen > max(lastBreak, start) {
		// A hyphen gets more of the word onto the line than the last space.
		ln := b.finish(units, start, lastHyphen, 0)
		ln.Width += units[lastHyphen-1].Hyphen
		ln.Hyphen = true
		return lastHyphen, ln
	}
	if lastBreak > start {
		return lastBreak, b.finish(units, start, lastBreak, 0)
	}
	// Nothing on the line allows a break, so one rule has to give way.
	return i, b.finish(units, start, i, 0)
}

// tryHang keeps the comma or full stop at i on the line, past its end. The
// character after it must be able to start the next line.
func (b *Breaker) tryHang(units []Unit, i int, x, maxWidth float32, opts Options) (float32, bool) {
	c := b.class[i]
	if !opts.Hanging || (c != ClassComma && c != ClassFullStop) {
		return 0, false
	}
	if i+1 < len(units) && !b.canBreak(units, i, opts) {
		return 0, false
	}
	b.X[i] = x
	return x + b.Advance[i] - maxWidth, true
}

// tryOikomi takes the characters from i that must not start a line into the
// line, by squeezing the spaces the line holds.
func (b *Breaker) tryOikomi(units []Unit, start, i int, indent, maxWidth float32, opts Options) (int, bool) {
	if opts.Kinsoku != Oikomi {
		return 0, false
	}
	end := i
	for end < len(units) && prohibitedAtStart(b.class[end], opts.Strictness) {
		end++
	}
	if end == i {
		return 0, false
	}
	var give float32
	for k := start; k < end; k++ {
		give += b.shrinkable(units, k, k == end-1)
	}
	overflow := b.naturalWidth(units, start, end, indent, opts) - maxWidth
	if give < overflow {
		return 0, false
	}
	ratio := overflow / give
	x := indent + b.leading(units, start, opts)
	for k := start; k < end; k++ {
		if k > start {
			x += b.before[k]
		}
		b.X[k] = x
		x += b.advanceAt(units, k, x, opts) + b.after[k] - b.shrinkable(units, k, k == end-1)*ratio
		if k == start {
			x += units[k].StartExtra
			b.Advance[k] += units[k].StartExtra
		}
	}
	// The line ends at the far edge of its last unit, so that what that unit
	// gives up comes off its advance; the loop above only moved x past it.
	b.Advance[end-1] -= b.shrinkable(units, end-1, true) * ratio
	return end, true
}

// naturalWidth measures the units before any Oikomi squeezing.
func (b *Breaker) naturalWidth(units []Unit, start, end int, indent float32, opts Options) float32 {
	x := indent + b.leading(units, start, opts)
	for k := start; k < end; k++ {
		if k > start {
			x += b.before[k]
		}
		x += b.advanceAt(units, k, x, opts)
		if k == start {
			x += units[k].StartExtra
		}
		if k+1 < end {
			x += b.after[k]
		}
	}
	return x + units[end-1].EndExtra
}

// shrinkable returns what unit k can give up under Oikomi: the rest of the
// blank half of punctuation, half of a Western space, or half of the space
// between Japanese and Western text. last is set for the last unit of the
// line: the space after it, b.after[k], goes with the line end and is not in
// the line's width, so the unit gives up only its own blank half.
func (b *Breaker) shrinkable(units []Unit, k int, last bool) float32 {
	u := &units[k]
	after := b.after[k]
	if last {
		after = 0
	}
	switch {
	case endsWithBlank(b.endClass[k]) && u.FullWidth:
		return max(u.Em/2+after, 0)
	case b.class[k] == ClassSpace:
		return u.Advance / 2
	case after > 0:
		return after / 2
	}
	return 0
}

// finish measures the line. Trailing spaces take no width, hanging
// punctuation is reported apart, and the last unit gets its EndExtra.
func (b *Breaker) finish(units []Unit, start, end int, hang float32) Line {
	last := end - 1
	for last > start && b.class[last] == ClassSpace {
		last--
	}
	b.Advance[last] += units[last].EndExtra
	width := b.X[last] + b.Advance[last] - hang
	return Line{Start: start, End: end, Width: max(width, 0), Hang: hang}
}

// canBreak reports whether a line may end after unit i.
func (b *Breaker) canBreak(units []Unit, i int, opts Options) bool {
	a, c := b.endClass[i], b.class[i+1]
	if c == ClassSpace {
		return false // spaces stay at the end of the line before them
	}
	if a == ClassOpening || prohibitedAtStart(c, opts.Strictness) {
		return false
	}
	ra, rc := units[i].last(), units[i+1].Rune
	if a == ClassInseparable && c == ClassInseparable && ra == rc {
		return false
	}
	if isDigit(ra) && (isDigit(rc) || c == ClassPostfix) {
		return false
	}
	if a == ClassPrefix && isDigit(rc) {
		return false
	}
	if a == ClassWestern || c == ClassWestern || a == ClassSpace || a == ClassOther || c == ClassOther {
		return units[i].BreakAfter
	}
	return true
}

// prohibitedAtStart reports whether c must not start a line (JLReq 3.1.7).
func prohibitedAtStart(c Class, s Strictness) bool {
	switch c {
	case ClassClosing, ClassComma, ClassFullStop, ClassMiddleDot, ClassDividing:
		return true
	case ClassSmallKana, ClassProlonged:
		return s == Strict
	case ClassIteration, ClassHyphen:
		return s != Loose
	}
	return false
}
