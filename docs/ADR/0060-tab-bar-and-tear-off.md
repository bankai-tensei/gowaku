# 0060: TabBar とタブの引き剥がし（15b）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 §2 で、TabBar の範囲を次のとおり決めた。

- 閉じる、中クリックで閉じる、ドラッグで並べ替え、溢れたときのスクロールと一覧、Ctrl+Tab
- **タブを外へドラッグすると新しいウィンドウになり、別のウィンドウの TabBar に戻せる**（ブラウザと同じ）

15a の `Tabs` は中身まで持つ部品だったが、TabBar は帯だけの部品にする。中身は利用者が出す。

## 決定

### 1. API

- `TabBar{Tabs []TabItem, Selected, Label, OnSelect, OnClose, OnMove, OnNew}`。
- `TabItem{Key, Title, Icon}`。`Key` は、移動しても、別の帯に移っても変わらないタブの名前。
- タブのリストは利用者が持ち、帯はコールバックで知らせるだけ。`OnMove` の後には、選択中のタブの新しい位置で `OnSelect` を呼ぶ。
- 帯をまたぐ操作には `Group *TabGroup`、`OnReceive(key, index)`、`OnDetach(i)`、`OnTearOff(TabTearOff)` を使う。
- キーボード:
  - Ctrl+Tab / Ctrl+Shift+Tab / Ctrl+PageUp / PageDown で切り替える。Ctrl+W で閉じる。
  - Tab で止まる場所は 1 つで、矢印で動く。
  - ドラッグの代わりに Ctrl+Shift+← / → で並べ替えられる。

### 2. 引き剥がしの継ぎ目

widgets はルートのパッケージ（`OpenWindow` を持つ）を import できない。そこで次のように分けた。

- **ドラッグは元のウィンドウの木から出ない。** ポインタが窓の外に出ても、OS は元のウィンドウに移動を送り続ける。Win32 はボタンを押したときの `SetCapture`、X11 と Wayland は暗黙のグラブによる。
- 帯は、窓の中のポインタの位置を `core.ScreenOf` で画面の物理ピクセルに直す。物理ピクセルにするのは、モニターごとに論理ピクセルの大きさが違うため。
- 段階は「押した → ドラッグ中（4px の遊び） → 帯から 32px 離れると引き剥がし」。帯の近くに戻ると、ふつうの並べ替えに戻る。Escape で取り消す。
- **ポインタの下の帯を探す。**
  - `TabGroup` はただの共有の Go のオブジェクト。すべてのウィンドウが 1 つの UI goroutine を共有しているので、これで足りる。
  - 同じウィンドウの帯は窓の中の座標で、別のウィンドウの帯は画面座標と `ShowsScreenPoint` で探す。`ShowsScreenPoint` は重なり順を見る（Win32 は `WindowFromPoint` + `GetAncestor`、X11 は XdndAware の窓をたどる）。
  - ポインタの下の帯には、差し込む位置に半透明のタブを出し、隣のタブを横へずらす。半透明のタブはスクリーンリーダーには見せない。
- **落としたとき。**
  - 別の帯の上なら、その帯の `OnReceive` と `OnSelect` を呼ぶ。別のウィンドウならそれを前に出し、元の帯の `OnDetach` を呼ぶ。
  - それ以外の場所なら `OnTearOff` を呼ぶ。
- **新しいウィンドウの位置。** `Origin = Screen − (帯の窓の中の位置 + 掴んだ位置) × scale` にする。新しいウィンドウが帯を同じ場所に置けば、タブがポインタの下に出る。
  - ルートの `TearOffWindow(e, opts, root)` が、その位置にウィンドウを開く。
  - `Alone`（窓に最後の 1 枚しかない）のときは、新しく開かずに今のウィンドウを動かしてもよい（`WindowHandle.MoveTo`）。
- core に `ScreenHost`（`ScreenOrigin`、`ShowsScreenPoint`、`Activate`）を足した。platform には次を足した。
  - `Window` に埋め込む `Placement`: `ClientOrigin`、`MoveClientTo`、`ShowsScreenPoint`
  - `WindowOptions.Placed / ClientX / ClientY`

### 3. 環境ごとの対応

| | 状態 |
|---|---|
| Win32 | 全部。`ClientToScreen` / `SetWindowPos`（枠を考える）。作るときに置き先のモニターの DPI で大きさを直す。重なり順は `WindowFromPoint`。実機で確かめた |
| X11 | できる限り。`XTranslateCoordinates`、`XMoveWindow`（`WM_NORMAL_HINTS` の USPosition + StaticGravity）、重なり順は XdndAware の窓をたどる。コンパイルは通るが、X サーバーでは動かしていない |
| Wayland | 画面の座標が無い。`ClientOrigin` は「分からない」を返し、移動は無視する。引き剥がしはできるが、新しいウィンドウの位置はコンポジタが決める（`Placed=false`）。別のウィンドウへ戻すことはできず、同じウィンドウの帯だけが見つかる。xdg-toplevel-drag は範囲の外 |

### 4. 見た目

- **選択中のタブは Card の面にした。** 上の角が 8 の角丸で、Hair の線で囲む。帯の下の Hair の線に立ってそれを隠し、下の Card の中身とつながる（フォルダのタブと同じ）。
  - 15a の `Tabs` の板（Fill）にしなかったのは、これが書類のタブだから。選択中のタブは「選んだ項目」ではなく「その書類のページ」に見えるべきで、2 つの部品の見分けもつく。
  - アクセント色は使わない。
- ホバー中のタブは柔らかい Fill の形。区切りの線は、何もしていないタブの間にだけ出す。
- 高さ 34（タブは 30）。タブの幅は 120〜208 で分け合い、長い題名は「…」で切る。× はホバー中と選択中に出る。
- 並べ替え、閉じる、差し込むときは、`Key` ごとのスプリングで今の場所から動く。ReduceMotion では跳ぶ。

### 5. Semantics

- 帯は `RoleTabList`、タブは `RoleTab`（Selected、タップ）。
- × は `RoleButton` で、名前は「X を閉じる」/ "Close X"。

## 結果

- `examples/tabs` で、引き剥がしと戻しを実機（Windows、150%、ライトとダーク）で確かめた。
  - 「坊っちゃん」を帯から引き剥がすと、ポインタの下に新しいウィンドウが開いた。
  - それを最初のウィンドウの上へ戻すと、隣のタブがずれて差し込む位置が見えた。
  - 落とすと 3 番目に入って選択され、空になった 2 枚目のウィンドウは閉じた。
- golden は `tabbar`、`tabbar-dark`、`tabbar-drag`、`tabbar-drag-dark`。
- 共通のデザインテストの press の表には入れていない。タブは押した時点で選ばれ（ブラウザと同じ）、選択の状態は利用者が持つため。
- 残り:
  - 窓の外にいる間、タブの影が浮かんでいない。カーソルも変わらない。
  - 溢れた帯の端でドラッグしたときの自動スクロール。
  - タブを閉じた後の幅を、ポインタが離れるまで保つこと（ブラウザはそうする）。
  - 切れた題名のツールチップ。
  - X11 の実機確認、Wayland での戻し。
  - NVDA / Narrator で耳で確かめていない。
