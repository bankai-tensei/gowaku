package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

// readme returns a Japanese README of the repository, frozen in testdata so
// that rewriting the README does not move what these tests check. Its pictures
// still come from the repository through os.DirFS("..").
func readme(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("testdata/readme.ja.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

func TestHTML_ReadmeHead(t *testing.T) {
	var tapped []string
	tt := gowakutest.New(t, scrolled(Markdown{
		Source:    readme(t),
		Images:    os.DirFS(".."),
		OnLinkTap: func(u string) { tapped = append(tapped, u) },
	}), gowakutest.Options{Width: 900, Height: 900})
	logo, ok := tt.Find("gowaku")
	if !ok || logo.Role != semantics.RoleImage {
		t.Fatalf("no wordmark\n%s", tt.Dump())
	}
	if logo.Rect.W != 280 || logo.Rect.H < 96 || logo.Rect.H > 98 {
		t.Errorf("the wordmark is %v by %v, want 280 wide in its proportions", logo.Rect.W, logo.Rect.H)
	}
	if mid := logo.Rect.X + logo.Rect.W/2; mid < 449 || mid > 451 {
		t.Errorf("the wordmark is centered at %v, want 450", mid)
	}
	tagline := findRich(tt, "日本語に本気な")
	if tagline == nil || tagline.spans[0].Style.Weight != strongWeight || tagline.style.Align != text.AlignCenter {
		t.Fatalf("the tagline is not bold and centered: %+v", tagline)
	}
	if strings.Contains(tagline.Text(), "<b>") {
		t.Errorf("the tagline shows its tags: %q", tagline.Text())
	}
	badges := findRich(tt, "Go Reference")
	if badges == nil || badges.Text() != "Go Reference CI MIT License" {
		t.Fatalf("the badges are not one row of their names: %v", badges)
	}
	tt.Tap("CI")
	tt.Tap("English")
	want := []string{"https://github.com/bankai-tensei/gowaku/actions/workflows/ci.yml", "README.md"}
	if strings.Join(tapped, " ") != strings.Join(want, " ") {
		t.Errorf("tapped %q, want %q", tapped, want)
	}
	shot, ok := tt.Find("パネルとタブのドックレイアウト")
	if !ok || shot.Rect.W != 584 {
		t.Errorf("the screenshot is not 584 wide: %+v", shot)
	}
	for _, n := range tt.FindAll("<") {
		t.Errorf("a tag shows as text: %q", n.Label)
	}
}

func TestHTML_PictureFollowsTheTheme(t *testing.T) {
	var asked []string
	var hs *themeHostState
	host := themeHost{self: &hs, child: Markdown{
		Source: "<picture>\n<source media=\"(prefers-color-scheme: dark)\" srcset=\"d.png\">\n" +
			"<source media=\"(prefers-color-scheme: light)\" srcset=\"l.png\">\n<img src=\"i.png\" alt=\"pic\">\n</picture>\n\n" +
			"![only dark](x.png#gh-dark-mode-only)![only light](y.png#gh-light-mode-only)",
		ImageProvider: func(src, _ string) core.Widget {
			asked = append(asked, src)
			return basic.SizedBox{Width: 10, Height: 10}
		},
	}}
	tt := gowakutest.New(t, host, gowakutest.Options{})
	if strings.Join(asked, " ") != "l.png y.png" {
		t.Errorf("light theme asked for %q", asked)
	}
	asked = nil
	hs.SetState(func() { hs.dark = true })
	tt.Pump()
	if strings.Join(asked, " ") != "d.png x.png" {
		t.Errorf("dark theme asked for %q", asked)
	}
}

// themeHost shows child in a light or a dark theme.
type themeHost struct {
	core.StatefulBase
	self  **themeHostState
	child core.Widget
}

type themeHostState struct {
	core.StateOf[themeHost]
	dark bool
}

func (themeHost) CreateState() core.State { return &themeHostState{} }

func (s *themeHostState) InitState() { *s.Widget().self = s }

func (s *themeHostState) Build(core.BuildContext) core.Widget {
	th := theme.Default().Light
	if s.dark {
		th = theme.Default().Dark
	}
	return basic.Theme{Data: th, Child: s.Widget().child}
}

func TestHTML_ImageSize(t *testing.T) {
	cases := []struct {
		img  string
		w, h float32
	}{
		{`<img src="logo.png" alt="p">`, 200, 80},
		{`<img src="logo.png" alt="p" width="100">`, 100, 40},
		{`<img src="logo.png" alt="p" height="40px">`, 100, 40},
		{`<img src="logo.png" alt="p" width="30" height="30">`, 30, 30},
		{`<img src="logo.png" alt="p" width="50%">`, 200, 80},
		{`<img src="img/wide.png" alt="p" width="100%">`, 400, 100},
		{`<img src="mark.svg" alt="p">`, 64, 32},
		{`<img src="mark.svg" alt="p" width="32">`, 32, 16},
		{`![p](mark.svg)`, 64, 32},
		{`<img src="box.svg" alt="p">`, 30, 10},
	}
	files := pictures(t)
	files["mark.svg"] = &fstest.MapFile{Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="32"><rect width="64" height="32" fill="currentColor"/></svg>`)}
	files["box.svg"] = &fstest.MapFile{Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 10"><text y="9">a</text></svg>`)}
	for _, c := range cases {
		tt := gowakutest.New(t, Markdown{Source: c.img, Images: files}, gowakutest.Options{Width: 400, Height: 400})
		n, ok := tt.Find("p")
		if !ok || n.Role != semantics.RoleImage {
			t.Errorf("%s: no picture\n%s", c.img, tt.Dump())
			continue
		}
		if n.Rect.W != c.w || n.Rect.H != c.h {
			t.Errorf("%s: %v by %v, want %v by %v", c.img, n.Rect.W, n.Rect.H, c.w, c.h)
		}
	}
}

func TestHTML_PicturesShareARow(t *testing.T) {
	src := "<p align=\"right\">\n  <img src=\"logo.png\" alt=\"a\">\n  <img src=\"logo.png\" alt=\"b\" width=\"50\">\n</p>\n\n" +
		"[![c](logo.png)](https://c) [![d](logo.png)](https://d)\n\n<img src=\"logo.png\" alt=\"e\"><img src=\"logo.png\" alt=\"f\">"
	tt := gowakutest.New(t, Markdown{Source: src, Images: pictures(t), OnLinkTap: func(string) {}}, gowakutest.Options{Width: 600, Height: 600})
	at := func(label string) geom.Rect {
		n, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q\n%s", label, tt.Dump())
		}
		return n.Rect
	}
	a, b := at("a"), at("b")
	if (a.Y+a.H) != (b.Y+b.H) || b.X != (a.X+a.W)+pictureGap || (b.X+b.W) != 600 {
		t.Errorf("a %v and b %v are not one row at the right, a space apart", a, b)
	}
	c, d := at("c"), at("d")
	if c.Y != d.Y || d.X != (c.X+c.W)+pictureGap || c.X != 0 {
		t.Errorf("the linked pictures c %v and d %v are not one row", c, d)
	}
	e, f := at("e"), at("f")
	if e.Y != f.Y || f.X != (e.X+e.W) {
		t.Errorf("e %v and f %v, written without space, are not side by side", e, f)
	}
}

