package markdown

import (
	"strings"
	"testing"
)

type parseCase struct {
	name, src, want string
}

func runParseCases(t *testing.T, cases []parseCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dump(parse(c.src)); got != c.want {
				t.Errorf("source %q\n got %s\nwant %s", c.src, got, c.want)
			}
		})
	}
}

func TestParse_Blocks(t *testing.T) {
	runParseCases(t, []parseCase{
		{"空", "", ""},
		{"空白だけ", "  \n\t\n", ""},
		{"段落", "foo\nbar\n\nbaz", "p(foo⏎bar) p(baz)"},
		{"段落の行頭の空白", "  aaa\n bbb", "p(aaa⏎bbb)"},
		{"ATX 見出し", "# 1\n## 2\n### 3\n#### 4\n##### 5\n###### 6", "h1(1) h2(2) h3(3) h4(4) h5(5) h6(6)"},
		{"見出しの # が 7 個", "####### foo", "p(####### foo)"},
		{"# の後に空白が無い", "#5 bolt\n\n#hashtag", "p(#5 bolt) p(#hashtag)"},
		{"閉じの #", "# foo ##\n## bar #####   ", "h1(foo) h2(bar)"},
		{"閉じに見えない #", "# foo#\n# #", "h1(foo#) h1()"},
		{"空の見出し", "#\n## ", "h1() h2()"},
		{"見出しは段落を割り込む", "foo\n# bar", "p(foo) h1(bar)"},
		{"4 字下げの見出しはコード", "    # foo", "code(# foo↵)"},
		{"setext 見出し", "Foo\n===\n\nBar\n---", "h1(Foo) h2(Bar)"},
		{"setext の複数行", "Foo\nbar\n===", "h1(Foo⏎bar)"},
		{"区切り線", "***\n---\n___\n * * *\n-  -  -", "hr hr hr hr hr"},
		{"区切り線にならない", "--\n**\n+++", "p(--⏎**⏎+++)"},
		{"区切り線は段落を割り込む", "foo\n***\nbar", "p(foo) hr p(bar)"},
		{"フェンス", "```\n<\n >\n```", "code(<↵ >↵)"},
		{"チルダのフェンス", "~~~\naaa\n~~~", "code(aaa↵)"},
		{"言語", "```go  extra\nx := 1\n```", "code[go  extra](x := 1↵)"},
		{"閉じないフェンス", "```\nabc\n\ndef", "code(abc↵↵def↵)"},
		{"短い閉じは閉じない", "````\naaa\n```\n``````", "code(aaa↵```↵)"},
		{"違う文字は閉じない", "```\naaa\n~~~\n```", "code(aaa↵~~~↵)"},
		{"フェンスの字下げ", "  ```\n  aaa\n    bbb\n aaa\n  ```", "code(aaa↵  bbb↵aaa↵)"},
		{"バッククォートの info に `", "``` a`b\nfoo\n```", "p(``` a`b⏎foo) code()"},
		{"空のフェンス", "```\n```", "code()"},
		{"字下げコード", "    a\n      b\n\n    c\n\n\nd", "code(a↵  b↵↵c↵) p(d)"},
		{"字下げコードは段落を割り込まない", "foo\n    bar", "p(foo⏎bar)"},
		{"タブの字下げ", "\tfoo\tbaz", "code(foo\tbaz↵)"},
		{"引用", "> # Foo\n> bar\n> baz", "quote{h1(Foo) p(bar⏎baz)}"},
		{"遅延継続", "> bar\nbaz\n> foo", "quote{p(bar⏎baz⏎foo)}"},
		{"引用の入れ子", "> > > foo\nbar", "quote{quote{quote{p(foo⏎bar)}}}"},
		{"空行で引用が切れる", "> a\n\n> b", "quote{p(a)} quote{p(b)}"},
		{"> の後の空白は省ける", ">foo", "quote{p(foo)}"},
		{"引用の中のコード", ">     code\n\n>    not code", "quote{code(code↵)} quote{p(not code)}"},
		{"解釈しない HTML は文字", "<span>\n*hi*\n</span>", "p(<span>⏎*hi*⏎</span>)"},
		{"HTML のコメントは出さない", "<!-- x -->", ""},
		{"CRLF", "# a\r\nb\r\nc\r\n\r\nd", "h1(a) p(b⏎c) p(d)"},
		{"CR だけ", "a\rb\r\rc", "p(a⏎b) p(c)"},
		{"NUL", "a\x00b", "p(a�b)"},
		{"壊れた UTF-8", "a\xffb", "p(a�b)"},
	})
}

