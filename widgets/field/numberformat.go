package field

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// numberSymbols are the separators a locale writes numbers with. A zero group
// means the locale does not group digits.
type numberSymbols struct {
	decimal, group rune
}

// numberSample is the number whose spelling by the locale gives its
// separators: big enough to be grouped even where four digits are not.
const numberSample = 1234567.5

// symbolsFrom reads the separators out of numberSample as the locale spells
// it with one digit after the decimal separator: 1,234,567.5 in English,
// 1.234.567,5 in German, 1 234 567,5 in French.
func symbolsFrom(sample string) numberSymbols {
	sym := numberSymbols{decimal: '.', group: ','}
	runes := make([]rune, 0, len(sample))
	for _, r := range sample {
		// Leave out the marks of direction that a locale may write.
		if !unicode.Is(unicode.Cf, r) {
			runes = append(runes, r)
		}
	}
	if len(runes) < 4 {
		return sym
	}
	sym.decimal = runes[len(runes)-2]
	if g := runes[1]; digitValue(g) < 0 {
		sym.group = g
	} else {
		sym.group = 0
	}
	return sym
}

// formatNumber spells v with digits digits after the decimal separator of
// sym, and with its group separators if grouping is set. The digits are the
// ASCII ones.
func formatNumber(v float64, digits int, sym numberSymbols, grouping bool) string {
	s := strconv.FormatFloat(v, 'f', max(digits, 0), 64)
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	if strings.Trim(whole+frac, "0") == "" {
		// No minus before a zero that rounding made of a small negative.
		negative = false
	}
	var b strings.Builder
	if negative {
		b.WriteByte('-')
	}
	for i := range len(whole) {
		if i > 0 && grouping && sym.group != 0 && (len(whole)-i)%3 == 0 {
			b.WriteRune(sym.group)
		}
		b.WriteByte(whole[i])
	}
	if frac != "" {
		b.WriteRune(sym.decimal)
		b.WriteString(frac)
	}
	return b.String()
}

// The classes of the characters of a number being read.
const (
	classDigit     = 'd'
	classSign      = 's'
	classDecimal   = 'D'
	classGroup     = 'G'
	classAlternate = 'A'
)

