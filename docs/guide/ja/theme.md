# テーマ

[ガイドの目次](README.md) · [English](../en/theme.md)

gowaku は独自のデザインを採用している。落ち着いた無彩色の面、1 色のアクセントカラー、濃い影の代わりに使う細い線、和文を基準にした文字の尺度が特徴で、そのトークンは `theme` パッケージにまとめてある。このガイドでは、アクセントカラーの選び方、ライトとダークへの追従、実行中のテーマ変更、ウィジェットからトークンを読み出す方法を説明する。デザインそのものについては [docs/DESIGN.md](../../DESIGN.md) を参照のこと。

## Theme と Set

- `*theme.Theme` は、1 つの明るさに対応するトークン一式である。`Color`（`theme.Colors`）、`Text`（`theme.TextTheme`）、`Radius`（`theme.Radii`）、`Brightness` のフィールドがある。
- `theme.Set` はライトとダークの `*theme.Theme` の組で、これによってアプリが OS の設定に追従できる。

`theme.Default()` は、何も指定しないアプリに適用される Set で、アクセントカラーは藍（`#2f5fd0`）である。中のテーマは共有されているので、これを含めてテーマは読み取り専用として扱うこと。

## アクセントカラー 1 色

すべての色は 1 色のアクセントカラーから導き出される。その色を `theme.FromAccent` に渡し、返された Set を `App.Theme` に設定する。

```go
func app() gowaku.App {
	return gowaku.App{
		Title:     "notes",
		Theme:     theme.FromAccent(geom.Hex(0x2f8f5b)),
		ThemeMode: gowaku.ThemeSystem,
		Home:      home,
	}
}
```

`FromAccent` は、アクセントの塗り、その上に載せる文字の色、文字として使うアクセントの色、選択状態の色合いを、ライトとダークの両方について HCT 色空間で導き出す。面の色はアクセントにかかわらず無彩色のままになる。コントラストは検証済みで、`Ink` と `Ink2` はどの面の上でも 4.5:1 以上のコントラスト比を確保している（[ADR 0041](../../ADR/0041-ink-contrast.md)）。

`theme.New(accent, b)` は、`theme.BrightnessLight` か `theme.BrightnessDark` のどちらか 1 つの明るさについて `*theme.Theme` を作る。

## ライトとダーク

Set のどちらを使うかは `App.ThemeMode` で決める。

| モード | 動作 |
|---|---|
| `gowaku.ThemeSystem`（ゼロ値） | OS の設定に従う。実行中の切り替えにも追従する。 |
| `gowaku.ThemeLight` | 常にライト。 |
| `gowaku.ThemeDark` | 常にダーク。 |

OS が描画するタイトルバーと背景もテーマに従う。

## 実行中にテーマを変える

`gowaku.SetTheme(set, mode)` を呼び出すと、起動時に `App.Theme` と `App.ThemeMode` で指定したときと同じように、すべてのウィンドウの見た目が切り替わる。`gowaku.CurrentTheme()` は現在のテーマとモードを返す。設定画面でモードを選ばせる例を示す。

```go
func modeButtons() gowaku.Widget {
	set := func(mode gowaku.ThemeMode) func() {
		return func() {
			s, _ := gowaku.CurrentTheme()
			gowaku.SetTheme(s, mode)
		}
	}
	return basic.Row{Spacing: theme.Space8, Children: []gowaku.Widget{
		control.Button{Label: "System", OnPress: set(gowaku.ThemeSystem)},
		control.Button{Label: "Light", OnPress: set(gowaku.ThemeLight)},
		control.Button{Label: "Dark", OnPress: set(gowaku.ThemeDark)},
	}}
}
```

アクセントカラーを選ばせる例:

```go
func accentButton(accent geom.Color) gowaku.Widget {
	return control.Button{Label: "Use this color", OnPress: func() {
		_, mode := gowaku.CurrentTheme()
		gowaku.SetTheme(theme.FromAccent(accent), mode)
	}}
}
```

テーマを読み出しているウィジェットはすべて再ビルドされ、後から開くウィンドウにも新しいテーマが適用される。`SetTheme` は UI ゴルーチン上で、`Build` の外（コールバックなど）から呼び出す。[ADR 0067](../../ADR/0067-runtime-theme.md) も参照のこと。

## テーマを読み出す

`basic.ThemeOf(ctx)` は、ウィジェットより上位にある最も近いテーマを返し、そのテーマが変わるとウィジェットを再ビルドする。

```go
func section(ctx gowaku.BuildContext, title, note string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{
		Color:   th.Color.Card,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Padding: geom.Symmetric(theme.Space16, theme.Space12),
		Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space6, Children: []gowaku.Widget{
			basic.Text{Content: title, Style: th.Text.Headline},
			basic.Text{Content: note, Style: th.Text.Caption.WithColor(th.Color.Ink2)},
		}},
	}
}
```

