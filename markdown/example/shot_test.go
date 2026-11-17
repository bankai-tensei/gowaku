package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// TestExample builds the example, follows the outline and selects across
// blocks. With MARKDOWN_SHOTS set to a folder, it saves the window there:
//
//	MARKDOWN_SHOTS=/tmp/shots go test ./example
func TestExample(t *testing.T) {
	tt := gowakutest.New(t, app{}, gowakutest.Options{Width: 900, Height: 760})
	dir := os.Getenv("MARKDOWN_SHOTS")
	shot := func(name string) {
		if dir != "" {
			tt.SaveScreenshot(filepath.Join(dir, name))
		}
	}
	if _, ok := tt.Find("gowaku のロゴ"); !ok {
		t.Error("the logo is not in the document")
	}

	// The outline brings a heading to the top.
	press(t, tt, "リスト")
	head := findText(t, tt, "リスト")
	if y := core.GlobalOffset(head).Y; y < 0 || y > 40 {
		t.Errorf("after the outline the heading is at %v", y)
	}
	// A drag from the heading into the code block selects the list between.
	code := findText(t, tt, "package main")
	drag(tt, caretAt(head, 0), caretAt(code, len("package main\n\nimport")))
	shot("markdown-selection.png")

	// The outline again, to a heading further down.
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 100, Y: 740}, Button: input.ButtonPrimary, Time: 90000})
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 100, Y: 740}, Button: input.ButtonPrimary, Time: 90000})
	press(t, tt, "表")
	if y := core.GlobalOffset(findText(t, tt, "表")).Y; y < 0 || y > 40 {
		t.Errorf("after the outline the heading 表 is at %v", y)
	}
	shot("markdown-jump.png")
}

// press presses the outline button labeled label.
func press(t *testing.T, tt *gowakutest.Tester, label string) {
	t.Helper()
	for _, n := range tt.FindAll(label) {
		if n.Role == semantics.RoleButton {
			tt.TapAt(n.Rect.Center())
			tt.Pump()
			return
		}
	}
	t.Fatalf("no button %q", label)
}

// findText returns the text of the document that starts with s: a
// SelectableText other than a basic.Text, such as the labels of the outline.
func findText(t *testing.T, tt *gowakutest.Tester, s string) basic.SelectableText {
	t.Helper()
	var found basic.SelectableText
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		_, plain := ro.(*basic.RenderParagraph)
		if p, ok := ro.(basic.SelectableText); ok && !plain && found == nil && p.TextLayout() != nil && strings.HasPrefix(p.Text(), s) {
			found = p
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(tt.Tree().RootRenderObject())
	if found == nil {
		t.Fatalf("no text %q", s)
	}
	return found
}

func caretAt(p basic.SelectableText, off int) geom.Offset {
	c := p.TextLayout().CaretRect(off).Translate(p.TextOrigin())
	return core.LocalToGlobal(p, geom.Offset{X: c.X + c.W/2, Y: c.Y + c.H/2})
}

func drag(tt *gowakutest.Tester, from, to geom.Offset) {
	tree := tt.Tree()
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: from, Button: input.ButtonPrimary, Time: 50000})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: to, Button: input.ButtonPrimary, Time: 50100})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: to, Button: input.ButtonPrimary, Time: 50200})
	tt.Pump()
}
