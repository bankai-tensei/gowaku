# プラットフォーム

[English](../en/platforms.md)

gowaku は Windows、Linux、macOS（Apple Silicon）で動作し、どの OS でも cgo を使わない。このページでは、OS ごとに使う仕組み、どの環境で何を確認したか、既知の制限をまとめる。表中の「確認済み」は実際に動く環境で確かめたことを、「コードのテストまで」は OS の API を自動テストで呼び出してはいるものの、結果をまだ人の目で確かめていないことを表す。

## 一覧

| | Windows | Linux | macOS |
|---|---|---|---|
| アーキテクチャ | amd64、arm64 | amd64、arm64 | arm64 のみ |
| GPU | Direct3D 11 | Vulkan 1.0 | Metal |
| ウィンドウシステム | Win32 | Wayland または X11 | AppKit |
| IME のインライン変換表示と文節 | 対応（IMM32） | Wayland: 対応（注目文節のみ）。X11: 非対応 | 対応（コードのテストまで） |
| スクリーンリーダー | UI Automation | AT-SPI2 | NSAccessibility（コードのテストまで） |
| メニューバー | gowaku が描画 | gowaku が描画 | OS のメニューバー（`NSMenu`） |
| ファイルダイアログ | `IFileOpenDialog`、`IFileSaveDialog` | xdg-desktop-portal | `NSOpenPanel`、`NSSavePanel` |
| トレイ、通知 | `Shell_NotifyIcon`、トースト | StatusNotifierItem、`org.freedesktop.Notifications` | `NSStatusItem`、`UNUserNotificationCenter`（バンドルのみ） |
| 他のアプリとのドラッグ & ドロップ | OLE | XDND、`wl_data_device` | Finder（コードのテストまで） |
| ピンチ、タッチ、ペン | DirectManipulation、`WM_POINTER` | XInput 2、`wl_touch`、tablet-v2（ペンは Wayland のみ） | トラックパッドのピンチ |

その他の OS や Intel Mac でもビルドは通るが、`gowaku.Run` は `gowaku.ErrUnsupported` を返す。

## Windows

- **GPU**: Direct3D 11（機能レベル 11.0 または 11.1）。ハードウェアの GPU が必要で、ソフトウェアレンダラの WARP には切り替えないため、GPU が無ければ `Run` は失敗する。GPU を複数積んだマシンでは、UI はほとんどの時間アイドル状態にあるので、DXGI に省電力側の GPU（通常は内蔵 GPU）を要求する（Windows 10 1803 以降。それより前は OS の既定に従う）。Windows で使うのは Direct3D 11 だけで、gowaku の Vulkan バックエンドを Windows で動かすのは gowaku 自身の比較テストに限られる。
- **ウィンドウシステム**: Win32。タイトルバーは OS のものか、カスタムのもの（`CustomFrame`）を選べる。カスタムの場合でも、Windows 11 では最大化ボタンの上にスナップレイアウトが表示される。
- **IME**: IMM32。Windows 11 の Microsoft IME や Mozc のような TSF ベースの IME も、OS の互換レイヤーを経由して動作する。変換中の文字列は文節と注目文節を付けてその場に表示し、候補ウィンドウはキャレットの位置に出る。縦書きでも同様である。
- **アクセシビリティ**: UI Automation のプロバイダ。ナレーターでは手動で、NVDA では読み上げログで確認した。UI Automation のクライアントを使った自動テストもある。
- **確認済みの環境**: Windows 11、Intel Core i7-12700H（Iris Xe）と NVIDIA RTX 3050 Ti。README のベンチマークもこのマシンで計測した。

## Linux

ウィンドウシステムは起動時に選ぶ。`WAYLAND_DISPLAY` が設定されていれば Wayland を、そうでなければ X11 を使う。Wayland への接続に失敗し、かつ `DISPLAY` が設定されていれば X11 に切り替える（通常は XWayland が動いている）。`GOWAKU_BACKEND=wayland` または `GOWAKU_BACKEND=x11` で固定することもできる。描画はどちらの場合も Vulkan で行う。

ライブラリは起動時に purego で読み込む。読み込むのは `libvulkan.so.1` と、Wayland では `libwayland-client.so.0`、`libwayland-cursor.so.0`、`libxkbcommon.so.0`、X11 では `libX11.so.6`（あれば `libXi.so.6` も）である。ファイルダイアログ、デスクトップのテーマ、トレイ、通知、スクリーンリーダーとのブリッジは、純 Go の D-Bus クライアントを通じて扱う。セッションバスが無い場合はこれらの機能が無効になるだけで、アプリ自体はそのまま動く。

### Wayland

