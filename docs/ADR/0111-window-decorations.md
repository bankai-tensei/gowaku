# 0111: ウィンドウの装飾（アイコン、最小・最大の大きさ、常に手前、フルスクリーン、透過）

決定日: 2026-11-14

状態: 採用

## 背景

当初の計画の「ウィンドウ装飾」の行は、カスタムタイトルバー（ADR 0048）のほかに「最小 / 最大サイズ、アイコン、常に手前、フルスクリーン、透過」を約束している。どれも作っていなかった。ウィンドウにアイコンを付ける手段は、どの OS にも無かった（Win32 の `iconOf` はトレイのアイコンにしか使っていなかった）。

番号について: 依頼では 0110 だったが、0110 は「作業領域に収める」がすでに使っているので 0111 にした。

## 公開 API

```go
// gowaku（ルート）
type App struct {
	...
	Icon             []image.Image
	MinSize, MaxSize geom.Size
	AlwaysOnTop      bool
	Fullscreen       bool
	Transparent      bool
	AppID            string // Linux の app_id / WM_CLASS
}
type WindowOptions struct {
	...
	Icon             []image.Image
	MinSize, MaxSize geom.Size
	AlwaysOnTop      bool
	Fullscreen       bool
	Transparent      bool
}
func (h *WindowHandle) SetIcon(images []image.Image)
func (h *WindowHandle) SetSizeLimits(minSize, maxSize geom.Size)
func (h *WindowHandle) SetAlwaysOnTop(on bool)
func (h *WindowHandle) SetFullscreen(on bool)
func (h *WindowHandle) Fullscreen() bool
func (h *WindowHandle) Transparent() bool

// platform
type WindowOptions struct {
	...
	AppID            string
	Icon             []*image.NRGBA
	MinSize, MaxSize geom.Size
	AlwaysOnTop, Fullscreen, Transparent bool
}
type Window interface {
	...
	SetIcon(images []*image.NRGBA)
	SetSizeLimits(minSize, maxSize geom.Size)
	SetAlwaysOnTop(on bool)
	SetFullscreen(on bool)
	Fullscreen() bool
	Transparent() bool
}
// Win32Handle、X11Handle、WaylandHandle に Transparent bool
```

- ゼロ値はどれも「何もしない」: アイコン nil は OS の既定、MinSize / MaxSize のゼロの軸は制限なし、それ以外は false。
- 最初のウィンドウは `App` のフィールド、ほかは `WindowOptions`、開いたあとは `WindowOf(ctx)` の `WindowHandle` で変える。透過だけは作るときに決まり、あとから変えられない（Win32 はスタイルを、X11 は visual を作成時に決めるため）。

## 決定

### 1. アイコン

**入力は `[]image.Image`。** 1 枚の大きな画像（256px 推奨）でも、同じ絵の複数サイズでもよい。OS が要るサイズごとに「そのサイズを覆う最小の画像」を選び、Catmull-Rom（`x/image/draw`）で縮める。正方形でなければ長辺を合わせて透明の上に中央寄せする。1 枚で済む簡単さと、16px だけ手で描き直したい人の両方に応える。選んで縮める処理は `platform/internal/icon` にまとめ、4 つの platform で共有した。

| OS | やること |
|---|---|
| Windows | `WM_SETICON` の `ICON_BIG` と `ICON_SMALL` を、ウィンドウの DPI での `SM_CXICON` / `SM_CXSMICON` の大きさで作る（`GetSystemMetricsForDpi`）。`WM_DPICHANGED` で作り直す。前の HICON は差し替えたあとに `DestroyIcon`、ウィンドウが消えるとき（`WM_DESTROY` と `Close`）にも捨てる。nil を渡すと 0 を送り、クラスの既定に戻る |
| X11 | `_NET_WM_ICON`（CARDINAL、幅・高さ・ARGB の並び）に 16〜256 のうち元画像以下のサイズをすべて入れる。nil は `XDeleteProperty` |
| Wayland | コンポジタが `xdg_toplevel_icon_manager_v1` を出していれば、`wl_shm` の ARGB8888（乗算済み）のバッファで `add_buffer`。サイズは `icon_size` で頼まれたものと上の段々。バッファはアイコンより後まで生かすという決まりがあるので、アイコンとバッファは次の `SetIcon` か `Close` まで持つ。無ければ何もしない。そのときは **`App.AppID` と同名の desktop ファイルのアイコン** をコンポジタが出す（GNOME、sway など。KWin 6.7、labwc 0.20 などはこのプロトコルを持つ） |
| macOS | ウィンドウごとのアイコンは無い。`NSApplication setApplicationIconImage:` で **Dock のアプリのアイコン** を変える（最後に設定したウィンドウのものが勝つ）。nil でバンドルの既定に戻る |

