package main

import (
	"encoding/json"
	"fmt"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/editor"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// The dock page: an IDE-like DockLayout with files at the left, two editors
// and a preview, whose tab is italic, with a console below them in the middle, properties at the right, and an
// outline hidden on the strip along the right edge. The console holds a dock
// of its own, with the output, the problems and a terminal. The buttons keep
// both arrangements as JSON in memory and bring them back. Closing an editor
// whose text changed asks first.

// pageDock returns the dock page.
func pageDock() core.Widget { return dockPage{} }

// dockDefault is where the panels of the page start.
func dockDefault() nav.DockArrangement {
	return nav.DockArrangement{Root: nav.DockNode{
		Weights: []float32{1, 3, 1},
		Children: []nav.DockNode{
			{Panels: []string{"files"}},
			{Axis: layout.Vertical, Weights: []float32{3, 2}, Children: []nav.DockNode{
				{Panels: []string{"main.go", "dock.go", "README.md"}, Active: "main.go"},
				{Panels: []string{"console"}},
			}},
			{Panels: []string{"props"}},
		},
	}, Hidden: []nav.DockHidden{{ID: "outline", Side: nav.DockRight, Size: 240}}}
}

// consoleDefault is where the panels of the dock in the console start.
func consoleDefault() nav.DockArrangement {
	return nav.DockArrangement{Root: nav.DockNode{
		Weights: []float32{3, 2},
		Children: []nav.DockNode{
			{Panels: []string{"output", "problems"}, Active: "output"},
			{Panels: []string{"terminal"}},
		},
	}}
}

// dockSaved is what the save button keeps: the arrangements of both docks,
// each from its own controller.
type dockSaved struct {
	Outer   nav.DockArrangement `json:"outer"`
	Console nav.DockArrangement `json:"console"`
}

type dockPage struct{ core.StatefulBase }

type dockPageState struct {
	core.StateBase
	dock, console nav.DockController
	main, doc     editor.Controller
	// saved is the arrangement kept by the save button, and status what the
	// last button did.
	saved  []byte
	status string
	// mainText and docText are what the editors started with, which tell
	// whether closing one loses changes.
	mainText, docText string
}

// CreateState implements core.StatefulWidget.
func (dockPage) CreateState() core.State { return &dockPageState{} }

// InitState implements core.State.
func (s *dockPageState) InitState() {
	s.main.SetText("package main\n\nimport \"github.com/bankai-tensei/gowaku\"\n\nfunc main() {\n\tgowaku.Run(gowaku.App{Title: \"ドック\"})\n}\n")
	s.doc.SetText("// タブを他の群や端へドラッグすると、パネルが移ります。\n// 移しても、ここに打った文字は消えません。\n// 書き換えてから閉じると、閉じる前に聞きます。\n")
	s.mainText, s.docText = s.main.Text(), s.doc.Text()
	s.status = "タブをドラッグして配置を変えられます"
}

func (s *dockPageState) save() {
	data, err := json.Marshal(dockSaved{Outer: s.dock.Arrangement(), Console: s.console.Arrangement()})
	s.SetState(func() {
		if err != nil {
			s.status = "保存できませんでした: " + err.Error()
			return
		}
		s.saved = data
		s.status = fmt.Sprintf("配置を保存しました（%d バイト）", len(data))
	})
}

func (s *dockPageState) restore() {
	if s.saved == nil {
		s.SetState(func() { s.status = "まだ保存していません" })
		return
	}
	var a dockSaved
	if err := json.Unmarshal(s.saved, &a); err != nil {
		s.SetState(func() { s.status = "復元できませんでした: " + err.Error() })
		return
	}
	s.dock.SetArrangement(a.Outer)
	s.console.SetArrangement(a.Console)
	s.SetState(func() { s.status = "保存した配置に戻しました" })
}

func (s *dockPageState) reset() {
	s.dock.Reset()
	s.console.Reset()
	s.SetState(func() { s.status = "初期配置に戻しました" })
}

// closeRequest lets an editor close at once while its text is as it began,
// and otherwise asks first: the panel then closes only from the dialog.
func (s *dockPageState) closeRequest(id string) bool {
	var c *editor.Controller
	var start string
	switch id {
	case "main.go":
		c, start = &s.main, s.mainText
	case "dock.go":
		c, start = &s.doc, s.docText
	}
	if c == nil || c.Text() == start {
		return true
	}
	overlay.ShowDialog(s.Context(), func(dismiss func()) core.Widget {
		return overlay.Dialog{
			Title:   id + " の変更を破棄しますか",
			Message: "閉じると、打った文字は失われます。",
			Actions: []core.Widget{
				control.Button{Label: "キャンセル", OnPress: dismiss},
				control.Button{Label: "破棄して閉じる", Style: control.ButtonPrimary, OnPress: func() {
					dismiss()
					c.SetText(start)
					s.dock.Close(id)
					s.SetState(func() { s.status = id + " を閉じました（変更は破棄しました）" })
				}},
			},
		}
	})
	return false
}

// outlineMenu is the context menu of the outline. ctx is that of the panel,
// so that the dialog belongs to it.
func (s *dockPageState) outlineMenu(ctx core.BuildContext) []overlay.MenuItem {
	return []overlay.MenuItem{
		{Action: interact.Action{Label: "main へ移動", OnInvoke: func() {
			s.SetState(func() { s.status = "main へ移動しました" })
		}}},
		{Action: interact.Action{Label: "情報…", OnInvoke: func() {
			overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
				return overlay.Dialog{
					Title:   "アウトライン",
					Message: "メニューやダイアログを開いても、出ているパネルは隠れません。",
					Actions: []core.Widget{control.Button{Label: "OK", Style: control.ButtonPrimary, OnPress: dismiss}},
				}
			})
		}}},
	}
}

