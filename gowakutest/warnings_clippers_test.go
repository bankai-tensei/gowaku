package gowakutest_test

import (
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

type cake struct {
	name  string
	price int
}

// longTable is a table of many more rows than fit its height, with columns
// wider than the window, so that rows are built past its bottom edge and
// cells past its right one.
func longTable() gowaku.Widget {
	var rows []cake
	for i := range 200 {
		rows = append(rows, cake{"Cake " + strconv.Itoa(i), i})
	}
	return collection.Table[cake]{
		Label: "Cakes", Rows: rows,
		Columns: []collection.TableColumn[cake]{
			{Title: "Name", Width: 300, Text: func(c cake) string { return c.name }},
			{Title: "Price", Width: 300, Text: func(c cake) string { return strconv.Itoa(c.price) }},
			{Title: "Tag", Width: 200, Kind: collection.CellWidget, Text: func(c cake) string { return c.name },
				Widget: func(c cake) gowaku.Widget { return basic.Text{Content: c.name} }},
		},
	}
}

// TestWarnings_LongTableClipsItsRows: the rows a Table builds below its bottom
// edge, and the cells of columns past its right edge, are clipped, not
// overflow.
func TestWarnings_LongTableClipsItsRows(t *testing.T) {
	tt := gowakutest.New(t, longTable(), gowakutest.Options{Width: 500, Height: 300, Fonts: goFonts.Fonts})
	for _, w := range tt.Warnings() {
		t.Error(w)
	}
	tt.Scroll("Cake 0", 0, -2, 0)
	for _, w := range tt.Warnings() {
		t.Errorf("scrolled: %v", w)
	}
}

// sideSheet opens a side sheet that holds only a text.
type sideSheet struct {
	gowaku.StatelessBase
	edge overlay.SheetEdge
}

func (s sideSheet) Build(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: control.Button{Label: "Open", OnPress: func() {
		overlay.ShowSheet(ctx, overlay.Sheet{Edge: s.edge, Label: "Details"}, func(func()) gowaku.Widget {
			return basic.Text{Content: "Only a text"}
		})
	}}}
}

// TestWarnings_SheetCornersHangOutsideOnPurpose: the corners of a sheet that
// hang past the edge of the window are cut away, not overflow.
func TestWarnings_SheetCornersHangOutsideOnPurpose(t *testing.T) {
	for _, edge := range []overlay.SheetEdge{overlay.SheetTrailing, overlay.SheetLeading, overlay.SheetBottom} {
		t.Run(edge.String(), func(t *testing.T) {
			tt := gowakutest.New(t, sideSheet{edge: edge}, goFonts)
			tt.Tap("Open")
			if _, ok := tt.Find("Only a text"); !ok {
				t.Fatalf("no sheet:\n%s", tt.Dump())
			}
			for _, w := range tt.Warnings() {
				t.Error(w)
			}
		})
	}
}
