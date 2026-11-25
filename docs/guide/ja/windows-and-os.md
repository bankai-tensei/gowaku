# ウィンドウと OS

[English](../en/windows-and-os.md)

このガイドでは、ウィジェットのツリーの外側にあるものを扱う。アプリとウィンドウ、タイトルバー、メニュー、トレイと通知、クリップボード、ファイルダイアログ、ドラッグ & ドロップ、URL を開く方法、そしてゴルーチンに関する規則である。

## アプリと UI ゴルーチン

`gowaku.Run` は最初のウィンドウを開き、最後のウィンドウが閉じるまで戻らない。`Run` を呼び出したゴルーチンが UI ゴルーチンになり、Build、レイアウト、描画、ウィジェットのコールバック、`SetState` はすべてこのゴルーチンだけで実行される。そのため、アプリの状態にロックは要らない。

```go
func main() {
	err := gowaku.Run(gowaku.App{
		Title:  "Notes",
		Width:  900,
		Height: 600,
		Home:   notes{},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

`Run` が error を返すのは、起動できない場合（GPU が無い、ウィンドウシステムが無い、gowaku が対応していない OS では `gowaku.ErrUnsupported`）に限られる。フレームの途中で起きた契約違反は panic になる。macOS では UI ゴルーチンがメインゴルーチンでなければならないため、`Run` は別のゴルーチンからではなく `main` から呼び出す。

`App` ではほかに、テーマ（`Theme`、`ThemeMode`）、標準ウィジェットのロケールと文言の表（`Locale`、`Strings`）、Linux のアプリ ID（`AppID`）、最初のウィンドウの設定を指定できる。`OnClose`、`OnCloseRequest`、`CustomFrame`、`Icon`、`MinSize`、`MaxSize`、`AlwaysOnTop`、`Fullscreen`、`Transparent` は、後述する `WindowOptions` の同名のフィールドと同じように働く。

### 別のゴルーチンで行う処理

ファイルやネットワークからの読み込みのような時間のかかる処理は別のゴルーチンで行い、結果は `gowaku.Post` で UI ゴルーチンに戻す。`Post` はどのゴルーチンからでも呼び出せ、渡した関数を UI ゴルーチンで実行する。関数が実行される時点では State がツリーから外れている可能性があるので、先に `Mounted` を確認する。

```go
func (s *pageState) load(path string) {
	go func() {
		data, err := os.ReadFile(path)
		gowaku.Post(func() {
			if !s.Mounted() {
				return
			}
			s.SetState(func() { s.data, s.err = data, err })
		})
	}()
}
```

`Run` の外で呼ばれた `Post` は関数を破棄するので、作業用のゴルーチンがアプリより長く生き残っても問題は無い。`gowaku.Quit` もどのゴルーチンからでも呼び出せ、すべてのウィンドウを閉じて `Run` から戻る。このときウィンドウへの確認は行わない（`OnCloseRequest` は呼ばれない）が、`OnClose` は呼ばれる。

Build の最中に `SetState` を呼んだ場合や、ツリーから外れた State で呼んだ場合は panic になる。

## 複数のウィンドウ

`gowaku.OpenWindow` は、独自のツリーを持つウィンドウをもう 1 つ開く。すべてのウィンドウは 1 つの GPU デバイスを共有し、1 本の UI ゴルーチンで動くので、状態をそのまま共有できる。例は `examples/multiwindow` にある。

```go
h, err := gowaku.OpenWindow(gowaku.WindowOptions{Title: "Inspector", Width: 360, Height: 480}, inspector{})
if err != nil {
	return err
}
// later, on the UI goroutine:
h.Close()
```

`WindowOptions` には `Title`、`Width`、`Height`、`Background` のほか、次のフィールドがある。

| フィールド | 働き |
|---|---|
| `OnClose` | ウィンドウが閉じるときに呼ばれる。利用者が閉じた場合も、`Close` や `Quit` による場合も、最後のウィンドウが閉じたことによる場合も呼ばれる。ツリーはまだ残っているので、保存処理はここで行う |
| `OnCloseRequest` | 利用者がウィンドウを閉じようとしたときに呼ばれる。false を返すとウィンドウは開いたままになる |
| `OnDrawError` | GPU で描画できなかったもの（受け付けられなかったカスタムシェーダーなど）を通知する |
| `CustomFrame` | OS のタイトルバーを外す（後述） |
| `Icon` | ウィンドウのアイコン。256 ピクセル四方の画像 1 枚がよい。同じ図柄を複数のサイズで渡してもよい |
| `MinSize`、`MaxSize` | クライアント領域のサイズの範囲。論理ピクセル単位 |
| `AlwaysOnTop`、`Fullscreen` | 名前のとおり。Wayland では `AlwaysOnTop` は無視される |
| `Transparent` | 描画内容のアルファ値に応じてデスクトップを透かす。X11 ではコンポジットマネージャーが必要 |

`gowaku.WindowOf(ctx)` は、そのウィジェットが属するウィンドウの `*WindowHandle` を返す。ハンドルからは `SetTitle`、`SetIcon`、`SetFullscreen`、`SetAlwaysOnTop`、`SetSizeLimits`、`Activate`、`Hide`、`Show`、`Close`、`RequestClose` を呼び出せ、`Fullscreen()`、`Transparent()`、`Hidden()` で状態を取得できる。ハンドルのメソッドはすべて UI ゴルーチンで呼び出す。

### 閉じる前に確認する

`OnCloseRequest` は、利用者がウィンドウを閉じようとしたときに呼ばれる。閉じるボタン、Alt+F4、タスクバー、ウィンドウマネージャー、スクリーンリーダーのいずれから閉じようとした場合も同じである。このコールバックの中でブロックしてはならない。保存されていない変更について確認するには、false を返してダイアログを表示し、答えが決まってから `Close` を呼ぶ。ウィンドウの中身を把握しているウィジェットが、自分でコールバックを設定することもできる。

```go
func (s *docState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	win := gowaku.WindowOf(ctx)
	win.SetOnCloseRequest(func() bool {
		if !s.dirty {
			return true
		}
		overlay.ShowDialog(ctx, func(dismiss func()) gowaku.Widget {
			return overlay.Dialog{
				Title:   "保存されていない変更があります",
				Message: "閉じる前に保存しますか。",
				Actions: []gowaku.Widget{
					control.Button{Label: "保存しない", OnPress: func() { dismiss(); win.Close() }},
					control.Button{Label: "保存", Style: control.ButtonPrimary, OnPress: func() { s.save(); dismiss(); win.Close() }},
				},
			}
		})
		return false
	})
	return s.page()
}
```

`Close`、`Quit`、モーダルウィンドウの親を閉じた場合のように、プログラムから閉じるときは確認しない。`RequestClose` は閉じるボタンと同じく確認を行う。

macOS では、OS がアプリ全体に終了を求めることもある。⌘Q、Dock の「終了」、ログアウトである。この要求には `App.OnQuitRequest` が答える。true を返すと `Quit` と同じく終了し、false を返すと動き続ける。指定しなければ、これまでどおりすべてのウィンドウに閉じる要求が送られ、すべて閉じたところでアプリが終わる。Windows と Linux にはこの要求が無く、呼ばれることは無い。

### トレイに常駐する

期限を知らせるアプリのようにトレイに常駐するアプリは、利用者がウィンドウを閉じたらウィンドウを隠し、終了を求められたときだけ終了する。

```go
var win *gowaku.WindowHandle // Home の InitState で WindowOf(ctx) を取っておく
gowaku.Run(gowaku.App{
	Home:           home{},
	OnCloseRequest: func() bool { win.Hide(); return false }, // 閉じるボタン、⌘W
	OnQuitRequest:  func() bool { return true },              // ⌘Q、Dock、ログアウト
})
// トレイ: OnActivate: func() { win.Show() } と、gowaku.Quit を呼ぶ項目。
```

`WindowHandle.Hide` は、ウィンドウを閉じずに画面とタスクバーから外す。その上のモーダルウィンドウも一緒に隠れる。ツリーと状態は残り、描画はせず、`Run` は続く。`Show` で元に戻して前面に出し、`Hidden` で状態を取得できる。macOS では Dock のアイコンをクリックすると、隠したウィンドウが再び表示される。Wayland にはウィンドウを隠す手段が無いので、`Hide` は最小化になり、`Show` では戻せない。利用者がタスクバーから戻す。

### モーダルウィンドウ

`gowaku.ShowModal` は `ctx` のウィンドウを親とするウィンドウを開き、それが閉じるまで親ウィンドウへの入力を止める。OS のダイアログと同じ振る舞いである。呼び出しはすぐに戻り、結果は `OnClose` で受け取る。Build の中ではなく、コールバックから呼び出す。

```go
var h *gowaku.WindowHandle
h, err := gowaku.ShowModal(ctx, gowaku.WindowOptions{Title: "設定", Width: 420, Height: 300, OnClose: s.reload},
	settings{onDone: func() { h.Close() }})
