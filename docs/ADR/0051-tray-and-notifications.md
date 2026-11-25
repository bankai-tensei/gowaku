# 0051: トレイと通知（14b）

決定日: 2026-11-09

状態: 採用

## 背景

当初の計画の「トレイと通知」。ヒアリングで次を決めた。

- Windows の通知は `Shell_NotifyIcon` のバルーンにする（Windows 10 / 11 ではトーストとして出る）。
- Linux は StatusNotifierItem と `org.freedesktop.Notifications`。受け手がいない環境（素の GNOME）では黙って無効にする。XEmbed のトレイは作らない。

## 決定

```go
type TrayOptions struct{ Icon image.Image; Tooltip string; Menu []MenuItem; OnActivate, OnNotificationClick func() }
func ShowTray(opts TrayOptions) (*Tray, error)   // ErrNoTray
func (t *Tray) Update(opts TrayOptions); func (t *Tray) Close()
func Notify(title, body string) error            // ErrNoNotifications
```

- **トレイのメニューは OS が描く。** どのウィンドウにも属さないので、gowaku は描けない。メニューを自前で描く方針（ADR 0046）の、唯一の例外。
  - 項目には `MenuItem` をそのまま使う。ラベルの `&`、ショートカット、チェック、無効、サブメニューもそのまま写す。
  - Windows は `TrackPopupMenuEx`。メニューのモーダルループの間も描画を続ける。
  - Linux は com.canonical.dbusmenu で、パネルが描く。ニーモニックの `&` は `_` に直す。
- トレイの出来事（アイコンのクリック、メニューの選択、通知のクリック）は、OS がどのゴルーチンから呼んでも、`Post` で UI ゴルーチンに運んでから利用者のコールバックを呼ぶ。プラットフォームのイベントキューは変えない。
- Run が終わると、トレイのアイコンも消える。

### Windows

- 見えないウィンドウを 1 つ作り、それがアイコンを持つ。`NOTIFYICON_VERSION_4` を使う。
- アイコンは画像から `CreateDIBSection` と `CreateIconIndirect` で作る。画像が無ければ、OS のアプリのアイコンを使う。
- Explorer が再起動すると `TaskbarCreated` が届くので、アイコンを足し直す。
- 通知はアイコンのバルーンにする。トレイが無いときは、バルーンのあいだだけ一時的なアイコンを出す。

### Linux

- トレイごとにセッションバスへの接続を 1 つ持つ。名前は `org.kde.StatusNotifierItem-PID-N`、`/StatusNotifierItem` と `/MenuBar` を公開し、`org.kde.StatusNotifierWatcher` に登録する。登録できなければ `ErrNoTray`。
- アイコンは ARGB32 のネットワークバイト順で渡す。`Update` で `NewIcon` / `NewTitle` / `NewToolTip` と `LayoutUpdated` を出す。
- 通知は `Notify` に `default` の操作を付けて出す。`ActionInvoked` を見張り、自分の通知のクリックだけを最初のトレイに伝える。

## 結果

- app のテストで、メニューの写し方（区切り、無効、サブメニュー、チェック）、クリック、選択、通知、Run の終わりにアイコンが消えることを確かめた。
- Windows：実物のトレイにアイコンを足し、`NIN_SELECT` でのクリックと、`WM_CONTEXTMENU` で開いたネイティブのメニューの選択（↓ と Enter をメニューに送る）を確かめた。
- Linux：セッションバスに偽のパネル（StatusNotifierWatcher）と偽の通知サービスを立て、パネルのふりをして確かめた。
  - 項目のプロパティと dbusmenu の `GetLayout`、項目の `clicked` とアイコンの `Activate`
  - 通知と `ActionInvoked`、`Update`
  - 受け手がいないときの `ErrNoTray` / `ErrNoNotifications`
- **残したもの:**
  - 通知を Windows で実際に表示しての確認（通知センターに残るので自動ではしない）
  - 実在のパネル（KDE、waybar）での表示
  - トレイのアイコンのスクロールと中ボタン
  - 画像やボタンの付いた通知

## 後の変更

- 通知の許可を求める `RequestNotifications` を足し、macOS の `Notify` は許可の答えを待つようにした。トレイに常駐するアプリのために `WindowHandle.Hide` / `Show` を足した。ADR 0161。
