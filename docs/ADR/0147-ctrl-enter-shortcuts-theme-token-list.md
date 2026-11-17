# 0147: 入力欄の Ctrl+Enter をショートカットに渡す、テーマのトークンの一覧

決定日: 2026-11-17

状態: 採用

## 背景

gowaku を使う 2 人の開発者から、次の 2 つの報告があった。

1. **入力欄にフォーカスがあると Ctrl+Enter のショートカットが動かない：** 「確定」「確定して次へ」を `interact.Actions` の Ctrl+Enter に置いたが、入力欄に打っている間は一度も動かなかった。複数行の `field.Input` では、`RenderEditable.edit` が修飾キーを見ずに Enter で改行を入れていた。1 行の欄は、`OnSubmit` が無くても、どの修飾の Enter も使ったことにしていた。`OnSubmit` があれば Ctrl+Enter で送信もした。`editor.Editor` も、どの修飾の Enter でも改行を入れていた。`Actions` の godoc は「欄が使わない Ctrl と Alt のショートカットは動く」と書いていたが、実際には Enter が例外だった。
2. **テーマのトークンの名前が見つからない：** `go doc ./theme` は定数のまとまりを `const Space2 = 2 ...`、`const LiftNone Lift = iota ...` の 1 行に縮めるので、余白、高さ、角丸の名前がわからなかった。まとまりの前の説明文も、パッケージの一覧には出ない（`go doc ./theme Space8` のように名前を 1 つ指定したときだけ、まとまりの全体と説明文が出る）。

## 選択肢

### 入力欄の Enter

- **A. 欄は Enter と Shift+Enter だけを使い、Ctrl か Alt の付いた Enter（Shift の有無を問わない）を上に渡す。** Enter は複数行なら改行、`OnSubmit` か `SubmitOnEnter` が当てはまれば送信、Shift+Enter は複数行なら（`SubmitOnEnter` でも）改行、1 行なら今まで通り送信である。Ctrl+Enter、Ctrl+Shift+Enter、Alt+Enter は何もせず `false` を返し、`Actions` に届く。IME の変換中は、修飾にかかわらず Enter は変換の確定で、IME のものとする。チャットやメールのアプリで Ctrl+Enter（macOS では Command+Enter）が「送信」「確定」に使われるのと同じである。
- **B. 欄が実際に使わない Enter をすべて渡す（`OnSubmit` の無い 1 行の欄の Enter も）。** 欄に打って Enter を押すと、行を開く Enter の Action や、周りのリストの行の Enter が動く。入力欄の中で Enter が欄の外に漏れるのは驚きが大きく、欄が送信するかどうかで Enter の行き先が変わるのもわかりにくい。
- **C. `Input` に `CtrlEnterSubmits` のような設定を足す。** 欄ごとに設定が要り、`Actions` の godoc の約束（欄が使わない Ctrl と Alt のショートカットは動く）も直らない。

### テーマのトークン

- **A. ガイド（en、ja）の「テーマを読み出す」に短い表を置き、godoc ではまとまりの説明文とパッケージの説明文に名前を並べる。** `go doc ./theme` はパッケージの説明文を全部出すので、そこに一覧を置けば名前が見える。`Lift` は型の説明文にも並べる（`go doc ./theme Lift` は型の説明文を出し、定数は 1 行に縮める）。
- **B. 定数をまとまりに分けず、1 つずつ宣言する。** `go doc` は全部を出すが、余白の 10 個が一覧の中で散らばり、説明文を 1 つずつ書くことになる。

## 決定

- 案 A。`widgets/internal/kit.ShortcutEnter(e, composing)` を足し、`field.RenderEditable.HandleKey`（`Input`、`Search`、`NumberField`、`Autocomplete`、`TokenField` などすべての入力欄）と `editor.Editor` の `HandleKey` は、変換中でない Ctrl か Alt の Enter を使わずに返す。`RenderEditable` では `Extras.onKey` の後に見るので、欄を組む部品は今まで通り先に受け取れる。`TokenField` の、トークンを選んでいるときに Enter を使う分岐も、同じキーを渡すようにした。`editor.Editor` は、変換中の Enter で改行を入れていたのを、`RenderEditable` と同じく何もしないようにした（プラットフォームは IME の扱うキーを渡さないので、普段は届かない）。
- `interact.Actions` の godoc に、欄が持つキー（文字を打つキーとその Shift、Enter と Shift+Enter、Backspace、Delete、左右、Home、End と複数行の上下、エディターの Page Up、Page Down、Tab、Ctrl+A、C、V、X、Y、Z とエディターの Ctrl+F、Ctrl+H、F3、検索バーの Escape、変換中のすべてのキー）と、それ以外がショートカットに届くことを正確に書いた。`input.Modifiers` の godoc に、macOS の Command が `ModControl` として届くこと（`platform/cocoa/keys.go` の `modifiersOf`、ADR 0082）と、Option が `ModAlt` であることを書いた。`Input.OnSubmit` と `Multiline` の godoc も直した。
- 案 A。`docs/guide/en/theme.md` と `docs/guide/ja/theme.md` に「トークンの一覧」の表（`Space*`、`ControlHeight`、`Lift*`、`Radius`、`Text`、`Color` と、その型）を足した。`theme` のパッケージの説明文、余白のまとまりの説明文、`Lift` の型とまとまりの説明文に名前を並べた。

## 結果

- Ctrl+Enter、Ctrl+Shift+Enter、Alt+Enter のショートカットが、フォーカスのある入力欄とエディターから動く。今まで 1 行の欄で `OnSubmit` を Ctrl+Enter でも呼んでいたアプリは、Enter だけで送信するようになる。
- `gowakutest` のテストは、`OnSubmit` の無い 1 行の欄、ある 1 行の欄、複数行の欄、`SubmitOnEnter` の複数行の欄、エディターのそれぞれで、Ctrl+Enter、Ctrl+Shift+Enter、Alt+Enter の Action が 1 回ずつ動き、欄の文字が変わらず、送信されないことを確かめる。修飾なしの Enter と Shift+Enter は、今まで通り改行または送信をし、Enter と Shift+Enter の Action は動かないことを確かめる。変換中の Ctrl+Enter と Enter は、どの Action も動かさず改行も入れないことを確かめる。
- `go doc ./theme` で、余白、高さ、角丸、文字スタイルの名前が見える。
