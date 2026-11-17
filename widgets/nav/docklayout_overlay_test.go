package nav_test

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
)

// flyoutRig is a DockLayout whose files panel hides at the left. The panel
// has a context menu, one of whose items opens a dialog from the context the
// panel is built in.
type flyoutRig struct {
	ctrl *nav.DockController
	// ran are the menu items that ran, and focused whether the panel has
	// the keyboard.
	ran     *[]string
	focused *bool
}

func (r flyoutRig) Key() core.Key { return nil }

func (r flyoutRig) Build(core.BuildContext) core.Widget {
	files := func(ctx core.BuildContext) core.Widget {
		return overlay.ContextMenu{
			Items: []overlay.MenuItem{
				{Action: interact.Action{Label: "Rename", OnInvoke: func() { *r.ran = append(*r.ran, "Rename") }}},
				{Action: interact.Action{Label: "Ask", OnInvoke: func() {
					*r.ran = append(*r.ran, "Ask")
					overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
						return overlay.Dialog{Title: "Confirm", Actions: []core.Widget{
							control.Button{Label: "OK", OnPress: dismiss},
						}}
					})
				}}},
			},
			Child: interact.Focus{
				Enabled:       true,
				OnFocusChange: func(f bool) { *r.focused = f },
				Child:         basic.Container{Color: geom.Color{B: 1, A: 1}},
			},
		}
	}
	return nav.DockLayout{
		Panels: []nav.DockPanel{
			{ID: "files", Title: "files", Build: files},
			{ID: "a", Title: "a", Build: func(core.BuildContext) core.Widget {
				return interact.Focus{Enabled: true, Child: basic.SizedBox{}}
			}},
		},
		Controller: r.ctrl,
		Default: nav.DockArrangement{Root: nav.DockNode{
			Weights:  []float32{1, 3},
			Children: []nav.DockNode{{Panels: []string{"files"}}, {Panels: []string{"a"}}},
		}},
		AutoHide: true,
	}
}

// StatelessWidget is what flyoutRig is.
var _ core.StatelessWidget = flyoutRig{}

// inFlyout is a place on the flyout of files, and outside one on the group of a.
var (
	inFlyout = geom.Offset{X: 120, Y: 400}
	outside  = geom.Offset{X: 700, Y: 400}
)

func newFlyoutRig(t *testing.T) (*gowakutest.Tester, flyoutRig) {
	t.Helper()
	r := flyoutRig{ctrl: &nav.DockController{}, ran: new([]string), focused: new(bool)}
	tt := gowakutest.New(t, r, gowakutest.Options{Width: 1000, Height: 700})
	r.ctrl.Unpin("files", nav.DockLeft)
	tt.Pump()
	r.ctrl.Show("files")
	tt.Pump()
	tt.Settle()
	if r.ctrl.Flyout() != "files" || !*r.focused {
		t.Fatalf("the panel slides out with the keyboard, got %q %v", r.ctrl.Flyout(), *r.focused)
	}
	return tt, r
}

// rightClick clicks the secondary button at at.
func rightClick(tt *gowakutest.Tester, at geom.Offset) {
	tree := tt.Tree()
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonSecondary})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonSecondary})
	tt.Pump()
	tt.Settle()
}

