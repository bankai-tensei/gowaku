# 0088: Input.Obscure（パスワード欄）と安全な入力

決定日: 2026-11-13

状態: 採用

## 背景

当初の計画の `Input.Obscure`。パスワード欄は利用者が毎回自作するが、自作すると次の 2 つを防げない。

1. **日本語 IME がパスワードに掛かる。** かな入力のままパスワード欄に移ると「hunter2」が「ふんてr2」になる。IME は辞書の学習や予測のために入力を覚えることもある。`Input` は IME を切る口を持たず、アプリの側からは切れない。
2. **中身が外に出る。** 伏せ字で描くだけでは、コピー、Undo の履歴、スクリーンリーダーの Value から本文が出る。

ウィジェットは増やさず、`Input` のフラグにする（当初の計画の決定どおり）。

## 決定

### 1. API

- `field.Input.Obscure bool` を足した。ゼロ値は今までどおりの入力欄。
- 実行中に切り替えてよい。「パスワードを表示」のボタンは `Obscure: !shown` と書く。

### 2. 描画: 書記素クラスタごとに 1 つの「•」

- 本文は今までどおり `Content` に持ち、レイアウトだけを「•」（U+2022）の列で作る。クラスタは `text.Shaper.GraphemeBounds`（新設、go-text の segmenter）で数える。結合文字、ZWJ の絵文字、国旗、異体字セレクタ付きの漢字は、どれも 1 つの「•」になる。
- 表示のオフセットと「•」のオフセットの対応は、ルビの射影（ADR 0074）と同じ `shownLayout` に 3 つ目の写像 `obscureView` として入れた。キャレット、選択、ヒットテスト、Backspace / Delete、Shift での選択の拡大は、すべて本文のクラスタ単位で動く。
- **単語の移動は端まで飛ぶ。** Ctrl+←→、Ctrl+Backspace / Delete、ダブルクリックは、パスワード全体を 1 語として扱う。単語の境界で空白の位置が分かるのを防ぐ。Windows の ES_PASSWORD の edit と同じ。
- 「•」の列は `obscureView` が長めに 1 本持ち、その部分文字列を `Reshape` に渡す。シェイピングは編集のときだけ（キャッシュミスでホットパスではない）。Paint はレイアウトのグリフを描くだけで、アロケーションしない（`TestInputObscure_PaintDoesNotAllocate`、`BenchmarkInputObscure_Paint` で 0 allocs/op）。
- **最後に打った文字は見せない。** スマートフォンの「一瞬見せる」はデスクトップの OS には無く、肩越しに見られる場面で不利。オプションも作らなかった。
- 文字は「•」に固定した。macOS の NSSecureTextField と同じで、静かな見た目の方針（Apple 寄り）に合う。Windows の「●」に切り替える口は作っていない。
- プレースホルダはそのまま文字で描く。秘密ではない。

### 3. IME を切る: `core.SecureInputHost` と `platform.SecureInputWindow`

まず既存の `SetIMEEnabled(false)` が各 OS で何をしているかを確かめた。

| OS | `SetIMEEnabled(false)` | 完全に切れているか |
|---|---|---|
| Windows | `ImmAssociateContextEx(hwnd, NULL, 0)` で入力コンテキストを外す | はい。IMM の IME も TSF の IME（Windows 11 の MS-IME）もキーを見ない |
| X11 | `XUnsetICFocus` で入力コンテキストのフォーカスを外し、Xlib が自分でキーを引く | はい（XIM で取れる手段の範囲で） |
| Wayland | `zwp_text_input_v3` を disable | はい。入力メソッドはキーを受け取らない |
| macOS | ビューの `inputContext` が nil を返す | はい。AppKit は IME を通さずにキーを渡す |

どれも IME を完全に外していたので、**パスワード欄はフォーカス中も `SetIMEEnabled(false)` を呼ぶだけで IME が掛からない**。そのうえで、足りないものだけを足した。

- `core.SecureInputHost`（任意のインターフェース、`VerticalIMEHost` と同じ形）: `SetSecureInput(secure bool)`。パスワード欄はフォーカスを得たときに true、失ったときに false を呼ぶ。
  - 順序: 得るときは `SetSecureInput(true)` → `SetIMEEnabled(false)`、失うときは `SetIMEEnabled(false)` → `SetSecureInput(false)`。安全な状態が先に始まり、後に終わる。
  - フォーカスを持ったままツリーから外れたフィールドには `FocusChanged` が来ない（Build 中のため）。そこで `core.Tree.detached` が `SetIMEEnabled(false)` と一緒に `SetSecureInput(false)` を呼ぶ。macOS で安全な入力が残り続けるのを防ぐ。
