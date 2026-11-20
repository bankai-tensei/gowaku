package jlreq

import (
	"strings"
	"testing"
)

// em is the font size of the made-up metrics: Japanese is one em wide, Western
// half an em. Widths in the tests are in ems.
const em = 10

func units(s string) []Unit {
	var us []Unit
	rs := []rune(s)
	for i, r := range rs {
		u := Unit{Rune: r, Em: em, Advance: em, FullWidth: true}
		c := Classify(r)
		if c == ClassWestern || c == ClassSpace || (r < 0x80) {
			u.Advance, u.FullWidth = em/2, false
		}
		// A stand-in for UAX #14: break after spaces and around Japanese.
		if i+1 < len(rs) {
			next := Classify(rs[i+1])
			u.BreakAfter = c == ClassSpace || (c != ClassWestern && r >= 0x80) || (next != ClassWestern && rs[i+1] >= 0x80)
		}
		us = append(us, u)
	}
	return us
}

func wrap(s string, widthInEm float32, opts Options) []string {
	us := units(s)
	rs := []rune(s)
	var b Breaker
	b.Break(us, widthInEm*em, opts)
	lines := make([]string, 0, len(b.Lines))
	for _, l := range b.Lines {
		lines = append(lines, string(rs[l.Start:l.End]))
	}
	return lines
}

