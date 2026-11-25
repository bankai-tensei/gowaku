# 0161: 終了の要求、ウィンドウを隠す、通知の許可

決定日: 2026-11-25

状態: 採用

## 背景

gowaku v0.1.0 で作った macOS の todo アプリで、期限の通知とメニューバーへの常駐を作ったところ、次の 3 点が足りなかった。アプリは purego/objc で gowaku の内部を書き換えてしのいでいた。

1. `Notify` が、`requestAuthorizationWithOptions:completionHandler:` の答えを待たずに `addNotificationRequest:` を送っていた。completion handler も空だった。そのため、許可のダイアログが出ている間の最初の通知は失われうるし、拒否されていても（「Notifications are not allowed for this application」）`Notify` は nil を返していた。ドキュメントの「表示できないときは `ErrNoNotifications`」と食い違う。
2. app delegate が `userNotificationCenter:willPresentNotification:withCompletionHandler:` を実装していなかった。この場合 macOS は、アプリが前面にある間はバナーを出さない。
3. 常駐アプリ（閉じてもメニューバーに残るもの）を作れなかった。`applicationShouldTerminate:` が全ウィンドウに `EventClose` を送るので（ADR 0082）、`OnCloseRequest` からは、閉じるボタン・⌘W と、⌘Q・Dock の「終了」・ログアウトを区別できない。ウィンドウを隠す手段も無く、最後のウィンドウが閉じると `Run` が終わる。

あわせて、`core.NewTree` がツリーを組み立ててから `SetHost` していたため、最初の Build の `InitState` から呼んだ `ShowTray`、`Notify`、クリップボードは、ホストが無いものとして失敗していた。`ShowTray` のドキュメントは「InitState から呼べる」としていた。ガイドの「`InitState` から `Post` する」は、この不具合を避ける書き方だった。

## 決定

### 1. 通知の許可

```go
func RequestNotifications(ctx BuildContext, done func(error))
```

- **許可を求める関数を足した。** 答えは `done` に、UI ゴルーチンで、呼び出しが戻った後に届く。nil なら `Notify` は表示でき、そうでなければ `ErrNoNotifications`。`ShowFileDialog` と同じく `ctx` のホストを通るので、`gowakutest` でも動き、`Run` の外では `ErrNoNotifications`。
- **`Notify` も許可を待つ。** 答えが出る前の通知は、作った `UNNotificationRequest` を保持して待たせ、許可されたら送り、拒否されたら捨てる。拒否が分かった後は `ErrNoNotifications` を返す。最初の 1 件だけは nil を返すことになるが、その時点では答えが無いので、ブロックしない限りこれ以上は分からない。答えを知りたいアプリは `RequestNotifications` を先に呼ぶ。
  - 案として「答えが出るまで `Notify` を止める」は採らなかった。UI ゴルーチンの上でダイアログの答えを待つことになる。
  - 案として「`Notify` に完了のコールバックを付ける」も採らなかった。ほとんどの呼び出しは答えを使わず、許可は通知ごとではなくアプリに 1 つしかない。
- **拒否の後も聞き直す。** 拒否の後の `Notify` は、エラーを返したうえで macOS にもう一度聞く。2 回目からはダイアログが出ず、今の設定がすぐに返る。こうして、利用者がシステム設定で許可したら、次の通知から出るようにした。`RequestNotifications` も毎回聞く。
- 答えのブロックは UserNotifications のキューで呼ばれる。状態は `notifyState` のミューテックスで守り、`addNotificationRequest:` はそのスレッドからそのまま送る（スレッドを問わない API）。
- **Windows** はトレイのバルーンに許可を求めないし、表示するかどうかも教えないので、トレイの隠しウィンドウが作れれば nil。**Linux** は `org.freedesktop.Notifications` の `GetCapabilities` に答えがあれば nil。D-Bus が起動するサービスも、これで起動する。
- `platform.Trays` に `RequestNotifications(done func(error))` を足した。`done` はどのゴルーチンから呼んでもよく、呼び出しの中で呼んでもよい。`gowaku` が UI ゴルーチンに運ぶ。
- **gowakutest:** `Tester.SetNotificationsAllowed(bool)`。既定は許可。拒否すると `Notify` は `ErrNoNotifications` を返して何も記録せず、`RequestNotifications` の `done` は次の `Pump` でこのエラーを受け取る。デスクトップと同じく、答えは呼び出しの後に届く。

### 2. 前面にあるときの通知

- delegate に `willPresentNotification` を足し、バナー・通知センターの一覧・音を返す。Windows と Linux では、アプリが前面にあっても通知は出る。macOS だけ出し方を変える理由が無いので、選ぶ API は作らなかった。

### 3. 常駐アプリ

```go
type App struct{ ...; OnQuitRequest func() bool }
func (h *WindowHandle) Hide()
func (h *WindowHandle) Show()
func (h *WindowHandle) Hidden() bool
```

要望には 3 つの案（閉じる要求に理由を渡す、`Hide` / `Show`、閉じても終わらないオプション）があった。前の 2 つを採った。

