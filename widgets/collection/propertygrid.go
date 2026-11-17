package collection

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// propertyRowHeight is the least height of a row: a control and room of
	// 2 above and below it.
	propertyRowHeight = theme.ControlHeight + 2*theme.Space2
	// propertyIndent is the room before a name, which lines it up with the
	// name of its category after the chevron.
	propertyIndent = theme.Space8 + propertyChevron + theme.Space6
	// propertyChevron is the size of the chevron of a category.
	propertyChevron = 16
	// propertyNameMin and propertyValueMin are the least widths of the
	// columns, and propertyNameShare the share of the names by default.
	propertyNameMin   = 80
	propertyValueMin  = 120
	propertyNameShare = 0.4
	// propertyDescriptionHeight is the height of the area of the description:
	// a name and two lines of caption.
	propertyDescriptionHeight = 2*theme.Space12 + 20 + 2*18
)

// Property is a row of a PropertyGrid: a name and the editor of its value.
type Property struct {
	Name string
	// Description says what the property does. The area at the bottom of the
	// grid shows that of the property in focus, and a screen reader reads it
	// with the editor.
	Description string
	// Editor is the value and the control that edits it. Nil leaves the row
	// without one.
	Editor PropertyEditor
}

// PropertyCategory is a group of properties under a title, which folds.
type PropertyCategory struct {
	Name       string
	Properties []Property
	// Collapsed folds the category when it first shows. The user unfolds it.
	Collapsed bool
}

// PropertyGrid is an inspector: rows of names and values, in categories that
// fold, with the editor of each value to the right of its name.
//
//	PropertyGrid{Filter: true, Categories: []PropertyCategory{{Name: "Layout", Properties: []Property{
//		{Name: "Width", Description: "The width in pixels.", Editor: PropertyInt{Value: w, OnChange: setW}},
//		{Name: "Visible", Editor: PropertyBool{Value: v, OnChange: setV}},
//	}}}}
//
// The properties are described by value, with a typed editor for each kind
// (see PropertyEditor), not read from the fields of a struct: the caller keeps
// the values and changes them in the callbacks, as for every control.
//
// The boundary between the columns drags; a double click puts it back. Tab
// moves through the titles of the categories and the editors; Enter or Space
// on a title folds and unfolds its category, as do Left and Right. Filter adds
// a search field on top, which keeps the properties whose name, or whose
// category's name, holds the text. When any property has a description, an area
// at the bottom shows that of the property last in focus.
//
// Each editor is named to assistive technology by the name of its property,
// with the description as its hint. A category is a group, whose title is a
// button that is expanded or collapsed. The grid takes all the room it is
// given, which must be bounded; the rows scroll.
type PropertyGrid struct {
	core.StatefulBase
	Categories []PropertyCategory
	// Filter shows a search field above the rows.
	Filter bool
	// NameWidth is the width of the column of names, in logical pixels. Zero
	// selects two fifths of the width. The user drags it, as a SplitPane, and
	// a new value from the caller replaces it.
	NameWidth float32
	// OnNameWidthChange reports the width the user dragged the column to.
	OnNameWidthChange func(float32)
	// Label names the grid to assistive technology.
	Label string
}

// propertyKey says which property: the category and the row in it.
type propertyKey struct{ category, row int }

type propertyGridState struct {
	core.StateOf[PropertyGrid]
	nameWidth float32
	// folded holds the categories the user folded or unfolded, by name.
	folded map[string]bool
	filter string
	// focus is the property in focus, and last the one in focus last, whose
	// description shows. Both have a category of -1 for none.
	focus, last propertyKey
	// boundary is the column boundary under the pointer or a drag.
	hovered, dragging bool
	grip              float32
	fade              motion.Implicit
	hot               *motion.Value[float32]
	body              *renderPropertyBody
	right, down       *paint.Path
}

// CreateState implements core.StatefulWidget.
func (PropertyGrid) CreateState() core.State { return &propertyGridState{} }

