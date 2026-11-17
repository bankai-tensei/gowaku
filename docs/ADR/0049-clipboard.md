# 0049: クリップボード拡張（14a）

決定日: 2026-11-09

状態: 採用

## 背景

当初の計画の「クリップボード拡張」。v1 はテキストだけ（TextField のコピーと貼り付け）。ヒアリングで「画像・ファイル・HTML を読み書きし、X11 の大きい画像は INCR も実装する」と決めた。

## 決定

### 1. API

```go
type ClipboardContent struct{ Text, HTML string; Image *image.NRGBA; Files []string }
func ClipboardHas(ctx BuildContext) ClipboardKinds          // 読まずに種類だけ
func ReadClipboard(ctx BuildContext, kinds ClipboardKinds) ClipboardContent
func WriteClipboard(ctx BuildContext, c ClipboardContent)    // 持っている種類を全部
```

- 書くときは、持っている種類を全部並べ、相手のプログラムが分かる形を選べるようにする。読むときは、頼んだ種類だけ取りに行く（Linux では種類ごとに相手とやり取りが要る）。
- `platform.Clipboard`（`ClipboardKinds` / `ReadClipboard` / `WriteClipboard`）を `platform.Window` に足した。既存の `ClipboardText` / `SetClipboardText` は残す。
- 形の変換は `platform/internal/clip` にまとめた。I/O をしないので、Windows の形式も含めてどの OS でもテストできる。

### 2. 形式

| 種類 | Windows | X11 / Wayland（MIME） |
|---|---|---|
| テキスト | `CF_UNICODETEXT` | `text/plain;charset=utf-8`、`UTF8_STRING`、`text/plain`、`TEXT`、`STRING` |
| HTML | `HTML Format`（UTF-8、バイト位置のヘッダ + `<!--StartFragment-->` の印） | `text/html`。Chrome / Firefox が送る文書全体や UTF-16（BOM 付き）から断片を取り出す |
| 画像 | 読み：`PNG` を先に、無ければ `CF_DIB`（24 / 32 ビット、無圧縮か BI_BITFIELDS、上下どちら向きも）。書き：`PNG` と `CF_DIB`（32 ビット、下から） | `image/png` |
| ファイル | `CF_HDROP`（DROPFILES + UTF-16 の二重 NUL 終端） | `text/uri-list` と `x-special/gnome-copied-files`（Nautilus はこちらからしか貼らない） |

- アルファが全部 0 の 32 ビット DIB は不透明として読む（多くのプログラムがアルファを使わずに 32 ビットで出す）。
- 外から来る画像は 2^26 画素までに制限した。DIB と DROPFILES の読み取りは fuzz にかけた。

### 3. X11 の INCR

- 読み：`SelectionNotify` の型が INCR なら、プロパティを消して最初の断片を求め、ヘルパーウィンドウへの `PropertyNotify`（NewValue）ごとに読んで消し、長さ 0 で終わる。待ち時間の上限 300ms は、断片が届くたびに延ばす。
- 書き：256KB を超える値は、型 INCR と大きさで答え、相手のウィンドウに `PropertyChangeMask` を付け、相手がプロパティを消すたびに 64KB ずつ書き、最後に長さ 0 を書く。同時に複数の相手に送れる。
- TARGETS の名前は、gowaku の知っている MIME の atom と比べて引く（`XGetAtomName` を往復しない）。

### 4. Wayland

- `wl_data_offer` が告げた MIME から種類を知り、`receive` とパイプで読む。読む側の待ち時間も断片ごとに延ばす。
- 書きは `wl_data_source` に全部の MIME を `offer` し、`send` で頼まれた形を別のゴルーチンで書く（遅い相手で UI ゴルーチンを止めない）。

## 結果

- `platform/internal/clip` のテストで、全種類の往復、ブラウザの HTML の 4 通り、Chrome の HTML Format、DIB とアルファ、DROPFILES、uri-list のコメントと遠隔の URI を確かめた。fuzz は計 35 秒で失敗 0。
- Windows：実際のクリップボードで、gowaku が書いた 4 種類を .NET（Windows PowerShell 5.1、別プログラム）が読み、.NET が書いた画像とファイルを gowaku が読めた（テキストは戻す。他の種類は消える）。
- X11（WSLg の XWayland）：別の接続の間で 4 種類が渡り、50 万画素の雑音画像（PNG で 256KB 超）が INCR で両方向に通った（読みに 15〜37ms）。
- Wayland（sway と Weston）：別の接続の間で 4 種類が渡った。
- **残したもの:** `image/bmp` や `image/jpeg` しか出さないプログラムからの画像は読まない。ファイルの「切り取り」（GNOME の `cut`、Windows の `Preferred DropEffect`）は区別しない。