- `platform.SecureInputWindow`（任意のインターフェース。`platform.Window` は内部 interface なので足してもよいが、テストの偽物すべてに波及しないよう別にした）: `SetSecureInput(secure bool)`。4 つのバックエンドすべてが実装する。**secure の間は、`SetIMEEnabled(true)` が来ても IME を戻さない。** フィールドの呼び順に頼らず、窓の側でも IME が掛からないことを保証する。
  - **Windows**: secure の間は入力コンテキストを外したままにする。Windows に macOS の secure event input に当たるものは無い。
  - **macOS**: `inputContext` を nil にしたうえで、窓がキーウィンドウの間だけ Carbon（HIToolbox）の `EnableSecureEventInput` を呼び、外れたら `DisableSecureEventInput` を呼ぶ。NSSecureTextField と同じく、他のプロセスがキーを読めなくなり、システムは入力ソースをローマ字に切り替える。呼び出しはプロセス全体で数えられるので、窓ごとに `secureOn` で対を保つ。閉じた窓は返す。シンボルは起動時に解決し、欠けていれば `error`（規約 §8）。
  - **Wayland**: secure の間は text-input を disable のままにする。
    - 当初の計画は `text-input-v3` の content type（purpose password、hint sensitive_data / hidden_text）を挙げていた。しかし content type は入力メソッドへの**お願い**で、IME がかな変換を止める保証は無い（fcitx5 は直接入力に切り替えるが、ibus-mozc などは実装次第）。text-input を有効にすると入力メソッドがキーボードを握るので、その変換を捨てると打った文字が消える。だから「入力メソッドにキーを渡さない」方を選んだ。content type を送る場面は無い。
    - 代わりに失うのは、画面キーボード（squeekboard など）がパスワード欄で出ないこと。デスクトップを対象にする方針（当初の計画）なので受け入れた。
    - クライアント同士がキーを読めないのは Wayland の性質なので、secure event input に当たるものは要らない。
  - **X11**: secure の間は入力コンテキストのフォーカスを外したままにし、未確定文字列を捨てる。X11 で他のクライアントからキーを隠す手段はキーボードのグラブしかなく、ウィンドウマネージャーのショートカットまで奪うので採らなかった。
- `gowaku.windowHost` は `SetSecureInput` をネイティブの窓が `SecureInputWindow` なら渡す。そうでない窓でも、フィールドが IME を切るので IME は掛からない。

### 4. クリップボードとドラッグ

- Ctrl+C と Ctrl+X はキーを受け取り（上の Shortcut に流さない）、何もしない。切り取りで本文も消えない。編集メニューから呼ばれる `InputController.Copy` / `Cut` も同じく何もしない。Ctrl+V と `Paste` は使える。パスワードマネージャーからの貼り付けが要る。
- `Input` には今、選択したテキストをドラッグで外に出す機能も、文字を落とす機能も、コンテキストメニューも無い。将来どれかを足すときは Obscure で止めること（コピーと同じ扱い）。

### 5. Undo の履歴を持たない

- Obscure の間は編集を履歴に記録しない。Ctrl+Z / Ctrl+Y は何もしない。履歴の `Removed` には消したパスワードの断片が残り、メモリに長く留まるからである。macOS の NSSecureTextField も Undo を持たない。
- Obscure が切り替わったときは、どちら向きでも履歴を消す。表示中に打った文字を伏せた後に Undo で戻したり、伏せていた間の状態を表示中に戻したりしない。
- 切り替えのとき IME の未確定文字列があれば、確定せずに捨てる（`UpdateRenderObject` は Build 中なので OnChange を呼べない）。

### 6. スクリーンリーダー

- `semantics.Obscured`（State に追記）を足した。Obscure のフィールドの Value と `TextInfo.Text` は「•」の列で、本文は渡さない。選択の端は「•」の数（= UTF-16 のオフセット）で渡す。
  - 空文字列にしなかったのは、読み上げソフトが文字数と、キャレットがどこにあるかを伝えられるようにするため。Chromium のパスワード欄も同じ。
- 対応: UIA は `IsPassword` = true（コントロールの種類は Edit のまま）、AT-SPI は role `password text`（40）、NSAccessibility は AXTextField に subrole `AXSecureTextField`。
- スクリーンリーダーからの SetSelection は「•」の数をクラスタの先頭のバイト位置に戻す。SetValue は打ったのと同じく本文を置き換える。

### 7. 他のオプションとの組み合わせ

- **Multiline**: 無視して 1 行にする。Enter は OnSubmit（またはフォームの送信）、貼り付けた改行は空白になる。複数行のパスワードは無い。
- **Ruby**: 無視する。記法も文字なので、`｜漢字《かんじ》` は 8 つの「•」になる。ルビを描くと、隠すべき文字の形と位置が出る。
- **縦書き**（`Style.Orientation = text.Vertical`）: 残す。「•」が縦に並び、↑↓で動く。伏せ字の性質は変わらない。
- **Controller**: そのまま使える。ただし `Copy` / `Cut` は上のとおり何もしない（編集メニューの操作は利用者の操作だから）。`Text` / `SelectedText` はアプリのコードが読むものなので止めない（アプリは Value も持っている）。

## 結果

- 公開 API の追加: `field.Input.Obscure`、`semantics.Obscured`、`core.SecureInputHost`、`platform.SecureInputWindow`、`text.Shaper.GraphemeBounds`。既存の API と既定の動きは変えていない。
- Windows では gpu タグのテスト（`platform/win32/ime_secure_gpu_windows_test.go`）で実機の IME を確かめた。ひらがなモードの MS-IME で K、A を送ると、IME を有効にした窓では未確定文字列が 2 回来て、`SetSecureInput(true)` の窓では同じキーが「ka」としてそのまま届き、未確定文字列は来なかった。`SetIMEEnabled(true)` を後から呼んでも変わらない。
- Linux（X11 / Wayland）と macOS はクロスコンパイルと `go vet` まで。実機では動かしていない。
- カタログの頁は `examples/gallery/pages_obscure.go`。ギャラリーに登録した。

## 残したこと

- 最後に打った文字を一瞬見せるオプション。
- 伏せ字の文字を選ぶ口。
- Caps Lock が入っていることの警告。OS からロックキーの状態を取る口が `platform` に無い。
- Wayland の画面キーボードでのパスワード入力（上の理由で text-input を有効にしない）。
