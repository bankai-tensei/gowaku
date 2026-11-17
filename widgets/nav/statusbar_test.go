package nav

import (
	"slices"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// statusRig is a status bar at the bottom of a window of the given width.
type statusRig struct {
	tree    *core.Tree
	size    geom.Size
	pressed []string
}

func newStatusRig(t *testing.T, width float32, bar StatusBar) *statusRig {
	t.Helper()
	r := &statusRig{size: geom.Size{W: width, H: 200}}
	r.tree = wtest.TextTree(t, overlay.Overlay{Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Expanded{Child: basic.SizedBox{}},
		bar,
	}}})
	r.frame()
	return r
}

func (r *statusRig) frame() { testutil.RenderTree(r.tree, r.size) }

// resize lays the window out at width w.
func (r *statusRig) resize(w float32) {
	r.size.W = w
	r.frame()
}

// node returns the node labeled label, or nil.
func (r *statusRig) node(label string) *semantics.Node {
	root, _ := r.tree.SemanticsTree()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == label {
			found = n
		}
		return found == nil
	})
	return found
}

// shown returns which of labels the bar shows.
func (r *statusRig) shown(labels []string) []string {
	var out []string
	for _, l := range labels {
		if r.node(l) != nil {
			out = append(out, l)
		}
	}
	return out
}

func editorStatus() StatusBar {
	return StatusBar{
		Leading: []StatusItem{
			{Text: "行 12, 列 4", Priority: 3},
			{Text: "選択なし"},
		},
		Trailing: []StatusItem{
			{Text: "UTF-8", Priority: 1},
			{Text: "LF"},
			{Text: "日本語", Priority: 2},
		},
	}
}

func TestStatusBar_PlacesTheGroups(t *testing.T) {
	r := newStatusRig(t, 600, editorStatus())
	bar := r.node("Status bar")
	if bar == nil {
		t.Fatal("no node for the bar")
	}
	if bar.Rect.H != statusBarHeight+1 || bar.Rect.Y != r.size.H-statusBarHeight-1 || bar.Rect.W != 600 {
		t.Errorf("the bar is at %v; want the bottom 25 of the window", bar.Rect)
	}
	first, second := r.node("行 12, 列 4"), r.node("選択なし")
	if first == nil || second == nil {
		t.Fatal("the leading items are missing")
	}
	if first.Rect.X != theme8 || second.Rect.X <= first.Rect.X+first.Rect.W {
		t.Errorf("the leading items are at %v and %v", first.Rect, second.Rect)
	}
	last := r.node("日本語")
	if last == nil || geom.Abs32(last.Rect.X+last.Rect.W-(600-theme8)) > 1 {
		t.Errorf("the last trailing item ends at %v, want at 592", last.Rect)
	}
	utf, lf := r.node("UTF-8"), r.node("LF")
	if !(utf.Rect.X < lf.Rect.X && lf.Rect.X < last.Rect.X) {
		t.Errorf("the trailing items are out of order: %v %v %v", utf.Rect, lf.Rect, last.Rect)
	}
	// Each item is centered in the height of the bar.
	if mid := first.Rect.Y + first.Rect.H/2; geom.Abs32(mid-(bar.Rect.Y+1+statusBarHeight/2)) > 1 {
		t.Errorf("the text is centered at %v", mid)
	}
}

// theme8 is the room at the ends of the bar.
const theme8 = 8

func TestStatusBar_HidesByPriorityAsItNarrows(t *testing.T) {
	labels := []string{"行 12, 列 4", "選択なし", "UTF-8", "LF", "日本語"}
	// The order in which they go: lowest priority first, and of equal
	// priority the last one.
	order := []string{"LF", "選択なし", "UTF-8", "日本語"}
	r := newStatusRig(t, 800, editorStatus())
	if got := r.shown(labels); len(got) != len(labels) {
		t.Fatalf("at 800 the bar shows %v", got)
	}
	hiddenBefore := 0
	for w := float32(800); w >= 30; w -= 5 {
		r.resize(w)
		got := r.shown(labels)
		hidden := len(labels) - len(got)
		if hidden < hiddenBefore {
			t.Fatalf("at %v an item came back: %v", w, got)
		}
		hiddenBefore = hidden
		for _, gone := range order[:min(hidden, len(order))] {
			if slices.Contains(got, gone) {
				t.Fatalf("at %v the bar shows %v, which should have gone before the others: %v", w, gone, got)
			}
		}
	}
	if got := r.shown(labels); !slices.Equal(got, []string{"行 12, 列 4"}) {
		t.Errorf("at 30 the bar shows %v, want the item of the highest priority", got)
	}
	// It comes back as the window widens.
	r.resize(800)
	if got := r.shown(labels); len(got) != len(labels) {
		t.Errorf("widened again, the bar shows %v", got)
	}
}

func TestStatusBar_SeveralWidths(t *testing.T) {
	cases := []struct {
		width float32
		want  []string
	}{
		{900, []string{"行 12, 列 4", "選択なし", "UTF-8", "LF", "日本語"}},
		{60, []string{"行 12, 列 4"}},
		{10, []string{"行 12, 列 4"}},
	}
	labels := []string{"行 12, 列 4", "選択なし", "UTF-8", "LF", "日本語"}
	for _, c := range cases {
		r := newStatusRig(t, c.width, editorStatus())
		if got := r.shown(labels); !slices.Equal(got, c.want) {
			t.Errorf("at %v: %v, want %v", c.width, got, c.want)
		}
	}
	// An item that cannot fit alone stays inside the bar.
	r := newStatusRig(t, 10, editorStatus())
	if n := r.node("Status bar"); n.Rect.W != 10 {
		t.Errorf("the bar is %v wide in a window of 10", n.Rect.W)
	}
}

