package basic

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

func box(w, h float32, c geom.Color) core.Widget {
	return Container{Width: w, Height: h, Color: c}
}

func TestLayout_Placement(t *testing.T) {
	tests := []struct {
		name string
		w    core.Widget
		want []geom.Rect
	}{
		{"Padding は内側に置く", Padding{Insets: geom.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4}, Child: Container{Color: wtest.Red}},
			[]geom.Rect{{X: 1, Y: 2, W: 196, H: 94}}},
		{"Center は中央", Center{Child: box(20, 10, wtest.Red)}, []geom.Rect{{X: 90, Y: 45, W: 20, H: 10}}},
		{"Align 左上", Align{Child: box(20, 10, wtest.Red)}, []geom.Rect{{W: 20, H: 10}}},
		{"Align 右下", Align{Alignment: layout.Alignment{X: 1, Y: 1}, Child: box(20, 10, wtest.Red)}, []geom.Rect{{X: 180, Y: 90, W: 20, H: 10}}},
		{"SizedBox は親の tight に負ける", SizedBox{Width: 20, Height: 10, Child: Container{Color: wtest.Red}}, []geom.Rect{{W: 200, H: 100}}},
		{"ConstrainedBox の上限", Center{Child: ConstrainedBox{MaxWidth: 30, Child: Container{Color: wtest.Red, Width: 50, Height: 5}}},
			[]geom.Rect{{X: 85, Y: 47.5, W: 30, H: 5}}},
		{"Row は左から", Align{Child: Row{Size: layout.MainMin, Children: []core.Widget{box(10, 5, wtest.Red), box(20, 8, wtest.Green)}}},
			[]geom.Rect{{W: 10, H: 5}, {X: 10, W: 20, H: 8}}},
		{"Column は上から", Column{Children: []core.Widget{box(10, 5, wtest.Red), box(20, 8, wtest.Green)}},
			[]geom.Rect{{W: 10, H: 5}, {Y: 5, W: 20, H: 8}}},
		{"Spacing", Row{Spacing: 3, Children: []core.Widget{box(10, 5, wtest.Red), box(10, 5, wtest.Green)}},
			[]geom.Rect{{W: 10, H: 5}, {X: 13, W: 10, H: 5}}},
		{"MainEnd", Row{Main: layout.MainEnd, Children: []core.Widget{box(10, 5, wtest.Red)}}, []geom.Rect{{X: 190, W: 10, H: 5}}},
		{"MainCenter", Row{Main: layout.MainCenter, Children: []core.Widget{box(10, 5, wtest.Red)}}, []geom.Rect{{X: 95, W: 10, H: 5}}},
		{"SpaceBetween", Row{Main: layout.MainSpaceBetween, Children: []core.Widget{box(10, 5, wtest.Red), box(10, 5, wtest.Green)}},
			[]geom.Rect{{W: 10, H: 5}, {X: 190, W: 10, H: 5}}},
		{"SpaceAround", Row{Main: layout.MainSpaceAround, Children: []core.Widget{box(20, 5, wtest.Red), box(20, 5, wtest.Green)}},
			[]geom.Rect{{X: 40, W: 20, H: 5}, {X: 140, W: 20, H: 5}}},
		{"SpaceEvenly", Row{Main: layout.MainSpaceEvenly, Children: []core.Widget{box(20, 5, wtest.Red), box(20, 5, wtest.Green)}},
			[]geom.Rect{{X: 160.0 / 3, W: 20, H: 5}, {X: 160.0/3*2 + 20, W: 20, H: 5}}},
		{"CrossEnd", Row{Cross: layout.CrossEnd, Children: []core.Widget{box(10, 5, wtest.Red), box(10, 20, wtest.Green)}},
			[]geom.Rect{{Y: 95, W: 10, H: 5}, {X: 10, Y: 80, W: 10, H: 20}}},
		{"CrossCenter", Row{Cross: layout.CrossCenter, Children: []core.Widget{box(10, 10, wtest.Red)}}, []geom.Rect{{Y: 45, W: 10, H: 10}}},
		{"CrossStretch", Row{Cross: layout.CrossStretch, Children: []core.Widget{Container{Width: 10, Color: wtest.Red}}}, []geom.Rect{{W: 10, H: 100}}},
		{"Expanded は残りを分ける", Row{Children: []core.Widget{
			box(20, 5, wtest.Red),
			Expanded{Child: Container{Color: wtest.Green}},
			Expanded{Flex: 3, Child: Container{Color: wtest.Blue}},
		}}, []geom.Rect{{W: 20, H: 5}, {X: 20, W: 45, H: 0}, {X: 65, W: 135, H: 0}}},
		{"Flexible は小さいままでよい", Row{Children: []core.Widget{Flexible{Child: box(5, 5, wtest.Red)}, box(10, 5, wtest.Green)}},
			[]geom.Rect{{W: 5, H: 5}, {X: 5, W: 10, H: 5}}},
		{"Stack は重ねて左上", Layers(box(50, 20, wtest.Red), box(10, 10, wtest.Green)), []geom.Rect{{X: 0, Y: 0, W: 50, H: 20}, {W: 10, H: 10}}},
		{"Stack の中央揃え", Stack{Alignment: layout.Alignment{X: 0.5, Y: 0.5}, Children: []core.Widget{box(50, 20, wtest.Red), box(10, 10, wtest.Green)}},
			[]geom.Rect{{X: 75, Y: 40, W: 50, H: 20}, {X: 95, Y: 45, W: 10, H: 10}}},
		{"Positioned 左上基準", Stack{Fit: layout.StackExpand, Children: []core.Widget{
			Positioned{Left: 5, Top: 6, Width: 10, Height: 11, Child: Container{Color: wtest.Red}},
		}}, []geom.Rect{{X: 5, Y: 6, W: 10, H: 11}}},
		{"Positioned 右下基準", Stack{Fit: layout.StackExpand, Children: []core.Widget{
			Positioned{AnchorRight: true, Right: 5, AnchorBottom: true, Bottom: 6, Child: box(10, 11, wtest.Red)},
		}}, []geom.Rect{{X: 185, Y: 83, W: 10, H: 11}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wtest.Rects(t, tt.w, wtest.Page)
			// Empty boxes are dropped by the canvas; compare what was drawn.
			var want []geom.Rect
			for _, r := range tt.want {
				if !r.Size().Empty() {
					want = append(want, r)
				}
			}
			if len(got) != len(want) {
				t.Fatalf("got %v, want %v", got, want)
			}
			for i := range got {
				if !wtest.NearRect(got[i], want[i]) {
					t.Errorf("rect %d: got %v, want %v", i, got[i], want[i])
				}
			}
		})
	}
}

