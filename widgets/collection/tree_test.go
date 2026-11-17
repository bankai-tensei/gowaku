package collection

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

var treeScreen = geom.Size{W: 320, H: 300}

// sampleFolders is a small file system: "" holds the roots.
func sampleFolders() map[string][]string {
	return map[string][]string{
		"":          {"Documents", "Music", "Pictures", "notes.txt"},
		"Documents": {"Work", "Taxes", "resume.pdf"},
		"Work":      {"plan.md", "budget.xlsx"},
		"Taxes":     {"2025.pdf"},
		"Music":     {"Albums", "Singles"},
		"Albums":    {"Blue.flac"},
		"Pictures":  {"beach.png", "cat.png"},
	}
}

// treeRig drives a Tree over sampleFolders.
type treeRig struct {
	t      *testing.T
	tree   *core.Tree
	swap   func(core.Widget)
	ctrl   *TreeController[string]
	data   map[string][]string
	lazy   map[string]bool
	multi  bool
	clock  uint32
	now    time.Duration
	extra  func(*Tree[string])
	moves  []TreeMove[string]
	names  [][2]string
	opened []string
	loads  []string
	picks  [][]string
}

func newTreeRig(t *testing.T, multi bool, extra func(*Tree[string])) *treeRig {
	t.Helper()
	r := &treeRig{t: t, ctrl: &TreeController[string]{}, data: sampleFolders(), lazy: map[string]bool{}, multi: multi, extra: extra, now: time.Second}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	return r
}

func (r *treeRig) widget() core.Widget {
	v := Tree[string]{
		Roots:       r.data[""],
		ChildrenOf:  func(k string) []string { return r.data[k] },
		HasChildren: func(k string) bool { return len(r.data[k]) > 0 || r.lazy[k] },
		Label:       "Files",
		Controller:  r.ctrl,
		Multiselect: r.multi,
		OnSelectionChange: func(sel []string) {
			r.picks = append(r.picks, sel)
		},
		OnActivate: func(k string) { r.opened = append(r.opened, k) },
		OnLoad:     func(k string) { r.loads = append(r.loads, k) },
		OnRename:   func(k, name string) { r.names = append(r.names, [2]string{k, name}) },
		OnMove:     func(m TreeMove[string]) { r.moves = append(r.moves, m) },
	}
	if r.extra != nil {
		r.extra(&v)
	}
	return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: treeScreen.W, Height: treeScreen.H, Child: v}}
}

// refresh rebuilds the tree from the data, as the application would.
func (r *treeRig) refresh() {
	r.swap(r.widget())
	r.frame()
}

func (r *treeRig) frame() { wtest.Ops(r.tree, treeScreen) }

// tick plays frames for d.
func (r *treeRig) tick(d time.Duration) { wtest.Advance(r.tree, treeScreen, &r.now, d) }

func (r *treeRig) items() []*semantics.Node {
	return wtest.RolesOf(r.tree.Semantics(), semantics.RoleTreeItem)
}

// rows returns the labels of the rows that speak, nested by two spaces.
func (r *treeRig) rows() []string {
	var out []string
	for _, n := range r.items() {
		out = append(out, indent(n.Level)+n.Label)
	}
	return out
}

func indent(level int) string {
	s := ""
	for range level - 1 {
		s += "  "
	}
	return s
}

func (r *treeRig) item(label string) *semantics.Node {
	r.t.Helper()
	for _, n := range r.items() {
		if n.Label == label {
			return n
		}
	}
	r.t.Fatalf("no row %q in %v", label, r.rows())
	return nil
}

func (r *treeRig) focused() string { return wtest.FocusedLabel(r.tree) }

func (r *treeRig) Key(k input.KeyCode, m input.Modifiers) {
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
	r.frame()
}

func (r *treeRig) typeText(s string) {
	r.tree.DispatchText(s)
	r.frame()
}

// at returns a point on the label of a row, or dy from its top at x.
func (r *treeRig) at(label string, x, dy float32) geom.Offset {
	n := r.item(label)
	return geom.Offset{X: n.Rect.X + x, Y: n.Rect.Y + dy}
}

func (r *treeRig) pointer(kind core.PointerKind, pos geom.Offset, m input.Modifiers) {
	r.clock += 1000
	r.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: pos, Modifiers: m, Time: r.clock})
}