func TestStatusBar_PressableItems(t *testing.T) {
	var r *statusRig
	bar := StatusBar{
		Leading: []StatusItem{{Text: "準備完了"}},
		Trailing: []StatusItem{
			{Text: "UTF-8", Tooltip: "文字コード", OnPress: func() { r.pressed = append(r.pressed, "UTF-8") }},
			{Text: "LF", OnPress: func() { r.pressed = append(r.pressed, "LF") }},
		},
	}
	r = newStatusRig(t, 500, bar)
	wtest.Tap(r.tree, wtest.Center(r.node("UTF-8").Rect))
	r.frame()
	if !slices.Equal(r.pressed, []string{"UTF-8"}) {
		t.Fatalf("a click pressed %v", r.pressed)
	}
	// The click gave the button the keyboard; Tab goes on to the next one,
	// and Enter presses it.
	if got := wtest.FocusedLabel(r.tree); got != "UTF-8" {
		t.Errorf("the click left the focus on %q", got)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	r.frame()
	if got := wtest.FocusedLabel(r.tree); got != "LF" {
		t.Fatalf("the focus is on %q", got)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	r.frame()
	if !slices.Equal(r.pressed, []string{"UTF-8", "LF"}) {
		t.Errorf("Enter pressed %v", r.pressed)
	}
	if n := r.node("UTF-8"); n.Role != semantics.RoleButton {
		t.Errorf("a pressable item is %v", n.Role)
	}
}

func TestStatusBar_TooltipShowsOnHover(t *testing.T) {
	r := newStatusRig(t, 500, StatusBar{Trailing: []StatusItem{{Text: "UTF-8", Tooltip: "文字コード"}}})
	at := wtest.Center(r.node("UTF-8").Rect)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	now := time.Second
	wtest.Advance(r.tree, r.size, &now, 2*time.Second)
	if r.node("文字コード") == nil {
		t.Error("the tooltip did not show")
	}
}

func TestStatusBar_HiddenItemsLeaveTab(t *testing.T) {
	pressed := 0
	bar := StatusBar{
		Leading:  []StatusItem{{Text: "とても長い状態の説明がここに入ります", Priority: 5}},
		Trailing: []StatusItem{{Text: "設定", OnPress: func() { pressed++ }}},
	}
	r := newStatusRig(t, 200, bar)
	if r.node("設定") != nil {
		t.Fatal("the button of low priority still shows")
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	r.frame()
	if got := wtest.FocusedLabel(r.tree); got == "設定" {
		t.Error("Tab reached a hidden item")
	}
}

func TestStatusBar_ChildItemsAndLabels(t *testing.T) {
	bar := StatusBar{Label: "エディタの状態", Leading: []StatusItem{
		{Child: basic.SizedBox{Width: 12, Height: 12}, Label: "同期済み"},
	}}
	r := newStatusRig(t, 300, bar)
	if r.node("エディタの状態") == nil {
		t.Error("the bar does not take its Label")
	}
	if n := r.node("同期済み"); n == nil || n.Rect.W != 12 {
		t.Errorf("the widget item: %+v", n)
	}
}

func TestStatusBar_RightToLeftText(t *testing.T) {
	bar := StatusBar{
		Leading:  []StatusItem{{Text: "جاهز", Priority: 1}, {Text: "السطر ١٢"}},
		Trailing: []StatusItem{{Text: "עברית"}},
	}
	r := newStatusRig(t, 400, bar)
	for _, l := range []string{"جاهز", "السطر ١٢", "עברית"} {
		if r.node(l) == nil {
			t.Fatalf("%q is missing", l)
		}
	}
	// The bar keeps its left-to-right places: it does not mirror.
	if a, b := r.node("جاهز"), r.node("עברית"); a.Rect.X >= b.Rect.X {
		t.Errorf("the leading item is at %v, the trailing one at %v", a.Rect, b.Rect)
	}
	// Overflow works on right-to-left text as on any other.
	r.resize(60)
	if got := r.shown([]string{"جاهز", "السطر ١٢", "עברית"}); !slices.Equal(got, []string{"جاهز"}) {
		t.Errorf("at 60: %v", got)
	}
}

func TestStatusBar_ZeroValue(t *testing.T) {
	r := newStatusRig(t, 300, StatusBar{})
	n := r.node("Status bar")
	if n == nil || n.Rect.H != statusBarHeight+1 {
		t.Fatalf("the zero value: %+v", n)
	}
	r = newStatusRig(t, 300, StatusBar{Leading: []StatusItem{{}}, Trailing: []StatusItem{{}}})
	if n := r.node("Status bar"); n == nil {
		t.Error("empty items break the bar")
	}
}

func TestStatusBar_LayoutDoesNotAllocate(t *testing.T) {
	r := newStatusRig(t, 800, editorStatus())
	widths := [...]float32{800, 300, 120, 60}
	for _, w := range widths {
		r.resize(w)
	}
	i := 0
	allocs := testing.AllocsPerRun(50, func() {
		i++
		r.tree.Layout(geom.Tight(geom.Size{W: widths[i%len(widths)], H: 200}))
	})
	if allocs != 0 {
		t.Errorf("a layout of the bar allocates %v times", allocs)
	}
}