func TestDockLayout_OverlaysFromAFlyoutKeepItOut(t *testing.T) {
	t.Run("コンテキストメニュー", func(t *testing.T) {
		tt, r := newFlyoutRig(t)
		rightClick(tt, inFlyout)
		if _, ok := tt.Find("Rename"); !ok {
			t.Fatalf("no menu in\n%s", tt.Dump())
		}
		if r.ctrl.Flyout() != "files" {
			t.Fatal("a menu opened in the flyout keeps it out")
		}
		tt.Tap("Rename")
		if !slices.Equal(*r.ran, []string{"Rename"}) {
			t.Fatalf("got %v", *r.ran)
		}
		if r.ctrl.Flyout() != "files" || !*r.focused {
			t.Errorf("choosing an item keeps it out, with the keyboard back in it, got %q %v", r.ctrl.Flyout(), *r.focused)
		}
		tt.TapAt(outside)
		if r.ctrl.Flyout() != "" {
			t.Error("a press outside still hides it")
		}
	})
	t.Run("メニューを Escape で閉じる", func(t *testing.T) {
		tt, r := newFlyoutRig(t)
		rightClick(tt, inFlyout)
		tt.Key(input.KeyEscape, 0)
		if _, ok := tt.Find("Rename"); ok {
			t.Fatal("Escape closes the menu")
		}
		if r.ctrl.Flyout() != "files" || !*r.focused {
			t.Errorf("and only the menu, got %q %v", r.ctrl.Flyout(), *r.focused)
		}
		tt.Key(input.KeyEscape, 0)
		if r.ctrl.Flyout() != "" {
			t.Error("the next Escape hides the flyout")
		}
	})
	t.Run("ダイアログ", func(t *testing.T) {
		tt, r := newFlyoutRig(t)
		rightClick(tt, inFlyout)
		tt.Tap("Ask")
		if _, ok := tt.Find("Confirm"); !ok {
			t.Fatalf("no dialog in\n%s", tt.Dump())
		}
		if r.ctrl.Flyout() != "files" {
			t.Fatal("a dialog opened from the flyout keeps it out")
		}
		tt.Tap("OK")
		if _, ok := tt.Find("Confirm"); ok {
			t.Fatal("OK closes the dialog")
		}
		if r.ctrl.Flyout() != "files" || !*r.focused {
			t.Errorf("closing it keeps the flyout out, with the keyboard back in it, got %q %v", r.ctrl.Flyout(), *r.focused)
		}
		tt.TapAt(outside)
		if r.ctrl.Flyout() != "" {
			t.Error("a press outside still hides it")
		}
	})
	t.Run("メニューの横を押す", func(t *testing.T) {
		tt, r := newFlyoutRig(t)
		rightClick(tt, inFlyout)
		tt.TapAt(outside)
		if _, ok := tt.Find("Rename"); ok {
			t.Fatal("a press beside the menu closes it")
		}
		if r.ctrl.Flyout() != "files" || !*r.focused {
			t.Errorf("and only the menu, which takes the press, got %q %v", r.ctrl.Flyout(), *r.focused)
		}
	})
	t.Run("メニューから外へフォーカスが移る", func(t *testing.T) {
		tt, r := newFlyoutRig(t)
		rightClick(tt, inFlyout)
		// The focus goes from the menu straight to the panel a, as a command
		// of the app that shows it would move it.
		tt.Tree().RequestFocus(focusAt(t, tt.Tree().RootRenderObject(), outside))
		tt.Pump()
		tt.Settle()
		if r.ctrl.Flyout() != "" {
			t.Error("the focus leaving the menu for another panel hides the flyout")
		}
		tt, r = newFlyoutRig(t)
		rightClick(tt, inFlyout)
		tt.Tree().RequestFocus(nil)
		tt.Pump()
		if r.ctrl.Flyout() != "" {
			t.Error("so does the focus leaving it for nowhere")
		}
	})
}

// focusAt returns an object below ro that takes the focus and lies at at.
func focusAt(t *testing.T, ro core.RenderObject, at geom.Offset) core.FocusTarget {
	t.Helper()
	var walk func(ro core.RenderObject) core.FocusTarget
	walk = func(ro core.RenderObject) core.FocusTarget {
		if f, ok := ro.(core.FocusTarget); ok {
			o, sz := core.LocalToGlobal(ro, geom.Offset{}), ro.Size()
			if (geom.Rect{X: o.X, Y: o.Y, W: sz.W, H: sz.H}).Contains(at) {
				return f
			}
		}
		for i := range ro.ChildCount() {
			if f := walk(ro.ChildAt(i)); f != nil {
				return f
			}
		}
		return nil
	}
	f := walk(ro)
	if f == nil {
		t.Fatalf("nothing takes the focus at %v", at)
	}
	return f
}
