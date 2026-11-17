package interact_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/theme"
)

// pressableFonts keeps the tests independent of the fonts of the machine.
var pressableFonts = gowakutest.Options{Fonts: [][]byte{goregular.TTF}}

// cursorHost is a window that remembers the cursor it was given.
type cursorHost struct{ cursor input.Cursor }

var _ core.Host = (*cursorHost)(nil)

func (*cursorHost) SetIMEEnabled(bool)         {}
func (*cursorHost) SetIMECursorRect(geom.Rect) {}
func (*cursorHost) CancelComposition()         {}
func (h *cursorHost) SetCursor(c input.Cursor) { h.cursor = c }
func (*cursorHost) ClipboardText() string      { return "" }
func (*cursorHost) SetClipboardText(string)    {}

// pressRig is a Pressable in the middle of the window that records the
// PressState of its last build.
type pressRig struct {
	tt    *gowakutest.Tester
	host  *cursorHost
	last  interact.PressState
	taps  int
	place geom.Offset
}

func newPressRig(t *testing.T, enabled bool) *pressRig {
	t.Helper()
	r := &pressRig{host: &cursorHost{}}
	p := interact.Pressable{Label: "Play", Build: func(s interact.PressState) core.Widget {
		r.last = s
		return basic.SizedBox{Width: 100, Height: 40}
	}}
	if enabled {
		p.OnPress = func() { r.taps++ }
	}
	r.tt = gowakutest.New(t, basic.Center{Child: p}, pressableFonts)
	r.tt.Tree().SetHost(r.host)
	n, ok := r.tt.Find("Play")
	if !ok {
		t.Fatalf("no node labeled Play in\n%s", r.tt.Dump())
	}
	r.place = geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2}
	return r
}

func (r *pressRig) pointer(kind core.PointerKind, pos geom.Offset) {
	r.tt.Tree().DispatchPointer(core.PointerEvent{Kind: kind, Pos: pos, Button: input.ButtonPrimary})
	r.tt.Pump()
}

func TestPressable_HoverEnterAndExit(t *testing.T) {
	r := newPressRig(t, true)
	if r.last.Hovered {
		t.Fatal("hovered before the pointer came")
	}
	r.pointer(core.PointerMove, r.place)
	if !r.last.Hovered {
		t.Error("not hovered with the pointer over it")
	}
	if r.host.cursor != input.CursorHand {
		t.Errorf("cursor %v over an enabled area, want the hand", r.host.cursor)
	}
	r.pointer(core.PointerMove, geom.Offset{X: 2, Y: 2})
	if r.last.Hovered {
		t.Error("still hovered after the pointer left")
	}
	if r.host.cursor != input.CursorArrow {
		t.Errorf("cursor %v away from the area, want the arrow", r.host.cursor)
	}
}

func TestPressable_PressedWhileDown(t *testing.T) {
	r := newPressRig(t, true)
	r.pointer(core.PointerMove, r.place)
	r.pointer(core.PointerDown, r.place)
	if !r.last.Pressed {
		t.Error("not pressed while the button is down")
	}
	if r.taps != 0 {
		t.Error("OnPress ran before the release")
	}
	r.pointer(core.PointerUp, r.place)
	if r.last.Pressed {
		t.Error("still pressed after the release")
	}
	if r.taps != 1 {
		t.Errorf("OnPress ran %d times, want 1", r.taps)
	}
}

func TestPressable_ReleaseOutsideCancels(t *testing.T) {
	r := newPressRig(t, true)
	r.pointer(core.PointerMove, r.place)
	r.pointer(core.PointerDown, r.place)
	r.pointer(core.PointerMove, geom.Offset{X: 2, Y: 2})
	if !r.last.Pressed {
		t.Error("a drag outside ended the press before the release")
	}
	r.pointer(core.PointerUp, geom.Offset{X: 2, Y: 2})
	if r.last.Pressed || r.taps != 0 {
		t.Errorf("after a release outside: pressed %v, %d taps; want false, 0", r.last.Pressed, r.taps)
	}
}

func TestPressable_TapRunsOnPress(t *testing.T) {
	r := newPressRig(t, true)
	r.tt.Tap("Play")
	r.tt.Tap("Play")
	if r.taps != 2 {
		t.Errorf("OnPress ran %d times for 2 taps", r.taps)
	}
}

func TestPressable_KeyboardActivates(t *testing.T) {
	r := newPressRig(t, true)
	r.tt.Key(input.KeyTab, 0)
	if !r.last.Focused {
		t.Fatal("Tab did not focus the area")
	}
	r.tt.Key(input.KeyEnter, 0)
	r.tt.Key(input.KeySpace, 0)
	if r.taps != 2 {
		t.Errorf("OnPress ran %d times for Enter and Space", r.taps)
	}
}

