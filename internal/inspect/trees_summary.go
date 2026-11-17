//go:build gowakudebug

package inspect

import (
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/core"
)

// The widget tree of a real window is deep: the root of a window, the
// inherited widgets of the theme and the locale, and the private parts of every
// standard widget come between the application and what it built. A button
// label sits 40 levels down, where the indentation of a Tree pushes it out
// of the panel. So the tree hides the inner workings of gowaku by default, and
// shows a part of the tree below a root of its own when the selection is deep.

// gowakuModule is the path of the module whose inner workings the tree hides.
const gowakuModule = "github.com/bankai-tensei/gowaku"

// maxViewDepth is how many levels a selection may sit below the root of the
// view before the view moves its root down, and viewContext how many levels it
// keeps above the selection when it does.
const (
	maxViewDepth = 9
	viewContext  = 5
)

// internal reports whether el belongs to the inner workings of gowaku: an
// inherited widget, or a widget of an unexported type of the gowaku module,
// such as the paragraph of a Text. The widgets of the application, exported
// or not, are never internal.
func internal(el core.ElementView) bool {
	if el.Kind() == core.ElementInherited {
		return true
	}
	t := reflect.TypeOf(el.Widget())
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	path := t.PkgPath()
	if path != gowakuModule && !strings.HasPrefix(path, gowakuModule+"/") {
		return false
	}
	// Keyed shows the key of its child, which the child's row does not.
	if t.Name() == "keyed" && strings.HasSuffix(path, "/core") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(t.Name())
	return !unicode.IsUpper(r)
}

// summary is the widget tree with or without the inner workings.
type summary struct {
	all bool
}

// shown reports whether el has a row.
func (m summary) shown(el core.ElementView) bool { return m.all || !internal(el) }

// children returns the rows below el: its children, with each hidden child
// replaced by its own rows.
func (m summary) children(el core.ElementView) []core.ElementView {
	var out []core.ElementView
	m.appendChildren(&out, el)
	return out
}

func (m summary) appendChildren(out *[]core.ElementView, el core.ElementView) {
	for i := range el.ChildCount() {
		c := el.ChildAt(i)
		if m.shown(c) {
			*out = append(*out, c)
		} else {
			m.appendChildren(out, c)
		}
	}
}

// hasChildren reports whether el has rows below it.
func (m summary) hasChildren(el core.ElementView) bool {
	for i := range el.ChildCount() {
		c := el.ChildAt(i)
		if m.shown(c) || m.hasChildren(c) {
			return true
		}
	}
	return false
}

// row returns the row that stands for el: el itself, or its nearest shown
// ancestor. It returns the zero view only if no ancestor is shown.
func (m summary) row(el core.ElementView) core.ElementView {
	for ; !el.IsZero(); el = el.Parent() {
		if m.shown(el) {
			return el
		}
	}
	return core.ElementView{}
}

// parent returns the row above the row el, or the zero view at the top.
func (m summary) parent(el core.ElementView) core.ElementView {
	if el.IsZero() {
		return el
	}
	return m.row(el.Parent())
}

// top returns the first rows of a tree whose root element is root.
func (m summary) top(root core.ElementView) []core.ElementView {
	if m.shown(root) {
		return []core.ElementView{root}
	}
	return m.children(root)
}

// path returns the rows from the top down to row, both included.
func (m summary) path(row core.ElementView) []core.ElementView {
	var out []core.ElementView
	for r := row; !r.IsZero(); r = m.parent(r) {
		out = append(out, r)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// below reports how many rows down from view the row lies, or -1 when it does
// not lie below view.
func (m summary) below(view, row core.ElementView) int {
	n := 0
	for r := row; !r.IsZero(); r = m.parent(r) {
		if r == view {
			return n
		}
		n++
	}
	return -1
}
