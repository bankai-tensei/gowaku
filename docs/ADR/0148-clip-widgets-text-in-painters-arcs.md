# 0148: 切り取りのウィジェット、Painter の中の文字、円弧と楕円

決定日: 2026-11-17

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、同じ求めが何度も出た。

1. **切り取り：** 子の描画を箱の中に切り取るウィジェットが無かった。角の丸いカードに画像を収める、はみ出す飾りを隠す、といった場面で、`basic.Translate{Clip: true}` を代わりに使った人がいた。ところが `RenderTranslate.Paint` は X と Y が 0 のとき切り取りを飛ばしていたので、godoc の「Clip は描画を箱の中に保つ」と違い、何も切り取らなかった。
2. **Painter の中の文字：** `control.CustomPaint` の Painter で目盛りの数字、グラフのラベル、図の注記を描きたい、という声が 4 人から出た。ADR 0135 で Shaper の取り方は書いたが、組んだ `text.Layout` の `Runs` を `DrawGlyphs` で 1 つずつ描き、中央そろえやベースラインの位置を自分で計算する必要があった。ベースラインの位置を返すメソッドも無く、行の高さとフォントの寸法から推し量るしかなかった。描くときに初めて決まるラベル（大きさで変わる目盛り）は、毎フレーム組み直すと割り当てが起きる。
3. **円弧と楕円：** `paint.Path` に円弧と楕円を足す手段が無く、ドーナツのグラフを折れ線で、楕円を 4 本の 3 次曲線を手で書いて近似していた。`layout` の進捗の輪は、内部の `arc` 関数で同じことをしていた。

## 選択肢

### 切り取りのウィジェット

- **A. `basic.ClipRect` と `basic.ClipRRect` を足し、1 つの `layout.RenderClip`（`Radius` が 0 なら矩形）で描く。** Flutter と同じ名前で、探しやすい。
- **B. `Container` に `Clip` を足す。** 飾りと切り取りが 1 つになり、飾りの無い切り取りでも `Container` を使うことになる。`Container` は既にフィールドが多い。
- **C. `basic.Clip{Shape}` 1 つにする。** 形の種類の型が要り、よく使う 2 つのために API が大きくなる。

A を採った。

### 角の半径

- **A. `Radius float32` を持ち、0 ならテーマのカードの角（`theme.Radii.Group`）にする。** ゼロ値の `ClipRRect{}` がカードの角と合い、ゼロ値で使えるという規則に沿う。角の無い切り取りは `ClipRect` で書ける。
- **B. 0 は角が無いとする。** ゼロ値の `ClipRRect` が `ClipRect` と同じになり、意味が無い。
- **C. 4 つの角を別々に持つ。** 求めは無く、`Container` の `Radius` とも揃わない。

A を採った。

### 文字を描く手段の置き場所

`paint` は `text` を知らず、`text` は `paint` を型のためだけに使う（CODING_STYLE §3.1）。

- **A. `paint.Canvas.DrawText` を足し、引数は `paint` の中で定義する小さなインターフェイス `paint.TextLayout`（`Size`、`Baseline`、`Runs`）にする。** `*text.Layout` がこれを満たす。インターフェイスは使う側で定義するという規則に沿い、ポインターを渡すだけなので割り当ては無い。依存の向きも変わらない。
- **B. `text.Layout` に `Paint(cv, ...)` を足す。** `text` が描画の命令を出すことになり、「型のためだけ」を破る。
- **C. `widgets/control` に関数を置く。** Painter を書く人は `paint` を使っているので見つけにくく、`layout` や自作の RenderObject からは使えない。

A を採った。

### 位置の指定

- **A. `paint.TextAnchor{X, Y, Baseline}`：X と Y は箱の中の割合（0 が左・上、0.5 が中央、1 が右・下）、`Baseline` が true なら Y の代わりに最初の行のベースラインを位置に合わせる。** `layout.Alignment` と同じ 0..1 の範囲で、ゼロ値は左上（`DrawImage` の矩形の角と同じ）。ラベルを線の上に載せる場合は `Baseline: true` だけで済む。
- **B. 左・中央・右と上・ベースライン・下を組み合わせた 12 個の定数。** 数が多く、中間の割合を書けない。
- **C. `text.Align` を使う。** 段落の中の行のそろえで、点に対する置き方とは意味が違う。

A を採った。X は箱の幅で測るので、ラベルは無限の長さ（`geom.Inf`）で組む。有限の長さで `AlignCenter` や `AlignEnd` にした段落は、その長さ全体が箱になる（ADR 0135）。

### 描くときに決まるラベル

- **A. `text.Labels` を足す。** `Shaper` と `Style` を Build で設定し、Painter が `Get(content)` で組んだ `Layout` を受け取る。初めての文字だけ組み（割り当てがある）、2 回目からは同じ `Layout` を返す。数字は Painter が持つ `[]byte` に `strconv.AppendInt` で書き、`GetBytes` で引けば文字列を作らない。256 個を超えたら、また `Shaper` か `Style` が変わったら全部忘れる。色は `Layout` に入らないので、色だけの変化では忘れない。
- **B. 文字列を受け取る `DrawText` にし、中で毎回組む。** 毎フレーム組み直しで割り当てが起きる。
- **C. 何も足さず、Build で組んだ `Layout` を渡す形だけにする。** 大きさで変わる目盛りのように、Painter が描くときにしか分からないラベルに対応できない。

