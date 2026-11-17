# 0058: DataTable（15b）

決定日: 2026-11-09

状態: 一部を ADR 0127 と ADR 0150 で置き換え

## 背景

ADR 0053 §2 で、DataTable の範囲を次のとおり決めた。

- 仮想化、列のリサイズ、ソート
- 行の複数選択
- その場のセル編集
- 列の並べ替えと左端の固定列
- ヘッダーからの絞り込み

完了条件は「100 万行が 120fps」。

## 決定

### 1. API

- `DataTable[T any]{Rows []T, Columns []DataColumn[T], ...}`。列は `Text func(T) string` で利用者の型を読む。`any` もリフレクションも使わない。`Rows` はスライスのヘッダーなので、表は行を写さない。データが別の場所にあるなら、並び順のスライスを渡して `DataTable[int]` にできる（gallery はこの形）。
- ソートと絞り込みは表の仕事にしない。`OnSort(TableSort)` と `OnFilter(column, TableFilter)` を呼ぶだけにした。100 万行を表の中で並べ替えると、利用者のデータと二重になるため。
- 選択、焦点のセル、列の幅と順番は `DataTableController` に持つ（`FormController` と同じく、ポインタでゼロ値から使える）。渡さなければ表が自分で持つので、ゼロ値の表でも選択とリサイズが動く。
  - 選択は行の範囲の集まり（`RowRange`）で持つので、100 万行で Ctrl+A をしても範囲が 1 つで済む。
  - 添字は `Rows` の添字なので、ソートや絞り込みの後は利用者が選択を消すか付け直す。
- 選択の方式は `TableSelectMultiple`（ゼロ値）、`TableSelectSingle`、`TableSelectNone`。

### 2. 100 万行で 120fps にした方法

- **セルは表が自分で描く。** 1 つの RenderObject が、見えている行をデータから直接描く。スクロールの 1 段では何も Build しない。
- **子の RenderObject は、フォーカス、ホバー、Semantics のためだけにある。** 行とセルのオブジェクトを「見えている行 + 2」だけ用意して使い回す。行 i はいつもオブジェクト `i % pool` を使うので、見えている間はノードの ID が変わらない。
- **整形した文字はキャッシュする。** 120 回の描画で使われなかったものは捨てる。
- **行の高さは固定にした（既定 32）。** 見える範囲とポインタの下の行が掛け算で決まり、スクロールの範囲も正確になる。代わりに、行ごとに高さを変えられない。また float32 なので、100 万行の下の方では 1px 単位でしか動かない。
- ベンチ（i7-12700H）: Layout + Paint 4.8µs、スクロール 1 段（tick + build + layout + paint）4.9µs、どちらも 0 allocs/op。

### 3. キーボードと編集

- Tab で止まる場所は 1 つで、焦点のセルに入る。矢印、Home / End、Ctrl+Home / End、PageUp / PageDown で動く。Shift 付きは選択を広げ、Ctrl 付きは選択を変えずに焦点だけ動かす。Space は行の選択を切り替える。
- 行 0 から ↑ でヘッダーの行に上がる。ヘッダーでは Enter でソート、Alt+↓ で絞り込みを開く。
- 編集は Enter、F2、または編集できるセルで文字を打つと始まる（表計算と同じ。Space では始まらない）。Enter で確定、Escape で取り消し、Tab / Shift+Tab で前後の編集できるセルへ移る。確定は `OnCommit` に聞き、false なら編集を続ける。編集欄は既定で TextField で、`Editor` で差し替えられる。

### 4. 列

- ヘッダーの端をドラッグして幅を変える（最小幅あり）。端には列幅を変えるカーソルを出す。このカーソルは 15b で `input` に足した（ADR 0062）。
- ヘッダーをドラッグして並べ替える。浮いたヘッダーと、落ちる場所の 2px の線を出す。
- `Frozen` 本の左端の列は、横にスクロールしても動かない。その境に Hair の線を引く。
- コールバックとコントローラの列の添字は、見えている順ではなく `Columns` の添字。

### 5. 絞り込み

ヘッダーの漏斗を押すと、カードが開く。中身は文字の欄と、`FilterValues` があればその一覧と「すべて」のチェック。変えるたびに `OnFilter` を呼ぶ（その場で絞り込む）。漏斗は、ホバー、焦点、絞り込み中のときだけ見える。絞り込み中は AccentInk。

### 6. 見た目

- ヘッダーは Ink2 の Caption（ウェイト 600）で、下に Hair の線、列の間に短い Hair の区切り。
- 行は高さ 32、ホバーは Fill、選択は AccentTint、縞は無し。
- 焦点のセルは、内側に 2px の AccentInk の輪。

### 7. Semantics と UIA

- grid には `Multiselectable` と、`RowCount`（行 + ヘッダー）と `ColumnCount` を付ける。
- 行は `RoleRow`（`Selected`、タップで選択）、セルは `RoleGridCell`。どちらにも `RowIndex` / `ColumnIndex`（1 から）を付ける。
- **仮想化すると、見えている行のノードからは表の大きさが分からない。** そこで `semantics.Node` に `RowCount`、`ColumnCount`、`RowIndex`、`ColumnIndex` を足した。
- 列見出しは、ソートの状態を Hint（「昇順」など）で持つ。タップでソートし、漏斗は子のボタン「絞り込み」。
- **行に名前を付けた。** gallery を UIA で操作したところ、選択（`GetSelection`）が名前の無い行で返ってきた。Chromium と同じく、見えている順のセルの文字をつないで行の名前にした。
- UIA には Grid、GridItem、Table、TableItem、Selection のパターンを足した。
  - Selection は、RadioGroup、TabList、List、Grid、Tree と、`Multiselectable` のノードに付く。`CanSelectMultiple` は `Multiselectable` から決まる。
  - 位置は新しい欄から取り、無ければ兄弟から数える。だからカレンダーの日にも付く。
- AT-SPI は属性（rowcount、colcount、rowindex、colindex）だけ。
- gallery を UIA から操作して確かめたこと: DataGrid「ファイル」が 100001×4 で CanSelectMultiple、セルを `Select()` するとその行が選ばれ、インスペクタが追従する。

## 結果

- golden は `table`、`table-filter-open`（それぞれライトとダーク）。
- 残り:
  - UIA の AddToSelection / RemoveFromSelection（Semantics に対応する操作が無い）。
  - AT-SPI の Table / TableCell / Selection インターフェース。
  - 見えていない行に対する `Grid.GetItem`（VirtualizedItem パターンが無い）。
  - 長い文字の省略表示（…）、ドラッグ中の自動スクロール、ドラッグでの範囲選択。
  - 「すべて」のチェックの中間状態。
  - 確定を断られた編集を欄から離れて閉じたとき、打った文字が消える。
  - NVDA / Narrator で耳で確かめていない。
