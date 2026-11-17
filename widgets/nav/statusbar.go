package nav

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

const (
	// statusBarHeight is the height of a status bar under its line: a line of
	// Caption with room of 3 above and below.
	statusBarHeight = 24
	// statusItemHeight is the height of the ground of a pressable item, and
	// statusItemPadding the room at its ends.
	statusItemHeight  = 20
	statusItemPadding = theme.Space6
	// statusSeparatorRoom is the room on each side of the line between two
	// items, and statusSeparatorLength the length of the line.
	statusSeparatorRoom   = theme.Space8
	statusSeparatorLength = 12
	// statusGroupGap is the least room between the two groups.
	statusGroupGap = theme.Space16
)

// StatusItem is one entry of a StatusBar: text, a small widget of your own,
// or either of them as a button.
//
//	StatusItem{Text: "行 12, 列 4"}
//	StatusItem{Text: "UTF-8", Tooltip: "文字コードを選ぶ", OnPress: chooseEncoding}
//	StatusItem{Child: syncIcon, Label: "同期済み", Priority: 2}
type StatusItem struct {
	// Text is what the item shows, in the supporting color of Caption.
	Text string
	// Child replaces Text with a widget of your own, such as an icon or a
	// small progress ring. Keep it within 20 high.
	Child core.Widget
	// Label names the item to assistive technology. Empty selects Text.
	Label string
	// Tooltip explains the item when the pointer rests on it.
	Tooltip string
	// OnPress makes the item a button: it takes a ground under the pointer,
	// a stop of Tab and Enter.
	OnPress func()
	// Priority decides which items stay when the bar is too narrow for all of
	// them: those of the lowest priority go first, and of equal priority the
	// one that comes last, counting the Leading items before the Trailing
	// ones. The zero value is the lowest priority but for negative ones.
	Priority int
}

// StatusBar is the band along the bottom edge of a window that tells the
// state of what the window shows: the place of the caret, the encoding, the
// state of a connection. It is an ordinary widget, not part of the window
// frame; put it at the bottom of the page:
//
//	basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
//		basic.Expanded{Child: page},
//		nav.StatusBar{Leading: left, Trailing: right},
//	}}
//
// Leading items line up from the start of the bar and Trailing items from its
// end, with a thin line between neighbors of a group. The bar is 25 high with
// its line on top, has room of 8 at its ends like a Toolbar, and takes the whole
// width it is given. When the items do not fit, those of the lowest Priority
// are hidden whole, one at a time, until the rest fit; hidden items leave the
// order of Tab and assistive technology too. When even the item of the highest
// priority does not fit alone, it is cut off at the end of the bar.
//
// The bar does not mirror for right-to-left languages, which no widget of
// gowaku does yet; right-to-left text inside items is set as usual.
type StatusBar struct {
	core.StatelessBase
	// Label names the bar to assistive technology. Empty selects the word for
	// Status bar.
	Label             string
	Leading, Trailing []StatusItem
}

// Build implements core.StatelessWidget.
func (b StatusBar) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	children := make([]core.Widget, 0, len(b.Leading)+len(b.Trailing))
	for i := range b.Leading {
		children = append(children, statusSlot{priority: b.Leading[i].Priority, child: statusItem(th, &b.Leading[i])})
	}
	for i := range b.Trailing {
		children = append(children, statusSlot{priority: b.Trailing[i].Priority, trailing: true, child: statusItem(th, &b.Trailing[i])})
	}
	label := b.Label
	if label == "" {
		label = basic.StringsOf(ctx).StatusBar
	}
	return interact.Semantics{Role: semantics.RoleGroup, Label: label, Child: basic.Container{
		Color: th.Color.Sidebar,
		Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{
			basic.Divider{},
			basic.Padding{
				Insets: geom.Symmetric(theme.Space8, 0),
				Child:  statusRow{line: th.Color.Hair, children: children},
			},
		}},
	}}
}

