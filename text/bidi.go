package text

import (
	gobidi "github.com/go-text/typesetting/bidi"
	"golang.org/x/text/unicode/bidi"
)

// Direction is the base direction of a paragraph (UAX #9).
type Direction uint8

// The directions.
const (
	// DirectionAuto takes the direction of the first strong character of each
	// block (UAX #9 P2), or left to right if there is none.
	DirectionAuto Direction = iota
	// DirectionLTR sets every block left to right.
	DirectionLTR
	// DirectionRTL sets every block right to left, as Arabic and Hebrew are.
	DirectionRTL
)

// String returns the name of d.
func (d Direction) String() string {
	switch d {
	case DirectionLTR:
		return "LTR"
	case DirectionRTL:
		return "RTL"
	}
	return "Auto"
}

// firstStrong returns whether the first strong character of runes is right to
// left, and whether there is one.
func firstStrong(runes []rune) (rtl, found bool) {
	for _, r := range runes {
		p, _ := bidi.LookupRune(r)
		switch p.Class() {
		case bidi.L:
			return false, true
		case bidi.R, bidi.AL:
			return true, true
		}
	}
	return false, false
}

// needsBidi reports whether runes hold anything that can leave left to right
// order: a right-to-left letter, an Arabic number, or an explicit embedding.
func needsBidi(runes []rune) bool {
	for _, r := range runes {
		p, _ := bidi.LookupRune(r)
		switch p.Class() {
		case bidi.R, bidi.AL, bidi.AN, bidi.RLE, bidi.RLO, bidi.RLI, bidi.LRE, bidi.LRO, bidi.LRI, bidi.FSI:
			return true
		}
	}
	return false
}

// blockLevels resolves the embedding levels of the runes of a block. It returns
// nil when every level is zero, which is the common case and costs nothing
// later on, and whether the block runs right to left.
func (s *Shaper) blockLevels(runes []rune, d Direction) (levels []uint8, rtl bool) {
	switch d {
	case DirectionRTL:
		rtl = true
	case DirectionAuto:
		rtl, _ = firstStrong(runes)
	}
	if !rtl && !needsBidi(runes) {
		return nil, false
	}
	def := gobidi.LeftToRight
	if rtl {
		def = gobidi.RightToLeft
	}
	runs := s.bidi.Segment(runes, def)
	levels = make([]uint8, len(runes))
	for i := range runs.NumRuns() {
		r := runs.Run(i)
		for k := r.Start; k < r.End; k++ {
			levels[k] = uint8(r.Level)
		}
	}
	return levels, rtl
}

// reorder writes the visual order of the clusters from first to last into
// order, by UAX #9 L2: from the highest level down to the lowest odd one,
// every run at that level or above turns around.
func reorder(levels []uint8, order []int) {
	var highest uint8
	lowestOdd := uint8(255)
	for i, l := range levels {
		order[i] = i
		highest = max(highest, l)
		if l%2 == 1 {
			lowestOdd = min(lowestOdd, l)
		}
	}
	for lvl := highest; lvl >= lowestOdd && lvl > 0; lvl-- {
		for i := 0; i < len(levels); {
			if levels[order[i]] < lvl {
				i++
				continue
			}
			j := i
			for j < len(levels) && levels[order[j]] >= lvl {
				j++
			}
			for a, b := i, j-1; a < b; a, b = a+1, b-1 {
				order[a], order[b] = order[b], order[a]
			}
			i = j
		}
	}
}
