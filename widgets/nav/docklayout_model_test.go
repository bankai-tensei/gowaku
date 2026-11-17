package nav

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// shape writes the tree below n: a group as its panels in brackets, the
// active one starred, and a split as H(…) or V(…) of its children.
func shape(n *dockNode) string {
	if n == nil {
		return "<nil>"
	}
	if n.isGroup() {
		parts := make([]string, len(n.panels))
		for i, id := range n.panels {
			parts[i] = id
			if id == n.active {
				parts[i] += "*"
			}
		}
		return "[" + strings.Join(parts, " ") + "]"
	}
	parts := make([]string, len(n.children))
	for i, c := range n.children {
		parts[i] = shape(c)
	}
	axis := "H"
	if n.axis == layout.Vertical {
		axis = "V"
	}
	return axis + "(" + strings.Join(parts, " | ") + ")"
}

// checkParents fails if a node of the tree does not know its parent.
func checkParents(t *testing.T, n *dockNode) {
	t.Helper()
	for _, c := range n.children {
		if c.parent != n {
			t.Fatalf("%s has the wrong parent", shape(c))
		}
		checkParents(t, c)
	}
}

func near(a, b float32) bool { return geom.Abs32(a-b) < 1e-4 }

// controllerOf returns a controller synced to ids from def.
func controllerOf(t *testing.T, def DockArrangement, ids ...string) *DockController {
	t.Helper()
	c := &DockController{}
	c.sync(ids, &def)
	checkParents(t, c.root)
	return c
}

// twoGroups is files at the left and the editors at the right.
func twoGroups() DockArrangement {
	return DockArrangement{Root: DockNode{
		Weights:  []float32{1, 3},
		Children: []DockNode{{Panels: []string{"files"}}, {Panels: []string{"a", "b"}, Active: "b"}},
	}}
}

func TestDockController_ZeroValueIsOneGroup(t *testing.T) {
	c := controllerOf(t, DockArrangement{}, "a", "b", "c")
	if got := shape(c.root); got != "[a* b c]" {
		t.Errorf("got %s", got)
	}
	var zero DockController
	if a := zero.Arrangement(); !a.empty() {
		t.Errorf("a controller that never showed has nothing, got %+v", a)
	}
	empty := controllerOf(t, DockArrangement{})
	if got := shape(empty.root); got != "[]" {
		t.Errorf("no panels is one empty group, got %s", got)
	}
}

func TestDockController_DefaultAndNewPanels(t *testing.T) {
	c := controllerOf(t, twoGroups(), "files", "a", "b", "out")
	if got := shape(c.root); got != "H([files* out] | [a b*])" {
		t.Errorf("without a current group a new panel goes to the first, got %s", got)
	}
	c.current = c.groupOf("a")
	c.sync([]string{"files", "a", "b", "out", "props"}, &DockArrangement{})
	if got := shape(c.root); got != "H([files* out] | [a b* props])" {
		t.Errorf("a new panel goes to the current group, got %s", got)
	}
	if !near(c.root.weights[0], 0.25) || !near(c.root.weights[1], 0.75) {
		t.Errorf("got weights %v", c.root.weights)
	}
}

// got returns the shape, for tests that only want it built.
func got(t *testing.T, c *DockController) string {
	t.Helper()
	checkParents(t, c.root)
	return shape(c.root)
}