// InitState implements core.State.
func (s *propertyGridState) InitState() {
	s.nameWidth = s.Widget().NameWidth
	s.folded = map[string]bool{}
	s.focus, s.last = propertyKey{-1, 0}, propertyKey{-1, 0}
	s.fade.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.hot = motion.Track(&s.fade, 0, motion.LerpFloat)
	s.right, s.down = kit.ChevronPath(true), kit.ChevronGlyph()
}

// DidUpdateWidget implements core.State.
func (s *propertyGridState) DidUpdateWidget(old core.Widget) {
	if w := s.Widget(); w.NameWidth != old.(PropertyGrid).NameWidth {
		s.nameWidth = w.NameWidth
	}
}

// Dispose implements core.State.
func (s *propertyGridState) Dispose() { s.fade.Dispose() }

// isFolded reports whether category c shows its title alone.
func (s *propertyGridState) isFolded(c *PropertyCategory) bool {
	if f, ok := s.folded[c.Name]; ok {
		return f
	}
	return c.Collapsed
}

func (s *propertyGridState) setFolded(c *PropertyCategory, fold bool) {
	if s.isFolded(c) != fold {
		s.folded[c.Name] = fold
		kit.Rebuild(&s.StateBase)
	}
}

// matches reports whether text, lowered, holds the filter.
func (s *propertyGridState) matches(text string) bool {
	return s.filter == "" || strings.Contains(strings.ToLower(text), s.filter)
}

func (s *propertyGridState) hover(inside bool) {
	s.hovered = inside
	s.syncBoundary()
}

func (s *propertyGridState) syncBoundary() {
	s.hot.Set(kit.OnOff(s.hovered || s.dragging))
	s.fade.Commit(kit.HoverDuration, motion.Linear)
	kit.Rebuild(&s.StateBase)
}

func (s *propertyGridState) dragStart() {
	s.dragging = true
	if s.body != nil {
		s.grip = s.body.name
	}
	s.syncBoundary()
}

func (s *propertyGridState) drag(delta float32) {
	if s.body == nil {
		return
	}
	width := propertyNameWidth(s.grip+delta, s.body.Size().W)
	if width != s.nameWidth {
		s.nameWidth = width
		if f := s.Widget().OnNameWidthChange; f != nil {
			f(width)
		}
		kit.Rebuild(&s.StateBase)
	}
}

func (s *propertyGridState) dragEnd() {
	s.dragging = false
	s.syncBoundary()
}

// reset puts the boundary back at NameWidth, or its default.
func (s *propertyGridState) reset() {
	s.nameWidth = s.Widget().NameWidth
	if f := s.Widget().OnNameWidthChange; f != nil && s.body != nil {
		f(propertyNameWidth(s.nameWidth, s.body.Size().W))
	}
	kit.Rebuild(&s.StateBase)
}

// focusChanged follows the focus into and out of the row of k.
func (s *propertyGridState) focusChanged(k propertyKey, within bool) {
	switch {
	case within:
		s.focus, s.last = k, k
	case s.focus == k:
		s.focus = propertyKey{-1, 0}
	default:
		return
	}
	kit.Rebuild(&s.StateBase)
}