func (r *treeRig) click(label string, m input.Modifiers) {
	pos := r.at(label, 150, treeRowHeight/2)
	r.pointer(core.PointerMove, pos, 0)
	r.pointer(core.PointerDown, pos, m)
	r.pointer(core.PointerUp, pos, m)
	r.frame()
}

func (r *treeRig) doubleClick(label string) {
	pos := r.at(label, 150, treeRowHeight/2)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Time: 50000})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Time: 50010})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Time: 50100})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Time: 50110})
	r.frame()
}

func (r *treeRig) chevron(label string) {
	n := r.item(label)
	pos := geom.Offset{X: n.Rect.X + treeContentX(n.Level-1) + treeChevron/2, Y: n.Rect.Y + treeRowHeight/2}
	r.pointer(core.PointerDown, pos, 0)
	r.pointer(core.PointerUp, pos, 0)
	r.frame()
}

func (r *treeRig) selection() []string { return r.ctrl.Selection() }

func (r *treeRig) wantRows(want ...string) {
	r.t.Helper()
	if got := r.rows(); !slices.Equal(got, want) {
		r.t.Errorf("rows\n got %q\nwant %q", got, want)
	}
}

func (r *treeRig) wantFocus(want string) {
	r.t.Helper()
	if got := r.focused(); got != want {
		r.t.Errorf("focus on %q, want %q", got, want)
	}
}

func (r *treeRig) wantSelection(want ...string) {
	r.t.Helper()
	got := r.selection()
	slices.Sort(got)
	w := slices.Clone(want)
	slices.Sort(w)
	if !slices.Equal(got, w) {
		r.t.Errorf("selection %q, want %q", got, w)
	}
}

func TestTreeView_ZeroValue(t *testing.T) {
	tree := wtest.TextTree(t, basic.SizedBox{Width: 100, Height: 100, Child: Tree[string]{}})
	wtest.Ops(tree, treeScreen)
	if n := len(wtest.RolesOf(tree.Semantics(), semantics.RoleTreeItem)); n != 0 {
		t.Errorf("got %d items", n)
	}
	tree2 := wtest.TextTree(t, basic.SizedBox{Width: 200, Height: 100, Child: Tree[string]{Roots: []string{"a", "b"}}})
	wtest.Ops(tree2, treeScreen)
	items := wtest.RolesOf(tree2.Semantics(), semantics.RoleTreeItem)
	if len(items) != 2 || items[0].Label != "a" {
		t.Errorf("string keys must label themselves, got %v", items)
	}
}

func TestTreeView_Semantics(t *testing.T) {
	r := newTreeRig(t, true, nil)
	r.ctrl.Expand("Documents")
	r.ctrl.Select("Work")
	r.frame()
	r.wantRows("Documents", "  Work", "  Taxes", "  resume.pdf", "Music", "Pictures", "notes.txt")
	trees := wtest.RolesOf(r.tree.Semantics(), semantics.RoleTree)
	if len(trees) != 1 || trees[0].Label != "Files" || trees[0].State&semantics.Multiselectable == 0 {
		t.Fatalf("tree node %+v", trees)
	}
	docs := r.item("Documents")
	if docs.State&semantics.Expanded == 0 || docs.Actions&semantics.ActionCollapse == 0 || docs.PosInSet != 1 || docs.SetSize != 4 {
		t.Errorf("documents %+v", docs)
	}
	work := r.item("Work")
	if work.Level != 2 || work.State&(semantics.Selected|semantics.Collapsed) != semantics.Selected|semantics.Collapsed ||
		work.Actions&semantics.ActionExpand == 0 || work.PosInSet != 1 || work.SetSize != 3 {
		t.Errorf("work %+v", work)
	}
	if leaf := r.item("resume.pdf"); leaf.State&(semantics.Expanded|semantics.Collapsed) != 0 || leaf.PosInSet != 3 {
		t.Errorf("a leaf neither opens nor closes: %+v", leaf)
	}
	focusable := 0
	for _, n := range r.items() {
		if n.State&semantics.Focusable != 0 {
			focusable++
		}
	}
	if focusable != 1 {
		t.Errorf("one row must take Tab, got %d", focusable)
	}
	if single := newTreeRig(t, false, nil); wtest.RolesOf(single.tree.Semantics(), semantics.RoleTree)[0].State&semantics.Multiselectable != 0 {
		t.Error("a single selection tree is not multiselectable")
	}
}

