package interact

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

func TestZeroValues_Work(t *testing.T) {
	for name, w := range map[string]core.Widget{
		"Padding": basic.Padding{}, "Align": basic.Align{}, "Center": basic.Center{Child: basic.SizedBox{}}, "SizedBox": basic.SizedBox{},
		"ConstrainedBox": basic.ConstrainedBox{}, "Row": basic.Row{}, "Column": basic.Column{}, "Stack": basic.Stack{},
		"Flex": basic.Flex{}, "Container": basic.Container{}, "CustomLayout": basic.CustomLayout{}, "Semantics": Semantics{},
		"DecoratedBox": basic.DecoratedBox{},
	} {
		t.Run(name, func(t *testing.T) {
			if got := testutil.Render(w, wtest.Page); got != "" {
				t.Errorf("got %q, want nothing drawn", got)
			}
		})
	}
}

func TestSemantics_RectAndNesting(t *testing.T) {
	tree := core.NewTree(basic.Padding{Insets: geom.All(5), Child: Semantics{
		Role: semantics.RoleButton, Label: "保存", Child: Semantics{Role: semantics.RoleText, Value: "x", Child: basic.Container{Width: 10, Height: 10, Color: wtest.Red}},
	}})
	testutil.RenderTree(tree, wtest.Page)
	root := tree.Semantics()
	n := root.Children[0]
	if n.Label != "保存" || n.Rect != (geom.Rect{X: 5, Y: 5, W: 190, H: 90}) {
		t.Errorf("got %+v", n)
	}
	if len(n.Children) != 1 || n.Children[0].Role != semantics.RoleText {
		t.Errorf("got %+v", n.Children)
	}
}

func TestExcludeSemantics_LeavesTheSubtreeOut(t *testing.T) {
	tapped := 0
	tree := wtest.A11yTree(t, basic.Row{Children: []core.Widget{
		GestureDetector{OnTap: func() { tapped++ }, Child: ExcludeSemantics{Child: basic.Text{Content: "S"}}},
		basic.Text{Content: "Sato"},
	}})
	texts := wtest.NodesOf(tree.Semantics(), semantics.RoleText, nil)
	if len(texts) != 1 || texts[0].Label != "Sato" {
		labels := make([]string, len(texts))
		for i, n := range texts {
			labels[i] = n.Label
		}
		t.Fatalf("text nodes %q, want only Sato", labels)
	}
	// It stays on screen and under the pointer.
	wtest.Tap(tree, geom.Offset{X: 2, Y: 5})
	if tapped != 1 {
		t.Errorf("tapped %d times, want 1", tapped)
	}
}
