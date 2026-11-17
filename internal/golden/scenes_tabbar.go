package golden

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// tabBarScenes returns the scenes of TabBar: a bar with the pointer over a
// tab, one that overflows, and one with a tab dragged halfway. The dark one
// also has the keyboard on the first bar.
func tabBarScenes() []Scene {
	return []Scene{
		{Name: "tabbar", Width: 720, Height: 380, Scale: 1, Root: tabBars, Act: hoverTab},
		{Name: "tabbar-dark", Width: 1080, Height: 570, Scale: 1.5, Dark: true, Root: tabBars, Act: hoverTab},
		{Name: "tabbar-drag", Width: 720, Height: 380, Scale: 1, Root: tabBars, Act: dragTab},
		{Name: "tabbar-drag-dark", Width: 1080, Height: 570, Scale: 1.5, Dark: true, Root: tabBars, Act: dragTab},
	}
}

// The places of the bars of tabBars, in logical pixels.
const (
	tabScenePad     = 20
	tabSceneContent = 60
	tabSceneGap     = 24
	tabSceneBar     = 34
)

// tabSceneTop returns the top of bar i.
func tabSceneTop(i int) float32 {
	return tabScenePad + float32(i)*(tabSceneBar+tabSceneContent+tabSceneGap)
}

// hoverTab puts the pointer over the third tab of the first bar.
func hoverTab(tree *core.Tree, _ core.BuildContext) {
	// Four tabs share 680 less the new tab button: 162 each.
	at := geom.Offset{X: tabScenePad + 162*2 + 80, Y: tabSceneTop(0) + 20}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
}

// dragTab drags the first tab of the last bar past the middle of the second
// without letting go, and gives the first bar the keyboard.
func dragTab(tree *core.Tree, _ core.BuildContext) {
	move := func(kind core.PointerKind, x, y float32) {
		tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: geom.Offset{X: x, Y: y}})
	}
	// Four tabs share 680: 170 each.
	y := tabSceneTop(2) + 20
	move(core.PointerMove, tabScenePad+85, y)
	move(core.PointerDown, tabScenePad+85, y)
	move(core.PointerMove, tabScenePad+90, y)
	move(core.PointerMove, tabScenePad+85+120, y+4)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
}

// tabDocIcon returns a page with a folded corner, on the 24 unit grid.
func tabDocIcon() *paint.Path {
	var p paint.Path
	p.MoveTo(6, 3)
	p.LineTo(14, 3)
	p.LineTo(19, 8)
	p.LineTo(19, 21)
	p.LineTo(6, 21)
	p.Close()
	p.MoveTo(14, 3)
	p.LineTo(14, 8)
	p.LineTo(19, 8)
	return p.Stroke(paint.StrokeStyle{Width: 1.6, Join: paint.JoinRound})
}

func tabBars() core.Widget {
	icon := tabDocIcon()
	doc := func(titles ...string) []nav.TabItem {
		out := make([]nav.TabItem, len(titles))
		for i, t := range titles {
			out[i] = nav.TabItem{Key: t, Label: t, Icon: icon}
		}
		return out
	}
	many := make([]string, 12)
	for i := range many {
		many[i] = fmt.Sprintf("chapter-%02d.md", i+1)
	}
	none := func(int) {}
	section := func(bar nav.TabBar, width float32) core.Widget {
		return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
			th := basic.ThemeOf(ctx)
			return basic.SizedBox{Width: width, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{
				bar,
				basic.Container{Height: tabSceneContent, Color: th.Color.Card},
			}}}
		}}
	}
	return basic.Padding{Insets: geom.All(tabScenePad), Child: basic.Column{
		Spacing: tabSceneGap, Cross: layout.CrossStart, Children: []core.Widget{
			section(nav.TabBar{
				Tabs: doc("README.md", "main.go", "A very long name that does not fit.txt", "go.mod"), Selected: 1,
				Label: "Files", OnSelect: none, OnClose: none, OnNew: func() {},
			}, 680),
			section(nav.TabBar{
				Tabs: doc(many...), Selected: 11, Label: "Chapters", OnSelect: none, OnClose: none,
			}, 520),
			section(nav.TabBar{
				Tabs:     []nav.TabItem{{Key: 1, Label: "Inbox"}, {Key: 2, Label: "Drafts"}, {Key: 3, Label: "Sent"}, {Key: 4, Label: "Archive"}},
				Label:    "Mail",
				OnSelect: none, OnClose: none, OnMove: func(int, int) {},
			}, 680),
		},
	}}
}