- **`App.OnQuitRequest`。** OS がアプリ全体に終了を求めたとき（macOS の ⌘Q、Dock、ログアウト）に呼ぶ。true なら `Quit` と同じく、ウィンドウに聞かずに全部閉じる。false なら何もしない。nil なら今までどおり、全ウィンドウに閉じる要求を出す（閉じるボタンと同じく `OnCloseRequest` に聞く）。
  - プラットフォームには新しいイベント `EventQuit`（Window は 0）を足した。cocoa の `applicationShouldTerminate:` は、全ウィンドウに `EventClose` を積む代わりにこれを積み、`NSTerminateCancel` を返す。
  - **閉じる要求に理由を渡す案は採らなかった。** `OnCloseRequest func() bool` の形が変わり、全アプリに影響する。終了はウィンドウではなくアプリの出来事で、ウィンドウが N 個あれば同じ問いが N 回来る。ADR 0076 で「全ウィンドウに聞く `RequestQuit`」を採らなかった理由（プロンプトが同時に開く）にも当たる。アプリに 1 回聞く方がよい。
  - ADR 0076 の「`Quit` は無条件」は変わらない。`OnQuitRequest` は OS からの要求だけに答え、プログラムの `Quit` は聞かない。
  - **Windows と Linux では呼ばない。** アプリ全体への終了の要求に当たるものが無い。WM_QUERYENDSESSION と logind は ADR 0076 のとおり扱わない。
- **`WindowHandle.Hide` / `Show` / `Hidden`。** ウィンドウを閉じずに画面とタスクバーから外す。ツリーも State もトレイも残り、`Run` は続く。最後のウィンドウを隠しても `Run` は終わらない（ウィンドウは開いている）。常駐アプリは `OnCloseRequest` で `Hide` して false を返し、トレイの `OnActivate` で `Show` する。
  - 隠れたウィンドウは描かない。最小化と同じく、ティッカーは `tickWhileHidden` の間隔で進む。`Show` で再描画する。
  - モーダルウィンドウも一緒に隠し、一緒に戻す。親だけ隠すと、入力を止める相手の居ないモーダルが残る。
  - プラットフォームには `Window.SetVisible(bool)` を足した。
    - Windows: `ShowWindow(SW_HIDE)` / `SW_SHOW` と `SetForegroundWindow`。
    - X11: `XWithdrawWindow`（ICCCM 4.1.4 の Withdrawn 状態。タスクバーからも消える）/ `XMapWindow` と `_NET_ACTIVE_WINDOW`。
    - macOS: `orderOut:` / `makeKeyAndOrderFront:`。アプリ全体を隠す `NSApp hide:` ではなくウィンドウ単位にした。ほかの OS と意味がそろい、ほかのウィンドウを巻き込まない。
    - Wayland: xdg-shell にはトップレベルを残したまま隠す手段が無い。null のバッファーを付けると unmap できるが、Vulkan の WSI がサーフェスのバッファーを持っているので使えない。そこで `Hide` は最小化にし、`Show` は何もしない（アプリからは前面に出せない。`Activate` と同じ）。利用者がタスクバーから戻す。
  - **macOS の Dock のクリック。** 全ウィンドウを隠した常駐アプリの Dock のアイコンを押しても、AppKit は `orderOut:` したウィンドウを戻さない。`applicationShouldHandleReopen:hasVisibleWindows:` で見えるウィンドウが無ければ新しいイベント `EventReopen` を積み、gowaku は `Hide` で隠したウィンドウ（モーダルでないもの）を `Show` する。Windows と Linux では、隠したウィンドウはタスクバーに出ないので当たるものが無い。
- **「最後のウィンドウを閉じても終わらない」オプションは採らなかった。** `Hide` があれば要らない。閉じたウィンドウを開き直すと State が失われるので、常駐アプリにはむしろ不便である。`Run` の終わりは「ウィンドウが無くなったとき」のままにした。

### 4. 最初の Build からホストに届く

- `core.NewTreeWithHost(root, host)` を足した。ホストを付けてからツリーを組み立てる。`NewTree` はホスト無しの同じもの。Run のウィンドウと `gowakutest.New` はこれを使う。最初の Build の `InitState` から `ShowTray`、`Notify`、`RequestNotifications`、クリップボードが使える。ガイドの「`InitState` から `Post` する」は「`InitState` から呼ぶ」に直した。

## 結果

- app のテスト（偽のプラットフォーム）で確かめた。
  - `OnQuitRequest` が nil なら全ウィンドウに聞くこと。true なら聞かずに閉じること、false なら残ること。閉じるボタンは今までどおり `OnCloseRequest` に聞くこと。
  - `Hide` したウィンドウが残って描かれないこと、`Show` で戻って描くこと、State が残ること、`EventReopen` で戻ること、モーダルも一緒に隠れて戻ること。
  - `RequestNotifications` の `done` が、呼び出しが戻った後に呼ばれること。拒否のときは `ErrNoNotifications` を受け取り、`Notify` もそれを返すこと。
  - `ShowTray` と `RequestNotifications` が `Home` の `InitState` から使えること。
- gowakutest のテストで、`SetNotificationsAllowed` の答えが次の `Pump` で届くこと、拒否の間は `Notify` が記録しないこと、ホストの無いツリーでは `RequestNotifications` が `ErrNoNotifications` を受け取ることを確かめた。
- macOS、Windows、Linux（X11 と Wayland）でビルドと `go vet` を通した。
- **残したもの:**
  - 実機での確認。macOS の許可ダイアログ、前面にあるときのバナー、⌘Q と Dock の「終了」とログアウト、Dock のクリックでの再表示。Windows と X11 の `Hide` / `Show`。Wayland の最小化。
  - アプリが通知のクリックで起動されたとき。通知センターの delegate は最初の `Notify` か `RequestNotifications` で設定するので、それより前に届いたクリックは聞こえない。Apple は起動の完了前に delegate を設定するよう勧めている。
  - 隠したウィンドウを `ShowFileDialog` の前面のウィンドウに選ばないこと。
