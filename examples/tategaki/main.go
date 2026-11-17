// Command tategaki shows vertical Japanese text: the opening of Natsume
// Soseki's Botchan (1906, public domain, from Aozora Bunko) set in columns
// with ruby, tate-chu-yoko and Western text turned sideways, next to the same
// text set horizontally and justified.
//
//	tategaki                  opens the window
//	tategaki -png out.png     draws one frame to a file and exits
//	tategaki -lines -height 448 [-vertical] [-justify]
//	                          prints where stdin breaks into lines, for
//	                          comparing with other typesetting programs
package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"strings"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
	"github.com/bankai-tensei/gowaku/text/jlreq"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

const (
	windowWidth  = 1280
	windowHeight = 820
	fontSize     = 18
	// columnLength is the height of a box: columns of 30 em, plus the padding.
	columnLength = 30*fontSize + 24
)

// botchan is the opening of Botchan in the notation of Aozora Bunko: ruby in
// 《》 after its base, which is the run of kanji before it or starts at ｜.
const botchan = `親譲《おやゆず》りの無鉄砲《むてっぽう》で小供《こども》の時から損ばかりしている。小学校に居《お》る時分《じぶん》学校の二階から飛び降りて一週間ほど腰《こし》を抜《ぬ》かした事がある。なぜそんな無闇《むやみ》をしたと聞く人があるかも知れぬ。別段深い理由でもない。新築の二階から首を出していたら、同級生の一人が冗談《じょうだん》に、いくら威張《いば》っても、そこから飛び降りる事は出来まい。弱虫やーい。と囃《はや》したからである。小使《こづかい》に負《お》ぶさって帰って来た時、おやじが大きな眼《め》をして二階ぐらいから飛び降りて腰を抜かす奴《やつ》があるかと云《い》ったから、この次は抜かさずに飛んで見せますと答えた。
親類のものから西洋製のナイフを貰《もら》って奇麗《きれい》な刃《は》を日に翳《かざ》して、友達《ともだち》に見せていたら、一人が光る事は光るが切れそうもないと云った。切れぬ事があるか、何でも切ってみせると受け合った。そんなら君の指を切ってみろと注文したから、何だ指ぐらいこの通りだと右の手の親指の甲《こう》をはすに切り込《こ》んだ。幸《さいわい》ナイフが小さいのと、親指の骨が堅《かた》かったので、今だに親指は手に付いている。しかし創痕《きずあと》は死ぬまで消えぬ。`

// colophon mixes the cases that vertical text treats apart: digits set
// horizontally, Western words turned sideways, and an upright abbreviation.
func colophon() []text.Span {
	return []text.Span{
		{Text: "明治"},
		{Text: "39", TateChuYoko: true},
		{Text: "年"},
		{Text: "4", TateChuYoko: true},
		{Text: "月、雑誌「ホトトギス」に発表。"},
		{Text: "gowaku", Style: text.Style{}},
		{Text: "は"},
		{Text: "GPU", Upright: true},
		{Text: "で縦書きを描く（"},
		{Text: "100", TateChuYoko: true},
		{Text: "％自前の組版）。"},
	}
}

type page struct{ gowaku.StatefulBase }

type pageState struct {
	gowaku.StateBase
	draft string
}

// CreateState implements core.StatefulWidget.
func (page) CreateState() gowaku.State {
	return &pageState{draft: "縦書きで入力。下キーで次の字、左キーで次の行に移る。"}
}

