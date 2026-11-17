package text

// RubyMark is ruby or emphasis written into a text with a notation, such as
// the ｜漢字《かんじ》 or the 対象［＃「対象」に傍点］ of Aozora Bunko: the byte
// ranges of the whole notation, of the base text and of the ruby. A TextField
// can draw the ruby or the emphasis marks beside the base while the text keeps
// the notation (field.Input.Ruby). Package text/aozora finds the marks of the
// Aozora notation.
type RubyMark struct {
	// Start and End bound the whole notation, from the opening mark or the
	// first character of the base to the closing mark of the ruby or of the
	// note.
	Start, End int
	// BaseStart and BaseEnd bound the base text, which lies in Start..End.
	BaseStart, BaseEnd int
	// RubyStart and RubyEnd bound the ruby, which lies in Start..End after the
	// base. They are equal for a mark of emphasis only.
	RubyStart, RubyEnd int
	// Emphasis sets emphasis marks beside the base, as Span.Emphasis does.
	Emphasis Emphasis
}
