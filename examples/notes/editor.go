package main

import (
	"fmt"
	"slices"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// textStyle is text.Style, which the files of the sample name often.
type textStyle = text.Style

const (
	// editorLineHeight is the line pitch of the editor, airy for reading.
	editorLineHeight = 1.8
	statusHeight     = 28
)

// editorPage is the page of a window with the tabs and the text of the
// selected one.
type editorPage struct {
	core.StatefulBase
	win *windowModel
}

type editorState struct {
	core.StateOf[editorPage]
	// fields hold the selection of each document, so that a tab that comes
	// back has its caret where it was.
	fields map[int]*fieldw.InputController
	find   findState
	// shown is the document the field was last given.
	shown int
	// line and col are where the caret is.
	line, col int
}

// CreateState implements core.StatefulWidget.
func (editorPage) CreateState() core.State { return &editorState{shown: -1, line: 1, col: 1} }

// InitState implements core.State.
func (s *editorState) InitState() {
	s.find.fold = true
	s.fields = make(map[int]*fieldw.InputController)
}

// Dispose implements core.State.
func (s *editorState) Dispose() {
	if ws := s.Widget().win.state; ws != nil && ws.editor == s {
		ws.editor = nil
	}
}

// fieldOf returns the controller of the field of d. A new one starts with the
// caret where d last had it; no field shows it yet, so this may run in Build.
func (s *editorState) fieldOf(d *document) *fieldw.InputController {
	c := s.fields[d.id]
	if c == nil {
		c = &fieldw.InputController{}
		c.SetSelection(d.caret, d.caret)
		s.fields[d.id] = c
	}
	return c
}

// field returns the controller of the field of the current document, or nil.
func (s *editorState) field() *fieldw.InputController {
	d := s.Widget().win.current()
	if d == nil {
		return nil
	}
	return s.fieldOf(d)
}

// selection returns the ends of the selection of the current document, in
// order.
func (s *editorState) selection() (start, end int) {
	c := s.field()
	if c == nil {
		return 0, 0
	}
	a, b := c.Selection()
	return min(a, b), max(a, b)
}

// later runs fn on the field of the current document after the frame, which
// is what a Build asks for: the field it builds is not laid out yet, and the
// controller runs callbacks.
func (s *editorState) later(fn func(*fieldw.InputController)) {
	s.Widget().win.app.shell.post(func() {
		if s.Mounted() {
			if c := s.field(); c != nil {
				fn(c)
			}
		}
	})
}

// caretMoved keeps the caret of d, and shows its line and column.
func (s *editorState) caretMoved(d *document, caret int) {
	d.caret = caret
	if d.id != s.shown || caret > len(d.text) {
		return
	}
	if line, col := lineColumn(d.text, caret); line != s.line || col != s.col {
		s.SetState(func() { s.line, s.col = line, col })
	}
}

// noteCaret keeps the caret of the current document. The field reports every
// move, so this has only to read the controller.
func (s *editorState) noteCaret() {
	if d := s.Widget().win.current(); d != nil && d.id == s.shown {
		_, d.caret = s.fieldOf(d).Selection()
	}
}

// editText runs an editing command on the field with the keyboard: the query
// or the replacement of the find bar, or else the text.
func (s *editorState) editText(fn func(*fieldw.InputController)) {
	switch {
	case s.find.open && s.find.queryField.Focused():
		fn(&s.find.queryField)
	case s.find.open && s.find.withField.Focused():
		fn(&s.find.withField)
	default:
		if c := s.field(); c != nil {
			fn(c)
		}
	}
}

// Build implements core.State.
func (s *editorState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().win
	if ws := w.state; ws != nil {
		ws.editor = s
	}
	a := w.app
	th := basic.ThemeOf(ctx)
	d := w.current()
	if d == nil {
		return basic.Container{Color: th.Color.Canvas}
	}
	// A field built anew for d runs no callbacks, so its controller can take
	// a selection and the keyboard here; the field takes them once laid out.
	fresh := d.id != s.shown
	if fresh {
		s.shown = d.id
		s.line, s.col = lineColumn(d.text, d.caret)
		s.forget()
		s.fieldOf(d).Focus()
	}
	if p := w.pick; p != nil {
		w.pick = nil
		*p = wholeMarks(d.text, *p)
		s.line, s.col = lineColumn(d.text, p.start)
		pick := func(c *fieldw.InputController) {
			c.SetSelection(p.start, p.end)
			c.Focus()
		}
		if fresh {
			pick(s.fieldOf(d))
		} else {
			s.later(pick)
		}
	}
	style := textStyle{Family: a.family(), Size: a.fontSize, LineHeight: editorLineHeight}
	if d.vertical {
		style.Orientation = text.Vertical
	}
	// The text is the page: a bare field, whose margins scroll with it.
	field := fieldw.Input{
		Value: d.text, Multiline: true, Style: style, Placeholder: "ここに書く",
		Bare:              true,
		Padding:           geom.Insets{Left: theme.Space24, Top: theme.Space4, Right: theme.Space24, Bottom: theme.Space16},
		OnChange:          func(v string) { s.edited(d, v) },
		OnSelectionChange: func(_, caret int) { s.caretMoved(d, caret) },
		Controller:        s.fieldOf(d),
		Ruby:              rubyMarks,
	}
	editor := core.Keyed(d.id, interact.Listener{
		OnPointer: func(e *core.PointerEvent) {
			// Ctrl and the wheel come as a pinch, which the field leaves alone.
			if e.Kind == core.PointerPinch {
				e.Handled = true
				switch {
				case e.Pinch > 1.01:
					a.zoom(1)
				case e.Pinch < 0.99:
					a.zoom(-1)
				}
			}
		},
		Child: field,
	})
	page := []core.Widget{editor}
	if s.find.open {
		// The bar keeps off the start of the text: the top right in vertical
		// writing, where the first column is.
		bar := basic.Positioned{Top: theme.Space8, Right: theme.Space28, AnchorRight: true, Width: findWidth, Child: s.findBar(ctx, d)}
		if d.vertical {
			bar.Left, bar.Right, bar.AnchorRight = theme.Space28, 0, false
		}
		page = append(page, bar)
	}
	return basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Padding{Insets: geom.Insets{Left: theme.Space8, Top: theme.Space6, Right: theme.Space8}, Child: s.tabBar()},
		basic.Expanded{Child: basic.Container{Color: th.Color.Card, Child: basic.Column{
			Cross: layout.CrossStretch, Children: []core.Widget{
				s.header(th, d),
				basic.Expanded{Child: basic.Stack{Fit: layout.StackExpand, Children: page}},
			},
		}}},
		basic.Container{Height: 1, Color: th.Color.Hair},
		s.statusBar(th, d),
	}}
}

