package markdown

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// everything uses every construct the package supports.
const everything = `# 見出し 1
## Heading 2
### 見出し 3
#### Heading 4
##### 見出し 5
###### Heading 6

吾輩は猫である。名前はまだ無い。
どこで生れたかとんと見当がつかぬ。 *emphasis*, **strong**, ~~strike~~, ` + "`code`" + `,
a [link](https://example.com "title") and <https://go.dev> and www.example.org.
Hard break here` + "  \n" + `and here\
and there.

> quote
> > nested quote

1. one
2. two
   - nested bullet
     - deeper
- [ ] todo
- [x] done

` + "```go\nfunc main() {\n\tprintln(\"a very long line that should not wrap but scroll sideways instead of wrapping\")\n}\n```" + `

    indented code

***

| 左 | 中央 | 右 |
|:---|:---:|---:|
| a | b | c |
| 1 |

![alt text](image.png)

<span>raw *html*</span>
`

// walk calls fn for every node of the accessibility tree.
func walk(n *gowakutest.Node, fn func(*gowakutest.Node)) {
	fn(n)
	for _, c := range n.Children {
		walk(c, fn)
	}
}

func nodesWithRole(tt *gowakutest.Tester, role semantics.Role) []*gowakutest.Node {
	var out []*gowakutest.Node
	walk(tt.Semantics(), func(n *gowakutest.Node) {
		if n.Role == role {
			out = append(out, n)
		}
	})
	return out
}

func scrolled(w core.Widget) core.Widget { return basic.Scroll{Child: w} }

func TestMarkdown_ZeroValue(t *testing.T) {
	tt := gowakutest.New(t, Markdown{}, gowakutest.Options{})
	if n := len(nodesWithRole(tt, semantics.RoleText)); n != 0 {
		t.Errorf("the zero Markdown says %d texts", n)
	}
}

func TestMarkdown_Everything(t *testing.T) {
	tt := gowakutest.New(t, scrolled(Markdown{Source: everything, OnLinkTap: func(string) {}}), gowakutest.Options{Width: 640, Height: 2400})

	headings := nodesWithRole(tt, semantics.RoleHeading)
	if len(headings) != 6 {
		t.Fatalf("%d headings, want 6\n%s", len(headings), tt.Dump())
	}
	for i, h := range headings {
		if h.Level != i+1 {
			t.Errorf("heading %q has level %d, want %d", h.Label, h.Level, i+1)
		}
	}
	if headings[0].Label != "見出し 1" {
		t.Errorf("first heading %q", headings[0].Label)
	}

	links := nodesWithRole(tt, semantics.RoleLink)
	var urls []string
	for _, l := range links {
		urls = append(urls, l.Value)
	}
	want := []string{"https://example.com", "https://go.dev", "http://www.example.org"}
	if strings.Join(urls, " ") != strings.Join(want, " ") {
		t.Errorf("links %q, want %q", urls, want)
	}

	if n := len(nodesWithRole(tt, semantics.RoleList)); n != 4 {
		t.Errorf("%d lists, want 4 (ordered, two nested, tasks)", n)
	}
	checks := nodesWithRole(tt, semantics.RoleCheckbox)
	if len(checks) != 2 || checks[0].State&semantics.Checked != 0 || checks[1].State&semantics.Checked == 0 ||
		checks[0].State&semantics.ReadOnly == 0 || checks[0].Label != "todo" {
		t.Errorf("task checkboxes %+v", checks)
	}

	grids := nodesWithRole(tt, semantics.RoleGrid)
	if len(grids) != 1 || grids[0].RowCount != 3 || grids[0].ColumnCount != 3 {
		t.Fatalf("tables %+v", grids)
	}
	if n := len(nodesWithRole(tt, semantics.RoleColumnHeader)); n != 3 {
		t.Errorf("%d column headers, want 3", n)
	}
	if n := len(nodesWithRole(tt, semantics.RoleGridCell)); n != 6 {
		t.Errorf("%d cells, want 6", n)
	}
	if _, ok := tt.Find("alt text"); !ok {
		t.Error("the image without a provider does not show its alternative text")
	}
	if _, ok := tt.Find("<span>raw html</span>"); !ok {
		t.Errorf("raw HTML is not shown as text\n%s", tt.Dump())
	}
	if _, ok := tt.Find("go"); !ok {
		t.Error("the code block does not name its language")
	}
}