// statusItem returns the widget of one item.
func statusItem(th *theme.Theme, it *StatusItem) core.Widget {
	c := &th.Color
	label := it.Label
	if label == "" {
		label = it.Text
	}
	content := func(ink geom.Color) core.Widget {
		if it.Child != nil {
			return it.Child
		}
		return basic.Text{Content: it.Text, Style: kit.Colored(th.Text.Caption, ink)}
	}
	var w core.Widget
	switch {
	case it.OnPress != nil:
		press := it.OnPress
		w = interact.Pressable{
			Label:   label,
			OnPress: press,
			Build: func(p interact.PressState) core.Widget {
				var ground geom.Color
				ink := c.Ink2
				switch {
				case p.Pressed:
					ground, ink = c.FillStrong, c.Ink
				case p.Hovered:
					ground, ink = c.Fill, c.Ink
				}
				return interact.FocusRing{Visible: p.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
					Height: statusItemHeight, Radius: th.Radius.Control, Color: ground,
					Padding: geom.Symmetric(statusItemPadding, 0),
					Child:   basic.Center{Child: content(ink)},
				}}
			},
		}
	case it.Child != nil && it.Label != "":
		w = interact.Semantics{Role: semantics.RoleText, Label: it.Label, Child: it.Child}
	default:
		w = content(c.Ink2)
	}
	if it.Tooltip != "" {
		w = overlay.Tooltip{Message: it.Tooltip, Child: w}
	}
	return w
}

// statusSlot is the widget of renderStatusSlot.
type statusSlot struct {
	priority int
	trailing bool
	child    core.Widget
}

// Key implements core.Widget.
func (statusSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t statusSlot) CreateRenderObject() core.RenderObject {
	return &renderStatusSlot{priority: t.priority, trailing: t.trailing}
}

// UpdateRenderObject implements core.RenderWidget.
func (t statusSlot) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderStatusSlot)
	if r.priority != t.priority || r.trailing != t.trailing {
		r.priority, r.trailing = t.priority, t.trailing
		if p := r.Parent(); p != nil {
			p.MarkNeedsLayout()
		}
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t statusSlot) ChildWidget() core.Widget { return t.child }

// renderStatusSlot holds one item of a status bar, which the row hides when it
// does not fit: the pointer, Tab and assistive technology then pass it by.
type renderStatusSlot struct {
	layout.RenderProxy
	priority int
	trailing bool
	hidden   bool
}

// Hidden implements core.Hider.
func (r *renderStatusSlot) Hidden() bool { return r.hidden }

// Paint implements core.RenderObject.
func (r *renderStatusSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	if !r.hidden {
		r.RenderProxy.Paint(cv, off)
	}
}

// statusRow is the widget of renderStatusRow.
type statusRow struct {
	line     geom.Color
	children []core.Widget
}

// Key implements core.Widget.
func (statusRow) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t statusRow) CreateRenderObject() core.RenderObject { return &renderStatusRow{line: t.line} }

// UpdateRenderObject implements core.RenderWidget.
func (t statusRow) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderStatusRow)
	if r.line != t.line {
		r.line = t.line
		r.MarkNeedsPaint()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (t statusRow) ChildWidgets() []core.Widget { return t.children }

// renderStatusRow lays out the slots of a status bar in a row 24 high: the
// leading ones from the start, the trailing ones from the end. It hides the
// slots of the lowest priority until the rest fit, and draws the lines
// between neighbors of a group.
type renderStatusRow struct {
	core.RenderBase
	line     geom.Color
	children []core.RenderObject
	widths   []float32
	// clipped says the items left overflow the row, which cuts them off.
	clipped bool
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderStatusRow) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderStatusRow) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderStatusRow) ChildAt(i int) core.RenderObject { return r.children[i] }

func (r *renderStatusRow) slot(i int) *renderStatusSlot { return r.children[i].(*renderStatusSlot) }