// forget drops the controllers of documents that are no longer open.
func (s *editorState) forget() {
	docs := s.Widget().win.app.docs
	for id := range s.fields {
		if _, ok := docs[id]; !ok {
			delete(s.fields, id)
		}
	}
}

// edited takes an edit of the field.
func (s *editorState) edited(d *document, v string) {
	w := s.Widget().win
	w.app.edit(d, v)
	s.SetState(func() {})
}

// header is the title of the document over the text, where the title of a
// card lands when a note opens.
func (s *editorState) header(th *theme.Theme, d *document) core.Widget {
	w := s.Widget().win
	detail := "新しいテキスト。保存するとノート帳に入ります"
	if d.path != "" {
		detail = d.path
	}
	return basic.Padding{
		Insets: geom.Insets{Left: theme.Space16, Top: theme.Space12, Right: theme.Space24, Bottom: theme.Space12},
		Child: basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space12, Children: []core.Widget{
			control.Button{Label: "‹ ノート帳", Style: control.ButtonQuiet, Hint: "Ctrl+Shift+L", OnPress: w.showNotebook},
			basic.Expanded{Child: basic.Column{Size: layout.MainMin, Spacing: theme.Space2, Children: []core.Widget{
				nav.Hero{Tag: w.heroFor(d), Child: basic.Text{Content: d.title(), Style: colored(th.Text.Title, th.Color.Ink)}},
				basic.Text{Content: truncate(detail, 80), Style: colored(th.Text.Caption, th.Color.Ink2)},
			}}},
		}},
	}
}

