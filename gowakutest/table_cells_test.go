package gowakutest_test

import (
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

type tagRow struct{ name string }

// taggedTable is a table whose widget cells are padded pills of Body text
// that show the count of presses of a button outside the table.
type taggedTable struct{ gowaku.StatefulBase }

type taggedTableState struct {
	gowaku.StateBase
	presses int
}

func (taggedTable) CreateState() gowaku.State { return &taggedTableState{} }

func (s *taggedTableState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Column{Children: []gowaku.Widget{
		control.Button{Label: "Press", OnPress: func() { s.SetState(func() { s.presses++ }) }},
		basic.Expanded{Child: collection.Table[tagRow]{
			Label: "Tags", Rows: []tagRow{{"a"}, {"b"}, {"c"}},
			Columns: []collection.TableColumn[tagRow]{
				{Title: "Name", Text: func(r tagRow) string { return r.name }},
				{Title: "Tag", Width: 200, Kind: collection.CellWidget, Text: func(r tagRow) string { return r.name },
					Widget: func(r tagRow) gowaku.Widget {
						return basic.Container{
							Padding: geom.Symmetric(8, 4), Color: th.Color.Fill, Radius: th.Radius.Check,
							Child: basic.Text{Content: r.name + " " + strconv.Itoa(s.presses), Style: th.Text.Body},
						}
					}},
			},
		}},
	}}
}

// TestTable_WidgetCellsFitCenterAndFollowTheParent checks that a line of Body
// in a padded Container fits the room of a widget cell, in the middle of its
// row, and that a rebuild of the parent of the table builds the widgets of
// the rows in view again.
func TestTable_WidgetCellsFitCenterAndFollowTheParent(t *testing.T) {
	tt := gowakutest.New(t, taggedTable{}, gowakutest.Options{Width: 500, Height: 300, Fonts: goFonts.Fonts})
	for _, w := range tt.Warnings() {
		t.Error(w)
	}
	for _, name := range []string{"a", "b", "c"} {
		cell, ok := tt.FindRole(semantics.RoleText, name+" 0")
		if !ok {
			t.Fatalf("no cell %q:\n%s", name+" 0", tt.Dump())
		}
		row, ok := tt.FindRole(semantics.RoleRow, name+" "+name)
		if !ok {
			t.Fatalf("no row of %q:\n%s", name, tt.Dump())
		}
		if got, want := cell.Rect.Y+cell.Rect.H/2, row.Rect.Y+row.Rect.H/2; got != want || cell.Rect.H != 20 {
			t.Errorf("the text of %q is %v, its middle at %v, want 20 high with its middle at %v", name, cell.Rect, got, want)
		}
	}
	tt.Tap("Press")
	tt.Settle()
	for _, name := range []string{"a", "b", "c"} {
		if _, ok := tt.FindRole(semantics.RoleText, name+" 1"); !ok {
			t.Errorf("the cell of %q did not follow the rebuild of the parent:\n%s", name, tt.Dump())
		}
	}
}