func TestMarkdown_LinkTap(t *testing.T) {
	var got []string
	tt := gowakutest.New(t, Markdown{
		Source:    "Read [the docs](https://example.com/docs) or [日本語](/ja).",
		OnLinkTap: func(url string) { got = append(got, url) },
	}, gowakutest.Options{})
	tt.Tap("the docs")
	tt.Tap("日本語")
	if strings.Join(got, " ") != "https://example.com/docs /ja" {
		t.Errorf("taps %q", got)
	}
	// A press on the text beside a link is no tap.
	tt.TapAt(geom.Offset{X: 2, Y: 8})
	if len(got) != 2 {
		t.Errorf("a press on plain text tapped %q", got[2:])
	}
}

func TestMarkdown_LinkKeyboard(t *testing.T) {
	var got []string
	tt := gowakutest.New(t, Markdown{
		Source:    "[a](/a) and [b](/b)",
		OnLinkTap: func(url string) { got = append(got, url) },
	}, gowakutest.Options{})
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeyEnter, 0)
	if strings.Join(got, " ") != "/b" {
		t.Errorf("Tab Tab Enter followed %q, want /b", got)
	}
	n, _ := tt.Find("b")
	if n.State&semantics.Focused == 0 {
		t.Errorf("the second link is not focused: %+v", n)
	}
	l := findLink(t, tt, "/b")
	if !l.shown {
		t.Error("the focus ring does not show for keyboard focus")
	}
}

func TestMarkdown_LinkScreenReader(t *testing.T) {
	var got string
	tt := gowakutest.New(t, Markdown{Source: "[a](/a)", OnLinkTap: func(url string) { got = url }}, gowakutest.Options{})
	n, _ := tt.Find("a")
	if n.Actions&semantics.ActionTap == 0 {
		t.Fatalf("the link offers no tap: %+v", n)
	}
	tt.Tree().SemanticsTree() // records which object opened each node
	if !tt.Tree().PerformSemanticsAction(n.ID, semantics.ActionTap, core.SemanticsArg{}) {
		t.Error("the link did not take the tap")
	}
	if got != "/a" {
		t.Errorf("the screen reader's tap followed %q", got)
	}
}

func TestMarkdown_LinksInertWithoutCallback(t *testing.T) {
	tt := gowakutest.New(t, Markdown{Source: "[a](/a)"}, gowakutest.Options{})
	if n := len(nodesWithRole(tt, semantics.RoleLink)); n != 0 {
		t.Errorf("%d link nodes without OnLinkTap", n)
	}
	tt.Key(input.KeyTab, 0)
	if f := tt.Tree().Focused(); f != nil {
		t.Errorf("Tab focused %T without OnLinkTap", f)
	}
}

// findLink returns the object of the link to url.
func findLink(t *testing.T, tt *gowakutest.Tester, url string) *renderLink {
	t.Helper()
	var found *renderLink
	var visit func(ro core.RenderObject)
	visit = func(ro core.RenderObject) {
		if l, ok := ro.(*renderLink); ok && l.url == url {
			found = l
		}
		for i := range ro.ChildCount() {
			visit(ro.ChildAt(i))
		}
	}
	visit(tt.Tree().RootRenderObject())
	if found == nil {
		t.Fatalf("no link to %q", url)
	}
	return found
}

func findRich(tt *gowakutest.Tester, contains string) *renderRich {
	var found *renderRich
	var visit func(ro core.RenderObject)
	visit = func(ro core.RenderObject) {
		if r, ok := ro.(*renderRich); ok && found == nil && strings.Contains(r.Text(), contains) {
			found = r
		}
		for i := range ro.ChildCount() {
			visit(ro.ChildAt(i))
		}
	}
	visit(tt.Tree().RootRenderObject())
	return found
}

func TestMarkdown_LinkOverLines(t *testing.T) {
	var got string
	src := "Some words first and then [a link whose text runs over more than one line of the narrow box](/long) after."
	tt := gowakutest.New(t, Markdown{Source: src, OnLinkTap: func(url string) { got = url }}, gowakutest.Options{Width: 180, Height: 400})
	l := findLink(t, tt, "/long")
	if len(l.boxes) < 2 {
		t.Fatalf("the link has %d fragments in a narrow box, want 2 or more", len(l.boxes))
	}
	parent := core.GlobalOffset(l.Parent())
	second := l.boxes[1].Translate(parent)
	tt.TapAt(second.Center())
	if got != "/long" {
		t.Errorf("a tap on the second line of the link followed %q", got)
	}

	// Hovering any fragment underlines all of them.
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: second.Center()})
	if !l.hovered {
		t.Error("the link is not hovered under the pointer")
	}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 179, Y: 399}})
	if l.hovered {
		t.Error("the link stays hovered after the pointer left")
	}
}

