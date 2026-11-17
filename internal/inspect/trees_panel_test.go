//go:build gowakudebug

package inspect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
)

// shoot saves a screenshot of tt as name in $GOWAKU_INSPECT_SHOTS, when set,
// for a person to look at.
func shoot(tt *gowakutest.Tester, name string) {
	if dir := os.Getenv("GOWAKU_INSPECT_SHOTS"); dir != "" {
		tt.SaveScreenshot(filepath.Join(dir, name+".png"))
	}
}

func newInspector(t *testing.T, ses *Session, dark bool) *gowakutest.Tester {
	t.Helper()
	return gowakutest.New(t, Inspector{Session: ses}, gowakutest.Options{Width: 1180, Height: 780, Dark: dark})
}

func TestTreesPanel_FollowsAPick(t *testing.T) {
	ft, ses := newSample(t)
	tt := newInspector(t, ses, false)
	if _, ok := tt.Find(stringsEN.noSelection); !ok {
		t.Errorf("no empty state\n%s", tt.Dump())
	}
	// A pick selects the render object of the label of the button, deep in
	// the tree.
	label := findElement(t, ft.tree.RootElement(), "control.Button").NearestRenderObject()
	for label.ChildCount() > 0 {
		label = label.ChildAt(0)
	}
	ses.SelectRenderObject(ft, label)
	tt.Settle()
	_, el, _ := ses.Selection()
	if el.IsZero() {
		t.Fatal("the pick found no element")
	}
	// The element of the label is an inner working of Text, whose row stands
	// for it while the inner workings are hidden.
	shown := summary{}.row(el)
	if shown == el || TypeName(shown.Widget()) != "basic.Text" {
		t.Errorf("the row of %s is %s, want basic.Text", elementLabel(el), elementLabel(shown))
	}
	row, ok := tt.Find(elementLabel(shown))
	if !ok {
		t.Fatalf("the row of %s does not show\n%s", elementLabel(shown), tt.Dump())
	}
	if row.State&semantics.Selected == 0 {
		t.Errorf("the row of the pick is not selected: %+v", row)
	}
	if !strings.Contains(tt.Dump(), TypeName(label)) {
		t.Error("the details do not name the render object")
	}
	shoot(tt, "widgets-light")

	// The render tree selects the object.
	tt.Tap(stringsEN.renderTree)
	if _, ok := tt.Find(describe(label)); !ok {
		t.Errorf("the render tree does not show %s\n%s", describe(label), tt.Dump())
	}
	shoot(tt, "render-light")
}

func TestTreesPanel_ClickSelects(t *testing.T) {
	ft, ses := newSample(t)
	tt := newInspector(t, ses, false)
	text := findElement(t, ft.tree.RootElement(), "basic.Text")
	tt.Tap(elementLabel(text))
	target, el, ro := ses.Selection()
	if target != ft || el != text || ro != text.NearestRenderObject() {
		t.Errorf("got %v %v %v", target, el, ro)
	}
	if !strings.Contains(tt.Dump(), `"Content" = "\"Hello\""`) {
		t.Errorf("the details do not show the content of the text\n%s", tt.Dump())
	}
}

func TestTreesPanel_Semantics(t *testing.T) {
	ft, ses := newSample(t)
	tt := newInspector(t, ses, true)
	tt.Tap(stringsEN.semanticsTree)
	tt.Tap(`button "OK"`)
	_, _, ro := ses.Selection()
	if ro == nil || core.SemanticsIDOf(ro) == 0 {
		t.Fatalf("the node selected %v", ro)
	}
	// A selection elsewhere moves to the node that speaks for it.
	text := findElement(t, ft.tree.RootElement(), "basic.Text")
	ses.Select(ft, text)
	tt.Settle()
	row, ok := tt.Find(`text "Hello"`)
	if !ok || row.State&semantics.Selected == 0 {
		t.Errorf("the node of the text is not selected\n%s", tt.Dump())
	}
	shoot(tt, "semantics-dark")
}

func TestTreesPanel_Search(t *testing.T) {
	ft, ses := newSample(t)
	tt := newInspector(t, ses, false)
	tt.Tap(stringsEN.findInTree)
	tt.Type("button")
	_, el, _ := ses.Selection()
	if got := TypeName(el.Widget()); got != "control.Button" {
		t.Errorf("search: got %s", got)
	}
	tt.Type("zzz")
	if _, ok := tt.Find(stringsEN.noMatch); !ok {
		t.Error("no word for a search that finds nothing")
	}
	_ = ft
}

func TestDetailsPanel_DisplayList(t *testing.T) {
	ft, ses := newSample(t)
	tt := newInspector(t, ses, true)
	boundary := findElement(t, ft.tree.RootElement(), "basic.RepaintBoundary").NearestRenderObject()
	ses.SelectRenderObject(ft, boundary)
	tt.Settle()
	shoot(tt, "details-dark")
	tt.Tap(stringsEN.displayList)
	if !strings.Contains(tt.Dump(), "layer x=") {
		t.Errorf("the display list does not show\n%s", tt.Dump())
	}
	shoot(tt, "displaylist-dark")
}
