// Command gallery shows the widgets of gowaku in the design of docs/DESIGN.md, as
// a settings window. It follows the light and dark modes of the OS.
//
//	gallery                     opens the window
//	gallery -png out.png        draws one frame to a file and exits
//	gallery -png out.png -dark  the same in the dark
//	gallery -png out.png -state hover|focus|dialog|fields|parts|display|forms|date|submit|desktop|desktop-memo|others|navigation|navigation-detail
//	                            with the page in that state
//	gallery -png out.png -state disclosure|obscure|selection|grid|viewer|inputs|visual|bars|dock|editors|svg|lines
//	                            on that page; disclosure-end and the like
//	                            scroll it to its end
//	gallery -accent 0x0f766e    takes another accent color
//	gallery -locale ja|en       speaks that language; the pages are labeled in
//	                            Japanese, so the words of the library are
//	                            Japanese unless told otherwise, whatever the OS
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/icons"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// iconCloud is a shape that package icons lacks, from the path data of an
// SVG icon.
const iconCloud = "M19 18H6a4 4 0 0 1-.6-8A6 6 0 0 1 17 9a4.5 4.5 0 0 1 2 9z"

func path(data string) *paint.Path {
	p, err := paint.ParseSVGPath(data)
	if err != nil {
		log.Fatal(err)
	}
	return p
}

// gallery is the window. A locale other than the root one overrides that of the
// window for everything below, the dialogs and popups opened from there
// included: the pages are labeled in Japanese, so the words of the library
// are Japanese too unless -locale says otherwise, whatever the OS speaks. -png
// has no OS locale to follow at all.
type gallery struct {
	gowaku.StatelessBase
	locale language.Tag
}

// Build implements core.StatelessWidget.
func (g gallery) Build(ctx gowaku.BuildContext) gowaku.Widget {
	media := basic.MediaQueryOf(ctx)
	if g.locale != language.Und {
		media.Locale = g.locale
	}
	return basic.MediaQuery{Data: media, Child: galleryPages{}}
}

// galleryPages is the sidebar and the page under the MediaQuery of gallery.
type galleryPages struct{ gowaku.StatefulBase }

type galleryState struct {
	gowaku.StateBase
	page              int
	name, mail        string
	appearance        int
	notify, sound     bool
	volume            float32
	launch            bool
	moved             bool
	home, star, cloud *paint.Path
	plus              *paint.Path

	// The display and form pages.
	tab, plan          int
	bold, pinned       bool
	tags               map[string]bool
	query, fruit, city string
	date               time.Time
	clock              time.Duration
	form               field.FormController
	nick, code         string
	agree              bool

	// The desktop page.
	desk desktopState

	// The page of the other widgets and the navigation page.
	other otherState
}

// today is fixed so that screenshots stay the same from day to day.
var today = time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local)

func (galleryPages) CreateState() gowaku.State { return &galleryState{} }

func (s *galleryState) InitState() {
	s.name, s.mail = "山田 太郎", "taro@example.jp"
	s.appearance, s.notify, s.volume, s.launch = 2, true, 0.45, true
	s.home, s.star, s.cloud, s.plus = icons.Home(), icons.Star(), path(iconCloud), icons.Plus()
	s.tags = map[string]bool{"仕事": true}
	s.fruit, s.plan, s.date = "りんご", 1, today
	s.clock = 9*time.Hour + 30*time.Minute
}

func (s *galleryState) set(fn func()) { s.SetState(fn) }

