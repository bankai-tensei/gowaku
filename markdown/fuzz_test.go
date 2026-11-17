package markdown

import (
	"strings"
	"testing"
	"time"
)

// FuzzParse checks that no input makes the parser panic or loop. The seeds
// cover every construct and the inputs known to be slow for naive parsers.
// Run longer with: go test -run ^$ -fuzz FuzzParse -fuzztime 30s
func FuzzParse(f *testing.F) {
	seeds := []string{
		"", "# h\n\npara *em* **strong** ~~del~~ `code`\n",
		"> quote\n> > nested\nlazy\n", "- a\n  - b\n    1. c\n- [x] task\n",
		"```go\nx := 1\n```\n", "    indented\n", "***\n", "a | b\n:-|-:\n1 | 2\n",
		"[link](http://a \"t\") ![img](i.png) <http://x> www.example.com\n",
		"[ref]\n\n[ref]: /url\n", "&amp; &#12354; \\* <b>html</b>\n",
		"日本語の**強調**と\n改行\n", "a\r\nb\rc\x00d\xff",
		strings.Repeat("[", 100), strings.Repeat("*a", 100), strings.Repeat("> ", 100),
		strings.Repeat("- ", 100), "`" + strings.Repeat("``", 50), strings.Repeat("<", 100),
		"[a](" + strings.Repeat("(", 40) + ")", "\t-\t-\t- \t>\t",
		// HTML (docs/ADR/0119-markdown-html-subset.md).
		"<p align=\"center\">\n  <picture>\n    <source media=\"(prefers-color-scheme: dark)\" srcset=\"d.svg\">\n" +
			"    <img src=\"l.svg\" alt=\"logo\" width=\"280\">\n  </picture>\n</p>\n",
		"<details open>\n<summary>s <b>b</b></summary>\n\n- md\n\n</details>\n",
		"<table><tr><th>a</th></tr><tr><td align=right>\n\n**x**\n\n</td></tr></table>\n",
		"<!-- c -->\n<script>x</script>\n<pre>\na &lt; b\n</pre>\n<?x?>\n<!X>\n<![CDATA[x]]>\n",
		"a <kbd>k</kbd> <sub>1</sub><sup>2</sup> <a href=\"javascript:x\">j</a> <img src=x width=50%><br>\n",
		"<div align=right>\n\n# h\n\n</div>\n", "<b><i>x</b></i> </p> <a href=y>[t](u)</a>",
		strings.Repeat("<details>", 50), strings.Repeat("<b>", 50), strings.Repeat("<a b=\"", 30),
		"<a\nhref='x'\n>t</a>", "<p\n", "</>", "<a b=c d='e' f=\"g\" h>",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		doc := parse(src)
		if d := maxDepth(doc); d > 2*maxNesting+2 {
			t.Errorf("tree %d deep", d)
		}
		_ = dump(doc) // walks every node; a broken tree would panic here
	})
}

// FuzzHTMLTags checks the tokenizer of HTML: no input makes it panic or
// loop, a tag it reads is the text it says it is, and tokens of text and tags
// never take more than the input. Run longer with:
// go test -run ^$ -fuzz FuzzHTMLTags -fuzztime 30s
func FuzzHTMLTags(f *testing.F) {
	for _, s := range []string{
		"<a href=\"x\">t</a>", "<IMG SRC=a.png Alt='b' />", "</p >", "<!-- c -->x", "<?pi?>", "<!DOCTYPE x>",
		"<![CDATA[x]]>", "<script>a<b>c</script>", "<a b=\"<c>\">", "<a b='", "<a\nb\n=\nc>", "&amp;&#x41;&bogus;",
		"<picture><source media=\"(prefers-color-scheme: light)\" srcset=\"a 1x, b 2x\"><img src=c></picture>",
		"<details><summary>x</summary>y</details>", "<table><tr><td>1<td>2<tr><th>3</table>", "<p><div></p></div>",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for i := 0; i < len(s); i++ {
			if s[i] != '<' {
				continue
			}
			for _, strict := range []bool{true, false} {
				tag, n := scanTag(s[i:], strict)
				if n < 0 || n > len(s)-i || (n > 0 && tag.raw != s[i:i+n]) {
					t.Fatalf("scanTag(%q, %v) = %d, raw %q", s[i:], strict, n, tag.raw)
				}
			}
			if n := markupEnd(s[i:], true); n < 0 || n > len(s)-i {
				t.Fatalf("markupEnd(%q) = %d", s[i:], n)
			}
		}
		total := 0
		for _, tok := range tokenizeHTML(s) {
			if tok.tag != nil {
				total += len(tok.tag.raw)
			}
		}
		if total > len(s) {
			t.Fatalf("tags of %d bytes out of %d", total, len(s))
		}
		doc := parse(s)
		if d := maxDepth(doc); d > 2*maxNesting+2 {
			t.Errorf("tree %d deep", d)
		}
		_ = dump(doc)
	})
}