// parseNumber reads s as a number written with the separators of sym. It
// accepts what people type besides: full-width digits and signs from an IME,
// the long vowel mark ー that a Japanese IME makes of a minus, Arabic-Indic
// and Devanagari digits, any space as the group separator of a locale that
// groups with a space, and spaces around the number. A lone '.' or ',' that is
// not the decimal separator of the locale counts as one when the text has no
// other, and so does a group separator that is not followed by three digits:
// "1.5" is one and a half in German, and "1,5" in English. Exponents are not
// numbers here.
func parseNumber(s string, sym numberSymbols) (float64, bool) {
	s = strings.TrimFunc(s, unicode.IsSpace)
	if s == "" {
		return 0, false
	}
	// Each character becomes a class and, for digits and signs, its ASCII
	// form.
	classes := make([]byte, 0, len(s))
	ascii := make([]byte, 0, len(s))
	for _, r := range s {
		c, a := classify(r, sym)
		if c == 0 {
			return 0, false
		}
		classes, ascii = append(classes, c), append(ascii, a)
	}
	// A sign comes first or not at all.
	start := 0
	if classes[0] == classSign {
		start = 1
	}
	for _, c := range classes[start:] {
		if c == classSign {
			return 0, false
		}
	}
	decimals, alternates := 0, 0
	for _, c := range classes {
		switch c {
		case classDecimal:
			decimals++
		case classAlternate:
			alternates++
		}
	}
	switch {
	case decimals > 1, decimals == 1 && alternates > 0, alternates > 1:
		return 0, false
	case alternates == 1:
		classes[indexOfClass(classes, classAlternate)] = classDecimal
		decimals = 1
	}
	if decimals == 0 {
		// A single loose group separator of a dot or a comma is the decimal
		// separator the person meant.
		if i := indexOfClass(classes, classGroup); i >= 0 && indexOfClass(classes[i+1:], classGroup) < 0 &&
			digitsAfter(classes, i) != 3 && (ascii[i] == '.' || ascii[i] == ',') {
			classes[i] = classDecimal
		}
	}
	out := make([]byte, 0, len(classes))
	seenDecimal, digits := false, 0
	for i, c := range classes {
		switch c {
		case classSign:
			out = append(out, ascii[i])
		case classDigit:
			out = append(out, ascii[i])
			digits++
		case classDecimal:
			seenDecimal = true
			out = append(out, '.')
		case classGroup:
			// A group separator sits between digits of the whole part, with
			// three digits after it.
			if seenDecimal || i == start || classes[i-1] != classDigit || digitsAfter(classes, i) != 3 {
				return 0, false
			}
		}
	}
	if digits == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(string(out), 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	return v, true
}

// classify returns the class of r and its ASCII form, or a zero class for a
// character that has no place in a number.
func classify(r rune, sym numberSymbols) (class, ascii byte) {
	r = narrow(r)
	if d := digitValue(r); d >= 0 {
		return classDigit, byte('0' + d)
	}
	switch r {
	case '+':
		return classSign, '+'
	case '-', '−', '‐', '‒', '–', 'ー':
		return classSign, '-'
	case '٫':
		return classDecimal, '.'
	case '٬':
		return classGroup, ','
	}
	switch {
	case r == narrow(sym.decimal):
		return classDecimal, byte(r)
	case sym.group != 0 && (r == narrow(sym.group) || spacey(sym.group) && spacey(r) || quotey(sym.group) && quotey(r)):
		return classGroup, byte(min(r, utf8.RuneSelf-1))
	case r == '.' || r == ',':
		return classAlternate, byte(r)
	}
	return 0, 0
}

// narrow turns a full-width form, as a Japanese IME types, into ASCII, and the
// ideographic space into a space.
func narrow(r rune) rune {
	switch {
	case r >= '！' && r <= '～':
		return r - 0xfee0
	case r == '　':
		return ' '
	}
	return r
}

// spacey reports whether r is a space that separates groups of digits.
func spacey(r rune) bool {
	return r == ' ' || r == ' ' || r == ' ' || r == ' '
}

// quotey reports whether r is an apostrophe that separates groups of digits,
// as in Switzerland.
func quotey(r rune) bool { return r == '\'' || r == '’' }

// digitValue returns the value of a decimal digit, or -1.
func digitValue(r rune) int {
	for _, zero := range [...]rune{'0', '٠', '۰', '०'} {
		if r >= zero && r <= zero+9 {
			return int(r - zero)
		}
	}
	return -1
}

func indexOfClass(classes []byte, c byte) int {
	for i, x := range classes {
		if x == c {
			return i
		}
	}
	return -1
}

// digitsAfter counts the digits that follow classes[i] up to the next
// character that is not one.
func digitsAfter(classes []byte, i int) int {
	n := 0
	for _, c := range classes[i+1:] {
		if c != classDigit {
			break
		}
		n++
	}
	return n
}

// roundTo rounds v to digits digits after the point, half away from zero.
func roundTo(v float64, digits int) float64 {
	if digits < 0 || digits > 15 {
		return v
	}
	p := math.Pow10(digits)
	r := math.Round(v*p) / p
	if math.IsInf(r, 0) || math.IsNaN(r) {
		return v
	}
	return r
}

// decimalsOf returns the number of digits after the point that step needs,
// up to six: 0 for 1, 1 for 0.5, 2 for 0.25.
func decimalsOf(step float64) int {
	for d := range 7 {
		if math.Abs(roundTo(step, d)-step) < 1e-9*math.Max(1, math.Abs(step)) {
			return d
		}
	}
	return 6
}