func TestTreeView_SemanticsActions(t *testing.T) {
	r := newTreeRig(t, false, nil)
	root, _ := r.tree.SemanticsTree()
	var music semantics.ID
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == "Music" {
			music = n.ID
		}
		return true
	})
	if !r.tree.PerformSemanticsAction(music, semantics.ActionExpand, core.SemanticsArg{}) {
		t.Fatal("expand must be done")
	}
	r.frame()
	r.wantRows("Documents", "Music", "  Albums", "  Singles", "Pictures", "notes.txt")
	r.tree.SemanticsTree() // the actions go to the nodes of the last tree
	if !r.tree.PerformSemanticsAction(music, semantics.ActionTap, core.SemanticsArg{}) {
		t.Fatal("tap must be done")
	}
	r.frame()
	r.wantSelection("Music")
}

func TestTreeView_TabFocusesOneRow(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.Key(input.KeyTab, 0)
	r.wantFocus("Documents")
	r.click("Music", 0)
	r.wantFocus("Music")
	// The tree is the only stop, so Tab comes back to the row of the focus.
	r.Key(input.KeyTab, 0)
	r.wantFocus("Music")
}

func TestTreeView_Keyboard(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.Key(input.KeyTab, 0)
	steps := []struct {
		name  string
		key   input.KeyCode
		focus string
		rows  int
	}{
		{"下へ", input.KeyDown, "Music", 4},
		{"右で開く", input.KeyRight, "Music", 6},
		{"右で子へ", input.KeyRight, "Albums", 6},
		{"左で親へ", input.KeyLeft, "Music", 6},
		{"左で閉じる", input.KeyLeft, "Music", 4},
		{"End", input.KeyEnd, "notes.txt", 4},
		{"Home", input.KeyHome, "Documents", 4},
		{"上は先頭で止まる", input.KeyUp, "Documents", 4},
	}
	for _, s := range steps {
		r.Key(s.key, 0)
		if got := r.focused(); got != s.focus {
			t.Errorf("%s: focus on %q, want %q", s.name, got, s.focus)
		}
		if got := len(r.items()); got != s.rows {
			t.Errorf("%s: %d rows, want %d", s.name, got, s.rows)
		}
	}
	r.wantSelection("Documents")
}

func TestTreeView_CollapseMovesFocusToTheNode(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.ctrl.Expand("Documents")
	r.ctrl.Expand("Work")
	r.frame()
	r.click("plan.md", 0)
	r.chevron("Documents")
	r.wantRows("Documents", "Music", "Pictures", "notes.txt")
	r.wantFocus("Documents")
}

func TestTreeView_StarOpensSiblings(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.Key(input.KeyTab, 0)
	r.typeText("*")
	r.wantRows("Documents", "  Work", "  Taxes", "  resume.pdf", "Music", "  Albums", "  Singles",
		"Pictures", "  beach.png", "  cat.png", "notes.txt")
}

func TestTreeView_TypeAhead(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.Key(input.KeyTab, 0)
	r.typeText("p")
	r.wantFocus("Pictures")
	r.typeText("n")
	r.wantFocus("Pictures") // "pn" matches nothing; the focus stays
	r.tick(2 * time.Second)
	r.typeText("n")
	r.wantFocus("notes.txt")
	r.tick(2 * time.Second)
	r.typeText("m")
	r.typeText("u")
	r.wantFocus("Music")
	r.wantSelection("Music")
}

func TestTreeView_PageKeys(t *testing.T) {
	data := map[string][]string{"": nil}
	for i := range 100 {
		data[""] = append(data[""], "row"+strconv.Itoa(i))
	}
	r := &treeRig{t: t, ctrl: &TreeController[string]{}, data: data, lazy: map[string]bool{}, now: time.Second}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyPageDown, 0)
	page := int(treeScreen.H/treeRowHeight) - 1
	r.wantFocus("row" + strconv.Itoa(page))
	r.Key(input.KeyEnd, 0)
	r.wantFocus("row99")
	if r.item("row99").Rect.Y+treeRowHeight > treeScreen.H+0.5 {
		t.Error("the focus must scroll into view")
	}
	r.Key(input.KeyPageUp, 0)
	r.wantFocus("row" + strconv.Itoa(99-page))
}