// Build implements core.State.
func (s *propertyGridState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	rows := make([]core.Widget, 0, 2*len(w.Categories))
	described := false
	for ci := range w.Categories {
		cat := &w.Categories[ci]
		whole := s.matches(cat.Name)
		var props []core.Widget
		for pi := range cat.Properties {
			p := &cat.Properties[pi]
			described = described || p.Description != ""
			if !whole && !s.matches(p.Name) {
				continue
			}
			props = append(props, s.row(th, propertyKey{ci, pi}, p))
		}
		if len(props) == 0 && s.filter != "" {
			continue
		}
		// A filter shows what it found, folded or not.
		folded := s.isFolded(cat) && s.filter == ""
		children := make([]core.Widget, 0, 1+len(props))
		children = append(children, propertyHeader{
			name: cat.Name, folded: folded, right: s.right, down: s.down,
			onFold: func(fold bool) { s.setFolded(&s.Widget().Categories[ci], fold) },
		})
		if !folded {
			children = append(children, props...)
		}
		rows = append(rows, interact.Semantics{Role: semantics.RoleGroup, Label: cat.Name, Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStretch, Children: children,
		}})
	}
	line := c.Hair.WithAlpha(0)
	line = motion.LerpColor(line, c.Ink3, s.hot.Get())
	body := propertyBody{
		nameWidth: s.nameWidth, ref: &s.body,
		children: [2]core.Widget{
			basic.Scroll{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: rows}},
			kit.SplitHandle{
				Axis: layout.Horizontal, Line: line,
				OnHover: s.hover, OnStart: s.dragStart, OnDrag: s.drag, OnEnd: s.dragEnd, OnReset: s.reset,
			},
		},
	}
	parts := make([]core.Widget, 0, 4)
	if w.Filter {
		words := basic.StringsOf(ctx)
		parts = append(parts, basic.Padding{Insets: geom.Insets{Left: theme.Space8, Top: theme.Space8, Right: theme.Space8, Bottom: theme.Space4},
			Child: field.Search{Value: s.filter, Placeholder: words.FilterProperties,
				OnChange: func(t string) {
					s.filter = strings.ToLower(strings.TrimSpace(t))
					kit.Rebuild(&s.StateBase)
				}},
		})
	}
	parts = append(parts, basic.Expanded{Child: body})
	if described {
		parts = append(parts, basic.Container{Height: 1, Color: c.Hair}, s.description(th))
	}
	grid := basic.Column{Cross: layout.CrossStretch, Children: parts}
	if w.Label == "" {
		return grid
	}
	return interact.Semantics{Role: semantics.RoleGroup, Label: w.Label, Child: grid}
}

// row returns the row of property p.
func (s *propertyGridState) row(th *theme.Theme, k propertyKey, p *Property) core.Widget {
	c := &th.Color
	ink, fill := c.Ink2, geom.Color{}
	if s.focus == k {
		ink, fill = c.Ink, c.Fill
	}
	var editor core.Widget = basic.SizedBox{}
	if p.Editor != nil {
		editor = p.Editor.editor()
	}
	return focusWithin{
		onChange: func(within bool) { s.focusChanged(k, within) },
		child: basic.Container{Color: fill, Child: propertyRow{
			nameWidth: s.nameWidth,
			children: [2]core.Widget{
				// The editor says the name; the text would say it twice.
				silent{child: basic.Padding{
					Insets: geom.Insets{Left: propertyIndent, Right: theme.Space8},
					Child:  basic.Text{Content: p.Name, Style: kit.Colored(th.Text.Body, ink)},
				}},
				basic.Padding{
					Insets: geom.Insets{Top: theme.Space2, Right: theme.Space8, Bottom: theme.Space2},
					Child:  propertyName{name: p.Name, hint: p.Description, child: basic.Align{Alignment: layout.AlignCenterLeft, Child: editor}},
				},
			},
		}},
	}
}

// description returns the area that shows the description of the property last
// in focus.
func (s *propertyGridState) description(th *theme.Theme) core.Widget {
	var name, text string
	if k := s.last; k.category >= 0 {
		cats := s.Widget().Categories
		if k.category < len(cats) && k.row < len(cats[k.category].Properties) {
			p := &cats[k.category].Properties[k.row]
			name, text = p.Name, p.Description
		}
	}
	return basic.SizedBox{Height: propertyDescriptionHeight, Child: kit.PaneSlot{Child: basic.Padding{
		Insets: geom.Symmetric(theme.Space16, theme.Space12),
		// The editor in focus reads the same as its hint.
		Child: silent{child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space2, Children: []core.Widget{
			basic.Text{Content: name, Style: kit.Colored(th.Text.Headline, th.Color.Ink)},
			basic.Text{Content: text, Style: kit.Colored(th.Text.Caption, th.Color.Ink2)},
		}}},
	}}}
}

// propertyHeader is the title of a category: a chevron and the name, which
// folds and unfolds the category.
type propertyHeader struct {
	core.StatefulBase
	name        string
	folded      bool
	right, down *paint.Path
	onFold      func(fold bool)
}

