package gowaku

import (
	"math"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/theme"
)

// The title bar that gowaku draws when the OS leaves it to the application
// (platform.Window.ClientDecorated). See docs/ADR/0021-wayland-platform.md.
const (
	titleBarHeight  = 36
	titleButtonW    = 36
	titleButtonH    = 26
	titleButtonGap  = 2
	titleBarInset   = 6
	titleIconSize   = 16
	doubleClickTime = 400
	doubleClickSlop = 4
	// dragThreshold is how far the pointer must move with the button down
	// before the window moves, so that a click stays a click.
	dragThreshold    = 4
	titleButtonRound = 6
)

// The glyphs of the buttons, on a 16-pixel grid, made once. They are filled
// shapes, which the glyph atlas draws without multisampling.
var (
	iconMinimize = rectPath(4, 8, 12, 9)
	iconMaximize = outlinePath(4, 4, 12, 12)
	iconRestore  = restorePath()
	iconClose    = closePath()
)

func rectPath(x0, y0, x1, y1 float32) *paint.Path {
	p := &paint.Path{}
	addRect(p, x0, y0, x1, y1, false)
	return p
}

// addRect adds a rectangle, clockwise on the screen, or the other way round to
// cut a hole out of what is clockwise.
func addRect(p *paint.Path, x0, y0, x1, y1 float32, reverse bool) {
	p.MoveTo(x0, y0)
	if reverse {
		p.LineTo(x0, y1)
		p.LineTo(x1, y1)
		p.LineTo(x1, y0)
	} else {
		p.LineTo(x1, y0)
		p.LineTo(x1, y1)
		p.LineTo(x0, y1)
	}
	p.Close()
}

func outlinePath(x0, y0, x1, y1 float32) *paint.Path {
	p := &paint.Path{}
	addOutline(p, x0, y0, x1, y1)
	return p
}

func addOutline(p *paint.Path, x0, y0, x1, y1 float32) {
	addRect(p, x0, y0, x1, y1, false)
	addRect(p, x0+1, y0+1, x1-1, y1-1, true)
}

// restorePath is a window in front of another.
func restorePath() *paint.Path {
	p := &paint.Path{}
	addOutline(p, 4, 6, 10, 12)
	p.MoveTo(6, 4)
	p.LineTo(12, 4)
	p.LineTo(12, 10)
	p.LineTo(11, 10)
	p.LineTo(11, 5)
	p.LineTo(6, 5)
	p.Close()
	return p
}

// closePath is a cross of two bars.
func closePath() *paint.Path {
	p := &paint.Path{}
	const half = 0.65
	bar := func(ax, ay, bx, by float32) {
		dx, dy := bx-ax, by-ay
		n := float32(math.Hypot(float64(dx), float64(dy)))
		// The same side of each bar, so that both turn the same way.
		px, py := dy/n*half, -dx/n*half
		p.MoveTo(ax+px, ay+py)
		p.LineTo(bx+px, by+py)
		p.LineTo(bx-px, by-py)
		p.LineTo(ax-px, ay-py)
		p.Close()
	}
	bar(4.5, 4.5, 11.5, 11.5)
	bar(11.5, 4.5, 4.5, 11.5)
	return p
}

// frameState is what a title bar drawn by gowaku shows of its window. It is
// handed down from the root, and changes when the window does.
type frameState struct {
	window     *window
	title      string
	focused    bool
	maximized  bool
	fullscreen bool
	scale      float32
}

// TitleBar is the title bar of a window with CustomFrame: put it at the top of
// the root, with what it should hold, such as a MenuBar, a title or a search
// field. It draws the buttons that minimize, maximize and close the window on
// its right. Dragging it where nothing takes the pointer moves the window, a
// double click maximizes it, and a secondary click opens the menu of the
// window; Windows 11 offers its snap layouts over the maximize button. It is
// 36 logical pixels high, and takes no room while the window is fullscreen. A
// nil Child shows the title of the window.
//
//	basic.Column{Children: []gowaku.Widget{gowaku.TitleBar{Child: menuBar}, basic.Expanded{Child: page}}}
//
// See docs/ADR/0048-custom-title-bar.md.
type TitleBar struct {
	core.StatelessBase
	Child Widget
}