func TestTreeView_EnterActivates(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyEnter, 0)
	if !slices.Equal(r.opened, []string{"Documents"}) {
		t.Errorf("activated %v", r.opened)
	}
	r.doubleClick("Music")
	if !slices.Equal(r.opened, []string{"Documents", "Music"}) {
		t.Errorf("activated %v", r.opened)
	}
	// Without OnActivate, Enter and a double click open and close.
	q := newTreeRig(t, false, func(v *Tree[string]) { v.OnActivate = nil })
	q.Key(input.KeyTab, 0)
	q.Key(input.KeyEnter, 0)
	if !q.ctrl.Expanded("Documents") {
		t.Error("Enter must open")
	}
	q.doubleClick("Music")
	if !q.ctrl.Expanded("Music") {
		t.Error("a double click must open")
	}
}

func TestTreeView_ClickSelection(t *testing.T) {
	r := newTreeRig(t, true, nil)
	r.click("Music", 0)
	r.wantSelection("Music")
	r.wantFocus("Music")
	r.click("notes.txt", input.ModControl)
	r.wantSelection("Music", "notes.txt")
	r.click("Music", input.ModControl)
	r.wantSelection("notes.txt")
	// The anchor is the row Ctrl last clicked.
	r.click("Documents", input.ModShift)
	r.wantSelection("Documents", "Music")
	r.click("Pictures", 0)
	r.wantSelection("Pictures")
	if len(r.picks) == 0 || !slices.Equal(r.picks[len(r.picks)-1], []string{"Pictures"}) {
		t.Errorf("OnSelectionChange got %v", r.picks)
	}
	// A press on one of several selected rows keeps them for a drag, and a
	// release without one selects that row alone.
	r.click("Documents", input.ModShift)
	r.click("Music", 0)
	r.wantSelection("Music")
	// A press below the rows clears the selection.
	r.pointer(core.PointerDown, geom.Offset{X: 100, Y: treeScreen.H - 5}, 0)
	r.pointer(core.PointerUp, geom.Offset{X: 100, Y: treeScreen.H - 5}, 0)
	r.frame()
	r.wantSelection()
}

func TestTreeView_ChevronDoesNotSelect(t *testing.T) {
	r := newTreeRig(t, true, nil)
	r.click("notes.txt", 0)
	r.chevron("Music")
	r.wantSelection("notes.txt")
	if !r.ctrl.Expanded("Music") {
		t.Error("the chevron must open")
	}
}

func TestTreeView_KeyboardSelection(t *testing.T) {
	r := newTreeRig(t, true, nil)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyDown, input.ModShift)
	r.Key(input.KeyDown, input.ModShift)
	r.wantSelection("Documents", "Music", "Pictures")
	r.Key(input.KeyUp, input.ModShift)
	r.wantSelection("Documents", "Music")
	r.Key(input.KeyDown, input.ModControl)
	r.Key(input.KeyDown, input.ModControl)
	r.wantFocus("notes.txt")
	r.wantSelection("Documents", "Music")
	r.Key(input.KeySpace, 0)
	r.wantSelection("Documents", "Music", "notes.txt")
	r.Key(input.KeySpace, 0)
	r.wantSelection("Documents", "Music")
	r.Key(input.KeyA, input.ModControl)
	r.wantSelection("Documents", "Music", "Pictures", "notes.txt")
	r.Key(input.KeyDown, 0)
	r.wantSelection("notes.txt")

	single := newTreeRig(t, false, nil)
	single.Key(input.KeyTab, 0)
	single.Key(input.KeyDown, input.ModShift)
	single.Key(input.KeyA, input.ModControl)
	single.wantSelection("Music")
	single.click("notes.txt", input.ModControl)
	single.wantSelection("notes.txt")
}

