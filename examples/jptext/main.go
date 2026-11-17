// Command jptext shows the Japanese typesetting of gowaku. Each row sets one
// sentence twice, with a rule of composition on and off, inside a box as wide as
// the measure. Ruby and a field for trying the IME follow.
//
//	jptext                    opens the window
//	jptext -png out.png       draws one frame to a file and exits
//	jptext -lines -width 420  prints where stdin breaks into lines, for
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
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/jlreq"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

const (
	windowWidth  = 1040
	windowHeight = 860
	fontSize     = 16
	// measure is the width of a sample: a whole number of ems, as a Japanese
	// text frame has.
	measure = 28 * fontSize
)

const (
	prohibited = "長音が行頭に来る例を見るために、字数を合わせた上でサーバーと書く。strict は語の頭から次の行に送り、loose は長音を行頭に置く。"
	brackets   = "彼は「『今日は休みだ。』と言った。」と書いた（昨日の話だ）。「続き」は明日、「終わり」は来週です。"
	mixed      = "gowakuはGoで書くUIライブラリで、Windows 11とUbuntu 24.04で動く。120fpsで描き、CPU使用率は3%以下を目標にする。"
	hanging    = "行末にちょうど読点が来るように字数を合わせた文章なら読点、これをぶら下げて版面の外に出すと、次の行へ送らずに済む。"
	pulled     = "追い込みでは、行の約物の空きを使って句点を同じ行に収める。追い出しの方は、前の文字ごと次の行へ送る。"
)

type sample struct {
	title  string
	left   string
	leftS  text.Style
	right  string
	rightS text.Style
	body   string
}

func samples() []sample {
	base := text.Style{Size: fontSize}
	with := func(f func(*text.Style)) text.Style {
		s := base
		f(&s)
		return s
	}
	return []sample{
		{
			"行頭禁則", "strict（既定）", base, "loose",
			with(func(s *text.Style) { s.LineBreak = jlreq.Loose }), prohibited,
		},
		{
			"約物の詰め", "詰める（既定）", base, "全角のまま",
			with(func(s *text.Style) { s.Punctuation = jlreq.PunctuationFull }), brackets,
		},
		{
			"和欧間アキ", "四分アキ（既定）", base, "アキなし",
			with(func(s *text.Style) { s.WesternSpacing = jlreq.WesternNone }), mixed,
		},
		{
			"ぶら下げ", "なし（既定）", base, "あり",
			with(func(s *text.Style) { s.Hanging = true }), hanging,
		},
		{
			"禁則の解決", "追い出し（既定）", base, "追い込み",
			with(func(s *text.Style) { s.Kinsoku = jlreq.Oikomi }), pulled,
		},
	}
}

func label(th *theme.Theme, s string) gowaku.Widget {
	return basic.Text{Content: s, Style: text.Style{Size: 12, Color: th.Color.Ink2}}
}

func heading(th *theme.Theme, s string) gowaku.Widget {
	return basic.Text{Content: s, Style: text.Style{Size: 14, Weight: 700, Color: th.Color.Ink}}
}

func cell(th *theme.Theme, caption, body string, st text.Style) gowaku.Widget {
	return basic.Column{Spacing: 4, Cross: layout.CrossStart, Children: []gowaku.Widget{
		label(th, caption),
		basic.Container{
			Width:  measure,
			Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
			Child:  basic.Text{Content: body, Style: st},
		},
	}}
}

// ruby sets a sentence with ruby on single words and on a long compound.
func ruby() []text.Span {
	return []text.Span{
		{Text: "組版", Ruby: "くみはん"},
		{Text: "の"},
		{Text: "規則", Ruby: "きそく"},
		{Text: "は、"},
		{Text: "日本語組版処理", Ruby: "にほんごくみはんしょり"},
		{Text: "の"},
		{Text: "要件", Ruby: "ようけん"},
		{Text: "に"},
		{Text: "従", Ruby: "したが"},
		{Text: "う。ルビが"},
		{Text: "親文字", Ruby: "おやもじ"},
		{Text: "より"},
		{Text: "長", Ruby: "なが"},
		{Text: "ければ、"},
		{Text: "隣", Ruby: "となり"},
		{Text: "の"},
		{Text: "仮名", Ruby: "かな"},
		{Text: "に"},
		{Text: "掛", Ruby: "か"},
		{Text: "かる。"},
	}
}

type page struct{ gowaku.StatefulBase }

type pageState struct {
	gowaku.StateBase
	draft string
}

// CreateState implements core.StatefulWidget.
func (page) CreateState() gowaku.State { return &pageState{} }

