# 0138: 色合いの上の文字の色、表のセルのウィジェットの置き方、タップの修飾キー、処理中のスピナー

決定日: 2026-11-17

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 5 つが見つかった。

1. **色合いの上の文字が 4.5:1 に届かない：** `gowakutest` の警告（コントラスト）が、ライトのテーマで `control.Chip` の文字を 4.45:1 と報告した。調べると、選択中の Chip（`AccentTint` の上の `AccentInk`）にポインターを重ねたときだった。アプリが `AccentTint` で塗った選択中の行（`ListView` や `ReorderableList` の行）の上では、控えめな `control.Badge`（`Fill` の上の `Ink2`）が 3.9〜4.4:1、ポインターを重ねた控えめな `control.Button`（`AccentInk`）と `ButtonDanger`（`Danger`）が 4.4:1 前後だった。`theme.Colors` は「文字の色はどの面の上でも、`Fill` の下でも 4.5:1」と約束するが、`AccentTint` は面を暗く（ダークでは明るく）するので、その上では約束が成り立たない。ADR 0133 は `SidebarItem` の `Detail` と件数だけを `Ink` に変えて直したが、ほかの部品と、アプリが塗った行には効かなかった。
2. **表のセルのウィジェットがはみ出す：** `collection.Table` の `CellWidget` の列で、上下に 4 の余白を持つ `Container` に `Body` の文字を入れると、文字が下にはみ出し、行ごとに警告が出た。セルのウィジェットには行の高さから上下 4 ずつを引いた 24 しか与えられず（既定の行は 32）、20 の行の高さの文字と 8 の余白が収まらなかった。godoc の「スクロールで別の行が来たときにセルを作り直す」という文は、親の状態が変わってもセルが変わらないように読めた（実際は、表を作り直すと見えている行のセルはすべて作り直される）。ヘッダーと行の高さも書いていなかった。
3. **タップに修飾キーが無い：** `interact.GestureDetector` のタップは修飾キーを渡さないので、Shift を押しながらのクリックで範囲を選ぶには、`Listener` で生のポインターのイベントを読むしかなかった。
4. **処理中のスピナーが無い：** `control.ProgressRing` と `control.ProgressBar` は進み具合を示すだけで、長さの分からない処理を示せなかった。
5. **`basic.Column` の交差軸の既定：** Flutter では中央だが、gowaku では先頭（`layout.CrossStart`）である。godoc に書いていなかった。

## 選択肢

### 色合いの上の文字

- **A. 文字の色そのものを、`AccentTint` の上でも読めるほど濃くする。** 計算すると、すべての面の上の `AccentTint` に `Fill` を重ねた地で 4.5:1 に届くには、ライトの `AccentInk`、`Danger`、`Success` をトーン 41 から 37.5 に、ダークでは 72〜74 から 81.5 にする必要がある。ダークでは色がほとんど抜けたパステルになり、ページ全体の見た目が変わる。
- **B. `AccentTint` を薄くする。** `Sidebar` の上に `Fill` を重ねた地では、色合いが無くても `AccentInk` はぎりぎりなので、アルファを 2〜3% まで下げても届かない。選択が見えなくなる。
- **C. 色合いの上に置くもののための色を、テーマが計算して渡す。** `theme.Colors.OnTint()` が、`Ink2`、`AccentInk`、`Danger`、`Success` のそれぞれを、`AccentTint` を重ねたすべての面の上で（`Fill` とホバーの下でも）4.5:1 に届く最小の量だけ `Ink` に寄せた色を返す。すでに読める色はそのまま。選択中の行を描く部品は、中身をその色で描く（`basic.Theme{Data: th.OnTint()}`）。ページ全体の色は変わらず、変わるのは色合いの上だけで、そこで必要な分だけである。アプリが塗った行は、アプリが同じ一行を書く必要がある。
- **D. `Container` の `Color` が `AccentTint` のとき、自動で C の色を子に渡す。** 選択が切り替わるたびに `Theme` が出入りして子の状態が失われる。常に包むと、すべての `Container` に要素が 1 つ増える。色の等しさで振る舞いが変わるのも分かりにくい。

