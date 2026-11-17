// Package catalog is the widget catalog of docs/catalog: one entry for each
// public widget of the packages under widgets, with a short description in
// Japanese and English, a scene drawn on the CPU in light and dark, and the
// code of that scene as the example. internal/cmd/catalog writes the Markdown
// pages and the static site from it:
//
//	go run ./internal/cmd/catalog
//
// An entry names its widget with the package, as in control.Button, since
// packages under widgets may share type names. The example of an entry is the
// body of its scene function, scene followed by the package with a capital
// and the type, as in sceneControlButton, read from the source of this
// package, so the code on the page is the code that drew the picture and the
// compiler checks it. Lines between "// catalog:begin" and "// catalog:end"
// narrow the example to the part that matters.
//
// The pictures are set in the Go fonts, which have no Japanese glyphs, so the
// scenes are written in English for both languages.
package catalog

import (
	"strings"

	"github.com/bankai-tensei/gowaku/core"
)

// Category groups entries on one page.
type Category struct {
	// Slug names the page: docs/catalog/ja/<Slug>.md.
	Slug   string
	JA, EN string
	// AboutJA and AboutEN introduce the page.
	AboutJA, AboutEN string
}

// Entry is one widget of the catalog.
type Entry struct {
	// Name is the exported type the entry is about, with its package, as in
	// "control.Button".
	Name string
	// Also are the other exported types that the entry shows, such as the
	// items of a list or the options of a choice, each with its package.
	Also     []string
	Category string
	// JA and EN describe the widget in a sentence or two.
	JA, EN string
	// Scene builds what the picture shows. Its source, the function that
	// Func names, is the example on the page.
	Scene func() core.Widget
	// Show, if set, builds the picture in place of Scene, for a widget that
	// needs surroundings the example leaves out, such as the Navigator around
	// a BackButton.
	Show func() core.Widget
	// Act runs after the first frame, to open a menu or move the focus.
	Act func(tree *core.Tree, ctx core.BuildContext)
	// Width and Height are the canvas in logical pixels. Zero selects 560 by
	// 320. The picture is cropped to what the scene draws.
	Width, Height float32
	// NoPicture leaves the picture out, for a widget that draws nothing of
	// its own: the example still compiles and runs in the test.
	NoPicture bool
}

// modulePath is the import path of the module.
const modulePath = "github.com/bankai-tensei/gowaku"

// splitName splits a package-qualified name such as "control.Button" into the
// package and the type. A name without a package has an empty package.
func splitName(name string) (pkg, typ string) {
	pkg, typ, ok := strings.Cut(name, ".")
	if !ok {
		return "", name
	}
	return pkg, typ
}

// Pkg returns the package of e, such as "field".
func (e *Entry) Pkg() string {
	pkg, _ := splitName(e.Name)
	return pkg
}

// Type returns the type of e without its package, such as "Text".
func (e *Entry) Type() string {
	_, typ := splitName(e.Name)
	return typ
}

// ImportPath returns the import path of the package of e.
func (e *Entry) ImportPath() string { return importPath(e.Pkg()) }

func importPath(pkg string) string { return modulePath + "/widgets/" + pkg }

// File returns the base name of the pictures of e: the package and the type
// in lower case, as in "control-button".
func (e *Entry) File() string { return strings.ToLower(e.Pkg() + "-" + e.Type()) }

// Func returns the name of the scene function of e: "scene", the package
// with a capital and the type, as in "sceneControlButton".
func (e *Entry) Func() string {
	pkg := e.Pkg()
	if pkg != "" {
		pkg = strings.ToUpper(pkg[:1]) + pkg[1:]
	}
	return "scene" + pkg + e.Type()
}

// Package is one of the widget packages, in the order of their layers.
type Package struct {
	Name   string
	JA, EN string
}

// Packages returns the widget packages from the bottom layer up (ADR 0081).
func Packages() []Package {
	return []Package{
		{Name: "basic", JA: "テーマ、箱とレイアウト、効果と変形、暗黙のアニメーション、文字と画像、スクロール。", EN: "The theme, boxes and layout, effects and transforms, implicit animation, text and images, scrolling."},
		{Name: "interact", JA: "ジェスチャー、マウス、フォーカス、ショートカット、ドラッグ&ドロップ、セマンティクスなど、入力を受け取るウィジェット。", EN: "What takes input: gestures, the mouse, focus, shortcuts, drag and drop and semantics."},
		{Name: "control", JA: "ボタン、チェックボックス、スライダー、タブ、分割ペイン、進捗表示など、独自の見た目を備えたコントロール。", EN: "Controls with a look of their own: buttons, checks, sliders, tabs, split panes and progress."},
		{Name: "overlay", JA: "ダイアログ、シート、トースト、ツールチップ、メニューなど、ページの上に重ねて表示するもの。", EN: "What floats above the page: dialogs, sheets, toasts, tooltips and menus."},
		{Name: "field", JA: "テキスト入力欄、検索、フォーム、ドロップダウン、日付と時刻など、入力や選択で値を受け取るもの。", EN: "What takes a value by typing or picking: text fields, search, forms, dropdowns, dates and times."},
		{Name: "nav", JA: "Navigator、Hero、戻るボタン、パンくずリスト、タブバー、ツールバーなど、画面や場所を移動するためのもの。", EN: "What moves between pages and places: the Navigator, heroes, the back button, breadcrumbs, tab bars and toolbars."},
		{Name: "collection", JA: "リスト、ツリー、表、プロパティグリッドなど、多数の項目を表示するもの。", EN: "What shows many items: lists, trees, tables and property grids."},
		{Name: "editor", JA: "長い文書を編集するエディタ。", EN: "The editor of long documents."},
	}
}