### トークンの一覧

| トークン | 名前 | 型 |
|---|---|---|
| `theme.Space*` | `Space2`、`Space4`、`Space6`、`Space8`、`Space10`、`Space12`、`Space16`、`Space20`、`Space24`、`Space28`（数字が大きさ） | 型なしの定数。単位は論理ピクセル |
| `theme.ControlHeight` | 30 | 型なしの定数。単位は論理ピクセル |
| `theme.Lift*` | `LiftNone`、`LiftControl`、`LiftKnob`、`LiftFloat` | `theme.Lift`。`th.Color.Shadows` に渡す |
| `th.Radius` | `Check`（4）、`Control`（8）、`Group`（12）、`Dialog`（16） | `theme.Radii` の `float32` のフィールド |
| `th.Text` | `LargeTitle`、`Title`、`Headline`、`Body`、`Control`、`Caption`、`Section`、`Reading`、`Code` | `theme.TextTheme` の `text.Style` のフィールド |
| `th.Color` | 下の「色」の表 | `theme.Colors` の `geom.Color` のフィールド |

### 色

色には、色相ではなく用途に基づいた名前を付けている。

| フィールド | 用途 |
|---|---|
| `Canvas`, `Sidebar`, `Card`, `Raised` | ウィンドウの背景、サイドバー、グループ、ダイアログなど浮き上がって見えるもの。 |
| `Glass` | `Raised` を半透明にしたもの。ぼかした背景の上に置く面に使う。 |
| `Field`, `Control` | 入力欄の背景と、通常のボタンの背景。 |
| `Ink`, `Ink2`, `Ink3` | 本文、補足、プレースホルダなどの控えめな表示。`Ink3` はあえて 4.5:1 未満にしてあるので、情報を伝える文字には使わない。 |
| `Hair` | 罫線。 |
| `Fill`, `FillStrong` | どの面の上でも使える控えめな背景。 |
| `Accent`, `OnAccent`, `AccentInk`, `AccentTint` | アクセントの塗り、その上に載せる文字、文字や細い線として使うアクセント、選択状態の表示。 |
| `Danger`、`Success` | 破壊的な操作とエラー。成功したこと。どちらもすべての面で文字として読め、ライトテーマでは塗りとして白い文字を載せられる。 |
| `Warning` | 問題になる前に注意を促すこと。アイコン、点、枠線、薄い塗りに使う明るい琥珀色で、文字には使わない。文言は横に `Ink` で書く。 |
| `Shadow`, `Scrim` | 影の色。ダイアログの背後でページを覆う色。 |

### 文字スタイル

`th.Text` が文字の尺度で、役割ごとにサイズ、ウェイト、行送りが決まっている。

| 役割 | サイズ | ウェイト | 行送り |
|---|---|---|---|
| `LargeTitle` | 24 | 600 | 32 |
| `Title` | 16 | 600 | 24 |
| `Headline` | 13 | 600 | 20 |
| `Body` | 13 | 400 | 20 |
| `Control` | 13 | 500 | 20 |
| `Caption` | 12 | 400 | 18 |
| `Section` | 12 | 500 | 18 |
| `Reading` | 15 | 400 | 26 |
| `Code` | 13 | 400 | 20 |

`Code` は等幅である。`Family` は `theme.MonoFamilies` で、Windows、macOS、Linux の等幅の書体を並べた一覧を、各システムが先頭から順に探す。そのため `basic.Text{Content: id, Style: th.Text.Code}` と書けば書体を指定しなくてよい。`editor.Editor` と markdown のコードは、指定がなければこれを使う。

役割には色が含まれていない。`basic.Text` は、色が未設定なら `Ink` を、サイズが未設定なら `Body` のサイズを補う。そのため `basic.Text{Content: "..."}` と書くだけでも読みやすく表示される。別の色にするには、`th.Text.Caption.WithColor(th.Color.Ink2)` のように `WithColor` を使う。`WithSize`、`WithWeight`、`WithAlign` で他の属性も変更できる。`text.Style` のそれ以外の部分は[文字と IME](text-and-ime.md)で扱う。

### 余白、角丸、高さ、影

