package gowakutest_test

import (
	"errors"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// trayApp shows a tray icon from a button, notifies from its menu, and says
// what the user did to the icon.
type trayApp struct{ core.StatefulBase }

func (trayApp) CreateState() core.State { return &trayAppState{status: "idle"} }

type trayAppState struct {
	core.StateBase
	status string
	tray   *gowaku.Tray
}

func (s *trayAppState) say(what string) { s.SetState(func() { s.status = what }) }

func (s *trayAppState) show() {
	tray, err := gowaku.ShowTray(s.Context(), gowaku.TrayOptions{
		Tooltip: "Notes",
		Menu: []overlay.MenuItem{
			{Action: interact.Action{Label: "Notify(&N)", OnInvoke: func() {
				if err := gowaku.Notify(s.Context(), "Notes", "saved"); err != nil {
					s.say(err.Error())
				}
			}}},
			{Separator: true},
			{Action: interact.Action{Label: "Off"}},
			{Action: interact.Action{Label: "More"}, Items: []overlay.MenuItem{
				{Action: interact.Action{Label: "Hide", OnInvoke: func() { s.tray.Close() }}},
			}},
		},
		OnActivate:          func() { s.say("activated") },
		OnNotificationClick: func() { s.say("notification clicked") },
	})
	if err != nil {
		s.say(err.Error())
		return
	}
	s.tray = tray
}

func (s *trayAppState) Build(core.BuildContext) core.Widget {
	return basic.Column{Children: []core.Widget{
		control.Button{Label: "Show tray", OnPress: s.show},
		basic.Text{Content: s.status},
	}}
}

// TestTray_RecordsAndClicks: under a Tester, ShowTray and Notify do not panic
// but reach a fake tray, which the test reads and clicks.
func TestTray_RecordsAndClicks(t *testing.T) {
	tt := gowakutest.New(t, trayApp{}, goFonts)
	if len(tt.Trays()) != 0 || len(tt.Notifications()) != 0 {
		t.Fatal("a tray or a notification before any was shown")
	}
	tt.Tap("Show tray")
	trays := tt.Trays()
	if len(trays) != 1 {
		t.Fatalf("%d trays, want 1", len(trays))
	}
	tray := trays[0]
	if o := tray.Options(); o.Tooltip != "Notes" || len(o.Items) != 4 || o.Items[2].Enabled {
		t.Errorf("options %+v", o)
	}
	tray.Activate()
	if _, ok := tt.Find("activated"); !ok {
		t.Errorf("Activate did not run OnActivate:\n%s", tt.Dump())
	}
	tray.Choose("Notify(&N)")
	if n := tt.Notifications(); len(n) != 1 || n[0] != (gowakutest.Notification{Title: "Notes", Body: "saved"}) {
		t.Errorf("notifications %v", n)
	}
	tt.ClickNotification()
	if _, ok := tt.Find("notification clicked"); !ok {
		t.Errorf("ClickNotification did not run OnNotificationClick:\n%s", tt.Dump())
	}
	tray.Choose("Hide")
	if len(tt.Trays()) != 0 {
		t.Error("Close left the tray in Trays")
	}
}

// TestTray_NoTrayOutsideAHost: without a host that has a tray, as a tree
// outside Run and outside a Tester, ShowTray and Notify return errors instead
// of panicking.
func TestTray_NoTrayOutsideAHost(t *testing.T) {
	var trayErr, notifyErr, askErr error
	tree := core.NewTree(probe{run: func(ctx core.BuildContext) {
		_, trayErr = gowaku.ShowTray(ctx, gowaku.TrayOptions{})
		notifyErr = gowaku.Notify(ctx, "a", "b")
		gowaku.RequestNotifications(ctx, func(err error) { askErr = err })
	}})
	tree.Build()
	defer tree.Dispose()
	if !errors.Is(trayErr, gowaku.ErrNoTray) {
		t.Errorf("ShowTray: %v, want ErrNoTray", trayErr)
	}
	if !errors.Is(notifyErr, gowaku.ErrNoNotifications) {
		t.Errorf("Notify: %v, want ErrNoNotifications", notifyErr)
	}
	if !errors.Is(askErr, gowaku.ErrNoNotifications) {
		t.Errorf("RequestNotifications: %v, want ErrNoNotifications", askErr)
	}
}

// permissionApp asks for leave to notify when it starts, and says the answer.
type permissionApp struct{ core.StatefulBase }

func (permissionApp) CreateState() core.State { return &permissionAppState{status: "asking"} }

type permissionAppState struct {
	core.StateBase
	status string
}

func (s *permissionAppState) InitState() { s.ask() }

func (s *permissionAppState) ask() {
	gowaku.RequestNotifications(s.Context(), func(err error) {
		s.SetState(func() {
			s.status = "allowed"
			if err != nil {
				s.status = "refused"
			}
		})
	})
}

func (s *permissionAppState) Build(core.BuildContext) core.Widget {
	return basic.Column{Children: []core.Widget{
		control.Button{Label: "Ask", OnPress: s.ask},
		control.Button{Label: "Notify", OnPress: func() {
			if err := gowaku.Notify(s.Context(), "Notes", "due"); err != nil {
				s.SetState(func() { s.status = "not shown" })
			}
		}},
		basic.Text{Content: s.status},
	}}
}

// TestRequestNotifications_TheTestDecides: the Tester answers
// RequestNotifications after the call, as the desktop does, with what
// SetNotificationsAllowed says, and a refusal keeps Notify from recording.
func TestRequestNotifications_TheTestDecides(t *testing.T) {
	tt := gowakutest.New(t, permissionApp{}, goFonts)
	if _, ok := tt.Find("allowed"); !ok {
		t.Fatalf("the first answer must allow:\n%s", tt.Dump())
	}
	tt.SetNotificationsAllowed(false)
	tt.Tap("Ask")
	if _, ok := tt.Find("refused"); !ok {
		t.Fatalf("SetNotificationsAllowed(false) must refuse:\n%s", tt.Dump())
	}
	tt.Tap("Notify")
	if _, ok := tt.Find("not shown"); !ok || len(tt.Notifications()) != 0 {
		t.Errorf("a refused Notify must fail and record nothing: %v\n%s", tt.Notifications(), tt.Dump())
	}
	tt.SetNotificationsAllowed(true)
	tt.Tap("Notify")
	if len(tt.Notifications()) != 1 {
		t.Errorf("notifications %v", tt.Notifications())
	}
}

// probe runs run with its context once it is mounted.
type probe struct {
	core.StatefulBase
	run func(core.BuildContext)
}

func (probe) CreateState() core.State { return &probeState{} }

type probeState struct{ core.StateBase }

func (s *probeState) InitState() { s.Widget().(probe).run(s.Context()) }

func (s *probeState) Build(core.BuildContext) core.Widget { return basic.SizedBox{} }