func TestFlex_ExpandedWidthsFillTheRow(t *testing.T) {
	got := wtest.Rects(t, Row{Cross: layout.CrossStretch, Spacing: 10, Children: []core.Widget{
		Expanded{Child: Container{Color: wtest.Red}},
		Expanded{Child: Container{Color: wtest.Green}},
	}}, wtest.Page)
	if got[0].W != 95 || got[1].X != 105 || got[1].W != 95 {
		t.Errorf("got %v", got)
	}
}

func TestFlex_FlexibleInUnboundedAxisPanics(t *testing.T) {
	inner := Column{Children: []core.Widget{Expanded{Child: Container{Color: wtest.Red}}}}
	wtest.ExpectPanic(t, "Expanded or Flexible in a Column with an unbounded height", func() {
		wtest.Rects(t, Column{Children: []core.Widget{inner}}, wtest.Page)
	})
}

func TestFlex_StretchInUnboundedCrossPanics(t *testing.T) {
	inner := Row{Cross: layout.CrossStretch, Children: []core.Widget{box(1, 1, wtest.Red)}}
	wtest.ExpectPanic(t, "Row with CrossStretch has an unbounded height", func() {
		wtest.Rects(t, Column{Children: []core.Widget{inner}}, wtest.Page)
	})
}

func TestFlex_StretchInVerticalScrollPanics(t *testing.T) {
	inner := Row{Cross: layout.CrossStretch, Children: []core.Widget{box(1, 1, wtest.Red)}}
	wtest.ExpectPanic(t, "Row with CrossStretch has an unbounded height", func() {
		wtest.Rects(t, Scroll{Child: inner}, wtest.Page)
	})
}

func TestScroll_HorizontalInVerticalPanics(t *testing.T) {
	inner := Scroll{Horizontal: true, Child: box(500, 20, wtest.Red)}
	wtest.ExpectPanic(t, "horizontal Scroll has an unbounded height", func() {
		wtest.Rects(t, Scroll{Child: inner}, wtest.Page)
	})
}

func TestScroll_InColumnPanics(t *testing.T) {
	inner := Scroll{Child: box(20, 500, wtest.Red)}
	wtest.ExpectPanic(t, "Scroll has an unbounded height", func() {
		wtest.Rects(t, Column{Children: []core.Widget{inner}}, wtest.Page)
	})
}

func TestScroll_HorizontalWithHeightInVerticalLaysOut(t *testing.T) {
	got := wtest.Rects(t, Scroll{Child: SizedBox{Height: 40, Child: Scroll{Horizontal: true, Child: box(500, 20, wtest.Red)}}}, wtest.Page)
	if got[0].Y != 0 || got[0].H != 20 {
		t.Errorf("got %v", got)
	}
}

// halves is a delegate that splits the space between two children.
type halves struct{}

func (halves) Layout(c layout.Constraints, children []core.RenderObject) geom.Size {
	size := c.Biggest()
	half := geom.Tight(geom.Size{W: size.W / 2, H: size.H})
	for i, child := range children {
		core.LayoutChild(child, half)
		child.SetOffset(geom.Offset{X: float32(i) * size.W / 2})
	}
	return size
}

