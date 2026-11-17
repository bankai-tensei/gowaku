package nav

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

const (
	// breadcrumbHeight is the height of the trail, that of a control, and
	// breadcrumbPill the height of the ground a link shows under the pointer.
	breadcrumbHeight = theme.ControlHeight
	breadcrumbPill   = 24
	// breadcrumbPadding is the room at the ends of a segment.
	breadcrumbPadding = theme.Space6
	// breadcrumbIcon is the size of the icon of a segment, and
	// breadcrumbIconGap the gap to its label.
	breadcrumbIcon    = 16
	breadcrumbIconGap = theme.Space4
	// breadcrumbChevron is the size of the chevron between segments.
	breadcrumbChevron = 12
)

// BreadcrumbItem is one segment of a Breadcrumb.
type BreadcrumbItem struct {
	Label string
	// Icon is drawn before the label on a 24 unit grid, such as
	// icons.Home() or icons.Folder() from package icons.
	Icon *paint.Path
}

// Breadcrumb shows where the view is in a hierarchy, as a trail of segments
// from the top to the current place, with chevrons between them:
//
//	Breadcrumb{Items: []BreadcrumbItem{{Label: "Home"}, {Label: "Notes"}, {Label: "2026"}},
//		OnSelect: func(i int) { s.open(path[:i+1]) }}
//
// Every segment but the last is a link to its place, which OnSelect gets the
// index of; without OnSelect, they are plain text. The last segment is the
// current place, which is no link. When the trail does not fit its width, the
// segments after the first fold into a … button, from the first one, and its
// menu opens them; when even the first does not fit beside the last, it folds
// too.
//
// The links are stops of Tab each, as links in a web page are, and Enter opens
// one. A screen reader finds a group named "Breadcrumb" with the links in it,
// and the current place as text described as the current location. The trail
// is 30 high, and takes the whole width it is given.
type Breadcrumb struct {
	core.StatefulBase
	Items    []BreadcrumbItem
	OnSelect func(index int)
	// Label names the trail to assistive technology. Empty selects
	// "Breadcrumb" in the language of the window.
	Label string
}

type breadcrumbState struct {
	core.StateOf[Breadcrumb]
	// from and to are the segments folded into the … button at the last
	// layout; from == to when none are.
	from, to int
	chevron  *paint.Path
	more     *paint.Path
}

// CreateState implements core.StatefulWidget.
func (Breadcrumb) CreateState() core.State { return &breadcrumbState{} }

// InitState implements core.State.
func (s *breadcrumbState) InitState() {
	s.chevron = kit.ChevronPath(true)
	s.more = ellipsisPath(kit.DefaultIconSize)
}

// folded follows the layout, which finds what fits.
func (s *breadcrumbState) folded(from, to int) {
	if s.from != from || s.to != to {
		s.from, s.to = from, to
		kit.Rebuild(&s.StateBase)
	}
}

// select opens segment i.
func (s *breadcrumbState) choose(i int) {
	if f := s.Widget().OnSelect; f != nil {
		f(i)
	}
}

// menuItems returns the menu of the folded segments.
func (s *breadcrumbState) menuItems() []overlay.MenuItem {
	w := s.Widget()
	to := min(s.to, len(w.Items))
	out := make([]overlay.MenuItem, 0, max(to-s.from, 0))
	for i := s.from; i < to; i++ {
		act := interact.Action{Label: escapeMnemonic(w.Items[i].Label)}
		if w.OnSelect != nil {
			act.OnInvoke = func() { s.choose(i) }
		}
		out = append(out, overlay.MenuItem{Action: act})
	}
	return out
}

// escapeMnemonic doubles the & of a label, which a menu would take for the
// mark of a mnemonic.
func escapeMnemonic(label string) string {
	out := label
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] == '&' {
			out = out[:i] + "&" + out[i:]
		}
	}
	return out
}