- プロトコルの XML（`protocol/xdg-toplevel-icon-v1.xml`）は wayland-protocols の staging から、リクエストとイベントの順番を保って書き写し（説明は短くした）、`go generate ./platform/wayland/` で束ねた。
- **app_id を設定できるようにした。** これまで Wayland は `"gowaku"` 固定、X11 の `WM_CLASS` も `"gowaku"` 固定で、desktop ファイルと結び付けられなかった。`App.AppID`（空なら従来どおり `"gowaku"`）を全ウィンドウに渡す。NUL を含むと panic。Windows の AppUserModelID は今回は入れていない（タスクバーのグループ分けに効くが、ショートカットの登録と組で考える必要がある）。

### 2. 最小・最大の大きさ

クライアント領域の論理ピクセル。ウィンドウの `Width` / `Height` と同じ単位で、DPI が変わっても論理サイズで保つ。

| OS | やること |
|---|---|
| Windows | `WM_GETMINMAXINFO` で、そのときの DPI で論理サイズを物理にし、`AdjustWindowRectExForDpi` で枠を足して `ptMinTrackSize` / `ptMaxTrackSize` に入れる（CustomFrame では上の枠を足さない。`setClientSize` と同じ計算）。作るときの大きさも制限の中に収める。`SetSizeLimits` で今の大きさが外れたら中に入れる（最大化中とフルスクリーン中はしない） |
| X11 | `WM_NORMAL_HINTS` の `PMinSize` / `PMaxSize`。片方の軸だけの最大は、もう片方を 32768 にする。`Placed` の位置のヒントと同じプロパティなので、1 か所（`writeNormalHints`）でまとめて書く |
| Wayland | `xdg_toplevel.set_min_size` / `set_max_size`。ゼロの軸が制限なしなのはプロトコルと同じ。window geometry は設定していないのでサーフェス全体（自前 CSD のタイトルバーも含む）。実行中に変えたときは `wl_surface.commit` で反映する |
| macOS | `contentMinSize` / `contentMaxSize`（ポイント = 論理ピクセル）。最大のゼロの軸は `CGFLOAT_MAX` |

### 3. 常に手前

| OS | やること |
|---|---|
| Windows | `SetWindowPos(HWND_TOPMOST / HWND_NOTOPMOST)` |
| X11 | 地図に載せる前は `_NET_WM_STATE` に `_NET_WM_STATE_ABOVE` を書く（モーダルの MODAL、フルスクリーンと一緒に `initialState` でまとめて書く）。載せたあとは `_NET_WM_STATE` のクライアントメッセージ（add / remove） |
| Wayland | **標準のプロトコルが無いので何もしない。** wlr-layer-shell などはパネル用で、普通のウィンドウには使えない |
| macOS | `NSFloatingWindowLevel` / `NSNormalWindowLevel` |

### 4. フルスクリーン

| OS | やること |
|---|---|
| Windows | 定番の方法: `GetWindowPlacement` とスタイルを覚え、`WS_OVERLAPPEDWINDOW` を外してモニター全体に `SetWindowPos`（`SWP_FRAMECHANGED`）。シェルはこれをフルスクリーンと見てタスクバーを隠す。戻すときはスタイルを戻して `SetWindowPlacement`。最大化していたウィンドウは最大化に戻る。排他的フルスクリーン（DXGI）は使わない（Alt+Enter も切ってある）|
| X11 | `_NET_WM_STATE_FULLSCREEN` を作成前はプロパティで、あとはクライアントメッセージで。状態は `_NET_WM_STATE` の PropertyNotify（ウィンドウに `PropertyChangeMask` を足した）と ConfigureNotify で読み直す |
| Wayland | `set_fullscreen`（出力はコンポジタに任せる）/ `unset_fullscreen`。状態は configure の `fullscreen` |
| macOS | `toggleFullScreen:`（専用のスペースに移るアニメーション付き）。`collectionBehavior` に FullScreenPrimary を付ける（所有されたウィンドウには付けない）。状態は `styleMask` の FullScreen ビット、`windowDidEnterFullScreen:` / `windowDidExitFullScreen:` で知らせる |

