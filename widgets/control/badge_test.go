package control

import (
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
)

func TestBadge_Text(t *testing.T) {
	tests := []struct {
		name string
		b    Badge
		want string
	}{
		{"ゼロ値", Badge{}, ""},
		{"件数", Badge{Count: 3}, "3"},
		{"上限ちょうど", Badge{Count: 99}, "99"},
		{"上限を超える", Badge{Count: 100}, "99+"},
		{"上限を変える", Badge{Count: 10, Max: 9}, "9+"},
		{"負の件数", Badge{Count: -2}, ""},
		{"語が件数に勝つ", Badge{Count: 3, Label: "New"}, "New"},
		{"点", Badge{Dot: true}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.text(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBadge_NothingToShowLeavesTheChildAlone(t *testing.T) {
	child := box(24, 24, wtest.Red)
	want := testutil.Render(testutil.WithShaper(testutil.NewShaper(t), basic.Align{Child: child}), wtest.Screen)
	for name, b := range map[string]Badge{
		"ゼロ値":  {Child: child},
		"件数ゼロ": {Count: 0, Child: child},
	} {
		if got := testutil.Render(testutil.WithShaper(testutil.NewShaper(t), basic.Align{Child: b}), wtest.Screen); got != want {
			t.Errorf("%s: got\n%s\nwant\n%s", name, got, want)
		}
	}
	tree := wtest.TextTree(t, basic.Align{Child: Badge{}})
	if got := wtest.Rrects(tree, wtest.Screen); len(got) != 0 {
		t.Errorf("the zero value alone draws nothing: %+v", got)
	}
}

func TestBadge_HangsOverTheCornerWithoutTakingRoom(t *testing.T) {
	th := theme.Default().Light
	for _, tt := range []struct {
		name        string
		b           Badge
		x, y, w, h  float32
		wantAtLeast bool
	}{
		// The middle of a round badge is 2 inside the corner of the 24 box.
		{"点", Badge{Dot: true}, 24 - 2 - 4, 2 - 4, 8, 8, false},
		{"一桁", Badge{Count: 3}, 24 - 2 - 9, 2 - 9, 18, 18, false},
		// A longer one keeps the same left edge and grows outward.
		{"三桁", Badge{Count: 120}, 24 - 2 - 9, 2 - 9, 18, 18, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.b
			b.Child = box(24, 24, wtest.Red)
			tree := wtest.TextTree(t, basic.Align{Child: b})
			got := wtest.Rrects(tree, wtest.Screen)
			if len(got) != 2 {
				t.Fatalf("got %d boxes", len(got))
			}
			r := got[1].RRect.Rect
			if r.X != tt.x || r.Y != tt.y || r.H != tt.h || (!tt.wantAtLeast && r.W != tt.w) || r.W < tt.w {
				t.Errorf("got %+v", r)
			}
			if got[1].Color != th.Color.Accent {
				t.Errorf("got fill %+v, want Accent", got[1].Color)
			}
		})
	}
}

func TestBadge_TakesTheSizeOfItsChild(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: basic.Row{Size: layout.MainMin, Children: []core.Widget{
		Badge{Count: 5, Child: box(24, 24, wtest.Red)},
		box(10, 10, wtest.Red),
	}}})
	got := wtest.Rrects(tree, wtest.Screen)
	if len(got) != 3 || got[2].RRect.Rect.X != 24 {
		t.Errorf("the next box must follow the child at once: %+v", got)
	}
}

func TestBadge_QuietIsGray(t *testing.T) {
	th := theme.Default().Light
	tree := wtest.TextTree(t, basic.Align{Child: Badge{Count: 4, Quiet: true}})
	got := wtest.Rrects(tree, wtest.Screen)
	if len(got) != 1 || got[0].Color != th.Color.Fill {
		t.Fatalf("got %+v", got)
	}
	if got[0].RRect.Rect.H != badgeHeight || got[0].RRect.Rect.W != badgeHeight {
		t.Errorf("a one digit pill is round: %+v", got[0].RRect.Rect)
	}
}

func TestBadge_Semantics(t *testing.T) {
	tests := []struct {
		name string
		b    Badge
		want []string
	}{
		{"件数は読む", Badge{Count: 120, Child: basic.Text{Content: "Inbox"}}, []string{"Inbox", "99+"}},
		{"語は読む", Badge{Label: "New"}, []string{"New"}},
		{"点は黙る", Badge{Dot: true, Child: basic.Text{Content: "Inbox"}}, []string{"Inbox"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: tt.b})
			wtest.Ops(tree, wtest.Screen)
			texts := wtest.NodesOf(tree.Semantics(), semantics.RoleText, nil)
			var got []string
			for _, n := range texts {
				got = append(got, n.Label)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %q, want %q", got, tt.want)
				}
			}
		})
	}
}
