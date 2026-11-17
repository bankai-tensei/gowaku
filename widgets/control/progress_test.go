package control

import (
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestProgressRing_SizeValueAndChild(t *testing.T) {
	ring := ProgressRing{Value: 0.25, Size: 120, Label: "残り", Child: basic.Text{Content: "25:00"}}
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), basic.Center{Child: ring}))
	defer tree.Dispose()
	out := testutil.RenderTree(tree, geom.Size{W: 300, H: 300})
	n := wtest.FindRole(tree.Semantics(), semantics.RoleProgress)
	if n == nil {
		t.Fatal("no progress node")
	}
	if n.Value != "25%" || n.Label != "残り" {
		t.Errorf("got %q %q", n.Label, n.Value)
	}
	if n.Rect != (geom.Rect{X: 90, Y: 90, W: 120, H: 120}) {
		t.Errorf("rect %+v", n.Rect)
	}
	if got := strings.Count(out, "path"); got != 2 {
		t.Errorf("want a track and an arc, got %d paths:\n%s", got, out)
	}
	if !strings.Contains(out, "glyphs") {
		t.Errorf("the child is not drawn:\n%s", out)
	}
}

func TestProgressRing_ZeroDrawsTrackOnly(t *testing.T) {
	out := testutil.Render(ProgressRing{}, geom.Size{W: 100, H: 100})
	if got := strings.Count(out, "path"); got != 1 {
		t.Errorf("got %d paths:\n%s", got, out)
	}
}

func TestProgressBar_FillsByValue(t *testing.T) {
	out := testutil.Render(basic.Center{Child: basic.SizedBox{Width: 200, Child: ProgressBar{Value: 0.5}}}, geom.Size{W: 300, H: 100})
	if got := strings.Count(out, "rrect"); got != 2 {
		t.Fatalf("got %d rrects:\n%s", got, out)
	}
	if !strings.Contains(out, "x=50 y=47 w=100 h=6") {
		t.Errorf("the done part should be half the bar:\n%s", out)
	}
}

func TestProgress_NewValueSlides(t *testing.T) {
	value := float32(0)
	var host *progressHostState
	tree := core.NewTree(progressHost{value: &value, state: &host})
	defer tree.Dispose()
	size := geom.Size{W: 200, H: 20}
	testutil.RenderTree(tree, size)
	host.SetState(func() { value = 1 })
	first := testutil.RenderTree(tree, size)
	if !tree.Ticking() {
		t.Fatal("a new value should animate")
	}
	now := time.Duration(0)
	var last string
	for range 200 {
		now += 16 * time.Millisecond
		tree.Tick(now)
		last = testutil.RenderTree(tree, size)
		if !tree.Ticking() {
			break
		}
	}
	if tree.Ticking() {
		t.Fatal("the animation never ends")
	}
	if first == last {
		t.Error("the bar did not move")
	}
	if want := testutil.Render(ProgressBar{Value: 1}, size); last != want {
		t.Errorf("got\n%s\nwant\n%s", last, want)
	}
}

// progressHost holds a ProgressBar whose value the test changes.
type progressHost struct {
	core.StatefulBase
	value *float32
	state **progressHostState
}

type progressHostState struct{ core.StateBase }

func (h progressHost) CreateState() core.State {
	*h.state = &progressHostState{}
	return *h.state
}

func (s *progressHostState) Build(core.BuildContext) core.Widget {
	return ProgressBar{Value: *s.Widget().(progressHost).value}
}

func TestCustomPaint_GetsItsSize(t *testing.T) {
	var got geom.Size
	p := layout.PainterFunc(func(cv *paint.Canvas, sz geom.Size) {
		got = sz
		cv.DrawRRect(geom.RRectAll(geom.Rect{W: sz.W, H: sz.H}, 4), paint.Paint{Color: geom.Hex(0xff0000)})
	})
	out := testutil.Render(basic.Center{Child: CustomPaint{Painter: p, Width: 40, Height: 30}}, geom.Size{W: 100, H: 100})
	if got != (geom.Size{W: 40, H: 30}) {
		t.Errorf("size %v", got)
	}
	if !strings.Contains(out, "rrect") {
		t.Errorf("nothing drawn:\n%s", out)
	}
	testutil.Render(CustomPaint{Painter: p}, geom.Size{W: 70, H: 50})
	if got != (geom.Size{W: 70, H: 50}) {
		t.Errorf("a zero size should fill, got %v", got)
	}
}

func TestPercentRange_ReadsAsTheShareSays(t *testing.T) {
	if got := kit.PercentRange(0.3).Value; got != 30 {
		t.Errorf("0.3 reads as %v, want 30", got)
	}
	if got := kit.PercentRange(0.12345).Value; got != 12.35 {
		t.Errorf("0.12345 reads as %v, want 12.35", got)
	}
	if got := kit.PercentRange(2).Value; got != 100 {
		t.Errorf("2 reads as %v, want 100", got)
	}
}
