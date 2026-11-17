# 0031: TextStyle の一部だけを変える方法と、名前付きの Alignment

決定日: 2026-11-08

状態: 採用

## 背景

公開 API だけで音楽プレーヤーの見本アプリを書いたところ、次の 2 か所で回り道をした。

1. テーマの文字スタイル（`th.Text.Caption` など）は値で、色を持たない（`TextTheme` は色を呼び出し側に任せる）。色や大きさだけを変えたいとき、ウィジェットのリテラルの中では式が要るので、アプリは `colored(style, color)` と `sized(style, size, weight)` という自前のヘルパを書いた。見本の中で 60 か所以上使っている。
2. `layout.Alignment` は 0..1 の割合で、ゼロ値は左上。Flutter は -1..1 で中央が 0。Flutter に慣れた作者は、中央左のつもりで `Alignment{X: -1}`、中央のつもりで `Alignment{}` と書いた。どちらもエラーにならず、子は箱の外か左上に置かれた。

## 選択肢と決定

### 1. TextStyle の一部だけを変える

- (a) Flutter の `copyWith` に当たるものを作る。
  - Go には名前付き引数もオプション引数も無い。全フィールドを引数にするか、関数オプションか、ポインタだらけのオプション構造体になる。
  - CODING_STYLE §4.3 は `copyWith` の乱用を、§5.3 は関数オプションを禁じている。
- (b) フィールドへの代入だけを正しい書き方として doc に書く（`st := th.Text.Body; st.Color = c`）。
  - いちばん素直な Go だが、ウィジェットのリテラルの中では書けない。Build の頭に変数を並べることになる。
  - 見本のように、どのアプリも同じヘルパを書き直すことになる。
- (c) 1 フィールドを差し替えた写しを返す値メソッドを、よく変えるフィールドにだけ置く。`WithColor`、`WithSize`、`WithWeight` の 3 つ。

**(c) を採る。**

- §5.3 が禁じるメソッドチェーンは、ウィジェットを組み立てるビルダー（`.Spacing(8).Children(...)`）のこと。設定をすべてメソッドで渡し、構造体リテラルの代わりにする書き方を禁じている。
- 値を 1 段変えて返すメソッドは別物で、すでに `geom.Color.WithAlpha` や `geom.Rect.Translate` がある。`WithColor` はその仲間。
- §5.3 の「関数オプション（`WithX(...)`）」は、閉包を返すパッケージ関数の話。レシーバーを持つ値メソッドとは形が違う。
- メソッドは 3 つに絞る。見本で変えていたのは色、大きさ、太さだけだった。ほかのフィールドを変えるとき、または 3 つ以上を一度に変えるときは、写してから代入する。このことを doc に例つきで書く。
- `TextStyle` は普通の値のまま。メソッドは値レシーバーで、アロケーションしない（テストで確認する）。
- ゼロ値の意味は変えない。`WithSize(0)` は 14、`WithWeight(0)` は 400、`WithColor(geom.Color{})` は不透明な黒になる。

### 2. Alignment の範囲

範囲を -1..1 に変えると、すべての呼び出しとピクセル golden が壊れる。範囲は 0..1 のまま、正しく書くのを易しく、間違えるのを難しくする。

- 名前付きの値を 9 つ置く。`AlignTopLeft`、`AlignTopCenter`、`AlignTopRight`、`AlignCenterLeft`、`AlignCenter`、`AlignCenterRight`、`AlignBottomLeft`、`AlignBottomCenter`、`AlignBottomRight`。
  - 構造体は Go の定数にできないので `var` にする。
  - `layout` に置き、ルートの `gowaku` からも同じ名前で出す。
  - ルートの `Center` はウィジェットなので、名前の頭に `Align` を付ける。
  - `text.AlignCenter`（行の揃え）とは別のパッケージなので衝突しない。ただし、これをルートから出すときは `TextAlignCenter` のような名前にする必要がある。
- `Alignment`、`Align`、`Stack`、`AnimatedAlign`、`Scale.Origin`、ルートの別名の doc に「0..1 で、Flutter の -1..1 ではない」と書く。
- 範囲の外で panic するかを検討し、**しない** ことにした。
  - `AnimatedAlign` に `motion.SpringOut` のような行き過ぎる曲線を渡すと、補間の途中で 1 を越える。ギャラリーの例がまさにこれを使っている。
  - 子を箱の少し外に置く使い方も成り立つ。
  - 正当な使い道があるので、止めると正しいコードを壊す。代わりに、範囲外でもそのまま置くことをテストで固定した。
- ライブラリの中で名前にしたほうが明らかに読みやすい所だけを置き換えた。`Center`、ダイアログの `Scale.Origin`、ギャラリーの `AnimatedAlign` の 3 か所。テストのリテラルは、期待する座標の計算と対応しているのでそのままにした。

## 影響

- 追加だけで、既存の挙動とピクセル golden は変わらない。
- 見本アプリ の `colored` / `sized` は `WithColor` / `WithSize` + `WithWeight` に置き換えられる。`layout.Alignment{Y: 0.5}` などは `gowaku.AlignCenterLeft` などに置き換えられる（この ADR の範囲では見本を直していない）。
- 名前付きの Alignment は `var` なので、書き換えることはできてしまう。`io.EOF` などと同じ扱いとし、守らない。
