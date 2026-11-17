# 0021: platform/wayland の構成

決定日: 2026-11-08

状態: 採用

## 背景

Wayland 対応の前半（10a）。窓、Vulkan、キーボードとマウス、HiDPI、タイトルバー、クリップボードを Wayland で直接扱う。IME（`text-input-v3`）と D-Bus は 10b。

## 決定

### バインディングは生成し、イベントは 1 つのディスパッチャで受ける

- `internal/cmd/wlgen` がプロトコルの XML から Go を生成する。interface ごとに `wl_interface` の表、request ごとのメソッド、event ごとのメソッドを持つ listener の interface、それを全部無視する `…Nop` 型、enum の定数。生成物は `platform/wayland/protocol_linux.go`。
- **当初の計画は listener のコールバックを `purego.NewCallback` で作ると書いたが、`wl_proxy_add_dispatcher` にした。** event の数だけコールバックを作ると purego の上限（約 2000）と GC の問題に当たる（当初の計画が挙げていたリスク）。ディスパッチャ 1 個なら、全 proxy の全 event がそこに来て、opcode と `wl_argument` の配列を生成コードが型付きのメソッド呼び出しに戻す。
- request は `wl_proxy_marshal_array_flags`（1.20 以降）で送る。無ければ `wl_proxy_marshal_array_constructor_versioned` と `wl_proxy_destroy`。可変長引数の関数は呼ばない。
- `wl_interface` の表は Go のメモリに置く。Go のヒープは動かないので、パッケージ変数が持っていれば C から指してよい。文字列は NUL 付きのリテラルを `unsafe.StringData` で渡す。
- XML は `platform/wayland/protocol/` にそのまま置く（ライセンスも XML の中）。`wayland.xml`、`xdg-shell`、`xdg-decoration`、`fractional-scale`、`viewporter`、`cursor-shape`、`text-input-v3`（10b 用）。`wl_shell` は生成しない。

### 窓

- `xdg_toplevel`。最初の configure が来るまで `CreateWindow` は戻らない（最大 4 往復）。サイズは configure の通り。0 なら `WindowOptions` の値。
- **最小化の通知は Wayland に無い。** `xdg_toplevel` の `suspended` 状態（v6）が来たら 0×0 の `EventResize` にする（Win32 と X11 に合わせた）。
- `app_id` は X11 の `WM_CLASS` と同じ `gowaku`。

### タイトルバー: SSD があればそれ、無ければアプリが描く

- ヒアリングで決めた通り、libdecor は使わない。`zxdg_decoration_manager_v1` があればサーバー側（SSD）を頼む。無いか、コンポジタが断ったら `platform.Window.ClientDecorated()` が true になり、ルートの Widget がタイトルバーを描く（`titlebar.go`）。GNOME と WSLg の Weston はこちら。
- **タイトルバーは platform ではなく gowaku（ルートパッケージ）が描く。** platform は描画を知らない（CODING_STYLE §3.1）。platform.Window に `ClientDecorated`、`Maximized`、`BeginMove`、`ToggleMaximize`、`Minimize`、`ShowWindowMenu` を足した。Win32 と X11 では何もしない。
- タイトルバーは高さ 36。地は `Canvas`、下に `Hair` の線、題は中央、ボタンは右に最小化・最大化・閉じる。閉じるは押すと `Danger`。フォーカスが無いと題は `Ink3`。
- **アイコンは塗りのパスにした。** 小さい塗りのパスはグリフアトラスに入るので、MSAA を起こさない（ADR 0017）。線のパスは MSAA になり、タイトルバーは常に見えるので毎フレーム MSAA になってしまう。
- **ボタンの無い所を押しただけでは移動しない。** 4px 動いたら `xdg_toplevel.move`。押した瞬間に move を頼むと、押してすぐ離した場合に sway が離した後で移動を始め、次のクリックを飲み込んだ（自動操作で見つけた）。GTK のヘッダーバーと同じ。ダブルクリック（400ms、4px 以内）で最大化、右クリックでコンポジタのメニュー。
- **端のリサイズは platform が受け持つ。** 窓の内側 5px（角は 12px）でポインタをリサイズのカーソルにし、押したら `xdg_toplevel.resize`。アプリにはその間のポインタを渡さない（入ったときに `EventPointerLeave`）。最大化中とフルスクリーン中は無し。影も透明な余白も持たない。
- 試すために `GOWAKU_WAYLAND_DECORATIONS=client` で SSD があってもアプリが描く。