// Build implements core.State.
func (s *breadcrumbState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	words := basic.StringsOf(ctx)
	label := w.Label
	if label == "" {
		label = words.Breadcrumb
	}
	n := len(w.Items)
	children := make([]core.Widget, 0, 2*n+1)
	chevron := func() core.Widget {
		return crumbSlot{kind: slotSeparator, child: basic.Padding{
			Insets: geom.Insets{Left: theme.Space2, Right: theme.Space2},
			Child:  basic.Icon{Path: s.chevron, Size: breadcrumbChevron, Color: th.Color.Ink3},
		}}
	}
	for i := range w.Items {
		it := &w.Items[i]
		if i > 1 {
			children = append(children, chevron())
		}
		if i == n-1 {
			children = append(children, crumbSlot{child: crumbCurrent{label: it.Label, icon: it.Icon, hint: words.CurrentLocation}})
		} else {
			var press func(bool)
			if w.OnSelect != nil {
				press = func(bool) { s.choose(i) }
			}
			children = append(children, crumbSlot{child: crumbLink{label: it.Label, icon: it.Icon, role: semantics.RoleLink, onPress: press}})
		}
		if i == 0 {
			// The … button and its separator come after the separator
			// before the second segment, which keeps them in the order of
			// Tab too: first › … › later.
			if n > 1 {
				children = append(children, chevron())
			}
			children = append(children,
				crumbSlot{more: true, child: crumbLink{
					label: words.HiddenPath, icon: s.more, role: semantics.RoleButton, iconOnly: true,
					onPress: s.openMenu,
				}},
				chevron(),
			)
		}
	}
	return interact.Semantics{Role: semantics.RoleGroup, Label: label, Child: crumbRow{onFold: s.folded, children: children}}
}

// openMenu opens the menu of the folded segments below the … button.
func (s *breadcrumbState) openMenu(keyboard bool) {
	items := s.menuItems()
	if len(items) == 0 {
		return
	}
	ctx := s.Context()
	r := s.moreButton()
	if r == nil {
		return
	}
	overlay.ShowMenuBelow(ctx, r, items, keyboard)
}

// moreButton returns the render object of the … button, or nil.
func (s *breadcrumbState) moreButton() core.RenderObject {
	ro := s.Context().RenderObject()
	for ro != nil {
		if row, ok := ro.(*renderCrumbRow); ok {
			for _, c := range row.children {
				if slot := c.(*renderCrumbSlot); slot.more {
					return slot
				}
			}
			return nil
		}
		if ro.ChildCount() == 0 {
			return nil
		}
		ro = ro.ChildAt(0)
	}
	return nil
}

// crumbLink is a segment that opens its place, or the … button: a label on no
// ground until the pointer comes, a focus ring from the keyboard, and Enter
// to open, as a link takes.
type crumbLink struct {
	core.StatefulBase
	label    string
	icon     *paint.Path
	role     semantics.Role
	iconOnly bool
	// onPress opens the place; keyboard says the keyboard did it. Nil makes the
	// segment plain text.
	onPress func(keyboard bool)
}

type crumbLinkState struct {
	core.StateBase
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (crumbLink) CreateState() core.State { return &crumbLinkState{} }

// InitState implements core.State.
func (s *crumbLinkState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *crumbLinkState) Dispose() { s.Control.Dispose() }

func (s *crumbLinkState) press(keyboard bool) {
	if f := s.Widget().(crumbLink).onPress; f != nil {
		f(keyboard)
	}
}

// Build implements core.State.
func (s *crumbLinkState) Build(ctx core.BuildContext) core.Widget {
	l := s.Widget().(crumbLink)
	th := basic.ThemeOf(ctx)
	c := &th.Color
	if l.onPress == nil {
		return crumbFace(th, l.label, l.icon, c.Ink2, c.Ink2, false)
	}
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	hot := s.Hot.Get()
	fill := c.Fill.WithAlpha(c.Fill.A * hot)
	fill = theme.Over(fill, c.Ink, (kit.PressStrength-kit.HoverStrength)*sink)
	ink := theme.Over(c.Ink2, c.Ink, hot)
	face := basic.Container{
		Height: breadcrumbPill, Radius: th.Radius.Control, Color: fill, Sink: kit.ButtonSink * sink,
		Child: crumbFace(th, l.label, l.icon, ink, ink, l.iconOnly),
	}
	if l.iconOnly {
		face.Width = breadcrumbPill
	}
	sem := interact.Semantics{Role: l.role, Label: l.label, OnTap: func() { s.press(false) }}
	sem.Child = interact.Focus{
		Enabled: true,
		OnKey: func(e *core.KeyEvent) bool {
			// A link opens with Enter; a button also with Space.
			if e.Key != input.KeyEnter && (e.Key != input.KeySpace || s.Widget().(crumbLink).role == semantics.RoleLink) {
				return false
			}
			if !e.Repeat {
				s.Sink.Kick(kit.KeyKick)
				s.press(true)
			}
			return true
		},
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: interact.MouseRegion{
			Cursor:  input.CursorHand,
			OnEnter: func() { s.Hover(true) },
			OnExit:  func() { s.Hover(false) },
			Child: interact.GestureDetector{
				OnTapDown:   s.press0,
				OnTapCancel: s.Release,
				OnTap: func() {
					s.Release()
					s.press(false)
				},
				Child: s.Ring(th.Radius.Control, c.AccentInk, face),
			},
		},
	}
	return basic.Center{Child: sem}
}

// press0 sinks the segment under a press, which Control.Press does.
func (s *crumbLinkState) press0() { s.Press() }

// crumbFace returns the icon and label of a segment, with its padding.
func crumbFace(th *theme.Theme, label string, icon *paint.Path, ink, tint geom.Color, iconOnly bool) core.Widget {
	if iconOnly {
		return basic.Center{Child: basic.Icon{Path: icon, Size: breadcrumbIcon, Color: tint}}
	}
	text := basic.Text{Content: label, Style: kit.Colored(th.Text.Body, ink)}
	var face core.Widget = text
	if icon != nil {
		face = basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: breadcrumbIconGap, Children: []core.Widget{
			basic.Icon{Path: icon, Size: breadcrumbIcon, Color: tint}, text,
		}}
	}
	return basic.Padding{Insets: geom.Symmetric(breadcrumbPadding, 0), Child: basic.Center{Child: face}}
}