// Build implements core.StatelessWidget.
func (t TitleBar) Build(ctx BuildContext) Widget {
	frame, ok := core.DependOn[frameState](ctx)
	if !ok {
		panic("gowaku: TitleBar found no window; put it below the root of a window of Run")
	}
	if frame.fullscreen {
		return basic.SizedBox{}
	}
	return titleBar{frame: frame, theme: basic.ThemeOf(ctx), content: t.Child}
}

// titleBar is the bar at the top of a window that draws its own frame.
type titleBar struct {
	core.StatelessBase
	frame frameState
	theme *theme.Theme
	// content takes the place of the title.
	content core.Widget
}

// Build implements core.StatelessWidget.
func (t titleBar) Build(ctx core.BuildContext) core.Widget {
	words := basic.StringsOf(ctx)
	c := &t.theme.Color
	style := t.theme.Text.Control
	style.Color = c.Ink
	if !t.frame.focused {
		style.Color = c.Ink3
	}
	maximize, maximizeLabel := iconMaximize, words.Maximize
	if t.frame.maximized {
		maximize, maximizeLabel = iconRestore, words.Restore
	}
	w := t.frame.window
	native := w.native
	buttons := basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: titleButtonGap, Children: []core.Widget{
		titleButton{icon: iconMinimize, label: words.Minimize, theme: t.theme, onTap: native.Minimize},
		maxButtonArea{child: titleButton{icon: maximize, label: maximizeLabel, theme: t.theme, onTap: native.ToggleMaximize}},
		titleButton{icon: iconClose, label: words.Close, theme: t.theme, danger: true, onTap: func() { w.app.requestCloseAsking(w) }},
	}}
	var bar core.Widget
	if t.content != nil {
		bar = basic.Row{Cross: layout.CrossCenter, Children: []core.Widget{
			basic.Expanded{Child: basic.Padding{Insets: geom.Insets{Left: titleBarInset}, Child: t.content}},
			basic.Padding{Insets: geom.Insets{Left: titleButtonGap, Right: titleBarInset}, Child: buttons},
		}}
	} else {
		bar = basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
			basic.Center{Child: basic.Text{Content: t.frame.title, Style: style}},
			basic.Positioned{Right: titleBarInset, AnchorRight: true, Height: titleBarHeight - 1, Child: buttons},
		}}
	}
	return titleDrag{native: native, scale: t.frame.scale, child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStretch,
		Children: []core.Widget{
			basic.Container{Height: titleBarHeight - 1, Color: c.Canvas, Child: bar},
			basic.Container{Height: 1, Color: c.Hair},
		},
	}}
}

// maxButtonArea marks the maximize button, which Windows must know of to offer
// its snap layouts.
type maxButtonArea struct {
	child core.Widget
}

// Key implements core.Widget.
func (maxButtonArea) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (maxButtonArea) CreateRenderObject() core.RenderObject { return &renderMaxButton{} }

// UpdateRenderObject implements core.RenderWidget.
func (maxButtonArea) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (m maxButtonArea) ChildWidget() core.Widget { return m.child }

type renderMaxButton struct {
	layout.RenderProxy
}

// hitArea tells the window system what the point (x, y) of the window is, in
// physical pixels: the maximize button, a part of the title bar that moves the
// window, or the application's.
func (w *window) hitArea(x, y int) platform.HitArea {
	if !w.decorated || w.scale <= 0 {
		return platform.HitClient
	}
	pos := geom.Offset{X: float32(x) / w.scale, Y: float32(y) / w.scale}
	path := w.tree.HitTest(pos).Path
	for _, ro := range path {
		if _, ok := ro.(*renderMaxButton); ok {
			return platform.HitMaximize
		}
	}
	// The path runs from the deepest object up: what takes the pointer below
	// the bar keeps it.
	for _, ro := range path {
		switch ro.(type) {
		case *renderTitleDrag:
			return platform.HitCaption
		case core.PointerTarget:
			return platform.HitClient
		}
	}
	return platform.HitClient
}

