# 0047: モーダルウィンドウ（14a）

決定日: 2026-11-09

状態: 採用

## 背景

当初の計画の「モーダルウィンドウ」。v1 の `ShowDialog` は同じウィンドウの中のオーバーレイで、ウィンドウの外に出せない。設定画面のように別ウィンドウで開き、閉じるまで元のウィンドウを触らせないものが要る。ヒアリングで「結果はコールバックで受ける（`ShowModal(opts, root, onClose)`）、イベントループは 1 本のまま」と決めた。

## 選択肢

1. 入れ子のイベントループで、閉じるまで返らない `ShowModal` を作る（Win32 の `DialogBox` 流）。書きやすいが、再入の契約（Build 中に呼ばれたら、閉じる前に元の木が変わったら）が難しく、macOS の `beginSheet` とも合わない。
2. すぐ返し、結果はコールバックで受ける。

## 決定

2 を採る。

```go
func ShowModal(ctx BuildContext, opts WindowOptions, root Widget) (*WindowHandle, error)
func WindowOf(ctx BuildContext) *WindowHandle
```

- オーナーは ctx のウィンドウ。結果は `opts.OnClose` で受ける（既存の `WindowOptions.OnClose` をそのまま使う）。中から閉じるには、返った `*WindowHandle` を閉包で掴んで `Close` するか、`WindowOf(ctx).Close()`。
- `platform.WindowOptions` に `Owner Window` と `Modal bool`、`platform.Window` に `Activate()` を足した。

| OS | やること |
|---|---|
| Windows | `CreateWindowEx` の親にオーナーを渡す（所有ウィンドウ。タスクバーにボタンが出ず、常にオーナーの前）。最小化ボタンを外し、オーナーの中央（モニターの作業領域の内側）に置く。`EnableWindow(owner, FALSE)`。閉じるときは **DestroyWindow の前に** `EnableWindow(owner, TRUE)`（後だと Windows が別のアプリを前に出す） |
| X11 | 地図に載せる前に `WM_TRANSIENT_FOR`、`_NET_WM_WINDOW_TYPE_DIALOG`、`_NET_WM_STATE_MODAL`。配置はウィンドウマネージャーに任せる。`Activate` は `_NET_ACTIVE_WINDOW` |
| Wayland | `xdg_toplevel.set_parent`。`xdg-dialog-v1`（モーダルの印）はまだ束ねていない。`Activate` は xdg-activation のトークンが要るので何もしない |

- **入力を止めるのは app 層。** オーナーにモーダルの子がある間、そのウィンドウへのポインタ、キー、文字、IME、スクリーンリーダーの操作、閉じる要求を捨てる。Windows は無効化で OS が止めるので二重になるが、X11 / Wayland はウィンドウマネージャーが止めてくれない（Weston の xwm は MODAL の状態を消しさえする）。オーナーを押されたり閉じる要求が来たりしたら、モーダルの方を `Activate` する。
- 閉じる順：オーナーを閉じると、先にモーダルの子を新しい順に閉じる（それぞれの `OnClose` が走る）。モーダルが閉じてオーナーに子が残らなければ、オーナーを `Activate` する（Windows は自分で戻すが、X11 では戻らないことがある）。
- オーナーへの閉じる要求（タイトルバーの ×）は、モーダルが開いている間は捨てる。プログラムからの `Close` と `Quit` は通す。

## 結果

- app のテスト（偽の platform）で、オーナーへのキー・押下・閉じる要求が捨てられてモーダルが前に出ること、閉じるとオーナーに戻ってキーが届くこと、オーナーを閉じるとモーダルが先に閉じることを確かめた。
- Windows の実ウィンドウで、メニューから開いたモーダルがオーナーに所有され、開いている間オーナーが `IsWindowEnabled = FALSE`、Escape で閉じると TRUE に戻ることを確かめた。
- X11（WSLg の XWayland）で、3 つのプロパティが付くことを確かめた。Weston の xwm は地図に載せたあと `_NET_WM_STATE` から MODAL を消す。止めるのは app 層なので動きは変わらない。
- **残したもの:** Wayland の `xdg-dialog-v1` と xdg-activation。どちらもプロトコルの XML を足して `go generate ./platform/wayland/` で束ねる作業になる。
