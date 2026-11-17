package basic

import (
	"image"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
)

func TestIcon_ScalesFromItsViewBox(t *testing.T) {
	p, err := paint.ParseSVGPath("M0 0H24V24H0Z")
	if err != nil {
		t.Fatal(err)
	}
	icon := Icon{Path: p, Size: 48}
	if got := icon.scaled().Bounds().W; got != 48 {
		t.Errorf("got %v", got)
	}
	if got := (Icon{Path: p}).scaled(); got != p {
		t.Error("at its own size the path needs no copy")
	}
	tree := wtest.TextTree(t, Align{Child: icon})
	if got := testutil.RenderTree(tree, geom.Size{W: 100, H: 100}); !strings.HasPrefix(got, "path") {
		t.Errorf("got %q", got)
	}
}

func TestIcon_ZeroValueIsAnEmptySquare(t *testing.T) {
	tree := wtest.TextTree(t, Align{Child: Icon{}})
	if got := testutil.RenderTree(tree, geom.Size{W: 100, H: 100}); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestIcon_RebuildKeepsTheScaledPath(t *testing.T) {
	p, err := paint.ParseSVGPath("M0 0H24V24Z")
	if err != nil {
		t.Fatal(err)
	}
	var state *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: Icon{Path: p, Size: 48}, State: &state})
	size := geom.Size{W: 100, H: 100}
	testutil.RenderTree(tree, size)
	r := tree.HitTest(geom.Offset{X: 1, Y: 1}).Path[0].(*iconObject)
	scaled := r.Path
	state.SetState(func() { state.Child = Icon{Path: p, Size: 48, Color: geom.Hex(0xff0000)} })
	testutil.RenderTree(tree, size)
	if r.Path != scaled {
		t.Error("a new color must not scale the path again")
	}
	if r.Color != geom.Hex(0xff0000) {
		t.Errorf("got %v", r.Color)
	}
}

func picture200x100() *paint.Image {
	return paint.NewImage(image.NewRGBA(image.Rect(0, 0, 200, 100)))
}

func TestImage_Sizing(t *testing.T) {
	src := picture200x100()
	tests := []struct {
		name string
		w    Image
		want geom.Size
	}{
		{"元の大きさ", Image{Source: src}, geom.Size{W: 200, H: 100}},
		{"幅だけ指定で比率を保つ", Image{Source: src, Width: 50}, geom.Size{W: 50, H: 25}},
		{"高さだけ指定で比率を保つ", Image{Source: src, Height: 50}, geom.Size{W: 100, H: 50}},
		{"両方指定", Image{Source: src, Width: 30, Height: 40}, geom.Size{W: 30, H: 40}},
		{"画像なし", Image{}, geom.Size{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro := tt.w.CreateRenderObject()
			if got := ro.Layout(geom.Loose(geom.Size{W: 1000, H: 1000})); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImage_Fit(t *testing.T) {
	tests := []struct {
		name string
		fit  layout.ImageFit
		want string
	}{
		{"contain", layout.FitContain, "image x=0 y=25 w=100 h=50"},
		{"cover", layout.FitCover, "image x=-50 y=0 w=200 h=100"},
		{"fill", layout.FitFill, "image x=0 y=0 w=100 h=100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, Align{Child: Image{Source: picture200x100(), Width: 100, Height: 100, Fit: tt.fit}})
			got := testutil.RenderTree(tree, geom.Size{W: 300, H: 300})
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("got %q", got)
			}
		})
	}
}

func TestImage_RadiusClips(t *testing.T) {
	tree := wtest.TextTree(t, Align{Child: Image{Source: picture200x100(), Radius: 10}})
	got := testutil.RenderTree(tree, geom.Size{W: 300, H: 300})
	if !strings.HasPrefix(got, "clip-push") || !strings.Contains(got, "clip-pop") {
		t.Errorf("got %q", got)
	}
}

func TestRichText_ColorsAndSemantics(t *testing.T) {
	tree := wtest.TextTree(t, RichText{
		Spans: []text.Span{{Text: "plain "}, {Text: "red", Style: text.Style{Color: geom.Hex(0xff0000)}}},
		Style: text.Style{Color: geom.Hex(0x0000ff)},
	})
	got := testutil.RenderTree(tree, geom.Size{W: 300, H: 100})
	if !strings.Contains(got, "#0000ffff") || !strings.Contains(got, "#ff0000ff") {
		t.Errorf("got %q", got)
	}
	n := tree.Semantics().Children[0]
	if n.Role != semantics.RoleText || n.Label != "plain red" {
		t.Errorf("got %+v", n)
	}
}

func TestImage_QualityReachesTheCanvas(t *testing.T) {
	tree := wtest.TextTree(t, Align{Child: Image{Source: picture200x100(), Quality: paint.FilterNearest}})
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 300, H: 300}))
	var cv paint.Canvas
	tree.Repaint(&cv)
	found := false
	for _, op := range cv.DisplayList().Ops {
		if op.Kind == paint.OpImage {
			found = op.Filter == paint.FilterNearest
		}
	}
	if !found {
		t.Error("the image must be drawn nearest")
	}
	ro := Image{Source: picture200x100()}.CreateRenderObject().(*layout.RenderImage)
	Image{Source: picture200x100(), Quality: paint.FilterLinear}.UpdateRenderObject(ro)
	if ro.Quality != paint.FilterLinear {
		t.Errorf("got %v", ro.Quality)
	}
}

func TestIconAndImage_LabelMakesAnImageNode(t *testing.T) {
	p, err := paint.ParseSVGPath("M0 0H24V24H0Z")
	if err != nil {
		t.Fatal(err)
	}
	tree := wtest.A11yTree(t, Row{Children: []core.Widget{
		Icon{Path: p, Label: "Warning"},
		Image{Source: picture200x100(), Width: 40, Height: 20, Label: "Cover"},
		Icon{Path: p},
	}})
	root, _ := tree.SemanticsTree()
	for _, label := range []string{"Warning", "Cover"} {
		if n := wtest.ByLabel(t, root, label); n.Role != semantics.RoleImage {
			t.Errorf("%s is a %v", label, n.Role)
		}
	}
	if len(root.Children) != 2 {
		t.Errorf("an icon without a label must be left out: %d nodes", len(root.Children))
	}
}
