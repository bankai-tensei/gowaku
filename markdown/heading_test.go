package markdown

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

func TestSlugify(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Getting Started", "getting-started"},
		{"API: v2.0 (beta)", "api-v20-beta"},
		{"見出し", "見出し"},
		{"はじめに「概要」", "はじめに概要"},
		{"Go 言語の UI", "go-言語の-ui"},
		{"snake_case and-dash", "snake_case-and-dash"},
		{"Emoji 🎉 party", "emoji--party"},
		{"ＡＢＣ　全角", "ａｂｃ全角"},
		{"Café Ünïcode", "café-ünïcode"},
		{"100% sure?", "100-sure"},
		{"", ""},
	} {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHeadings(t *testing.T) {
	src := "# 見出し\n\ntext\n\n## FAQ!\n\n## FAQ?\n\n> ### In a *quote*\n\n- ## In a list\n\n# 見出し\n\nSetext `code`\n---\n\n## faq-1\n"
	got := Headings(src)
	want := []Heading{
		{Level: 1, Text: "見出し", Slug: "見出し", Line: 1},
		{Level: 2, Text: "FAQ!", Slug: "faq", Line: 5},
		{Level: 2, Text: "FAQ?", Slug: "faq-1", Line: 7},
		{Level: 3, Text: "In a quote", Slug: "in-a-quote", Line: 9},
		{Level: 2, Text: "In a list", Slug: "in-a-list", Line: 11},
		{Level: 1, Text: "見出し", Slug: "見出し-1", Line: 13},
		{Level: 2, Text: "Setext code", Slug: "setext-code", Line: 15},
		// The slug "faq-1" is taken, so this one counts on from its own base.
		{Level: 2, Text: "faq-1", Slug: "faq-1-1", Line: 18},
	}
	for i := range want {
		want[i].Offset = lineStart(src, want[i].Line)
	}
	if !slices.Equal(got, want) {
		t.Errorf("Headings\n%v\nwant\n%v", got, want)
	}
	if Headings("") != nil {
		t.Error("an empty document has headings")
	}
}

// lineStart returns the byte offset of line n, from 1, of src, whose lines
// end at "\n".
func lineStart(src string, n int) int {
	off := 0
	for range n - 1 {
		off += strings.IndexByte(src[off:], '\n') + 1
	}
	return off
}

func TestHeadings_SourcePosition(t *testing.T) {
	type pos struct {
		text string
		line int
	}
	for _, c := range []struct {
		name string
		src  string
		want []pos
	}{
		{"ATX", "intro\n\n# One\ntext\n### Three ###\n", []pos{{"One", 3}, {"Three", 5}}},
		{"setext", "Title\n=====\n\npara\n\nSub\nline two\n---\n", []pos{{"Title", 1}, {"Sub line two", 6}}},
		{"setext after a reference definition", "[a]: /x\nTitle\n===\n", []pos{{"Title", 2}}},
		{"inside a fence", "```\n# not a heading\n```\n# After\n", []pos{{"After", 4}}},
		{"after a long fence", "# Before\n\n~~~go\n// # no\n\nx := 1\n~~~\n\nAfter\n---\n", []pos{{"Before", 1}, {"After", 9}}},
		{"unclosed fence", "# Top\n```\n# in code\n", []pos{{"Top", 1}}},
		{"in a quote and a list", "> quote\n>\n> ## Q\n\n1. item\n\n   ## L\n", []pos{{"Q", 3}, {"L", 7}}},
		{"HTML", "text\n\n<div>\n<p>x</p>\n<h2>Html</h2>\n</div>\n\n## Md\n", []pos{{"Html", 5}, {"Md", 8}}},
		{"in details", "<details>\n<summary>S</summary>\n\n## Inside\n\n</details>\n", []pos{{"Inside", 4}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Headings(c.src)
			if len(got) != len(c.want) {
				t.Fatalf("Headings = %v, want %v", got, c.want)
			}
			for i, h := range got {
				w := c.want[i]
				if h.Text != w.text || h.Line != w.line {
					t.Errorf("heading %d = %q on line %d, want %q on line %d", i, h.Text, h.Line, w.text, w.line)
				}
				if off := lineStart(c.src, w.line); h.Offset != off {
					t.Errorf("heading %q at offset %d, want %d", h.Text, h.Offset, off)
				}
			}
		})
	}
}

func TestHeadings_OffsetIndexesTheSourceAsWritten(t *testing.T) {
	// CRLF and CR line ends, and invalid UTF-8 that the parser replaces,
	// must not shift the offsets: they index the source as given.
	src := "a\xff\r\n\r\n# One\rtext\r\n\r\n## Two\r\n"
	got := Headings(src)
	if len(got) != 2 {
		t.Fatalf("Headings = %v", got)
	}
	for i, want := range []struct {
		line int
		at   string
	}{{3, "# One"}, {6, "## Two"}} {
		h := got[i]
		if h.Line != want.line || !strings.HasPrefix(src[h.Offset:], want.at) {
			t.Errorf("heading %q: line %d offset %d (%q), want line %d at %q", h.Text, h.Line, h.Offset, src[h.Offset:], want.line, want.at)
		}
	}
}

// longDoc has a link to each of its headings at the top, and enough text
// between the headings to scroll.
func longDoc() string {
	var b strings.Builder
	b.WriteString("[最初へ](#最初) [to second](#second-part) [escaped](#%E4%B8%89%E7%95%AA%E7%9B%AE) [nowhere](#nowhere) [web](https://example.com)\n\n")
	for _, h := range []string{"最初", "Second Part", "三番目"} {
		b.WriteString("## " + h + "\n\n")
		for range 30 {
			b.WriteString("line\n\n")
		}
	}
	return b.String()
}

// headingTop returns where the heading with slug is in the window.
func headingTop(t *testing.T, tt *gowakutest.Tester, slug string) float32 {
	t.Helper()
	r := findHeading(tt.Tree().RootRenderObject(), slug)
	if r == nil {
		t.Fatalf("no heading %q", slug)
	}
	return core.GlobalOffset(r).Y
}

func TestAnchorLink_ScrollsWithoutOnLinkTap(t *testing.T) {
	tt := gowakutest.New(t, basic.Scroll{Child: Markdown{Source: longDoc()}}, gowakutest.Options{Height: 400})
	// Links to headings work, the others stay inert.
	var labels []string
	walk(tt.Semantics(), func(n *gowakutest.Node) {
		if n.Role == semantics.RoleLink {
			labels = append(labels, n.Label)
		}
	})
	if strings.Join(labels, ",") != "最初へ,to second,escaped" {
		t.Errorf("links that work without OnLinkTap: %q", labels)
	}
	follow := func(url string) {
		findLink(t, tt, url).PerformSemanticsAction(semantics.ActionTap, core.SemanticsArg{})
		tt.Pump()
	}
	tt.Tap("to second")
	if y := headingTop(t, tt, "second-part"); y != headingMargin {
		t.Errorf("after the link the heading is at %v, want %v", y, headingMargin)
	}
	// An escaped Japanese anchor, near the end: the view stops at the end.
	follow("#%E4%B8%89%E7%95%AA%E7%9B%AE")
	if y := headingTop(t, tt, "三番目"); y < headingMargin || y > 400 {
		t.Errorf("after the escaped link the heading is at %v", y)
	}
	follow("#最初")
	if y := headingTop(t, tt, "最初"); y != headingMargin {
		t.Errorf("after the Japanese link the heading is at %v, want %v", y, headingMargin)
	}
}

func TestAnchorLink_AppOverrides(t *testing.T) {
	var anchors []Heading
	var links []string
	tt := gowakutest.New(t, basic.Scroll{Child: Markdown{
		Source:      longDoc(),
		OnLinkTap:   func(u string) { links = append(links, u) },
		OnAnchorTap: func(h Heading) { anchors = append(anchors, h) },
	}}, gowakutest.Options{Height: 400})
	before := headingTop(t, tt, "second-part")
	tt.Tap("to second")
	tt.Tap("nowhere")
	tt.Tap("web")
	if !slices.Equal(anchors, []Heading{{Level: 2, Text: "Second Part", Slug: "second-part", Line: anchors[0].Line, Offset: anchors[0].Offset}}) {
		t.Errorf("OnAnchorTap got %v", anchors)
	}
	if strings.Join(links, " ") != "#nowhere https://example.com" {
		t.Errorf("OnLinkTap got %q", links)
	}
	if y := headingTop(t, tt, "second-part"); y != before {
		t.Errorf("the view moved with OnAnchorTap set: %v to %v", before, y)
	}
}

// outline shows longDoc with a controller, and asks for a heading in its
// InitState, before any layout.
type outline struct {
	core.StatefulBase
	first string
	self  **outlineState
}

type outlineState struct {
	core.StateOf[outline]
	c Controller
}

func (outline) CreateState() core.State { return &outlineState{} }

func (s *outlineState) InitState() {
	*s.Widget().self = s
	if f := s.Widget().first; f != "" {
		s.c.ShowHeading(f)
	}
}

func (s *outlineState) Build(core.BuildContext) core.Widget {
	return basic.Scroll{Child: Markdown{Source: longDoc(), Controller: &s.c}}
}

func TestController(t *testing.T) {
	var s *outlineState
	tt := gowakutest.New(t, outline{first: "#second-part", self: &s}, gowakutest.Options{Height: 400})
	// The heading asked for before the layout comes on the next frame.
	tt.Pump()
	if y := headingTop(t, tt, "second-part"); y != headingMargin {
		t.Errorf("the heading asked for in InitState is at %v, want %v", y, headingMargin)
	}
	hs := s.c.Headings()
	if len(hs) != 3 || hs[2].Level != 2 || hs[2].Text != "三番目" || hs[2].Slug != "三番目" {
		t.Errorf("Controller.Headings = %v", hs)
	}
	if s.c.ShowHeading("nowhere") {
		t.Error("ShowHeading reports a heading the document does not have")
	}
	if !s.c.ShowHeading("最初") {
		t.Error("ShowHeading does not find a Japanese slug without #")
	}
	tt.Pump()
	if y := headingTop(t, tt, "最初"); y != headingMargin {
		t.Errorf("after ShowHeading the heading is at %v, want %v", y, headingMargin)
	}
	var zero Controller
	if zero.Headings() != nil {
		t.Error("a controller no Markdown shows has headings")
	}
}

func TestController_RevealDoesNotAllocateAtLayout(t *testing.T) {
	var s *outlineState
	tt := gowakutest.New(t, outline{self: &s}, gowakutest.Options{Height: 400})
	idle := testing.AllocsPerRun(10, func() { tt.Pump() })
	s.c.ShowHeading("三番目")
	tt.Pump()
	// The scroll bar shows for a while after a move.
	tt.Advance(5 * time.Second)
	// Frames after a jump do no more work than before it: the wait for the
	// layout has stopped.
	if allocs := testing.AllocsPerRun(10, func() { tt.Pump() }); allocs != idle {
		t.Errorf("an idle frame after ShowHeading allocates %v times, %v before", allocs, idle)
	}
	if tt.Tree().Ticking() {
		t.Error("a ticker still runs after the heading came into view")
	}
}

func TestController_NewerRequestWinsOverAPendingOne(t *testing.T) {
	var s *outlineState
	tt := gowakutest.New(t, outline{first: "#second-part", self: &s}, gowakutest.Options{Height: 400})
	// The document is laid out now, but the request from InitState still
	// waits for its tick. A newer request moves at once and must not be
	// undone by the older one.
	if !s.c.ShowHeading("三番目") {
		t.Fatal("ShowHeading does not find the heading")
	}
	if s.c.view.ticker.Running() {
		t.Error("the wait for the layout still runs after a move that succeeded at once")
	}
	want := headingTop(t, tt, "三番目")
	tt.Pump()
	tt.Pump()
	if y := headingTop(t, tt, "三番目"); y != want {
		t.Errorf("an older request took the view away: the newer heading went from %v to %v", want, y)
	}
}