// host is a stateful parent that rebuilds the Markdown below it.
type host struct {
	core.StatefulBase
	source  string
	onParse func()
	// self receives the state, so that the test can rebuild it.
	self **hostState
}

type hostState struct {
	core.StateOf[host]
	source string
	count  int
}

func (host) CreateState() core.State { return &hostState{} }

func (s *hostState) InitState() {
	s.source = s.Widget().source
	*s.Widget().self = s
}

func (s *hostState) Build(core.BuildContext) core.Widget {
	// A new closure on every build, as an application would write it.
	count := s.count
	return Markdown{Source: s.source, OnLinkTap: func(string) { _ = count }, onParse: s.Widget().onParse}
}

func TestMarkdown_RebuildDoesNotReparse(t *testing.T) {
	parses := 0
	var hs *hostState
	tt := gowakutest.New(t, host{source: "# a\n\n[b](/b)", onParse: func() { parses++ }, self: &hs}, gowakutest.Options{})
	if parses != 1 {
		t.Fatalf("%d parses after the first build", parses)
	}
	for range 3 {
		hs.SetState(func() { hs.count++ })
		tt.Pump()
	}
	if parses != 1 {
		t.Errorf("%d parses after rebuilds with the same source, want 1", parses)
	}
	hs.SetState(func() { hs.source = "# c" })
	tt.Pump()
	if parses != 2 {
		t.Errorf("%d parses after the source changed, want 2", parses)
	}
	if _, ok := tt.Find("c"); !ok {
		t.Errorf("the new source is not shown\n%s", tt.Dump())
	}
}

func TestMarkdown_JapaneseSoftBreak(t *testing.T) {
	tt := gowakutest.New(t, Markdown{Source: "日本語の\n文章です。\nEnglish\nwords 混在\nする"}, gowakutest.Options{})
	if _, ok := tt.Find("日本語の文章です。 English words 混在する"); !ok {
		t.Errorf("soft breaks\n%s", tt.Dump())
	}
}

// TestMarkdown_EmphasisIsVisible checks that *emphasis* draws differently from
// plain text with the default fonts, which have no italic: the text layer
// slants them (ADR 0101). The advances stay, so only the ink differs.
func TestMarkdown_EmphasisIsVisible(t *testing.T) {
	for _, words := range []string{"強調された日本語", "emphasis", "日本語と English"} {
		shot := func(src string) *gowakutest.Tester {
			return gowakutest.New(t, Markdown{Source: src}, gowakutest.Options{Width: 300, Height: 60})
		}
		plain, emphasis := shot(words), shot("*"+words+"*")
		rp, re := findRich(plain, words), findRich(emphasis, words)
		if rp == nil || re == nil {
			t.Fatalf("%q: no paragraph", words)
		}
		if rp.Size() != re.Size() {
			t.Errorf("%q: emphasis is %v, plain %v", words, re.Size(), rp.Size())
		}
		a, b := plain.Screenshot(), emphasis.Screenshot()
		if string(a.Pix) == string(b.Pix) {
			t.Errorf("%q: emphasis draws the same pixels as plain text", words)
		}
	}
}

func TestMarkdown_HardBreak(t *testing.T) {
	tt := gowakutest.New(t, Markdown{Source: "one  \ntwo\\\nthree"}, gowakutest.Options{})
	r := findRich(tt, "one")
	if r == nil {
		t.Fatal("no paragraph")
	}
	if r.Text() != "one\ntwo\nthree" || r.laidOut.LineCount() != 3 {
		t.Errorf("text %q on %d lines, want three lines", r.Text(), r.laidOut.LineCount())
	}
}

func TestMarkdown_JapaneseWraps(t *testing.T) {
	src := strings.Repeat("吾輩は猫である。名前はまだ無い。", 6)
	tt := gowakutest.New(t, Markdown{Source: src}, gowakutest.Options{Width: 200, Height: 600})
	r := findRich(tt, "吾輩")
	if r == nil {
		t.Fatal("no paragraph")
	}
	if n := r.laidOut.LineCount(); n < 3 {
		t.Errorf("a long Japanese paragraph in 200 pixels takes %d lines", n)
	}
	if r.Size().W > 200 {
		t.Errorf("the paragraph is %v wide in a box of 200", r.Size().W)
	}
}

