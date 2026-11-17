// Command example shows a long Markdown document, in Japanese and English,
// that uses every construct the markdown package supports, with an outline of
// its headings at the left. The text can be selected and copied, and the
// links to "#" headings and the outline scroll to them. Run it from the
// markdown directory:
//
//	go run ./example
package main

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/markdown"
)

// assets holds the pictures of the document: Markdown reads logo.png from it.
//
//go:embed logo.png
var assets embed.FS

const document = `# gowaku の Markdown

これは **gowaku** の ` + "`markdown`" + ` モジュールの見本です。本文はライブラリ自身の行分割で折り返し、
日本語の行の途中で改行しても
余計な空白は入りません。English text, on the other hand,
gets a space where the source breaks the line.

目次: [文字の書式](#文字の書式)、[リンク](#リンク)、[リスト](#リスト)、[コード](#コード)、[表](#表)。
見出しへのリンクは文書の中で移ります。文字はドラッグで選んで Ctrl+C でコピーできます。

## 文字の書式

*強調*、**強い強調**、***両方***、~~取り消し線~~、` + "`インラインのコード`" + `。
_Emphasis_, __strong__, ~~strikethrough~~ and ` + "`inline code`" + ` work in English too.
かぎ括弧を含む強調も**「このように」**書けます。

行末に空白を二つ置くと` + "  \n" + `強制改行になります。バックスラッシュでも\
同じです。

## リンク

[gowaku のリポジトリ](https://github.com/bankai-tensei/gowaku "GitHub") へのリンク、
自動リンク <https://go.dev>、裸の URL www.example.com や https://example.org/path も押せます。
A [reference link][commonmark] is defined at the bottom of the document.
長いリンクは [行をまたいでも、どの行の部分を押しても同じ宛先に飛ぶ長い長いリンクの文字列](https://example.com/long) です。

[commonmark]: https://commonmark.org "CommonMark"

## 引用

> 吾輩は猫である。名前はまだ無い。
> どこで生れたかとんと見当がつかぬ。
>
> > 引用は入れ子にできます。Quotes can nest.

## リスト

1. 最初の項目
2. 二番目の項目
   - 入れ子の箇条書き
     - さらに深い項目
   - Another nested item
3. 三番目の項目

- [x] 設計を書く (write the ADR)
- [x] パーサを書く
- [ ] スクリーンショットを撮る

10. 番号は 10 から
11. 始めることもできます

## コード

` + "```go" + `
package main

import "fmt"

func main() {
	fmt.Println("こんにちは、世界。This line is long enough that it scrolls sideways instead of wrapping.")
}
` + "```" + `

    字下げしたコードブロック
    indented code block

## 表

| 項目 | 説明 | 数 |
|:-----|:----:|---:|
| りんご | 赤い果物 | 3 |
| banana | a long yellow fruit that makes the column wrap when the window is narrow | 12 |
| みかん | ` + "`code`" + ` と *強調* | 100 |

---

## 画像

![gowaku のロゴ](logo.png)

画像は ` + "`Images`" + ` に渡したフォルダから読みます。ネットワークには取りに行かないので、
次の画像は代替テキストになります。

![遠くにある画像](https://example.com/far.png)

## 生の HTML

<div class="note">HTML は解釈されず、そのまま文字として表示されます。</div>

###### 以上 — The end.
`

type app struct {
	gowaku.StatefulBase
}

type appState struct {
	gowaku.StateBase
	last string
	// doc moves the document to a heading of the outline.
	doc markdown.Controller
}

// headings is the outline of the document.
var headings = markdown.Headings(document)

func (app) CreateState() gowaku.State { return &appState{} }

func (s *appState) Build(gowaku.BuildContext) gowaku.Widget {
	status := "リンクを押すと、ここに宛先が出ます。"
	if s.last != "" {
		status = "押したリンク: " + s.last
	}
	outline := make([]gowaku.Widget, 0, len(headings))
	for _, h := range headings {
		if h.Level > 2 {
			continue
		}
		slug := h.Slug
		outline = append(outline, basic.Padding{
			Insets: geom.Insets{Left: float32(h.Level-1) * 12},
			Child: control.Button{
				Label: h.Text, Style: control.ButtonQuiet, Size: control.ButtonSmall,
				OnPress: func() { s.doc.ShowHeading(slug) },
			},
		})
	}
	return basic.Column{Children: []gowaku.Widget{
		basic.Expanded{Child: basic.Row{Cross: layout.CrossStretch, Children: []gowaku.Widget{
			basic.SizedBox{Width: 200, Child: basic.Scroll{Child: basic.Padding{
				Insets: geom.Symmetric(8, 16),
				Child:  basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 2, Children: outline},
			}}},
			basic.Expanded{Child: field.SelectionArea{Child: basic.Scroll{Child: basic.Padding{
				Insets: geom.Symmetric(32, 24),
				Child: markdown.Markdown{
					Source:     document,
					OnLinkTap:  func(url string) { s.SetState(func() { s.last = url }) },
					Images:     assets,
					Highlight:  highlightGo,
					Controller: &s.doc,
				},
			}}}},
		}}},
		basic.Padding{Insets: geom.Symmetric(16, 8), Child: basic.Text{Content: status}},
	}}
}

// highlightGo colors the keywords of Go, to show the Highlight hook.
func highlightGo(lang, code string) []text.Span {
	if lang != "go" {
		return nil
	}
	keyword := text.Style{Color: geom.Color{R: 0.55, G: 0.2, B: 0.75, A: 1}, Weight: 700}
	str := text.Style{Color: geom.Color{R: 0.1, G: 0.5, B: 0.25, A: 1}}
	var spans []text.Span
	for code != "" {
		switch {
		case code[0] == '"':
			end := strings.IndexByte(code[1:], '"')
			if end < 0 {
				end = len(code) - 2
			}
			spans = append(spans, text.Span{Text: code[:end+2], Style: str})
			code = code[end+2:]
		case isWordStart(code):
			n := wordLength(code)
			w := code[:n]
			st := text.Style{}
			switch w {
			case "package", "import", "func", "return", "var", "const", "type", "if", "for", "range":
				st = keyword
			}
			spans = append(spans, text.Span{Text: w, Style: st})
			code = code[n:]
		default:
			_, n := utf8.DecodeRuneInString(code)
			spans = append(spans, text.Span{Text: code[:n]})
			code = code[n:]
		}
	}
	return spans
}

func isWordStart(s string) bool {
	c := s[0]
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func wordLength(s string) int {
	n := 0
	for n < len(s) {
		c := s[n]
		if c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			break
		}
		n++
	}
	return n
}

func main() {
	err := gowaku.Run(gowaku.App{Title: "gowaku: markdown", Width: 900, Height: 760, Home: app{}})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
