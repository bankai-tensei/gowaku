package jlreq

// advanceAt works out the extent of unit i when it starts at x, and keeps it in
// b.Advance. Only a tab depends on where it starts.
func (b *Breaker) advanceAt(units []Unit, i int, x float32, opts Options) float32 {
	a := units[i].Advance
	if units[i].Tab && opts.TabStop > 0 {
		stop := opts.TabStop
		a = float32(int(x/stop)+1)*stop - x
	}
	b.Advance[i] = a
	return a
}

// gap is what justification may do to the space after a unit.
type gap uint8

const (
	gapNone gap = iota
	// gapWord is a Western word space, or the space between Japanese and
	// Western text.
	gapWord
	// gapChar is the space between two characters set on the em box.
	gapChar
)

// justify spreads the room left on ln over its gaps (JLReq 3.8.3). Word gaps
// open up first, by up to a quarter em each. The rest goes evenly to the gaps
// between characters, or to the word gaps if there are none. A line with no gap
// at all stays as it is.
func (b *Breaker) justify(units []Unit, ln *Line, maxWidth float32) {
	extra := maxWidth - ln.Width
	if extra <= 0 {
		return
	}
	last := ln.End - 1
	if ln.Hang > 0 {
		last-- // the hanging mark moves with the character before it
	}
	for last > ln.Start && b.class[last] == ClassSpace {
		last--
	}
	var words, chars int
	var wordRoom float32
	for k := ln.Start; k < last; k++ {
		switch b.gapKind(units, k) {
		case gapWord:
			words++
			wordRoom += units[k].Em / 4
		case gapChar:
			chars++
		}
	}
	if words+chars == 0 {
		return
	}
	wordShare := min(extra, wordRoom)
	var perChar float32
	if chars > 0 {
		perChar = (extra - wordShare) / float32(chars)
	} else {
		wordShare = extra
	}
	var shift float32
	for k := ln.Start; k < ln.End; k++ {
		b.X[k] += shift
		if k >= last {
			continue
		}
		switch b.gapKind(units, k) {
		case gapWord:
			shift += wordShare * units[k].Em / 4 / wordRoom
		case gapChar:
			shift += perChar
		}
	}
	ln.Width += extra
}

// gapKind says how the gap between units k and k+1 may open up. Nothing opens
// inside a Western word or a number, inside a dash or an ellipsis, after an
// opening bracket, or before a mark that closes.
func (b *Breaker) gapKind(units []Unit, k int) gap {
	a, c := b.endClass[k], b.class[k+1]
	ra, rc := units[k].last(), units[k+1].Rune
	switch {
	case a == ClassSpace:
		return gapWord
	case c == ClassSpace:
		return gapNone
	case b.after[k] > 0:
		return gapWord
	case a == ClassOpening, c == ClassClosing, c == ClassComma, c == ClassFullStop, c == ClassMiddleDot:
		return gapNone
	case !isJapanese(a) && !isJapanese(c):
		// Western text opens only at its spaces, and its punctuation stays
		// with the word.
		return gapNone
	case a == ClassInseparable && c == ClassInseparable && ra == rc:
		return gapNone
	case isDigit(ra) && (isDigit(rc) || c == ClassPostfix):
		return gapNone
	case a == ClassPrefix && isDigit(rc):
		return gapNone
	}
	return gapChar
}
