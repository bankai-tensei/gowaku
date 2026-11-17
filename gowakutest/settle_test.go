package gowakutest_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// orbit turns on every frame for as long as it is mounted, as a map of
// satellites does, so the tree never comes to rest.
type orbit struct{ gowaku.StatefulBase }

type orbitState struct {
	gowaku.StateBase
	frames int
}

func (orbit) CreateState() gowaku.State { return &orbitState{} }

func (s *orbitState) InitState() {
	s.NewTicker(func(time.Duration) { s.SetState(func() { s.frames++ }) }).Start()
}

func (s *orbitState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Container{Height: 40, Child: basic.Text{Content: "orbit"}}
}

// mission has a form beside an orbit.
type mission struct{ gowaku.StatefulBase }

type missionState struct {
	gowaku.StateBase
	launched int
	name     string
	sent     string
}

func (mission) CreateState() gowaku.State { return &missionState{} }

func (s *missionState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Pad(16, basic.Column{Spacing: 8, Children: []gowaku.Widget{
		orbit{},
		control.Button{Label: "Launch", OnPress: func() { s.SetState(func() { s.launched++ }) }},
		basic.Text{Content: fmt.Sprintf("launched: %d", s.launched)},
		field.Input{
			Label: "Name", Value: s.name,
			OnChange: func(v string) { s.SetState(func() { s.name = v }) },
			OnSubmit: func(v string) { s.SetState(func() { s.sent = v }) },
		},
		basic.Text{Content: "sent: " + s.sent},
	}})
}

func TestOptionsSettle_ActsOnATreeThatNeverRests(t *testing.T) {
	opts := goFonts
	opts.Settle = 300 * time.Millisecond
	tt := gowakutest.New(t, mission{}, opts)

	tt.Tap("Launch")
	if tt.Now() != 300*time.Millisecond {
		t.Errorf("Tap moved the clock to %v, want 300ms", tt.Now())
	}
	if _, ok := tt.Find("launched: 1"); !ok {
		t.Errorf("the tap did not arrive:\n%s", tt.Dump())
	}
	tt.Tap("Name")
	tt.Type("ibuki")
	tt.Key(input.KeyEnter, 0)
	if _, ok := tt.Find("sent: ibuki"); !ok {
		t.Errorf("typing did not arrive:\n%s", tt.Dump())
	}
	if tt.Now() != 4*300*time.Millisecond {
		t.Errorf("the clock is at %v after four actions", tt.Now())
	}
	tt.Settle()
	if tt.Now() != 5*300*time.Millisecond {
		t.Errorf("Settle moved the clock to %v, want 1.5s", tt.Now())
	}
}

// fatalTB records the first failure of a Tester instead of ending the test.
type fatalTB struct {
	*testing.T
	msg string
}

func (f *fatalTB) Fatal(args ...any) {
	f.msg = fmt.Sprint(args...)
	runtime.Goexit()
}

