# 0050: ドラッグ & ドロップ（14b）

決定日: 2026-11-09

状態: 採用

## 背景

当初の計画の「ドラッグ & ドロップ」。完了条件は「エクスプローラー / Nautilus からファイルをドロップできる」。ヒアリングで「受け取りと送り出しの両方、型はテキストとファイル一覧」と決めた。

## 選択肢

1. アプリ内のドラッグは gowaku の中だけで扱い、OS の DnD は外とのやり取りにだけ使う。
2. アプリ内のドラッグも OS の DnD を通す。

2 を採る。同じ道を通れば、ウィンドウの外へ出るドラッグもそのまま動き、組み合わせの数が半分になる。型がテキストとファイルだけなので、OS を通して困る値は無い。

## 決定

### 1. API

```go
type DragData struct{ Text string; Files []string }                 // input
type DropEffect uint8 // DropCopy | DropMove | DropLink              // input
type DragTarget struct{ Kinds; Effect; OnHover func(bool); OnDrop func(DragData, Offset); Child }
type Draggable struct{ Data; Allowed; OnTap; OnDragStart; OnDragEnd func(DropEffect); Child }
```

- core に `DropTarget`（`DragMove` / `DragExit` / `Drop`）と `Tree.DispatchDrag`、`DragHost` を足した。ポインタの下で最も深く、受け取りを承諾した DropTarget が受ける。
- `DragTarget` は種類と効果を見て承諾する。ソースが望みの効果を許さなければ、許す効果に落とす。
- `Draggable` は 4px 動いたらドラッグを始める。動かなければ `OnTap`。中のボタンは自分の押下を持つので、ボタンの上からはドラッグしない。
- データは drop のときにだけ届く。ドラッグ中に分かるのは種類だけ（他のプログラムは drop まで中身を渡さない）。
- ドロップを受けた後のフレームは、プラットフォームが `EventRedraw` を積んで起こす。ドラッグ中のハンドラーは `WaitEvent` の中で同期的に呼ばれ、イベントを返さないため。

### 2. Windows（OLE）

- `OleInitialize`。ウィンドウごとに `RegisterDragDrop` で、Go で書いた `IDropTarget`（12a の `platform/win32/com`）を登録する。`DragEnter` / `DragOver` / `DragLeave` / `Drop` は UI スレッドで同期的に呼ばれるので、その場で app に聞く。`POINTL` は値渡しで、1 つのレジスタに入って来る。
- 中身は `IDataObject::GetData` で `CF_HDROP` と `CF_UNICODETEXT` を読む（形の変換は 14a の `clip`）。
- 送り出しは `IDataObject` を自作しない。シェルの汎用データオブジェクト（`SHCreateDataObject`）に `SetData` で詰め、Go で書いた `IDropSource` と一緒に `DoDragDrop` する。
- `DoDragDrop` はモーダルループなので、ツリーへの配送の途中では呼ばない。`StartDrag` は予約だけして、次の `WaitEvent` の入口で走らせる。その間も、移動やリサイズと同じ仕組み（ADR 0001）で描画を続ける。

### 3. X11（XDND 5）

- 受け取り：`XdndAware`。`XdndEnter`（型が 3 つを超えれば `XdndTypeList`）、`XdndPosition` に `XdndStatus` で答える。`XdndDrop` で `XdndSelection` を変換し（14a の INCR も通る）、`XdndFinished` を返す。
- 送り出し：ヘルパーウィンドウがソースになる。`XdndSelection` を持ち、`XdndTypeList` を付け、ポインタをつかむ。動くたびにポインタの下から `XdndAware` を持つウィンドウを探す（ウィンドウマネージャーの枠の中まで下る）。`Enter` / `Position` / `Leave` / `Drop` を送り、`Finished` で終える。Escape で取り消す。
- XDND の発信元は、どれもコピーを受け付ける約束なので、許す効果は「頼まれた効果 + コピー」とする。

### 4. Wayland

- 受け取り：`wl_data_device` の `enter` / `motion` / `leave` / `drop`。`wl_data_offer.accept`（断るときは NULL）と `set_actions`。データは drop で `receive` する。自分のドラッグを自分で受けるときは、パイプを通さずに元の値を使う（UI ゴルーチンがパイプで待つと、送る側が動けない）。
- 送り出し：`wl_data_source` に MIME を並べて `start_drag`（直前の押下の serial）。`dnd_finished` の `action` を結果にする。リンクは Wayland に無いので断る。

## 結果

- widgets と app のテストで、種類と効果の承諾、ホバーの出入り、drop の位置、タップかドラッグかの判定、プラットフォームを通した往復を確かめた。
- Windows：実物の OLE で、シェルのデータオブジェクトを Go の `IDropTarget` に通した。テキストとファイル、修飾キー、許されない効果の拒否、出入りの順を確かめた。
- X11（WSLg の XWayland）：2 つの接続の間で XDND が通った。
- Wayland（sway）：仮想ポインタで押してから `start_drag` し、別のウィンドウに落として move で終わるまで、コンポジタを通して確かめた。
- **残したもの:** エクスプローラーや Nautilus との実際の受け渡しは、マウスを動かす必要があるので確かめていない（レビューで手で試す）。ドラッグ中の画像（ドラッグイメージ）は出さない。アプリ内で任意の Go の値を運ぶドラッグは無い。
