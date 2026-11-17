package markdown

import (
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// headingTarget is the object of a heading that carries its anchor: its text,
// or the box around a heading of pictures alone.
type headingTarget interface {
	core.RenderObject
	headingSlug() string
	Attached() bool
	// ready reports whether the object was laid out.
	ready() bool
}

// Markdown shows the Markdown document Source. The zero value shows nothing.
//
// It is as tall as its content and as wide as it is allowed, so a long
// document goes in a vertical basic.Scroll; its width must be bounded, as code
// blocks scroll sideways within it. Source is parsed when it changes, not on
// every build. Any text is a document: malformed Markdown shows as text, and
// nothing in Source can make it panic.
//
// The text wraps by the rules of the library's line breaking, Japanese
// included. Links are colored, underlined while the pointer is over them or
// the keyboard is on them, and run OnLinkTap; images come from Images or
// ImageProvider. Pictures that stand on their own with only white space
// between them, as the badges of a README, share a row.
//
// A document that grows as it streams in, as the reply of a chat does, can be
// set as Source on every change: a fenced code block whose closing fence has
// not come yet shows as code up to the end of Source, and CodeBlock tells it
// apart as Open. CopyButton gives each code block a Copy button, and CodeBlock
// replaces or wraps the drawing of a block, for buttons of the app.
//
// The HTML that READMEs use is read within a safe subset, as GitHub reads it:
// centered paragraphs and headings, <details>, <img> with its size, <picture>
// with a source for the dark theme, links, <b>, <kbd>, <sub>, <sup> and the
// like, and tables. Other tags show as the text they are. Nothing in HTML runs
// code or reaches a network (docs/ADR/0119-markdown-html-subset.md).
// Assistive technology sees headings with their levels, lists, links, code,
// tables as grids and task items as read-only checkboxes.
//
// Inside a field.SelectionArea its text can be selected and copied: headings,
// paragraphs, list items, code and the cells of tables, with blank lines
// between blocks and tabs between cells (docs/ADR/0105-markdown-selection.md).
//
// Each heading has a slug, as GitHub makes it (see Headings), and a link to
// "#" and a slug, such as [Install](#install), scrolls the basic.Scroll around
// the document to that heading. A Controller lists the headings and scrolls to
// one from code, for an outline (docs/ADR/0106-markdown-headings.md).
type Markdown struct {
	core.StatefulBase
	Source string
	// OnLinkTap runs when a link is clicked, or activated with Enter or a
	// screen reader, with the destination as written. Nil leaves links
	// colored but inert, and out of the order of Tab. A link to a heading of
	// the document does not come here (see OnAnchorTap); one to "#" and a
	// slug no heading has does.
	OnLinkTap func(url string)
	// OnAnchorTap runs when a link to a heading of the document is followed:
	// its destination is "#" and the slug of a heading, escaped or not, in any
	// case. Nil scrolls the heading into view, as Controller.ShowHeading
	// does. Such links work whether OnLinkTap is set or not.
	OnAnchorTap func(h Heading)
	// Controller, if set, lists the headings of the document and brings one
	// into view from code.
	Controller *Controller
	// Images is where the pictures of the document are: an image whose
	// source is a relative path, as in ![logo](img/logo.png) or <img
	// src="img/logo.svg" width="200">, is read from it as a PNG, JPEG or
	// WebP file, or an SVG drawing if its name ends in .svg, and shown at its
	// own size or the size its width and height ask for, or smaller where the
	// document is narrower. os.DirFS of the folder of the document
	// serves a file on disk, and an embed.FS a document that ships with the
	// program. A picture is read once, when it first shows, on the thread of
	// the UI: keep large pictures out of it, or hand them over with
	// ImageProvider. What was read is kept until Source changes, so a file
	// that changed on disk shows with the next document. Nil reads nothing. Nothing is ever fetched from a
	// network: an address with a scheme shows its alternative text, like a
	// file that is missing or no picture.
	Images fs.FS
	// ImageProvider returns the widget for an image, given its source and its
	// alternative text, and comes before Images: a nil ImageProvider, or a
	// nil widget from it, leaves the image to Images. It is called on every
	// build, so it should look pictures up, not load them. The source of a
	// <picture> is the one for the current theme, and a widget for an <img>
	// with both a width and a height is shown at that size. It is how an app
	// shows remote pictures, such as badges, which the package never fetches;
	// without it they show their alternative text.
	ImageProvider func(src, alt string) core.Widget
	// Highlight colors a code block: it returns the spans of code in order,
	// whose texts joined must be code, as text.Span with the colors and
	// weights to use; zero fields take the code style. lang is the first word
	// of the info string, which may be empty. Nil, or a nil result, shows the
	// code in one color. It panics if the spans do not spell out code.
	Highlight func(lang, code string) []text.Span
	// CopyButton puts a Copy button at the top right of each code block,
	// which writes the code to the clipboard as it is in Source, tabs
	// included, and then says Copied with a check for a moment. It goes
	// through the Host of the window, so in a gowakutest Tester the copy
	// lands in Tester.Clipboard.
	CopyButton bool
	// CodeBlock, if set, replaces or wraps the drawing of each code block,
	// fenced or indented: it gets the block and the widget that Markdown
	// would show, and returns what to show in its place, such as that widget
	// with buttons of the app around it. A nil result keeps the widget of
	// Markdown. It is called on every build.
	CodeBlock func(c CodeBlock) core.Widget
	// Styles overrides the look the theme gives.
	Styles Styles

	// onParse is called after each parse, for tests.
	onParse func()
}

// CodeBlock is a code block of a document, as Markdown.CodeBlock gets it.
type CodeBlock struct {
	// Lang is the first word of Info, the info string after the opening
	// fence. Both are empty for an indented block.
	Lang, Info string
	// Code is the text of the block as Source has it, without the fences
	// and with its tabs; the drawing expands tabs to stops of four columns.
	Code string
	// Open says that a fenced block has no closing fence: Source ends inside
	// it, as a reply that is still streaming in does, and Code grows as more
	// of it comes. The block shows as code meanwhile.
	Open bool
	// Default is the block as Markdown draws it.
	Default core.Widget
}

// Styles are the looks of a Markdown. Each zero field takes a value from the
// theme. A Style that is set is used as it is, with the theme's ink when its
// Color is zero.
type Styles struct {
	// Body is the running text. The zero Style selects the theme's Reading
	// style.
	Body text.Style
	// Headings are the levels 1 to 6. The zero Styles are Body, larger and
	// heavier: 1.6, 1.33, 1.13, 1, 0.93 and 0.87 times its size.
	Headings [6]text.Style
	// Code sets code spans and code blocks. A zero Family selects that of the
	// Code text of the theme (theme.MonoFamilies), and a zero Size 0.9 times
	// that of Body.
	Code text.Style
	// CodeBackground is the ground of code. The zero value selects the
	// theme's Fill.
	CodeBackground geom.Color
	// Link colors links. The zero value selects the theme's AccentInk.
	Link geom.Color
	// Muted colors block quotes, the language of a code block and the
	// alternative text of images. The zero value selects the theme's Ink2.
	Muted geom.Color
	// Rule colors thematic breaks, the lines of tables and the bar of block
	// quotes. The zero value selects the theme's Hair.
	Rule geom.Color
	// BlockSpacing is the space between blocks. Zero selects 12.
	BlockSpacing float32
	// Indent is how far lists and block quotes move their content in. Zero
	// selects 24.
	Indent float32
}

// looks are Styles with every field resolved, and the theme colors that
// Styles does not cover.
type looks struct {
	body, code              text.Style
	headings                [6]text.Style
	codeBackground          geom.Color
	link, muted, rule, ink  geom.Color
	accent, onAccent, focus geom.Color
	headerFill              geom.Color
	blockSpacing, indent    float32
	radius                  float32
}

// headingScale returns the size of heading level i+1 relative to the body.
func headingScale(i int) float32 {
	switch i {
	case 0:
		return 1.6
	case 1:
		return 1.33
	case 2:
		return 1.13
	case 3:
		return 1
	case 4:
		return 0.93
	}
	return 0.87
}

// resolve fills the zero fields of s from th.
func (s *Styles) resolve(th *theme.Theme) looks {
	c := &th.Color
	l := looks{
		body:           s.Body,
		code:           s.Code,
		codeBackground: or(s.CodeBackground, c.Fill),
		link:           or(s.Link, c.AccentInk),
		muted:          or(s.Muted, c.Ink2),
		rule:           or(s.Rule, c.Hair),
		ink:            c.Ink,
		accent:         c.Accent,
		onAccent:       c.OnAccent,
		focus:          c.AccentInk.WithAlpha(ringOpacity),
		headerFill:     c.Fill,
		blockSpacing:   s.BlockSpacing,
		indent:         s.Indent,
		radius:         th.Radius.Check,
	}
	if l.body == (text.Style{}) {
		l.body = th.Text.Reading
	}
	l.body = inked(l.body, c.Ink)
	bodySize := l.body.Size
	if bodySize <= 0 {
		bodySize = defaultTextSize
	}
	for i := range l.headings {
		h := s.Headings[i]
		if h == (text.Style{}) {
			h = l.body
			h.Size = evenPixels(bodySize * headingScale(i))
			h.Weight = 600
			// The body's leading suits reading at length; headings are
			// short and set tighter.
			h.LineHeight = evenPixels(h.Size*1.3) / h.Size
			if i == 5 {
				h.Color = l.muted
			}
		}
		l.headings[i] = inked(h, c.Ink)
	}
	if l.code.Family == "" {
		l.code.Family = th.Text.Code.Family
	}
	if l.code.Size == 0 {
		l.code.Size = evenPixels(bodySize * 0.9)
	}
	l.code = inked(l.code, c.Ink)
	if l.blockSpacing == 0 {
		l.blockSpacing = theme.Space12
	}
	if l.indent == 0 {
		l.indent = theme.Space24
	}
	return l
}

// ringOpacity is the alpha of the focus ring, as the controls draw it.
const ringOpacity = 0.28

func or(c, fallback geom.Color) geom.Color {
	if c == (geom.Color{}) {
		return fallback
	}
	return c
}

func inked(s text.Style, ink geom.Color) text.Style {
	if s.Color == (geom.Color{}) {
		s.Color = ink
	}
	return s
}

// evenPixels rounds a length to whole pixels.
func evenPixels(v float32) float32 { return float32(int(v + 0.5)) }

// CreateState implements core.StatefulWidget.
func (Markdown) CreateState() core.State { return &markdownState{} }

type markdownState struct {
	core.StateOf[Markdown]
	// source is what doc was parsed from, slugs its headings by slug and
	// headings the list of them.
	source   string
	doc      *block
	slugs    map[string]*block
	headings []Heading
	// tap is made once, so that the widgets built from the document do not
	// change with the closure the parent passes as OnLinkTap.
	tap func(url string)
	// onResize is resized, made once like tap.
	onResize func()
	// controller is the Controller the state is bound to.
	controller *Controller
	// pending is a heading to bring into view once the document is laid out,
	// which ticker waits for over at most waitFrames frames. A newer request
	// replaces it. following tells that it brings follow back after a change
	// of height, which the user scrolling cancels.
	pending   string
	following bool
	waited    int
	ticker    *core.Ticker
	// follow is the heading last brought into view, which stays at the top
	// while the document changes height above it, as when a picture comes in
	// late, until the user scrolls: followView is the nearest Scroll and
	// followAt the offset it was moved to.
	follow     string
	followView *layout.RenderViewport
	followAt   float64
	// pictures are the pictures read from Images.
	pictures imageCache
	// opened holds the details blocks the reader opened or closed, by
	// whether they are open now; toggle opens or closes one.
	opened map[*block]bool
	toggle func(c *block)
}

// waitFrames bounds how long a heading asked for before the layout waits
// for it.
const waitFrames = 10

// InitState implements core.State.
func (s *markdownState) InitState() {
	s.tap = func(url string) {
		w := s.Widget()
		if b := anchorOf(s.slugs, url); b != nil {
			if w.OnAnchorTap != nil {
				w.OnAnchorTap(Heading{Level: b.level, Text: headingText(b), Slug: b.slug})
			} else {
				s.showHeading(b.slug)
			}
			return
		}
		if w.OnLinkTap != nil {
			w.OnLinkTap(url)
		}
	}
	s.onResize = s.resized
	s.toggle = func(c *block) {
		s.SetState(func() { s.setOpen(c, !s.isOpen(c)) })
	}
	s.ticker = s.NewTicker(func(time.Duration) {
		s.waited++
		if s.pending == "" || (s.following && s.userScrolled()) || s.reveal(s.pending) || s.waited >= waitFrames {
			s.cancelWait()
		}
	})
}

// Dispose implements core.State.
func (s *markdownState) Dispose() { s.bind(nil) }

// bind makes c the controller of the state.
func (s *markdownState) bind(c *Controller) {
	if s.controller == c {
		return
	}
	if old := s.controller; old != nil && old.view == s {
		old.view = nil
	}
	s.controller = c
	if c != nil {
		c.view = s
		if p := c.pending; p != "" {
			c.pending = ""
			if b := anchorOf(s.slugs, "#"+p); b != nil {
				// This runs in Build, before the document is built, so the
				// details open in this build.
				s.openAround(b)
				s.wait(b.slug)
			}
		}
	}
}

// isOpen reports whether the details block c is open.
func (s *markdownState) isOpen(c *block) bool {
	if open, ok := s.opened[c]; ok {
		return open
	}
	return c.startsOpen
}

func (s *markdownState) setOpen(c *block, open bool) {
	if s.opened == nil {
		s.opened = map[*block]bool{}
	}
	s.opened[c] = open
}

// openAround opens the closed details blocks around b, so that it shows, and
// reports whether there were any.
func (s *markdownState) openAround(b *block) bool {
	changed := false
	for p := b.parent; p != nil; p = p.parent {
		if p.kind == blockDetails && !s.isOpen(p) {
			s.setOpen(p, true)
			changed = true
		}
	}
	return changed
}

// showHeading brings the heading with slug into view, now if the document
// is laid out and else after its layout, and reports whether it has one. A
// heading in a closed details opens it, as a browser opens one for a link to
// what it holds.
func (s *markdownState) showHeading(slug string) bool {
	b := s.slugs[slug]
	if slug == "" || b == nil {
		return false
	}
	if s.openAround(b) {
		s.SetState(func() {})
		s.wait(slug)
		return true
	}
	if s.reveal(slug) {
		// An older request still waiting would take the view away again.
		s.cancelWait()
	} else {
		s.wait(slug)
	}
	return true
}

// wait brings the heading with slug into view after the next layout, in
// place of any heading waiting for it.
func (s *markdownState) wait(slug string) {
	s.pending, s.following, s.waited = slug, false, 0
	s.ticker.Start()
}

// cancelWait drops the heading waiting for the layout, if any.
func (s *markdownState) cancelWait() {
	s.pending, s.following = "", false
	s.ticker.Stop()
}

// headingMargin is the room left above a heading brought into view.
const headingMargin = theme.Space8

// reveal scrolls the heading with slug to the top of the Scrolls around the
// document, and reports whether it could: false before the layout.
func (s *markdownState) reveal(slug string) bool {
	root := s.Context().RenderObject()
	if root == nil {
		return false
	}
	r := findHeading(root, slug)
	if r == nil || !r.ready() || !r.Attached() {
		return false
	}
	sz := r.Size()
	basic.ScrollIntoView(r, geom.Rect{Y: -headingMargin, W: sz.W, H: sz.H + headingMargin}, 0)
	s.follow, s.followView = slug, nearestViewport(r)
	if v := s.followView; v != nil {
		s.followAt = v.Position.Value()
	}
	return true
}

// resized is called when the document lays out at another size. The heading
// brought into view last moves with the text above it, so it comes back to
// the top on the next frame, unless the user has scrolled since.
func (s *markdownState) resized() {
	if s.pending != "" {
		// A heading waits already, and comes to the top on the next frame.
		return
	}
	if s.follow == "" || s.userScrolled() || s.slugs[s.follow] == nil {
		s.follow, s.followView = "", nil
		return
	}
	s.wait(s.follow)
	s.following = true
}

// userScrolled reports whether the Scroll moved since follow was brought
// into view: the wheel, the scroll bar or another jump moved it.
func (s *markdownState) userScrolled() bool {
	v := s.followView
	return v == nil || v.Position == nil || v.Position.Value() != s.followAt
}

// nearestViewport returns the Scroll around ro that is nearest to it, or nil.
func nearestViewport(ro core.RenderObject) *layout.RenderViewport {
	for p := ro.Parent(); p != nil; p = p.Parent() {
		if v, ok := p.(*layout.RenderViewport); ok && v.Position != nil {
			return v
		}
	}
	return nil
}

// docBox holds the document and tells the state when its size changes.
type docBox struct {
	onResize func()
	child    core.Widget
}

// Key implements core.Widget.
func (docBox) Key() core.Key { return nil }

// ChildWidget implements core.SingleChildWidget.
func (d docBox) ChildWidget() core.Widget { return d.child }

// CreateRenderObject implements core.RenderWidget.
func (d docBox) CreateRenderObject() core.RenderObject { return &renderDoc{onResize: d.onResize} }

// UpdateRenderObject implements core.RenderWidget.
func (d docBox) UpdateRenderObject(ro core.RenderObject) { ro.(*renderDoc).onResize = d.onResize }

type renderDoc struct {
	layout.RenderProxy
	onResize func()
	last     geom.Size
}

// Layout implements core.RenderObject.
func (r *renderDoc) Layout(c geom.Constraints) geom.Size {
	sz := r.RenderProxy.Layout(c)
	if sz != r.last {
		first := r.last == (geom.Size{})
		r.last = sz
		if !first && r.onResize != nil {
			r.onResize()
		}
	}
	return sz
}

// findHeading returns the object of the heading with slug below ro, or nil.
func findHeading(ro core.RenderObject, slug string) headingTarget {
	if r, ok := ro.(headingTarget); ok && r.headingSlug() == slug {
		return r
	}
	for i := range ro.ChildCount() {
		if r := findHeading(ro.ChildAt(i), slug); r != nil {
			return r
		}
	}
	return nil
}

// Build implements core.State.
func (s *markdownState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	if s.doc == nil || s.source != w.Source {
		s.doc, s.source = parse(w.Source), w.Source
		s.slugs = assignSlugs(s.doc)
		s.headings = headingsOf(s.doc, w.Source)
		s.pictures.reset()
		clear(s.opened)
		if w.onParse != nil {
			w.onParse()
		}
	}
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: Markdown needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	th := basic.ThemeOf(ctx)
	b := builder{
		shaper:    shaper,
		looks:     w.Styles.resolve(th),
		dark:      th.Brightness == theme.BrightnessDark,
		isOpen:    s.isOpen,
		toggle:    s.toggle,
		images:    w.ImageProvider,
		pictures:  &s.pictures,
		files:     w.Images,
		highlight: w.Highlight,
		onCode:    w.CodeBlock,
		tap:       s.tap,
		external:  w.OnLinkTap != nil,
		slugs:     s.slugs,
		brk:       blockBreak,
	}
	if w.CopyButton {
		words := basic.StringsOf(ctx)
		b.copyLabel, b.copiedLabel = words.Copy, words.Copied
		b.copy = s.copyCode
	}
	s.bind(w.Controller)
	return docBox{onResize: s.onResize, child: b.document(s.doc)}
}

// copyCode writes code to the clipboard, through the host of the window.
func (s *markdownState) copyCode(code string) {
	if s.Mounted() {
		s.Context().Host().SetClipboardText(code)
	}
}

// Controller lists the headings of the document a Markdown shows and brings
// one into view, for an outline beside the document or a link from outside
// it. The zero value is ready; keep it where it outlives a build, such as in
// a State, and put it in Markdown.Controller. A controller belongs to one
// Markdown at a time, and its methods run on the UI goroutine.
type Controller struct {
	// view is the Markdown that shows the controller, or nil.
	view *markdownState
	// pending is a heading asked for while no Markdown showed the
	// controller.
	pending string
}

// Headings returns the headings of the document in order, as the function
// Headings does, or nil while no Markdown shows the controller.
func (c *Controller) Headings() []Heading {
	if c.view == nil {
		return nil
	}
	return slices.Clone(c.view.headings)
}

// ShowHeading scrolls the basic.Scroll around the document, and those around
// it, so that the heading with the slug anchor is at the top of the view, as
// far as the content allows. anchor may start with "#" and be escaped, as the
// destination of a link, and is matched as a link is (see
// Markdown.OnAnchorTap). It reports whether the document has the heading.
//
// Before the document is laid out, as in the InitState of the parent, the
// move waits for the layout; while no Markdown shows the controller, it waits
// for one, and ShowHeading reports true.
func (c *Controller) ShowHeading(anchor string) bool {
	if !strings.HasPrefix(anchor, "#") {
		anchor = "#" + anchor
	}
	if c.view == nil {
		c.pending = strings.TrimPrefix(anchor, "#")
		return true
	}
	b := anchorOf(c.view.slugs, anchor)
	if b == nil {
		return false
	}
	return c.view.showHeading(b.slug)
}
