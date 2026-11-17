package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// settingsPage is a page of settings with an overflowing row and an unlabeled
// control, and a button that opens a dialog, with faint text if faint is set.
type settingsPage struct {
	gowaku.StatelessBase
	faint bool
}

func (p settingsPage) Build(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.VStack(
		collection.Group{Title: "General", Children: []gowaku.Widget{
			collection.GroupRow{Label: "Name", Detail: "Shown to others"},
		}},
		basic.Row{Children: []gowaku.Widget{basic.SizedBox{Width: 900, Height: 4}}},
		interact.Pressable{OnPress: func() {}, Child: basic.SizedBox{Width: 20, Height: 20}},
		control.Button{Label: "Open", OnPress: func() {
			overlay.ShowDialog(ctx, func(dismiss func()) gowaku.Widget {
				var content gowaku.Widget
				if p.faint {
					content = basic.Text{Content: "faint", Style: ink(geom.Hex(0xcccccc))}
				}
				return overlay.Dialog{Title: "Settings", Content: content, Actions: []gowaku.Widget{
					control.Button{Label: "Done", OnPress: dismiss},
				}}
			})
		}},
	)
}

// TestWarnings_SkipWhatIsUnderADialog: the scrim dims the page under a dialog
// on purpose, and the page is out of the semantics tree, so its text, its
// overflow and its controls are not checked while the dialog is open.
func TestWarnings_SkipWhatIsUnderADialog(t *testing.T) {
	tt := gowakutest.New(t, settingsPage{}, goFonts)
	before := kinds(tt.Warnings())
	if before[gowakutest.WarnOverflow] != 1 || before[gowakutest.WarnUnlabeled] != 1 || before[gowakutest.WarnContrast] != 0 {
		t.Fatalf("before the dialog: got %v, want one overflow and one unlabeled", before)
	}
	tt.Tap("Open")
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("with the dialog open: got %v, want none", ws)
	}
	tt.Tap("Done")
	if after := kinds(tt.Warnings()); after[gowakutest.WarnOverflow] != 1 || after[gowakutest.WarnUnlabeled] != 1 {
		t.Errorf("after the dialog: got %v, want the page checked again", after)
	}
}

// TestWarnings_ChecksTheDialog: what the dialog shows is still checked.
func TestWarnings_ChecksTheDialog(t *testing.T) {
	tt := gowakutest.New(t, settingsPage{faint: true}, goFonts)
	tt.Tap("Open")
	ws := tt.Warnings()
	if len(ws) != 1 || ws[0].Kind != gowakutest.WarnContrast || ws[0].Text != "faint" {
		t.Errorf("got %v, want the faint text of the dialog only", ws)
	}
}
