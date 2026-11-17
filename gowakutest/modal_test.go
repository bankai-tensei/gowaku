package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// resets counts the presses of the two Reset buttons of resetPage.
type resets struct{ page, dialog int }

// resetPage has a Reset button, and a dialog with a Reset button of its own.
type resetPage struct {
	gowaku.StatelessBase
	got *resets
}

func (p resetPage) Build(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: basic.VStack(
		control.Button{Label: "Reset", OnPress: func() { p.got.page++ }},
		control.Button{Label: "Settings", OnPress: func() {
			overlay.ShowDialog(ctx, func(dismiss func()) gowaku.Widget {
				return overlay.Dialog{Title: "Settings", Actions: []gowaku.Widget{
					control.Button{Label: "Notify", OnPress: func() {
						overlay.ShowToast(ctx, overlay.Toast{Message: "Saved", Action: "Undo"})
					}},
					control.Button{Label: "Reset", OnPress: func() {
						p.got.dialog++
						dismiss()
					}},
				}}
			})
		}},
	)}
}

func TestModal_FindAndTapStayInTheDialog(t *testing.T) {
	var got resets
	tt := gowakutest.New(t, resetPage{got: &got}, goFonts)
	page, ok := tt.Find("Reset")
	if !ok {
		t.Fatal("no Reset on the page")
	}
	pageRect := page.Rect

	tt.Tap("Settings")
	if _, ok := tt.Find("Notify"); !ok {
		t.Fatalf("no dialog:\n%s", tt.Dump())
	}
	if all := tt.FindAll("Reset"); len(all) != 1 || all[0].Rect == pageRect {
		t.Fatalf("the Reset under the dialog is found:\n%s", tt.Dump())
	}
	if n, ok := tt.Find("Settings"); !ok || n.Role != semantics.RoleDialog {
		t.Errorf("the Settings button under the dialog is found:\n%s", tt.Dump())
	}
	tt.Tap("Notify")
	if _, ok := tt.Find("Undo"); !ok {
		t.Errorf("a toast over the dialog must stay in the tree:\n%s", tt.Dump())
	}

	tt.Tap("Reset")
	if got.dialog != 1 || got.page != 0 {
		t.Fatalf("pressed the page %d times and the dialog %d times, want the dialog once", got.page, got.dialog)
	}
	if n, ok := tt.Find("Reset"); !ok || n.Rect != pageRect {
		t.Fatalf("the page must come back when the dialog closes:\n%s", tt.Dump())
	}
	tt.Tap("Reset")
	if got.page != 1 {
		t.Errorf("pressed the page %d times, want once", got.page)
	}
}