func (s *galleryState) sidebar(th *theme.Theme) gowaku.Widget {
	item := func(i int, label string, icon *paint.Path) gowaku.Widget {
		return collection.SidebarItem{Label: label, Icon: icon, Selected: s.page == i, OnPress: func() { s.SetState(func() { s.page = i }) }}
	}
	pages := extraPages()
	items := make([]gowaku.Widget, 0, firstExtra+1+len(pages))
	items = append(items,
		item(0, "一般", s.home),
		item(1, "部品", s.star),
		item(2, "動き", s.cloud),
		item(3, "表示", s.home),
		item(4, "フォーム", s.star),
		item(5, "デスクトップ", s.cloud),
		item(6, "そのほか", s.home),
		item(7, "ナビゲーション", s.star),
	)
	section := th.Text.Caption
	section.Color = th.Color.Ink2
	items = append(items, basic.Padding{
		Insets: geom.Insets{Left: theme.Space10, Top: theme.Space16, Bottom: theme.Space4},
		Child:  basic.Text{Content: "ウィジェット", Style: section},
	})
	marks := []*paint.Path{s.home, s.star, s.cloud}
	for i, p := range pages {
		items = append(items, item(firstExtra+i, p.label, marks[i%len(marks)]))
	}
	return collection.Sidebar{Title: "gowaku", Children: items}
}

// extraPage is a page under the second heading of the sidebar. Each is a
// widget of its own that keeps its state, unlike the pages above it, which live
// in galleryState.
type extraPage struct {
	label, state string
	build        func() gowaku.Widget
	// body pages go in the scrolling column of the window, as the pages above
	// them do. The others scroll themselves or fill the window.
	body bool
}

// firstExtra is the index of the first extra page in the sidebar.
const firstExtra = 8

// extraPages are the extra pages in the order of the sidebar.
func extraPages() []extraPage {
	return []extraPage{
		{label: "開閉とリンク", state: "disclosure", build: pageDisclosure, body: true},
		{label: "パスワード欄", state: "obscure", build: pageObscure, body: true},
		{label: "テキストの選択", state: "selection", build: func() gowaku.Widget { return basic.Scroll{Child: pageSelection()} }},
		{label: "グリッドと並べ替え", state: "grid", build: pageGrid},
		{label: "パンとズーム", state: "viewer", build: pageViewer},
		{label: "数値とタグ", state: "inputs", build: pageInputs},
		{label: "色と読み込み", state: "visual", build: pageVisual},
		{label: "通知とバー", state: "bars", build: pageBars},
		{label: "ドック", state: "dock", build: pageDock},
		{label: "エディターのビュー", state: "editors", build: pageEditors},
		{label: "SVG の描画", state: "svg", build: pageSVG, body: true},
		{label: "差分とメニュー", state: "lines", build: pageLines},
	}
}

func heading(th *theme.Theme, title, detail string) gowaku.Widget {
	big, small := th.Text.LargeTitle, th.Text.Body
	big.Color, small.Color = th.Color.Ink, th.Color.Ink2
	return basic.Column{Size: layout.MainMin, Spacing: 2, Cross: layout.CrossStart, Children: []gowaku.Widget{
		basic.Text{Content: title, Style: big},
		basic.Text{Content: detail, Style: small},
	}}
}

func (s *galleryState) general(th *theme.Theme) []gowaku.Widget {
	textField := func(value string, change func(string)) gowaku.Widget {
		return basic.SizedBox{Width: 240, Child: field.Input{Value: value, OnChange: change}}
	}
	return []gowaku.Widget{
		heading(th, "一般", "名前、通知、表示をまとめて設定します。"),
		collection.Group{Title: "アカウント", Children: []gowaku.Widget{
			collection.GroupRow{Label: "名前", Child: textField(s.name, func(v string) { s.SetState(func() { s.name = v }) })},
			collection.GroupRow{Label: "メール", Child: textField(s.mail, func(v string) { s.SetState(func() { s.mail = v }) })},
		}},
		collection.Group{Title: "表示", Children: []gowaku.Widget{
			collection.GroupRow{Label: "外観", Child: control.SegmentedControl{
				Options: []string{"ライト", "ダーク", "自動"}, Selected: s.appearance, Label: "外観",
				OnChange: func(i int) { s.SetState(func() { s.appearance = i }) },
			}},
			collection.GroupRow{Label: "通知を受け取る", Child: control.Switch{
				Value: s.notify, Label: "通知", OnChange: func(v bool) { s.SetState(func() { s.notify = v }) },
			}},
			collection.GroupRow{Label: "音を鳴らす", Detail: "通知のときに短い音を出します", Child: control.Switch{
				Value: s.sound, Label: "音", OnChange: func(v bool) { s.SetState(func() { s.sound = v }) },
			}},
			collection.GroupRow{Label: "音量", Child: basic.SizedBox{Width: 200, Child: control.Slider{
				Value: s.volume, Label: "音量", OnChange: func(v float32) { s.SetState(func() { s.volume = v }) },
			}}},
		}},
		basic.Row{Spacing: 8, Cross: layout.CrossCenter, Children: []gowaku.Widget{
			control.Checkbox{Value: s.launch, Label: "起動時に開く", OnChange: func(v bool) { s.SetState(func() { s.launch = v }) }},
			basic.Expanded{Child: basic.SizedBox{}},
			control.Button{Label: "削除", Style: control.ButtonDanger, OnPress: s.dialog},
			control.Button{Label: "下書き", OnPress: func() {}},
			control.Button{Label: "保存", Style: control.ButtonPrimary, OnPress: s.dialog},
		}},
	}
}