// Build implements core.State.
func (s *pageState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	paragraphs := strings.Split(botchan, "\n")
	vertical := text.Style{Size: fontSize, Orientation: text.Vertical, Indent: fontSize, Align: text.AlignJustify, Kinsoku: jlreq.Oikomi, Family: "Yu Mincho"}
	columns := make([]gowaku.Widget, 0, len(paragraphs)+1)
	// Vertical text starts on the right, so the Row runs backwards.
	columns = append(columns, basic.RichText{Spans: colophon(), Style: vertical})
	for i := len(paragraphs) - 1; i >= 0; i-- {
		columns = append(columns, basic.RichText{Spans: aozora.Spans(paragraphs[i], text.Style{}), Style: vertical})
	}
	horizontal := text.Style{Size: 15, Indent: 15, Align: text.AlignJustify, Kinsoku: jlreq.Oikomi}
	box := func(caption string, child gowaku.Widget) gowaku.Widget {
		return basic.Column{Spacing: 6, Cross: layout.CrossStart, Children: []gowaku.Widget{
			basic.Text{Content: caption, Style: text.Style{Size: 12, Color: th.Color.Ink2}},
			basic.Container{Border: paint.Stroke{Width: 1, Color: th.Color.Hair}, Padding: geom.All(12), Child: child},
		}}
	}
	return basic.Pad(28, basic.Row{Spacing: 32, Cross: layout.CrossStart, Children: []gowaku.Widget{
		box("横組・両端揃え・字下げ（Yu Gothic 15px）", basic.SizedBox{Width: 300, Child: basic.Column{Spacing: 8, Cross: layout.CrossStretch, Children: []gowaku.Widget{
			basic.RichText{Spans: aozora.Spans(paragraphs[0], text.Style{}), Style: horizontal},
			basic.RichText{Spans: aozora.Spans(paragraphs[1], text.Style{}), Style: horizontal},
		}}}),
		// The columns take the room left over and scroll sideways when the
		// window is too narrow for them. Reverse opens the Scroll at the right,
		// where the text starts, and keeps a text narrower than the box there.
		basic.Expanded{Child: box("縦組・両端揃え・字下げ・ルビ・縦中横（Yu Mincho 18px）", basic.SizedBox{Height: columnLength, Child: basic.Scroll{
			Horizontal: true, Reverse: true,
			Child: basic.Row{Size: layout.MainMin, Spacing: fontSize, Cross: layout.CrossStart, Children: columns},
		}})},
		box("縦書きの入力", basic.SizedBox{Height: columnLength, Child: field.Input{
			Value:     s.draft,
			Multiline: true,
			Style:     text.Style{Size: 16, Orientation: text.Vertical},
			OnChange:  func(v string) { s.SetState(func() { s.draft = v }) },
		}}),
	}})
}

// printLines breaks each line of r at length and prints the result, one output
// line per line of text.
func printLines(r io.Reader, length float32, st text.Style) error {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		return err
	}
	in := bufio.NewScanner(r)
	in.Buffer(nil, 1<<20)
	var out strings.Builder
	for in.Scan() {
		content := aozora.Plain(strings.TrimLeft(in.Text(), string(rune(0xfeff))))
		l := shaper.Shape(content, st).Layout(length)
		for i := range l.LineCount() {
			a, b := l.LineRange(i)
			out.WriteString(strings.TrimRight(content[a:b], " "))
			out.WriteByte('\n')
		}
		out.WriteByte('\n')
	}
	if err := in.Err(); err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(out.String())
	return err
}

func screenshot(path string) error {
	h, err := shot.Open(page{}, shot.Options{Width: windowWidth, Height: windowHeight, Theme: shot.DefaultTheme()})
	if err != nil {
		return err
	}
	defer h.Close()
	return h.Screenshot(path)
}

func main() {
	pngPath := flag.String("png", "", "write one frame to this file and exit")
	lines := flag.Bool("lines", false, "print the line breaks of stdin and exit")
	length := flag.Float64("height", 30*fontSize, "line length for -lines")
	size := flag.Float64("size", fontSize, "font size for -lines")
	family := flag.String("family", "", "font family for -lines")
	vertical := flag.Bool("vertical", false, "set vertically, for -lines")
	justify := flag.Bool("justify", false, "justify, for -lines")
	indent := flag.Float64("indent", 0, "first line indent in ems, for -lines")
	full := flag.Bool("full-punctuation", false, "keep punctuation on full em boxes, for -lines")
	oikomi := flag.Bool("oikomi", false, "pull prohibited characters into the line, for -lines")
	flag.Parse()

	var err error
	switch {
	case *lines:
		st := text.Style{Size: float32(*size), Family: *family, Indent: float32(*indent * *size)}
		if *vertical {
			st.Orientation = text.Vertical
		}
		if *justify {
			st.Align = text.AlignJustify
		}
		if *full {
			st.Punctuation = jlreq.PunctuationFull
		}
		if *oikomi {
			st.Kinsoku = jlreq.Oikomi
		}
		err = printLines(os.Stdin, float32(*length), st)
	case *pngPath != "":
		err = screenshot(*pngPath)
	default:
		err = gowaku.Run(gowaku.App{Title: "gowaku: tategaki", Width: windowWidth, Height: windowHeight, Home: page{}})
	}
	if err != nil {
		log.Fatal(err)
	}
}
