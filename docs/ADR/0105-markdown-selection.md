# 0105: Markdown の文字を SelectionArea で選んでコピーする

決定日: 2026-11-13

状態: 採用

## 背景

Markdown を表示するアプリからの要望（その 3）。描いた Markdown をマウスで選んでコピーしたい。

`field.SelectionArea`（ADR 0089）は配下の `*basic.RenderParagraph` だけを集めていた。入れ子のモジュール `markdown/` の段落は自前の RenderObject `renderRich`（リンク・コードの地・取り消し線を描くため。ADR 0099）なので集められず、`Markdown` を `SelectionArea` で囲んでも何も選べなかった。

`markdown` は本体を import する別モジュールなので、`field` の側から `renderRich` は見えない。両者が見える層に「選べる文字」の約束を置く必要がある。

## 選択肢

1. **`basic` にインターフェースを置く**。`field`（段 5）も `markdown`（本体の外）も `basic` を import している。
2. **`widgets/internal/kit` に置く**。`internal` は別モジュールから import できないので、`markdown` が実装できない。
3. **`core` に置く**。`core` は `text` を知ってよいが、選択は描画の約束ではなくウィジェットの約束で、`core` の窓口を増やす理由が薄い。
4. **`markdown` が `basic.RichText` を使うように戻す**。リンクの当たり判定・コードの地・取り消し線を `RichText` に足すことになり、ADR 0099 の判断を覆す。

1 を採った。`basic` は `SelectionArea` が既に依っている `RenderParagraph` の置き場で、`SelectionArea` は `RenderParagraph` を具体的な型として扱っていたところを、同じ層のインターフェースで扱うだけになる。

## 決定

### 1. 約束 `basic.SelectableText`

```go
type SelectableText interface {
	core.RenderObject
	Text() string
	TextLayout() *text.Layout       // nil なら選択に入らない
	TextOrigin() geom.Offset         // TextLayout の原点がオブジェクトのどこにあるか
	SetSelection(start, end int, color geom.Color)
	ClearSelection()
	Selection() (start, end int)
	SelectionBreak() string          // コピーで前の文字との間に入れるもの
}
```

- `RenderParagraph` はもともと `Text` / `TextLayout` / `SetSelection` / `ClearSelection` / `Selection` を持っていた。足したのは `TextOrigin`（常に 0）と `SelectionBreak`（常に `"\n"`）の 2 つ。どちらもイベントの時点でしか呼ばれない。
- `TextLayout` が nil のものは集めない。レイアウト前の段落に加えて、「描くが選ばせない」文字（リストの番号など）を同じ口で外せる。型を分ける・フラグのメソッドを足すより口が小さい。
- `TextOrigin` は、`renderRich` が揃え（`Align`）のために文字を右へずらして描くことがあるため（表の右揃え・中央揃えのセル）。`SelectionArea` はヒットテスト・キャレットの位置・行の起点をこの分だけずらして扱う。
- `SelectionBreak` は、ブロックの間を空行、表のセルの間をタブにするため。`SelectionArea` は以前どおり段落の部分をつなぐが、間に入れる文字を後ろの段落に聞く。`RenderParagraph` は `"\n"` を返すので、`Text` / `RichText` のコピーは 1 文字も変わらない。

### 2. `SelectionArea` の変更

- 収集・ヒットテスト（I ビームの判定）・コピーで、`*basic.RenderParagraph` を `basic.SelectableText` に置き換えた。収集の順（描画順）、`core.FocusTarget` の下を飛ばすこと、内側の `SelectionArea` を飛ばすことは ADR 0089 のまま。
- I ビームは `TextLayout` が nil でない文字の上だけ（番号の上では矢印）。
- キーボード（Ctrl+A / Ctrl+C / Escape / Shift+矢印・Home・End・↑↓、メニューキー）も ADR 0089 のまま。段落の端で隣の段落へ移るので、表のセルの間や見出しとリストの間もキーボードで渡れる。

### 3. ホットパス

- `renderRich` に足したのはポインタのフィールド `selection *richSelection` ひとつと、`Paint` での nil 判定ひとつ。`SelectionArea` の外の Markdown ではアロケーションも毎フレームの処理も増えない。選択の状態は最初に選ばれたとき（イベントの時点）に確保し、矩形の作業領域を使い回すので、選択があっても `Paint` はアロケーションしない。
- 選択はコードの地の上、字の下に塗る（コードの中を選んでも見える）。
- 確認: `TestSelection_PaintDoesNotAllocate`（文書全体を `SelectionArea` で囲み、選択の有無の両方で `AllocsPerRun` = 0）、`TestSelection_OutsideAnAreaHasNoState`。本体側の既存の `TestSelectionArea_PaintDoesNotAllocate` / `TestParagraphSelection_PaintDoesNotAllocate` もそのまま通る。

### 4. コピーする文字列

