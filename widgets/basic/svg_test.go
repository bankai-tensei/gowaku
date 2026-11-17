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
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">` +
	`<circle cx="12" cy="12" r="9"/><path d="M8 12h8" stroke="#ff0000"/></svg>`

func TestSVG_DrawsWithThemeInkAndSize(t *testing.T) {
	data, err := paint.ParseSVG([]byte(testSVG))
	if err != nil {
		t.Fatal(err)
	}
	th := theme.Default().Light
	tree := wtest.TextTree(t, Theme{Data: th, Child: Align{Child: SVG{Data: data, Size: 48}}})
	got := testutil.RenderTree(tree, geom.Size{W: 100, H: 100})
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %q", got)
	}
	ink := th.Color.Ink.Std()
	if want := "color=#" + hexByte(ink.R) + hexByte(ink.G) + hexByte(ink.B); !strings.Contains(lines[0], want) {
		t.Errorf("currentColor is the ink: %q, want %s", lines[0], want)
	}
	if !strings.Contains(lines[1], "color=#ff0000ff") {
		t.Errorf("got %q", lines[1])
	}
	if !strings.Contains(lines[0], " w=40.0") {
		t.Errorf("the circle and its stroke scale to 40 of 48: %q", lines[0])
	}
}

func hexByte(b uint8) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[b>>4], digits[b&15]})
}

func TestSVG_ZeroValueIsAnEmptySquare(t *testing.T) {
	tree := wtest.TextTree(t, Align{Child: SVG{}})
	if got := testutil.RenderTree(tree, geom.Size{W: 100, H: 100}); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestSVG_LabelMakesAnImage(t *testing.T) {
	data, err := paint.ParseSVG([]byte(testSVG))
	if err != nil {
		t.Fatal(err)
	}
	tree := wtest.A11yTree(t, Align{Child: SVG{Data: data, Label: "Settings"}})
	root, _ := tree.SemanticsTree()
	if n := wtest.ByLabel(t, root, "Settings"); n.Role != semantics.RoleImage {
		t.Error("a labeled SVG is an image to assistive technology")
	}
}

func TestSVG_SetsTextWithTheShaper(t *testing.T) {
	data, err := paint.ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 24">` +
		`<text x="2" y="18" font-size="16" fill="#ff0000">Go</text></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if data.Skipped != 1 {
		t.Fatalf("without a shaper the text is skipped: %d", data.Skipped)
	}
	tree := wtest.TextTree(t, Align{Child: SVG{Data: data, Size: 48}})
	got := testutil.RenderTree(tree, geom.Size{W: 100, H: 100})
	if data.Skipped != 0 || strings.Count(got, "color=#ff0000ff") != 2 {
		t.Errorf("want two red glyphs, %d skipped: %q", data.Skipped, got)
	}
}

func TestSVG_CompositedPaintDoesNotAllocate(t *testing.T) {
	data, err := paint.ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` +
		`<filter id="f"><feGaussianBlur stdDeviation="1"/></filter>` +
		`<g opacity="0.5" filter="url(#f)"><circle cx="9" cy="12" r="6"/><circle cx="15" cy="12" r="6"/></g></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	r := &layout.RenderSVG{Extent: 48, Data: data, Color: geom.Color{A: 1}}
	core.LayoutChild(r, geom.Loose(geom.Size{W: 100, H: 100}))
	var cv paint.Canvas
	frame := func() {
		cv.Reset()
		r.Paint(&cv, geom.Offset{X: 3, Y: 4})
	}
	frame()
	// The group of the drawing, inside the cached group that keeps the
	// composited picture.
	dl := cv.DisplayList()
	if ops := dl.Ops; len(ops) != 6 || ops[0].Kind != paint.OpGroupPush || ops[1].Kind != paint.OpGroupPush ||
		ops[4].Kind != paint.OpGroupPop || ops[5].Kind != paint.OpGroupPop {
		t.Fatalf("got %d ops", len(ops))
	}
	if dl.Groups[0].Style.Cache == nil || dl.Ops[0].Origin != (geom.Offset{X: 3, Y: 4}) {
		t.Errorf("outer group %+v at %v, want a cached group at the drawing", dl.Groups[0], dl.Ops[0].Origin)
	}
	if n := testing.AllocsPerRun(20, frame); n != 0 {
		t.Errorf("got %v allocs", n)
	}
}
