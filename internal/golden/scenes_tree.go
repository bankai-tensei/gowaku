package golden

import (
	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
)

// treeScenes returns the scenes of Tree: open folders with icons, a
// loading node and a selection of two rows with the keyboard in the tree; a
// rename in progress; and a drag showing where the rows would go, between
// rows and into a folder. The window asks for less motion, so that the
// loading row holds still.
func treeScenes() []Scene {
	return []Scene{
		{Name: "tree", Width: 540, Height: 666, Scale: 1.5, Root: treePage, Act: focusTree},
		{Name: "tree-dark", Width: 540, Height: 666, Scale: 1.5, Dark: true, Root: treePage, Act: focusTree},
		{Name: "tree-rename", Width: 540, Height: 666, Scale: 1.5, Root: treePage, Act: renameInTree},
		{Name: "tree-drag", Width: 540, Height: 666, Scale: 1.5, Root: treePage, Act: dragBetweenRows},
		{Name: "tree-drag-dark", Width: 540, Height: 666, Scale: 1.5, Dark: true, Root: treePage, Act: dragIntoFolder},
	}
}

// treeFolders returns the data of the scenes; "" holds the roots, and Server
// has children that are still loading.
func treeFolders() map[string][]string {
	return map[string][]string{
		"":          {"Projects", "Documents", "Server", "notes.txt"},
		"Projects":  {"gowaku", "website"},
		"gowaku":    {"widgets", "docs", "go.mod", "README.md"},
		"widgets":   {"tree.go", "table.go", "tabs.go"},
		"docs":      {"tree.md"},
		"website":   {"index.html"},
		"Documents": {"taxes.pdf"},
	}
}

func treePage() core.Widget {
	return basic.MediaQuery{
		Data:  basic.MediaQueryData{Scale: 1.5, ReduceMotion: true, Locale: language.English},
		Child: basic.Padding{Insets: geom.All(16), Child: treeCard{child: treeSample{}}},
	}
}

// treeCard is the card a tree sits on in a sidebar.
type treeCard struct {
	core.StatelessBase
	child core.Widget
}

// Build implements core.StatelessWidget.
func (t treeCard) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{Color: th.Color.Card, Radius: th.Radius.Group, Padding: geom.Symmetric(0, 6), Child: t.child}
}

// treeSample is a tree with some folders open and two files selected.
type treeSample struct{ core.StatefulBase }

type treeSampleState struct {
	core.StateBase
	ctrl collection.TreeController[string]
	data map[string][]string
}

// CreateState implements core.StatefulWidget.
func (treeSample) CreateState() core.State { return &treeSampleState{} }

// InitState implements core.State.
func (s *treeSampleState) InitState() {
	s.data = treeFolders()
	for _, k := range []string{"Projects", "gowaku", "widgets", "Server"} {
		s.ctrl.Expand(k)
	}
	s.ctrl.Select("table.go")
	s.ctrl.SetSelected("tree.go", true)
}

// Build implements core.State.
func (s *treeSampleState) Build(core.BuildContext) core.Widget {
	folder := func(k string) bool { return len(s.data[k]) > 0 || k == "Server" }
	return collection.Tree[string]{
		Roots:       s.data[""],
		ChildrenOf:  func(k string) []string { return s.data[k] },
		HasChildren: folder,
		IconOf:      func(k string) core.Widget { return treeIcon{folder: folder(k)} },
		Label:       "Files",
		Controller:  &s.ctrl,
		Multiselect: true,
		OnLoad:      func(string) {},
		OnRename:    func(string, string) {},
		OnMove:      func(collection.TreeMove[string]) {},
	}
}

// treeIcon is a folder or a page.
type treeIcon struct {
	core.StatelessBase
	folder bool
}

// Build implements core.StatelessWidget.
func (t treeIcon) Build(ctx core.BuildContext) core.Widget {
	c := &basic.ThemeOf(ctx).Color
	if t.folder {
		return basic.Icon{Path: folderIcon(), Size: 16, Color: c.AccentInk}
	}
	return basic.Icon{Path: pageIcon(), Size: 16, Color: c.Ink3}
}

func folderIcon() *paint.Path {
	p := &paint.Path{}
	p.AddRRect(geom.RRectAll(geom.Rect{X: 2, Y: 4, W: 9, H: 5}, 1.5))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 2, Y: 6, W: 20, H: 14}, 2))
	return p
}

func pageIcon() *paint.Path {
	p := &paint.Path{Rule: paint.FillEvenOdd}
	p.AddRRect(geom.RRectAll(geom.Rect{X: 5, Y: 2, W: 14, H: 20}, 2))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 7, Y: 4, W: 10, H: 16}, 1))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 9, Y: 8, W: 6, H: 1.5}, 0.75))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 9, Y: 12, W: 6, H: 1.5}, 0.75))
	return p
}

// focusTree gives the keyboard to the tree, which shows the selection in the
// accent and the ring on the row of the focus.
func focusTree(tree *core.Tree, _ core.BuildContext) {
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
}

// renameInTree starts renaming README.md and types a new name.
func renameInTree(tree *core.Tree, _ core.BuildContext) {
	pressTreeRow(tree, "README.md", 0)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: treeRowPoint(tree, "README.md", 0)})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyF2})
	frameTree(tree)
	tree.DispatchText("GUIDE")
}

// dragBetweenRows drags notes.txt to just above docs.
func dragBetweenRows(tree *core.Tree, _ core.BuildContext) {
	pressTreeRow(tree, "notes.txt", 0)
	to := treeRowPoint(tree, "docs", -12)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: to.Add(geom.Offset{Y: -20})})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: to})
}

// dragIntoFolder drags notes.txt onto the closed folder website, which opens
// when the drag rests on it.
func dragIntoFolder(tree *core.Tree, _ core.BuildContext) {
	pressTreeRow(tree, "notes.txt", 0)
	to := treeRowPoint(tree, "website", 0)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: to.Add(geom.Offset{Y: -20})})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: to})
}

// treeRowPoint returns a point on the row labeled label, dy from its middle.
func treeRowPoint(tree *core.Tree, label string, dy float32) geom.Offset {
	var at geom.Rect
	tree.Semantics().Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleTreeItem && n.Label == label {
			at = n.Rect
			return false
		}
		return true
	})
	return geom.Offset{X: at.X + at.W*0.6, Y: at.Y + at.H/2 + dy}
}

func pressTreeRow(tree *core.Tree, label string, dy float32) {
	pos := treeRowPoint(tree, label, dy)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
}

// frameTree runs a build and a layout, so that what an event changed shows
// before the next event, as a frame between them would.
func frameTree(tree *core.Tree) {
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 360, H: 444}))
	for tree.NeedsBuild() {
		tree.Build()
		tree.Layout(geom.Tight(geom.Size{W: 360, H: 444}))
	}
}