func (s *galleryState) parts(th *theme.Theme) []gowaku.Widget {
	press := func() {}
	row := func(children ...gowaku.Widget) gowaku.Widget {
		return basic.Row{Size: layout.MainMin, Spacing: 8, Cross: layout.CrossCenter, Children: children}
	}
	return []gowaku.Widget{
		heading(th, "部品", "ボタンは 4 種類、大きさは 3 段階です。"),
		collection.Group{Title: "ボタン", Children: []gowaku.Widget{
			basic.Pad(theme.Space16, basic.Column{Size: layout.MainMin, Spacing: theme.Space10, Cross: layout.CrossStart, Children: []gowaku.Widget{
				row(
					control.Button{Label: "主要", Style: control.ButtonPrimary, OnPress: press},
					control.Button{Label: "通常", OnPress: press},
					control.Button{Label: "控えめ", Style: control.ButtonQuiet, OnPress: press},
					control.Button{Label: "削除", Style: control.ButtonDanger, OnPress: press},
				),
				row(
					control.Button{Label: "無効", Style: control.ButtonPrimary},
					control.Button{Label: "無効"},
					control.Button{Label: "無効", Style: control.ButtonQuiet},
				),
				row(
					control.Button{Label: "小", Size: control.ButtonSmall, OnPress: press},
					control.Button{Label: "標準", OnPress: press},
					control.Button{Label: "大", Size: control.ButtonLarge, Style: control.ButtonPrimary, OnPress: press},
					control.Button{Label: "追加", Icon: s.plus, OnPress: press},
				),
			}}),
		}},
		collection.Group{Title: "入力", Children: []gowaku.Widget{
			basic.Pad(theme.Space16, basic.Row{Spacing: theme.Space12, Children: []gowaku.Widget{
				basic.Expanded{Child: field.Input{Label: "お名前", Value: s.name, Placeholder: "氏名"}},
				basic.Expanded{Child: field.Input{Label: "メモ", Placeholder: "何か書く"}},
			}}),
		}},
	}
}

func (s *galleryState) motions(th *theme.Theme) []gowaku.Widget {
	c := &th.Color
	size, color, radius := float32(64), c.AccentTint, th.Radius.Control
	align := layout.AlignCenterLeft
	if s.moved {
		size, color, radius = 96, c.Accent, th.Radius.Dialog
		align = layout.AlignCenterRight
	}
	return []gowaku.Widget{
		heading(th, "動き", "値を変えるだけで、前の値から補間されます。"),
		basic.Row{Size: layout.MainMin, Children: []gowaku.Widget{
			control.Button{Label: "動かす", OnPress: func() { s.SetState(func() { s.moved = !s.moved }) }},
		}},
		basic.SizedBox{Height: 112, Child: basic.Row{Spacing: 16, Cross: layout.CrossCenter, Children: []gowaku.Widget{
			basic.AnimatedContainer{
				Duration: 350 * time.Millisecond, Curve: motion.SpringOut,
				Width: size, Height: size, Color: color, Radius: radius,
			},
			basic.Expanded{Child: basic.AnimatedAlign{
				Duration: 400 * time.Millisecond, Curve: motion.SpringOut, Alignment: align,
				Child: basic.Container{Width: 32, Height: 32, Radius: 16, Color: c.Accent},
			}},
		}}},
	}
}