func TestMarkdown_CodeBlockDoesNotWrap(t *testing.T) {
	src := "```\n" + strings.Repeat("long_identifier ", 30) + "\nsecond\n```"
	tt := gowakutest.New(t, scrolled(Markdown{Source: src}), gowakutest.Options{Width: 300, Height: 400})
	r := findRich(tt, "long_identifier")
	if r == nil {
		t.Fatal("no code")
	}
	if n := r.laidOut.LineCount(); n != 2 {
		t.Errorf("the code takes %d lines, want 2: it must scroll, not wrap", n)
	}
	if r.Size().W <= 300 {
		t.Errorf("the long line is %v wide", r.Size().W)
	}
}

func TestMarkdown_Highlight(t *testing.T) {
	var gotLang string
	red := geom.Color{R: 1, A: 1}
	tt := gowakutest.New(t, Markdown{
		Source: "```go\nfunc f()\n```",
		Highlight: func(lang, code string) []text.Span {
			gotLang = lang
			return []text.Span{{Text: "func", Style: text.Style{Color: red}}, {Text: code[4:]}}
		},
	}, gowakutest.Options{})
	if gotLang != "go" {
		t.Errorf("Highlight got language %q", gotLang)
	}
	r := findRich(tt, "func f()")
	if r == nil || len(r.spans) != 2 || r.spans[0].Style.Color != red {
		t.Errorf("the highlighted spans are not used: %+v", r)
	}
}

func TestMarkdown_HighlightMismatchPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("spans that do not spell out the code did not panic")
		}
	}()
	gowakutest.New(t, Markdown{
		Source:    "```\nabc\n```",
		Highlight: func(string, string) []text.Span { return []text.Span{{Text: "xyz"}} },
	}, gowakutest.Options{})
}

func TestMarkdown_ImageProvider(t *testing.T) {
	var src, alt string
	tt := gowakutest.New(t, Markdown{
		Source: "Before ![a cat](cat.png) after.",
		ImageProvider: func(s, a string) core.Widget {
			src, alt = s, a
			return basic.SizedBox{Width: 40, Height: 30}
		},
	}, gowakutest.Options{})
	if src != "cat.png" || alt != "a cat" {
		t.Errorf("ImageProvider got %q, %q", src, alt)
	}
	if _, ok := tt.Find("Before"); !ok {
		t.Errorf("the text before the image is gone\n%s", tt.Dump())
	}
	if _, ok := tt.Find("after."); !ok {
		t.Errorf("the text after the image is gone\n%s", tt.Dump())
	}
}

func TestMarkdown_LinkedImage(t *testing.T) {
	var got string
	tt := gowakutest.New(t, Markdown{Source: "[![logo](logo.png)](https://example.com)", OnLinkTap: func(u string) { got = u }}, gowakutest.Options{})
	tt.Tap("logo")
	if got != "https://example.com" {
		t.Errorf("tapping the linked image followed %q", got)
	}
}

func TestMarkdown_Styles(t *testing.T) {
	green := geom.Color{G: 0.5, A: 1}
	tt := gowakutest.New(t, Markdown{Source: "[a](/a) b", OnLinkTap: func(string) {}, Styles: Styles{Link: green, Body: text.Style{Size: 20}}}, gowakutest.Options{})
	r := findRich(tt, "a b")
	if r == nil {
		t.Fatal("no paragraph")
	}
	if r.style.Size != 20 {
		t.Errorf("body size %v, want 20", r.style.Size)
	}
	if r.spans[0].Style.Color != green {
		t.Errorf("link color %v, want %v", r.spans[0].Style.Color, green)
	}
}

func TestMarkdown_BodyWithoutSize(t *testing.T) {
	// A Body that sets only a weight leaves the size to text.Style's default;
	// the headings and code derive from that, not from zero.
	tt := gowakutest.New(t, Markdown{Source: "# h\n\n`c`", Styles: Styles{Body: text.Style{Weight: 500}}}, gowakutest.Options{})
	r := findRich(tt, "h")
	if r == nil || r.style.Size != evenPixels(defaultTextSize*1.6) || r.style.LineHeight <= 0 {
		t.Errorf("heading style %+v", r.style)
	}
}

func TestMarkdown_Adversarial(t *testing.T) {
	sources := []string{
		strings.Repeat(">", 500) + " deep",
		deepList(300),
		strings.Repeat("[", 2000) + strings.Repeat("]", 2000),
		strings.Repeat("|a", 200) + "\n" + strings.Repeat("|-", 200),
		"```\n" + strings.Repeat("\t", 100) + "x\n```",
		"- [x]\n- [ ]\n-\n1.\n#\n>\n|a|\n|-|",
		"\x00\xff\r\n\r",
	}
	for _, src := range sources {
		gowakutest.New(t, scrolled(Markdown{Source: src, OnLinkTap: func(string) {}}), gowakutest.Options{Width: 400, Height: 400})
	}
}

