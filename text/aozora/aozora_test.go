package aozora

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/text"
)

// marked describes a mark by its text: base and ruby, and the whole notation.
type marked struct{ whole, base, ruby string }

func describe(s string, marks []text.RubyMark) []marked {
	var out []marked
	for _, m := range marks {
		out = append(out, marked{s[m.Start:m.End], s[m.BaseStart:m.BaseEnd], s[m.RubyStart:m.RubyEnd]})
	}
	return out
}

func TestMarks(t *testing.T) {
	for _, tc := range []struct {
		in    string
		want  []marked
		plain string
	}{
		{"", nil, ""},
		{"plain text", nil, "plain text"},
		{"｜漢字《かんじ》", []marked{{"｜漢字《かんじ》", "漢字", "かんじ"}}, "漢字"},
		{"漢字《かんじ》を読む", []marked{{"漢字《かんじ》", "漢字", "かんじ"}}, "漢字を読む"},
		// The implicit base is the run of kanji, not the kana before it.
		{"この漢字《かんじ》", []marked{{"漢字《かんじ》", "漢字", "かんじ"}}, "この漢字"},
		{"佐々木《ささき》と〆切《しめきり》", []marked{
			{"佐々木《ささき》", "佐々木", "ささき"},
			{"〆切《しめきり》", "〆切", "しめきり"},
		}, "佐々木と〆切"},
		{"一ヶ月《いっかげつ》", []marked{{"一ヶ月《いっかげつ》", "一ヶ月", "いっかげつ"}}, "一ヶ月"},
		// A variation selector stays with the kanji it follows.
		{"葛\U000E0100《かつら》", []marked{{"葛\U000E0100《かつら》", "葛\U000E0100", "かつら"}}, "葛\U000E0100"},
		{"葛\U000E0100飾《かつしか》", []marked{{"葛\U000E0100飾《かつしか》", "葛\U000E0100飾", "かつしか"}}, "葛\U000E0100飾"},
		{"神︀社《じんじゃ》", []marked{{"神︀社《じんじゃ》", "神︀社", "じんじゃ"}}, "神︀社"},
		// A selector after kana is not part of the base.
		{"か︀字《じ》", []marked{{"字《じ》", "字", "じ"}}, "か︀字"},
		// A bar lets the base be anything, kana and Latin included.
		{"お｜ABC《エービーシー》だ", []marked{{"｜ABC《エービーシー》", "ABC", "エービーシー"}}, "おABCだ"},
		{"｜東京《とうきょう》｜大阪《おおさか》", []marked{
			{"｜東京《とうきょう》", "東京", "とうきょう"},
			{"｜大阪《おおさか》", "大阪", "おおさか"},
		}, "東京大阪"},
		// The base of the second mark cannot reach into the first.
		{"漢《かん》字《じ》", []marked{
			{"漢《かん》", "漢", "かん"},
			{"字《じ》", "字", "じ"},
		}, "漢字"},
		// No base: kana before 《, or an empty base after ｜.
		{"かな《かな》", nil, "かな《かな》"},
		{"《るび》", nil, "《るび》"},
		{"｜《るび》", nil, "｜《るび》"},
		// No closing mark, or not on the same line.
		{"漢字《かんじ", nil, "漢字《かんじ"},
		{"漢字《かん\nじ》", nil, "漢字《かん\nじ》"},
		{"漢字《かん\r\nじ》", nil, "漢字《かん\r\nじ》"},
		// Empty ruby is plain.
		{"漢字《》", nil, "漢字《》"},
		// A second 《 leaves the first unclosed.
		{"漢《字《じ》", []marked{{"字《じ》", "字", "じ"}}, "漢《字"},
		// The base does not reach back over a line break.
		{"漢\n字《じ》", []marked{{"字《じ》", "字", "じ"}}, "漢\n字"},
		{"｜漢\n字《じ》", []marked{{"字《じ》", "字", "じ"}}, "｜漢\n字"},
		// A bar with no ruby after it stays.
		{"｜漢字です", nil, "｜漢字です"},
	} {
		got := describe(tc.in, Marks(tc.in, nil))
		if len(got) != len(tc.want) {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		} else {
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("%q: mark %d is %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		}
		if p := Plain(tc.in); p != tc.plain {
			t.Errorf("Plain(%q) = %q, want %q", tc.in, p, tc.plain)
		}
	}
}

func TestMarks_Appends(t *testing.T) {
	dst := []text.RubyMark{{Start: -1}}
	dst = Marks("漢《かん》", dst)
	if len(dst) != 2 || dst[0].Start != -1 || dst[1].Start != 0 {
		t.Errorf("got %+v", dst)
	}
}

func TestSpans(t *testing.T) {
	bold := text.Style{Weight: 700}
	for _, tc := range []struct {
		in   string
		want []text.Span
	}{
		{"", []text.Span{}},
		{"plain", []text.Span{{Text: "plain", Style: bold}}},
		{"この漢字《かんじ》を｜ABC《エービーシー》で", []text.Span{
			{Text: "この", Style: bold},
			{Text: "漢字", Style: bold, Ruby: "かんじ"},
			{Text: "を", Style: bold},
			{Text: "ABC", Style: bold, Ruby: "エービーシー"},
			{Text: "で", Style: bold},
		}},
		{"漢《かん》字《じ》", []text.Span{
			{Text: "漢", Style: bold, Ruby: "かん"},
			{Text: "字", Style: bold, Ruby: "じ"},
		}},
		{"かな《かな》\n", []text.Span{{Text: "かな《かな》\n", Style: bold}}},
	} {
		got := Spans(tc.in, bold)
		if len(got) != len(tc.want) || cap(got) != len(got) {
			t.Errorf("%q: got %+v, want %+v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%q: span %d is %+v, want %+v", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestSpans_AllocatesTwoSlices(t *testing.T) {
	s := strings.Repeat("吾輩は猫《ねこ》である。｜名前《なまえ》はまだ無い。\n", 100)
	if n := testing.AllocsPerRun(10, func() { _ = Spans(s, text.Style{}) }); n != 2 {
		t.Errorf("got %v allocs", n)
	}
}

func ExampleSpans() {
	for _, sp := range Spans("吾輩《わがはい》は猫である。", text.Style{}) {
		fmt.Printf("%q %q\n", sp.Text, sp.Ruby)
	}
	// Output:
	// "吾輩" "わがはい"
	// "は猫である。" ""
}

// checkMarks fails if marks are not valid, ordered ranges of s.
func checkMarks(t *testing.T, s string, marks []text.RubyMark) {
	t.Helper()
	prev := 0
	for i, m := range marks {
		ordered := prev <= m.Start && m.Start <= m.BaseStart && m.BaseStart < m.BaseEnd &&
			m.BaseEnd <= m.RubyStart && m.RubyStart <= m.RubyEnd && m.RubyEnd <= m.End && m.End <= len(s)
		if !ordered {
			t.Fatalf("%q: mark %d %+v is out of order", s, i, m)
		}
		ruby := m.RubyEnd > m.RubyStart
		if ruby && (s[m.BaseEnd:m.RubyStart] != openMark || !strings.HasPrefix(s[m.RubyEnd:m.End], closeMark)) {
			t.Fatalf("%q: mark %d %+v misses its brackets", s, i, m)
		}
		if !ruby && (m.Emphasis == text.EmphasisNone || m.RubyStart != m.BaseEnd) {
			t.Fatalf("%q: mark %d %+v has neither ruby nor emphasis", s, i, m)
		}
		if m.End > m.RubyEnd+len(closeMark) || (!ruby && m.End > m.BaseEnd) {
			// The rest of the notation is a note of emphasis, or the
			// mark is a stretch of a longer emphasized text.
			if note := s[max(m.RubyEnd, m.BaseEnd):m.End]; m.Emphasis == text.EmphasisNone ||
				!strings.HasSuffix(note, closeNote) || !strings.Contains(note, openNote) {
				t.Fatalf("%q: mark %d %+v ends in %q, not a note", s, i, m, note)
			}
		}
		if strings.ContainsAny(s[m.Start:m.End], "\r\n") {
			t.Fatalf("%q: mark %d spans lines", s, i)
		}
		prev = m.End
	}
}

func FuzzMarks(f *testing.F) {
	for _, s := range []string{"｜漢字《かんじ》", "漢字《かんじ》を", "《》", "｜《》", "漢《字《じ》", "a\n漢《か\nん》", "\xff《\xfe》",
		"重い荷物《にもつ》を［＃「重い荷物を」に傍点］", "｜青空《あおぞら》［＃「青空」に傍点］", "a［＃「a」に傍点］［＃「a」に傍点］"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		marks := Marks(s, nil)
		checkMarks(t, s, marks)
		p := Plain(s)
		if len(marks) == 0 && p != s {
			t.Fatalf("Plain changed %q without marks", s)
		}
		var joined strings.Builder
		for _, sp := range Spans(s, text.Style{}) {
			joined.WriteString(sp.Text)
		}
		if joined.String() != p {
			t.Fatalf("Spans of %q read %q, Plain %q", s, joined.String(), p)
		}
	})
}

func TestMarks_DoesNotAllocate(t *testing.T) {
	s := strings.Repeat("吾輩は猫《ねこ》である。｜名前《なまえ》はまだ無い。\n", 100)
	dst := make([]text.RubyMark, 0, 256)
	if n := testing.AllocsPerRun(10, func() { dst = Marks(s, dst[:0]) }); n != 0 {
		t.Errorf("got %v allocs", n)
	}
	if len(dst) != 200 {
		t.Errorf("got %d marks", len(dst))
	}
}

func BenchmarkMarks_100kChars(b *testing.B) {
	line := "親譲りの無鉄砲《むてっぽう》で小供の時から損ばかりしている。小学校に居る時分学校の二階から飛び降りて一週間ほど腰を抜かした事がある。"
	s := strings.Repeat(line+"\n", 100000/len([]rune(line)))
	dst := make([]text.RubyMark, 0, 4096)
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		dst = Marks(s, dst[:0])
	}
}
