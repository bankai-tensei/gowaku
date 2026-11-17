package field

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// fieldBox returns the box of the one field that w draws on the screen.
func fieldBox(t *testing.T, w core.Widget) geom.Rect {
	t.Helper()
	ground := theme.Default().Light.Color.Field
	var boxes []geom.Rect
	for _, op := range wtest.Rrects(wtest.TextTree(t, w), wtest.Screen) {
		if op.Color == ground && op.BorderWidth > 0 {
			boxes = append(boxes, op.RRect.Rect)
		}
	}
	if len(boxes) != 1 {
		t.Fatalf("got %d field boxes, want 1", len(boxes))
	}
	return boxes[0]
}

func TestInput_MultilineFillsExpanded(t *testing.T) {
	for _, label := range []string{"", "Body"} {
		box := fieldBox(t, basic.Column{Children: []core.Widget{
			basic.Expanded{Child: Input{Label: label, Multiline: true}},
		}})
		if box.Y+box.H != wtest.Screen.H || box.H < wtest.Screen.H-40 {
			t.Errorf("label %q: the box is %v; it must fill the %v high Expanded", label, box, wtest.Screen.H)
		}
	}
}

func TestInput_MultilineGrowsWhereNothingSetsItsHeight(t *testing.T) {
	short := fieldBox(t, basic.Column{Children: []core.Widget{Input{Label: "Body", Multiline: true}}})
	long := fieldBox(t, basic.Column{Children: []core.Widget{Input{Label: "Body", Multiline: true, Value: "a\nb\nc\nd"}}})
	if short.H > 40 || long.H <= short.H {
		t.Errorf("an empty field is %v high and one of four lines %v; it must grow with its text", short.H, long.H)
	}
	if short.W != wtest.Screen.W {
		t.Errorf("the box is %v wide, want the width of the column", short.W)
	}
}

func TestInput_SingleLineKeepsItsHeightInExpanded(t *testing.T) {
	box := fieldBox(t, basic.Column{Children: []core.Widget{basic.Expanded{Child: Input{Label: "Name"}}}})
	if box.H != theme.ControlHeight {
		t.Errorf("the box is %v high, want %v", box.H, theme.ControlHeight)
	}
}