```

ウィンドウの中に表示するダイアログには、`overlay.ShowDialog` と `overlay.Dialog` を使う。

### タブを引き剥がしてウィンドウにする

`nav.TabBar` は、タブがグループ内のどのタブバーの外にドロップされたときにも、`OnTearOff` に `nav.TabTearOff` を渡して通知する。アプリはそのタブをタブバーから取り除き、`gowaku.TearOffWindow` でそのタブを表示するウィンドウを開く。新しいウィンドウは、ポインタの下にタブが来る位置に配置される。タブバーにタブが 1 つしか無かった場合は、代わりに `WindowHandle.MoveTo` でウィンドウ自体を移動する。アプリがウィンドウの位置を決められない Wayland では、コンポジタが決めた位置に開く。

```go
OnTearOff: func(e nav.TabTearOff) {
	if e.Alone {
		gowaku.WindowOf(ctx).MoveTo(e)
		return
	}
	s.remove(e.Index)
	gowaku.TearOffWindow(e, gowaku.WindowOptions{Title: "文書"}, docWindow{tab: e.Key})
},
```

## タイトルバー

既定ではタイトルバーは OS が描画する。サーバー側の装飾に対応していない Wayland のコンポジタ（GNOME、Weston）では、gowaku が小さなタイトルバーを自分で描画する。

`CustomFrame: true` を指定すると OS のタイトルバーが外れ、アプリが自分でタイトルバーを描く。ルートの最上部に `gowaku.TitleBar` を置き、メニューバーや検索欄など、好きなものを入れる。TitleBar は最小化・最大化・閉じるボタンを描画し、ポインタ操作を受け取る部品が無い場所のドラッグでウィンドウを移動し、ダブルクリックで最大化し、右クリックでウィンドウメニューを開く。Windows 11 では最大化ボタンの上にスナップレイアウトが表示される。ウィンドウの縁でのリサイズもそのまま使える。

```go
gowaku.App{
	Title:       "Editor",
	CustomFrame: true,
	Home: basic.Column{Children: []gowaku.Widget{
		gowaku.TitleBar{Child: menuBar},
		basic.Expanded{Child: page},
	}},
}
```

`Child` が nil の場合はウィンドウのタイトルを表示する。高さは 36 論理ピクセルで、全画面表示では場所を取らない。`examples/decorations` では、カスタムフレーム、アイコン、サイズの範囲、常に手前に表示、全画面、透過ウィンドウの例を見られる。

## メニューとショートカット

`interact.Action` はコマンドを表し、ラベル、ショートカット、現在実行できるかどうか、チェックの有無を持つ。一度作れば、同じ値をメニューバー、コンテキストメニュー、ツールバーに渡せる。`OnInvoke` の無いアクションは無効として扱われ、メニュー項目はグレー表示になり、ショートカットも何もしない。

```go
save := interact.Action{
	Label:    "保存(&S)",
	Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl},
	OnInvoke: s.save,
}
bar := overlay.MenuBar{Items: []overlay.MenuItem{
	{Action: interact.Action{Label: "ファイル(&F)"}, Items: []overlay.MenuItem{
		{Action: save},
		{Separator: true},
		{Action: interact.Action{Label: "終了(&X)", OnInvoke: gowaku.Quit}},
	}},
}}
```

- `&` はアクセスキー（ニーモニック）の目印である。日本語のラベルでは `ファイル(&F)` のように括弧の中に入れる。
- `overlay.MenuBar` は普通のウィジェットなので、Column の最上部や `TitleBar` の中に置く。Alt 単独か F10 でフォーカスがメニューバーに移り、矢印キー、Enter、Escape は Windows と同じように動作する。各項目のショートカットは、ウィンドウ内のどこにフォーカスがあっても効く。
- macOS ではメニューバーが OS のメニューバー（`NSMenu`）になり、アプリケーションメニューとウィンドウメニューが追加される。その場合ニーモニックは表示されず、ショートカットの `Ctrl` は ⌘ キーになる。
- `interact.Actions` は、どのメニューにも含まれないアクションのショートカットを、フォーカスのある部品がそのキーを使わない場合に実行する。文字を打つキー（単独か Shift と一緒）と、Enter と Shift+Enter は、フォーカスのあるテキスト欄が使うので、Space や英字、Enter だけのショートカットは入力中には動かない。欄が使わない Ctrl や Alt のショートカットは、Ctrl+Enter も含めて入力中でも動く（macOS では Command が Ctrl として扱われる）。
- `overlay.ContextMenu` は、子の上での右クリック、指やペンの長押し、メニューキーまたは Shift+F10 でメニューを開く。`overlay.ShowPopupMenu` を使うと、コードから指定した位置にメニューを開ける。

```go
overlay.ContextMenu{
	Items: []overlay.MenuItem{{Action: copyAction}, {Action: pasteAction}},
	Child: row,
}
```

メニューはどの OS でも gowaku が同じ見た目で描画する。例外は macOS のメニューバーとトレイアイコンのメニューで、これらは OS が描画する。

## トレイアイコンと通知

`gowaku.ShowTray` はデスクトップのトレイにアイコンを表示する。メニューは OS が描画し、メニューバーと同じ `overlay.MenuItem` を受け取る。`Run` の実行中に UI ゴルーチンで呼び出す必要があり、たとえば `InitState` から呼ぶ。

```go
tray, err := gowaku.ShowTray(ctx, gowaku.TrayOptions{
	Icon:       icon, // image.Image, drawn at 16 to 32 pixels
	Tooltip:    "Notes",
	OnActivate: func() { gowaku.WindowOf(ctx).Activate() },
	Menu:       []overlay.MenuItem{{Action: interact.Action{Label: "終了", OnInvoke: gowaku.Quit}}},
})
if errors.Is(err, gowaku.ErrNoTray) {
	// GNOME without the AppIndicator extension has no tray.
}
```

`tray.Update` で表示内容を差し替え、`tray.Close` でアイコンを消す。

`gowaku.Notify(ctx, title, body)` は通知を表示する。Windows ではトースト、Linux では通知サービス、macOS では通知センターを使う。アプリが前面にあるときも表示される。通知がクリックされると、最初に作ったトレイの `OnNotificationClick` が呼ばれる。通知を表示できないデスクトップや、利用者が通知を許可していない場合は `gowaku.ErrNoNotifications` を返す。macOS では、.app バンドルから起動していないプログラムはすべてこのエラーになる。[ビルドと配布](packaging.md)を参照。

macOS は、最初の通知の前に利用者に許可を求める。`gowaku.RequestNotifications` を使うと、起動時などに前もって許可を求め、その答えを受け取れる。

```go
gowaku.RequestNotifications(ctx, func(err error) {
	// 呼び出しが戻った後に、UI ゴルーチンで呼ばれる。
	s.SetState(func() { s.canNotify = err == nil })
})
```

答えが出る前に送った通知は答えを待ち、利用者が拒否すると破棄される。拒否された後は `Notify` が `ErrNoNotifications` を返す。後でシステム設定で変えた場合は、次の `Notify` か `RequestNotifications` から反映される。Windows は何も聞かず nil を返し、Linux は通知サービスが動いているかを返す。`gowakutest.Tester` の下では、`SetNotificationsAllowed` で答えを決める。

| | トレイ | 通知 |
|---|---|---|
| Windows | `Shell_NotifyIcon` | トレイアイコンのバルーン。トーストとして表示される |
| Linux | D-Bus の StatusNotifierItem | `org.freedesktop.Notifications` |
| macOS | `NSStatusItem` | `UNUserNotificationCenter`。.app バンドル内でのみ |

## クリップボード

入力欄はコピーと貼り付けを自分で処理する。それ以外のデータについては、クリップボードはテキスト、HTML の断片、画像、ファイルの一覧を、それぞれ有るか無いかの形で保持する。読み書きは UI ゴルーチンで、Build の外で行う。

```go
// What can be pasted?
if gowaku.ClipboardHas(ctx)&gowaku.ClipboardImage != 0 {
	c := gowaku.ReadClipboard(ctx, gowaku.ClipboardImage)
	s.SetState(func() { s.picture = c.Image })
}

