package control

import (
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// infoRig is an InfoBar whose Open lives in the test, as an app keeps it,
// above a box that shows how much room the bar takes.
type infoRig struct {
	t      *testing.T
	tree   *core.Tree
	swap   func(core.Widget)
	bar    InfoBar
	mq     basic.MediaQueryData
	size   geom.Size
	now    time.Duration
	closed int
	acted  int
}

func newInfoRig(t *testing.T, bar InfoBar) *infoRig {
	t.Helper()
	r := &infoRig{t: t, bar: bar, mq: basic.MediaQueryData{Scale: 1}, size: geom.Size{W: 400, H: 300}, now: time.Second}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	return r
}

func (r *infoRig) widget() core.Widget {
	return basic.MediaQuery{Data: r.mq, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{
		r.bar,
		basic.SizedBox{Height: 10},
	}}}
}

// set rebuilds the rig with bar.
func (r *infoRig) set(bar InfoBar) {
	r.bar = bar
	r.swap(r.widget())
	r.frame()
}

func (r *infoRig) frame() { wtest.Ops(r.tree, r.size) }

func (r *infoRig) step() {
	r.now += 8 * time.Millisecond
	r.tree.Tick(r.now)
	r.frame()
}

func (r *infoRig) settle() { wtest.Settled(r.t, r.tree, r.size, &r.now) }

// live returns the live node of the bar, or nil.
func (r *infoRig) live() *semantics.Node {
	root, _ := r.tree.SemanticsTree()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Live != semantics.LiveOff {
			found = n
		}
		return found == nil
	})
	return found
}

// height returns the room the bar takes, read from where the box below it
// starts.
func (r *infoRig) height() float32 {
	if n := r.live(); n != nil {
		return n.Rect.H
	}
	return 0
}

func TestInfoBar_AnnouncesOnceWhenItOpens(t *testing.T) {
	bar := InfoBar{Severity: SeverityWarning, Title: "オフライン", Message: "接続が戻ると同期します。"}
	r := newInfoRig(t, bar)
	if r.live() != nil {
		t.Fatal("a closed bar has a live node")
	}
	bar.Open = true
	r.set(bar)
	n := r.live()
	if n == nil {
		t.Fatal("the open bar has no live node")
	}
	if n.Label != "" {
		t.Errorf("the live node starts with %q; readers would not announce it", n.Label)
	}
	if n.Live != semantics.LivePolite {
		t.Errorf("a warning is %v, want polite", n.Live)
	}
	announcements, last := 0, ""
	watch := func() {
		if l := r.live().Label; l != last {
			if l != "" {
				announcements++
			}
			last = l
		}
	}
	for range 40 {
		r.step()
		watch()
	}
	// Rebuilding the parent with the same bar says nothing new.
	for range 5 {
		r.set(bar)
		r.step()
		watch()
	}
	if announcements != 1 {
		t.Errorf("announced %d times, want once", announcements)
	}
	if want := "Warning: オフライン 接続が戻ると同期します。"; last != want {
		t.Errorf("the label is %q, want %q", last, want)
	}

	// Closed and opened again, it is announced again.
	bar.Open = false
	r.set(bar)
	r.settle()
	if r.live() != nil {
		t.Fatal("the closed bar keeps its live node")
	}
	bar.Open = true
	r.set(bar)
	last = r.live().Label
	for range 5 {
		r.step()
		watch()
	}
	if announcements != 2 {
		t.Errorf("announced %d times after reopening, want 2", announcements)
	}
}

func TestInfoBar_LivenessFollowsSeverity(t *testing.T) {
	for _, c := range []struct {
		severity InfoSeverity
		live     semantics.Live
		word     string
	}{
		{SeverityInfo, semantics.LivePolite, "Information"},
		{SeveritySuccess, semantics.LivePolite, "Success"},
		{SeverityWarning, semantics.LivePolite, "Warning"},
		{SeverityError, semantics.LiveAssertive, "Error"},
	} {
		t.Run(c.severity.String(), func(t *testing.T) {
			r := newInfoRig(t, InfoBar{Open: true, Severity: c.severity, Message: "本文"})
			r.step()
			n := r.live()
			if n == nil || n.Live != c.live {
				t.Fatalf("got %+v, want live %v", n, c.live)
			}
			if want := c.word + ": 本文"; n.Label != want {
				t.Errorf("label %q, want %q", n.Label, want)
			}
		})
	}
}

func TestInfoBar_OpenFromTheStartShowsAtOnce(t *testing.T) {
	r := newInfoRig(t, InfoBar{Open: true, Title: "保存しました"})
	if h := r.height(); h < 30 {
		t.Errorf("an initially open bar takes %v, want its full height at once", h)
	}
	if r.tree.Ticking() {
		r.step()
	}
	if got := r.live().Label; got != "Information: 保存しました" {
		t.Errorf("label %q", got)
	}
}

