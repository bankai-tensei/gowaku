package golden

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// transientScenes returns the scenes of the transient widgets.
func transientScenes() []Scene {
	return []Scene{
		{Name: "tooltip", Width: 720, Height: 330, Scale: 1.5, Root: notesPage, Act: focusTooltip},
		{Name: "tooltip-dark", Width: 720, Height: 330, Scale: 1.5, Dark: true, Root: notesPage, Act: focusTooltip},
		{Name: "toasts", Width: 900, Height: 540, Scale: 1.5, Root: notesPage, Act: showToasts},
		{Name: "toasts-dark", Width: 900, Height: 540, Scale: 1.5, Dark: true, Root: notesPage, Act: showToasts},
		{Name: "sheet-bottom", Width: 1200, Height: 660, Scale: 1.5, Root: notesPage, Act: showBottomSheet},
		{Name: "sheet-bottom-dark", Width: 1200, Height: 660, Scale: 1.5, Dark: true, Root: notesPage, Act: showBottomSheet},
		{Name: "sheet-trailing", Width: 900, Height: 540, Scale: 1.5, Root: notesPage, Act: showTrailingSheet},
		{Name: "sheet-trailing-dark", Width: 900, Height: 540, Scale: 1.5, Dark: true, Root: notesPage, Act: showTrailingSheet},
	}
}

// notesPage is a toolbar with tooltips over a few lines of text.
func notesPage() core.Widget {
	tool := func(label, tip string) core.Widget {
		return overlay.Tooltip{Message: tip, Child: control.Button{Label: label, Style: control.ButtonQuiet, OnPress: func() {}}}
	}
	return basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Column{Spacing: theme.Space12, Children: []core.Widget{
		basic.Row{Spacing: theme.Space4, Children: []core.Widget{
			tool("Bold", "Bold (Ctrl+B)"),
			tool("Italic", "Italic (Ctrl+I)"),
			tool("Link", "Insert a link (Ctrl+K)"),
		}},
		basic.Text{Content: "Meeting notes", Style: role(theme.Default().Light.Text.Headline)},
		basic.Text{Content: "Ship the transient widgets: tooltips, toasts and sheets. Check them in light and dark."},
		basic.Row{Main: layout.MainEnd, Spacing: theme.Space8, Children: []core.Widget{
			control.Button{Label: "Share", OnPress: func() {}},
			control.Button{Label: "Save", Style: control.ButtonPrimary, OnPress: func() {}},
		}},
	}}}
}

// focusTooltip moves the keyboard to the first tool, whose tooltip shows at once.
func focusTooltip(tree *core.Tree, _ core.BuildContext) {
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
}

func showToasts(_ *core.Tree, ctx core.BuildContext) {
	// An hour keeps the toasts long after the scene comes to rest.
	overlay.ShowToast(ctx, overlay.Toast{Message: "Note saved", Duration: time.Hour})
	overlay.ShowToast(ctx, overlay.Toast{Message: "Moved 3 notes to the trash", Action: "Undo", OnAction: func() {}, Duration: time.Hour})
	overlay.ShowToast(ctx, overlay.Toast{Message: "Back online. Your changes are synced.", Duration: time.Hour})
}

func sheetContent(dismiss func()) core.Widget {
	return basic.Column{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
		basic.Text{Content: "Share note", Style: role(theme.Default().Light.Text.Title)},
		basic.Text{Content: "Anyone with the link can read this note."},
		basic.SizedBox{Height: theme.Space8},
		basic.Row{Main: layout.MainEnd, Spacing: theme.Space8, Children: []core.Widget{
			control.Button{Label: "Cancel", OnPress: dismiss},
			control.Button{Label: "Copy link", Style: control.ButtonPrimary, OnPress: dismiss},
		}},
	}}
}

// role is a text style of the theme without its color, which the theme of the
// scene fills in.
func role(s text.Style) text.Style {
	s.Color = geom.Color{}
	return s
}

func showBottomSheet(_ *core.Tree, ctx core.BuildContext) {
	overlay.ShowSheet(ctx, overlay.Sheet{Label: "Share note"}, sheetContent)
}

func showTrailingSheet(_ *core.Tree, ctx core.BuildContext) {
	overlay.ShowSheet(ctx, overlay.Sheet{Edge: overlay.SheetTrailing, Label: "Share note"}, sheetContent)
}
