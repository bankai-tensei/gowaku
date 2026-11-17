package control

import (
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestNumberStepper_ClampsAndDisables(t *testing.T) {
	cases := []struct {
		name               string
		s                  NumberStepper
		down, up           bool
		afterDown, afterUp int
	}{
		{"中ほど", NumberStepper{Value: 4, Min: 2, Max: 8}, true, true, 3, 5},
		{"下限", NumberStepper{Value: 2, Min: 2, Max: 8}, false, true, 2, 3},
		{"上限", NumberStepper{Value: 8, Min: 2, Max: 8}, true, false, 7, 8},
		{"上限なし", NumberStepper{Value: 100}, true, true, 99, 101},
		{"刻み 5 は端で止まる", NumberStepper{Value: 7, Min: 5, Max: 10, Step: 5}, true, true, 5, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := -1
			s := tc.s
			s.OnChange = func(v int) { got = v }
			tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), basic.Center{Child: s}))
			defer tree.Dispose()
			testutil.RenderTree(tree, geom.Size{W: 300, H: 100})
			bs := wtest.Buttons(tree.Semantics(), nil)
			if len(bs) != 2 {
				t.Fatalf("got %d buttons", len(bs))
			}
			for i, want := range []struct {
				enabled bool
				after   int
			}{{tc.down, tc.afterDown}, {tc.up, tc.afterUp}} {
				got = -1
				wtest.Tap(tree, wtest.Center(bs[i].Rect))
				switch {
				case want.enabled && got != want.after:
					t.Errorf("button %d: got %d, want %d", i, got, want.after)
				case !want.enabled && got != -1:
					t.Errorf("button %d should be disabled, got %d", i, got)
				}
			}
		})
	}
}

func TestNumberStepper_Format(t *testing.T) {
	s := NumberStepper{Value: 3, Label: "回数", Format: func(v int) string { return strconv.Itoa(v) + " 回" }}
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), s))
	defer tree.Dispose()
	testutil.RenderTree(tree, geom.Size{W: 300, H: 100})
	n := wtest.FindRole(tree.Semantics(), semantics.RoleStepper)
	if n == nil || n.Value != "3 回" || n.Label != "回数" {
		t.Fatalf("got %+v", n)
	}
}