### 表のセルのウィジェット

- **A. 上下の余白を 2 にし、ウィジェットに行の高さから 4 を引いた高さを与え、行の中央に置く。** 既定の行で 28 になり、上下 4 の `Container` の 1 行の `Body` が収まる。ボタン（24）やスパークラインは今まで通り中央に置かれる。
- **B. 行の高さをそのまま与える。** 高さいっぱいに広がるウィジェットが隣の行に触れる。
- **C. 既定の行を高くする。** 文字だけの表まで間延びする。

### タップの修飾キー

- **A. `OnTapMods func(input.Modifiers)` のような欄を足す。** 位置の欲しい人と修飾キーの欲しい人で欄が増え続ける。
- **B. `OnTap` を `func(input.Modifiers)` にする。** ほとんどの呼び出しは修飾キーを使わないのに、すべての呼び出しが引数を受けることになる。
- **C. 位置を渡す `At` の付く呼び出しが、位置と修飾キーを持つ `layout.Tap` を受け取るようにし、`OnTap` の `At` 版 `OnTapAt` を足す。** `OnTapDownAt`、`OnTapAt`、`OnSecondaryTapAt` が `func(t layout.Tap)` になる。引数の無い `OnTapDown` と `OnTap` はそのまま。`RenderGesture` も同じ形にする。

### 処理中のスピナー

- **A. `Indeterminate bool` を足す。** ゼロ値は今まで通りの進み具合で、WinUI の `IsIndeterminate` や ARIA の考え方と同じ名前である。
- **B. 負の `Value` を「分からない」とする。** 値の計算の結果が負になると、意図せずスピナーになる。読んで分からない。
- **C. 別の型 `Spinner` を作る。** リングと同じ描き方と同じセマンティクスを二重に持つことになる。

## 決定

- 色合いは案 C。`theme.Colors.OnTint()` と、色だけを差し替えた写しを返す `theme.Theme.OnTint()` を足す。地の候補は `Canvas`、`Sidebar`、`Card`、`Raised`、`Field` に `AccentTint` を重ねたものと、さらに `Fill` を重ねたもので、`Fill` はホバーの洗い（`Ink` の 5%）より強いのでホバーも含む。目標は 8 ビットの丸めの分の余裕を見て 4.6:1 とし、混ぜる量は 12 回の二分探索で決める。輝度の計算は 1 回あたり十数回の `math.Pow` で、毎フレームの経路ではない。
  - これを使う部品：選択中の `control.Chip`（文字とアイコン）、選択中の `collection.SidebarItem`（中身全体。ADR 0133 の `Ink` への置き換えは、これで置き換える）、`overlay` のメニューの光っている行のショートカット、`field.CommandPalette` の選択中の行の説明と一致箇所、`nav.Toolbar` のオンのボタン、`editor.Editor` の検索バーのオンの切り替え。
  - アプリが塗った行のために、`AccentTint` と `OnTint` の godoc に `basic.Theme{Data: th.OnTint()}` で中身を包むことを書く。