func TestJLReq_LineBreaks(t *testing.T) {
	loose, normal := Options{Strictness: Loose}, Options{Strictness: Normal}
	hang := Options{Hanging: true}
	oikomi := Options{Kinsoku: Oikomi}
	tests := []struct {
		name  string
		text  string
		width float32
		opts  Options
		want  string // lines joined with |
	}{
		{"折り返しなし", "あいう", 5, Options{}, "あいう"},
		{"幅ちょうど", "あいうえ", 2, Options{}, "あい|うえ"},
		{"幅ゼロでも 1 文字は置く", "あい", 0, Options{}, "あ|い"},

		{"行頭禁則: 句点", "あいう。えお", 3, Options{}, "あい|う。え|お"},
		{"行頭禁則: 読点", "あいう、えお", 3, Options{}, "あい|う、え|お"},
		{"行頭禁則: 閉じ括弧", "あい（う）えお", 4, Options{}, "あい|（う）え|お"},
		{"行頭禁則: かぎ括弧閉じ", "あいう」えお", 3, Options{}, "あい|う」え|お"},
		{"行頭禁則: 感嘆符", "あいう！えお", 3, Options{}, "あい|う！え|お"},
		{"行頭禁則: 中点", "あいう・えお", 3, Options{}, "あい|う・え|お"},
		{"行頭禁則: 長音と拗音の連続", "コンピュータ", 4, Options{}, "コン|ピュータ"},
		{"行頭禁則: 拗音", "あいうっえ", 3, Options{}, "あい|うっえ"},
		{"行頭禁則: 々", "あいう々え", 3, Options{}, "あい|う々え"},
		{"行頭禁則: 連続", "あいう」。えお", 3, Options{}, "あい|う」。|えお"},

		{"normal は小書きかなを行頭に許す", "あいうっえ", 3, normal, "あいう|っえ"},
		{"normal は長音を行頭に許す", "あいうーえ", 3, normal, "あいう|ーえ"},
		{"normal は々を許さない", "あいう々え", 3, normal, "あい|う々え"},
		{"loose は々を行頭に許す", "あいう々え", 3, loose, "あいう|々え"},
		{"loose でも句点は許さない", "あいう。え", 3, loose, "あい|う。え"},

		{"行末禁則: 始め括弧", "あい「うえお", 3, Options{}, "あい|「うえ|お"},
		{"行末禁則: 二重括弧", "あい『うえお", 3, Options{}, "あい|『うえ|お"},
		{"行末禁則: 始め括弧の連続", "あ「（うえお", 3, Options{}, "あ|「（うえ|お"},

		{"分離禁止: ダッシュ", "あいう——えお", 4, Options{}, "あいう|——えお"},
		{"分離禁止: 三点リーダ", "あいう……えお", 4, Options{}, "あいう|……えお"},
		{"分離禁止: 二点リーダ", "あいう‥‥えお", 4, Options{}, "あいう|‥‥えお"},
		{"分離禁止: 数字と単位", "あいう１００％えお", 6, Options{}, "あいう|１００％えお"},
		{"分離禁止: 数字と℃", "あいうえ3℃おか", 5.5, Options{}, "あいうえ|3℃おか"},
		{"分離禁止: 通貨と数字", "あいう￥１００", 4, Options{}, "あいう|￥１００"},

		{"欧単語は分割しない", "あいう hello world", 5, Options{}, "あいう |hello |world"},
		{"長すぎる欧単語は割る", "abcdefgh", 2, Options{}, "abcd|efgh"},
		{"行末の空白は幅に数えない", "ab cd", 1, Options{}, "ab |cd"},

		{"ぶら下げ: 句点", "あいう。えお", 3, hang, "あいう。|えお"},
		{"ぶら下げ: 読点", "あいう、えお", 3, hang, "あいう、|えお"},
		{"ぶら下げ: 閉じ括弧は下げない", "あいう」えお", 3, hang, "あい|う」え|お"},
		{"ぶら下げ: 次が禁則なら下げない", "あいう。」えお", 3, hang, "あい|う。」|えお"},
		{"ぶら下げ: 収まるなら下げない", "あい。うえお", 3, hang, "あい。|うえお"},

		{"追い込み: 約物の空きで吸収", "あ、い。うえお", 3.5, oikomi, "あ、い。|うえお"},
		{"追い込み: 吸収できなければ追い出す", "あいう。えお", 3, oikomi, "あい|う。え|お"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(wrap(tt.text, tt.width, tt.opts), "|")
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// positions lays s out on one line and returns the unit positions in ems.
func positions(s string, opts Options) []float32 {
	var b Breaker
	b.Break(units(s), 1000*em, opts)
	out := make([]float32, len(b.X))
	for i, x := range b.X {
		out[i] = x / em
	}
	return out
}

func TestJLReq_Spacing(t *testing.T) {
	tests := []struct {
		name string
		text string
		opts Options
		want []float32
	}{
		{"詰めなし", "あい", Options{}, []float32{0, 1}},
		{"閉じ括弧と句点", "あ」。い", Options{}, []float32{0, 1, 1.5, 2.5}},
		{"句点と閉じ括弧", "あ。」い", Options{}, []float32{0, 1, 1.5, 2.5}},
		{"読点と始め括弧", "あ、「い", Options{}, []float32{0, 1, 1.5, 2.5}},
		{"閉じ括弧と始め括弧", "」「", Options{}, []float32{0, 0.5}},
		{"始め括弧の連続", "あ「「い", Options{}, []float32{0, 1, 1.5, 2.5}},
		{"閉じ括弧と和字は詰めない", "」あ", Options{}, []float32{0, 1}},
		{"詰めを切る", "あ」。い", Options{Punctuation: PunctuationFull}, []float32{0, 1, 2, 3}},

		{"行頭の始め括弧", "「あ", Options{}, []float32{-0.5, 0.5}},
		{"行頭の始め括弧を切る", "「あ", Options{Punctuation: PunctuationFull}, []float32{0, 1}},

		{"和欧間: 和→欧", "あa", Options{}, []float32{0, 1.25}},
		{"和欧間: 欧→和", "aあ", Options{}, []float32{0, 0.75}},
		{"和欧間: 数字", "第1回", Options{}, []float32{0, 1.25, 2}},
		{"和欧間: 空白があれば足さない", "あ a", Options{}, []float32{0, 1, 1.5}},
		{"和欧間: 約物には足さない", "」a", Options{}, []float32{0, 1}},
		{"和欧間を切る", "あa", Options{WesternSpacing: WesternNone}, []float32{0, 1}},
		{"和欧間: 全角スペースには足さない", "　a　", Options{}, []float32{0, 1, 1.5}},
		{"和欧間: ノーブレークスペースには足さない", "あ a", Options{}, []float32{0, 1, 1.5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := positions(tt.text, tt.opts)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// A unit whose ends differ in class, as the base of group ruby, meets the
// unit before it by its first rune and the unit after it by its last.
func TestJLReq_GroupMeetsItsNeighborsByItsEnds(t *testing.T) {
	group := func(first, last rune) Unit {
		return Unit{Rune: first, Last: last, Em: em, Advance: 2 * em, BreakAfter: true}
	}
	kana := Unit{Rune: 'あ', Em: em, Advance: em, FullWidth: true, BreakAfter: true}
	for _, tc := range []struct {
		name  string
		units []Unit
		want  float32 // position of the last unit, in em
	}{
		{"Latin then kanji, kana after", []Unit{group('A', '代'), kana}, 2},
		{"kanji then Latin, kana after", []Unit{group('時', 'I'), kana}, 2.25},
		{"kana before Latin then kanji", []Unit{kana, group('A', '代')}, 1.25},
		{"kana before kanji then Latin", []Unit{kana, group('時', 'I')}, 1},
	} {
		var b Breaker
		b.Break(tc.units, 100*em, Options{})
		if got := b.X[len(tc.units)-1] / em; got != tc.want {
			t.Errorf("%s: at %v, want %v", tc.name, got, tc.want)
		}
	}
	// A group that ends with an opening bracket does not end a line.
	us := []Unit{kana, group('漢', '「'), kana}
	var b Breaker
	b.Break(us, 3.5*em, Options{})
	if len(b.Lines) != 2 || b.Lines[0].End != 1 {
		t.Errorf("got lines %+v, want a break before the group", b.Lines)
	}
}

func TestJLReq_WesternSpaceDisappearsAtLineEdges(t *testing.T) {
	var b Breaker
	us := units("あいab")
	b.Break(us, 2*em, Options{})
	if len(b.Lines) != 2 {
		t.Fatalf("got %d lines", len(b.Lines))
	}
	if got := b.Lines[0].Width; got != 2*em {
		t.Errorf("first line is %v wide, want %v", got, 2*em)
	}
	if got := b.X[2]; got != 0 {
		t.Errorf("second line starts at %v, want 0", got)
	}
}

func TestJLReq_LineStartBracketWidth(t *testing.T) {
	var b Breaker
	b.Break(units("あいう「えお"), 3*em, Options{})
	line := b.Lines[1]
	if b.X[line.Start] != -em/2 {
		t.Errorf("bracket at %v, want %v", b.X[line.Start], -em/2)
	}
	if line.Width != 2.5*em {
		t.Errorf("width %v, want %v", line.Width, 2.5*em)
	}
}

func TestJLReq_HangReportsOverflow(t *testing.T) {
	var b Breaker
	b.Break(units("あいう。えお"), 3*em, Options{Hanging: true})
	l := b.Lines[0]
	if l.Width != 3*em || l.Hang != em {
		t.Errorf("width %v hang %v, want %v and %v", l.Width, l.Hang, 3*em, em)
	}
}

func TestJLReq_OikomiFitsTheWidth(t *testing.T) {
	var b Breaker
	us := units("あ、い。うえお")
	b.Break(us, 3.5*em, Options{Kinsoku: Oikomi})
	l := b.Lines[0]
	if l.End != 4 {
		t.Fatalf("line ends at %d, want 4", l.End)
	}
	// The ink of the full stop is its left half.
	if got := b.X[3] + em/2; got > 3.5*em {
		t.Errorf("ink ends at %v, past %v", got, 3.5*em)
	}
}

// The space after the last character of a line disappears at the line end, so
// Oikomi cannot squeeze it to make room.
func TestJLReq_OikomiDoesNotCountTheSpaceAfterTheLine(t *testing.T) {
	var b Breaker
	// 々 must not start a line, and a quarter em of space would follow it
	// before the A. Only that space could give, and it is past the line end.
	us := units("あああ々A")
	b.Break(us, 3.9*em, Options{Kinsoku: Oikomi})
	l := b.Lines[0]
	if l.End == 4 {
		t.Fatalf("々 was pushed in: the line is %v wide, over %v", l.Width, 3.9*em)
	}
	if l.Width > 3.9*em {
		t.Errorf("line is %v wide, over %v", l.Width, 3.9*em)
	}
}

func TestJLReq_MinWidth(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want float32 // in ems
	}{
		{"", 0},
		{"banana", 3},
		{"a long yellow fruit", 3}, // "yellow"
		{"100", 1.5},               // digits stay together
		{"りんご", 1},                 // any ideograph boundary breaks
		{"赤い果物。", 2},               // the full stop must not start a line
		{"word    ", 2},            // trailing spaces do not count
		{"「かぎ」", 2},                // 「か is 1.5 (the bracket loses its blank half at the line start); ぎ」 is 2
	} {
		var b Breaker
		got := b.MinWidth(units(tc.s), Options{Punctuation: PunctuationSqueeze})
		if got != tc.want*em {
			t.Errorf("MinWidth(%q) = %v em, want %v", tc.s, got/em, tc.want)
		}
		b.Break(units(tc.s), got, Options{Punctuation: PunctuationSqueeze})
		for _, l := range b.Lines {
			if l.Hyphen {
				t.Errorf("%q at its min width breaks inside a word", tc.s)
			}
		}
	}
}

func TestJLReq_MinWidthDoesNotAllocate(t *testing.T) {
	var b Breaker
	us := units(strings.Repeat("あいう、えお。 word ", 50))
	b.MinWidth(us, Options{})
	if allocs := testing.AllocsPerRun(10, func() { b.MinWidth(us, Options{}) }); allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func TestJLReq_ReusesBuffers(t *testing.T) {
	var b Breaker
	us := units(strings.Repeat("あいう、えお。", 50))
	b.Break(us, 20*em, Options{})
	allocs := testing.AllocsPerRun(10, func() { b.Break(us, 20*em, Options{}) })
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		r    rune
		want Class
	}{
		{'「', ClassOpening}, {'』', ClassClosing}, {'、', ClassComma}, {'。', ClassFullStop},
		{'・', ClassMiddleDot}, {'！', ClassDividing}, {'〜', ClassHyphen}, {'…', ClassInseparable},
		{'々', ClassIteration}, {'ー', ClassProlonged}, {'っ', ClassSmallKana}, {'ャ', ClassSmallKana},
		{'￥', ClassPrefix}, {'％', ClassPostfix}, {'か', ClassKana}, {'カ', ClassKana},
		{'漢', ClassIdeograph}, {'　', ClassIdeograph}, {'１', ClassIdeograph},
		{'a', ClassWestern}, {'7', ClassWestern}, {'é', ClassWestern}, {' ', ClassSpace}, {'(', ClassOther},
	}
	for _, tt := range tests {
		if got := Classify(tt.r); got != tt.want {
			t.Errorf("%q: got %d, want %d", tt.r, got, tt.want)
		}
	}
}

// A unit with StartExtra or EndExtra takes that room only where it starts or
// ends a line.
func TestBreak_LineEdgeExtra(t *testing.T) {
	us := units("あいうえお")
	us[2].StartExtra, us[2].EndExtra = em/2, em/2
	var b Breaker
	b.Break(us, 10*em, Options{})
	if len(b.Lines) != 1 || b.Advance[2] != em || b.Lines[0].Width != 5*em {
		t.Errorf("inside a line: lines %+v, advance %v", b.Lines, b.Advance[2])
	}
	// う needs 3.5 em to end the first line, so it starts the second.
	b.Break(us, 3*em, Options{})
	if len(b.Lines) < 2 || b.Lines[1].Start != 2 {
		t.Fatalf("got %+v", b.Lines)
	}
	if b.Advance[2] != 1.5*em || b.X[3] != 1.5*em {
		t.Errorf("at the head: advance %v, next at %v", b.Advance[2], b.X[3])
	}
	b.Break(us, 3.5*em, Options{})
	if len(b.Lines) != 2 || b.Lines[0].End != 3 || b.Lines[0].Width != 3.5*em {
		t.Errorf("at the end: %+v", b.Lines)
	}
	if w := b.MinWidth(us, Options{}); w != 2*em {
		t.Errorf("min width %v", w)
	}
}