ブラウザで GitHub の README を選んでテキストとして貼ったときに近く、かつ表を表計算に貼れる形にした。

| 部分 | コピーされるもの |
|---|---|
| ブロックの間（段落・見出し・コードブロック・引用の中の段落・表・ブロックの画像の代替テキスト） | 空行（`"\n\n"`） |
| 見出し | 文字だけ（`#` は入れない） |
| 段落の中 | ソフト改行は描いたとおり（全角どうしは何も入れず、それ以外は空白。ADR 0099）、ハード改行は `"\n"` |
| 強調・取り消し線・コードスパン・リンク | 文字だけ（`*` `~~` `` ` `` や URL は入れない） |
| 詰めたリストの項目の間、入れ子のリスト | 改行（`"\n"`）。リストの最初の項目の前は、リストの外側の区切り（ふつうは空行） |
| 緩いリストの項目の間 | 空行 |
| 箇条書きの点・番号・タスクのチェックボックス | **入れない**。ブラウザの `<ol>` / `<ul>` のコピーと同じで、点や番号は文字ではなく飾りとして扱う。番号は描くが `TextLayout` が nil で、選択の色も付かない |
| コードブロック | コードそのまま（タブは描くときと同じく 4 桁ごとの空白）。言語名の見出し（`go` など）は入れない |
| 引用 | 中の文字だけ（`>` は入れない） |
| 表 | セルの間はタブ、行の間は改行（TSV）。見出しの行も 1 行目として入る。欠けたセルは空の文字列（`"3\t"`） |
| 区切り線 | 何も入れない（文字が無い） |
| 画像（`Images` / `ImageProvider` で絵が出たもの） | 何も入れない。絵は `SelectableText` ではないので集めない |
| 画像（読めず代替テキストを出したもの、文中の画像） | 代替テキスト。画面に文字として見えているので、ほかの文字と同じに扱う |
| ルビ | Markdown にルビの書き方は無い（ADR 0099）。`Text` 側の規則（親文字だけ）は ADR 0089 のまま |

例: 

```
# 見出し
段落の *強調* と [リンク](https://example.com)。
- one
- two
| A | B |
|---|---|
| 1 | 2 |
```

（実際には各ブロックの間に空行がある）を全部選ぶと `"見出し\n\n段落の 強調 と リンク。\n\none\ntwo\n\nA\tB\n1\t2"` になる。

### 5. リンクとフォーカス

- リンクの RenderObject（`renderLink`）は `FocusTarget` なので、`SelectionArea` は今までどおりその下を集めない。リンクの文字は段落（`renderRich`）の文字の一部なので、段落ごと選ばれ、コピーに入る。
- リンクの上で押すとリンクが押下を受ける（選択は始まらない）。ブラウザでリンクの上からドラッグするとリンクのドラッグになるのと同じで、リンクの外から始めれば越えて選べる。
- 画像のリンク（`Pressable`）の下も集めない。

### 6. スクリーンリーダー

ADR 0089 §9 と同じく、静的な文字の選択を支援技術へ伝える口はまだ無い。`renderRich` の semantics（`RoleText` とラベル、リンクの `RoleLink`）は変えていない。

## 結果

- `Markdown` を `field.SelectionArea` で囲めば、見出し・段落・リスト・コード・表のセルをまたいでマウスとキーボードで選び、Ctrl+C でコピーできる。
- 本体の変更: `basic.SelectableText`（新しいインターフェース）、`RenderParagraph.TextOrigin` / `SelectionBreak`、`SelectionArea` がインターフェースで集めること。`Text` / `RichText` の挙動とコピーは変わらない。
- `markdown/` の変更: `renderRich` が `SelectableText` を実装（`selection.go`）。番号と言語名は `builder.label` で作り、選ばせない。リンクのターゲットは押せるリンクだけに作り、`index` で段落の中のリンクを指す（ADR 0106 で `#` のリンクだけが押せる場合があるため）。
- テスト: `markdown/selection_test.go`（全選択のコピーの全文、見出しからコードへのドラッグ、表のセルと Shift+←、選択の中のリンクを押せること、Paint のアロケーション 0）、`TestSelection_SkipsPictures`。
- スクリーンショットで、見出し・リスト・コードブロックにまたがる選択を確かめた（番号と点には色が付かない）。`markdown/example` の `TestExample` を、環境変数 `MARKDOWN_SHOTS` に書き出し先のディレクトリを入れて `go test ./example` で走らせると撮れる。

## 残したこと

- 箇条書きの点・番号を含めてコピーするモード（Markdown のソースに近い形でのコピー）。要望が出たら `SelectionBreak` と同じ口で前置きを返す形にできる。
- 静的テキストの選択のスクリーンリーダーへの公開（ADR 0089 と同じ）。
- `SelectionArea` が `List` / `Table` の中の文字を選ぶこと（行は `FocusTarget` なので集めない。ADR 0089 のまま）。
