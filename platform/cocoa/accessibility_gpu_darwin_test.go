//go:build gpu && darwin && arm64

package cocoa

import (
	"math"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

// a11yTree is a window with a button, a slider and a field.
func a11yTree() *semantics.Node {
	return &semantics.Node{Rect: geom.Rect{W: 200, H: 100}, Children: []*semantics.Node{
		{ID: 1, Role: semantics.RoleButton, Label: "Save", Hint: "Writes the file", Actions: semantics.ActionTap,
			State: semantics.Focusable, Rect: geom.Rect{X: 10, Y: 10, W: 80, H: 20}},
		{ID: 2, Role: semantics.RoleSlider, Label: "Volume", Actions: semantics.ActionSetValue | semantics.ActionIncrease,
			Range: semantics.Range{Max: 10, Value: 3}, Rect: geom.Rect{X: 10, Y: 40, W: 180, H: 20}},
		{ID: 3, Role: semantics.RoleTextField, Label: "Name", Actions: semantics.ActionSetValue | semantics.ActionSetSelection,
			Rect: geom.Rect{X: 10, Y: 70, W: 180, H: 20}, Text: &semantics.TextInfo{Text: "Ada", SelectionStart: 3, SelectionEnd: 3}},
	}}
}

func TestAccessibility_TreeAndActions(t *testing.T) {
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "a11y", Width: 200, Height: 100})
		if err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		w := pw.(*Window)
		defer w.Close()
		a := w.Accessibility()
		if a == nil {
			t.Errorf("no Accessibility")
			return
		}
		if a != w.Accessibility() {
			t.Error("Accessibility changes between calls")
		}
		// Drain what the window queued on opening, and what a real reader
		// running on this Mac may have asked.
		for p.WaitEvent(0).Kind != platform.EventNone {
		}

		p.libs.pool(func() {
			// The first question starts the tree.
			if got := count(w.view.Send(sel("accessibilityChildren"))); got != 0 {
				t.Errorf("children before Update: %d", got)
			}
		})
		if !a.Listening() {
			t.Errorf("not listening after a question")
			return
		}
		if !w.Accessibility().Listening() {
			t.Error("the bridge was made anew")
		}
		root := a11yTree()
		var d semantics.Diff
		semantics.Compare(nil, root, 0, 1, &d)
		a.Update(root, 1, &d)

		p.libs.pool(func() {
			children := w.view.Send(sel("accessibilityChildren"))
			if count(children) != 3 {
				t.Errorf("children: %d", count(children))
				return
			}
			button, slider, field := at(children, 0), at(children, 1), at(children, 2)
			if again := at(w.view.Send(sel("accessibilityChildren")), 0); again != button {
				t.Error("the element of a node changed between questions")
			}
			str := func(el objc.ID, name string) string { return goString(el.Send(sel(name))) }
			if got := str(button, "accessibilityRole"); got != "AXButton" {
				t.Errorf("role %q", got)
			}
			if got := str(button, "accessibilityLabel"); got != "Save" {
				t.Errorf("label %q", got)
			}
			if got := str(button, "accessibilityHelp"); got != "Writes the file" {
				t.Errorf("help %q", got)
			}
			if got := str(button, "accessibilityRoleDescription"); got == "" {
				t.Error("no role description")
			}
			if !objc.Send[bool](button, sel("isAccessibilityFocused")) || objc.Send[bool](slider, sel("isAccessibilityFocused")) {
				t.Error("focus")
			}
			if parent := button.Send(sel("accessibilityParent")); parent != w.view {
				t.Errorf("parent %#x, want the view %#x", parent, w.view)
			}
			if focused := w.view.Send(sel("accessibilityFocusedUIElement")); focused != button {
				t.Errorf("focused element %#x, want %#x", focused, button)
			}

			// The frame is in the coordinates of the screen, y up.
			frame := objc.Send[NSRect](button, sel("accessibilityFrame"))
			want := p.caretRectOf(w, geom.Rect{X: 10, Y: 10, W: 80, H: 20})
			if math.Abs(frame.Origin.X-want.Origin.X) > 0.5 || math.Abs(frame.Origin.Y-want.Origin.Y) > 0.5 ||
				frame.Size.Width != 80 || frame.Size.Height != 20 {
				t.Errorf("frame %+v, want %+v", frame, want)
			}
			// Hit testing the middle of the button finds it.
			center := NSPoint{frame.Origin.X + 40, frame.Origin.Y + 10}
			if hit := w.view.Send(sel("accessibilityHitTest:"), center); hit != button {
				t.Errorf("hit test %#x, want %#x", hit, button)
			}

			if v := objc.Send[float64](slider.Send(sel("accessibilityValue")), sel("doubleValue")); v != 3 {
				t.Errorf("slider value %v", v)
			}
			if v := objc.Send[float64](slider.Send(sel("accessibilityMaxValue")), sel("doubleValue")); v != 10 {
				t.Errorf("slider max %v", v)
			}
			if got := str(field, "accessibilityValue"); got != "Ada" {
				t.Errorf("field value %q", got)
			}
			if r := objc.Send[NSRange](field, sel("accessibilitySelectedTextRange")); r != (NSRange{3, 0}) {
				t.Errorf("selected range %+v", r)
			}
			allowed := func(el objc.ID, name string) bool {
				return objc.Send[bool](el, sel("isAccessibilitySelectorAllowed:"), sel(name))
			}
			if !allowed(button, "accessibilityPerformPress") || allowed(slider, "accessibilityPerformPress") {
				t.Error("press is offered where there is no tap")
			}
			if allowed(button, "accessibilityNumberOfCharacters") || !allowed(field, "accessibilityNumberOfCharacters") {
				t.Error("text is offered where there is none")
			}

			// What VoiceOver does comes back as actions.
			if !objc.Send[bool](button, sel("accessibilityPerformPress")) {
				t.Error("press refused")
			}
			slider.Send(sel("setAccessibilityValue:"), nsNumber(7))
			field.Send(sel("setAccessibilityValue:"), nsString("Grace"))
			field.Send(sel("setAccessibilitySelectedTextRange:"), NSRange{1, 2})
		})
		wantActions := []platform.Event{
			{Node: 1, Action: semantics.ActionTap},
			{Node: 2, Action: semantics.ActionSetValue, Number: 7},
			{Node: 3, Action: semantics.ActionSetValue, Text: "Grace"},
			{Node: 3, Action: semantics.ActionSetSelection, SelectionStart: 1, SelectionEnd: 3},
		}
		redraw := false
		for _, want := range wantActions {
			var ev platform.Event
			for {
				ev = p.WaitEvent(0)
				if ev.Kind == platform.EventRedraw {
					redraw = true
					continue
				}
				break
			}
			want.Kind, want.Window = platform.EventAccessibilityAction, w.ID()
			if ev != want {
				t.Errorf("got %+v, want %+v", ev, want)
			}
		}
		if !redraw {
			t.Error("the first question queued no redraw")
		}

		// A node that goes away lets go of its element; the others stay.
		var button objc.ID
		p.libs.pool(func() { button = at(w.view.Send(sel("accessibilityChildren")), 0) })
		next := a11yTree()
		next.Children = next.Children[:2]
		next.Children[1].Range.Value = 7
		semantics.Compare(root, next, 1, 2, &d)
		a.Update(next, 2, &d)
		p.libs.pool(func() {
			children := w.view.Send(sel("accessibilityChildren"))
			if count(children) != 2 || at(children, 0) != button {
				t.Errorf("children after removal: %d", count(children))
			}
			if !objc.Send[bool](at(children, 1), sel("isAccessibilityFocused")) {
				t.Error("focus did not move")
			}
		})
	})
}

// caretRectOf converts a box of the window to the screen as caretRect does,
// to check accessibilityFrame against.
func (p *Platform) caretRectOf(w *Window, r geom.Rect) NSRect {
	local := NSRect{Origin: NSPoint{float64(r.X), float64(r.Y)}, Size: NSSize{float64(r.W), float64(r.H)}}
	inWindow := objc.Send[NSRect](w.view, sel("convertRect:toView:"), local, objc.ID(0))
	return objc.Send[NSRect](w.window, sel("convertRectToScreen:"), inWindow)
}
