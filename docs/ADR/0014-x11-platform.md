# 0014: platform/x11 の構成

決定日: 2026-11-07

状態: 採用

## 背景

当初の計画の表が方針を決めていたが、実装で変えた点と、表に無かった点がある。

## 決定

- **libxkbcommon は使わない。** 当初の計画はキーマップに dlopen すると書いたが、`XLookupKeysym`（修飾なしのキーシム）と `Xutf8LookupString`（文字）で足りる。依存が 1 つ減る。
- **XIM は `XIMPreeditNothing | XIMStatusNothing`。** 入力メソッドが自分のウィンドウに変換中の文字を出す。当初の計画の「確定のみ」のこと。`SetIMECursorRect` は何もしない。候補窓の位置はこのスタイルでは伝えられない。
- **`XCreateIC` は可変長引数だが purego で呼べる。** 引数が整数とポインタだけなら、x86-64 の呼び出し規約は固定長と同じ。
- **ウェイクアップは自己パイプ + `poll`。** 当初の計画は `select` と書いたが、fd の番号に上限が無い `poll` にした。
- **クリップボードは見えないヘルパーウィンドウが持つ。** ウィンドウを閉じてもコピーした内容が残る。読み取りは最大 300ms 待つ同期処理で、その間に届いた他のイベントはキューに入る。INCR（分割転送）は v1 では読まない。
- **`XInitThreads` を最初に呼ぶ。** Vulkan のドライバは同じ接続を別スレッドから触る。
- **X のエラーで死なない。** 既定のハンドラは何でもプロセスを終わらせる。クリップボードの相手が応答の前に消えるような無害なものもあるので、`slog.Debug` に落とす。
- **`DestroyNotify` は `EventClose` にする。** `xdotool windowclose` のように外から破棄されたとき、アプリが存在しないウィンドウを待ち続けていた。
- **最小化は 0×0 の `EventResize`。** Win32 に合わせた。`UnmapNotify` で 0×0、`MapNotify` で実サイズ。
- **スケールは `RESOURCE_MANAGER` の `Xft.dpi`。** ルートウィンドウの `PropertyNotify` を見ているので実行中の変更に追従する。
- **テーマは起動時に `gsettings` を読むだけ。** `color-scheme`、無ければ `gtk-theme` の名前。追従は D-Bus が要るので後回しにした（ADR 0022）。
- **イベントキューは `platform/internal/evqueue` に移した。** win32 と共有する。毎イベントの経路は ADR 0013 の `hot` と同じ方法で割り当てない。
- **ロジックはタグ無しのファイルに置く。** `logic.go`（キー変換、DPI と gsettings の解析）は Windows の `go test` でも走る。

## 結果

実 X サーバーが要るテストは `//go:build gpu` に置いた。CI には X サーバーも GPU も無いため。

未検証の点: 実 IME（XIM）での確定入力（ibus + Mozc で `XCreateIC` までは成功したが、キーが Mozc に届かなかった。原因がコードか検証環境かは切り分けていない）、Linux の実 GPU（lavapipe でだけ確認）、実機の GNOME との組み合わせ、`_NET_WM_PING` の応答。
