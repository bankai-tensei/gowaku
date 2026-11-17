package aozora

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/text"
)

// emphasized describes a mark by its text: the whole notation, the base, the
// ruby and the emphasis.
type emphasized struct {
	whole, base, ruby string
	mark              text.Emphasis
}

func TestMarks_Emphasis(t *testing.T) {
	const sesame, circle = text.EmphasisSesame, text.EmphasisCircle
	for _, tc := range []struct {
		in    string
		want  []emphasized
		plain string
	}{
		{"対象［＃「対象」に傍点］です", []emphasized{{"対象［＃「対象」に傍点］", "対象", "", sesame}}, "対象です"},
		{"この語［＃「語」に丸傍点］", []emphasized{{"語［＃「語」に丸傍点］", "語", "", circle}}, "この語"},
		{"白［＃「白」に白ゴマ傍点］黒［＃「黒」に黒三角傍点］", []emphasized{
			{"白［＃「白」に白ゴマ傍点］", "白", "", text.EmphasisOpenSesame},
			{"黒［＃「黒」に黒三角傍点］", "黒", "", text.EmphasisTriangle},
		}, "白黒"},
		// The named text is read without its ruby, which it keeps.
		{"｜青空《あおぞら》［＃「青空」に傍点］", []emphasized{{"｜青空《あおぞら》［＃「青空」に傍点］", "青空", "あおぞら", sesame}}, "青空"},
		{"漢字《かんじ》［＃「漢字」に傍点］", []emphasized{{"漢字《かんじ》［＃「漢字」に傍点］", "漢字", "かんじ", sesame}}, "漢字"},
		// Text over ruby and text around it: one mark per stretch, the last
		// with the note.
		{"重い荷物《にもつ》を［＃「重い荷物を」に傍点］", []emphasized{
			{"重い", "重い", "", sesame},
			{"荷物《にもつ》", "荷物", "にもつ", sesame},
			{"を［＃「重い荷物を」に傍点］", "を", "", sesame},
		}, "重い荷物を"},
		{"前の荷物《にもつ》［＃「の荷物」に傍点］", []emphasized{
			{"の", "の", "", sesame},
			{"荷物《にもつ》［＃「の荷物」に傍点］", "荷物", "にもつ", sesame},
		}, "前の荷物"},
		// A base after a note does not reach back into it.
		{"漢字［＃「漢字」に傍点］漢字《かんじ》", []emphasized{
			{"漢字［＃「漢字」に傍点］", "漢字", "", sesame},
			{"漢字《かんじ》", "漢字", "かんじ", 0},
		}, "漢字漢字"},
		// Plain text: the named text is not just before the note, it starts
		// inside a base, it reaches back into an earlier note or over a
		// line, the note is not one of emphasis, or it is not closed.
		{"対象です［＃「対象」に傍点］", nil, "対象です［＃「対象」に傍点］"},
		{"｜青空《あおぞら》［＃「空」に傍点］", []emphasized{{"｜青空《あおぞら》", "青空", "あおぞら", 0}}, "青空［＃「空」に傍点］"},
		{"ab［＃「b」に傍点］［＃「ab」に傍点］", []emphasized{{"b［＃「b」に傍点］", "b", "", sesame}}, "ab［＃「ab」に傍点］"},
		{"a\nb［＃「a\nb」に傍点］", nil, "a\nb［＃「a\nb」に傍点］"},
		{"改［＃改ページ］", nil, "改［＃改ページ］"},
		{"対象［＃「対象」の左に傍点］", nil, "対象［＃「対象」の左に傍点］"},
		{"対象［＃「対象」に傍線］", nil, "対象［＃「対象」に傍線］"},
		{"対象［＃「対象」に傍点", nil, "対象［＃「対象」に傍点"},
		{"［＃「」に傍点］", nil, "［＃「」に傍点］"},
		{"対象［＃「長い対象」に傍点］", nil, "対象［＃「長い対象」に傍点］"},
	} {
		marks := Marks(tc.in, nil)
		checkMarks(t, tc.in, marks)
		var got []emphasized
		for _, m := range marks {
			got = append(got, emphasized{tc.in[m.Start:m.End], tc.in[m.BaseStart:m.BaseEnd], tc.in[m.RubyStart:m.RubyEnd], m.Emphasis})
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%q: got %+v, want %+v", tc.in, got, tc.want)
		}
		if p := Plain(tc.in); p != tc.plain {
			t.Errorf("Plain(%q) = %q, want %q", tc.in, p, tc.plain)
		}
	}
}

func TestSpans_Emphasis(t *testing.T) {
	got := Spans("この｜青空《あおぞら》［＃「青空」に傍点］を見る［＃「見る」に丸傍点］。", text.Style{})
	want := []text.Span{
		{Text: "この"},
		{Text: "青空", Ruby: "あおぞら", Emphasis: text.EmphasisSesame},
		{Text: "を"},
		{Text: "見る", Emphasis: text.EmphasisCircle},
		{Text: "。"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestMarks_EmphasisDoesNotAllocate(t *testing.T) {
	s := strings.Repeat("重い荷物《にもつ》を［＃「重い荷物を」に傍点］運ぶ。\n", 100)
	dst := make([]text.RubyMark, 0, 512)
	if n := testing.AllocsPerRun(10, func() { dst = Marks(s, dst[:0]) }); n != 0 {
		t.Errorf("got %v allocs", n)
	}
	if len(dst) != 300 {
		t.Errorf("got %d marks", len(dst))
	}
}
