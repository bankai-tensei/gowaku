# ビルドと配布

[English](../en/packaging.md)

gowaku のアプリは 1 つの Go バイナリになる。ビルドに C コンパイラは要らず、一緒に配るランタイムも無い。gowaku は OS と GPU を純 Go から呼び出し、OS のライブラリは起動時に読み込む。インストーラー、署名、アップデーターの機能は gowaku 自身には無い。このガイドでは、ビルドの方法、アイコン、そして各 OS がバイナリのほかに必要とするファイルを扱う。

## リリースビルド

```bash
go build -trimpath -ldflags "-s -w" -o notes .
```

`-s -w` はシンボルテーブルとデバッグ情報を取り除き、バイナリを小さくする（`examples/hello` で OS によって 6〜9MB 程度）。`-trimpath` は手元のパスをバイナリに残さないためのオプションである。

Windows では `-H windowsgui` を加えて、コンソールウィンドウが開かないようにする。

```bash
go build -trimpath -ldflags "-s -w -H windowsgui" -o notes.exe .
```

`-H windowsgui` を付けると、標準出力と標準エラー出力はどこにも表示されない。メッセージを残したい場合はファイルに書き出す。

## クロスコンパイル

`CGO_ENABLED=0` を指定すれば、どの OS からでも任意のターゲット向けにビルドできる。

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -H windowsgui" -o notes.exe .
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o notes .
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o notes .
```

PowerShell では、先に環境変数を設定しておく: `$env:GOOS = "linux"; $env:CGO_ENABLED = "0"`。

| ターゲット | 動作 |
|---|---|
| `windows/amd64`、`windows/arm64` | 動作する。Direct3D 11 |
| `linux/amd64`、`linux/arm64` | 動作する。Wayland または X11 上の Vulkan |
| `darwin/arm64` | 動作する。Metal |
| `darwin/amd64`（Intel Mac）とその他の OS | ビルドは通るが、`gowaku.Run` が `gowaku.ErrUnsupported` を返す |

各ターゲットで使える機能は[プラットフォーム](platforms.md)を参照。

## アイコン

ウィンドウのアイコンは実行時に設定する。画像を `go:embed` で埋め込み、`App.Icon`（最初のウィンドウ）、`WindowOptions.Icon`（それ以外のウィンドウ）、`WindowHandle.SetIcon` に渡す。

```go
//go:embed icon.png
var iconPNG []byte

func appIcon() []image.Image {
	img, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return nil // the default icon of the OS
	}
	return []image.Image{img}
}
```

256 ピクセル四方の画像を 1 枚渡すのがよく、OS が必要な大きさに縮小する。小さいサイズの見た目を自分で決めたい場合は、同じ図柄を複数のサイズで渡す。アイコンが表示される場所は次のとおり。

- **Windows**: タイトルバー、タスクバー、ウィンドウの切り替え画面。エクスプローラーに表示される `.exe` ファイルのアイコンはバイナリに埋め込むリソースであり、gowaku は作らない。Go のリンカー向けに `.syso` ファイルを出力するリソースコンパイラを使えば追加できる。
- **macOS**: ウィンドウごとのアイコンは無く、最後に渡したアイコンがアプリ全体のアイコンとして Dock に表示される。.app バンドルにすれば、Finder にはバンドルのアイコンが表示される（後述）。
- **Linux**: X11 では渡したアイコンが表示される。Wayland ではコンポジタが `xdg-toplevel-icon-v1` に対応している場合だけ表示され、それ以外は `App.AppID` が指すデスクトップファイルのアイコンが使われる（後述）。

トレイアイコンはこれとは別で、`gowaku.TrayOptions.Icon` を 16〜32 ピクセルで描画する。

## macOS: .app バンドル

macOS ではバイナリ単体でも動くが、バンドルが必要な機能もある。通知（`gowaku.Notify`）はバンドル ID を持つアプリでしか使えず、それ以外では `gowaku.ErrNoNotifications` を返す。また、バンドルにすると Finder と Dock にアプリの名前とアイコンが表示される。

構成:

```
Notes.app/
  Contents/
    Info.plist
    MacOS/
      notes          the binary from GOOS=darwin GOARCH=arm64
    Resources/
      notes.icns     optional
```

最小限の `Info.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>     <string>notes</string>
  <key>CFBundleIdentifier</key>     <string>com.example.notes</string>
  <key>CFBundleName</key>           <string>Notes</string>
  <key>CFBundlePackageType</key>    <string>APPL</string>
  <key>CFBundleShortVersionString</key> <string>1.0.0</string>
  <key>CFBundleIconFile</key>       <string>notes</string>
  <key>NSHighResolutionCapable</key> <true/>
</dict>
</plist>
```

通知に必要なのは `CFBundleIdentifier` で、最初に通知を出すときに利用者へ許可を求める。Go のリンカーは `darwin/arm64` の実行ファイルにアドホック署名を付けるため、Apple Silicon ではそのまま動く。他の人に配布する場合は、Apple のツールでバンドルに署名し、公証を受ける。

## Linux: デスクトップファイル

Linux のデスクトップ環境は、アプリ ID を使ってウィンドウとデスクトップファイルを結び付ける。アプリ ID は Wayland では `app_id`、X11 では `WM_CLASS` のクラスにあたる。`App.AppID` には、デスクトップファイルの名前から `.desktop` を除いたものを指定する。空の場合は `"gowaku"` になる。

```go
gowaku.Run(gowaku.App{Title: "Notes", AppID: "com.example.Notes", Home: notes{}})
```

`com.example.Notes.desktop` を `~/.local/share/applications`（または `/usr/share/applications`）に置く。

```ini
[Desktop Entry]
Type=Application
Name=Notes
Exec=/opt/notes/notes %F
Icon=com.example.Notes
Categories=Utility;
StartupWMClass=com.example.Notes
```

アイコンはアイコンテーマの下、たとえば `~/.local/share/icons/hicolor/256x256/apps/com.example.Notes.png` に置く。Wayland では、タイトルバーやタスクの切り替え画面のアイコンはここから読み込まれる。

バイナリが実行時に読み込むもの:

- 常に必要: Vulkan のローダー `libvulkan.so.1` と Vulkan のドライバ。
- Wayland: `libwayland-client.so.0`、`libwayland-cursor.so.0`、`libxkbcommon.so.0`。
- X11: `libX11.so.6`。ピンチ操作のために、あれば `libXi.so.6` も。
- D-Bus 経由で使い、無くても構わないもの: ファイルダイアログ（xdg-desktop-portal）、デスクトップのテーマ、トレイ、通知、スクリーンリーダーとのブリッジ。セッションバスが無い場合はこれらが無効になるだけで、アプリはそのまま動く。

日本語を表示するには、Noto Sans CJK JP のような和文フォントが OS に入っている必要がある。gowaku はフォントを同梱しない。

## Windows

`.exe` 以外に配るものは無い。Direct3D 11（機能レベル 11.0 以上）が動く GPU が必要である。ソフトウェアレンダラの WARP には切り替えないので、ハードウェアの GPU が無いマシンでは `Run` がエラーを返す。日本語の表示には、Windows に標準で入っている Yu Gothic を使う。