- **余白**: `theme.Space2`、`Space4`、`Space6`、`Space8`、`Space10`、`Space12`、`Space16`、`Space20`、`Space24`、`Space28`。単位は論理ピクセルで、すべて偶数なので、表示倍率が 100%、150%、200% のいずれでも物理ピクセルが整数になる。数値を直接書くより、これらのトークンを使うとよい。
- **角丸**: `th.Radius.Check`（4）、`Control`（8。ボタン、入力欄、行）、`Group`（12。カード）、`Dialog`（16）。
- **高さ**: `theme.ControlHeight`（30）が通常のコントロールの高さ。
- **影**: `th.Color.Shadows(lift)` は、ページからの浮き上がりの高さに応じた 2 層の影を返す。高さは `theme.LiftNone`、`LiftControl`、`LiftKnob`、`LiftFloat` から選ぶ。各層は `paint.BoxShadow` で、`basic.Container`、`layout.Decoration`、各コントロールが受け取る型と同じなので、`Shadows: th.Color.Shadows(theme.LiftFloat)` のようにそのまま渡せる。呼ぶたびに新しいスライスを作るので、頻繁に実行される処理の中では、テーマごとに 1 回だけ作って持っておくとよい。

## 一部だけ見た目を変える

`basic.Theme` は、その配下のウィジェットに別のテーマを適用する。ライトのウィンドウの中にダークのパネルを置く場合などに使う。

```go
var darkPanel = theme.New(geom.Hex(0x2f5fd0), theme.BrightnessDark)

func inspector(child gowaku.Widget) gowaku.Widget {
	return basic.Theme{Data: darkPanel, Child: child}
}
```

このように適用したテーマは、`SetTheme` でアプリのテーマを変えてもそのまま維持される。

`basic.MediaQueryOf(ctx).Brightness` は、使用中のテーマとは関係なく、OS が要求している明るさを表す。一方、`basic.ThemeOf(ctx).Brightness` は使用中のテーマの明るさである。テーマに合わせて描画する場合は後者を読む。

## 独自のトークン

テーマには、標準のトークンと並べて独自の型の値を持たせることができる。`theme.WithExtensionPair` で Set にライト用とダーク用の値を追加し、`theme.Extension` か `theme.ExtensionOr` で読み出す。

```go
type Brand struct{ Highlight geom.Color }

func brandSet() theme.Set {
	set := theme.FromAccent(geom.Hex(0x2f5fd0))
	return theme.WithExtensionPair(set,
		Brand{Highlight: geom.Hex(0xffd60a)},
		Brand{Highlight: geom.Hex(0x806b05)})
}

func highlight(ctx gowaku.BuildContext) geom.Color {
	b := theme.ExtensionOr(basic.ThemeOf(ctx), Brand{Highlight: geom.Hex(0xffd60a)})
	return b.Highlight
}
```

`theme.WithExtension` は、1 つの `*theme.Theme` に対して同じことを行う。どちらもコピーを返し、元のテーマは変更しない。拡張の読み出しではメモリ割り当てが発生しないので、`Build` の中で使ってよい。

## アイコン

`icons` パッケージには、よく使うアイコン 24 種が、`basic.Icon` の 24 単位の格子の上の `*paint.Path` として入っている。`Folder`、`File`、`Document`、`Image`、`Search`、`Plus`、`Minus`、`Close`、`Check`、4 方向の山形、`Settings`、`Trash`、`Edit`、`Copy`、`Home`、`Star`、`Info`、`Warning`、`Error`、`Menu`、`More` である。ウィジェットが自分で描くアイコンと同じく、太さ 1.75 単位で端の丸い線で描かれており、指定した色で塗られる。同じパスは `SidebarItem.Icon`、`BreadcrumbItem.Icon`、`Button.Icon` や、ファイルのツリーの `Tree.IconOf` にも使える。

```go
// The paths are made once and kept: basic.Icon scales a path again
// whenever it gets another pointer.
type fileIcons struct {
	folder, search *paint.Path
}

func newFileIcons() fileIcons {
	return fileIcons{folder: icons.Folder(), search: icons.Search()}
}

func (ic fileIcons) header(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []gowaku.Widget{
		basic.Icon{Path: ic.folder, Size: 16, Color: th.Color.AccentInk},
		basic.Icon{Path: ic.search, Size: 16, Color: th.Color.Ink2, Label: "Search"},
	}}
}
```

呼び出すたびに新しいパスを作るので、`Build` のたびに呼ばず、State やこのような構造体に持っておく。この中に無い形は、アイコンセットのパスデータを `paint.ParseSVGPath` で読み込む。

## さらに読む

- [docs/DESIGN.md](../../DESIGN.md): デザインの色、文字、形、ウィジェットと、その理由。
- [ADR 0011](../../ADR/0011-own-design.md): gowaku が独自のデザインを採用する理由。
- [ADR 0065](../../ADR/0065-theme-extensions-and-coverage.md): テーマの拡張。
- `examples/gallery` でデザインを確認できる。OS のモードに従って表示される。