gowaku.WriteClipboard(ctx, gowaku.ClipboardContent{Text: "gowaku", HTML: "<b>gowaku</b>"})
```

クリップボードのデータは他のアプリが提供するものなので、読み取り時にはその応答を待つ。Linux では最大で 1 秒弱かかる。これらの関数はツリーのウィンドウを通るので、`gowakutest.Tester` の中ではその Tester 専用のクリップボードに届く。

## ファイルダイアログ

`gowaku.ShowFileDialog` は `ctx` のウィンドウの手前に OS のファイルダイアログを表示し、後で UI ゴルーチンで `done` を呼び出す。`done` には選択されたパスが渡され、キャンセルされた場合は nil が渡される。ゼロ値の `FileDialog` は既存のファイルを 1 つ開くダイアログになり、`Save`、`Multiple`、`Directory`、`Name`、`Folder`、`Filters` で動作を変えられる。

```go
gowaku.ShowFileDialog(ctx, gowaku.FileDialog{
	Title:   "画像を開く",
	Filters: []gowaku.FileFilter{{Name: "Images", Patterns: []string{"*.png", "*.jpg"}}},
}, func(paths []string, err error) {
	if err != nil || paths == nil {
		return // gowaku.ErrNoFileDialog where the system has none
	}
	s.open(paths[0])
})
```

ウィンドウを持っていてもその `ctx` を持たないコードは、`WindowHandle.ShowFileDialog` を使う。そのウィンドウの手前に同じダイアログを表示する。Windows のダイアログはモーダルである。Linux では xdg-desktop-portal のダイアログを使い、表示中もアプリは動き続ける。ポータルが無い場合は結果が `gowaku.ErrNoFileDialog` になるので、パスは別の方法で尋ねる必要がある。gowaku は自前のファイルダイアログのウィジェットを提供しない。macOS では `NSOpenPanel` と `NSSavePanel` を使う。

`gowakutest.Tester` の下では、`gowaku.ShowFileDialog` は偽のダイアログを表示する。テストはそれに `tt.FileDialog().Choose(paths...)` で答える。

## ドラッグ & ドロップ

`interact.DragTarget` は、子の上にドロップされたテキストやファイルを受け取る。ドラッグ元は同じウィンドウでも、別の gowaku のウィンドウでも、エクスプローラー、Finder、Nautilus、ブラウザのような他のアプリでも構わない。`interact.Draggable` は子からデータをドラッグで取り出し、これらのどこへでも渡せる。

```go
interact.DragTarget{
	Kinds:   input.DataFiles,
	OnHover: func(over bool) { s.SetState(func() { s.lit = over }) },
	OnDrop: func(d input.DragData, pos geom.Offset) {
		s.SetState(func() { s.files = append(s.files, d.Files...) })
	},
	Child: dropArea,
}
```

```go
interact.Draggable{
	Data:      input.DragData{Files: []string{path}},
	Allowed:   input.DropCopy,
	OnDragEnd: func(effect input.DropEffect) {},
	Child:     row,
}
```

データはテキスト、ファイルの絶対パスの一覧、またはその両方である。移動操作の後始末は、アプリが `OnDragEnd` で行う。内部では、Windows は OLE、X11 は XDND、Wayland は `wl_data_device`、macOS は `NSDraggingDestination` を使う。`examples/dragdrop` では、双方向のドラッグとトレイアイコンの例を見られる。

## URL を開く

`gowaku.OpenURL` は、URL のスキームに対して OS が割り当てたアプリでその URL を開く。Web ページならブラウザで、`mailto:` ならメールソフトで開く。どのゴルーチンからでも、`Run` の実行中かどうかにかかわらず呼び出せ、OS が要求を受け付けた時点で戻る。絶対 URL でなければ `gowaku.ErrInvalidURL` になる。

OpenURL はスキームを制限しない。スキームによってはプログラムが起動することもある。文書中のリンクのように、自分で書いたものではないリンクを開くときは、意図したスキームだけを許可する。

```go
if p, err := url.Parse(link); err == nil && (p.Scheme == "https" || p.Scheme == "http" || p.Scheme == "mailto") {
	err = gowaku.OpenURL(link)
}
```

Windows では ShellExecute、macOS では NSWorkspace、Linux では OpenURI ポータルを使い、ポータルが使えなければ `xdg-open` に切り替える。URL がシェルを経由することはない。

## 実行中のテーマ切り替え

`gowaku.SetTheme(ctx, set, mode)` はすべてのウィンドウのテーマを差し替え、`gowaku.CurrentTheme(ctx)` は現在のテーマを返す。`ThemeSystem` を指定すると、実行中も OS のライトモードとダークモードの切り替えに追従する。

## ゴルーチンの規則のまとめ

| どのゴルーチンからでも呼び出せる | UI ゴルーチンからのみ |
|---|---|
| `Post`、`Quit`、`OpenURL`、`OpenInspector` | `SetState`、`OpenWindow`、`ShowModal`、`TearOffWindow`、`WindowOf` と `WindowHandle` のすべてのメソッド、`ShowTray` と `Tray` のメソッド、`Notify`、`RequestNotifications`、`ShowFileDialog`、クリップボードの関数、`SetTheme`、`CurrentTheme` |

設計の背景: [ADR 0046](../../ADR/0046-menus-and-shortcuts.md)（メニュー）、[ADR 0047](../../ADR/0047-modal-windows.md)（モーダルウィンドウ）、[ADR 0048](../../ADR/0048-custom-title-bar.md)（タイトルバー）、[ADR 0049](../../ADR/0049-clipboard.md)（クリップボード）、[ADR 0050](../../ADR/0050-drag-and-drop.md)（ドラッグ & ドロップ）、[ADR 0051](../../ADR/0051-tray-and-notifications.md)（トレイ）、[ADR 0076](../../ADR/0076-close-request.md)（閉じる要求）、[ADR 0161](../../ADR/0161-quit-request-hidden-windows-notification-permission.md)（終了の要求、隠したウィンドウ、通知の許可）。
