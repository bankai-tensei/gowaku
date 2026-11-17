package golden

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// tableScenes returns the scenes of Table: a table sorted by name, with
// rows selected, a frozen column, a filter in force and a cell being edited,
// scrolled sideways; and the filter of a column open.
func tableScenes() []Scene {
	s := make([]Scene, 0, 4)
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		edit := &collection.TableController{}
		open := &collection.TableController{}
		s = append(s,
			Scene{
				Name: "table" + suffix, Width: 1080, Height: 600, Scale: 1.5, Dark: dark,
				Root: func() core.Widget { return tablePage(edit) },
				Act:  editTable,
			},
			Scene{
				Name: "table-filter-open" + suffix, Width: 1080, Height: 600, Scale: 1.5, Dark: dark,
				Root: func() core.Widget { return tablePage(open) },
				Act:  openTableFilter,
			},
		)
	}
	return s
}

type file struct {
	name, kind, size, owner, modified string
}

func files() []file {
	kinds := []string{"Folder", "Document", "Image", "Spreadsheet"}
	owners := []string{"Aiko", "Ben", "Chiara"}
	names := []string{"Annual report", "Budget 2027", "Cover photo", "Draft", "Expenses", "Floor plan", "Garden",
		"Holiday", "Invoices", "Journal", "Kitchen", "Letters", "Minutes", "Notes", "Orders"}
	out := make([]file, len(names))
	for i, n := range names {
		out[i] = file{
			name: n, kind: kinds[i%len(kinds)], size: strconv.Itoa(12+i*37) + " KB", owner: owners[i%len(owners)],
			modified: "Nov " + strconv.Itoa(1+i) + ", 2026",
		}
	}
	return out
}

// tablePage is a table of files in a group, sorted by name, with two rows
// selected and the documents filtered.
func tablePage(ctrl *collection.TableController) core.Widget {
	if len(ctrl.Selection()) == 0 {
		ctrl.SetSelection(collection.RowRange{Start: 2, End: 4}, collection.RowRange{Start: 6, End: 7})
		ctrl.SetFocusedCell(3, 3)
	}
	return basic.Padding{Insets: geom.All(16), Child: tableGroup{child: collection.Table[file]{
		Label: "Files", Rows: files(), Controller: ctrl, Frozen: 1,
		Sort: collection.TableSort{Column: 0, Order: collection.SortAscending}, OnSort: func(collection.TableSort) {},
		OnFilter: func(int, collection.TableFilter) {}, OnCommit: func(collection.TableEdit) bool { return true },
		Columns: []collection.TableColumn[file]{
			{Title: "Name", Width: 180, Text: func(f file) string { return f.name }, Sortable: true, Editable: true},
			{Title: "Kind", Width: 150, Text: func(f file) string { return f.kind }, Filterable: true,
				FilterValues: []string{"Document", "Folder", "Image", "Spreadsheet"},
				Filter:       collection.TableFilter{Values: []string{"Document", "Folder", "Image"}}},
			{Title: "Size", Width: 110, Text: func(f file) string { return f.size }, Trailing: true, Sortable: true},
			{Title: "Owner", Width: 140, Text: func(f file) string { return f.owner }, Editable: true, Filterable: true},
			{Title: "Modified", Width: 160, Text: func(f file) string { return f.modified }, Sortable: true},
		},
	}}}
}

// tableGroup is the group a table sits in, in an application.
type tableGroup struct {
	core.StatelessBase
	child core.Widget
}

// Build implements core.StatelessWidget.
func (g tableGroup) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{
		Color:   th.Color.Card,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Padding: geom.All(4),
		Child:   g.child,
	}
}

// focusTable gives the keyboard to the cell in focus, as Tab does.
func focusTable(tree *core.Tree) {
	root, _ := tree.SemanticsTree()
	var id semantics.ID
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleGridCell && n.State&semantics.Focusable != 0 {
			id = n.ID
		}
		return id == 0
	})
	tree.PerformSemanticsAction(id, semantics.ActionFocus, core.SemanticsArg{})
}

// editTable scrolls the table sideways and edits the owner of the row below
// the cell in focus, keeping the selection.
func editTable(tree *core.Tree, _ core.BuildContext) {
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 300, Y: 200}, Scroll: geom.Offset{X: 0.5}})
	focusTable(tree)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyDown, Modifiers: input.ModControl})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyF2})
	tree.DispatchText(" & Chiara")
}

// openTableFilter opens the filter of the Kind column from the keyboard.
func openTableFilter(tree *core.Tree, _ core.BuildContext) {
	focusTable(tree)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyLeft})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyLeft})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyDown, Modifiers: input.ModAlt})
}