- **タイトルバー**: コンポジタが `xdg-decoration` に対応していれば、コンポジタが描画する。対応していない場合（GNOME と Weston）は、gowaku がアプリのテーマに合わせた小さなタイトルバー（最小化、最大化、閉じる、移動、リサイズ）を描画する。`GOWAKU_WAYLAND_DECORATIONS=client` を設定すると、試しに常に gowaku 側で描画させられる。
- **拡大率**: `fractional-scale-v1` と `viewporter` を使う。どちらも無い場合は、出力の整数倍率に従う。
- **IME**: `text-input-v3`。変換中の文字列をその場に表示し、候補ウィンドウをキャレットの位置に置く。このプロトコルには文節の情報が無いため、Mozc のように入力メソッドが範囲を選択している場合は、その範囲を注目文節として表示する。sway 上の fcitx5 + Mozc で確認した。GNOME の ibus では、プロトコルのやり取り自体は正しかったものの、試験環境でキー入力が ibus に届かなかった。原因は分かっていない。
- **プロトコル上の制限**: アプリがウィンドウの位置を決められないため、タブを引き剥がして作ったウィンドウはコンポジタが決めた位置に開く。`AlwaysOnTop` は無視される。ウィンドウのアイコンが表示されるのはコンポジタが `xdg-toplevel-icon-v1` に対応している場合だけで、それ以外では `App.AppID` が指すデスクトップファイルのアイコンが使われる。スクリーンリーダーに渡す座標は、画面上ではなくウィンドウを基準にした位置になる。

### X11

- **IME**: XIM。gowaku が受け取るのは確定した文字列だけで、変換中の文字列と候補は入力メソッドが自前のウィンドウに表示する。gowaku からそれをキャレットの位置に置くことはできず、X11 ではインラインの変換表示は行わない。
- **透過ウィンドウ**には、コンポジットマネージャーが必要である。
- **カスタムのタイトルバー**: `_NET_WM_MOVERESIZE` によるドラッグとリサイズは、実際のポインタでは確認していない。

### 確認済みの環境

gowaku を動作させた環境は次のとおり。

- X11: XWayland と Xvfb。
- Wayland: WSLg の Weston と、ヘッドレスの sway。
- Vulkan: いずれも Mesa のソフトウェアドライバ（lavapipe）。

実機の Ubuntu デスクトップ、実際の GPU、GNOME や KDE のセッションでは**まだ確認していない**。スクリーンリーダーは、Xvfb とヘッドレスの sway で Orca のデバッグログを読んで確認した。入力メソッドからの入力はヘッドレスの sway でしか確かめておらず、実機のデスクトップでの通しの確認はしていない。

デスクトップのサービスについて:

- トレイには StatusNotifierItem のホストが必要である。GNOME では AppIndicator 拡張を入れた場合にしか存在せず、ホストが無ければ `ShowTray` は `gowaku.ErrNoTray` を返す。
- ファイルダイアログには xdg-desktop-portal が必要で、無い場合の結果は `gowaku.ErrNoFileDialog` になる。GTK 3 のポータルバックエンドでは、ダイアログが親ウィンドウの手前に固定されないことがある。
- ライトモードとダークモードはポータルの `color-scheme` に従い、ポータルが無ければ GNOME の設定に従う。動きを減らす設定は GNOME の `enable-animations` に従い、KDE には対応していない。

## macOS

- **Apple Silicon のみ。** `darwin/arm64` でビルドでき、動作する。Intel Mac では `Run` が `gowaku.ErrUnsupported` を返す。
- **GPU**: Metal。シェーダーはソースのままバイナリに埋め込み、GPU デバイスの作成時にコンパイルする。ビルドに Xcode は要らない。
- **スレッド**: AppKit はメインスレッドでしか動かないので、`Run` はメインゴルーチンから呼び出す。
- **キー**: ⌘ キーは `input.ModControl` として届く。Ctrl+C や Ctrl+S として定義したショートカットは、⌘C や ⌘S として機能する。Control キーも `ModControl` になる。
- **メニューバー**: `overlay.MenuBar` は OS のメニューバーとして表示され、両端にアプリケーションメニューとウィンドウメニューが付く。⌘ 付きのショートカットはメニューが受け取り、それ以外のキーはこれまでどおりアプリに届く。
- **通知**を使うには、バンドル ID を持つ .app バンドルにアプリを収める必要がある。詳しくは[ビルドと配布](packaging.md)を参照。
- **コードのテストまでで、手動ではまだ確認していないもの**: VoiceOver、Finder とのドラッグ & ドロップ、IME（マークされたテキストからの文節付きインライン変換表示と、キャレットの位置に出る候補ウィンドウ）。
- **確認済みの環境**: Apple M5 搭載の Mac。macOS のベンチマークもこのマシンで計測した。

## 実行時の切り替え

| 変数 | 働き |
|---|---|
| `GOWAKU_BACKEND=wayland` / `x11` | Linux: ウィンドウシステムを固定する |
| `GOWAKU_WAYLAND_DECORATIONS=client` | Wayland: コンポジタが描画できる場合も gowaku がタイトルバーを描く |
| `GOWAKU_A11Y=1` | Linux: デスクトップがスクリーンリーダー無しと通知していても、アクセシビリティバスに接続する |
| `NO_AT_BRIDGE=1` | Linux: アクセシビリティバスに接続しない |

設計の背景: [ADR 0013](../../ADR/0013-vulkan-backend.md)（Vulkan）、[ADR 0014](../../ADR/0014-x11-platform.md)（X11）、[ADR 0021](../../ADR/0021-wayland-platform.md)（Wayland）、[ADR 0022](../../ADR/0022-linux-ime-dbus-file-dialog.md)（Linux の IME と D-Bus）、[ADR 0082](../../ADR/0082-macos-cocoa-metal.md)（macOS）。
