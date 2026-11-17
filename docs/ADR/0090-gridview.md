# 0090: GridView（仮想化グリッド）

決定日: 2026-11-13

状態: 採用

## 背景

当初の計画の `GridView`。仮想化は `List` / `Tree` / `Table` の 1 次元だけで、`basic.Grid` は子を全部作って組む。サムネイル一覧やファイルのアイコン表示は数万〜数十万の項目を並べるので、見えている行だけを作る 2 次元のグリッドが要る。列数は幅から決め、選択とキーボード移動は既存の部品に揃える。

## 選択肢

1. `layout` に `RenderGridView` を足し、`RenderList` と同じく `ScrollOffset` に直接つなぐ。
2. `layout.RenderList` をそのまま使い、1 行を 1 子として「タイルを横に並べる行」を作る。
3. `Tree` と同じく、`collection` の中に自前の本体（render object）を持ち、`ScrollState` の `OnMove` で行を動かす。

## 決定

選択肢 3。`layout` にも `List` にも手を入れない。

- 2 は `RenderList` の行の高さが 1 種類なので、上下の Padding を表せない。フォーカスのあるタイルを範囲外でも残す（Tab の止まり先を失わない）こともできない。
- 1 は `layout` の共有ファイルに型が増える。`Tree` がすでに 3 の形で 10 万行を 0 allocs で動かしているので、同じ形にした。

### 1. API

```go
type GridView struct {
    ItemCount int
    Columns int; MinColumnWidth float32       // basic.Grid と同じ名前
    Spacing, RunSpacing float32
    TileHeight, AspectRatio, ExtraHeight float32
    Padding geom.Insets
    Direction text.Direction                  // DirectionRTL で右から並べる
    Label string; LabelOf func(i int) string  // スクリーンリーダー向けの名前
    Select TableSelectMode
    Controller *TableController
    OnActivate func(i int); OnSelectionChange func()
    ItemBuilder func(i int) core.Widget
}
```

- 寸法の名前と意味は `basic.Grid` に合わせた。違うのは既定値だけで、`Columns` も `MinColumnWidth` も 0 なら最小 120 の列、`TileHeight` も `AspectRatio` も 0 なら正方形。仮想化には高さが決まっている必要があるので、「行ごとに最も高い子に揃える」は無い。
- タイルの高さはすべて同じ。行 r の上端は `Padding.Top + r*(高さ+RunSpacing)` で、見えている行は割り算で求まる。項目数に依らない。
- 子は幅いっぱい・高さいっぱいに組むが、四辺に 4px の余白を残す。選択の地とフォーカスリングをそこに描く。

### 2. 選択とフォーカスは TableController を使い回す

`List` には選択もキーボードも無い（ADR 0034、0078）。行の添字で選択を持つ既存の型は `TableController` で、範囲の集合なので 10 万項目の全選択も範囲 1 つで済む。並行する `GridController` は作らず、これを使う。

- コントローラの「行」をグリッドの項目とみなす。`FocusedCell` の行がフォーカスのある項目で、列は使わない。`SelectAll` は最後に作ったときの `ItemCount` を全部選ぶ。
- 選択の規則は `Table` と同じ（`selectItem` は `tableCore.choose` と同じ分岐）。クリックで 1 つ、Ctrl+クリックで足す・外す、Shift+クリックで項目の順の範囲、Ctrl+A で全部、Space でフォーカスの項目を足す・外す、`TableSelectMode` で個数を絞る。Ctrl+クリックはアンカーをその項目に移す。
- `datatable_*.go` は他の作業が触っているので、`choose` を共通化はせず同じ分岐を写した。`TableController` の非公開メソッド（`add` / `remove` / `clip`）と `tableListener` はそのまま使った。

### 3. キーボード

- グリッドは Tab で 1 回だけ止まる。フォーカスのあるタイルだけが `CanFocus`（`Tree` の行と同じ）。
- 矢印は見たままに動く。左右は項目の順に 1 つ進み、行の端では隣の行へ回り込む（エクスプローラーと Finder のアイコン表示と同じ）。上下は同じ列。最後の行が短く、真下に項目が無いときは最後の項目へ行く。最後の行で下、最初の行で上は動かない。
- Home / End は最初と最後の項目（Ctrl があっても同じ）。PageUp / PageDown は見えている行数 −1 だけ同じ列で動く。
- Shift は範囲を広げ、Ctrl はフォーカスだけを動かす（`Table` と同じ）。Enter とダブルクリックが `OnActivate`。
- RTL では左が「次」、右が「前」。
- タイプアヘッドは付けない。仕様は「List にあれば」で、List には無い。`LabelOf` があるので後から足せる。

### 4. 幅が変わったとき

- 列数が変わったら、ビューの先頭にあった項目を先頭に保つ。行の中のどこまでスクロールしていたか（ピクセル）も保つ。
- フォーカスのある項目が見えていたなら、そちらを基準にする。
- 列数が変わると行の中身が入れ替わるので、タイルの Element は作り直しになる（行がキー）。フォーカスを失ったタイルは `lostFocus` で取り戻す。

### 5. Semantics

- 本体が `RoleGrid` を開き、`RowCount`（全行数）と `ColumnCount` を言う。見えている行と、フォーカスのある行だけが `RoleRow`（`RowIndex`）を開き、タイルは `RoleGridCell`（`RowIndex`、`ColumnIndex`、`Selected`、Tap と ScrollIntoView）になる。
- UIA の `GridItem` は `RoleRow` の子を数えるので、行の render object を 1 段挟んだ（タイルを本体に直接並べると、行のノードの矩形が作れない）。
- `TableSelectMultiple` のときだけ `Multiselectable`。

### 6. RTL

ウィンドウ全体の向き（Directionality）はまだ無い（当初の計画で対象外）。`Direction text.Direction` を受け、`DirectionRTL` なら右上から並べる。Auto と LTR は左から。全体の向きができたら Auto をそれに従わせる。

## 結果

- 本体・行・タイルの Layout と Paint はアロケーションしない。スクロールで新しい行が入らないフレームは build も走らない。
- 性能（i7-12700H、10 万項目、8 列、800×600）:

| ベンチ | 結果 |
|---|---|
| 同じ行の中でスクロール（tick, build, layout, paint） | 6.8µs/frame、0 B/op、0 allocs/op |
| 見えているタイルを全部組み直して描く | 12.0µs、0 allocs/op |
| 1 画面ぶん新しい行が入る | 253µs、1887 allocs/op（タイルの widget を作るため。List と同じ） |

## 残したこと

- 行ごとに高さが違うグリッド、メイソンリー。
- ラバーバンド（ドラッグでの範囲選択）、ドラッグでの並べ替え。
- タイプアヘッド。
- コードからのジャンプ（`ListController.JumpToIndex` 相当）。今は `SetFocusedCell` の後にキーで動かすか、利用者のスクロールだけ。