func TestCustomLayout_UsesDelegate(t *testing.T) {
	got := wtest.Rects(t, CustomLayout{Delegate: halves{}, Children: []core.Widget{Container{Color: wtest.Red}, Container{Color: wtest.Green}}}, wtest.Page)
	if want := (geom.Rect{X: 100, W: 100, H: 100}); len(got) != 2 || got[1] != want {
		t.Errorf("got %v, want second %v", got, want)
	}
}

func TestHelpers_MatchTheirStructs(t *testing.T) {
	a, b := box(1, 1, wtest.Red), box(2, 2, wtest.Green)
	if got, want := testutil.Render(VStack(a, b), wtest.Page), testutil.Render(Column{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{a, b}}, wtest.Page); got != want {
		t.Errorf("VStack: got %q, want %q", got, want)
	}
	if got, want := testutil.Render(HStack(a, b), wtest.Page), testutil.Render(Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{a, b}}, wtest.Page); got != want {
		t.Errorf("HStack: got %q, want %q", got, want)
	}
	if got, want := testutil.Render(Layers(a, b), wtest.Page), testutil.Render(Stack{Children: []core.Widget{a, b}}, wtest.Page); got != want {
		t.Errorf("Layers: got %q, want %q", got, want)
	}
	if got, want := testutil.Render(Pad(4, a), wtest.Page), testutil.Render(Padding{Insets: geom.All(4), Child: a}, wtest.Page); got != want {
		t.Errorf("Pad: got %q, want %q", got, want)
	}
}

func TestSnapshot_Card(t *testing.T) {
	card := Center{Child: Container{
		Width: 120, Height: 60, Radius: 12, Padding: geom.All(8),
		Color:   geom.Hex(0xfef7ff),
		Border:  paint.Stroke{Width: 1, Color: wtest.Green},
		Shadows: []paint.BoxShadow{{Color: geom.Hex(0).WithAlpha(0.3), Offset: geom.Offset{Y: 2}, Blur: 6, Spread: 1}},
		Child: Row{Spacing: 4, Cross: layout.CrossStretch, Children: []core.Widget{
			Expanded{Child: Container{Color: wtest.Red, Radius: 4}},
			Container{
				Width:    30,
				Gradient: &paint.Gradient{End: geom.Offset{X: 30}, StartColor: wtest.Red, EndColor: wtest.Blue},
			},
		}},
	}}
	testutil.Golden(t, "card", testutil.Render(card, wtest.Page))
}

func TestDecoration_GradientFollowsTheBox(t *testing.T) {
	got := testutil.Render(Padding{Insets: geom.All(10), Child: Container{
		Gradient: &paint.Gradient{End: geom.Offset{X: 5}, StartColor: wtest.Red, EndColor: wtest.Blue},
	}}, wtest.Page)
	if !strings.Contains(got, "from=10,10") || !strings.Contains(got, "to=15,10") {
		t.Errorf("got %q", got)
	}
}

func TestRepaintOnly_ColorChangeSkipsLayout(t *testing.T) {
	color := wtest.Red
	var s *wtest.SwapState
	tree := core.NewTree(wtest.Swap{Build: func() core.Widget { return Container{Color: color} }, State: &s})
	testutil.RenderTree(tree, wtest.Page)
	if tree.NeedsFrame() {
		t.Fatal("tree must have settled")
	}
	s.SetState(func() { color = wtest.Green })
	got := testutil.RenderTree(tree, wtest.Page)
	if !strings.Contains(got, "#00ff00ff") {
		t.Errorf("got %q", got)
	}
	// An unchanged rebuild must not ask for anything.
	s.SetState(func() {})
	tree.Build()
	if tree.NeedsFrame() {
		t.Error("identical rebuild must not need a frame")
	}
}

func cards(n int) core.Widget {
	children := make([]core.Widget, n)
	for i := range children {
		children[i] = Padding{Insets: geom.All(1), Child: Container{Width: 10, Height: 10, Color: wtest.Red, Radius: 2}}
	}
	return Stack{Children: children}
}

func BenchmarkLayout_1kCards(b *testing.B) {
	tree := core.NewTree(cards(1000))
	testutil.RenderTree(tree, wtest.Page)
	c1, c2 := geom.Tight(wtest.Page), geom.Tight(geom.Size{W: 201, H: 100})
	b.ReportAllocs()
	flip := false
	for b.Loop() {
		flip = !flip
		if flip {
			tree.Layout(c2)
		} else {
			tree.Layout(c1)
		}
	}
}

func BenchmarkPaint_1kCards(b *testing.B) {
	tree := core.NewTree(cards(1000))
	testutil.RenderTree(tree, wtest.Page)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		tree.Paint(&cv)
	}
}
