# 0008: 入力の配送と IME の扱い

決定日: 2026-11-07

状態: 採用

## 背景

Phase 4 で入力を platform から widgets まで通す。`core` と `platform` は互いを import できない一方で、キーコードや変換中の文字列は両方が知る必要がある。

## 決定

### 値型は最下層の `input` パッケージに置く

`KeyCode`, `Modifiers`, `Button`, `Cursor`, `Composition` を、何も import しない `input` に置いた。規約 §3.1 の「共通部分を下の層に降ろす」に従う。`core` 側に定義すると `platform` が参照できず、両側に定義すると変換表が要る。

`core.Key` は Widget の Key で埋まっているので、キーは `input.KeyCode`。

### OS の機能は `core.Host` で受ける

IME、カーソル、クリップボードは `core` が定義する `Host` interface で呼び、`app` が `platform.Window` に橋渡しする。論理ピクセルと物理ピクセルの変換もここ。Host の無い木では何もしない実装が返るので、テストも `testutil` もそのまま動く。当初の計画の `Clipboard()` は値を返す interface をやめ、`Window` のメソッド 2 つにした。

### ポインタは RenderObject の interface で受ける

`PointerTarget` / `HoverTarget` / `CursorSource` / `FocusTarget` は小さな任意の interface で、実装した RenderObject にだけ届く。Flutter の gesture arena は作らない。深い順に配り、`Handled` を立てた対象がその押下を最後まで受ける。ボタンが入れ子でも内側が勝つ。Phase 5 のスクロール内ボタンで足りなければ見直す。

`Clicks`（連続クリック数）は `core` が数える。`Time` は OS のメッセージ時刻で、`time.Now()` を `core` で呼ばない規約（§9）を守れる。

### 配送中に木から外れた対象

押下中の対象が再ビルドで消えた場合、`ro.base().tree == nil` を見て配らない。フォーカス中の対象が消えた場合は `FocusChanged` を呼ばない。これは Build の途中で走り、ハンドラが `SetState` を呼ぶと panic するため。IME だけ木が切る。

### IME は IMM32 で受ける

`WM_IME_COMPOSITION` と `ImmGetCompositionStringW` で読む（当初の計画のとおり）。TSF 対応の IME も互換層を通るので、Mozc で動いた。

- 変換中の文字列は `EventComposition`、確定は `EventText` で、`WM_CHAR` と同じ経路に合流する。
- `GCS_COMPCLAUSE` と `GCS_COMPATTR` は UTF-16 単位なので、`buildComposition` が UTF-8 バイト位置に直す。OS 呼び出しの無いファイルに分け、サロゲートペアも含めてテストした。
- 既定の変換ウィンドウは `ISC_SHOWUICOMPOSITIONWINDOW` を落として出さず、TextField が自分で描く。
- ウィンドウ作成時は IME をオフにし、TextField がフォーカスを得たときだけオンにする。ボタン上で打鍵しても変換が始まらない。

### 候補ウィンドウの位置は Paint で伝える

ウィンドウ内の位置が確定するのは Paint なので、前回と違うときだけ `SetIMECursorRect` を呼ぶ。位置は注目文節の先頭で、無ければキャレット。`ImmSetCandidateWindow` は `CFS_EXCLUDE` で行全体を避けさせ、下に出す。

### フォーカスを失うときは自分で確定する

`CPS_COMPLETE` で IME に確定させると、確定文字はキュー経由で届き、そのときにはフォーカスが移っていて文字が消える。TextField が変換中の文字列を自分で本文に入れ、IME には `CPS_CANCEL` で捨てさせる。

### TextField は Value と内部の文字列を両方持つ

`Value` が前回の Widget の値と違うときだけ内部の文字列を置き換える。OnChange で親の状態に書き戻す使い方も、ゼロ値で放っておく使い方も同じ型で動く。Flutter の `TextEditingController` は作らない。

### Undo は差分で持ち、単語単位にまとめる

当初の計画には無かったが、テキスト入力に要るので Phase 4 に加えた。履歴は全文のスナップショットではなく、位置・消した文字列・入れた文字列の差分で持つ。1 手の記憶量が編集の大きさで済み、上限は 500 手。

1 文字ずつ戻ると使いにくいので、続けて打った文字は単語の切れ目までまとめ、Backspace と Delete の連打もそれぞれ 1 手にする。時間でまとめる方式は `core` と `widgets` に時計が要るので採らず、切れ目はキャレットの移動・空白の次の文字・改行で決める。貼り付け、切り取り、選択の削除、Ctrl+Backspace、IME の確定はそれぞれ 1 手。変換中の文字列は本文ではないので履歴に入らない。

`Value` で外から本文が置き換わると履歴は捨てる。差分の位置が新しい本文に合わないため。OnChange で書き戻しただけなら `Value` と本文が一致するので捨てない。

## 含めなかったもの

- **キャレットの点滅**: 点滅には Ticker が要り、アイドル時に何も動かないという軸 1 の条件とも関わる。Phase 6 で扱う。
- **複数行のスクロール**: Phase 5 の `Scroll` を待つ。今は高さが伸びる。
- **再変換**（`IMR_RECONVERTSTRING`）: 当初の計画に無い。

## 結果

- Mozc で打鍵からの全手順を実機で確認した。
- MS-IME は未確認。この PC の言語設定で有効になっていないため。
- ホットパスのアロケーションは 0 のまま。`Tree` の hit test は `hits` を使い回す。