type propertyHeaderState struct {
	core.StateBase
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (propertyHeader) CreateState() core.State { return &propertyHeaderState{} }

// InitState implements core.State.
func (s *propertyHeaderState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *propertyHeaderState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *propertyHeaderState) Build(ctx core.BuildContext) core.Widget {
	h := s.Widget().(propertyHeader)
	th := basic.ThemeOf(ctx)
	c := &th.Color
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	fill := c.Fill.WithAlpha(c.Fill.A * s.Hot.Get())
	fill = theme.Over(fill, c.Ink, (kit.PressStrength-kit.HoverStrength)*sink)
	glyph := h.down
	state := semantics.Expanded
	if h.folded {
		glyph, state = h.right, semantics.Collapsed
	}
	toggle := func() { s.Widget().(propertyHeader).onFold(!s.Widget().(propertyHeader).folded) }
	body := s.Ring(th.Radius.Control, c.AccentInk, basic.Container{
		Height: theme.ControlHeight, Color: fill, Radius: th.Radius.Control,
		Padding: geom.Insets{Left: theme.Space8},
		Child: basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space6, Children: []core.Widget{
			basic.Icon{Path: glyph, Size: propertyChevron, Color: c.Ink2},
			basic.Text{Content: h.name, Style: kit.Colored(th.Text.Headline, c.Ink)},
		}},
	})
	sem := interact.Semantics{
		Role: semantics.RoleButton, Label: h.name, State: state, OnTap: toggle,
		OnExpand:   func() { s.Widget().(propertyHeader).onFold(false) },
		OnCollapse: func() { s.Widget().(propertyHeader).onFold(true) },
	}
	inner := s.Focusable(toggle, body)
	focus := inner.(interact.Focus)
	// Left folds and Right unfolds, as in a tree.
	focus.OnKey = func(e *core.KeyEvent) bool {
		if e.Modifiers == 0 {
			switch e.Key {
			case input.KeyLeft:
				s.Widget().(propertyHeader).onFold(true)
				return true
			case input.KeyRight:
				s.Widget().(propertyHeader).onFold(false)
				return true
			}
		}
		return inner.(interact.Focus).OnKey(e)
	}
	sem.Child = focus
	return basic.Padding{Insets: geom.Insets{Top: theme.Space2, Bottom: theme.Space2}, Child: sem}
}

// propertyName names the first node inside it by the name of its property,
// with the description as its hint, unless that node names itself. GroupRow
// uses it too, for the control beside its label.
type propertyName struct {
	name, hint string
	child      core.Widget
}

// Key implements core.Widget.
func (propertyName) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p propertyName) CreateRenderObject() core.RenderObject {
	return &renderPropertyName{name: p.name, hint: p.hint}
}

// UpdateRenderObject implements core.RenderWidget.
func (p propertyName) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderPropertyName)
	r.name, r.hint = p.name, p.hint
}

// ChildWidget implements core.SingleChildWidget.
func (p propertyName) ChildWidget() core.Widget { return p.child }

type renderPropertyName struct {
	layout.RenderProxy
	name, hint string
}

// Semantics implements core.RenderObject.
func (r *renderPropertyName) Semantics(sb *core.SemanticsBuilder) {
	sb.Name(r.name)
	sb.Describe(r.hint)
	r.RenderProxy.Semantics(sb)
	sb.Name("")
	sb.Describe("")
}

// focusWithin reports the focus entering and leaving its child.
type focusWithin struct {
	onChange func(within bool)
	child    core.Widget
}

// Key implements core.Widget.
func (focusWithin) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f focusWithin) CreateRenderObject() core.RenderObject {
	return &renderFocusWithin{onChange: f.onChange}
}

// UpdateRenderObject implements core.RenderWidget.
func (f focusWithin) UpdateRenderObject(ro core.RenderObject) {
	ro.(*renderFocusWithin).onChange = f.onChange
}

// ChildWidget implements core.SingleChildWidget.
func (f focusWithin) ChildWidget() core.Widget { return f.child }

type renderFocusWithin struct {
	layout.RenderProxy
	onChange func(within bool)
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderFocusWithin) FocusWithinChanged(within bool) {
	if r.onChange != nil {
		r.onChange(within)
	}
}