### HiDPI

- `wp_fractional_scale_v1` と `wp_viewporter` があれば、バッファは `round(論理 × スケール)`、viewport の destination を論理サイズにする。
- 無ければ `wl_surface.preferred_buffer_scale`（v6）、それも無ければ入っている出力の `wl_output.scale` の最大で `set_buffer_scale`。
- スケールの変更は `EventResize`。次のフレームの commit で効く。

### 入力

- キーボードは libxkbcommon。キーマップはコンポジタが fd で渡す。キーコードは修飾なし・現在のレイアウトのキーシムを `platform/internal/keysym` で変換（X11 と共有）。文字は `xkb_state_key_get_utf8`。デッドキーは xkb の compose（無ければ無し）。
- **キーリピートはクライアントの仕事。** `repeat_info` の間隔で `WaitEvent` の待ち時間を縮めて出す。忙しくて遅れた分はまとめて出さず飛ばす。
- ホイールは `axis_value120`（v8）、無ければ `axis_discrete`、それも無ければ連続値を 15 で割って段にする。`wl_pointer.frame` でまとめて 1 つの `EventScroll`。
- カーソルは `wp_cursor_shape_v1`、無ければ libwayland-cursor のテーマ（`XCURSOR_THEME`、`XCURSOR_SIZE`）。無ければ設定しない。

### クリップボード

- `wl_data_device`。読み取りは X11 と同じく最大 300ms 待つ同期処理。書き出しは goroutine で書くので、遅い相手で UI が止まらない。
- Wayland では入力イベントの serial が要る。キーかボタンを一度も押していない窓は selection を取れない。

### 選択と退避

- `internal/linuxwin.Open`: `GOWAKU_BACKEND` が `x11` か `wayland` ならそれ。無ければ `WAYLAND_DISPLAY` があれば Wayland、失敗して `DISPLAY` があれば X11 に退避（`slog.Debug`）。
- Vulkan のインスタンスは、ローダーにある `VK_KHR_xlib_surface` と `VK_KHR_wayland_surface` の両方を有効にする。`NativeHandle` の型で作るサーフェスを選ぶ。

## 結果

- WSL の WSLg（Weston、CSD）と、deb から展開した headless sway（SSD と CSD）で todo が動く。入力は sway に仮想キーボード（`wtype`）と仮想ポインタ（`TestVirtualPointer`）を差して自動で操作した。
- 待機中の CPU は Wayland でも 0。
- **ポインタの移動 1 回で 6 回アロケートする。** 移動は motion と frame の 2 つの event で、purego のコールバックは reflect を通るので 1 回ごとに約 3 回。X11 は 0。規約のホットパス（reconcile / Layout / Paint / Batch）ではないが、毎イベントの経路を割り当て無しにしてきた方針からは外れる。libwayland は event をコールバックでしか渡さないので、purego 側が速い経路を持つまで残す。
- 6 個の golden は sway の Wayland でも X11 と同じ差（最大 2）。

## 検討してやめたこと

- libdecor: GNOME での見た目は揃うが、プラグイン経由で GTK を読み込む。どの環境でも同じ見た目で依存が少ない方を選んだ（ヒアリング）。
- event ごとの `purego.NewCallback`: 上の通り。
- Wayland のワイヤープロトコルを Go で自前実装: コールバックもアロケーションも無くなるが、Vulkan の WSI が libwayland の `wl_display*` を要る。