A を採った。Build で決まるラベルは、今まで通り `Shaper.Shape(content, st).Layout(geom.Inf)` で組んで渡せばよい。

### 円弧

- **A. `AddArc(center, radius, start, sweep)`（新しい輪郭を始める）と `ArcTo(center, radius, start, sweep)`（今の輪郭を続け、離れていれば線でつなぐ）を足す。** 角度はラジアンで x 軸の正の向きから測り、y が下向きなので正の sweep は画面で時計回りになる。円だけにし、楕円の弧は変形で描く。
- **B. SVG の `A` コマンドと同じ端点の形にする。** 中心と角度で考えるグラフには使いにくい。

A を採った。楕円は `AddOval(r geom.Rect)` で、右端から時計回りの 4 本の 3 次曲線にする。

## 決定

- `layout.RenderClip`（`Radius`）を足し、`basic.ClipRect` と `basic.ClipRRect`（`Radius`、0 ならテーマの `Radius.Group`）がこれを使う。
  - 描画は `Canvas.ClipRRect` で切り取る。角の無い矩形は scissor で費用が無く、丸い角はステンシルを 1 回使う。
  - 当たり判定は、箱の外と、丸い角で切り取った所では子に届かない。ADR 0142 の規則で、自分では何も描かないので、子の当たらない所では当たらない（`HitTestSelf` が false）。`layout/hit.go` の一覧に加えた。
  - `core.Clipper` を実装し、`ChildClip` は箱を返す（丸い角でも矩形を返す。`Tester.Warnings` は丸ごと外にある物を除くだけなので、これで足りる）。
- `basic.Translate{Clip: true}` は、X と Y が 0 のときも切り取る。`ChildClip` も `Clip` のときは常に true を返す。`SlideTransition` は動いている間だけ `Clip` を立てる。止まった子のフォーカスの輪が箱の外へ出るのを、今まで通り切らないためである。
- `paint.TextLayout`、`paint.TextAnchor`、`paint.Canvas.DrawText(t, pos, anchor, color)` を足した。`color` は自分の色を持たないグリフの色で、`DrawGlyphs` と同じ。描くのはグリフだけで、スパンの背景は描かない。割り当ては無い。
- `text.Layout.Baseline()` を足した。最初の行のベースラインの、箱の上からの距離である。縦書きにはこの意味のベースラインが無いので 0（上端）を返す。
- `text.Labels`（`Shaper`、`Style`、`Get`、`GetBytes`）を足した。ゼロ値には `Shaper` が無く、`Get` は panic する。
- `paint.Path.AddArc`、`ArcTo`、`AddOval` を足した。円弧は 4 分の 1 周以下ごとの 3 次曲線で近似し、1 周を超える sweep は 1 周に切る。float32 の π は π より少し大きいので、分割の数にわずかな余裕を持たせ、半周を 2 本に保つ。`layout.RingPath` は内部の `arc` をやめ、これを使う（輪郭は前と同じ）。
- `control.CustomPaint`、`layout.Painter`、`text.Shaper` の godoc と、ガイド（en、ja）の text-and-ime の 1 文で、`DrawText` と `text.Labels` を案内した。
- カタログに `basic.ClipRRect`（`basic.ClipRect` も同じ項目）を加えた。

## 結果

- テストは、`ClipRect` が子の描画を箱で切り取ること、`ClipRRect` の角の外と箱の外の押下が子に届かず、角の丸みの内側と辺の中央では届くこと、子の無い所で下の層に押下が届くこと、`core.Clipper` を実装すること、半径 0 でテーマの `Group` になること、描画が割り当てをしないことを確かめる。`Translate{Clip: true}` は X と Y が 0 でも切り取る。
- `DrawText` のテストは、左上、上中央、右下、中央、ベースライン左、ベースライン右（Y は無視）の各位置にグリフが置かれること、大きさの違う 2 つのラベルのベースラインが同じ高さに揃うことを確かめる。`text.Labels` は同じ文字に同じ `Layout` を返し、色だけの変化では保ち、大きさの変化では組み直し、256 個を超えない。10 個の目盛りの数字を `GetBytes` と `DrawText` で描くフレームは割り当てが 0 である。
- `Path` のテストは、円弧の点が円の上にあること、正の sweep が時計回りであること、1 周を超える sweep が 1 周になること、`ArcTo` が離れた点を線でつなぎ、同じ点からは線を足さないこと、`AddOval` の範囲を確かめる。
- 進捗の輪のカタログの画像と golden は変わらない。
- 残したこと：任意の形（`Path`）で切り取るウィジェット、4 つの角を別々に丸める切り取り、楕円の弧、縦書きのラベルのベースライン。