// propertyNameWidth returns the width of the column of names of a grid width
// wide, for the width asked for: zero for the default, and within the least
// widths of both columns.
func propertyNameWidth(asked, width float32) float32 {
	if asked <= 0 {
		asked = width * propertyNameShare
	}
	lo := min(float32(propertyNameMin), width)
	hi := max(width-propertyValueMin, lo)
	return geom.Floor32(geom.Clamp32(asked, lo, hi) + 0.5)
}

// propertyRow is the widget of renderPropertyRow.
type propertyRow struct {
	nameWidth float32
	children  [2]core.Widget
}

// Key implements core.Widget.
func (propertyRow) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p propertyRow) CreateRenderObject() core.RenderObject {
	return &renderPropertyRow{nameWidth: p.nameWidth}
}

// UpdateRenderObject implements core.RenderWidget.
func (p propertyRow) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderPropertyRow)
	if r.nameWidth != p.nameWidth {
		r.nameWidth = p.nameWidth
		r.MarkNeedsLayout()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (p propertyRow) ChildWidgets() []core.Widget { return p.children[:] }

// renderPropertyRow lays out a name and an editor in the two columns, as tall as
// the taller and at least propertyRowHeight, both centered across.
type renderPropertyRow struct {
	core.RenderBase
	nameWidth float32
	children  []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderPropertyRow) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderPropertyRow) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderPropertyRow) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *renderPropertyRow) Layout(c geom.Constraints) geom.Size {
	width := kit.FiniteOr(c.MaxW, 2*propertyNameMin+propertyValueMin)
	name := propertyNameWidth(r.nameWidth, width)
	n := core.LayoutChild(r.children[0], geom.Constraints{MinW: name, MaxW: name, MaxH: geom.Inf})
	v := core.LayoutChild(r.children[1], geom.Constraints{MinW: width - name, MaxW: width - name, MaxH: geom.Inf})
	h := max(n.H, v.H, propertyRowHeight)
	r.children[0].SetOffset(geom.Offset{Y: geom.Floor32((h-n.H)/2 + 0.5)})
	r.children[1].SetOffset(geom.Offset{X: name, Y: geom.Floor32((h-v.H)/2 + 0.5)})
	return c.Constrain(geom.Size{W: width, H: h})
}

// Paint implements core.RenderObject.
func (r *renderPropertyRow) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}

// propertyBody is the widget of renderPropertyBody.
type propertyBody struct {
	nameWidth float32
	ref       **renderPropertyBody
	children  [2]core.Widget
}

// Key implements core.Widget.
func (propertyBody) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p propertyBody) CreateRenderObject() core.RenderObject {
	r := &renderPropertyBody{}
	p.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p propertyBody) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderPropertyBody)
	if r.nameWidth != p.nameWidth {
		r.nameWidth = p.nameWidth
		r.MarkNeedsLayout()
	}
	*p.ref = r
}

// ChildWidgets implements core.MultiChildWidget.
func (p propertyBody) ChildWidgets() []core.Widget { return p.children[:] }

// renderPropertyBody fills its box with the rows, and lays the strip that drags
// the column boundary over them, where the rows put the boundary.
type renderPropertyBody struct {
	core.RenderBase
	nameWidth float32
	children  []core.RenderObject
	// name is the width of the column of names at the last layout.
	name float32
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderPropertyBody) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderPropertyBody) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderPropertyBody) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *renderPropertyBody) Layout(c geom.Constraints) geom.Size {
	sz := geom.Size{W: kit.FiniteOr(c.MaxW, c.MinW), H: kit.FiniteOr(c.MaxH, c.MinH)}
	core.LayoutChild(r.children[0], geom.Tight(sz))
	r.name = propertyNameWidth(r.nameWidth, sz.W)
	core.LayoutChild(r.children[1], geom.Tight(geom.Size{W: kit.SplitHit, H: sz.H}))
	r.children[1].SetOffset(geom.Offset{X: r.name - (kit.SplitHit-kit.DividerThickness)/2})
	return c.Constrain(sz)
}

// Paint implements core.RenderObject.
func (r *renderPropertyBody) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