func TestHTML_Details(t *testing.T) {
	src := "<details>\n<summary>More <b>info</b></summary>\n\nHidden text\n\n## Inside\n\n</details>\n\n[go](#inside)"
	var s *outlineHostState
	tt := gowakutest.New(t, outlineHost{self: &s, source: src}, gowakutest.Options{Height: 600})
	summary, ok := tt.Find("More info")
	if !ok || summary.Role != semantics.RoleButton || summary.State&semantics.Collapsed == 0 {
		t.Fatalf("no closed summary\n%s", tt.Dump())
	}
	if _, ok := tt.Find("Hidden text"); ok {
		t.Error("a closed details shows its content")
	}
	tt.Tap("More info")
	if _, ok := tt.Find("Hidden text"); !ok {
		t.Fatalf("the details does not open\n%s", tt.Dump())
	}
	if n, _ := tt.Find("More info"); n.State&semantics.Expanded == 0 {
		t.Error("an open details is not expanded")
	}
	tt.Tap("More info")
	if _, ok := tt.Find("Hidden text"); ok {
		t.Fatal("the details does not close")
	}
	// A link to a heading inside opens the details and goes there.
	tt.Tap("go")
	tt.Pump()
	if _, ok := tt.Find("Hidden text"); !ok {
		t.Errorf("a link to a heading in a closed details leaves it closed\n%s", tt.Dump())
	}
	if hs := s.c.Headings(); len(hs) != 1 || hs[0].Slug != "inside" {
		t.Errorf("headings %+v", hs)
	}
}