func TestPressable_NilOnPressIsDisabled(t *testing.T) {
	r := newPressRig(t, false)
	if r.last.Enabled {
		t.Error("enabled without OnPress")
	}
	r.pointer(core.PointerMove, r.place)
	if r.last.Hovered {
		t.Error("a disabled area reports hover")
	}
	if r.host.cursor == input.CursorHand {
		t.Error("the hand cursor over a disabled area")
	}
	r.pointer(core.PointerDown, r.place)
	if r.last.Pressed {
		t.Error("a disabled area reports a press")
	}
	r.pointer(core.PointerUp, r.place)
	r.tt.Key(input.KeyTab, 0)
	if r.last.Focused {
		t.Error("a disabled area took the focus")
	}
	r.tt.Key(input.KeyEnter, 0)
	if r.taps != 0 {
		t.Errorf("a disabled area ran OnPress %d times", r.taps)
	}
}

func TestPressable_Semantics(t *testing.T) {
	r := newPressRig(t, true)
	n, _ := r.tt.Find("Play")
	if n.Role != semantics.RoleButton {
		t.Errorf("role %v, want button by default", n.Role)
	}
	if n.Rect.W != 100 || n.Rect.H != 40 {
		t.Errorf("node is %gx%g, want the 100x40 content", n.Rect.W, n.Rect.H)
	}

	tt := gowakutest.New(t, interact.Pressable{
		Label: "Shuffle", Role: semantics.RoleCheckbox, Value: "checked", OnPress: func() {},
		Child: basic.SizedBox{Width: 20, Height: 20},
	}, pressableFonts)
	n, ok := tt.Find("Shuffle")
	if !ok || n.Role != semantics.RoleCheckbox || n.Value != "checked" {
		t.Errorf("want a checked checkbox named Shuffle in\n%s", tt.Dump())
	}
}

// switchable is a Pressable whose OnPress its parent turns on and off through
// enable.
type switchable struct {
	core.StatefulBase
	last   *interact.PressState
	enable *func(on bool)
	// states counts the States of the content, which must survive.
	states *int
}

type switchableState struct {
	core.StateBase
	enabled bool
}

func (switchable) CreateState() core.State { return &switchableState{enabled: true} }

func (s *switchableState) InitState() {
	*s.Widget().(switchable).enable = func(on bool) { s.SetState(func() { s.enabled = on }) }
}

func (s *switchableState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(switchable)
	p := interact.Pressable{Label: "Item", Build: func(ps interact.PressState) core.Widget {
		*w.last = ps
		return contentCounter{states: w.states}
	}}
	if s.enabled {
		p.OnPress = func() {}
	}
	return basic.Center{Child: p}
}

// contentCounter counts how many States it was given.
type contentCounter struct {
	core.StatefulBase
	states *int
}

type contentCounterState struct{ core.StateBase }

func (c contentCounter) CreateState() core.State {
	*c.states++
	return &contentCounterState{}
}

func (*contentCounterState) Build(core.BuildContext) core.Widget {
	return basic.SizedBox{Width: 100, Height: 40}
}

func TestPressable_DisableKeepsContentAndHover(t *testing.T) {
	var last interact.PressState
	var enable func(on bool)
	states := 0
	tt := gowakutest.New(t, switchable{last: &last, enable: &enable, states: &states}, pressableFonts)
	n, _ := tt.Find("Item")
	mid := geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: mid})
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: mid, Button: input.ButtonPrimary})
	tt.Pump()
	if !last.Hovered || !last.Pressed {
		t.Fatalf("want hovered and pressed, got %+v", last)
	}

	enable(false)
	tt.Pump()
	if last != (interact.PressState{}) {
		t.Errorf("disabled with %+v, want the zero PressState", last)
	}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: mid, Button: input.ButtonPrimary})
	enable(true)
	tt.Pump()
	if !last.Enabled || !last.Hovered || last.Pressed {
		t.Errorf("enabled again under the pointer: %+v, want hovered and not pressed", last)
	}
	if states != 1 {
		t.Errorf("the content got %d States, want 1", states)
	}
}

// A row of a list that lights up under the pointer and sinks while pressed.
func ExamplePressable() {
	th := theme.Default().Light
	var w core.Widget = interact.Pressable{Label: "Open", OnPress: func() {}, Build: func(s interact.PressState) core.Widget {
		var ground geom.Color
		if s.Hovered || s.Focused {
			ground = th.Color.Fill
		}
		sink := float32(0)
		if s.Pressed {
			sink = 0.03
		}
		return basic.Container{
			Color:   ground,
			Radius:  th.Radius.Control,
			Sink:    sink,
			Padding: geom.Symmetric(theme.Space12, theme.Space8),
			Child:   basic.Text{Content: "Open"},
		}
	}}
	_ = w
}

func TestPressable_ZeroValue(t *testing.T) {
	tt := gowakutest.New(t, basic.Center{Child: interact.Pressable{}}, pressableFonts)
	tt.TapAt(geom.Offset{X: 400, Y: 300})
}
