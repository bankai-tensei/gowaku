# 0048: カスタムタイトルバー（14a）

決定日: 2026-11-09

状態: 採用

## 背景

当初の計画の「ウィンドウ装飾」。メニューバーや検索欄をタイトルバーの中に置きたい（VS Code、Windows 11 のアプリ）。ヒアリングで Windows、X11、Wayland の 3 つで作ると決めた。Wayland には、デコレーションを持たないコンポジタ向けの自前 CSD が Phase 10 からある（ADR 0021）。

## 選択肢

1. `WindowOptions.TitleBar Widget` に中身を渡し、gowaku が枠を描く。
2. `WindowOptions.CustomFrame bool` で OS の枠を外し、利用者が自分の木の先頭に `TitleBar` ウィジェットを置く。

最初は 1 で作ったが、2 に変えた。タイトルバーの中身（メニューバー）は、ほとんどの場合アプリの状態（Action）を使う。1 だと中身が Home とは別の木になり、状態を渡す仕組みが別に要る。

## 決定

```go
type WindowOptions struct{ ...; CustomFrame bool }
type App struct{ ...; CustomFrame bool }
type TitleBar struct{ Child Widget }   // nil ならウィンドウのタイトル
```

- `TitleBar` は高さ 36。中身を左に、最小化・最大化・閉じるのボタンを右に描く（Wayland の自前 CSD と同じボタン）。ポインタを取る物の無い所をドラッグすると移動、ダブルクリックで最大化、右クリックでウィンドウメニュー。
- ルートは `frameState`（タイトル、フォーカス、最大化、スケール）を `Inherited` で配り、`TitleBar` はそれに従って描き直す。
- オーバーレイをタイトルバーの上にもかけるように、ルートの組み方を変えた（以前は Overlay がタイトルバーの下だけだった）。タイトルバーの中の MenuBar がメニューを開けるようにするため。
- `platform.WindowOptions.CustomFrame`、`platform.HitArea`（Client / Caption / Maximize）、`Window.SetHitTest(fn)` を足した。`ClientDecorated()` は CustomFrame のウィンドウでも true になる。

### Windows

- `WM_NCCALCSIZE` で、既定の計算をしたあとクライアント域の上端だけ元に戻す。左右と下の枠は OS のまま残るので、リサイズと DWM の影はそのまま。最大化中は画面の外にはみ出す枠の厚み（`SM_CYFRAME + SM_CXPADDEDBORDER`）だけ下げる。作成直後に `SWP_FRAMECHANGED`。クライアントの大きさの計算からはキャプションを除く。
- `WM_NCHITTEST`：まず `DefWindowProc` に聞いて、左右と下の枠と角はそれを返す。上端の枠の厚みの帯は `HTTOP` / `HTTOPLEFT` / `HTTOPRIGHT`。残りは app に聞く。app は木のヒットテストを深い方から見て、最大化ボタンの印（`renderMaxButton`）があれば `HTMAXBUTTON`、ポインタを取る物（PointerTarget）より先にタイトルバーのドラッグ（`renderTitleDrag`）に着けば `HTCAPTION`、どちらでもなければ `HTCLIENT`。
- `HTCAPTION` なので、移動、ダブルクリックでの最大化、右クリックのシステムメニュー、エアロスナップは OS がやる。
- `HTMAXBUTTON` を返すと、Windows 11 はボタンの上で**スナップレイアウト**を出す。代わりに、そこでのマウスはクライアントでなく非クライアントのメッセージ（`WM_NCMOUSEMOVE` など）で来る。これを `DefWindowProc` に渡すと古い最大化ボタンの追跡が始まるので、クライアントの座標に直して gowaku のポインタイベントとして積む。押下ではマウスをキャプチャし、離すのはクライアントのメッセージで来るようにする。`TME_NONCLIENT` で離れたことも受ける。

### X11

- `_MOTIF_WM_HINTS` で装飾なし（decorations = 0）。
- 移動：押下の root 座標を覚えておき、`XUngrabPointer` してから `_NET_WM_MOVERESIZE`（MOVE）をウィンドウマネージャーに送る。GTK の CSD と同じやり方。
- リサイズ：端から 5px（角は 12px）の押下を app に渡さず、方向付きの `_NET_WM_MOVERESIZE` にする。端の上ではフォントカーソル（`XC_top_side` など）を出す。端の判定は Wayland と共有にした（`platform/internal/frame`）。
- 最大化は `_NET_WM_STATE` のトグル、最小化は `WM_CHANGE_STATE`（IconicState）、ウィンドウメニューは `_GTK_SHOW_WINDOW_MENU`。最大化の状態は ConfigureNotify で `_NET_WM_STATE` を読んで知る。

### Wayland

- `CustomFrame` のときは xdg-decoration を頼まない（デコレーションのオブジェクトを作らなければクライアント側で描く決まり）。端のリサイズと `BeginMove` / `ToggleMaximize` / `ShowWindowMenu` は Phase 10 の CSD のものをそのまま使う。

## 結果

- app のテストで、メニューバーの上は Client、空いた所は Caption、最大化ボタンは Maximize、バーの下は Client と返り、タイトルバーの中の MenuBar が Alt+H から開くことを確かめた。
- Windows の実ウィンドウ（150%）で、クライアントが 1080×690（720×460 論理）になってキャプションが消えたこと、`WM_NCHITTEST` がメニューのタイトルで 1、空いた所で 2、最大化ボタンで 9、上端で 12、左上の角で 13、左の枠で 10 を返すこと、最大化ボタンの押下で最大化し、最大化中は上端が 2（リサイズでなくキャプション）になることを確かめた。
- sway（Wayland）では SSD が使えるコンポジタでも自前の枠で描き、WSLg の XWayland では装飾なしのヒントが付いて最大化のトグルが通ることを確かめた。
- **残したもの:** X11 のドラッグとリサイズは、実際にポインタを動かしては確かめていない（WSL にはポインタを動かせる X のウィンドウマネージャーが無い）。Windows 10 の上端の 1px の枠線は描かない（Windows 11 は不要）。タイトルバーの高さは 36 で固定。