// crumbCurrent is the last segment: the current place, in the weight of a
// control, which a screen reader reads as text described as the current
// location.
type crumbCurrent struct {
	core.StatelessBase
	label string
	icon  *paint.Path
	hint  string
}

// Build implements core.StatelessWidget.
func (c crumbCurrent) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	style := kit.Colored(th.Text.Control, th.Color.Ink)
	var face core.Widget = basic.Text{Content: c.label, Style: style}
	if c.icon != nil {
		face = basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: breadcrumbIconGap, Children: []core.Widget{
			basic.Icon{Path: c.icon, Size: breadcrumbIcon, Color: th.Color.Ink}, face,
		}}
	}
	return interact.Semantics{Role: semantics.RoleText, Label: c.label, Hint: c.hint, Child: basic.Padding{
		Insets: geom.Symmetric(breadcrumbPadding, 0), Child: basic.Center{Child: face},
	}}
}

// crumbSlot is the widget of renderCrumbSlot.
type crumbSlot struct {
	kind slotKind
	// more marks the … button.
	more  bool
	child core.Widget
}

// Key implements core.Widget.
func (crumbSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (c crumbSlot) CreateRenderObject() core.RenderObject {
	return &renderCrumbSlot{kind: c.kind, more: c.more}
}

// UpdateRenderObject implements core.RenderWidget.
func (c crumbSlot) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderCrumbSlot)
	if r.kind != c.kind || r.more != c.more {
		r.kind, r.more = c.kind, c.more
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (c crumbSlot) ChildWidget() core.Widget { return c.child }

// renderCrumbSlot holds one segment, separator or the … button, which the row
// hides when it does not show: the pointer, Tab and assistive technology then
// pass it by.
type renderCrumbSlot struct {
	layout.RenderProxy
	kind   slotKind
	more   bool
	hidden bool
}

// Hidden implements core.Hider.
func (r *renderCrumbSlot) Hidden() bool { return r.hidden }

// Paint implements core.RenderObject.
func (r *renderCrumbSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	if !r.hidden {
		r.RenderProxy.Paint(cv, off)
	}
}

// crumbRow is the widget of renderCrumbRow.
type crumbRow struct {
	onFold   func(from, to int)
	children []core.Widget
}

// Key implements core.Widget.
func (crumbRow) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (c crumbRow) CreateRenderObject() core.RenderObject {
	return &renderCrumbRow{onFold: c.onFold, from: -1}
}

// UpdateRenderObject implements core.RenderWidget.
func (c crumbRow) UpdateRenderObject(ro core.RenderObject) {
	ro.(*renderCrumbRow).onFold = c.onFold
}

// ChildWidgets implements core.MultiChildWidget.
func (c crumbRow) ChildWidgets() []core.Widget { return c.children }

// renderCrumbRow lays out the slots of a trail in a row: the first segment, the
// … button and its separator, then the other segments with a separator before
// each. When they do not all fit, it folds the segments after the first into
// the … button, as few as it can, and reports which.
type renderCrumbRow struct {
	core.RenderBase
	onFold   func(from, to int)
	children []core.RenderObject
	widths   []float32
	// from and to are the folded segments at the last layout; from is -1
	// before one.
	from, to int
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderCrumbRow) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderCrumbRow) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderCrumbRow) ChildAt(i int) core.RenderObject { return r.children[i] }

func (r *renderCrumbRow) slot(i int) *renderCrumbSlot { return r.children[i].(*renderCrumbSlot) }

