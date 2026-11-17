// Package hyphen finds where words may break with a hyphen, by Liang's
// algorithm and TeX patterns. It depends on the standard library only, so that
// its rules are tested without fonts.
//
// The American English patterns of hyph-utf8 (Gerard D.C. Kuiken, after
// Knuth's hyphen.tex) are built in; see patterns/hyph-en-us.license.txt.
package hyphen

import (
	_ "embed"
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

//go:embed patterns/hyph-en-us.pat.txt
var enUSPatterns string

//go:embed patterns/hyph-en-us.hyp.txt
var enUSExceptions string

// english parses the built-in patterns once, on first use. It is set at
// start and never changes.
var english = sync.OnceValue(func() *Patterns {
	p, err := Parse(enUSPatterns, enUSExceptions, 2, 3)
	if err != nil {
		panic("gowaku: built-in hyphenation patterns: " + err.Error())
	}
	return p
})

// English returns the patterns of American English. It is safe to call from
// any goroutine.
func English() *Patterns { return english() }

// Patterns are the hyphenation patterns of one language.
type Patterns struct {
	// values maps the letters of a pattern to its digits, one more than the
	// letters.
	values map[string][]uint8
	// exceptions maps a word to its break points, in runes from its start.
	exceptions map[string][]int
	longest    int
	// left and right are the fewest letters to keep before and after a break.
	left, right int
}

// Parse reads patterns in the format of TeX, one per line or separated by
// spaces ("a1b2c"), and exceptions as hyphenated words ("ta-ble"). left and
// right are the fewest letters a break leaves on either side.
func Parse(patterns, exceptions string, left, right int) (*Patterns, error) {
	p := &Patterns{
		values:     make(map[string][]uint8),
		exceptions: make(map[string][]int),
		left:       max(left, 1),
		right:      max(right, 1),
	}
	for _, pat := range strings.Fields(patterns) {
		var letters strings.Builder
		vals := []uint8{0}
		for _, r := range pat {
			if r >= '0' && r <= '9' {
				vals[len(vals)-1] = uint8(r - '0')
				continue
			}
			letters.WriteRune(r)
			vals = append(vals, 0)
		}
		key := letters.String()
		if key == "" {
			return nil, errors.New("hyphen: pattern without letters: " + pat)
		}
		p.values[key] = vals
		p.longest = max(p.longest, utf8.RuneCountInString(key))
	}
	for _, word := range strings.Fields(exceptions) {
		var points []int
		n := 0
		for _, r := range word {
			if r == '-' {
				points = append(points, n)
				continue
			}
			n++
		}
		p.exceptions[strings.ReplaceAll(word, "-", "")] = points
	}
	return p, nil
}

// Points appends to dst where word may break, in runes from its start: a point
// k means a hyphen between rune k-1 and rune k. Words shorter than the left and
// right minimums together, or with anything but letters, have none.
func (p *Patterns) Points(word string, dst []int) []int {
	lower := []rune(strings.ToLower(word))
	n := len(lower)
	if n < p.left+p.right {
		return dst
	}
	for _, r := range lower {
		if !unicode.IsLetter(r) {
			return dst
		}
	}
	if points, ok := p.exceptions[string(lower)]; ok {
		return append(dst, points...)
	}
	// Liang: every substring of ".word." that is a pattern raises the values
	// between its letters; odd values allow a break.
	dotted := make([]rune, 0, n+2)
	dotted = append(dotted, '.')
	dotted = append(dotted, lower...)
	dotted = append(dotted, '.')
	values := make([]uint8, len(dotted)+1)
	for i := range dotted {
		for j := i + 1; j <= len(dotted) && j-i <= p.longest; j++ {
			vals, ok := p.values[string(dotted[i:j])]
			if !ok {
				continue
			}
			for k, v := range vals {
				values[i+k] = max(values[i+k], v)
			}
		}
	}
	// values[k+1] sits before rune k of the word, which follows the dot.
	for k := p.left; k <= n-p.right; k++ {
		if values[k+1]%2 == 1 {
			dst = append(dst, k)
		}
	}
	return dst
}

// Hyphenate returns word with a hyphen at every point, for tests and tools.
func (p *Patterns) Hyphenate(word string) string {
	points := p.Points(word, nil)
	var b strings.Builder
	k := 0
	for i, r := range []rune(word) {
		if k < len(points) && points[k] == i {
			b.WriteByte('-')
			k++
		}
		b.WriteRune(r)
	}
	return b.String()
}
