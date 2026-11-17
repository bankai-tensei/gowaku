# 文字と日本語入力

[English](../en/text-and-ime.md)

gowaku は文字組みを自前で行う。HarfBuzz（go-text 経由）でシェイピングし、W3C の「日本語組版処理の要件」（JLReq）の規則に従って行を分割し、グリフを GPU で描画する。規則はどの OS でも共通なので、フォントが同じであれば、段落は Windows でも Linux でも macOS でも同じ位置で折り返される。

このガイドでは、文字を表示するウィジェット、フォント、日本語組版の規則とその切り替え方、縦書き、そして IME を使ったテキスト入力を扱う。

## 文字を表示する

`basic.Text` は 1 つの文字列を表示する。`basic.RichText` は見た目の異なるスパンを 1 つの段落に並べるもので、ルビも付けられる。どちらも `text.Style` を受け取る。

```go
package main

import (
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

func main() {
	body := text.Style{Size: 18}
	page := basic.Pad(24, basic.Column{Spacing: 12, Children: []gowaku.Widget{
		basic.Text{Content: "吾輩は猫である。名前はまだ無い。", Style: body},
		basic.RichText{Style: body, Spans: []text.Span{
			{Text: "吾輩", Ruby: "わがはい"},
			{Text: "は"},
			{Text: "猫", Ruby: "ねこ"},
			{Text: "である。"},
			{Text: "名前", Style: body.WithWeight(700)},
			{Text: "はまだ無い。"},
		}},
	}})
	err := gowaku.Run(gowaku.App{Title: "text", Width: 480, Height: 200, Home: page})
	if err != nil {
		log.Fatal(err)
	}
}
```

`text.Style` のゼロ値は、14 論理ピクセルで黒の標準的な文字を表し、JLReq の既定の規則で組まれる。スパンの `Style` でゼロのままのフィールドは、段落の値を引き継ぐ。テーマには用途ごとに用意されたスタイルがある（`basic.ThemeOf(ctx)` で得られる `th.Text.Body`、`th.Text.Title`、`th.Text.LargeTitle` など）。`With` で始まるメソッド（`WithSize`、`WithWeight`、`WithColor`、`WithAlign`、`WithMaxLines`）は、1 つのフィールドだけを変えた複製を返す。

既定では文字を選択できない。`field.SelectionArea` で囲むと、段落をまたいだ選択とコピーができるようになる。ルビの付いた文字は親文字だけがコピーされる。

### 文字を切り詰める

`Style.MaxLines` は文字を指定した行数に制限し、最後の行を省略記号「…」で終える。一覧の行に 1 行だけ出すプレビューや、カードに 2 行だけ出す説明に使う。幅を推測して文字列を切ってはいけない。

```go
package app

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// mailRow は件名を 1 行、本文を 2 行まで表示する。
func mailRow(ctx gowaku.BuildContext, subject, body string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Column{Size: layout.MainMin, Spacing: theme.Space4, Children: []gowaku.Widget{
		basic.Text{Content: subject, Style: th.Text.Headline.WithMaxLines(1)},
		basic.Text{Content: body, Style: th.Text.Body.WithColor(th.Color.Ink2).WithMaxLines(2)},
	}}
}
```

- 最後より前の行は普段どおりに改行する。最後の行は幅いっぱいまで詰め、書記素クラスタの境目で切る。絵文字やアクセント付きの文字が途中で割れることはない。`MaxLines: 1` では、収まらない文字は幅の位置で切れる。
- 省略記号は必ず幅の中に収まる。収まるまで末尾の文字を取り除く。空白と「「」などの始め括弧は省略記号の前に残さない。
- 省略記号は文字の論理上の末尾に付く。左から右へ進む段落では右端、右から左へ進む段落では左端である。縦書きでは縦の省略記号になり、最後の列の下に付く。
- スクリーンリーダーが読む内容と、`gowakutest` の `Find` が探す内容は全文のままである。切り詰めたかどうかは `Layout.Truncated` でわかるので、残りをツールチップで見せることができる。

