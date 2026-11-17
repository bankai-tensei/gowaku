package jlreq

import "unicode"

// Class is the character class of a rune, after JLReq Appendix A.
type Class uint8

// The classes.
const (
	// ClassOther is everything without a rule of its own.
	ClassOther Class = iota
	// ClassOpening is an opening bracket.
	ClassOpening
	// ClassClosing is a closing bracket.
	ClassClosing
	// ClassComma is a comma that can hang.
	ClassComma
	// ClassFullStop is a full stop that can hang.
	ClassFullStop
	// ClassMiddleDot is a middle dot, colon or semicolon.
	ClassMiddleDot
	// ClassDividing is an exclamation or question mark.
	ClassDividing
	// ClassHyphen is a hyphen or wave dash.
	ClassHyphen
	// ClassInseparable is a dash or ellipsis, which stays with its own kind.
	ClassInseparable
	// ClassIteration is an iteration mark.
	ClassIteration
	// ClassProlonged is the prolonged sound mark.
	ClassProlonged
	// ClassSmallKana is a small kana.
	ClassSmallKana
	// ClassPrefix is a symbol that comes before a number.
	ClassPrefix
	// ClassPostfix is a unit symbol that comes after a number.
	ClassPostfix
	// ClassKana is a hiragana or katakana of normal size.
	ClassKana
	// ClassIdeograph is a kanji or another character set on the em box.
	ClassIdeograph
	// ClassWestern is a letter or digit of a Western script.
	ClassWestern
	// ClassSpace is a space that disappears at the end of a line.
	ClassSpace
)

// Classify returns the class of r.
func Classify(r rune) Class {
	switch r {
	case '（', '〔', '［', '｛', '〈', '《', '「', '『', '【', '〖', '〘', '〝', '‘', '“', '｟', '«':
		return ClassOpening
	case '）', '〕', '］', '｝', '〉', '》', '」', '』', '】', '〗', '〙', '〟', '’', '”', '｠', '»':
		return ClassClosing
	case '、', '，', '､':
		return ClassComma
	case '。', '．', '｡':
		return ClassFullStop
	case '・', '：', '；', '･':
		return ClassMiddleDot
	case '！', '？', '‼', '⁇', '⁈', '⁉':
		return ClassDividing
	case '‐', '〜', '゠', '–', '～':
		return ClassHyphen
	case '—', '―', '…', '‥', '〳', '〴', '〵':
		return ClassInseparable
	case '々', '〻', 'ゝ', 'ゞ', 'ヽ', 'ヾ':
		return ClassIteration
	case 'ー', 'ｰ':
		return ClassProlonged
	case 'ぁ', 'ぃ', 'ぅ', 'ぇ', 'ぉ', 'っ', 'ゃ', 'ゅ', 'ょ', 'ゎ', 'ゕ', 'ゖ',
		'ァ', 'ィ', 'ゥ', 'ェ', 'ォ', 'ッ', 'ャ', 'ュ', 'ョ', 'ヮ', 'ヵ', 'ヶ',
		'ㇰ', 'ㇱ', 'ㇲ', 'ㇳ', 'ㇴ', 'ㇵ', 'ㇶ', 'ㇷ', 'ㇸ', 'ㇹ', 'ㇺ', 'ㇻ', 'ㇼ', 'ㇽ', 'ㇾ', 'ㇿ':
		return ClassSmallKana
	case '¥', '￥', '$', '＄', '£', '￡', '€', '#', '＃', '№':
		return ClassPrefix
	case '%', '％', '‰', '℃', '℉', '°', '′', '″', '¢', '￠',
		'㎜', '㎝', '㎞', '㎎', '㎏', '㎡', '㎥', '㏄', '㍉', '㌔', '㌘', '㍍', '㌧', '㌫':
		return ClassPostfix
	case ' ', '\t', ' ':
		return ClassSpace
	}
	switch {
	case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
		return ClassKana
	case unicode.Is(unicode.Han, r), r == '　', r >= '０' && r <= '９',
		r >= 'Ａ' && r <= 'Ｚ', r >= 'ａ' && r <= 'ｚ':
		return ClassIdeograph
	case r < 0x2e80 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
		return ClassWestern
	}
	return ClassOther
}

// isDigit reports whether r is a digit of either width.
func isDigit(r rune) bool { return (r >= '0' && r <= '9') || (r >= '０' && r <= '９') }

// isJapanese reports whether c is set on the em box as Japanese text, as
// opposed to Western text and symbols.
func isJapanese(c Class) bool {
	switch c {
	case ClassKana, ClassIdeograph, ClassSmallKana, ClassIteration, ClassProlonged:
		return true
	}
	return false
}

// endsWithBlank reports whether c has its ink on the left half of the em box
// and blank space on the right.
func endsWithBlank(c Class) bool {
	return c == ClassClosing || c == ClassComma || c == ClassFullStop
}