// titleButton is a button of the title bar, which shows its ground only under
// the pointer.
type titleButton struct {
	core.StatefulBase
	icon   *paint.Path
	label  string
	theme  *theme.Theme
	danger bool
	onTap  func()
}

type titleButtonState struct {
	core.StateBase
	hovered, pressed bool
}

// CreateState implements core.StatefulWidget.
func (titleButton) CreateState() core.State { return &titleButtonState{} }

// Build implements core.State.
func (s *titleButtonState) Build(core.BuildContext) core.Widget {
	b := s.Widget().(titleButton)
	c := &b.theme.Color
	ground, ink := geom.Color{}, c.Ink2
	switch {
	case b.danger && (s.hovered || s.pressed):
		ground, ink = c.Danger, c.OnAccent
		if s.pressed {
			ground = theme.Over(c.Danger, geom.Color{A: 1}, 0.15)
		}
	case s.pressed:
		ground, ink = c.FillStrong, c.Ink
	case s.hovered:
		ground, ink = c.Fill, c.Ink
	}
	set := func(fn func()) func() { return func() { s.SetState(fn) } }
	return interact.Semantics{Role: semantics.RoleButton, Label: b.label, OnTap: b.onTap, Child: interact.MouseRegion{
		OnEnter: set(func() { s.hovered = true }),
		OnExit:  set(func() { s.hovered, s.pressed = false, false }),
		Child: interact.GestureDetector{
			OnTapDown:   set(func() { s.pressed = true }),
			OnTapCancel: set(func() { s.pressed = false }),
			OnTap: func() {
				s.SetState(func() { s.pressed = false })
				if b.onTap != nil {
					b.onTap()
				}
			},
			Child: basic.Container{
				Width: titleButtonW, Height: titleButtonH, Radius: titleButtonRound, Color: ground,
				Child: basic.Center{Child: basic.Icon{Path: b.icon, ViewBox: titleIconSize, Size: titleIconSize, Color: ink}},
			},
		},
	}}
}

// titleDrag moves the window when the bar is pressed where no button is.
type titleDrag struct {
	native platform.Window
	scale  float32
	child  core.Widget
}

// Key implements core.Widget.
func (titleDrag) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t titleDrag) CreateRenderObject() core.RenderObject {
	return &renderTitleDrag{native: t.native, scale: t.scale}
}

// UpdateRenderObject implements core.RenderWidget.
func (t titleDrag) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTitleDrag)
	r.native, r.scale = t.native, t.scale
}

// ChildWidget implements core.SingleChildWidget.
func (t titleDrag) ChildWidget() core.Widget { return t.child }

type renderTitleDrag struct {
	layout.RenderProxy
	native platform.Window
	scale  float32

	// pressed means that the primary button went down on the bar, at lastPos,
	// and has not moved the window yet.
	pressed  bool
	lastDown uint32
	lastPos  geom.Offset
	hasLast  bool
}

func within(a, b geom.Offset, d float32) bool {
	v := a.Sub(b)
	return v.X*v.X+v.Y*v.Y < d*d
}

// HandlePointer implements core.PointerTarget. The bar sits at the top left of
// the window, so its positions are those of the window. A press that moves
// moves the window, and a second press soon after the first maximizes it.
func (r *renderTitleDrag) HandlePointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		if e.Handled {
			return
		}
		switch e.Button {
		case input.ButtonPrimary:
			e.Handled = true
			if r.hasLast && e.Time-r.lastDown < doubleClickTime && within(e.Pos, r.lastPos, doubleClickSlop) {
				r.hasLast, r.pressed = false, false
				r.native.ToggleMaximize()
				return
			}
			r.hasLast, r.pressed, r.lastDown, r.lastPos = true, true, e.Time, e.Pos
		case input.ButtonSecondary:
			e.Handled = true
			r.native.ShowWindowMenu(int(e.Pos.X*r.scale), int(e.Pos.Y*r.scale))
		}
	case core.PointerMove:
		if r.pressed && !within(e.Pos, r.lastPos, dragThreshold) {
			// The compositor takes the pointer from here, and the release
			// never comes to the window.
			r.pressed, r.hasLast = false, false
			r.native.BeginMove()
		}
	case core.PointerUp, core.PointerCancel:
		r.pressed = false
	}
}