func (f *fatalTB) Fatalf(format string, args ...any) {
	f.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// failure runs fn with a fatalTB on a goroutine of its own, as Goexit needs,
// and returns what failed.
func failure(t *testing.T, fn func(tb testing.TB)) string {
	tb := &fatalTB{T: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(tb)
	}()
	<-done
	return tb.msg
}

func TestOptionsSettle_ZeroStillWaitsForRest(t *testing.T) {
	msg := failure(t, func(tb testing.TB) {
		tt := gowakutest.New(tb, mission{}, goFonts)
		tt.Tap("Launch")
	})
	if !strings.Contains(msg, "never comes to rest") || !strings.Contains(msg, "Options.Settle") {
		t.Errorf("failure %q does not point at Options.Settle", msg)
	}
}

func TestTap_FailureShowsTheTree(t *testing.T) {
	msg := failure(t, func(tb testing.TB) {
		tt := gowakutest.New(tb, counter{}, goFonts)
		tt.Tap("Add two")
	})
	if !strings.Contains(msg, `"Add one"`) {
		t.Errorf("failure %q does not list what is there", msg)
	}
}

// fleet is a sample app: a sidebar picks a section, and the fleet section is a
// Navigator whose home page lists satellites as tiles in a grid, under an
// orbit, each with a Hero for its detail page.
type fleet struct{ gowaku.StatefulBase }

type fleetState struct {
	gowaku.StateBase
	section int
}

func (fleet) CreateState() gowaku.State { return &fleetState{} }

func (s *fleetState) Build(gowaku.BuildContext) gowaku.Widget {
	pick := func(i int) func() { return func() { s.SetState(func() { s.section = i }) } }
	var second gowaku.Widget = basic.Text{Content: "Overview"}
	if s.section == 1 {
		second = nav.Navigator{Home: fleetHome{}}
	}
	return control.SplitPane{
		First: basic.Column{Children: []gowaku.Widget{
			collection.SidebarItem{Label: "Overview", Selected: s.section == 0, OnPress: pick(0)},
			collection.SidebarItem{Label: "Fleet", Selected: s.section == 1, OnPress: pick(1)},
		}},
		Second: second,
	}
}

type fleetHome struct{ gowaku.StatelessBase }

func (fleetHome) Build(ctx gowaku.BuildContext) gowaku.Widget {
	navigator := nav.NavigatorOf(ctx)
	var tiles []gowaku.Widget
	for i, name := range []string{"いぶき 1 号", "いぶき 2 号"} {
		id := fmt.Sprintf("GOSAT-%d", i+1)
		tiles = append(tiles, interact.Pressable{Label: name, OnPress: func() {
			navigator.Push(nav.PageRoute{Name: "/" + id, Page: basic.Column{Children: []gowaku.Widget{
				nav.BackButton{},
				nav.Hero{Tag: id, Child: basic.Container{Width: 96, Height: 96, Child: basic.Text{Content: "い"}}},
			}}})
		}, Build: func(interact.PressState) gowaku.Widget {
			return basic.Container{Padding: geom.All(8), Child: basic.Row{Children: []gowaku.Widget{
				nav.Hero{Tag: id, Child: basic.Container{Width: 32, Height: 32, Child: basic.Text{Content: "い"}}},
				basic.Expanded{Child: basic.Column{Children: []gowaku.Widget{
					basic.Text{Content: name}, basic.Text{Content: id + " · LEO"},
				}}},
			}}}
		}})
	}
	return basic.Column{Children: []gowaku.Widget{
		orbit{},
		basic.Expanded{Child: basic.Scroll{Child: basic.Grid{MinColumnWidth: 160, TileHeight: 56, Children: tiles}}},
	}}
}

// press sends a click to the middle of the node labeled label straight to the
// tree, as a test that drives events itself does.
func press(t *testing.T, tt *gowakutest.Tester, label string) {
	t.Helper()
	n, ok := tt.Find(label)
	if !ok {
		t.Fatalf("nothing labeled %q in\n%s", label, tt.Dump())
	}
	pos := geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: input.ButtonPrimary})
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: input.ButtonPrimary})
}

func TestFind_SeesWhatAnEventChangedBeforeTheNextFrame(t *testing.T) {
	tt := gowakutest.New(t, fleet{}, goFonts)
	press(t, tt, "Fleet")
	// No frame since the click: Find builds the change first.
	if _, ok := tt.Find("いぶき 1 号"); !ok {
		t.Fatalf("the tile of the new section is missing:\n%s", tt.Dump())
	}
	if tt.Now() != 0 {
		t.Errorf("Find moved the clock to %v", tt.Now())
	}
}

func TestFind_TileInANavigatorWithHeroes(t *testing.T) {
	opts := goFonts
	opts.Settle = 500 * time.Millisecond
	tt := gowakutest.New(t, fleet{}, opts)
	tt.Tap("Fleet")
	n, ok := tt.Find("いぶき 1 号")
	if !ok || n.Role != semantics.RoleButton {
		t.Fatalf("want the tile as a button, got %v %v in\n%s", n, ok, tt.Dump())
	}
	tt.Tap("いぶき 1 号")
	if _, ok := tt.Find("いぶき 2 号"); ok {
		t.Errorf("the home page is still in the tree under the detail page:\n%s", tt.Dump())
	}
	tt.Tap("Back")
	if _, ok := tt.Find("いぶき 2 号"); !ok {
		t.Errorf("the home page did not come back:\n%s", tt.Dump())
	}
}

func TestAdvance_ZeroDrawsAFrame(t *testing.T) {
	tt := gowakutest.New(t, fleet{}, goFonts)
	press(t, tt, "Fleet")
	if !tt.Tree().NeedsFrame() {
		t.Fatal("the click changed nothing")
	}
	tt.Advance(0)
	if tt.Tree().NeedsFrame() {
		t.Error("Advance(0) drew no frame")
	}
	if tt.Now() != 0 {
		t.Errorf("Advance(0) moved the clock to %v", tt.Now())
	}
}
