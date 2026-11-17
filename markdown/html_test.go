package markdown

import (
	"strings"
	"testing"
)

func TestParse_HTMLBlocks(t *testing.T) {
	runParseCases(t, []parseCase{
		{"README のワードマーク",
			"<p align=\"center\">\n  <picture>\n    <source media=\"(prefers-color-scheme: dark)\" srcset=\"d.svg\">\n" +
				"    <img src=\"l.svg\" alt=\"logo\" width=\"280\">\n  </picture>\n</p>",
			"p[c](![logo](l.svg 280x dark=d.svg))"},
		{"中央揃えの太字", "<p align=\"center\">\n  <b>Bold words</b>\n</p>", "p[c](**Bold words**)"},
		{"行ごとのバッジ",
			"<p align=\"center\">\n  <a href=\"https://a\"><img src=\"a.svg\" alt=\"A\"></a>\n  <a href=\"b\"><img src=\"b.svg\" alt=\"B\"></a>\n</p>",
			"p[c]([![A](a.svg)](https://a)⏎[![B](b.svg)](b))"},
		{"揃えのある見出し", "<h1 align=\"center\">Title</h1>\n<h3>Sub</h3>", "h1[c](Title) h3(Sub)"},
		{"details の中の Markdown", "<details>\n<summary>More</summary>\n\n- a\n- b\n\n</details>\n\nafter",
			"details(More){ul{li{p(a)} li{p(b)}}} p(after)"},
		{"開いた details", "<details open><summary>S</summary>text</details>", "details[open](S){p(text)}"},
		{"summary の無い details", "<details>\nx\n</details>", "details(Details){p(x)}"},
		{"入れ子の details", "<details><summary>a</summary>\n\n<details><summary>b</summary>\n\ninner\n\n</details>\n\n</details>",
			"details(a){details(b){p(inner)}}"},
		{"div の揃えは中の Markdown に効く", "<div align=\"right\">\n\n**md**\n\n# h\n\n</div>", "p[r](**md**) h1[r](h)"},
		{"Markdown のブロックは p を閉じる", "<p align=\"center\">\n\ntext\n\n</p>", "p(text)"},
		{"型 7 は段落を割り込まない", "a\n<img src=\"x.png\">", "p(a⏎![](x.png))"},
		{"型 7 だけの行", "<img src=\"x.png\">", "p(![](x.png))"},
		{"型 6 は段落を割り込む", "a\n<div>b</div>", "p(a) p(b)"},
		{"空行で終わる", "<div>\nx\n\n*y*", "p(x) p(*y*)"},
		{"コメントは空行をまたぐ", "<!--\n\nhidden\n\n-->\nafter", "p(after)"},
		{"script は文字", "<script>alert(1)</script>", "p(<script>alert(1)</script>)"},
		{"script の中のタグも文字", "<script>\nx = \"<b>no</b>\"\n</script>", "p(<script>⏎x = \"<b>no</b>\"⏎</script>)"},
		{"知らない属性と style と on は無視", "<p align=center style=\"color:red\" onclick=\"x()\">y</p>", "p[c](y)"},
		{"javascript: のリンクは文字だけ", "<p><a href=\"javascript:alert(1)\">x</a> <a href=\" JaVa\tScript:x\">y</a></p>", "p(x y)"},
		{"br と hr", "<p>a<br>b<br/></p>\n<hr>", "p(a<br>b) hr"},
		{"空白はまとまる", "<p>\n  a   b\n  <b> c </b>\n</p>", "p(a b⏎**c**)"},
		{"表", "<table>\n<tr><th>A</th><th>B</th></tr>\n<tr><td align=\"center\">1</td><td>2</td></tr>\n</table>",
			"htable[h]({p(A)} | {p(B)} / {p[c](1)} | {p(2)})"},
		{"見出しの無い表と欠けたセル", "<table><tbody><tr><td>1</td><td>2</td></tr><tr><td>3</td></tr></tbody></table>",
			"htable({p(1)} | {p(2)} / {p(3)} | {})"},
		{"セルの中の Markdown", "<table><tr><td>\n\n**x**\n\n</td></tr></table>", "htable({p(**x**)})"},
		{"pre", "<pre>\na &lt; b\n  c\n</pre>", "code(a < b↵  c↵)"},
		{"ul は文字", "<ul><li>a</li></ul>", "p(<ul><li>a</li></ul>)"},
		{"文字参照", "<p>&amp; &copy; &#x41;</p>", "p(& © A)"},
		{"リストの中の HTML ブロック", "- <div>x</div>\n- y", "ul{li{p(x)} li{p(y)}}"},
		{"source の media と srcset", "<picture><source media=\"(min-width: 600px)\" srcset=\"w.png\">" +
			"<source srcset=\"w2.webp 2x, n.webp 1x\"><img src=\"a.png\"></picture>",
			"p(![](a.png any=n.webp))"},
		{"読めない type の source", "<picture><source type=\"image/avif\" srcset=\"a.avif\"><img src=\"a.png\"></picture>", "p(![](a.png))"},
		{"img の無い picture", "<p><picture><source srcset=\"a.png\"></picture>x</p>", "p(x)"},
		{"高さだけの img", "<img src=\"a.png\" height=\"20px\" alt=\"h\">", "p(![h](a.png x20))"},
		{"閉じない div", "<div>\na", "p(a)"},
		{"閉じタグだけ", "</div>\n\ntext", "p(text)"},
		{"宣言と処理命令", "<!DOCTYPE html>\n\n<?php echo 1; ?>\n\nx", "p(x)"},
	})
}