`text.Style` の主なフィールド:

| フィールド | 働き |
|---|---|
| `Family` | 既定の和文ファミリーより先に試すフォントファミリー。CSS と同じくカンマで区切って複数を並べると、先頭から順に試す（`"Cascadia Mono, Menlo, monospace"`）。システムに無いファミリーは飛ばす |
| `Size`、`Weight`、`Italic` | 論理ピクセル単位の大きさ（ゼロなら 14）、CSS と同じ太さの値（ゼロなら 400）、斜体 |
| `LineHeight` | 行送り。`Size` に対する倍率（ゼロなら 1.5）。固定値なので、代替フォントが混じっても行間が乱れない |
| `Color` | 文字の色。ゼロ値は不透明な黒 |
| `Align` | `AlignStart`、`AlignCenter`、`AlignEnd`、`AlignJustify` |
| `Direction` | BiDi の基本方向。ゼロなら最初の強い文字に従う |
| `Orientation` | `text.Horizontal` または `text.Vertical` |
| `MaxLines` | 表示する最大の行数（縦書きでは列数）。ゼロは無制限。切り詰めた文字は「…」で終わる |
| `Indent`、`HangingIndent`、`TabStop` | 1 行目の字下げ、2 行目以降の字下げ、タブ位置 |
| `LineBreak`、`Kinsoku`、`Punctuation`、`WesternSpacing`、`Hanging` | JLReq の規則。後述する |
| `Hyphenate` | 英単語をハイフンで分割する |
| `Variations` | 可変フォントの軸 |
| `RubyRoom` | ルビの無い箇所にもルビの領域を確保しておく |

## フォント

gowaku はフォントを同梱しておらず、`gowaku.Run` は OS にインストールされているフォントを読み込む。読み込みは、ウィンドウや GPU デバイスの作成と並行して別のゴルーチンで行う。フォントの索引はユーザーのキャッシュディレクトリに保存されるので、2 回目以降の起動は速い。

各文字について、次の順序でフォントファミリーを試す。

1. `Style.Family` のファミリー（指定されていれば、並べた順に）。
2. 既定の和文ファミリー: Yu Gothic（Windows）、Noto Sans CJK JP（Linux）、Hiragino Sans（macOS）、Meiryo、最後に OS の sans-serif。
3. 絵文字として表示する文字については、Segoe UI Emoji、Noto Color Emoji、Apple Color Emoji を先に試す。

Yu Gothic UI は意図的に除外している。仮名と約物がプロポーショナルな字幅のため、約物の詰めの規則が働く余地が無いからである。Linux で日本語をきれいに表示するには、Noto Sans CJK JP をインストールする（Debian と Ubuntu では `fonts-noto-cjk` パッケージ）。

カラー絵文字は、COLRv0、COLRv1、CBDT、sbix 形式のフォントから描画する。可変フォントの軸は `Style.Variations` で指定する。その軸を持つフォントでは、`Weight` から `wght` が、`Size` から `opsz` が自動的に設定される。和文フォントのように斜体を持たない書体に `Italic` を指定すると、約 12° 傾けて描画する。ただし、縦書きの行、カラー絵文字、右から左に書く文字は傾けない。

`gowaku.Run` に独自のフォントファイルを渡す手段は無い。バイト列でフォントを渡せるのは、どのマシンでも同じスクリーンショットを得るためのテストとプレビュー（`gowakutest.Options.Fonts`、`gowakupreview.Item.Fonts`）と、`text` パッケージを単独で使うときの `text.NewShaper` だけである。

## 日本語組版

既定の組み方は JLReq に従う。各規則には、それを変更するための `text.Style` のフィールドがある。`examples/jptext` では、規則を適用した組版と適用しない組版を、同じ文章で左右に並べて比較できる。

```bash
go run ./examples/jptext
```

### 行頭・行末の禁則