func TestParse_Lists(t *testing.T) {
	runParseCases(t, []parseCase{
		{"箇条", "- a\n- b\n- c", "ul{li{p(a)} li{p(b)} li{p(c)}}"},
		{"番号", "1. a\n2. b", "ol{li{p(a)} li{p(b)}}"},
		{"開始番号", "3) a\n4) b", "ol3{li{p(a)} li{p(b)}}"},
		{"記号が変わると別のリスト", "- a\n+ b\n1. c\n1) d", "ul{li{p(a)}} ul{li{p(b)}} ol{li{p(c)}} ol{li{p(d)}}"},
		{"入れ子", "- a\n  - b\n    - c\n- d", "ul{li{p(a) ul{li{p(b) ul{li{p(c)}}}}} li{p(d)}}"},
		{"番号の入れ子", "1. a\n   1. b\n2. c", "ol{li{p(a) ol{li{p(b)}}} li{p(c)}}"},
		{"字下げが足りないと兄弟", "- a\n - b\n  - c", "ul{li{p(a)} li{p(b)} li{p(c)}}"},
		{"字下げの混在", "- a\n\t- b\n  \t  - c", "ul{li{p(a) ul{li{p(b) ul{li{p(c)}}}}}}"},
		{"タブと空白の混在で兄弟", "- a\n\t- b\n  \t- c", "ul{li{p(a) ul{li{p(b)} li{p(c)}}}}"},
		{"緩いリスト", "- a\n\n- b", "ul.loose{li{p(a)} li{p(b)}}"},
		{"項目の中の空行で緩い", "- a\n\n  b\n- c", "ul.loose{li{p(a) p(b)} li{p(c)}}"},
		{"入れ子の空行は外を緩めない", "- a\n  - b\n\n    c\n- d", "ul{li{p(a) ul.loose{li{p(b) p(c)}}} li{p(d)}}"},
		{"項目の継続行", "- a\n  b\nc", "ul{li{p(a⏎b⏎c)}}"},
		{"項目の中のコード", "- ```\n  x\n  ```", "ul{li{code(x↵)}}"},
		{"項目の中の字下げコード", "-     code\n\n      more", "ul{li{code(code↵↵more↵)}}"},
		{"空の項目", "-\n- b\n-", "ul{li{} li{p(b)} li{}}"},
		{"空の項目の後の空行で終わる", "-\n\n  foo", "ul{li{}} p(foo)"},
		{"空の項目は段落を割り込まない", "foo\n-\nbar", "h2(foo) p(bar)"},
		{"1 以外の番号は段落を割り込まない", "The number of\n14. b", "p(The number of⏎14. b)"},
		{"1 は段落を割り込む", "a\n1. b", "p(a) ol{li{p(b)}}"},
		{"10 桁の番号は番号でない", "1234567890. not ok", "p(1234567890. not ok)"},
		{"区切り線はリストを割る", "- a\n***\n- b", "ul{li{p(a)}} hr ul{li{p(b)}}"},
		{"- - - は区切り線", "- - -", "hr"},
		{"タスク", "- [ ] todo\n- [x] done\n- [X] Done", "ul{li[ ]{p(todo)} li[x]{p(done)} li[x]{p(Done)}}"},
		{"タスクでない", "- [y] no\n- [ ]no\n- [x]", "ul{li{p([y] no)} li{p([ ]no)} li[x]{p()}}"},
		{"引用の中のリスト", "> - a\n> - b", "quote{ul{li{p(a)} li{p(b)}}}"},
		{"リストの中の引用", "- > a\n  > b", "ul{li{quote{p(a⏎b)}}}"},
		{"リストの中の見出し", "- # Foo\n- Bar\n  ---", "ul{li{h1(Foo)} li{h2(Bar)}}"},
	})
}