func TestInfoBar_OpensAndClosesWithMotion(t *testing.T) {
	bar := InfoBar{Title: "更新があります"}
	r := newInfoRig(t, bar)
	if r.height() != 0 {
		t.Fatal("a closed bar takes room")
	}
	bar.Open = true
	r.set(bar)
	r.step()
	r.step()
	partial := r.height()
	r.settle()
	full := r.height()
	if partial <= 0 || partial >= full {
		t.Errorf("opening goes through %v on the way to %v", partial, full)
	}
	bar.Open = false
	r.set(bar)
	r.step()
	r.step()
	r.step()
	if h := r.height(); h <= 0 || h >= full {
		t.Errorf("closing goes through %v", h)
	}
	r.settle()
	if r.live() != nil || r.height() != 0 {
		t.Error("the closed bar stays")
	}
}

func TestInfoBar_ReduceMotionOnlyFades(t *testing.T) {
	bar := InfoBar{Title: "更新があります"}
	r := newInfoRig(t, bar)
	r.mq.ReduceMotion = true
	r.set(bar)
	bar.Open = true
	r.set(bar)
	r.step()
	first := r.height()
	r.settle()
	if full := r.height(); first != full {
		t.Errorf("with ReduceMotion the bar takes %v at once, then %v", first, full)
	}
}

func TestInfoBar_CloseAndAction(t *testing.T) {
	var r *infoRig
	bar := InfoBar{
		Open: true, Severity: SeverityError, Title: "保存できません", Action: "再試行",
		OnAction: func() { r.acted++ },
		OnClose:  func() { r.closed++ },
	}
	r = newInfoRig(t, bar)
	r.settle()
	root, _ := r.tree.SemanticsTree()
	buttons := wtest.RolesOf(root, semantics.RoleButton)
	if len(buttons) != 2 {
		t.Fatalf("%d buttons, want the action and the close button", len(buttons))
	}
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, root, "再試行").Rect))
	r.frame()
	if r.acted != 1 || r.closed != 0 {
		t.Errorf("the action ran %d times and closed %d times", r.acted, r.closed)
	}
	root, _ = r.tree.SemanticsTree()
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, root, "Close").Rect))
	r.frame()
	if r.closed != 1 {
		t.Errorf("the close button ran OnClose %d times", r.closed)
	}
	// The bar is controlled: it stays until the parent clears Open.
	r.settle()
	if r.live() == nil {
		t.Error("the bar closed by itself")
	}
}

func TestInfoBar_WithoutCallbacks(t *testing.T) {
	r := newInfoRig(t, InfoBar{Open: true, Message: "本文", Action: "詳細"})
	r.settle()
	root, _ := r.tree.SemanticsTree()
	buttons := wtest.RolesOf(root, semantics.RoleButton)
	if len(buttons) != 1 {
		t.Fatalf("%d buttons, want the action alone", len(buttons))
	}
	if buttons[0].State&semantics.Disabled == 0 {
		t.Error("an action without OnAction is enabled")
	}
}

func TestInfoBar_LongTextWraps(t *testing.T) {
	long := strings.Repeat("ディスクの空きが少なくなっています。", 6)
	r := newInfoRig(t, InfoBar{Open: true, Title: "警告", Message: long})
	wide := r.height()
	r.size.W = 220
	r.frame()
	narrow := r.height()
	if narrow <= wide {
		t.Errorf("the bar is %v high at 220 and %v at 400; the message should wrap", narrow, wide)
	}
	if n := r.live(); n.Rect.W > 220 {
		t.Errorf("the bar is %v wide in 220", n.Rect.W)
	}
}

func TestInfoBar_ZeroValue(t *testing.T) {
	r := newInfoRig(t, InfoBar{})
	if r.live() != nil || r.height() != 0 {
		t.Error("the zero value shows something")
	}
	r.set(InfoBar{Open: true})
	r.settle()
	if n := r.live(); n == nil || n.Label != "Information" {
		t.Errorf("an empty open bar: %+v", n)
	}
}

func TestInfoSeverity_String(t *testing.T) {
	if got := InfoSeverity(9).String(); got != "InfoSeverity(9)" {
		t.Errorf("got %q", got)
	}
}

func TestInfoBar_WarningIsNotBeige(t *testing.T) {
	// The amber of a warning keeps its chroma in both themes, so that it never
	// washes out to a beige.
	for _, b := range []theme.Brightness{theme.BrightnessLight, theme.BrightnessDark} {
		th := theme.New(geom.Hex(0x2f5fd0), b)
		hct := theme.HCTFromColor(severityColor(th, SeverityWarning))
		if hct.Chroma < 40 {
			t.Errorf("%v: the warning color has chroma %v", b, hct.Chroma)
		}
	}
}