// Build implements core.State.
func (s *pageState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	list := samples()
	rows := make([]gowaku.Widget, 0, len(list)+3)
	rows = append(rows, basic.Text{Content: "gowaku 日本語組版の見本", Style: th.Text.LargeTitle})
	for i := range list {
		sm := &list[i]
		rows = append(rows, basic.Column{Spacing: 4, Cross: layout.CrossStart, Children: []gowaku.Widget{
			heading(th, sm.title),
			basic.Row{Spacing: 48, Cross: layout.CrossStart, Children: []gowaku.Widget{
				cell(th, sm.left, sm.body, sm.leftS),
				cell(th, sm.right, sm.body, sm.rightS),
			}},
		}})
	}
	rows = append(rows,
		basic.Column{Spacing: 4, Cross: layout.CrossStart, Children: []gowaku.Widget{
			heading(th, "ルビ"),
			basic.SizedBox{Width: measure, Child: basic.RichText{Spans: ruby(), Style: text.Style{Size: fontSize}}},
		}},
		basic.Column{Spacing: 4, Cross: layout.CrossStart, Children: []gowaku.Widget{
			heading(th, "入力"),
			basic.SizedBox{Width: measure, Child: field.Input{
				Value:       s.draft,
				Placeholder: "ここで変換を試す（注目文節が強調される）",
				Multiline:   true,
				OnChange:    func(v string) { s.SetState(func() { s.draft = v }) },
			}},
		}},
	)
	return basic.Scroll{Child: basic.Pad(32, basic.Column{Spacing: 18, Cross: layout.CrossStart, Children: rows})}
}

// corpus sets every line of a file at the measure, for a page to hold next to
// the same text in another program.
type corpus struct {
	gowaku.StatelessBase
	lines []string
	width float32
	size  float32
}

// Build implements core.StatelessWidget.
func (c corpus) Build(gowaku.BuildContext) gowaku.Widget {
	rows := make([]gowaku.Widget, 0, len(c.lines))
	for _, l := range c.lines {
		rows = append(rows, basic.SizedBox{Width: c.width, Child: basic.Text{
			Content: l,
			Style:   text.Style{Size: c.size, Punctuation: jlreq.PunctuationFull},
		}})
	}
	return basic.Pad(24, basic.Column{Cross: layout.CrossStart, Children: rows})
}

// printLines breaks each line of r at width and prints the result, one output
// line per line of text.
func printLines(r io.Reader, width, size float32, st text.Style) error {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		return err
	}
	st.Size = size
	in := bufio.NewScanner(r)
	in.Buffer(nil, 1<<20)
	var out strings.Builder
	for in.Scan() {
		content := strings.TrimLeft(in.Text(), string(rune(0xfeff)))
		l := shaper.Shape(content, st).Layout(width)
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

// screenshot draws one frame of root to path.
func screenshot(path string, root gowaku.Widget) error {
	h, err := shot.Open(root, shot.Options{Width: windowWidth, Height: windowHeight, Theme: shot.DefaultTheme()})
	if err != nil {
		return err
	}
	defer h.Close()
	return h.Screenshot(path)
}

func main() {
	pngPath := flag.String("png", "", "write one frame to this file and exit")
	lines := flag.Bool("lines", false, "print the line breaks of stdin and exit")
	width := flag.Float64("width", measure, "measure for -lines")
	size := flag.Float64("size", fontSize, "font size for -lines")
	hang := flag.Bool("hanging", false, "hanging punctuation for -lines")
	strictness := flag.Int("strictness", 0, "0 strict, 1 normal, 2 loose, for -lines")
	corpusPath := flag.String("corpus", "", "with -png: set the lines of this file at -width and -size")
	full := flag.Bool("full-punctuation", false, "keep punctuation on full em boxes, for -lines")
	flag.Parse()

	var err error
	switch {
	case *lines:
		st := text.Style{Hanging: *hang, LineBreak: jlreq.Strictness(*strictness)}
		if *full {
			st.Punctuation = jlreq.PunctuationFull
		}
		err = printLines(os.Stdin, float32(*width), float32(*size), st)
	case *pngPath != "" && *corpusPath != "":
		var data []byte
		if data, err = os.ReadFile(*corpusPath); err == nil {
			content := strings.TrimSpace(strings.ReplaceAll(string(data), "\r", ""))
			err = screenshot(*pngPath, corpus{lines: strings.Split(content, "\n"), width: float32(*width), size: float32(*size)})
		}
	case *pngPath != "":
		err = screenshot(*pngPath, page{})
	default:
		err = gowaku.Run(gowaku.App{Title: "gowaku: jptext", Width: windowWidth, Height: windowHeight, Home: page{}})
	}
	if err != nil {
		log.Fatal(err)
	}
}