// need returns the width the shown slots take, with their lines and the gap
// between the groups.
func (r *renderStatusRow) need() float32 {
	var total float32
	var lead, trail int
	for i := range r.children {
		s := r.slot(i)
		if s.hidden {
			continue
		}
		n := &lead
		if s.trailing {
			n = &trail
		}
		if *n > 0 {
			total += 2*statusSeparatorRoom + 1
		}
		*n++
		total += r.widths[i]
	}
	if lead > 0 && trail > 0 {
		total += statusGroupGap
	}
	return total
}

// hideOne hides the shown slot of the lowest priority, the last of equal
// ones, unless it is the only one left, and reports whether it did.
func (r *renderStatusRow) hideOne() bool {
	pick, shown := -1, 0
	for i := range r.children {
		s := r.slot(i)
		if s.hidden {
			continue
		}
		shown++
		if pick < 0 || s.priority <= r.slot(pick).priority {
			pick = i
		}
	}
	if shown <= 1 {
		return false
	}
	r.slot(pick).hidden = true
	return true
}

// Layout implements core.RenderObject.
func (r *renderStatusRow) Layout(c geom.Constraints) geom.Size {
	const height = statusBarHeight
	n := len(r.children)
	if cap(r.widths) < n {
		r.widths = make([]float32, n)
	}
	r.widths = r.widths[:n]
	loose := geom.Constraints{MaxW: geom.Inf, MaxH: height}
	for i := range n {
		r.slot(i).hidden = false
		r.widths[i] = core.LayoutChild(r.children[i], loose).W
	}
	limit := c.MaxW
	for limit < geom.Inf && r.need() > limit {
		if !r.hideOne() {
			break
		}
	}
	total := r.need()
	r.clipped = limit < geom.Inf && total > limit
	width := total
	if limit < geom.Inf {
		width = limit
	}

	// The leading slots go from the start, the trailing ones from the end.
	var x float32
	for i := range n {
		s := r.slot(i)
		if s.hidden || s.trailing {
			continue
		}
		if x > 0 {
			x += 2*statusSeparatorRoom + 1
		}
		s.SetOffset(geom.Offset{X: geom.Floor32(x + 0.5), Y: geom.Floor32((height-s.Size().H)/2 + 0.5)})
		x += r.widths[i]
	}
	x = width
	first := true
	for i := n - 1; i >= 0; i-- {
		s := r.slot(i)
		if s.hidden || !s.trailing {
			continue
		}
		if !first {
			x -= 2*statusSeparatorRoom + 1
		}
		first = false
		x -= r.widths[i]
		s.SetOffset(geom.Offset{X: geom.Floor32(x + 0.5), Y: geom.Floor32((height-s.Size().H)/2 + 0.5)})
	}
	return c.Constrain(geom.Size{W: width, H: height})
}

// ChildClip implements core.Clipper.
func (r *renderStatusRow) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), r.clipped
}

// Paint implements core.RenderObject.
func (r *renderStatusRow) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if r.clipped {
		cv.Save()
		cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	}
	lineTop := off.Y + (sz.H-statusSeparatorLength)/2
	// prev is the shown slot before i in its group, for the line between them.
	prevLead, prevTrail := -1, -1
	for i, child := range r.children {
		s := r.slot(i)
		if s.hidden {
			continue
		}
		prev := &prevLead
		if s.trailing {
			prev = &prevTrail
		}
		if *prev >= 0 {
			// Both groups are listed from the start, so the slot before is
			// on the left in either.
			a := r.children[*prev]
			x := (a.Offset().X + a.Size().W + child.Offset().X) / 2
			cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X + geom.Floor32(x), Y: lineTop, W: 1, H: statusSeparatorLength}}, paint.Paint{Color: r.line})
		}
		*prev = i
		child.Paint(cv, off.Add(child.Offset()))
	}
	if r.clipped {
		cv.Restore()
	}
}
