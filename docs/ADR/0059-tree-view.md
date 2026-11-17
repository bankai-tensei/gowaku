# 0059: TreeView（15b）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 §2 で、TreeView の範囲を次のとおり決めた。

- 開閉、キーボード、仮想化
- 子の遅延読み込み
- ドラッグでの並べ替え
- 複数選択
- F2 での名前変更

## 決定

### 1. API: キーの関数とコントローラ

- `TreeView[K comparable]{Roots, ChildrenOf, HasChildren, LabelOf, IconOf, ...}`。
- **木はインターフェースではなく関数で受ける。** ファイルの一覧のような用途では、パスを受け取る関数をそのまま渡せる。アダプタの型が要らない。
- 開いているノード、選択、焦点は `TreeController[K]` に持つ（`FormController` と同じ形で、ゼロ値から使える）。フォルダを開くたびに利用者が作り直さずに済む。渡さなければ木が自分で持つ。
- コールバック: `OnSelectionChange`、`OnActivate`（Enter とダブルクリック）、`OnLoad`、`OnRename`、`OnMove`、`CanDrop`。`OnRename` が無ければ F2 は効かず、`OnMove` が無ければドラッグは効かない。

### 2. 仮想化

- 見えている行を、使い回すスライスに平らに並べる。
- 並べ直すのは、`DidUpdateWidget`、コントローラの開閉の世代が変わったとき、開閉したときだけ。スクロールではデータの関数を呼ばない（ADR 0034 (a-2) と同じ理由）。
- ListView ではなく自前の RenderObject にした。**焦点の行は、スクロールで見えなくなっても作っておく。** そうしないと、外れた RenderFocus がフォーカスを落とし、Tab で止まる場所とキーボード操作が失われる。
- 行そのものが FocusTarget、PointerTarget、Semantics のノードを兼ねる。だから UIA のフォーカスが TreeItem に乗る。
- ベンチ（10.1 万行を開いた状態）: スクロール 1 段 1.3µs、Layout + Paint 1.4µs、平らに並べ直す 0.94ms。どれも 0 allocs/op。

### 3. 遅延読み込み

- 「読み込み中」は、開いていて、`HasChildren` が true で、`ChildrenOf` が何も返さない状態を指す。
- 利用者が開いたときは、すぐ `OnLoad` を呼ぶ。外から開いたときは、Build からは呼べないので、次のフレームで呼ぶ。
- 1 回開くごとに要求は 1 回。閉じると忘れるので、開き直すと再び要求する。
- 読み込み中の行は、回る弧と「読み込み中…」を出す。

### 4. ドラッグでの並べ替え

- **Draggable / DragTarget ではなく、木の中のドラッグにした。** OS のドラッグは、文字とファイルしか運べない。ホバー中の位置も取れず、DoDragDrop の modal ループにもなる。木の中の並べ替えには、Go のキー、前 / 後 / 中という落とし先、自動で開くこと、自動スクロールが要る。
- 落とし先の決め方:
  - 開けるノードの上では、上下の 1/4 が前と後、真ん中が中。葉では半分で分ける。
  - 子のある開いたノードの「後」は、最初の子として入れる。
- 断る落とし先: 自分自身、自分の子孫、開けないノードの中。`CanDrop` で利用者がさらに断れる。
- 閉じたノードの上に 700ms 留まると開く。端に近づくと最大 900px/s で自動スクロールする。Escape で取り消す。
- 印: 前と後は、点の付いた 2px の Accent の線。中は Accent の枠と 12% の塗り。

### 5. 複数選択とキーボード

- ARIA の tree パターンに合わせた。
  - ↑↓、Home / End、PageUp / PageDown で動く。
  - → は、閉じていれば開き、開いていれば子へ入る。← は、開いていれば閉じ、閉じていれば親へ戻る。
  - Enter、F2。
- 先頭の文字で飛ぶ操作と `*` は、`HandleText` で受ける。だから日本語のラベルでも、どのキー配列でも効く。
- 複数選択のとき:
  - Shift+矢印で選択を広げる。Ctrl+矢印は焦点だけ動かす。Space で切り替える。Ctrl+A ですべて選ぶ。
  - マウスは Ctrl / Shift のクリック。シェブロンを押すと、選択を変えずに開閉する。行の下を押すと選択が消える。

### 6. 名前の変更

- ラベルの上に TextField を出し、拡張子を除いた部分を選んだ状態で始める（Explorer / Finder と同じ）。
- Enter で確定して行に焦点を戻す。欄を離れても確定する。Escape で取り消す。
- 空白だけや変わらない名前では、`OnRename` を呼ばない。

### 7. 見た目

- 行は高さ 28、1 段ごとに 16 字下げ、内側に 4 寄せた角丸 6 の形。ホバーは Fill。
- **選択の色は、キーボードが木にあるときは AccentTint、外にあるときは FillStrong の灰色**（Finder と同じ）。
- 焦点の輪は行の内側に描く（2px、AccentInk、透明度 60%）。外側に描くと隣の行にはみ出すため。
- シェブロンは Ink3 で、snappy のスプリングで回る。ReduceMotion では跳ぶ。
- 共通のデザインテストの hover の表には入れていない。メニューの行と同じく、行のホバーは補間せずすぐに出すため。

### 8. Semantics

- `RoleTree`（複数選択なら `Multiselectable`）。
- 項目は `RoleTreeItem` で、Label、Level、Expanded / Collapsed、Selected、タップと開閉の操作を持つ。
- `semantics.Node` に `PosInSet` と `SetSize` を足した。UIA の PositionInSet / SizeOfSet / Level、AT-SPI の level / posinset / setsize に対応する。仮想化しても「5 項目中 2 番目」と読めるようにするため。
- gallery を UIA から操作して確かめたこと:
  - Tree「フォルダ」の TreeItem に開閉と選択の状態が出る。
  - ExpandCollapse の `Expand()` で docs が開く。
  - SelectionItem の `Select()` で選択が移り、パンくずリストが追従する。

## 結果

- golden は `tree`、`tree-dark`、`tree-rename`、`tree-drag`、`tree-drag-dark`。
- 残り:
  - 最後の行の下には落とせない。また、ポインタの横の位置では、グループの最後の子の後ろの深さを選べない。
  - 閉じた親の中の選択はそのまま残る。
  - ラベルの省略表示（…）が無い。
  - シェブロンの回転は、最後に開閉したものだけが動く。
  - 右から左の書字方向での矢印キー。
  - キーが変わる名前の変更では、利用者が `OnRename` の中で新しいキーを `Select` する。
  - NVDA / Narrator で耳で確かめていない。