func (s *galleryState) dialog() {
	overlay.ShowDialog(s.Context(), func(dismiss func()) gowaku.Widget {
		return overlay.Dialog{
			Title:   "保存しますか",
			Message: "変更はまだ保存されていません。閉じると、入力した内容は失われます。",
			Actions: []gowaku.Widget{
				control.Button{Label: "キャンセル", OnPress: dismiss},
				control.Button{Label: "保存", Style: control.ButtonPrimary, OnPress: dismiss},
			},
		}
	})
}

// Build implements core.State.
func (s *galleryState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	var body []gowaku.Widget
	switch s.page {
	case 1:
		body = s.parts(th)
	case 2:
		body = s.motions(th)
	case 3:
		body = s.display(th)
	case 4:
		body = s.forms(th)
	case 6:
		body = s.others(th)
	default:
		body = s.general(th)
	}
	// fill is an extra page that takes the whole window.
	var fill gowaku.Widget
	if i := s.page - firstExtra; i >= 0 && i < len(extraPages()) {
		if p := extraPages()[i]; p.body {
			body = []gowaku.Widget{p.build()}
		} else {
			fill = p.build()
		}
	}
	// The desktop page fills the window, as the panes of an application do, and
	// so do the extra pages that are not body pages.
	content := gowaku.Widget(basic.Scroll{Child: basic.Pad(theme.Space28, basic.Column{
		Size: layout.MainMin, Spacing: theme.Space24, Cross: layout.CrossStretch, Children: body,
	})})
	switch s.page {
	case 5:
		content = s.desktop(th)
	case 7:
		content = s.navigation()
	default:
		if fill != nil {
			content = fill
		}
	}
	return basic.Row{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		s.sidebar(th),
		basic.Expanded{Child: content},
	}}
}

func main() {
	pngPath := flag.String("png", "", "write one frame to this file and exit")
	dark := flag.Bool("dark", false, "use the dark theme with -png")
	state := flag.String("state", "", "bring the window to this state with -png; see the doc comment for the names")
	accent := flag.Uint("accent", 0x2f5fd0, "the accent color")
	localeFlag := flag.String("locale", "", "speak en, or another language, instead of Japanese")
	flag.Parse()
	// The pages are labeled in Japanese, so without -locale the words of the
	// library are Japanese too, whatever the OS speaks.
	locale := language.Japanese
	if *localeFlag != "" {
		var err error
		if locale, err = language.Parse(*localeFlag); err != nil {
			log.Fatal("gallery: -locale: ", err)
		}
	}
	themes := theme.FromAccent(geom.Hex(uint32(*accent)))
	var err error
	if *pngPath != "" {
		err = screenshot(*pngPath, themes, *dark, *state, locale)
	} else {
		err = gowaku.Run(gowaku.App{Title: "gowaku: gallery", Width: 860, Height: 720, Theme: themes, Locale: locale, Home: gallery{locale: locale}})
	}
	if err != nil {
		log.Fatal(err)
	}
}

func screenshot(out string, themes theme.Set, dark bool, state string, locale language.Tag) error {
	b := theme.BrightnessLight
	if dark {
		b = theme.BrightnessDark
	}
	w, ht := windowSize(state)
	h, err := shot.Open(gallery{locale: locale}, shot.Options{Width: w, Height: ht, Theme: themes.For(b)})
	if err != nil {
		return err
	}
	defer h.Close()
	if err := h.Settle(); err != nil {
		return err
	}
	if err := bringTo(shotDriver{h}, state); err != nil {
		return err
	}
	return h.Screenshot(out)
}

// windowSize is the size of the window of -png for state. The sidebar of
// eighteen pages needs 720 high; the panes of the desktop, dock, grid and
// editor pages need a wider window.
func windowSize(state string) (w, h float32) {
	if strings.HasPrefix(state, "desktop") || strings.HasPrefix(state, "dock") || strings.HasPrefix(state, "editors") || strings.HasPrefix(state, "lines") || state == "grid" {
		return 1180, 720
	}
	return 860, 720
}

