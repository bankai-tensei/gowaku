package main

import (
	"fmt"
	"slices"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/notes/store"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
)

// autosaveChoices are the intervals the settings offer.
func autosaveChoices() []time.Duration {
	return []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, time.Minute}
}

// openSettings shows the settings over the window. They apply when saved.
func (s *windowState) openSettings() {
	a := s.Widget().model.app
	overlay.ShowDialog(s.Context(), func(dismiss func()) core.Widget {
		return overlay.Dialog{Title: "設定", Width: 480, Content: settingsBody{app: a, dismiss: dismiss}}
	})
}

// settingsBody is the form of the settings.
type settingsBody struct {
	core.StatefulBase
	app     *app
	dismiss func()
}

type settingsState struct {
	core.StateOf[settingsBody]
	draft store.Settings
}

// CreateState implements core.StatefulWidget.
func (settingsBody) CreateState() core.State { return &settingsState{} }

// InitState implements core.State.
func (s *settingsState) InitState() { s.draft = s.Widget().app.settings }

// chooseFolder asks for the folder of the notebook.
func (s *settingsState) chooseFolder() {
	a := s.Widget().app
	d := gowaku.FileDialog{Title: "ノート帳のフォルダ", Directory: true, Folder: s.draft.Library}
	a.shell.fileDialog(d, func(paths []string, err error) {
		if err == nil && len(paths) > 0 {
			s.SetState(func() { s.draft.Library = paths[0] })
		}
	})
}

// Build implements core.State.
func (s *settingsState) Build(ctx core.BuildContext) core.Widget {
	b := s.Widget()
	th := basic.ThemeOf(ctx)
	family := 0
	if s.draft.Family == store.Gothic {
		family = 1
	}
	choices := autosaveChoices()
	every := slices.Index(choices, s.draft.AutosaveEvery)
	if every < 0 {
		every = 0
	}
	labels := []string{"5 秒", "15 秒", "30 秒", "1 分"}
	size := fmt.Sprintf("%.0f", s.draft.FontSize)
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
		collection.Group{Title: "文字", Children: []core.Widget{
			collection.GroupRow{Label: "大きさ", Child: basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space10, Children: []core.Widget{
				basic.SizedBox{Width: 160, Child: control.Slider{
					Value: s.draft.FontSize, Min: minFontSize, Max: maxFontSize, Step: 1, Label: "文字の大きさ",
					OnChange: func(v float32) { s.SetState(func() { s.draft.FontSize = v }) },
				}},
				basic.SizedBox{Width: 24, Child: basic.Text{Content: size, Style: colored(th.Text.Body, th.Color.Ink2)}},
			}}},
			collection.GroupRow{Label: "書体", Child: control.SegmentedControl{
				Options: []string{"明朝", "ゴシック"}, Selected: family, Label: "書体",
				OnChange: func(i int) {
					s.SetState(func() { s.draft.Family = store.Mincho + store.Family(i) })
				},
			}},
		}},
		collection.Group{Title: "ノート帳", Children: []core.Widget{
			collection.GroupRow{Label: "フォルダ", Detail: truncate(s.draft.Library, 34), Child: control.Button{
				Label: "選ぶ…", Size: control.ButtonSmall, OnPress: s.chooseFolder,
			}},
			collection.GroupRow{Label: "自動保存", Child: control.SegmentedControl{
				Options: labels, Selected: every, Label: "自動保存の間隔",
				OnChange: func(i int) { s.SetState(func() { s.draft.AutosaveEvery = choices[i] }) },
			}},
		}},
		basic.Padding{Insets: geom.Insets{Top: theme.Space4}, Child: basic.Row{Main: layout.MainEnd, Spacing: theme.Space8, Children: []core.Widget{
			control.Button{Label: "キャンセル", OnPress: b.dismiss},
			control.Button{Label: "保存", Style: control.ButtonPrimary, OnPress: func() {
				b.app.setSettings(s.draft)
				b.dismiss()
			}},
		}}},
	}}
}