func TestParse_HTMLInline(t *testing.T) {
	runParseCases(t, []parseCase{
		{"キーと上付き下付き", "Press <kbd>Ctrl</kbd>+<kbd>C</kbd>, H<sub>2</sub>O, x<sup>2</sup>",
			"p(Press <kbd>Ctrl</kbd>+<kbd>C</kbd>, H<sub>2</sub>O, x<sup>2</sup>)"},
		{"書式", "<code>c</code> <del>d</del> <s>s</s> <i>i</i> <em>e</em> <strong>s</strong>",
			"p(`c` ~~d~~ ~~s~~ *i* *e* **s**)"},
		{"閉じない b は段落の終わりまで", "<b>bold *em* rest", "p(**bold *em* rest**)"},
		{"開きの無い閉じタグは消える", "a </b> b", "p(a  b)"},
		{"a", "see <a href=\"https://x\">here</a>.", "p(see [here](https://x).)"},
		{"リンクの中の a", "[<a href=\"y\">t</a>](x)", "p([t](x))"},
		{"a の中のリンク", "<a href=\"y\">[t](x)</a>", "p([t](y))"},
		{"コメント", "a <!-- c --> b", "p(a  b)"},
		{"閉じないコメントは文字", "a <!-- c", "p(a <!-- c)"},
		{"幅のある img", "x <img src=\"a.png\" width=\"50%\">", "p(x ![](a.png 50%x))"},
		{"ブロックの要素は文字", "a <div> b", "p(a <div> b)"},
		{"br", "a<br>b<br />c", "p(a<br>b<br>c)"},
		{"強調をまたぐタグ", "*a <b>b* c</b>", "p(*a **b*** c)"},
		{"Markdown の javascript: リンク", "[x](javascript:alert(1)) [y](<java\tscript:z>) [ok](https://o)", "p(x y [ok](https://o))"},
		{"表のセルの中のタグ", "| a |\n|---|\n| <b>x</b> |", "table[-](a / **x**)"},
		{"見出しの中のタグ", "# a <sup>1</sup>", "h1(a <sup>1</sup>)"},
	})
}

func TestHeadings_HTML(t *testing.T) {
	src := "<h1 align=\"center\"><img src=\"logo.svg\" alt=\"gowaku\"></h1>\n\n<h2>HTML の見出し</h2>\n\n" +
		"<details><summary>s</summary>\n\n## 中の見出し\n\n</details>\n\n<table><tr><td>\n\n### 表の見出し\n\n</td></tr></table>"
	var got []string
	for _, h := range Headings(src) {
		got = append(got, h.Slug)
	}
	want := []string{"gowaku", "html-の見出し", "中の見出し", "表の見出し"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("slugs %q, want %q", got, want)
	}
}