// outlineHost shows source with a Controller.
type outlineHost struct {
	core.StatefulBase
	self   **outlineHostState
	source string
}

type outlineHostState struct {
	core.StateOf[outlineHost]
	c Controller
}

func (outlineHost) CreateState() core.State { return &outlineHostState{} }

func (s *outlineHostState) InitState() { *s.Widget().self = s }

func (s *outlineHostState) Build(core.BuildContext) core.Widget {
	return scrolled(Markdown{Source: s.Widget().source, Controller: &s.c})
}

func TestHTML_HeadingOfAPictureHasAnAnchor(t *testing.T) {
	src := "[top](#gowaku)\n\n" + strings.Repeat("filler\n\n", 60) +
		"<h1 align=\"center\"><img src=\"logo.png\" alt=\"gowaku\"></h1>\n\n" + strings.Repeat("more\n\n", 60)
	tt := gowakutest.New(t, scrolled(Markdown{Source: src, Images: pictures(t)}), gowakutest.Options{Width: 600, Height: 400})
	tt.Tap("top")
	tt.Pump()
	n, ok := tt.Find("gowaku")
	if !ok || n.Rect.Y < 0 || n.Rect.Y > 40 {
		t.Errorf("the heading of a picture is not at the top: %+v", n)
	}
	if n.Rect.X+n.Rect.W/2 != 300 {
		t.Errorf("the heading of a picture is not centered: %v", n.Rect)
	}
}

func TestHTML_UnsafeLinksAreText(t *testing.T) {
	var tapped []string
	src := "<p><a href=\"javascript:alert(1)\">one</a></p>\n\n[two](javascript:alert(2)) <a href=\"vbscript:x\">three</a>\n\n" +
		"<a href=\"javascript:x\"><img src=\"logo.png\" alt=\"four\"></a>"
	tt := gowakutest.New(t, Markdown{Source: src, Images: pictures(t), OnLinkTap: func(u string) { tapped = append(tapped, u) }},
		gowakutest.Options{})
	if links := nodesWithRole(tt, semantics.RoleLink); len(links) != 0 {
		t.Errorf("unsafe links are links: %+v", links)
	}
	for _, label := range []string{"one", "two", "three", "four"} {
		if n, ok := tt.Find(label); ok {
			tt.TapAt(n.Rect.Center())
		} else if findRich(tt, label) == nil {
			t.Errorf("%q is gone", label)
		}
	}
	if len(tapped) != 0 {
		t.Errorf("OnLinkTap got %q", tapped)
	}
}

func TestHTML_InlineStyles(t *testing.T) {
	src := "Press <kbd>Ctrl</kbd>, H<sub>2</sub>O and x<sup>2</sup>, <del>gone</del>, <i>it</i>, <code>c</code>."
	tt := gowakutest.New(t, Markdown{Source: src}, gowakutest.Options{})
	r := findRich(tt, "Press")
	if r == nil || r.Text() != "Press Ctrl, H2O and x2, gone, it, c." {
		t.Fatalf("text %v", r)
	}
	if len(r.keys) != 1 || len(r.strikes) != 1 || len(r.grounds) != 1 {
		t.Errorf("keys %v, strikes %v, code %v", r.keys, r.strikes, r.grounds)
	}
	var down, up int
	for _, s := range r.shifts {
		switch {
		case s > 0:
			down++
		case s < 0:
			up++
		}
	}
	if down != 1 || up != 1 {
		t.Errorf("shifts %v, want one run down and one up", r.shifts)
	}
}

