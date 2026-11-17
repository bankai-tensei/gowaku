# 0016: v1 の公開 API の範囲

決定日: 2026-11-07

状態: 一部を ADR 0081 で置き換え

> 追記（2026-11-10）: ルートの再エクスポートと「ルートだけで書ける」は ADR 0081 で取り下げた。ルートはアプリとウィンドウの入り口と、どのウィジェットも書く `Widget` などの別名だけを持ち、アプリは `widgets/basic` などの層のパッケージを直接 import する。表の `widgets` も 0081 の 8 パッケージに置き換えた。公開と内部の範囲、ABI 型、`WindowHandle.Close` の判断は今も有効。

## 背景

v1 を公開する前に、どこまでを互換の約束に含めるかを決める。Phase 0 と Phase 7 に、`gpu/d3d11` と `gpu/vulkan` の ABI 型が公開識別子になっている件を「API を見直すときに決める」と TODO で残していた。規約 §8 はヘッダの名前を変えないことを求めるので、`D3D11_TEXTURE2D_DESC` のような型は大文字で始まり、Go では公開になる。

あわせて、サンプルが `layout`、`theme`、`paint` まで import しないと書けない状態だった。

## 選択肢

ABI 型について:

1. `gpu/d3d11` と `gpu/vulkan` を `internal/` へ動かす。godoc から消えるが、規約、lint、当初の計画のパスがすべて変わる。
2. ABI 型だけ `internal/` のサブパッケージへ動かす。COM のメソッドが同じパッケージにいられなくなり、呼び出しが読みにくくなる。
3. パスはそのままにして、これらのパッケージを互換の約束から外す。

## 決定

3 を採る。当初の計画がすでに `gpu.Backend` と `platform.Window` を内部 interface と書いており、それに揃える。

| 区分 | パッケージ | 約束 |
|------|-----------|------|
| 公開 | `gowaku`, `gowakutest`（ADR 0025 で追加）, `core`, `layout`, `paint`, `text`, `text/jlreq`, `theme`, `motion`, `widgets`, `geom`, `input` | v2 で壊さない。追加のみ |
| 内部 | `gpu`, `gpu/d3d11`, `gpu/vulkan`, `platform`, `platform/win32`, `platform/x11`, `render` | 予告なく変える |

`input` は当初の計画の列挙に無かったが、`KeyEvent` の型が公開 API に現れるので公開に足す。

アプリがルートの `gowaku` だけで書けるように、次を再エクスポートする。

- `Offset`, `Size`, `Rect`, `Path`, `Stroke`, `KeyEvent`, `PointerEvent`
- `MainAxisAlignment` などの配置の型と定数（`CrossCenter`, `MainMin` など）
- ボタンの大きさ（`ButtonSmall` など）、`IgnorePointer`, `Scale`, `MediaQueryData`
- 余白のトークン `Space2`〜`Space28`
- `ParseSVGPath`
- 追記（2026-11-08）: `StateOf`（ADR 0029）, `Pressable` / `PressState`（0028）, `RepaintSignal`（0030）, `AlignTopLeft`〜`AlignBottomRight`（0031）, `Wrap` / `Grid` / `Axis` / `Horizontal` / `Vertical`（0033）

`widgets.RenderParagraph` と `widgets.RenderEditable` は公開のままにする。`layout` が `text` を import できない（規約 §3.1）ためここに置いた RenderObject で、独自のウィジェットから組み合わせる余地がある。ルートには出さない。

## 結果

- `examples/hello`, `counter`, `todo`, `form`, `multiwindow` はルートだけで書ける。`gallery` と `jptext` は `text.Style` の組版の設定などを使うので下の層も import する。
- ABI 型は godoc に並び続ける。各パッケージの doc に約束の外だと書いた。
- `WindowHandle.Close` は、そのウィンドウのコールバックの中から呼ばれてもよいと保証する。閉じるのは次の安全な地点まで遅らせる（`app.reap`）。以前はディスパッチの途中でツリーを破棄していた。
