// Command scripts shows what gowaku sets beyond Japanese: color emoji,
// right-to-left text mixed with left-to-right, variable fonts and English
// hyphenation.
//
//	scripts                  opens the window
//	scripts -png out.png     draws one frame to a file and exits
package main

import (
	"flag"
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

const (
	windowWidth  = 1180
	windowHeight = 640
)

const english = "Typesetting with a narrow measure leaves wide gaps between words unless long words such as international, hyphenation and responsibility may break across lines."

type page struct{ gowaku.StatelessBase }

func section(th *theme.Theme, title string, children ...gowaku.Widget) gowaku.Widget {
	body := basic.Column{Spacing: 8, Cross: layout.CrossStart, Children: children}
	return basic.Column{Spacing: 6, Cross: layout.CrossStart, Children: []gowaku.Widget{
		basic.Text{Content: title, Style: text.Style{Size: 12, Color: th.Color.Ink2}},
		basic.Container{Border: paint.Stroke{Width: 1, Color: th.Color.Hair}, Padding: geom.All(12), Child: body},
	}}
}

// Build implements core.StatelessWidget.
func (page) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	left := basic.Column{Spacing: 20, Cross: layout.CrossStart, Children: []gowaku.Widget{
		section(th, "カラー絵文字（COLRv1 / CBDT）",
			basic.SizedBox{Width: 520, Child: basic.Text{Content: "今日は🍣と🍜を食べた😋 🎉✨ ❤️ 👍🏽 👨‍👩‍👧 1️⃣ 🇯🇵", Style: text.Style{Size: 28}}},
			basic.Text{Content: "小さくても色が出る: 😀😃😄😁😆😅🤣😂🙂🙃 ☀ ☀️（VS15 / VS16）", Style: text.Style{Size: 14}},
		),
		section(th, "双方向テキスト（UAX #9）",
			basic.SizedBox{Width: 520, Child: basic.Column{Spacing: 8, Cross: layout.CrossStretch, Children: []gowaku.Widget{
				basic.Text{Content: "The word مرحبا means hello, and so does שלום. Numbers stay in order: 2026.", Style: text.Style{Size: 16}},
				basic.Text{Content: "هذا نص عربي يحتوي على English وأرقام 123 و456 في نفس السطر، ويلتف إلى السطر التالي.", Style: text.Style{Size: 18}},
				basic.Text{Content: "שלום עולם! gowaku 2.0 תומך בעברית (כולל סוגריים).", Style: text.Style{Size: 16}},
				field.Input{Value: "اكتب هنا: abc 123", Style: text.Style{Size: 16}},
			}}},
		),
	}}
	right := basic.Column{Spacing: 20, Cross: layout.CrossStart, Children: []gowaku.Widget{
		section(th, "可変フォント（Bahnschrift の wght と wdth）", variable()...),
		section(th, "欧文ハイフネーション（両端揃え、右は Hyphenate）", basic.Row{Spacing: 24, Cross: layout.CrossStart, Children: []gowaku.Widget{
			basic.SizedBox{Width: 220, Child: basic.Text{Content: english, Style: text.Style{Size: 15, Align: text.AlignJustify}}},
			basic.SizedBox{Width: 220, Child: basic.Text{Content: english, Style: text.Style{Size: 15, Align: text.AlignJustify, Hyphenate: true}}},
		}}),
	}}
	return basic.Pad(28, basic.Row{Spacing: 28, Cross: layout.CrossStart, Children: []gowaku.Widget{left, right}})
}

// variable sets one line at several points of the axes of a variable font.
func variable() []gowaku.Widget {
	weights := []uint16{300, 400, 500, 600, 700}
	rows := make([]gowaku.Widget, 0, len(weights))
	for _, w := range weights {
		rows = append(rows, basic.Row{Spacing: 24, Children: []gowaku.Widget{
			basic.Text{Content: "Variable 可変", Style: text.Style{Family: "Bahnschrift", Size: 22, Weight: w}},
			basic.Text{Content: "Condensed 75", Style: text.Style{Family: "Bahnschrift", Size: 22, Weight: w,
				Variations: [4]text.Variation{{Axis: "wdth", Value: 75}}}},
		}})
	}
	return rows
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
	flag.Parse()
	var err error
	if *pngPath != "" {
		err = screenshot(*pngPath)
	} else {
		err = gowaku.Run(gowaku.App{Title: "gowaku: scripts", Width: windowWidth, Height: windowHeight, Home: page{}})
	}
	if err != nil {
		log.Fatal(err)
	}
}