// segment returns the index of the slot of segment i, and separator that of the
// separator before it. The slots are the first segment, the separator before
// the second, the … button and its separator, then the other segments, each
// after its separator: folded, the trail reads first › … › later.
func segment(i int) int {
	if i == 0 {
		return 0
	}
	return 2*i + 2
}

func separator(i int) int {
	if i == 1 {
		return 1
	}
	return 2*i + 1
}

// The slots of the … button and of its separator.
const (
	crumbMore    = 2
	crumbMoreSep = 3
)

// crumbFold returns the segments [from, to) to fold so that a trail fits in
// limit: the first segment, the … button and its separator of widths more, and
// as many segments from the end as fit. seg holds the widths of the segments
// and sep those of the separators before them; sep[0] is unused. Nothing folds
// when all fit.
func crumbFold(seg, sep []float32, more, limit float32) (from, to int) {
	n := len(seg)
	total := seg[0]
	for i := 1; i < n; i++ {
		total += sep[i] + seg[i]
	}
	if total <= limit || n < 2 {
		return 0, 0
	}
	// The last segment always shows, with the … button before it.
	from, room := 1, limit-seg[0]-sep[1]-more
	if seg[n-1] > room {
		// Not even the first fits beside the last.
		from, room = 0, limit-more
	}
	to = n - 1
	room -= seg[n-1]
	for to-1 > from {
		need := seg[to-1] + sep[to]
		if need > room {
			break
		}
		room -= need
		to--
	}
	return from, to
}

// Layout implements core.RenderObject.
func (r *renderCrumbRow) Layout(c geom.Constraints) geom.Size {
	const height = breadcrumbHeight
	count := len(r.children)
	for i := range count {
		r.slot(i).hidden = true
	}
	var x float32
	if count <= 3 {
		// One segment, then the … button with its separator.
		if count > 0 {
			core.LayoutChild(r.children[0], geom.Constraints{MaxW: geom.Inf, MaxH: height})
			x = r.place(0, x, c.MaxW)
		}
		r.report(0, 0)
		return c.Constrain(geom.Size{W: kit.FiniteOr(c.MaxW, x), H: height})
	}
	n := (count - 1) / 2
	if cap(r.widths) < 2*n {
		r.widths = make([]float32, 2*n)
	}
	r.widths = r.widths[:2*n]
	seg, sep := r.widths[:n], r.widths[n:]
	loose := geom.Constraints{MaxW: geom.Inf, MaxH: height}
	for i := range n {
		seg[i] = core.LayoutChild(r.children[segment(i)], loose).W
		if i > 0 {
			sep[i] = core.LayoutChild(r.children[separator(i)], loose).W
		}
	}
	more := core.LayoutChild(r.children[crumbMore], loose).W + core.LayoutChild(r.children[crumbMoreSep], loose).W
	from, to := 0, 0
	if c.MaxW < geom.Inf {
		from, to = crumbFold(seg, sep, more, c.MaxW)
	}
	folded := to > from
	for i := range n {
		switch {
		case folded && i >= from && i < to:
			continue
		case folded && i == to:
			if from == 1 {
				x = r.place(separator(1), x, c.MaxW)
			}
			x = r.place(crumbMore, x, c.MaxW)
			x = r.place(crumbMoreSep, x, c.MaxW)
		case i > 0:
			x = r.place(separator(i), x, c.MaxW)
		}
		x = r.place(segment(i), x, c.MaxW)
	}
	r.report(from, to)
	width := x
	if c.MaxW < geom.Inf {
		width = c.MaxW
	}
	return c.Constrain(geom.Size{W: width, H: height})
}

// place shows slot i at x, laid out already, and returns where the next one
// goes. What does not fit before limit is cut at the end.
func (r *renderCrumbRow) place(i int, x, limit float32) float32 {
	s := r.slot(i)
	s.hidden = false
	w := s.Size().W
	if x+w > limit {
		w = core.LayoutChild(s, geom.Constraints{MaxW: max(limit-x, 0), MaxH: breadcrumbHeight}).W
	}
	s.SetOffset(geom.Offset{X: geom.Floor32(x + 0.5), Y: geom.Floor32((breadcrumbHeight-s.Size().H)/2 + 0.5)})
	return x + w
}

// report tells the state which segments folded, when that changed.
func (r *renderCrumbRow) report(from, to int) {
	if from != r.from || to != r.to {
		r.from, r.to = from, to
		if r.onFold != nil {
			r.onFold(from, to)
		}
	}
}

// Paint implements core.RenderObject.
func (r *renderCrumbRow) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