- 状態の変化は、プログラムからでも利用者からでも `EventResize` で届き、ルートが `Fullscreen()` を読み直す（最大化と同じ道）。
- **タイトルバーとの関係（決めたこと）:** フルスクリーン中は `TitleBar` が場所を取らない（`SizedBox{}` を返す）。Wayland で gowaku が描く CSD のタイトルバーも出さない。理由: フルスクリーンは「枠の無い全面」を求める操作で、OS の標準タイトルバーもすべての OS で消える。CustomFrame だけ残すと動画やスライドで邪魔になる。戻る手段（Esc や F11）はアプリが決める（gowaku は割り当てない）。
- **最大化との関係:** Win32 はフルスクリーン中の `ToggleMaximize` を無視し、`WM_NCHITTEST` はすべて `HTCLIENT`（端のリサイズも上端の帯も出さない）。`WM_GETMINMAXINFO` の制限もフルスクリーン中はかけない。戻ると、入る前が最大化なら最大化に戻る。

### 5. 透過ウィンドウ

作るときに `Transparent: true` を選ぶと、ウィンドウの画素のアルファでデスクトップが透ける。`Background` のゼロ値は、透過のウィンドウでは **透明**（テーマの地ではなく）になる。透過できなかった環境では不透明にし、ゼロの Background はテーマの地に戻す（`WindowHandle.Transparent()` で分かる）。クラッシュはしない。

gowaku のシェーダーは乗算済みの色を出し、合成も乗算済みのソースオーバー（`ONE, INV_SRC_ALPHA`）なので、バックバッファのアルファはそのまま正しい。足りないのはクリア色だけで、透過のウィンドウではルートが Background を乗算済みにしてから `BeginFrame` に渡す（不透明なウィンドウは今までどおり。アルファは捨てられる）。

| OS / バックエンド | やること | 状態 |
|---|---|---|
| Windows / D3D11 | HWND の flip モデルのスワップチェーンは DWM にアルファを渡せない（`DXGI_ALPHA_MODE_PREMULTIPLIED` は `CreateSwapChainForHwnd` で使えない）。そこで **DirectComposition** を使う: ウィンドウを `WS_EX_NOREDIRECTIONBITMAP` で作り、`CreateSwapChainForComposition`（`PREMULTIPLIED`、`STRETCH`、FLIP_DISCARD）のスワップチェーンを、`DCompositionCreateDevice` → `CreateTargetForHwnd` → ビジュアルの `SetContent` で窓に入れる。tearing は使わない（present のフラグはサーフェスごとに持つ） | 実装・実機で確認 |
| Windows / Vulkan | Windows の Vulkan ドライバの多くは `VK_COMPOSITE_ALPHA_OPAQUE` しか持たない。`gowaku.Run` は Windows で常に D3D11 を使うので、公開 API からは届かない。Vulkan のサーフェスは手に入るもののうち最良（乗算済み → 継承 → 最下位ビット）を選ぶが、`WS_EX_NOREDIRECTIONBITMAP` の窓に Vulkan が出せるかは **確かめていない** | 未確認（到達しない組み合わせ） |
| X11 / Vulkan | `XMatchVisualInfo` で 32 ビットの TrueColor visual を探し、専用の colormap と border pixel を付けて作る。Vulkan は `VK_COMPOSITE_ALPHA_PRE_MULTIPLIED` を選ぶ（Mesa はアルファ付きの visual では OPAQUE を出さない）。visual が無ければ普通のウィンドウ。**透けるのはコンポジットマネージャーがあるときだけ**（無いと黒） | 実装。WSLg の XWayland で visual が取れ、ウィンドウが開くことまで確認 |
| Wayland / Vulkan | コンポジタは常にアルファで合成する。Vulkan は `PRE_MULTIPLIED` を選ぶ。opaque region は設定していない | 実装・sway で確認 |
| macOS / Metal | `NSWindow` の `opaque = NO`、背景色 `clearColor`、`CAMetalLayer` の `opaque = NO`。Metal のバックエンドはそのまま | 実装・**未確認**（macOS 機が無い） |

