# 0076: ウィンドウの閉じる要求を断る

決定日: 2026-11-09

状態: 採用

## 背景

notes サンプルで、未保存の文書があるウィンドウを OS の × で閉じると、「保存しますか」を聞けずに閉じてしまった。`WindowOptions.OnClose` は閉じた後の通知で、閉じるのを止められない。ヒアリングで、ライブラリで直すことにした。

## 決定

### 1. API

- `WindowOptions.OnCloseRequest func() bool` を足した。最初のウィンドウ用に `App.OnCloseRequest` も足した。false を返すとウィンドウは開いたままになる。nil ならすぐに閉じる。
- `WindowHandle.RequestClose()`: OS の × と同じく、閉じる前に聞く。
- `WindowHandle.SetOnCloseRequest(fn)`: オプションを差し替える。未保存の状態はたいていウィンドウ自身のツリーが知っているので、State から `WindowOf(ctx)` で設定できるようにした。Flutter の PopScope をウィンドウ単位にしたもの。
- 要求は UI ゴルーチンのイベントループから呼ぶ。Build やウィジェットのコールバックの中からは呼ばない。ブロックしてはいけない。Dialog や ShowModal を開いて false を返し、答えが出たら `Close()` を呼ぶ。要求の中から `Close()` や `Quit()` を呼んでもよい。
- **bool を返して後で閉じる形にした。** 閉じる関数やトークンを渡す案は採らなかった。閉じ方が 1 つで済み、新しい型が要らない。
- 専用のガードのウィジェットは作らなかった。同じことに仕組みが増えるだけ。

### 2. どの閉じ方が聞くか

| 閉じ方 | 聞くか |
|---|---|
| OS の × / Alt+F4 / ウィンドウメニュー / タスクバー（Win32 の WM_CLOSE、SC_CLOSE） | 聞く |
| X11 の WM_DELETE_WINDOW、Wayland の xdg_toplevel.close | 聞く |
| gowaku が描くタイトルバーの閉じるボタン（Wayland の CSD、`CustomFrame` の `TitleBar`） | 聞く（タップのコールバックが戻ってから） |
| スクリーンリーダー: UIA の Window パターンの Close | 聞く（HWND のホストのプロキシが WM_CLOSE を送る） |
| スクリーンリーダー: AT-SPI | フレームに Action が無いので閉じる操作が無い。タイトルバーの閉じるボタンを押すと、同じ要求を通る |
| `WindowHandle.RequestClose()` | 聞く（呼んだコールバックが戻ってから） |
| モーダルが開いているときのオーナーへの要求 | オーナーには聞かず、一番新しいモーダルを前に出す（ADR 0047 のまま） |
| モーダル自身の × | モーダル自身の `OnCloseRequest` に聞く |
| `WindowHandle.Close()`、Inspector を閉じる、タブが空になった切り離しウィンドウ | 聞かない |
| `Quit()` | 聞かない。`OnClose` は呼ぶ |
| オーナーを閉じるとモーダルも閉じる | モーダルには聞かない |
| X11 の DestroyNotify（新しい `EventDestroyed`） | 聞かない |
| WM_QUERYENDSESSION（シャットダウン、ログオフ） | 扱わない。DefWindowProc が許し、何も聞かない |

### 3. 決めたこと

- **呼ぶ時機をそろえる。** タイトルバーのボタンと `RequestClose` は `ap.post` を通す。こうして、要求がウィンドウ自身のウィジェットのコールバックの中で走らないようにした。`EventClose` は `handle` の中で直接聞き、許されればその場で閉じる（今までどおり）。
- **新しいプラットフォームのイベント `EventDestroyed`。** X11 の DestroyNotify が断れる閉じる要求として届くと、もう窓の無いウィンドウが残る。`Event` に Forced の旗を足す案は採らず、種類を分けた。
- **`Quit` は無条件のまま。** 全ウィンドウに聞く `RequestQuit` は採らなかった。複数のウィンドウのプロンプトが同時に開き、答えは非同期に返る。どの順で何を終えるかを、ライブラリは決められない。聞きたいアプリは自分で聞いてから `Quit` を呼ぶ。
- **Win32 は変更が要らなかった。** WM_CLOSE で 0 を返せばウィンドウは壊れない。WM_CLOSE と SC_CLOSE がそれぞれ `EventClose` を 1 つ出し、ウィンドウが残ることを gpu テストで確かめた。

### 4. notes

- 各ウィンドウが Build のたびに `SetOnCloseRequest` で要求を設定する。要求はいつも今のウィンドウを閉じる流れ（保存 / 保存しない / キャンセル）を始めて false を返し、答えが出たら自分で閉じる。
- 最後のウィンドウの × も聞いてから終了する。
- 2 回目の × や Ctrl+Shift+W でプロンプトを重ねない。
- 未保存のタブを別のウィンドウに渡す回避策は消した。
- メニューの「終了」は今までどおり聞かない。未保存の内容は自動保存とセッションに残り、次の起動で戻る。

## 残したこと

- シャットダウンとログオフ（WM_QUERYENDSESSION、logind の inhibit）で止める手段は無い。notes は自動保存で内容を失わない。
- X11 / Wayland の実機での確認は、ビルドと単体テストまで。