閉じ括弧、小書きの仮名、長音記号、読点と句点は行頭に置かない。開き括弧は行末に置かない。ダッシュと三点リーダーは途中で分割しない。`LineBreak` は、行頭禁則の対象とする文字の範囲を決める。考え方は CSS の `line-break` の `strict`、`normal`、`loose` と同じである。

| `jlreq.Strictness` | 行頭に置いてよい文字 |
|---|---|
| `jlreq.Strict`（既定） | 禁則の対象となる文字はいずれも置かない |
| `jlreq.Normal` | 小書きの仮名と長音記号 |
| `jlreq.Loose` | さらに繰り返し記号、ハイフン、波ダッシュ |

`Kinsoku` は、行頭に来てはならない文字をどう処理するかを決める。

- `jlreq.Oidashi`（既定、追い出し）は、行末の文字を次の行へ送る。
- `jlreq.Oikomi`（追い込み）は、行内の字間を詰めてその文字を同じ行に収める。詰める余地が足りなければ追い出しにする。

### 約物の詰め

全角の括弧と句読点は、字幅の半分が空きになっている。`」「` や `。」` のように約物が 2 つ並ぶと、`jlreq.PunctuationSqueeze`（既定）が JLReq 3.1.5 に従ってその空きを詰める。`jlreq.PunctuationFull` は、どの約物も全角のまま配置する。詰めはグリフごとに判定し、全角の字形の場合にだけ行う。プロポーショナルなフォントで、もともと存在しない空きを削ることはない。

### 和欧間アキ

`jlreq.WesternQuarterEm`（既定）は、和文と欧文・数字の間に四分アキを入れる（JLReq 3.2.5）。`jlreq.WesternNone` を指定すると入れない。このアキは文字ではなく字間として扱うので、テキストには含まれず、コピーもされず、スクリーンリーダーも読み上げない。

### ぶら下げ

`Hanging: true` にすると、行末をわずかに越える読点と句点を、直前の文字ごと次の行へ送る代わりに、版面の外へぶら下げる。既定では無効である。

### 両端揃えと字下げ

`Align: text.AlignJustify` は、ブロックの最終行以外を行長いっぱいまで広げる（JLReq 3.8）。広げる際は、まず欧文の語間を、次に和文の字間を広げる。日本語の段落は通常 1 字下げで始めるが、これは `Indent` で指定する。

```go
st := text.Style{
	Size:    16,
	Indent:  16, // one em
	Align:   text.AlignJustify,
	Kinsoku: jlreq.Oikomi,
	Hanging: true,
}
```

`HangingIndent` は 2 行目以降の字下げ、`TabStop` は論理ピクセル単位のタブ位置である。`TabStop` がゼロの場合、タブは空白 1 つとして組む。

### ルビ

スパンに `Ruby` を指定すると、その上（縦書きでは右）にルビが付く。ルビの付いたスパンは 1 つのまとまりとして扱い、行をまたがせない。親文字より長いルビは、掛けてもよい場合には隣の仮名に掛かる。ただし行頭と行末では行の外にはみ出さず、親文字にそろえる。1 字ずつ読みを付けるモノルビにするには、親文字 1 字ごとにスパンを分ける。

```go
spans := []text.Span{
	{Text: "東", Ruby: "とう"},
	{Text: "京", Ruby: "きょう"},
	{Text: "の"},
	{Text: "日本語組版処理", Ruby: "にほんごくみはんしょり"}, // group ruby
}
```

ルビを含む文字の行送りは、少なくとも文字サイズの `text.RubyLineHeight`（1.75）倍になり、1 行目の上にもルビの領域を確保する。ルビが付いたり外れたりする文字では、`RubyRoom` を指定しておけば行の位置が動かない。

スパンに `Emphasis` を指定すると、1 字ごとに傍点（圏点）がルビと同じ側に付く。たとえば `text.EmphasisSesame` はゴマ点である。`text/aozora` は `対象［＃「対象」に傍点］` のような注記からも傍点を読む。