func TestParse_Tables(t *testing.T) {
	runParseCases(t, []parseCase{
		{"表", "| a | b |\n| --- | --- |\n| 1 | 2 |", "table[-,-](a | b / 1 | 2)"},
		{"揃え", "a | b | c | d\n:-|:-:|-:|-\n1|2|3|4", "table[l,c,r,-](a | b | c | d / 1 | 2 | 3 | 4)"},
		{"欠けたセルは空", "| a | b | c |\n|---|---|---|\n| 1 |\n| 1 | 2 | 3 | 4 |", "table[-,-,-](a | b | c / 1 |  |  / 1 | 2 | 3)"},
		{"列の数が合わないと表でない", "| a | b |\n| --- |\n| 1 |", "p(| a | b |⏎| --- |⏎| 1 |)"},
		{"エスケープしたパイプ", "| a | b |\n|---|---|\n| `x\\|y` | c \\| d |", "table[-,-](a | b / `x|y` | c | d)"},
		{"セルの中の書式", "| *a* | [b](c) |\n|-|-|", "table[-,-](*a* | [b](c))"},
		{"空行で終わる", "|a|\n|-|\n|1|\n\nx", "table[-](a / 1) p(x)"},
		{"ブロックで終わる", "|a|\n|-|\n|1|\n# h", "table[-](a / 1) h1(h)"},
		{"前の段落は残る", "intro\n| a |\n| - |\n| 1 |", "p(intro) table[-](a / 1)"},
		{"パイプの無い setext は見出し", "Foo\n---", "h2(Foo)"},
		{"区切り行だけでは表でない", "|---|---|", "p(|---|---|)"},
		{"リストの中の表", "- | a |\n  | - |\n  | 1 |", "ul{li{table[-](a / 1)}}"},
		{"日本語の表", "| 名前 | 値 |\n|:--|--:|\n| 東京 | 1 |", "table[l,r](名前 | 値 / 東京 | 1)"},
	})
}

func TestParse_Inlines(t *testing.T) {
	runParseCases(t, []parseCase{
		{"強調", "*a* _b_ **c** __d__", "p(*a* *b* **c** **d**)"},
		{"入れ子の強調", "***a***", "p(***a***)"},
		{"強調の中の強い強調", "*a **b** c*", "p(*a **b** c*)"},
		{"空白の前では開かない", "a * b *", "p(a * b *)"},
		{"単語の中の _ は強調でない", "snake_case_name and foo*bar*", "p(snake_case_name and foo*bar*)"},
		{"閉じない", "*a **b", "p(*a **b)"},
		{"3 の規則", "*foo**bar**baz*", "p(*foo**bar**baz*)"},
		{"3 の規則 2", "*foo**bar*", "p(*foo**bar*)"},
		{"区切りの長さの違い", "**foo*", "p(**foo*)"},
		{"取り消し線", "~~a~~ ~b~ ~~~c~~~", "p(~~a~~ ~~b~~ ~~~c~~~)"},
		{"チルダの長さが違うと取り消さない", "~~a~", "p(~~a~)"},
		{"コード", "`a` ``b`c`` ` d `", "p(`a` `b`c` `d`)"},
		{"コードの中は書式にならない", "`*a*` `<b>`", "p(`*a*` `<b>`)"},
		{"コードの改行は空白", "`a\nb`", "p(`a b`)"},
		{"閉じないコード", "`a ``b", "p(`a ``b)"},
		{"エスケープ", `\*a\* \# \\ \a`, `p(*a* # \ \a)`},
		{"エスケープしたバッククォート", "\\`a`", "p(`a`)"},
		{"ハード改行", "a  \nb\\\nc", "p(a<br>b<br>c)"},
		{"行末の空白 1 つはソフト", "a \nb", "p(a⏎b)"},
		{"段落末の空白は捨てる", "a  ", "p(a)"},
		{"リンク", `[a](/u "t") [b](<c d>) [e]()`, `p([a](/u "t") [b](c d) [e]())`},
		{"リンクの中の括弧", "[a](f(o)o) [b](f\\)oo)", "p([a](f(o)o) [b](f)oo))"},
		{"閉じない括弧", "[a](foo", "p([a](foo)"},
		{"空白のある宛先はリンクでない", "[a](b c)", "p([a](b c))"},
		{"参照リンク", "[a][x] [x][] [x]\n\n[x]: /url 'T'", `p([a](/url "T") [x](/url "T") [x](/url "T"))`},
		{"参照は大文字小文字を区別しない", "[FOO]\n\n[foo]: /f", "p([FOO](/f))"},
		{"未定義の参照は文字", "[nope] [a][nope]", "p([nope] [a][nope])"},
		{"定義だけの段落は消える", "[x]: /u\n\n[x]", "p([x](/u))"},
		{"先の定義が勝つ", "[x]\n\n[x]: /a\n[x]: /b", "p([x](/a))"},
		{"リンクは入れ子にならない", "[a [b](c) d](e)", "p([a [b](c) d](e))"},
		{"リンクの中の強調", "[*a*](b) *[c](d)*", "p([*a*](b) *[c](d)*)"},
		{"画像", `![alt *x*](/i.png "t")`, `p(![alt *x*](/i.png "t"))`},
		{"リンクの中の画像", "[![a](i)](u)", "p([![a](i)](u))"},
		{"自動リンク", "<http://a.b/c?d> <mailto:x@y> <foo@bar.example>", "p([http://a.b/c?d](http://a.b/c?d) [mailto:x@y](mailto:x@y) [foo@bar.example](mailto:foo@bar.example))"},
		{"自動リンクでない <", "<x b> < c >", "p(<x b> < c >)"},
		{"生の HTML のタグ", "a <b>c</b> <span class=\"x\">", "p(a **c** <span class=\"x\">)"},
		{"拡張自動リンク", "see www.example.com/a. and https://go.dev/x).", "p(see [www.example.com/a](http://www.example.com/a). and [https://go.dev/x](https://go.dev/x)).)"},
		{"拡張自動リンクは日本語の前で切れる", "詳しくは https://example.com/a を参照", "p(詳しくは [https://example.com/a](https://example.com/a) を参照)"},
		{"拡張自動リンクは日本語に続けて書いても切れる", "https://example.com/aを参照", "p([https://example.com/a](https://example.com/a)を参照)"},
		{"単語の途中は自動リンクでない", "xhttp://a.com wwwx.com", "p(xhttp://a.com wwwx.com)"},
		{"ドメインに . が要る", "http://localhost", "p(http://localhost)"},
		{"リンクの中の自動リンクは入れ子にしない", "[see https://a.com](b)", "p([see https://a.com](b))"},
		{"文字参照", "&amp; &lt; &copy; &#12354; &#x3042; &#0; &bogus; &", "p(& < © あ あ � &bogus; &)"},
		{"宛先の文字参照とエスケープ", `[a](/f&amp;o\_o "t&quot;")`, `p([a](/f&o_o "t""))`},
	})
}