func TestTreeView_LazyChildren(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.data[""] = append(r.data[""], "Server")
	r.lazy["Server"] = true
	r.refresh()
	r.chevron("Server")
	r.wantRows("Documents", "Music", "Pictures", "notes.txt", "Server", "  Loading…")
	if !slices.Equal(r.loads, []string{"Server"}) {
		t.Fatalf("loads %v", r.loads)
	}
	loading := r.item("Loading…")
	if loading.Actions != 0 || loading.State&semantics.Focusable != 0 {
		t.Errorf("the loading row does nothing: %+v", loading)
	}
	r.refresh()
	r.tick(100 * time.Millisecond)
	if len(r.loads) != 1 {
		t.Errorf("one load per opening, got %v", r.loads)
	}
	r.data["Server"] = []string{"logs", "www"}
	r.refresh()
	r.wantRows("Documents", "Music", "Pictures", "notes.txt", "Server", "  logs", "  www")

	// A node opened from outside loads on the next frame.
	r.data[""] = append(r.data[""], "Backup")
	r.lazy["Backup"] = true
	r.refresh()
	r.ctrl.Expand("Backup")
	r.frame()
	r.tick(20 * time.Millisecond)
	if !slices.Equal(r.loads, []string{"Server", "Backup"}) {
		t.Errorf("loads %v", r.loads)
	}
	// Closing forgets the request, so that opening again retries.
	r.chevron("Backup")
	r.chevron("Backup")
	if len(r.loads) != 3 {
		t.Errorf("reopening must retry, loads %v", r.loads)
	}
}

func TestTreeView_LoadingRowSkippedByKeys(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.data[""] = []string{"Server", "Local"}
	r.lazy["Server"] = true
	r.refresh()
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyRight, 0)
	r.wantRows("Server", "  Loading…", "Local")
	r.Key(input.KeyDown, 0)
	r.wantFocus("Local")
	r.Key(input.KeyUp, 0)
	r.wantFocus("Server")
	r.Key(input.KeyRight, 0)
	r.wantFocus("Server")
}

func TestTreeView_Rename(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.click("notes.txt", 0)
	r.Key(input.KeyF2, 0)
	field := fieldw.FindEditable(r.tree.HitTest(r.at("notes.txt", 150, 14)).Path[0])
	if field == nil {
		t.Fatal("F2 must show a field")
	}
	if a, b := field.Selection(); a != 0 || b != len("notes") {
		t.Errorf("the stem must be selected, got %d..%d", a, b)
	}
	if r.tree.Focused() != core.FocusTarget(field) {
		t.Fatal("the field must take the keyboard")
	}
	r.typeText("todo")
	r.Key(input.KeyEnter, 0)
	if !slices.Equal(r.names, [][2]string{{"notes.txt", "todo.txt"}}) {
		t.Errorf("renames %v", r.names)
	}
	r.wantFocus("notes.txt")

	r.Key(input.KeyF2, 0)
	r.typeText("x")
	r.Key(input.KeyEscape, 0)
	if len(r.names) != 1 {
		t.Errorf("Escape must cancel, renames %v", r.names)
	}
	r.wantFocus("notes.txt")
	if fieldw.FindEditable(r.tree.HitTest(r.at("notes.txt", 150, 14)).Path[0]) != nil {
		t.Error("the field must go")
	}

	// Leaving the field commits.
	r.Key(input.KeyF2, 0)
	r.typeText("memo")
	r.click("Music", 0)
	if len(r.names) != 2 || r.names[1] != [2]string{"notes.txt", "memo.txt"} {
		t.Errorf("renames %v", r.names)
	}
	r.wantFocus("Music")

	// A name left alone renames nothing; Controller.Rename starts an edit.
	r.ctrl.Rename("Pictures")
	r.frame()
	r.Key(input.KeyEnter, 0)
	if len(r.names) != 2 {
		t.Errorf("an unchanged name must not rename, got %v", r.names)
	}
}

func TestTreeView_RenameNeedsOnRename(t *testing.T) {
	r := newTreeRig(t, false, func(v *Tree[string]) { v.OnRename = nil })
	r.click("notes.txt", 0)
	r.Key(input.KeyF2, 0)
	if fieldw.FindEditable(r.tree.HitTest(r.at("notes.txt", 150, 14)).Path[0]) != nil {
		t.Error("without OnRename F2 does nothing")
	}
}

// drag presses on from, moves to the row to at dy from its top, and drops.
func (r *treeRig) drag(from, to string, dy float32, m input.Modifiers) {
	start := r.at(from, 150, treeRowHeight/2)
	end := r.at(to, 150, dy)
	r.pointer(core.PointerDown, start, m)
	r.pointer(core.PointerMove, start.Add(geom.Offset{Y: 6}), m)
	r.frame()
	r.pointer(core.PointerMove, end, m)
	r.frame()
	r.pointer(core.PointerUp, end, m)
	r.frame()
}