// Categories returns the categories in the order of the pages.
func Categories() []Category {
	return []Category{
		{
			Slug: "actions", JA: "ボタンと操作", EN: "Buttons and actions",
			AboutJA: "押して何かを実行するウィジェット。ボタンは `control.Button` の 1 種類だけで、`Style` で 4 種類の見た目を選ぶ。",
			AboutEN: "Controls that do something when pressed. One `control.Button` covers every button; its `Style` picks one of four looks.",
		},
		{
			Slug: "input", JA: "入力", EN: "Input",
			AboutJA: "文字や数値を入力するウィジェット。ラベルは欄の上か、`collection.GroupRow` の左に置く。",
			AboutEN: "Controls that take text and numbers. Labels sit above the field or at the left of a `collection.GroupRow`.",
		},
		{
			Slug: "selection", JA: "選択", EN: "Selection",
			AboutJA: "決まった選択肢から選ぶウィジェット。選択肢が 2〜4 個なら `control.SegmentedControl`、それより多ければ `field.DropdownMenu` を使う。",
			AboutEN: "Controls that pick from a set of choices: `control.SegmentedControl` for two to four, `field.DropdownMenu` for more.",
		},
		{
			Slug: "display", JA: "表示", EN: "Display",
			AboutJA: "文字、画像、数値を表示するウィジェット。操作は受け付けない。",
			AboutEN: "Widgets that show text, pictures and numbers, with no action of their own.",
		},
		{
			Slug: "navigation", JA: "ナビゲーション", EN: "Navigation",
			AboutJA: "画面の間を移動するためのウィジェット。アクセントの色は現在地にだけ使う。",
			AboutEN: "Widgets that move between pages and places. Only the current place takes the accent color.",
		},
		{
			Slug: "feedback", JA: "フィードバックとオーバーレイ", EN: "Feedback and overlays",
			AboutJA: "進み具合や結果を伝えるウィジェットと、画面の上に重ねて表示するウィジェット。重ねて表示するものは `overlay.Show...` 関数で表示する。",
			AboutEN: "Widgets that report progress and results, and the ones that float above the page, shown with the `overlay.Show...` functions.",
		},
		{
			Slug: "data", JA: "データ", EN: "Data",
			AboutJA: "多数の行を表示し、編集するウィジェット。どれも表示範囲付近の行だけを構築する。",
			AboutEN: "Widgets that show and edit many rows. Each builds only the rows in view.",
		},
		{
			Slug: "desktop", JA: "デスクトップ", EN: "Desktop",
			AboutJA: "メニューバー、右クリックメニュー、ツールバー、タブ、分割ペインなど、デスクトップアプリに特有のウィジェット。",
			AboutEN: "Widgets particular to desktop applications: the menu bar, context menus, toolbars, document tabs and split panes.",
		},
		{
			Slug: "layout", JA: "レイアウト", EN: "Layout",
			AboutJA: "子の位置と大きさを決めるウィジェット。余白と間隔には `theme.Space*` の偶数の値を使う。",
			AboutEN: "Widgets that place and size their children. Spacing uses the even steps of `theme.Space*`.",
		},
		{
			Slug: "effects", JA: "効果と描画", EN: "Effects and painting",
			AboutJA: "子の見た目を変えるウィジェットと、自分で描画するウィジェット。どちらもレイアウトには影響しない。",
			AboutEN: "Widgets that change how their child looks, and the ones that paint on their own. None changes the layout.",
		},
		{
			Slug: "animation", JA: "アニメーション", EN: "Animation",
			AboutJA: "値が変わると、補間しながら変化するウィジェット。OS が動きを減らすよう求めているときは、アニメーションせずに切り替わる。",
			AboutEN: "Widgets that move to new values over time. When the OS asks for less motion they jump instead.",
		},
		{
			Slug: "behavior", JA: "入力処理とアクセシビリティ", EN: "Input handling and accessibility",
			AboutJA: "自分では何も描かず、ポインタ、キーボード、ドラッグの扱いや支援技術への伝え方を子に加えるウィジェット。画像は無く、例のコードだけを載せる。",
			AboutEN: "Widgets that draw nothing and add pointer, keyboard, drag and assistive technology behavior to their child. They have examples but no pictures.",
		},
		{
			Slug: "infrastructure", JA: "ウィジェットツリーの基盤", EN: "Tree services",
			AboutJA: "配下のウィジェットに、テーマ、ウィンドウの情報、文言、重ねて表示する場所を提供するウィジェット。`gowaku.Run` がウィンドウのルートに置くため、自分で使うのは一部だけを変えたいときに限られる。",
			AboutEN: "Widgets that hand the theme, the window, the words and a place for overlays to the widgets below. `gowaku.Run` puts each at the root of a window; use them to change a part.",
		},
	}
}
