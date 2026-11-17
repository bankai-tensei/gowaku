package main

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The page of the bars: InfoBar, StatusBar and CommandPalette.

// pageBars returns the page of InfoBar, StatusBar and CommandPalette.
func pageBars() core.Widget { return barsPage{} }

// barsPage shows the four severities of InfoBar, a StatusBar along its
// bottom, and a CommandPalette over a few commands.
type barsPage struct{ core.StatefulBase }

type barsPageState struct {
	core.StateBase
	// open says which of the four bars show, by severity.
	open     [4]bool
	vertical bool
	saved    int
	last     string
}

// CreateState implements core.StatefulWidget.
func (barsPage) CreateState() core.State {
	return &barsPageState{open: [4]bool{true, true, true, true}}
}

// ran records the command that ran last, which the status bar shows.
func (s *barsPageState) ran(label string, also func()) func() {
	return func() {
		s.SetState(func() {
			s.last = label
			if also != nil {
				also()
			}
		})
	}
}

// actions are the commands of the page, which the palette finds.
func (s *barsPageState) actions() []interact.Action {
	ctrl := input.ModControl
	return []interact.Action{
		{Label: "新規作成(&N)", Shortcut: interact.Shortcut{Key: input.KeyN, Modifiers: ctrl}, OnInvoke: s.ran("新規作成", nil)},
		{Label: "上書き保存(&S)", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: ctrl}, OnInvoke: s.ran("上書き保存", func() { s.saved++ })},
		{Label: "名前を付けて保存(&A)…", OnInvoke: s.ran("名前を付けて保存", nil)},
		// 書き出し has no OnInvoke, so the palette shows it grayed out.
		{Label: "書き出し(&E)…"},
		{Label: "縦書き(&V)", Checkable: true, Checked: s.vertical, OnInvoke: s.ran("縦書き", func() { s.vertical = !s.vertical })},
		{Label: "すべての通知を表示", OnInvoke: s.ran("すべての通知を表示", func() { s.open = [4]bool{true, true, true, true} })},
		{Label: "すべての通知を閉じる", OnInvoke: s.ran("すべての通知を閉じる", func() { s.open = [4]bool{} })},
	}
}

// Build implements core.State.
func (s *barsPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	acts := s.actions()
	bars := []struct {
		severity control.InfoSeverity
		name     string
		bar      control.InfoBar
	}{
		{control.SeverityInfo, "情報", control.InfoBar{Title: "新しい版があります", Message: "再起動すると更新されます。", Action: "再起動"}},
		{control.SeveritySuccess, "成功", control.InfoBar{Title: "保存しました", Message: "「京都の旅.txt」に書き込みました。"}},
		{control.SeverityWarning, "警告", control.InfoBar{
			Title:   "ディスクの空きが少なくなっています",
			Message: "残りは 1.2 GB です。不要なファイルを削除するか、別の場所に保存してください。長い文はこのように折り返します。",
			Action:  "場所を変更",
		}},
		{control.SeverityError, "エラー", control.InfoBar{Title: "保存できませんでした", Message: "ネットワークに接続できません。", Action: "再試行"}},
	}
	toggles := make([]core.Widget, 0, len(bars))
	shown := make([]core.Widget, 0, len(bars))
	for i, b := range bars {
		toggles = append(toggles, basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []core.Widget{
			control.Switch{Value: s.open[i], Label: b.name, OnChange: func(v bool) { s.SetState(func() { s.open[i] = v }) }},
			basic.Text{Content: b.name, Style: barsStyle(th.Text.Body, th.Color.Ink)},
		}})
		bar := b.bar
		bar.Open, bar.Severity = s.open[i], b.severity
		bar.OnClose = func() { s.SetState(func() { s.open[i] = false }) }
		if bar.Action != "" {
			bar.OnAction = s.ran(bar.Action, nil)
		}
		shown = append(shown, bar)
	}

	status := nav.StatusBar{
		Leading: []nav.StatusItem{
			{Text: "行 12, 列 4", Priority: 3},
			{Text: "保存 " + strconv.Itoa(s.saved) + " 回", Priority: 1},
			{Text: "最後の操作: " + barsOrNone(s.last)},
		},
		Trailing: []nav.StatusItem{
			{Text: "UTF-8", Tooltip: "文字コード", OnPress: s.ran("文字コード", nil), Priority: 2},
			{Text: "LF", Tooltip: "改行コード"},
			{Text: barsWritingMode(s.vertical), Tooltip: "書字方向を切り替える", OnPress: s.ran("縦書き", func() { s.vertical = !s.vertical }), Priority: 1},
		},
	}

	page := basic.Scroll{Child: basic.Pad(theme.Space28, basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
			basic.Text{Content: "InfoBar", Style: barsStyle(th.Text.Title, th.Color.Ink)},
			basic.Wrap{Spacing: theme.Space20, RunSpacing: theme.Space8, Children: toggles},
			basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space8, Children: shown},
			basic.Text{Content: "CommandPalette", Style: barsStyle(th.Text.Title, th.Color.Ink)},
			basic.Text{
				Content: "Ctrl+Shift+P でも開きます。あいまい検索で、たとえば「ほぞん」ではなく「保存」や「名保」と打ちます。",
				Style:   barsStyle(th.Text.Body, th.Color.Ink2),
			},
			basic.Row{Size: layout.MainMin, Children: []core.Widget{basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
				return control.Button{Label: "コマンドパレットを開く", Style: control.ButtonPrimary, OnPress: func() { field.OpenCommandPalette(ctx) }}
			}}}},
		},
	})}

	return field.CommandPalette{
		Actions: acts, RecentFirst: true,
		Child: interact.Actions{Actions: acts, Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
			basic.Expanded{Child: page},
			status,
		}}},
	}
}

// barsStyle returns s in color.
func barsStyle(s text.Style, color geom.Color) text.Style {
	s.Color = color
	return s
}

func barsOrNone(s string) string {
	if s == "" {
		return "なし"
	}
	return s
}

func barsWritingMode(vertical bool) string {
	if vertical {
		return "縦書き"
	}
	return "横書き"
}
