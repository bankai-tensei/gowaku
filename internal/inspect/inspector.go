//go:build gowakudebug

package inspect

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// Inspector is the root of the Inspector window: a toolbar, the trees of the
// target window at the left, the details of the selection at the right, and
// the frames at the bottom.
type Inspector struct {
	core.StatefulBase
	Session *Session
}

// CreateState implements core.StatefulWidget.
func (Inspector) CreateState() core.State { return &inspectorState{} }

type inspectorState struct {
	core.StateOf[Inspector]
	cancel func()
	// target is the index in Host.Targets of the window shown.
	target int
	// tab is the tree shown: 0 widgets, 1 render objects, 2 semantics.
	tab int
	// split is the width of the trees in logical pixels.
	split float32
}

// framesHeight is the height of the frames at the bottom.
const framesHeight = 240

// InitState implements core.State.
func (s *inspectorState) InitState() {
	s.split = 460
	s.cancel = s.Widget().Session.Listen(func() { s.SetState(func() {}) })
}

// Dispose implements core.State.
func (s *inspectorState) Dispose() { s.cancel() }

// Target returns the window shown, or nil when there is none.
func (s *inspectorState) currentTarget() Target {
	ts := s.Widget().Session.Host().Targets()
	if len(ts) == 0 {
		return nil
	}
	// A pick in another window shows that window.
	if sel, _, _ := s.Widget().Session.Selection(); sel != nil {
		for i, t := range ts {
			if t == sel {
				s.target = i
			}
		}
	}
	s.target = min(s.target, len(ts)-1)
	return ts[s.target]
}

// Build implements core.State.
func (s *inspectorState) Build(ctx core.BuildContext) core.Widget {
	ses := s.Widget().Session
	str := stringsOf(ctx)
	th := basic.ThemeOf(ctx)
	t := s.currentTarget()

	var body core.Widget
	if t == nil {
		body = basic.Center{Child: basic.Text{Content: str.noWindow, Style: th.Text.Body.WithColor(th.Color.Ink2)}}
	} else {
		trees := control.Tabs{
			Label:    str.trees,
			Selected: s.tab,
			OnChange: func(i int) { s.SetState(func() { s.tab = i }) },
			Pages: []control.TabPage{
				{Label: str.widgetTree, Child: TreesPanel{Session: ses, Target: t, Kind: TreeWidgets}},
				{Label: str.renderTree, Child: TreesPanel{Session: ses, Target: t, Kind: TreeRenderObjects}},
				{Label: str.semanticsTree, Child: TreesPanel{Session: ses, Target: t, Kind: TreeSemantics}},
			},
		}
		top := control.SplitPane{
			First:    basic.Padding{Insets: geom.All(theme.Space8), Child: trees},
			Second:   DetailsPanel{Session: ses, Target: t},
			Position: s.split, Pixels: true, MinFirst: 240, MinSecond: 280,
			OnChange: func(p float32) { s.SetState(func() { s.split = p }) },
		}
		// The frames keep their height: the trees take what the window gains.
		body = basic.Column{
			Cross: layout.CrossStretch,
			Children: []core.Widget{
				basic.Expanded{Child: top},
				basic.Divider{},
				basic.SizedBox{Height: framesHeight, Child: FramesPanel{Session: ses}},
			},
		}
	}
	return basic.Container{
		Color: th.Color.Canvas,
		Child: basic.Column{
			Cross: layout.CrossStretch,
			Children: []core.Widget{
				s.toolbar(str),
				basic.Divider{},
				basic.Expanded{Child: body},
			},
		},
	}
}

func (s *inspectorState) toolbar(str *inspectStrings) core.Widget {
	ses := s.Widget().Session
	ts := ses.Host().Targets()
	items := []nav.ToolbarItem{
		{Action: interact.Action{Label: str.pick, Checkable: true, Checked: ses.Picking,
			OnInvoke: func() { ses.SetPicking(!ses.Picking) }}, Icon: iconPick, ShowLabel: true},
		{Separator: true},
		{Action: interact.Action{Label: str.bounds, Checkable: true, Checked: ses.ShowBounds,
			OnInvoke: func() { ses.SetShowBounds(!ses.ShowBounds) }}, Icon: iconBounds, ShowLabel: true},
		{Action: interact.Action{Label: str.repaints, Checkable: true, Checked: ses.ShowRepaints,
			OnInvoke: func() { ses.SetShowRepaints(!ses.ShowRepaints) }}, Icon: iconRepaints, ShowLabel: true},
		{Separator: true},
		{Action: interact.Action{Label: str.pause, Checkable: true, Checked: ses.Paused,
			OnInvoke: func() { ses.SetPaused(!ses.Paused) }}, Icon: iconPause, ShowLabel: true},
		{Spacer: true},
	}
	if len(ts) > 1 {
		options := make([]field.DropdownOption[int], len(ts))
		for i, t := range ts {
			options[i] = field.DropdownOption[int]{Value: i, Label: windowName(t, str)}
		}
		items = append(items, nav.ToolbarItem{
			Action: interact.Action{Label: str.window},
			Child: basic.SizedBox{Width: 220, Child: field.DropdownMenu[int]{
				Value: s.target, Options: options,
				OnChange: func(i int) {
					s.SetState(func() { s.target = i })
					ses.Select(nil, core.ElementView{})
				},
			}},
		})
	}
	return basic.Padding{
		Insets: geom.Symmetric(theme.Space8, theme.Space4),
		Child:  nav.Toolbar{Label: str.toolbar, Items: items},
	}
}

// windowName returns the title of t, or a placeholder for an untitled window.
func windowName(t Target, str *inspectStrings) string {
	if t.Title() != "" {
		return t.Title()
	}
	return str.untitled
}