// TestParse_Pathological feeds inputs that make naive Markdown parsers take
// quadratic time or blow the stack. Each must parse quickly.
func TestParse_Pathological(t *testing.T) {
	const n = 30000
	cases := map[string]string{
		"開き括弧":      strings.Repeat("[", n),
		"画像の開き括弧":   strings.Repeat("![", n),
		"閉じない強調":    strings.Repeat("*a ", n),
		"閉じない下線":    strings.Repeat("_a ", n),
		"交互の強調":     strings.Repeat("*_", n),
		"強調の入れ子":    strings.Repeat("*", n) + "a" + strings.Repeat("*", n),
		"閉じないコード":   strings.Repeat("`a", n),
		"長さの違うコード":  strings.Repeat("`` `", n/4),
		"山括弧":       strings.Repeat("<", n),
		"山括弧と文字":    strings.Repeat("<a", n),
		"リンクの括弧":    "[a](" + strings.Repeat("(", n),
		"リンクの閉じ括弧":  strings.Repeat("[a]", n),
		"参照の列":      strings.Repeat("[a][b]", n/2),
		"入れ子の引用":    strings.Repeat(">", n),
		"入れ子のリスト":   strings.Repeat("- ", n),
		"深い字下げのリスト": deepList(2000),
		"チルダ":       strings.Repeat("~", n),
		"表の列":       strings.Repeat("|a", n) + "\n" + strings.Repeat("|-", n),
		"表の行":       "|a|\n|-|\n" + strings.Repeat("|b|\n", n),
		"エンティティもどき": strings.Repeat("&#", n),
		"自動リンクもどき":  strings.Repeat("www.", n),
		"長い行":       strings.Repeat("あ", 10*n),
		"多くの空行":     strings.Repeat("\n", 10*n),
		"バックスラッシュ":  strings.Repeat("\\", n),
		"フェンスの山":    strings.Repeat("```\n", n),
		// HTML (docs/ADR/0119-markdown-html-subset.md).
		"div の山":           strings.Repeat("<div>", n),
		"details の行":       strings.Repeat("<details>\n\nx\n\n", n/4),
		"閉じない属性":           strings.Repeat("<a b=\"", n),
		"閉じない属性の段落":        "a " + strings.Repeat("<a b='", n),
		"閉じないコメント":         "a " + strings.Repeat("<!--", n),
		"閉じない宣言":           "a " + strings.Repeat("<!X", n),
		"閉じない処理命令":         "a " + strings.Repeat("<?", n),
		"閉じない CDATA":       "a " + strings.Repeat("<![CDATA[", n/4),
		"インラインのタグの山":       "a " + strings.Repeat("<b>", n),
		"閉じタグの山":           "a " + strings.Repeat("</b>", n),
		"食い違うタグ":           "a " + strings.Repeat("<b>x</i>", n/4),
		"script の山":        strings.Repeat("<script>", n/4),
		"閉じる script の山":    strings.Repeat("<script></script>", n/8),
		"表のセルの山":           "<table>" + strings.Repeat("<td>", n),
		"HTML ブロックの行":      strings.Repeat("<div>\n", n),
		"型 7 の行":           strings.Repeat("<x a=b>\n", n),
		"picture の source": "<picture>" + strings.Repeat("<source srcset=a>", n/4) + "<img src=b></picture>",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			doc := parse(src)
			if d := maxDepth(doc); d > 2*maxNesting+2 {
				t.Errorf("tree %d deep", d)
			}
			// The budget is generous so that the race detector and a slow
			// machine pass; a quadratic parser takes minutes here.
			if el := time.Since(start); el > 3*time.Second {
				t.Errorf("took %v", el)
			}
		})
	}
}

func deepList(n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(strings.Repeat(" ", 2*i))
		b.WriteString("- x\n")
	}
	return b.String()
}