func TestHTML_SelectionCopiesText(t *testing.T) {
	src := "<p align=\"center\">\n  <b>Bold</b> and <a href=\"https://x\">a link</a>\n</p>\n\n" +
		"<p><img src=\"missing.png\" alt=\"pic\"> text<br>next</p>\n\n<p><img src=\"logo.png\" alt=\"real\"></p>\n\nend"
	r := newSelRig(t, field.SelectionArea{Child: basic.Scroll{Child: Markdown{Source: src, Images: pictures(t), OnLinkTap: func(string) {}}}},
		gowakutest.Options{})
	p := findRich(r.tt, "Bold")
	r.drag(caret(p, 0), caret(p, 2))
	r.tt.Key(input.KeyA, input.ModControl)
	if got, want := r.copied(), "Bold and a link\n\npic text\nnext\n\nend"; got != want {
		t.Errorf("copied %q, want %q", got, want)
	}
}

func TestHTML_PaintAndLayoutDoNotAllocate(t *testing.T) {
	files := os.DirFS("..")
	tt := gowakutest.New(t, field.SelectionArea{Child: basic.Scroll{Child: Markdown{Source: readme(t), Images: files, OnLinkTap: func(string) {}}}},
		gowakutest.Options{Width: 900, Height: 1200})
	var cv paint.Canvas
	cv.Reset()
	tt.Tree().Paint(&cv)
	if n := testing.AllocsPerRun(10, func() {
		cv.Reset()
		tt.Tree().Paint(&cv)
	}); n != 0 {
		t.Errorf("paint: %v allocs", n)
	}
	root := tt.Tree().RootRenderObject()
	c := geom.Tight(geom.Size{W: 900, H: 1200})
	var dirty func(ro core.RenderObject)
	dirty = func(ro core.RenderObject) {
		ro.MarkNeedsLayout()
		for i := range ro.ChildCount() {
			dirty(ro.ChildAt(i))
		}
	}
	dirty(root)
	tt.Tree().Layout(c)
	if n := testing.AllocsPerRun(10, func() {
		dirty(root)
		tt.Tree().Layout(c)
	}); n != 0 {
		t.Errorf("layout: %v allocs", n)
	}
}

// htmlShowcase uses the HTML that READMEs use besides the head of gowaku's.
const htmlShowcase = `<h2 align="center">HTML の部分集合</h2>

<p align="right">右寄せの段落と <a href="https://example.com">リンク</a>。</p>

<p>キー <kbd>Ctrl</kbd>+<kbd>C</kbd>、H<sub>2</sub>O、E = mc<sup>2</sup>、<s>取り消し</s>、<code>code</code>、<i>斜体</i>、<b>太字</b>。<br>
改行のあとの行。</p>

<details>
<summary>閉じた <b>details</b></summary>

見えない文。

</details>

<details open>
<summary>開いた details</summary>

- 中の **Markdown**
- [見出しへ](#html-の部分集合)

</details>

<div align="center">

**div の中の Markdown は中央に揃う**

</div>

<p>知らないタグは文字のまま: <span style="color:red">span</span> <script>alert(1)</script></p>
`

// TestHTML_ReadmeShots renders the READMEs of the repository and the HTML of
// htmlShowcase, light and dark. With MARKDOWN_SHOTS set to a folder, it saves
// them there:
//
//	MARKDOWN_SHOTS=../docs/review/markdown-html go test -run ReadmeShots .
func TestHTML_ReadmeShots(t *testing.T) {
	dir := os.Getenv("MARKDOWN_SHOTS")
	ja := readme(t)
	en, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	table := ja[strings.Index(ja, "<table>"):]
	table = table[:strings.Index(table, "## ")]
	docs := []struct {
		name, src string
		height    float32
	}{
		{"readme-ja", ja, 1000}, {"readme-en", string(en), 1000}, {"readme-ja-table", table, 820}, {"showcase", htmlShowcase, 640},
	}
	for _, d := range docs {
		for _, dark := range []bool{false, true} {
			tt := gowakutest.New(t, basic.Padding{Insets: geom.All(24), Child: scrolled(Markdown{
				Source: d.src, Images: os.DirFS(".."), OnLinkTap: func(string) {},
			})}, gowakutest.Options{Width: 900, Height: d.height, Dark: dark})
			if dir == "" {
				continue
			}
			scheme := "-light.png"
			if dark {
				scheme = "-dark.png"
			}
			tt.SaveScreenshot(filepath.Join(dir, d.name+scheme))
		}
	}
}