func TestDockController_MoveZones(t *testing.T) {
	cases := []struct {
		name   string
		def    DockArrangement
		id     string
		target string
		zone   DockZone
		want   string
	}{
		{"同じ群の中央は何もしない", DockArrangement{}, "b", "a", DockCenter, "[a* b c]"},
		{"群の左", DockArrangement{}, "b", "a", DockLeft, "H([b*] | [a* c])"},
		{"群の右", DockArrangement{}, "b", "a", DockRight, "H([a* c] | [b*])"},
		{"群の上", DockArrangement{}, "b", "a", DockTop, "V([b*] | [a* c])"},
		{"群の下", DockArrangement{}, "b", "a", DockBottom, "V([a* c] | [b*])"},
		{"全体の左", DockArrangement{}, "c", "", DockLeft, "H([c*] | [a* b])"},
		{"全体の下", DockArrangement{}, "c", "", DockBottom, "V([a* b] | [c*])"},
		{"別の群の中央", twoGroups(), "a", "files", DockCenter, "H([files a*] | [b*])"},
		{"最後の 1 枚が出ると分割が畳まれる", twoGroups(), "files", "a", DockCenter, "[a b files*]"},
		{"同じ向きの親には兄弟として入る", twoGroups(), "a", "b", DockRight, "H([files*] | [b*] | [a*])"},
		{"違う向きは入れ子になる", twoGroups(), "a", "b", DockBottom, "H([files*] | V([b*] | [a*]))"},
		{"全体の端で同じ向きの根に入る", twoGroups(), "a", "", DockRight, "H([files*] | [b*] | [a*])"},
		{"1 枚だけの群を自分の端に置いても変わらない", twoGroups(), "files", "files", DockRight, "H([files*] | [a b*])"},
		{"1 枚だけの全体の端は変わらない", DockArrangement{}, "a", "", DockLeft, "[a*]"},
		{"無い ID は次の同期で消える", DockArrangement{}, "zzz", "", DockLeft, "[a* b c]"},
		{"無い先は何もしない", DockArrangement{}, "a", "zzz", DockLeft, "[a* b c]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids := []string{"a", "b", "c"}
			if !tc.def.empty() {
				ids = []string{"files", "a", "b"}
			}
			if tc.name == "1 枚だけの全体の端は変わらない" {
				ids = []string{"a"}
			}
			c := controllerOf(t, tc.def, ids...)
			c.Move(tc.id, tc.target, tc.zone)
			// A panel the layout does not have leaves at the next build.
			c.sync(ids, &tc.def)
			if got := got(t, c); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDockController_EdgeShares(t *testing.T) {
	c := controllerOf(t, DockArrangement{}, "a", "b")
	c.Move("b", "", DockLeft)
	if !near(c.root.weights[0], dockRootShare) {
		t.Errorf("an edge of the whole layout takes a quarter, got %v", c.root.weights)
	}
	c = controllerOf(t, twoGroups(), "files", "a", "b")
	c.Move("a", "b", DockRight)
	if w := c.root.weights; !near(w[0], 0.25) || !near(w[1], 0.375) || !near(w[2], 0.375) {
		t.Errorf("the edge of a group halves its room, got %v", w)
	}
	// A panel that is alone and moves to the edge of the whole layout does not
	// count its old group in the new share.
	c = controllerOf(t, twoGroups(), "files", "a", "b")
	c.Move("files", "", DockRight)
	if got := shape(c.root); got != "H([a b*] | [files*])" {
		t.Fatalf("got %s", got)
	}
	if w := c.root.weights; !near(w[1], dockRootShare) {
		t.Errorf("got %v", w)
	}
}

func TestDockController_FlattensAndCollapses(t *testing.T) {
	def := DockArrangement{Root: DockNode{Children: []DockNode{
		{Panels: []string{"a"}},
		{Axis: layout.Vertical, Children: []DockNode{{Panels: []string{"b"}}, {Panels: []string{"c"}}}},
	}}}
	c := controllerOf(t, def, "a", "b", "c")
	c.Move("c", "b", DockCenter)
	if got := got(t, c); got != "H([a*] | [b c*])" {
		t.Errorf("got %s", got)
	}
	// A split in a split along the same axis joins it.
	nested := DockArrangement{Root: DockNode{Weights: []float32{1, 1}, Children: []DockNode{
		{Panels: []string{"a"}},
		{Weights: []float32{1, 1}, Children: []DockNode{{Panels: []string{"b"}}, {Panels: []string{"c"}}}},
	}}}
	c = controllerOf(t, nested, "a", "b", "c")
	if got := got(t, c); got != "H([a*] | [b*] | [c*])" {
		t.Errorf("got %s", got)
	}
	if w := c.root.weights; !near(w[0], 0.5) || !near(w[1], 0.25) || !near(w[2], 0.25) {
		t.Errorf("the shares multiply, got %v", w)
	}
}

func TestDockController_Close(t *testing.T) {
	c := controllerOf(t, twoGroups(), "files", "a", "b", "c")
	// c went to the first group; select a and close it.
	g := c.groupOf("a")
	g.active = "a"
	c.Close("a")
	if got := got(t, c); got != "H([files* c] | [b*])" {
		t.Errorf("the next tab is selected, got %s", got)
	}
	c.Close("b")
	if got := got(t, c); got != "[files* c]" {
		t.Errorf("an empty group goes, got %s", got)
	}
	if a := c.Arrangement(); !slices.Equal(a.Closed, []string{"a", "b"}) {
		t.Errorf("got closed %v", a.Closed)
	}
	// A closed panel stays closed across a sync.
	c.sync([]string{"files", "a", "b", "c"}, &DockArrangement{})
	if got := got(t, c); got != "[files* c]" {
		t.Errorf("got %s", got)
	}
	c.Activate("b")
	if got := got(t, c); got != "[files c b*]" {
		t.Errorf("Activate opens a closed panel, got %s", got)
	}
	c.Close("files")
	c.Close("c")
	c.Close("b")
	if got := got(t, c); got != "[]" {
		t.Errorf("closing everything leaves one empty group, got %s", got)
	}
}

func TestDockController_ResetAndMaximize(t *testing.T) {
	c := controllerOf(t, twoGroups(), "files", "a", "b")
	c.Move("a", "", DockBottom)
	c.Close("files")
	c.Maximize("b")
	if c.maximized == nil {
		t.Fatal("the group of b must fill the layout")
	}
	if a := c.Arrangement(); a.Maximized != "b" {
		t.Errorf("got maximized %q", a.Maximized)
	}
	c.Reset()
	def := twoGroups()
	c.sync([]string{"files", "a", "b"}, &def)
	if got := got(t, c); got != "H([files*] | [a b*])" || c.maximized != nil || len(c.closed) != 0 {
		t.Errorf("got %s, maximized %v, closed %v", got, c.maximized, c.closed)
	}
	c.Maximize("files")
	c.Close("files")
	if c.maximized != nil {
		t.Error("a maximized group that goes is no longer maximized")
	}
}

func TestDockController_SyncDropsUnknownAndDuplicates(t *testing.T) {
	a := DockArrangement{Root: DockNode{Children: []DockNode{
		{Panels: []string{"gone", "a", "a"}, Active: "gone"},
		{Panels: []string{"a", "old"}},
		{Panels: []string{"b"}, Active: "b"},
	}}, Closed: []string{"zzz", "c", "c"}}
	c := controllerOf(t, a, "a", "b", "c", "d")
	if got := got(t, c); got != "H([a* d] | [b*])" {
		t.Errorf("got %s", got)
	}
	if !slices.Equal(c.closed, []string{"c"}) {
		t.Errorf("got closed %v", c.closed)
	}
}

func TestDockController_BadWeights(t *testing.T) {
	for _, w := range [][]float32{nil, {1}, {1, -1}, {0, 0}, {2, float32(geom.Inf)}} {
		c := controllerOf(t, DockArrangement{Root: DockNode{Weights: w, Children: []DockNode{
			{Panels: []string{"a"}}, {Panels: []string{"b"}},
		}}}, "a", "b")
		if ws := c.root.weights; !near(ws[0], 0.5) || !near(ws[1], 0.5) {
			t.Errorf("%v: got %v", w, ws)
		}
	}
}

func TestDockController_Neighbor(t *testing.T) {
	def := DockArrangement{Root: DockNode{Children: []DockNode{
		{Panels: []string{"a"}},
		{Axis: layout.Vertical, Children: []DockNode{{Panels: []string{"b"}}, {Panels: []string{"c"}}}},
	}}}
	c := controllerOf(t, def, "a", "b", "c")
	c.root.place(geom.Rect{W: 801, H: 601})
	a, b, cc := c.groupOf("a"), c.groupOf("b"), c.groupOf("c")
	cases := []struct {
		from *dockNode
		zone DockZone
		want *dockNode
	}{
		{a, DockRight, b}, {a, DockLeft, nil}, {b, DockLeft, a}, {b, DockBottom, cc},
		{cc, DockTop, b}, {cc, DockLeft, a}, {cc, DockRight, nil}, {a, DockCenter, nil},
	}
	for _, tc := range cases {
		if got := c.neighbor(tc.from, tc.zone); got != tc.want {
			t.Errorf("%s %v: got %s", shape(tc.from), tc.zone, shape(got))
		}
	}
}

func TestDockController_Place(t *testing.T) {
	c := controllerOf(t, twoGroups(), "files", "a", "b")
	c.root.place(geom.Rect{W: 801, H: 400})
	f, e := c.groupOf("files").rect, c.groupOf("a").rect
	if f != (geom.Rect{W: 200, H: 400}) || e != (geom.Rect{X: 201, W: 600, H: 400}) {
		t.Errorf("got %v and %v", f, e)
	}
	// The smallest sizes add up along the axis and take the largest across.
	if got := c.root.minAlong(layout.Horizontal); got != 2*dockMinGroupWidth+dividerThickness {
		t.Errorf("got %v", got)
	}
	if got := c.root.minAlong(layout.Vertical); got != dockMinGroupHeight {
		t.Errorf("got %v", got)
	}
}

func TestDockArrangement_JSONRoundTrip(t *testing.T) {
	c := controllerOf(t, twoGroups(), "files", "a", "b", "out")
	c.Move("out", "", DockBottom)
	c.Close("files")
	c.Maximize("b")
	a := c.Arrangement()
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"root":{"split":"vertical","weights":[0.75,0.25],"children":[{"panels":["a","b"],"active":"b"},{"panels":["out"],"active":"out"}]},"maximized":"b","closed":["files"]}`
	if string(data) != want {
		t.Errorf("got %s", data)
	}
	var back DockArrangement
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	d := &DockController{}
	d.SetArrangement(back)
	d.sync([]string{"files", "a", "b", "out"}, &DockArrangement{})
	if got := got(t, d); got != shape(c.root) {
		t.Errorf("got %s, want %s", got, shape(c.root))
	}
	if d.maximized != d.groupOf("b") || !slices.Equal(d.closed, []string{"files"}) {
		t.Error("the maximized group and the closed panels must come back")
	}
	again, err := json.Marshal(d.Arrangement())
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Errorf("a second round trip must not change the text, got %s", again)
	}
}

func TestDockArrangement_RestoreToleratesOtherPanels(t *testing.T) {
	saved := `{"version":1,"root":{"split":"horizontal","weights":[1,3],"children":[{"panels":["files","plugin"]},{"panels":["a"]}]},"windows":[{"panels":["x"]}],"closed":["props"]}`
	var a DockArrangement
	if err := json.Unmarshal([]byte(saved), &a); err != nil {
		t.Fatalf("an unknown field such as windows must be ignored: %v", err)
	}
	c := &DockController{}
	c.SetArrangement(a)
	c.sync([]string{"files", "a", "props", "terminal"}, &DockArrangement{})
	if got := got(t, c); got != "H([files* terminal] | [a*])" {
		t.Errorf("an unknown panel is dropped and a new one goes to the first group, got %s", got)
	}
	if !slices.Equal(c.closed, []string{"props"}) {
		t.Errorf("got closed %v", c.closed)
	}
}

func TestDockArrangement_JSONErrors(t *testing.T) {
	cases := []struct {
		name, text string
		version    bool
	}{
		{"版が無い", `{"root":{"panels":["a"]}}`, true},
		{"新しい版", `{"version":2,"root":{"panels":["a"]}}`, true},
		{"知らない向き", `{"version":1,"root":{"split":"diagonal","children":[{"panels":["a"]},{"panels":["b"]}]}}`, false},
		{"壊れた JSON", `{"version":1,`, false},
	}
	for _, tc := range cases {
		var a DockArrangement
		a.Maximized = "keep"
		err := json.Unmarshal([]byte(tc.text), &a)
		if err == nil {
			t.Errorf("%s: no error", tc.name)
			continue
		}
		if errors.Is(err, ErrDockVersion) != tc.version {
			t.Errorf("%s: got %v", tc.name, err)
		}
		if a.Maximized != "keep" {
			t.Errorf("%s: a failed unmarshal must leave the value alone", tc.name)
		}
	}
	// The zero value marshals to a version and nothing else.
	data, err := json.Marshal(DockArrangement{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"version":1}` {
		t.Errorf("got %s", data)
	}
	var zero DockArrangement
	if err := json.Unmarshal(data, &zero); err != nil || !zero.empty() {
		t.Errorf("got %v %+v", err, zero)
	}
}

func TestDockZone_String(t *testing.T) {
	want := []string{"center", "left", "right", "top", "bottom"}
	for i, w := range want {
		if got := DockZone(i).String(); got != w {
			t.Errorf("got %q, want %q", got, w)
		}
	}
}