func TestParse_Japanese(t *testing.T) {
	runParseCases(t, []parseCase{
		{"日本語の段落", "吾輩は猫である。\n名前はまだ無い。", "p(吾輩は猫である。⏎名前はまだ無い。)"},
		{"日本語の見出し", "# はじめに", "h1(はじめに)"},
		{"かぎ括弧の強調", "これは**「重要」**です", "p(これは**「重要」**です)"},
		{"単語の中の強調", "日本**語**です", "p(日本**語**です)"},
		{"_ の強調", "これは_斜体_です", "p(これは*斜体*です)"},
		{"取り消し線", "~~間違い~~正しい", "p(~~間違い~~正しい)"},
		{"全角の区切りは区切り線でない", "＊＊＊", "p(＊＊＊)"},
		{"日本語のリスト", "- りんご\n- みかん\n  - 温州", "ul{li{p(りんご)} li{p(みかん) ul{li{p(温州)}}}}"},
	})
}

func TestParse_DepthLimit(t *testing.T) {
	deep := strings.Repeat(">", 1000) + " x"
	doc := parse(deep)
	if got := maxDepth(doc); got > maxNesting+1 {
		t.Errorf("quotes nest %d deep, want at most %d", got, maxNesting+1)
	}
	var b strings.Builder
	for i := range 500 {
		b.WriteString(strings.Repeat("  ", i))
		b.WriteString("- x\n")
	}
	doc = parse(b.String())
	if got := maxDepth(doc); got > 2*maxNesting+1 {
		t.Errorf("lists nest %d deep, want at most %d", got, 2*maxNesting+1)
	}
}

// maxDepth returns the depth of the deepest block below b.
func maxDepth(b *block) int {
	d := 0
	for _, c := range b.children {
		d = max(d, maxDepth(c))
	}
	if b.kind == blockDocument {
		return d
	}
	return d + 1
}

func TestPlainText(t *testing.T) {
	doc := parse("a *b* `c` [d](e)\nf  \ng")
	if got := string(plainText(nil, doc.children[0].inlines)); got != "a b c d f\ng" {
		t.Errorf("plain text %q", got)
	}
}

func TestSoftBreak(t *testing.T) {
	cases := []struct{ before, after, want string }{
		{"日本", "語", ""},
		{"。", "次", ""},
		{"word", "next", " "},
		{"日本", "go", " "},
		{"go", "語", " "},
		{"", "a", ""},
		{"한", "국", ""},
	}
	for _, c := range cases {
		if got := softBreak(c.before, c.after); got != c.want {
			t.Errorf("softBreak(%q, %q) = %q, want %q", c.before, c.after, got, c.want)
		}
	}
}