- 透過はシェイプ付きのウィンドウやガラス風の見た目のためのもの。クリックは透けない（透明な所を押してもウィンドウに届く）。透明な所でクリックを下に通す機能（Win32 の `WS_EX_TRANSPARENT` / 入力領域、X11 の input shape、Wayland の input region）は今回の範囲外にした。
- 透過のウィンドウでも OS のタイトルバーと枠（Windows では DWM の枠と影）は残る。全面を透かしたいときは CustomFrame と組み合わせる。

## 結果

- `go test`（GPU 無し）: ルートで、App の装飾がそのまま platform に届くこと、ハンドルのセッターが届くこと、透過のウィンドウのクリア色（ゼロは透明、半透明の背景は乗算済み、透過できない環境ではテーマの地）、フルスクリーンで TitleBar が場所を取らなくなること。`platform/internal/icon` の選択と縮小、Vulkan のアルファの選び方、X11 の `_NET_WM_ICON` の並びと `WM_CLASS`、Wayland の乗算済み BGRA。
- `go test -tags gpu`（Windows 実機、150%）: `WM_GETICON` で大小のアイコンがあり、`SetIcon(nil)` で消え、HICON が差し替わる。100×100 に縮めると最小の 400×300 論理で止まり、5000 幅に広げると最大の 800 で止まる。`SetSizeLimits` で外れた大きさが中に入る。`WS_EX_TOPMOST` が付いて外れる。フルスクリーンでウィンドウの矩形がモニターと一致し、戻すと元の矩形に戻る。透過のウィンドウに `WS_EX_NOREDIRECTIONBITMAP` が付く。D3D11 で透過のサーフェスが DirectComposition になり、クリアした透明と半透明の赤がアルファごと残り、リサイズでき、3 回作って捨ててもデバイスの参照が増えない。
- `go test -tags gpu`（WSL）: X11（WSLg の XWayland）で `WM_CLASS` が app ID、`_NET_WM_ICON` が 16〜48 の 4 サイズ、`WM_NORMAL_HINTS` に最小と最大、アルファ付きの visual、`SetFullscreen` の後 `Fullscreen()` が true。Weston の xwm は地図に載せたあと `_NET_WM_STATE` から ABOVE を消す（MODAL と同じ。ADR 0047）。Wayland（sway、Weston）で、`SetFullscreen(true)` / `false` の configure が届いて状態が変わる。どちらも `xdg-toplevel-icon-v1` は持たないので、アイコンの経路は通っていない。
- `examples/decorations` を Windows で開き、タイトルバーのアイコン、透過（後ろの色付きのリストが透ける）、CustomFrame + 透過のスクリーンショットを撮った。sway でも透過で開いた。
- **DirectComposition の vtable の落とし穴:** `IDCompositionVisual::SetContent` は 15。SetOffsetX / SetOffsetY / SetTransform / SetClip がそれぞれ 2 つの多重定義を持ち、ほかに SetTransformParent、SetEffect、SetBitmapInterpolationMode、SetBorderMode があるので 3 + 12。最初 16 と数えて `E_INVALIDARG` を受け、実機で各スロットを試して確かめた。

## 残したもの

- macOS の全項目は実機で動かしていない（クロスコンパイルと vet、lint だけ）。
- Wayland の `xdg-toplevel-icon-v1` は、持っているコンポジタ（KWin 6.7 など）で試していない。
- X11 の透過の見た目（コンポジットマネージャーがある環境で透けること）、常に手前が効くことは、EWMH を全部やるウィンドウマネージャーで確かめていない。
- 透明な所でクリックを下に通す機能、Windows の AppUserModelID、Wayland の常に手前。