func TestTreeView_DragReorder(t *testing.T) {
	cases := []struct {
		name, from, to string
		dy             float32
		want           TreeMove[string]
	}{
		{"前へ", "notes.txt", "Music", 2, TreeMove[string]{Nodes: []string{"notes.txt"}, Target: "Music", Drop: TreeDropBefore, AtRoot: true, Index: 1}},
		{"後へ", "Documents", "Pictures", 26, TreeMove[string]{Nodes: []string{"Documents"}, Target: "Pictures", Drop: TreeDropAfter, AtRoot: true, Index: 3}},
		{"葉の下半分は後", "Music", "notes.txt", 16, TreeMove[string]{Nodes: []string{"Music"}, Target: "notes.txt", Drop: TreeDropAfter, AtRoot: true, Index: 4}},
		{"中へ", "notes.txt", "Pictures", 14, TreeMove[string]{Nodes: []string{"notes.txt"}, Target: "Pictures", Drop: TreeDropInto, Parent: "Pictures", Index: 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newTreeRig(t, true, nil)
			r.drag(c.from, c.to, c.dy, 0)
			if len(r.moves) != 1 {
				t.Fatalf("moves %v", r.moves)
			}
			got := r.moves[0]
			if !slices.Equal(got.Nodes, c.want.Nodes) || got.Target != c.want.Target || got.Drop != c.want.Drop ||
				got.Parent != c.want.Parent || got.AtRoot != c.want.AtRoot || got.Index != c.want.Index {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestTreeView_DragAfterAnOpenNodeGoesFirstInside(t *testing.T) {
	r := newTreeRig(t, true, nil)
	r.ctrl.Expand("Music")
	r.frame()
	r.drag("notes.txt", "Music", 26, 0)
	if len(r.moves) != 1 || r.moves[0].Parent != "Music" || r.moves[0].Index != 0 || r.moves[0].AtRoot {
		t.Errorf("moves %+v", r.moves)
	}
}

func TestTreeView_DragSelection(t *testing.T) {
	r := newTreeRig(t, true, nil)
	r.ctrl.Expand("Documents")
	r.frame()
	r.click("Documents", 0)
	r.click("Taxes", input.ModControl)
	r.click("notes.txt", input.ModControl)
	r.drag("notes.txt", "Pictures", 14, 0)
	if len(r.moves) != 1 || !slices.Equal(r.moves[0].Nodes, []string{"Documents", "notes.txt"}) {
		t.Fatalf("a selected child of a moved node moves with it: %+v", r.moves)
	}
	r.wantSelection("Documents", "Taxes", "notes.txt")
}

func TestTreeView_DragRefusals(t *testing.T) {
	r := newTreeRig(t, true, nil)
	r.ctrl.Expand("Documents")
	r.frame()
	r.drag("Documents", "Work", 14, 0)
	r.drag("Documents", "Documents", 14, 0)
	if len(r.moves) != 0 {
		t.Errorf("a node cannot go into itself: %+v", r.moves)
	}
	q := newTreeRig(t, true, func(v *Tree[string]) {
		v.CanDrop = func(m TreeMove[string]) bool { return m.Drop != TreeDropInto }
	})
	q.drag("notes.txt", "Pictures", 14, 0)
	if len(q.moves) != 0 {
		t.Errorf("CanDrop must refuse, got %+v", q.moves)
	}
	// Escape cancels a drag.
	start, end := q.at("notes.txt", 150, 14), q.at("Music", 150, 2)
	q.pointer(core.PointerDown, start, 0)
	q.pointer(core.PointerMove, end, 0)
	q.frame()
	q.Key(input.KeyEscape, 0)
	q.pointer(core.PointerUp, end, 0)
	q.frame()
	if len(q.moves) != 0 {
		t.Errorf("Escape must cancel, got %+v", q.moves)
	}
	// Without OnMove nothing drags.
	n := newTreeRig(t, true, func(v *Tree[string]) { v.OnMove = nil })
	n.drag("notes.txt", "Music", 2, 0)
	n.wantSelection("notes.txt")
}

func TestTreeView_DragIndicator(t *testing.T) {
	r := newTreeRig(t, true, nil)
	start, end := r.at("notes.txt", 150, 14), r.at("Music", 150, 2)
	r.pointer(core.PointerDown, start, 0)
	r.pointer(core.PointerMove, end, 0)
	r.frame()
	body := treeBodyOf(t, r.tree)
	if !body.drop.shown || body.drop.into || body.drop.line != treeRowHeight {
		t.Errorf("a line must show above Music, got %+v", body.drop)
	}
	r.pointer(core.PointerMove, r.at("Pictures", 150, 14), 0)
	r.frame()
	if !body.drop.shown || !body.drop.into || body.drop.row != 2 {
		t.Errorf("a frame must show around Pictures, got %+v", body.drop)
	}
	r.pointer(core.PointerUp, end, 0)
	r.frame()
	if body.drop.shown {
		t.Error("the indicator must go with the drop")
	}
}

func TestTreeView_DragOpensAClosedNode(t *testing.T) {
	r := newTreeRig(t, true, nil)
	start := r.at("notes.txt", 150, 14)
	r.pointer(core.PointerDown, start, 0)
	r.pointer(core.PointerMove, r.at("Music", 150, 14), 0)
	r.frame()
	r.tick(300 * time.Millisecond)
	if r.ctrl.Expanded("Music") {
		t.Fatal("too soon")
	}
	r.tick(time.Second)
	if !r.ctrl.Expanded("Music") {
		t.Error("resting on a closed node must open it")
	}
	r.pointer(core.PointerUp, r.at("Albums", 150, 14), 0)
	r.frame()
	if len(r.moves) != 1 || r.moves[0].Target != "Albums" || r.moves[0].Drop != TreeDropInto {
		t.Errorf("moves %+v", r.moves)
	}
}

func TestTreeView_DragScrollsNearTheEdge(t *testing.T) {
	data := map[string][]string{"": nil}
	for i := range 100 {
		data[""] = append(data[""], "row"+strconv.Itoa(i))
	}
	r := &treeRig{t: t, ctrl: &TreeController[string]{}, data: data, lazy: map[string]bool{}, now: time.Second}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	start := r.at("row1", 150, 14)
	r.pointer(core.PointerDown, start, 0)
	r.pointer(core.PointerMove, geom.Offset{X: 150, Y: treeScreen.H - 4}, 0)
	r.frame()
	r.tick(500 * time.Millisecond)
	if off := treeBodyOf(t, r.tree).position.Value(); off < 100 {
		t.Errorf("the view must scroll toward the pointer, offset %v", off)
	}
	r.pointer(core.PointerUp, geom.Offset{X: 150, Y: treeScreen.H - 4}, 0)
	r.frame()
	if len(r.moves) != 1 || r.moves[0].Nodes[0] != "row1" {
		t.Errorf("moves %+v", r.moves)
	}
}

func treeBodyOf(t testing.TB, tree *core.Tree) *renderTreeBody {
	t.Helper()
	for _, ro := range tree.HitTest(geom.Offset{X: 10, Y: 10}).Path {
		if b, ok := ro.(*renderTreeBody); ok {
			return b
		}
	}
	t.Fatal("no tree body")
	return nil
}

func TestTreeController_FromOutside(t *testing.T) {
	data := map[string][]string{"": nil}
	for i := range 200 {
		data[""] = append(data[""], "row"+strconv.Itoa(i))
	}
	r := &treeRig{t: t, ctrl: &TreeController[string]{}, data: data, lazy: map[string]bool{}, now: time.Second}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	r.ctrl.Select("row150")
	r.frame()
	if n := r.item("row150"); n.State&semantics.Selected == 0 || n.Rect.Y < 0 || n.Rect.Y > treeScreen.H {
		t.Errorf("Select must show the node: %+v", n)
	}
	if f, ok := r.ctrl.Focused(); !ok || f != "row150" {
		t.Errorf("focus %q %v", f, ok)
	}
	r.ctrl.SetSelected("row151", true)
	r.ctrl.ClearSelection()
	r.frame()
	if len(r.ctrl.Selection()) != 0 {
		t.Error("ClearSelection")
	}
	// Only the rows near the view have nodes.
	if n := len(r.items()); n > int(treeScreen.H/treeRowHeight)+2 {
		t.Errorf("%d nodes for a view of %v", n, treeScreen.H)
	}
}

func TestTreeView_ChevronTurns(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.chevron("Music")
	if !r.tree.Ticking() {
		t.Fatal("the chevron must turn")
	}
	wtest.Settled(t, r.tree, treeScreen, &r.now)
	s := treeBodyOf(t, r.tree).host.(*treeState[string])
	if s.turning {
		t.Error("the turn must end")
	}
	// A window that asks for less motion turns at once.
	tree := wtest.TextTree(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: true}, Child: basic.SizedBox{
		Width: 200, Height: 200, Child: Tree[string]{Roots: []string{"a"}, HasChildren: func(string) bool { return true }},
	}})
	wtest.Ops(tree, treeScreen)
	root, _ := tree.SemanticsTree()
	items := wtest.RolesOf(root, semantics.RoleTreeItem)
	if !tree.PerformSemanticsAction(items[0].ID, semantics.ActionExpand, core.SemanticsArg{}) {
		t.Fatal("expand must be done")
	}
	wtest.Ops(tree, treeScreen)
	if n := len(wtest.RolesOf(tree.Semantics(), semantics.RoleTreeItem)); n != 2 {
		t.Fatalf("the loading row must show, got %d rows", n)
	}
	if tree.Ticking() {
		t.Error("ReduceMotion must not turn the chevron nor spin the loading row")
	}
}

func TestTreeView_RenameThatChangesTheKeyKeepsTheKeyboard(t *testing.T) {
	var r *treeRig
	r = newTreeRig(t, false, func(v *Tree[string]) {
		v.OnRename = func(old, name string) {
			roots := r.data[""]
			roots[slices.Index(roots, old)] = name
			r.ctrl.Select(name)
			r.swap(r.widget())
		}
	})
	r.click("notes.txt", 0)
	r.Key(input.KeyF2, 0)
	r.typeText("todo")
	r.Key(input.KeyEnter, 0)
	r.wantRows("Documents", "Music", "Pictures", "todo.txt")
	r.wantFocus("todo.txt")
	r.Key(input.KeyUp, 0)
	r.wantFocus("Pictures")
}

func TestNameStem(t *testing.T) {
	for name, want := range map[string]int{"notes.txt": 5, "archive.tar.gz": 11, ".gitignore": 10, "README": 6, "": 0} {
		if got := nameStem(name); got != want {
			t.Errorf("nameStem(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestTreeView_RevealBeforeTheFirstLayout(t *testing.T) {
	data := map[string][]string{"": nil}
	for i := range 100 {
		data[""] = append(data[""], "row"+strconv.Itoa(i))
	}
	// The controller selects a row far down before the tree has a height, as
	// a tree that follows a selection from elsewhere does when it appears.
	r := &treeRig{t: t, ctrl: &TreeController[string]{}, data: data, lazy: map[string]bool{}, now: time.Second}
	r.ctrl.Select("row90")
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	r.frame()
	it := r.item("row90")
	if it.Rect.Y < 0 || it.Rect.Y+treeRowHeight > treeScreen.H+0.5 {
		t.Errorf("row90 at y %v is out of view", it.Rect.Y)
	}
}

// TestTreeView_CollapsingAboveTheEndOfAScrolledTree closes a node while the
// view is at the end of its rows, which leaves the offset past the fewer rows
// until the next layout. The build must not reach for rows past the end.
func TestTreeView_CollapsingAboveTheEndOfAScrolledTree(t *testing.T) {
	kids := make([]int, 1000)
	for i := range kids {
		kids[i] = i
	}
	c := &TreeController[int]{}
	tree := wtest.TextTree(t, Tree[int]{
		Roots: []int{-1, -2},
		ChildrenOf: func(k int) []int {
			if k == -1 {
				return kids
			}
			return nil
		},
		LabelOf:    func(int) string { return "node" },
		Controller: c,
	})
	size := geom.Size{W: 300, H: 200}
	wtest.Ops(tree, size)
	c.Expand(-1)
	wtest.Ops(tree, size)
	s := treeBodyOf(t, tree).host.(*treeState[int])
	s.Jump(float64(len(s.rows)) * treeRowHeight)
	wtest.Ops(tree, size)
	c.Collapse(-1)
	wtest.Ops(tree, size)
	if n := len(s.rows); n != 2 {
		t.Fatalf("%d rows after the collapse, want 2", n)
	}
	if got := s.Offset.Value(); got != 0 {
		t.Errorf("the view is at %v after the collapse, want 0", got)
	}
}