- 表は案 A。`tableWidgetInset` を 4 から 2 にする。ウィジェットの取れる大きさ（列の幅から左右 12、行の高さから上下 2）、中央に置くこと、ヘッダーと行の高さ（どちらも 32、行は `RowHeight` で変わる）、表を作り直すと見えている行の `Widget` がすべて呼ばれ直すことを、`Table` と `TableColumn.Widget` の godoc に書く。
- タップは案 C。`layout.Tap{Pos, Modifiers}` を足し、`interact.GestureDetector` の `OnTapDownAt`、`OnTapAt`（新設）、`OnSecondaryTapAt` と `layout.RenderGesture` の `OnDownAt`、`OnTapAt`（新設）、`OnSecondaryTapAt` がそれを受け取る。`OnTap` と `OnTapAt` が両方あれば `OnTap` が先に動く。修飾キーは、押下では押したとき、タップでは離したときのもの。呼び出し側（`overlay.ContextMenu`、カタログ、テスト）を直した。
- スピナーは案 A。`control.ProgressRing.Indeterminate` と `control.ProgressBar.Indeterminate` を足す。リングは 28% の弧が 0.9 秒で 1 回転し、バーは幅の 30% の帯が 1.4 秒で横切る。動きは `layout.RenderProgressRing` と `layout.RenderProgressBar` の `Busy` が描き、スケルトンの光と同じく、描かれて見えている間だけティッカーが動く（スクロールで外れた、クリップされた、透明な `Opacity` の下、木の外では止まる）。動きを減らす設定では、リングの弧は 12 時を中心に、バーの帯は中央に止まり、フレームを求めない。止まった弧が 28% の進み具合に見えないよう、弧は 12 時から始めず 12 時を中心に置く。セマンティクスは `RoleProgress` で、値も範囲も持たない（範囲の無いノードには、プラットフォームの層が UIA の RangeValue や AT-SPI の Value を付けない。`SkeletonGroup` の処理中の表示と同じ形である）。ラベルが無ければ、ウィンドウの言語の「読み込み中」（`Strings.Loading`）を名前にする。
- `basic.Row`、`basic.Column`、`basic.VStack` の godoc に、交差軸の既定が先頭（`CrossStart`）で、Flutter や SwiftUI と違って中央ではないこと、中央にするには `Cross: layout.CrossCenter` と書くことを書く。

## 結果

- 色合いの上のライブラリーの文字は、どのアクセントでもライトとダークで 4.5:1 に届く。`theme` のテストは、12 のアクセント（灰色を含む）のライトとダークで、`OnTint` の `Ink`、`Ink2`、`AccentInk`、`Danger`、`Success` が、すべての面の上の `AccentTint`（`Fill` とホバーの下も）で 4.5:1 に届くこと、`Ink`、`Ink3`、`Fill`、`Accent`、`AccentTint` を変えないこと、すでに読める色を動かさないことを確かめる。`gowakutest` のテストは、8 つのアクセントのライトとダーク、4 つの面のそれぞれで、Chip（タグ、絞り込み、ボタン、選択中）、控えめな Badge、控えめなボタンと危険なボタンを、そのままとポインターを重ねた状態で描き、選択中の行（`OnTint` で包んだもの）の上でも警告が出ないことを確かめる。
- `SidebarItem` の選択中の行の `Detail` と件数は、`Ink` ではなく、読める最小の量だけ濃くした `Ink2` になり、ラベルとの差が少し戻った。カタログの Sidebar、Chip、Toolbar の絵を描き直した。
- 選択中の Chip を選択中の行の上に置くと、色合いが 2 重になり、`OnTint` の計算の外になる（4.0〜4.4:1）。まれなので、そのままにした。
- 表のセルのウィジェットは、既定の行で 28 の高さまで取れ、行の中央に並ぶ。テストは、上下 4 の `Container` の文字が警告を出さず、行の中央にあること、表の親が `SetState` すると見えている行のセルが新しい値を示すことを確かめる。
- Shift や Ctrl を押しながらのクリックは、`GestureDetector` の `OnTapAt` で読める。テストは `gowakutest` の `Hold` で、押下、タップ、右クリックが修飾キーと子からの位置を受け取ること、順番が押下、`OnTap`、`OnTapAt` であること、`OnTapAt` だけでも主ボタンを取ることを確かめる。
- スピナーのテストは、値も範囲も持たないこと、ラベルが無いと「Loading…」と読むこと、動いている間はティッカーが動いて絵が変わること、動きを減らす設定では止まってフレームを求めないこと、透明な `Opacity` の下ではフレームを求めないこと、ゼロ値では今まで通り値を読んでフレームを求めないことを確かめる。カタログの ProgressRing と ProgressBar に不確定のものを足し、説明を書き足して絵を描き直した。
- 木の読み込み中の行のスピナー（`collection.Tree`）は、別の描き方のまま残した。
