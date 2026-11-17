package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// navigatorFrames is how many frames after the push the mid-motion scenes
// take their picture: 80 ms of 280, a little past half-way on the curve.
const navigatorFrames = 6

// navigatorScenes returns the scenes of Navigator, the page transitions and Hero:
// a list of notes half-way through a push to a plain page, sliding and fading
// in; the same push to the detail of a note, whose icon flies from its row;
// and that detail at rest, with its BackButton.
func navigatorScenes() []Scene {
	s := make([]Scene, 0, 6)
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		s = append(s,
			navigatorScene("navigator-push"+suffix, dark, navigatorFrames, func(c *nav.NavigatorController) {
				c.Push(nav.PageRoute{Name: "/settings", Page: settingsPage{}})
			}),
			navigatorScene("navigator-hero"+suffix, dark, navigatorFrames, func(c *nav.NavigatorController) {
				c.PushNamed("/notes/2")
			}),
			navigatorScene("navigator-detail"+suffix, dark, 0, func(c *nav.NavigatorController) {
				c.PushNamed("/notes/2")
			}),
		)
	}
	return s
}

// navigatorScene is the notes app of the scenes, with act run on its
// Navigator after the first frame.
func navigatorScene(name string, dark bool, frames int, act func(c *nav.NavigatorController)) Scene {
	c := &nav.NavigatorController{}
	return Scene{
		Name: name, Width: 900, Height: 600, Scale: 1.5, Dark: dark, Frames: frames,
		Root: func() core.Widget { return notesNavigator(c) },
		Act:  func(*core.Tree, core.BuildContext) { act(c) },
	}
}

// sceneNote is a note of the list.
type sceneNote struct {
	title, preview string
	hue            geom.Color
}

var sceneNotes = []sceneNote{
	{"Groceries", "Rice, miso, green onions, tofu", geom.RGBA(0.30, 0.62, 0.42, 1)},
	{"Trip to Kyoto", "Leave Friday at nine. Book the ryokan near Gion", geom.RGBA(0.86, 0.45, 0.28, 1)},
	{"Reading list", "The Pillow Book, Kokoro, Snow Country", geom.RGBA(0.36, 0.48, 0.82, 1)},
	{"Ideas", "A quiet timer that fades the screen", geom.RGBA(0.64, 0.42, 0.74, 1)},
}

func notesNavigator(c *nav.NavigatorController) core.Widget {
	return nav.Navigator{
		Controller: c,
		Home:       noteList{},
		Routes: []nav.RoutePattern{{Pattern: "/notes/:id", Build: func(p nav.RouteParams) core.Widget {
			i, _ := p.Int("id")
			return noteDetail{index: i}
		}}},
	}
}

// noteIcon is the square of a note, with its initial.
func noteIcon(n sceneNote, side float32) core.Widget {
	style := role(theme.Default().Light.Text.Headline)
	style.Size = side * 0.5
	style.Color = geom.RGBA(1, 1, 1, 1)
	return basic.Container{
		Width: side, Height: side, Radius: side / 4, Color: n.hue,
		Child: basic.Center{Child: basic.Text{Content: n.title[:1], Style: style}},
	}
}

// noteList is the first page: a title over the notes, each with its icon as a
// Hero.
type noteList struct{ core.StatelessBase }

// Build implements core.StatelessWidget.
func (noteList) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	rows := make([]core.Widget, 0, 1+len(sceneNotes))
	rows = append(rows,
		basic.Padding{Insets: geom.Insets{Left: theme.Space16, Top: theme.Space16, Bottom: theme.Space8}, Child: basic.Text{
			Content: "Notes", Style: role(th.Text.LargeTitle),
		}},
	)
	for i, n := range sceneNotes {
		caption := th.Text.Caption
		caption.Color = th.Color.Ink2
		rows = append(rows, basic.Padding{Insets: geom.Symmetric(theme.Space16, theme.Space8), Child: basic.Row{
			Cross: layout.CrossCenter, Spacing: theme.Space12, Children: []core.Widget{
				nav.Hero{Tag: i, Child: noteIcon(n, 36)},
				basic.Expanded{Child: basic.Column{Spacing: theme.Space2, Children: []core.Widget{
					basic.Text{Content: n.title, Style: role(th.Text.Headline)},
					basic.Text{Content: n.preview, Style: caption},
				}}},
			},
		}})
	}
	return basic.Container{Color: th.Color.Canvas, Child: basic.Column{Cross: layout.CrossStretch, Children: rows}}
}

// noteDetail is the page of one note: a bar with the way back, the icon large,
// and the text.
type noteDetail struct {
	core.StatelessBase
	index int
}

// Build implements core.StatelessWidget.
func (d noteDetail) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	n := sceneNotes[d.index]
	body := th.Text.Reading
	body.Color = th.Color.Ink2
	return basic.Container{Color: th.Color.Card, Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Padding{Insets: geom.Symmetric(theme.Space8, theme.Space8), Child: basic.Row{Children: []core.Widget{
			nav.BackButton{Label: "Notes"},
		}}},
		basic.Padding{Insets: geom.Symmetric(theme.Space24, theme.Space8), Child: basic.Row{
			Cross: layout.CrossCenter, Spacing: theme.Space16, Children: []core.Widget{
				nav.Hero{Tag: d.index, Child: noteIcon(n, 72)},
				basic.Text{Content: n.title, Style: role(th.Text.LargeTitle)},
			},
		}},
		basic.Padding{Insets: geom.Symmetric(theme.Space24, theme.Space8), Child: basic.Text{Content: n.preview + ". Check the timetable the night before.", Style: body}},
	}}}
}

// settingsPage is a plain page for the transition without heroes.
type settingsPage struct{ core.StatelessBase }

// Build implements core.StatelessWidget.
func (settingsPage) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{Color: th.Color.Card, Child: basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Column{
		Spacing: theme.Space12, Children: []core.Widget{
			basic.Text{Content: "Settings", Style: role(th.Text.LargeTitle)},
			settingRow("Sync notes", true),
			settingRow("Vertical writing", false),
			control.Button{Label: "Export all notes", OnPress: func() {}},
		},
	}}}
}

// settingRow is a switch with its label beside it.
func settingRow(label string, on bool) core.Widget {
	return basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space12, Children: []core.Widget{
		control.Switch{Value: on, Label: label, OnChange: func(bool) {}},
		basic.Text{Content: label},
	}}
}
