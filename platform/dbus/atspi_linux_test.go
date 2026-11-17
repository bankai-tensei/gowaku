package dbus

import (
	"errors"
	"strings"
	"testing"

	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

// newTestA11y returns a bridge that is not on any bus, for calling the
// methods of its objects directly.
func newTestA11y() *A11y {
	return &A11y{wake: func() {}, appName: "test", locale: "C", done: make(chan struct{}), busName: ":1.0"}
}

// callObject calls a method of the object at path, as godbus would after decoding.
func callObject(t *testing.T, a *A11y, path, iface, member string, args ...any) ([]any, error) {
	t.Helper()
	o := a.resolve(path)
	i, ok := o.LookupInterface(iface)
	if !ok {
		t.Fatalf("%s has no interface %s", path, iface)
	}
	m, ok := i.LookupMethod(member)
	if !ok {
		t.Fatalf("%s has no method %s.%s", path, iface, member)
	}
	return m.Call(args...)
}

func mustCall(t *testing.T, a *A11y, path, iface, member string, args ...any) []any {
	t.Helper()
	out, err := callObject(t, a, path, iface, member, args...)
	if err != nil {
		t.Fatalf("%s.%s: %v", iface, member, err)
	}
	return out
}

func TestA11y_Objects(t *testing.T) {
	a := newTestA11y()
	w := a.NewWindow(7, "Notes", func() WindowGeometry { return WindowGeometry{X: 100, Y: 50, Width: 600, Height: 400, Scale: 2} })
	root := &semantics.Node{Rect: geom.Rect{W: 300, H: 200}, Children: []*semantics.Node{
		{ID: 1, Role: semantics.RoleButton, Label: "Save", Actions: semantics.ActionTap,
			State: semantics.Focusable, Rect: geom.Rect{X: 10, Y: 10, W: 80, H: 30}},
		{ID: 2, Role: semantics.RoleSlider, Label: "Volume", Range: semantics.Range{Max: 100, Value: 30},
			Actions: semantics.ActionSetValue},
	}}
	w.Update(root, 1, &semantics.Diff{})

	const acc = "org.a11y.atspi.Accessible"
	frame := w.path(0)
	if got := mustCall(t, a, rootPath, acc, "GetChildren")[0].([]ref); len(got) != 1 || string(got[0].Path) != frame {
		t.Errorf("root children %v, want the frame", got)
	}
	if role := mustCall(t, a, frame, acc, "GetRole")[0]; role != uint32(atspiRoleFrame) {
		t.Errorf("frame role %v", role)
	}
	name, err := a.resolve(frame).property(acc, "Name")
	if err != nil || name != "Notes" {
		t.Errorf("frame name %v %v", name, err)
	}
	button := w.path(1)
	if p := getProp(t, a, button, acc, "Parent"); p.(ref).Path != godbus.ObjectPath(frame) {
		t.Errorf("parent %v", p)
	}
	st := mustCall(t, a, button, acc, "GetState")[0].([]uint32)
	if s := (stateSet{st[0], st[1]}); !s.has(atspiStateFocused) || !s.has(atspiStateFocusable) {
		t.Errorf("button states %x", st)
	}
	e := mustCall(t, a, button, "org.a11y.atspi.Component", "GetExtents", uint32(atspiCoordScreen))[0].(extents)
	if e != (extents{120, 70, 160, 60}) {
		t.Errorf("extents %v", e)
	}
	fe := mustCall(t, a, frame, "org.a11y.atspi.Component", "GetExtents", uint32(atspiCoordScreen))[0].(extents)
	if fe != (extents{100, 50, 600, 400}) {
		t.Errorf("frame extents %v", fe)
	}
	hit := mustCall(t, a, frame, "org.a11y.atspi.Component", "GetAccessibleAtPoint", int32(130), int32(80), uint32(atspiCoordScreen))[0].(ref)
	if string(hit.Path) != button {
		t.Errorf("hit %v, want the button", hit)
	}

	if ok := mustCall(t, a, button, "org.a11y.atspi.Action", "DoAction", int32(0))[0]; ok != true {
		t.Error("DoAction failed")
	}
	slider := w.path(2)
	if err := a.resolve(slider).setProperty("org.a11y.atspi.Value", "CurrentValue", godbus.MakeVariant(55.0)); err != nil {
		t.Fatal(err)
	}
	got := a.TakeActions(nil)
	want := []platform.Event{
		{Kind: platform.EventAccessibilityAction, Window: 7, Node: 1, Action: semantics.ActionTap},
		{Kind: platform.EventAccessibilityAction, Window: 7, Node: 2, Action: semantics.ActionSetValue, Number: 55},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("actions %+v, want %+v", got, want)
	}
	if len(a.TakeActions(nil)) != 0 {
		t.Error("actions must be taken once")
	}

	// A node that leaves stops answering.
	w.Update(&semantics.Node{Children: root.Children[1:]}, 0, &semantics.Diff{})
	_, err = callObject(t, a, button, acc, "GetRole")
	var de *godbus.Error
	if !errors.As(err, &de) || de.Name != errUnknownObject {
		t.Errorf("removed node: %v, want UnknownObject", err)
	}
	w.Destroy()
	if _, err := callObject(t, a, slider, acc, "GetRole"); err == nil {
		t.Error("the node of a destroyed window still answers")
	}
	if got := mustCall(t, a, rootPath, acc, "GetChildren")[0].([]ref); len(got) != 0 {
		t.Errorf("root children after destroy %v", got)
	}
}

func TestA11y_Text(t *testing.T) {
	a := newTestA11y()
	w := a.NewWindow(1, "", nil)
	// "こんにちは world" is 11 characters and 11 UTF-16 units; 😀 makes the
	// two differ after it.
	field := &semantics.Node{ID: 5, Role: semantics.RoleTextField, Label: "Note",
		Actions: semantics.ActionSetValue | semantics.ActionSetSelection,
		Text:    &semantics.TextInfo{Text: "😀こんにちは world", SelectionStart: 2, SelectionEnd: 2}}
	w.Update(&semantics.Node{Children: []*semantics.Node{field}}, 5, &semantics.Diff{})
	path := w.path(5)
	const text = "org.a11y.atspi.Text"
	if n := getProp(t, a, path, text, "CharacterCount"); n != int32(12) {
		t.Errorf("CharacterCount %v", n)
	}
	if c := getProp(t, a, path, text, "CaretOffset"); c != int32(1) {
		t.Errorf("CaretOffset %v", c)
	}
	out := mustCall(t, a, path, text, "GetStringAtOffset", int32(8), uint32(atspiGranularityWord))
	if out[0] != "world" || out[1] != int32(7) || out[2] != int32(12) {
		t.Errorf("word %v", out)
	}
	mustCall(t, a, path, text, "SetSelection", int32(0), int32(1), int32(6))
	mustCall(t, a, path, "org.a11y.atspi.EditableText", "SetTextContents", "new")
	got := a.TakeActions(nil)
	if len(got) != 2 || got[0].SelectionStart != 2 || got[0].SelectionEnd != 7 || got[0].Action != semantics.ActionSetSelection ||
		got[1].Text != "new" || got[1].Action != semantics.ActionSetValue {
		t.Errorf("actions %+v", got)
	}
}

func TestA11y_Introspect(t *testing.T) {
	a := newTestA11y()
	out := mustCall(t, a, rootPath, introspectIface, "Introspect")
	s := out[0].(string)
	for _, want := range []string{"org.a11y.atspi.Accessible", "org.a11y.atspi.Application", propertiesIface} {
		if !strings.Contains(s, want) {
			t.Errorf("introspection lacks %s", want)
		}
	}
	if strings.Contains(s, "org.a11y.atspi.Text") {
		t.Error("the root has no Text")
	}
	all := mustCall(t, a, rootPath, propertiesIface, "GetAll", "org.a11y.atspi.Application")[0].(map[string]godbus.Variant)
	if all["ToolkitName"].Value() != "gowaku" || all["AtspiVersion"].Value() != "2.1" {
		t.Errorf("application properties %v", all)
	}
	if err := a.resolve(rootPath).setProperty("org.a11y.atspi.Application", "Id", godbus.MakeVariant(int32(9))); err != nil {
		t.Fatal(err)
	}
	if id := getProp(t, a, rootPath, "org.a11y.atspi.Application", "Id"); id != int32(9) {
		t.Errorf("Id %v", id)
	}
}

// getProp returns a property of the object at path.
func getProp(t *testing.T, a *A11y, path, iface, name string) any {
	t.Helper()
	v, err := a.resolve(path).property(iface, name)
	if err != nil {
		t.Fatalf("%s.%s: %v", iface, name, err)
	}
	return v
}