// driver is the window that bringTo drives: the shot.Harness of -png, or a
// gowakutest.Tester in the tests.
type driver interface {
	tree() *core.Tree
	// settle draws frames until nothing moves any more.
	settle() error
	// frames draws at least one frame and at most n, until nothing moves,
	// for a page that keeps animating.
	frames(n int) error
}

// shotDriver drives the window of -png.
type shotDriver struct{ h *shot.Harness }

func (d shotDriver) tree() *core.Tree   { return d.h.Tree }
func (d shotDriver) settle() error      { return d.h.Settle() }
func (d shotDriver) frames(n int) error { return settleWithin(d.h, n) }

// stateNames are the states of -state that are not an extra page.
var stateNames = []string{
	"hover", "focus", "dialog", "fields", "parts", "display", "forms", "date", "submit",
	"desktop", "desktop-memo", "others", "navigation", "navigation-detail",
}

// bringTo drives the window to state with the events a user would cause. It
// finds the controls through the accessibility tree, by role and label, rather
// than counting presses of Tab, so that rows added to the sidebar or a page
// leave the states alone: a page opens from its sidebar row, which takes the
// keyboard and Enter, and a control takes the keyboard as from Tab, focus
// ring included.
func bringTo(d driver, state string) error {
	tree := d.tree()
	key := func(k input.KeyCode, m input.Modifiers) { tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m}) }
	click := func(at geom.Offset) {
		for _, kind := range []core.PointerKind{core.PointerMove, core.PointerDown, core.PointerUp} {
			tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: at})
		}
	}
	// find returns the node of role and label, after a frame so that the
	// tree is up to date.
	find := func(role semantics.Role, label string) (*semantics.Node, error) {
		if err := d.frames(1); err != nil {
			return nil, err
		}
		root, _ := tree.SemanticsTree()
		if n := findNode(root, role, label); n != nil {
			return n, nil
		}
		return nil, fmt.Errorf("gallery: state %q: no %v %q on the page", state, role, label)
	}
	focus := func(role semantics.Role, label string) error {
		n, err := find(role, label)
		if err != nil {
			return err
		}
		if !tree.PerformSemanticsAction(n.ID, semantics.ActionFocus, core.SemanticsArg{}) {
			return fmt.Errorf("gallery: state %q: %v %q takes no focus", state, role, label)
		}
		return nil
	}
	tap := func(role semantics.Role, label string) error {
		n, err := find(role, label)
		if err != nil {
			return err
		}
		click(n.Rect.Center())
		return nil
	}
	open := func(page string) error {
		if err := focus(semantics.RoleListItem, page); err != nil {
			return err
		}
		key(input.KeyEnter, 0)
		return d.settle()
	}
	for _, p := range extraPages() {
		end := state == p.state+"-end"
		// dock-flyout is the dock page with its hidden panel slid out.
		flyout := p.state == "dock" && state == "dock-flyout"
		if p.state != state && !end && !flyout {
			continue
		}
		if err := focus(semantics.RoleListItem, p.label); err != nil {
			return err
		}
		key(input.KeyEnter, 0)
		// The skeletons of the visual page shimmer for as long as they
		// show, so the page never settles; two seconds bring the rest to
		// rest.
		if err := d.frames(120); err != nil {
			return err
		}
		if flyout {
			if err := tap(semantics.RoleTab, "アウトライン"); err != nil {
				return err
			}
			if err := d.frames(120); err != nil {
				return err
			}
		}
		if p.state == "editors" && !end {
			// The left editor scrolls down and sideways; the right one
			// follows it down.
			n, err := find(semantics.RoleTextField, "左（折り返しなし）")
			if err != nil {
				return err
			}
			at := n.Rect.Center()
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: -4.5}})
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{X: 0.9}})
			if err := d.frames(120); err != nil {
				return err
			}
		}
		if p.state == "lines" && !end {
			// The quick open matches a query, and a right click on a row of
			// the tree selects it and opens its menu.
			if err := focus(semantics.RoleTextField, "クイックオープン"); err != nil {
				return err
			}
			tree.DispatchText("mdrn")
			n, err := find(semantics.RoleTreeItem, "render.go")
			if err != nil {
				return err
			}
			at := geom.Offset{X: n.Rect.X + 60, Y: n.Rect.Y + n.Rect.H/2}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonSecondary})
			if err := d.frames(2); err != nil {
				return err
			}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonSecondary})
		}
		if p.state == "obscure" {
			// The password shows its bullets.
			if err := focus(semantics.RoleTextField, "ユーザー名"); err != nil {
				return err
			}
			tree.DispatchText("taro")
			if err := focus(semantics.RoleTextField, "パスワード"); err != nil {
				return err
			}
			tree.DispatchText("hunter2")
		}
		if end {
			// The wheel at the right edge, clear of the widgets that
			// scroll themselves, takes the page to its end.
			at := geom.Offset{X: 840, Y: 700}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
			for range 20 {
				tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: -3}})
				if err := d.frames(2); err != nil {
					return err
				}
			}
		}
		return d.frames(120)
	}
	var err error
	switch state {
	case "":
		return nil
	case "hover":
		var n *semantics.Node
		if n, err = find(semantics.RoleButton, "下書き"); err == nil {
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: n.Rect.Center()})
		}
	case "focus":
		err = focus(semantics.RoleButton, "保存")
	case "fields":
		if err = focus(semantics.RoleTextField, "名前"); err == nil {
			key(input.KeyEnd, 0)
			tree.DispatchText("さん")
		}
	case "dialog":
		if err = focus(semantics.RoleButton, "保存"); err == nil {
			key(input.KeyEnter, 0)
		}
	case "parts":
		err = open("部品")
	case "display":
		err = open("表示")
	case "forms":
		err = open("フォーム")
	case "desktop":
		err = open("デスクトップ")
	case "others":
		err = open("そのほか")
	case "navigation":
		err = open("ナビゲーション")
	case "navigation-detail":
		if err = open("ナビゲーション"); err == nil {
			err = tap(semantics.RoleButton, "読みたい本")
		}
	case "desktop-memo":
		if err = open("デスクトップ"); err != nil {
			break
		}
		// Open the notes, then look for TODO in them.
		if err = tap(semantics.RoleTab, "会議のメモ.txt"); err != nil {
			break
		}
		if err = d.settle(); err != nil {
			break
		}
		if err = tap(semantics.RoleTextField, "会議のメモ"); err != nil {
			break
		}
		key(input.KeyF, input.ModControl)
		tree.DispatchText("TODO")
	case "date":
		if err = open("フォーム"); err == nil {
			err = tap(semantics.RoleComboBox, "日付を選択")
		}
	case "submit":
		if err = open("フォーム"); err != nil {
			break
		}
		// Leave the nickname empty and the postal code short: each shows its
		// error once the keyboard moves on, here to the agreement, which
		// takes the tick.
		if err = tap(semantics.RoleTextField, "ニックネーム"); err != nil {
			break
		}
		if err = tap(semantics.RoleTextField, "郵便番号"); err != nil {
			break
		}
		tree.DispatchText("123")
		err = tap(semantics.RoleCheckbox, "規約に同意する")
	default:
		return fmt.Errorf("unknown state %q", state)
	}
	if err != nil {
		return err
	}
	return d.settle()
}

// findNode returns the first node below n, n included, of role and label, or
// nil.
func findNode(n *semantics.Node, role semantics.Role, label string) *semantics.Node {
	var found *semantics.Node
	n.Walk(func(m *semantics.Node) bool {
		if m.Role == role && m.Label == label {
			found = m
		}
		return found == nil
	})
	return found
}

// settleWithin is Settle for at most the given number of frames, for a page
// that keeps animating.
func settleWithin(h *shot.Harness, frames int) error {
	for range frames {
		if _, err := h.Frame(); err != nil {
			return err
		}
		h.Pump()
		if !h.Tree.Ticking() {
			return nil
		}
	}
	return nil
}
