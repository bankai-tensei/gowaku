//go:build gpu

package dbus

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

// TestATSPI_EndToEnd talks to the bridge as a screen reader does, over the
// accessibility bus of at-spi2-core. It needs a session bus with
// at-spi-bus-launcher on it; without one it skips. It starts the bridge with
// org.a11y.Status off, then turns IsEnabled on, as Orca does, so that the
// bridge connects through its watch.
func TestATSPI_EndToEnd(t *testing.T) {
	s := session()
	if s == nil {
		t.Skip("no session bus")
	}
	status := s.Object(a11yBusName, a11yBusPath)
	var address string
	if err := status.Call(a11yBusName+".GetAddress", 0).Store(&address); err != nil {
		t.Skipf("no accessibility bus: %v", err)
	}
	setStatus := func(name string, on bool) {
		t.Helper()
		if err := status.SetProperty(a11yStatus+"."+name, godbus.MakeVariant(on)); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	setStatus("IsEnabled", false)
	setStatus("ScreenReaderEnabled", false)
	woke := make(chan struct{}, 64)
	a := NewA11y(func() {
		select {
		case woke <- struct{}{}:
		default:
		}
	})
	defer a.Close()
	// A window from before the connection becomes a child of the application.
	w := a.NewWindow(1, "E2E", func() WindowGeometry { return WindowGeometry{Width: 400, Height: 300, Scale: 1} })
	select {
	case <-woke:
		t.Fatal("the bridge woke the loop while assistive technology was off")
	case <-time.After(300 * time.Millisecond):
	}
	if w.Listening() {
		t.Fatal("Listening() = true while assistive technology is off")
	}
	setStatus("IsEnabled", true)
	wait := func(what string, done func() bool) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for !done() {
			select {
			case <-woke:
			case <-time.After(50 * time.Millisecond):
			case <-deadline:
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	wait("the bridge to connect", a.TakeStarted)
	if !w.Listening() {
		t.Fatal("Listening() = false after connecting")
	}

	button := &semantics.Node{ID: 1, Role: semantics.RoleButton, Label: "Save", Actions: semantics.ActionTap,
		State: semantics.Focusable, Rect: geom.Rect{X: 10, Y: 10, W: 80, H: 30}}
	check := &semantics.Node{ID: 2, Role: semantics.RoleCheckbox, Label: "Remember", State: semantics.Checked | semantics.Focusable,
		Actions: semantics.ActionTap, Rect: geom.Rect{X: 10, Y: 50, W: 80, H: 30}}
	slider := &semantics.Node{ID: 3, Role: semantics.RoleSlider, Label: "Volume", Actions: semantics.ActionSetValue,
		Range: semantics.Range{Max: 100, Value: 40}, Rect: geom.Rect{X: 10, Y: 90, W: 200, H: 30}}
	field := &semantics.Node{ID: 4, Role: semantics.RoleTextField, Label: "Greeting",
		Actions: semantics.ActionSetValue | semantics.ActionSetSelection, Rect: geom.Rect{X: 10, Y: 130, W: 200, H: 30},
		Text: &semantics.TextInfo{Text: "こんにちは world", LineRects: []geom.Rect{{X: 12, Y: 132, W: 190, H: 26}}}}
	emoji := &semantics.Node{ID: 5, Role: semantics.RoleTextField, Label: "Emoji",
		Actions: semantics.ActionSetValue | semantics.ActionSetSelection, Rect: geom.Rect{X: 10, Y: 170, W: 200, H: 30},
		Text: &semantics.TextInfo{Text: "😀 hi"}}
	root := &semantics.Node{Rect: geom.Rect{W: 400, H: 300}, Children: []*semantics.Node{button, check, slider, field, emoji}}
	var d semantics.Diff
	semantics.Compare(nil, root, 0, 1, &d)
	w.Update(root, 1, &d)

	client, err := godbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = client.Close() //nolint:errcheck // the test is over
	}()
	const (
		acc    = "org.a11y.atspi.Accessible."
		action = "org.a11y.atspi.Action."
		text   = "org.a11y.atspi.Text."
	)
	get := func(r ref, method string, out any, args ...any) {
		t.Helper()
		if err := client.Object(r.Name, r.Path).Call(method, 0, args...).Store(out); err != nil {
			t.Fatalf("%s on %s: %v", method, r.Path, err)
		}
	}
	prop := func(r ref, name string) any {
		t.Helper()
		v, err := client.Object(r.Name, r.Path).GetProperty(name)
		if err != nil {
			t.Fatalf("%s of %s: %v", name, r.Path, err)
		}
		return v.Value()
	}

	// The registry lists the application.
	var apps []ref
	get(ref{registryName, rootPath}, acc+"GetChildren", &apps)
	a.mu.Lock()
	ours := a.busName
	a.mu.Unlock()
	var app ref
	for _, r := range apps {
		if r.Name == ours {
			app = r
		}
	}
	if app.Name == "" {
		t.Fatalf("the registry lists %v, not %s", apps, ours)
	}
	if tk := prop(app, "org.a11y.atspi.Application.ToolkitName"); tk != "gowaku" {
		t.Errorf("ToolkitName %v", tk)
	}
	var frames []ref
	get(app, acc+"GetChildren", &frames)
	if len(frames) != 1 || prop(frames[0], acc+"Name") != "E2E" {
		t.Fatalf("frames %v", frames)
	}
	var children []ref
	get(frames[0], acc+"GetChildren", &children)
	find := func(role uint32, name string) ref {
		t.Helper()
		for _, c := range children {
			var r uint32
			get(c, acc+"GetRole", &r)
			if r == role && prop(c, acc+"Name") == name {
				return c
			}
		}
		t.Fatalf("no %s %q among %v", atspiRoleName(role), name, children)
		return ref{}
	}
	takeAction := func() platform.Event {
		t.Helper()
		var got []platform.Event
		wait("an action", func() bool {
			got = a.TakeActions(got)
			return len(got) > 0
		})
		return got[0]
	}
	states := func(r ref) stateSet {
		var st []uint32
		get(r, acc+"GetState", &st)
		return stateSet{st[0], st[1]}
	}

	save := find(atspiRolePushButton, "Save")
	var roleName string
	get(save, acc+"GetRoleName", &roleName)
	if roleName != "push button" {
		t.Errorf("role name %q", roleName)
	}
	if st := states(save); !st.has(atspiStateEnabled) || !st.has(atspiStateFocusable) || !st.has(atspiStateFocused) {
		t.Errorf("Save states %x", st)
	}
	var name string
	get(save, action+"GetName", &name, int32(0))
	var ok bool
	get(save, action+"DoAction", &ok, int32(0))
	if name != "click" || !ok {
		t.Errorf("action %q %v", name, ok)
	}
	if ev := takeAction(); ev.Node != 1 || ev.Action != semantics.ActionTap || ev.Window != 1 {
		t.Errorf("DoAction gave %+v", ev)
	}

	if st := states(find(atspiRoleCheckBox, "Remember")); !st.has(atspiStateChecked) {
		t.Errorf("checkbox states %x", st)
	}

	vol := find(atspiRoleSlider, "Volume")
	if v := prop(vol, "org.a11y.atspi.Value.CurrentValue"); v != 40.0 {
		t.Errorf("CurrentValue %v", v)
	}
	if err := client.Object(vol.Name, vol.Path).SetProperty("org.a11y.atspi.Value.CurrentValue", godbus.MakeVariant(75.0)); err != nil {
		t.Fatal(err)
	}
	if ev := takeAction(); ev.Node != 3 || ev.Action != semantics.ActionSetValue || ev.Number != 75 {
		t.Errorf("set value gave %+v", ev)
	}

	greet := find(atspiRoleEntry, "Greeting")
	var s0 string
	get(greet, text+"GetText", &s0, int32(0), int32(-1))
	if s0 != "こんにちは world" {
		t.Errorf("GetText %q", s0)
	}
	var word string
	var start, end int32
	if err := client.Object(greet.Name, greet.Path).Call(text+"GetStringAtOffset", 0, int32(7), uint32(atspiGranularityWord)).
		Store(&word, &start, &end); err != nil {
		t.Fatal(err)
	}
	if word != "world" || start != 6 || end != 11 {
		t.Errorf("word at 7: %q %d %d", word, start, end)
	}
	if err := client.Object(greet.Name, greet.Path).Call(text+"GetStringAtOffset", 0, int32(2), uint32(atspiGranularityWord)).
		Store(&word, &start, &end); err != nil {
		t.Fatal(err)
	}
	if word != "こんにちは" || start != 0 || end != 5 {
		t.Errorf("word at 2: %q %d %d", word, start, end)
	}
	var x, y, cw, ch int32
	if err := client.Object(greet.Name, greet.Path).Call(text+"GetCharacterExtents", 0, int32(3), uint32(atspiCoordWindow)).
		Store(&x, &y, &cw, &ch); err != nil {
		t.Fatal(err)
	}
	if x != 12 || y != 132 || cw != 190 || ch != 26 {
		t.Errorf("character extents %d %d %d %d", x, y, cw, ch)
	}
	get(greet, text+"SetSelection", &ok, int32(0), int32(6), int32(11))
	if ev := takeAction(); ev.Node != 4 || ev.Action != semantics.ActionSetSelection || ev.SelectionStart != 6 || ev.SelectionEnd != 11 {
		t.Errorf("SetSelection gave %+v", ev)
	}
	// After 😀 the code points and the UTF-16 units differ by one.
	get(find(atspiRoleEntry, "Emoji"), text+"SetSelection", &ok, int32(0), int32(2), int32(4))
	if ev := takeAction(); ev.Node != 5 || ev.SelectionStart != 3 || ev.SelectionEnd != 5 {
		t.Errorf("SetSelection after an emoji gave %+v, want UTF-16 3..5", ev)
	}

	if script := os.Getenv("GOWAKU_ATSPI_PYCHECK"); script != "" {
		out, err := exec.Command("python3", script, a.appName).CombinedOutput()
		t.Logf("pyatspi:\n%s", out)
		if err != nil || !strings.Contains(string(out), "push button 'Save'") {
			t.Errorf("pyatspi did not find the Save button: %v", err)
		}
	}

	// A node that is not there.
	err = client.Object(ours, godbus.ObjectPath(w.path(999))).Call(acc+"GetRole", 0).Store(new(uint32))
	var de godbus.Error
	if !errors.As(err, &de) || de.Name != errUnknownObject {
		t.Errorf("unknown node: %v, want %s", err, errUnknownObject)
	}
	var xml string
	get(save, "org.freedesktop.DBus.Introspectable.Introspect", &xml)
	if !strings.Contains(xml, `name="org.a11y.atspi.Action"`) || strings.Contains(xml, `name="org.a11y.atspi.Text"`) {
		t.Errorf("introspection of a button:\n%s", xml)
	}

	// Events.
	if err := client.AddMatchSignal(godbus.WithMatchInterface("org.a11y.atspi.Event.Object"), godbus.WithMatchSender(ours)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *godbus.Signal, 64)
	client.Signal(signals)
	renamed := *button
	renamed.Label = "Save as"
	root2 := &semantics.Node{Rect: root.Rect, Children: []*semantics.Node{&renamed, check, slider, field, emoji}}
	semantics.Compare(root, root2, 1, 2, &d)
	w.Update(root2, 2, &d)
	want := map[string]bool{"PropertyChange accessible-name Save as": false, "StateChanged focused 0": false, "StateChanged focused 1": false}
	deadline := time.After(10 * time.Second)
	for {
		missing := false
		for _, seen := range want {
			missing = missing || !seen
		}
		if !missing {
			break
		}
		select {
		case sig := <-signals:
			member := sig.Name[strings.LastIndex(sig.Name, ".")+1:]
			detail, _ := sig.Body[0].(string)
			key := member + " " + detail
			switch {
			case member == "PropertyChange" && string(sig.Path) == string(save.Path):
				v, _ := sig.Body[3].(godbus.Variant)
				s, _ := v.Value().(string)
				key += " " + s
			case member == "StateChanged" && detail == "focused":
				d1, _ := sig.Body[1].(int32)
				if (d1 == 0) != (string(sig.Path) == string(save.Path)) {
					t.Errorf("focused %d on %s", d1, sig.Path)
				}
				key += " " + strconv.Itoa(int(d1))
			}
			if _, ok := want[key]; ok {
				want[key] = true
			}
		case <-deadline:
			t.Fatalf("missing events: %v", want)
		}
	}

	w.Destroy()
	get(app, acc+"GetChildren", &frames)
	if len(frames) != 0 {
		t.Errorf("frames after Destroy: %v", frames)
	}
}