// tabBar is the strip of the window's documents.
func (s *editorState) tabBar() core.Widget {
	w := s.Widget().win
	a := w.app
	items := make([]nav.TabItem, len(w.tabs))
	for i, id := range w.tabs {
		items[i] = nav.TabItem{Key: id, Label: a.docs[id].title(), Modified: a.docs[id].dirty()}
	}
	ws := w.state
	return nav.TabBar{
		Tabs: items, Selected: w.selected, Label: "開いているテキスト", Group: &a.group,
		OnSelect: w.selectTab,
		OnClose:  func(i int) { ws.closeTab(i) },
		OnMove:   w.moveTab,
		OnNew:    ws.newDoc,
		OnReceive: func(k core.Key, i int) {
			if id, ok := k.(int); ok {
				w.insertTab(i, id)
			}
		},
		OnDetach:  ws.detach,
		OnTearOff: ws.tearOff,
	}
}

// statusBar shows the count of characters, the place of the caret and how
// the file is written.
func (s *editorState) statusBar(th *theme.Theme, d *document) core.Widget {
	style := colored(th.Text.Caption, th.Color.Ink2)
	label := func(v string) core.Widget { return basic.Text{Content: v, Style: style} }
	orientation := "横書き"
	if d.vertical {
		orientation = "縦書き"
	}
	breaks := "LF"
	if d.crlf {
		breaks = "CRLF"
	}
	encoding := "UTF-8"
	if d.bom {
		encoding = "UTF-8 (BOM)"
	}
	return basic.Container{
		Height: statusHeight, Padding: geom.Symmetric(theme.Space16, 0), Color: th.Color.Sidebar,
		Child: basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space20, Children: []core.Widget{
			label(fmt.Sprintf("%s 文字", formatCount(charCount(d.text)))),
			basic.Expanded{Child: basic.SizedBox{}},
			label(fmt.Sprintf("%d 行、%d 列", s.line, s.col)),
			label(orientation),
			label(breaks),
			label(encoding),
		}},
	}
}

// formatCount writes n with a comma every three digits.
func formatCount(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// detach takes out tab i, which went to the bar of another window. A window
// left without tabs closes, unless it is the last.
func (s *windowState) detach(i int) {
	w := s.Widget().model
	w.removeTab(i)
	if len(w.tabs) == 0 && len(w.app.windows) > 1 && slices.Contains(w.app.windows, w) {
		w.closed = true
		w.app.dropWindow(w)
		if w.handle != nil {
			w.handle.Close()
		}
	}
	w.app.saveSession()
}

// tearOff gives a tab dropped away from every bar a window of its own.
func (s *windowState) tearOff(e nav.TabTearOff) {
	w := s.Widget().model
	a := w.app
	id, ok := e.Key.(int)
	if !ok {
		return
	}
	if e.Alone {
		if w.handle != nil {
			w.handle.MoveTo(e)
		}
		return
	}
	w.removeTab(e.Index)
	nw := a.newWindow()
	nw.tabs, nw.startInEditor = []int{id}, true
	if err := a.shell.tearOff(e, nw.windowTitle(), windowView{model: nw}, func() { a.windowClosed(nw) }); err != nil {
		a.dropWindow(nw)
		w.insertTab(e.Index, id)
		return
	}
	a.saveSession()
}