func TestScanTag(t *testing.T) {
	cases := []struct {
		src        string
		strict     bool
		name       string
		n          int
		attrs      string
		closing    bool
		selfClosed bool
	}{
		{`<a href="x">`, true, "a", 12, "href=x", false, false},
		{`<IMG SRC='a.png' Alt=b>`, true, "img", 23, "src=a.png alt=b", false, false},
		{`<br/>`, true, "br", 5, "", false, true},
		{`<input disabled >`, true, "input", 17, "disabled=", false, false},
		{`</P >`, true, "p", 5, "", true, false},
		{`<a href="x&amp;y">`, true, "a", 18, "href=x&y", false, false},
		{"<a\nhref=x\n>", true, "a", 11, "href=x", false, false},
		{`<a href=x"y>`, true, "", 0, "", false, false},
		{`<a href=x"y>`, false, "a", 12, `href=x"y`, false, false},
		{`<a href="x>`, true, "", 0, "", false, false},
		{`<a b<c>`, true, "", 0, "", false, false},
		{`<1a>`, true, "", 0, "", false, false},
		{`< a>`, true, "", 0, "", false, false},
		{`<a`, true, "", 0, "", false, false},
		{`</a b>`, true, "", 0, "", false, false},
		{`<a@b>`, false, "", 0, "", false, false},
	}
	for _, c := range cases {
		tag, n := scanTag(c.src, c.strict)
		var attrs []string
		for _, a := range tag.attrs {
			attrs = append(attrs, a.name+"="+a.value)
		}
		if n != c.n || tag.name != c.name || strings.Join(attrs, " ") != c.attrs || tag.closing != c.closing || tag.selfClosing != c.selfClosed {
			t.Errorf("scanTag(%q, %v) = %q %d %q closing %v self %v", c.src, c.strict, tag.name, n, attrs, tag.closing, tag.selfClosing)
		}
	}
}

func TestHTMLBlockType(t *testing.T) {
	cases := map[string]int{
		"<pre>":                1,
		"<SCRIPT type=x>":      1,
		"<style":               1,
		"<textarea>x":          1,
		"<prex>":               7,
		"<!-- x":               2,
		"<?php":                3,
		"<!DOCTYPE html>":      4,
		"<![CDATA[":            5,
		"<div>":                6,
		"</DIV>":               6,
		"<p align=\"center\">": 6,
		"<details open>":       6,
		"<h1>":                 6,
		"<hr/>":                6,
		"<divx>":               7,
		"<img src=\"x\">":      7,
		"<a href=\"x\">  ":     7,
		"</b>":                 7,
		"<a href=\"x\">t":      0,
		"<b>bold</b>":          0,
		"<":                    0,
		"< div>":               0,
		"<pre/>":               0,
	}
	for src, want := range cases {
		if got := htmlBlockType(src); got != want {
			t.Errorf("htmlBlockType(%q) = %d, want %d", src, got, want)
		}
	}
}

func TestSafeURL(t *testing.T) {
	for _, u := range []string{"javascript:x", " JavaScript:x", "java\tscript:x", "java\nscript:x", "VBScript:x", "data:text/html,x", "\x01javascript:x"} {
		if safeURL(u) {
			t.Errorf("safeURL(%q) = true", u)
		}
	}
	for _, u := range []string{"https://x", "README.md", "#anchor", "mailto:a@b.c", "", "javascript", "/javascript:x"} {
		if !safeURL(u) {
			t.Errorf("safeURL(%q) = false", u)
		}
	}
}

func TestParseLength(t *testing.T) {
	cases := []struct {
		in      string
		v       float32
		percent bool
	}{
		{"280", 280, false}, {" 280px ", 280, false}, {"12.5", 12.5, false}, {"50%", 50, true}, {"150%", 100, true},
		{"", 0, false}, {"auto", 0, false}, {"-3", 0, false}, {"0", 0, false}, {"1e9", 0, false}, {"99999999", maxLength, false},
	}
	for _, c := range cases {
		if v, p := parseLength(c.in); v != c.v || p != c.percent {
			t.Errorf("parseLength(%q) = %v, %v; want %v, %v", c.in, v, p, c.v, c.percent)
		}
	}
}