func (s *dockPageState) changed(nav.DockArrangement) {
	s.SetState(func() { s.status = "配置が変わりました（保存はまだです）" })
}

// Build implements core.State.
func (s *dockPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	panels := []nav.DockPanel{
		{ID: "files", Title: "ファイル", Build: func(core.BuildContext) core.Widget { return dockFiles(th) }},
		{ID: "main.go", Title: "main.go", Closable: true, Build: func(core.BuildContext) core.Widget {
			return editor.Editor{Controller: &s.main, Label: "main.go", LineNumbers: true}
		}},
		{ID: "dock.go", Title: "dock.go", Closable: true, Build: func(core.BuildContext) core.Widget {
			return editor.Editor{Controller: &s.doc, Label: "dock.go", LineNumbers: true}
		}},
		// A preview, as an editor shows a file opened with one click until
		// it is edited: its tab is italic.
		{ID: "README.md", Title: "README.md", Closable: true, Italic: true, Build: func(core.BuildContext) core.Widget {
			return dockNote(th, "# gowaku\n\nプレビューのタブは斜体で表します。")
		}},
		{ID: "console", Title: "コンソール", Build: func(core.BuildContext) core.Widget { return s.consoleDock(th) }},
		{ID: "props", Title: "プロパティ", Closable: true, Build: func(core.BuildContext) core.Widget {
			return dockNote(th, "名前: main.go\n大きさ: 128 バイト\n文字コード: UTF-8")
		}},
		// The outline slides out from its strip; its context menu, and the
		// dialog one of its items opens, keep it out.
		{ID: "outline", Title: "アウトライン", Build: func(ctx core.BuildContext) core.Widget {
			return overlay.ContextMenu{Items: s.outlineMenu(ctx), Child: dockNote(th, "package main\n  import \"gowaku\"\n  func main()")}
		}},
	}
	return basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Padding{Insets: geom.Symmetric(theme.Space16, theme.Space12), Child: basic.Row{Spacing: theme.Space8, Cross: layout.CrossCenter, Children: []core.Widget{
			control.Button{Label: "配置を保存", Style: control.ButtonPrimary, OnPress: s.save},
			control.Button{Label: "保存した配置に戻す", OnPress: s.restore},
			control.Button{Label: "初期配置", Style: control.ButtonQuiet, OnPress: s.reset},
			basic.Expanded{Child: basic.Text{Content: s.status, Style: th.Text.Caption}},
		}}},
		basic.Expanded{Child: nav.DockLayout{
			Panels: panels, Controller: &s.dock, Default: dockDefault(), Label: "パネル", AutoHide: true,
			OnChange: s.changed, OnCloseRequest: s.closeRequest,
		}},
	}}
}

// consoleDock is the dock within the console panel, with an arrangement and a
// controller of its own.
func (s *dockPageState) consoleDock(th *theme.Theme) core.Widget {
	return nav.DockLayout{
		Panels: []nav.DockPanel{
			{ID: "output", Title: "出力", Build: func(core.BuildContext) core.Widget {
				return dockNote(th, "$ go build ./...\nビルドに成功しました。")
			}},
			{ID: "problems", Title: "問題", Build: func(core.BuildContext) core.Widget {
				return dockNote(th, "問題はありません。")
			}},
			{ID: "terminal", Title: "ターミナル", Closable: true, Build: func(core.BuildContext) core.Widget {
				return dockNote(th, "$ go test ./widgets/nav/\nok")
			}},
		},
		Controller: &s.console, Default: consoleDefault(), Label: "コンソール", AutoHide: true,
		OnChange: s.changed,
	}
}

// dockFiles is the file tree of the page, kept plain.
func dockFiles(th *theme.Theme) core.Widget {
	names := []string{"gowaku/", "  go.mod", "  main.go", "  dock.go", "  docs/", "    ARCHITECTURE.md"}
	rows := make([]core.Widget, len(names))
	for i, n := range names {
		rows[i] = basic.Text{Content: n, Style: th.Text.Body}
	}
	return basic.Padding{Insets: geom.All(theme.Space12), Child: basic.Column{Spacing: theme.Space4, Children: rows}}
}

// dockNote is a panel of plain lines.
func dockNote(th *theme.Theme, s string) core.Widget {
	return basic.Padding{Insets: geom.All(theme.Space12), Child: basic.Text{Content: s, Style: th.Text.Body}}
}
