//go:build gowakudebug

package inspect

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// Nest stands for a widget of the application. Its package lies inside the
// gowaku module, so only an exported type is shown, as an application's
// widgets all are.
type Nest struct {
	core.StatelessBase
	depth int
}

func (n Nest) Build(core.BuildContext) core.Widget {
	if n.depth == 0 {
		return basic.Text{Content: "leaf"}
	}
	return basic.Padding{Insets: geom.All(1), Child: Nest{depth: n.depth - 1}}
}

func TestSummary_HidesTheInnerWorkings(t *testing.T) {
	ft, _ := newSample(t)
	root := ft.tree.RootElement()
	var m summary
	if m.shown(root) {
		t.Error("the inherited shaper at the root is shown")
	}
	for _, name := range []string{"control.Button", "Keyed"} {
		if el := findElement(t, root, name); !m.shown(el) {
			t.Errorf("%s is hidden", name)
		}
	}
	// No row of the summary is internal, and every element has a row that
	// stands for it, which lies above it or is itself.
	var walk func(el core.ElementView)
	walk = func(el core.ElementView) {
		for _, c := range m.children(el) {
			if internal(c) {
				t.Errorf("%s is a row", elementLabel(c))
			}
			walk(c)
		}
	}
	for _, top := range m.top(root) {
		walk(top)
	}
	all := summary{all: true}
	if !all.shown(root) || len(all.children(root)) != root.ChildCount() {
		t.Error("with all, the summary is not the whole tree")
	}
}

func TestSummary_KeepsTheWidgetsOfTheApplication(t *testing.T) {
	ft := newFakeTarget("nest", core.Inherited[*text.Shaper]{Value: testutil.NewShaper(t), Child: Nest{depth: 3}}, geom.Size{W: 200, H: 100})
	var m summary
	tops := m.top(ft.tree.RootElement())
	if len(tops) != 1 || TypeName(tops[0].Widget()) != "inspect.Nest" {
		t.Fatalf("top rows: %v", tops)
	}
}

func TestTreesPanel_RerootsADeepSelection(t *testing.T) {
	shaper := testutil.NewShaper(t)
	ft := newFakeTarget("deep", core.Inherited[*text.Shaper]{Value: shaper, Child: Nest{depth: 30}}, geom.Size{W: 400, H: 300})
	ses := NewSession(&fakeHost{targets: []Target{ft}}, shaper)
	tt := newInspector(t, ses, false)
	leaf := findElement(t, ft.tree.RootElement(), "basic.Text")
	ses.Select(ft, leaf)
	tt.Settle()
	row, ok := tt.Find("basic.Text")
	if !ok {
		t.Fatalf("the row of the leaf does not show\n%s", tt.Dump())
	}
	// The row must stay inside the trees, which are 460 wide.
	if row.Rect.X > 400 {
		t.Errorf("the row of the leaf starts at x %v, out of the panel", row.Rect.X)
	}
	// The breadcrumb leads back to the top.
	tt.Tap("inspect.Nest")
	tt.Settle()
}