func TestMarkdown_NarrowTableWraps(t *testing.T) {
	src := "| " + strings.Repeat("long words in a cell ", 10) + " | b |\n|---|---|\n| x | y |"
	tt := gowakutest.New(t, Markdown{Source: src}, gowakutest.Options{Width: 300, Height: 800})
	var table *renderTable
	var visit func(ro core.RenderObject)
	visit = func(ro core.RenderObject) {
		if r, ok := ro.(*renderTable); ok {
			table = r
		}
		for i := range ro.ChildCount() {
			visit(ro.ChildAt(i))
		}
	}
	visit(tt.Tree().RootRenderObject())
	if table == nil {
		t.Fatal("no table")
	}
	if w := table.Size().W; w > 300 {
		t.Errorf("the table is %v wide in 300", w)
	}
}

func TestMarkdown_TableColumnsKeepTheirWords(t *testing.T) {
	src := "| Item | Description | N |\n|---|:-:|--:|\n" +
		"| apple | red fruit | 3 |\n" +
		"| banana | a long yellow fruit that makes the column wrap when the window is narrow | 12 |\n" +
		"| orange | `code` and *emphasis* | 100 |"
	for _, width := range []float32{360, 480, 640} {
		tt := gowakutest.New(t, Markdown{Source: src}, gowakutest.Options{Width: width, Height: 800})
		var table *renderTable
		var cells []*renderRich
		var visit func(ro core.RenderObject)
		visit = func(ro core.RenderObject) {
			switch r := ro.(type) {
			case *renderTable:
				table = r
			case *renderRich:
				if table != nil {
					cells = append(cells, r)
				}
			}
			for i := range ro.ChildCount() {
				visit(ro.ChildAt(i))
			}
		}
		visit(tt.Tree().RootRenderObject())
		if table == nil || len(cells) != 12 {
			t.Fatalf("width %v: table %v with %d cells", width, table != nil, len(cells))
		}
		if w := table.Size().W; w > width {
			t.Errorf("width %v: the table is %v wide", width, w)
		}
		for _, c := range cells {
			content := c.Text()
			var got []string
			for i := range c.laidOut.LineCount() {
				a, b := c.laidOut.LineRange(i)
				got = append(got, strings.Fields(content[a:b])...)
			}
			if strings.Join(got, " ") != strings.Join(strings.Fields(content), " ") {
				t.Errorf("width %v: %q breaks inside a word", width, content)
			}
		}
	}
}

func TestMarkdown_TableTooNarrowForItsWordsStillFits(t *testing.T) {
	src := "| " + strings.Repeat("unbreakablewordnumber ", 4) + "| " + strings.Repeat("anotherverylongword ", 4) + "|\n|---|---|\n| x | y |"
	tt := gowakutest.New(t, Markdown{Source: src}, gowakutest.Options{Width: 200, Height: 800})
	var table *renderTable
	var visit func(ro core.RenderObject)
	visit = func(ro core.RenderObject) {
		if r, ok := ro.(*renderTable); ok {
			table = r
		}
		for i := range ro.ChildCount() {
			visit(ro.ChildAt(i))
		}
	}
	visit(tt.Tree().RootRenderObject())
	if table == nil {
		t.Fatal("no table")
	}
	if w := table.Size().W; w > 200 {
		t.Errorf("the table is %v wide in 200", w)
	}
}

func TestMarkdown_TableCellWithAFractionalWidthStaysWhole(t *testing.T) {
	// At these sizes "（1）", which cannot break, is a fraction of a pixel
	// wide that adding the padding of the cell and taking it off again in
	// float32 rounds down, so the text would get less than it needs.
	src := "| N | Description |\n|---|---|\n" +
		"| （1） | a long description that makes the column wrap when the window is narrow |"
	for _, size := range []float32{12.8, 13.8, 14.8} {
		for _, width := range []float32{800, 360} {
			tt := gowakutest.New(t, Markdown{Source: src, Styles: Styles{Body: text.Style{Size: size}}},
				gowakutest.Options{Width: width, Height: 800})
			c := findRich(tt, "（1）")
			if c == nil {
				t.Fatal("no cell")
			}
			if n := c.laidOut.LineCount(); n != 1 {
				t.Errorf("size %v, width %v: %q takes %d lines in %v, at least %v", size, width, c.Text(), n, c.width, c.minWidth())
			}
		}
	}
}