`text/aozora` パッケージは、青空文庫のルビ記法 `｜漢字《かんじ》` と `漢字《かんじ》` を解釈する。`aozora.Spans` はそのような文字列を `basic.RichText` に渡すスパンに変え、各スパンに指定したスタイルを付ける（ゼロ値なら段落のスタイルになる）。`aozora.Marks` は記法の位置を返し、`aozora.Plain` は記法を取り除いた文字列を返す。入力欄では、本文に記法を残したままルビを描画できる。[入力欄のルビ](#入力欄のルビ)を参照。

```go
const opening = "吾輩《わがはい》は猫《ねこ》である。｜名前《なまえ》はまだ無い。"

page := basic.RichText{Spans: aozora.Spans(opening, text.Style{}), Style: text.Style{Size: 18}}
```

### ハイフネーション

`Hyphenate: true` にすると、英単語を行末でハイフンを付けて分割する。アルゴリズムは Liang のもので、hyph-utf8 のアメリカ英語のパターンを使う。ハイフンは行末に描画するだけで、テキストには挿入しない。英語以外の言語は組み込まれていない。`text/hyphen` パッケージを使えば他の TeX のパターンを読み込める（`hyphen.Parse`）が、それを段落に渡す手段はまだ無い。

### 双方向テキスト（BiDi）

アラビア語やヘブライ語が他の文字と混在する文は、UAX #9 に従って並べる。`Direction` は、ブロック（改行で区切られた範囲）ごとの基本方向を決める。

- `text.DirectionAuto`（既定）は、最初に現れる強い文字の方向に従う。アラビア語の段落は自動的に右から左になる。
- `text.DirectionLTR` と `text.DirectionRTL` は方向を固定する。

キャレット、ヒットテスト、選択は、画面上の並び順に従う。右から左に書く言語のためにウィジェットの配置を左右反転させることはなく、`Row` は左から右に並んだままである。縦書きは常に論理順で組む。

## 縦書き

`Orientation: text.Vertical` を指定する。行は列になり、各列は上から下へ組まれ、列は右から左へ並ぶ。和文は正立し、欧文は時計回りに 90° 回転し、約物はフォントの `vert` と `vrt2` 機能によって縦書き用の字形になる。縦書きでは、`Layout` に渡す幅は列の高さを意味し、`LineHeight` は隣り合う列の中心間の距離を意味する。

縦書きでのみ意味を持つスパンのフィールドが 2 つある。

- `TateChuYoko` は、2〜3 桁の数字を列の 1 字分の幅に横向きに並べる（縦中横、JLReq 3.4.3）。それより長い文字列は縮小して収める。
- `Upright` は、略語などの欧文を 1 字ずつ正立させて縦に並べる。

```go
vertical := text.Style{Size: 18, Orientation: text.Vertical, Family: "Yu Mincho"}
colophon := basic.RichText{Style: vertical, Spans: []text.Span{
	{Text: "明治"},
	{Text: "39", TateChuYoko: true},
	{Text: "年、雑誌「ホトトギス」に発表。"},
	{Text: "GPU", Upright: true},
	{Text: "で縦書きを描く。"},
}}
```

ルビは列の右側に付く。両端揃え、字下げ、JLReq の規則は横組みと同じように働く。1 つの段落が 1 つのウィジェットになるので、複数の段落を並べる場合は、縦書きが右から始まることに合わせて `basic.Row` に逆順で入れる。

長い縦書きの文章を横にスクロールさせるには、`Horizontal` と `Reverse` を指定した `basic.Scroll` に入れる。`Reverse` を指定すると、スクロールの範囲が右端から始まる。文章は最初の列が見える位置で開き、枠より短い文章は右端に寄り、ウィンドウの大きさを変えても右端の位置は動かない。ホイールとスクロールバーで文章が動く向きは、ほかの横スクロールと同じである。Scroll は与えられた領域をすべて使うので、`SizedBox` などで高さを決めておく。

```go
paragraphs := []string{"親譲りの無鉄砲で小供の時から損ばかりしている。", "小学校に居る時分学校の二階から飛び降りて一週間ほど腰を抜かした事がある。"}
style := text.Style{Size: 18, Orientation: text.Vertical, Indent: 18}
columns := make([]gowaku.Widget, 0, len(paragraphs))
for i := len(paragraphs) - 1; i >= 0; i-- {
	columns = append(columns, basic.Text{Content: paragraphs[i], Style: style})
}
page := basic.SizedBox{Height: 560, Child: basic.Scroll{
	Horizontal: true, Reverse: true,
	Child: basic.Row{Size: layout.MainMin, Spacing: 18, Cross: layout.CrossStart, Children: columns},
}}
```

本を読みかけの位置で開くときのように、コードから表示位置を動かすには、`Scroll.Controller` に `basic.ScrollController` を渡す。オフセットはスクロールの範囲の始まり（この例では右端）から数える。`JumpTo(0)` で最初の列に戻り、`Offset` で今の位置を、`MaxOffset` で動かせる最大の量を得られる。

`examples/tategaki` では、この方法で『坊っちゃん』の冒頭を組み、同じ文章の横組みと縦書きの入力欄を並べている。

```bash
go run ./examples/tategaki
```

## テキスト入力

`field.Input` は文字列を編集するウィジェットで、選択、クリップボード、取り消し、IME の変換、ルビ、縦書きに対応している。編集中の文字列は入力欄自身が保持するので、ゼロ値のままでも動作する。外部から文字列を差し替えるときは `Value` を設定し、編集を受け取るには `OnChange` を使う。IME で変換中の文字列は編集とはみなさず、IME で確定したときに `OnChange` が呼ばれる。

```go
package main

import (
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

type form struct{ gowaku.StatefulBase }

type formState struct {
	gowaku.StateBase
	name string
}

func (form) CreateState() gowaku.State { return &formState{} }

func (s *formState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.Pad(24, basic.Column{Spacing: 12, Children: []gowaku.Widget{
		field.Input{
			Label:       "名前",
			Placeholder: "山田花子",
			Value:       s.name,
			OnChange:    func(v string) { s.SetState(func() { s.name = v }) },
		},
		basic.Text{Content: "こんにちは、" + s.name + "さん"},
	}})
}

func main() {
	if err := gowaku.Run(gowaku.App{Title: "input", Home: form{}}); err != nil {
		log.Fatal(err)
	}
}
```

`field.Input` のその他のフィールド:

- `Multiline` を指定すると、Enter で改行し、長い行を折り返す。1 行の入力欄では Enter で `OnSubmit` が呼ばれ、`OnSubmit` が無ければ、`field.FormItem` の中では外側の `field.Form` を送信する（[フォーム](#フォーム)を参照）。ブラウザと同じく、Shift+Enter でも送信する。
- `SubmitOnEnter` を指定すると、複数行の入力欄がチャットの入力欄になる。Enter で `OnSubmit` が呼ばれ（無ければフォームを送信し）、Shift+Enter で改行する。IME の変換を確定する Enter は確定だけを行い、IME が確定した文字列の改行はそのまま残る。
- `Autofocus` を指定すると、入力欄は最初に現れたときにキーボードを受け取る。ダイアログの入力欄に使う。ダイアログ自身のフォーカスより優先される。
- `Obscure` はパスワード欄にする。1 字ごとに黒丸を描画し、IME をオフに保つので、入力メソッドが文字列を変換したり学習したりすることはない。フォーカスがある間は、ウィンドウに対して安全な入力も要求する。コピーと切り取りは何もせず、取り消しの履歴も残さない。スクリーンリーダーにはパスワード欄として伝わる。
- `Controller` には `*field.InputController` を渡す。これを使うと、選択範囲の取得と設定、利用者の操作と同じ編集（`Insert`、`Replace`、`Undo`、`Copy`、`Paste`）、フォーカスの移動（`Focus`）ができる。位置はテキストのバイト単位で表す。`Focus` と `SelectAll` は入力欄ができる前に呼んでもよい。ダイアログの中でも、入力欄はできたときにそれを受け取る。
- `Bare` を指定すると枠を描画しない。ページ全体が入力欄であるような場合に使う。

チャットの入力欄と、文字列を選択した状態で開く「名前の変更」ダイアログの例を示す。

```go
field.Input{
	Label: "メッセージ", Multiline: true, SubmitOnEnter: true,
	Value:    s.draft,
	OnChange: func(v string) { s.SetState(func() { s.draft = v }) },
	OnSubmit: func(v string) { s.send(v); s.SetState(func() { s.draft = "" }) },
}
```

```go
func (s *filesState) rename(ctx gowaku.BuildContext) {
	s.name.SelectAll() // s.name は State が持つ field.InputController
	s.name.Focus()
	overlay.ShowDialog(ctx, func(dismiss func()) gowaku.Widget {
		return overlay.Dialog{Title: "名前の変更", Content: field.Input{
			Value: s.file, Controller: &s.name,
			OnChange: func(v string) { s.SetState(func() { s.file = v }) },
			OnSubmit: func(string) { dismiss() },
		}}
	})
}
```

長い文書を扱うには、`widgets/editor` の `editor.Editor` を使う。表示中の行だけをシェイピングするので、10 万行の文書でも遅延なくスクロールや入力ができる。IME の変換、検索と置換、行番号、装飾に対応しているが、横組みのみでルビは使えない。縦書きやルビを付けたまま編集するには、複数行の `field.Input` を使う。

### フォーム

`field.Form` は、配下の `field.FormItem` をまとめて検証し、送信する。`FormItem` は 1 つの部品とその値を持ち、部品からキーボードのフォーカスが外れたときと送信時に値を検証して、エラーを部品の下に表示し、部品の枠を赤くする。`FormItem` の中では、どの入力部品も同じように振る舞う。対象は `Input`、`Search`、`NumberField`、`Autocomplete`、`TokenField`、`DropdownMenu`、`DatePicker`、`TimePicker`、`ColorPicker` である。`TimePicker` のように複数の部分からなる部品や、`DatePicker` のようにポップアップを開く部品は 1 つの欄として扱う。部分の間やカレンダーへフォーカスが移っても、欄を離れたことにはならない。

`Required` のエラーの文言は部品によって変わる。入力欄では「入力してください」（Required）、選択の部品（`DropdownMenu`、`DatePicker`、`TimePicker`、`ColorPicker`、`RadioGroup`、`SegmentedControl`、切り替えの `Chip`）では「選択してください」（Choose one）、`Checkbox` など bool の部品では「チェックしてください」（Check this box）になる。`RequiredMessage` で差し替えられる。

`FormItem` の中の 1 行の入力欄で Enter キーを押すと、フォームを送信する。ただし、欄に独自の `OnSubmit` があればそちらが呼ばれる。`NumberField` は、入力された数値を先に確定し、その値でフォームを送信する。`FormItem` の外の `NumberField` では、Enter キーは数値を確定するだけである。

生年月日の欄には、`DatePicker` に 30 年ほど前の `InitialMonth` と今日の `Max` を指定する。カレンダーはその日付の近くで開き、見出しを押すとその年の月の一覧、さらに 10 年分の年の一覧に切り替わるので、別の年も数回の操作で選べる。キーボードでは Tab キーで見出しに移動でき、月と年の一覧は矢印キー、Page Up / Page Down、Enter キーで操作できる。

登録フォームの例を示す。

```go
package main

import (
	"log"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

type signup struct{ gowaku.StatefulBase }

type signupState struct {
	gowaku.StateBase
	form   field.FormController
	name   string
	people float64
	born   time.Time
	agreed bool
}

func (signup) CreateState() gowaku.State { return &signupState{people: 1} }

func (s *signupState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	today := time.Now()
	return basic.Pad(24, field.Form{Controller: &s.form, OnSubmit: s.save, Child: basic.Column{Spacing: 12, Children: []gowaku.Widget{
		// 入力してください
		field.FormItem[string]{Value: s.name, Required: true, Child: field.Input{
			Label: "氏名", Value: s.name, OnChange: func(v string) { s.SetState(func() { s.name = v }) },
		}},
		// Enter here submits the form, with the number just typed.
		field.FormItem[float64]{Value: s.people, Child: field.NumberField{
			Label: "人数", Value: s.people, Min: 1, Max: 9,
			OnChange: func(v float64) { s.SetState(func() { s.people = v }) },
		}},
		// 選択してください. The calendar opens on 1995; its title goes to
		// the months and the years.
		field.FormItem[time.Time]{Value: s.born, Required: true, Child: field.DatePicker{
			Label: "生年月日", Value: s.born, Today: today, Max: today,
			InitialMonth: today.AddDate(-30, 0, 0),
			OnChange:     func(d time.Time) { s.SetState(func() { s.born = d }) },
		}},
		// チェックしてください, unless RequiredMessage says otherwise.
		field.FormItem[bool]{Value: s.agreed, Required: true, RequiredMessage: "利用規約への同意が必要です", Child: control.Checkbox{
			Label: "利用規約に同意する", Value: s.agreed,
			OnChange: func(v bool) { s.SetState(func() { s.agreed = v }) },
		}},
		control.Button{Label: "登録", Style: control.ButtonPrimary, OnPress: func() { s.form.Submit() }},
	}}})
}

func (s *signupState) save() { log.Printf("%s, %v, %s", s.name, s.people, s.born.Format("2006-01-02")) }

func main() {
	if err := gowaku.Run(gowaku.App{Title: "signup", Home: signup{}}); err != nil {
		log.Fatal(err)
	}
}
```

### 縦書きの入力

縦書きの `Style` を指定した入力欄は、列の方向に沿って編集する。キャレット、選択、IME も列に従い、下キーで次の文字へ、左キーで次の列へ移動する。IME の候補ウィンドウはキャレットの横に開く。

```go
field.Input{
	Multiline: true,
	Style:     text.Style{Size: 16, Orientation: text.Vertical},
	OnChange:  func(v string) { s.SetState(func() { s.draft = v }) },
}
```

### 入力欄のルビ

`Ruby` には、テキスト中のルビ記法を見つける関数を渡す。`aozora.Marks` をそのまま使える。

```go
field.Input{
	Multiline: true,
	Ruby:      aozora.Marks,
	Value:     "｜吾輩《わがはい》は猫である。",
	OnChange:  func(v string) { s.SetState(func() { s.text = v }) },
}
```

入力欄はルビを親文字の上に描画し、記法そのものは隠す。ただし、選択範囲の端が内側にある記法だけは、編集できるように記法のまま表示する。それ以外（値、`OnChange`、選択、コピー、取り消し、スクリーンリーダーに渡す文字列）はすべて記法を含んだテキストのまま扱うので、保存したファイルにも読みが残る。青空文庫以外の記法も使える。`Ruby` には、`text.RubyMark` を追加していく関数であれば何でも渡せる。

## IME

IME で変換している間、入力欄は変換中の文字列（プリエディット）を、キャレットの位置に入力欄と同じフォントでその場に描画する。

- 文節ごとに細い下線を引く。
- 変換対象になっている文節（注目文節）は、背景に色を付け、太い下線を引く。
- 候補ウィンドウは、注目文節の横（注目文節が無ければキャレットの横）に、行と重ならないように開く。縦書きでは列の横に開く。

IME がオンになるのは入力欄にフォーカスがある間だけなので、ボタンにフォーカスがある状態でキーを打っても変換は始まらない。変換中に入力欄がフォーカスを失った場合は、入力欄が変換中の文字列を自分で確定するので、入力した文字が消えることはない。パスワード欄（`Obscure`）では IME はオフのままである。

OS ごとの対応状況:

| OS | 経路 | インラインの変換表示 | 文節と注目文節 | キャレットの位置に出る候補ウィンドウ |
|---|---|---|---|---|
| Windows | IMM32。TSF ベースの IME は OS の互換レイヤーを経由する | 対応 | 対応 | 対応 |
| macOS | `NSTextInputClient` | 対応 | 対応。マークされたテキストの下線の太さから判定する | 対応 |
| Linux、Wayland | `text-input-v3` | 対応 | 注目文節のみ（後述） | 対応 |
| Linux、X11 | XIM | 非対応。変換中の文字列は入力メソッドが自前のウィンドウに表示する | 非対応 | 非対応 |

補足:

- **Wayland。** `text-input-v3` プロトコルで伝わるのは変換中の文字列とカーソルの範囲だけで、文節の情報は含まれない。Mozc が変換中の文節を選択するように、入力メソッドが範囲を選択している場合は、その範囲を注目文節として強調し、残りを変換済みとして扱う。範囲が無ければ全体を未変換として表示する。sway 上の fcitx5 + Mozc で確認した。GNOME の ibus は一部しか確認できていない。
- **X11。** gowaku が受け取るのは、入力メソッドが確定した文字列だけである。変換中の文字列と候補は入力メソッドが自前のウィンドウに表示し、gowaku からそれをキャレットの位置に置くことはできない。
- **macOS。** インラインの変換表示、文節、候補ウィンドウはコードのテストまでで、手動での確認はまだ行っていない。
- **Linux 全般。** 入力メソッドからの入力について、実機のデスクトップでの通しの確認はまだ行っていない。

Wayland は `WAYLAND_DISPLAY` が設定されているときに使われ、`GOWAKU_BACKEND=x11` で X11 を強制できる。[プラットフォーム](platforms.md)を参照。

## text パッケージを直接使う

`text` パッケージはウィジェット無しでも使える。文字列を一度シェイピングしておけば、任意の幅で行に分割できる。`examples/jptext` と `examples/tategaki` の `-lines` オプションは、この方法で行の区切りを出力し、他の組版ソフトの結果と突き合わせるためのものである。

```go
package main

import (
	"fmt"
	"log"

	"github.com/bankai-tensei/gowaku/text"
)

func main() {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		log.Fatal(err)
	}
	s := "「ほら、見て。」と彼は言った。行頭に句点が来ないように組む。"
	l := shaper.Shape(s, text.Style{Size: 16}).Layout(16 * 10) // ten ems
	for i := range l.LineCount() {
		start, end := l.LineRange(i)
		fmt.Println(s[start:end])
	}
}
```

`text.Shaper` と、そこから作られたオブジェクトは、1 つのゴルーチンからしか使えない。アプリの中では、ウィジェットは `Run` が作った 1 つの Shaper を共有する。自分で Shaper を作るのは、上の例のようにウィジェットのツリーの外で使う場合だけにする。自作のウィジェットや `layout.Painter` からは、Build の中で `core.DependOn[*text.Shaper](ctx)` を呼んで、共有の Shaper を取り出す。Painter は `paint.Canvas.DrawText` で文字を指定の位置に描く。描くときに初めて決まるラベルは、`text.Labels` が組んで取っておく。`text/jlreq` パッケージは行分割の規則だけを実装しており、フォント無しで仮の寸法に対して動作する。

## 関連

- [アクセシビリティ](accessibility.md): 文字と入力欄がスクリーンリーダーに渡す情報
- [テストとプレビュー](testing-and-preview.md): `gowakutest.Type` は確定済みの文字列を入力欄に送る
- [ADR 0007](../../ADR/0007-text-architecture.md)、[ADR 0023](../../ADR/0023-vertical-text.md)、[ADR 0074](../../ADR/0074-inline-ruby.md): 設計判断
