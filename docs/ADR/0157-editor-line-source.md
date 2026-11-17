# 0157: ファイルの読み書きは Editor から外し、行のソースのインターフェースで受け取る

決定日: 2026-11-18

状態: 採用

## 背景

ADR 0153、0155、0156 で、`editor.Controller` に巨大なファイルを扱う仕組みを入れた。ファイルの索引、行の読み出しとキャッシュ、OS ごとの開き方、外での変化の見張り、追記の追いかけ、一時ファイルを経由した保存、編集を残した読み直しである。

持ち主と見直したところ、このうちの多くは UI ライブラリの責務ではなかった。「いつ見張るか」「書き換えられたら保存を断るか」「衝突をどう扱うか」という方針まで、ライブラリが決めてしまっていた。Monaco（VS Code のエディタ部品）はファイルを読まず、それは周りのワークベンチの仕事である。Qt の `QTextDocument` も同じである。

置き場所としては、gowaku の別のパッケージにする案と、gowaku で作るアプリ（Nople）に移す案を比べた。

- **gowaku の別のパッケージ:** 「gowaku なら 10 GB が 24 ms で開ける」と、ライブラリの売りとして言える。代わりに、ファイルまわりの API を公開して約束することになり、OS ごとのロック、変化の検出、保存の安全性について、使う人の環境から報告が来る種類の保守が増える。範囲を絞る方針（配布の支援を `gowaku package` だけに絞ったのと同じ考え方）にも逆らう。
- **Nople に移す:** Nople の独自の機能になり、アプリを入口にするという狙いに合う。Nople は公開する予定なので、開発者は作り込みをそのコードで読める。

## 決定

Editor は、文書のもとを**抽象的な行のソース**として受け取る。ファイルの読み書きと、その方針は、アプリの側に置く。

### gowaku に残すもの

- ピーステーブルの文書（ADR 0153）。ピースは「ソースの行の範囲」か「メモリの行」。
- 裏での検索（ADR 0154）。ソースの行は `LineReader` で読む。
- 編集するときに、置き換えるソースの行と前後 2 行を覚える仕組み（ADR 0156）。文書のモデルの中でしか行えないため。
- そのほか、今回の一連の変更のうち UI の土台にあたるもの: float64 のスクロール（ADR 0150）、Table の `RowCount`/`Row`（ADR 0151）、`Poster`（ADR 0152）。

### API

```go
type LineSource interface {
	Len() int                 // 今わかっている行の数（1 以上）
	Line(i int) string        // 改行を除いた i 行目。見えている行は毎フレーム呼ぶ
	Offset(i int) int64       // i 行目が始まる位置（1 行につき改行 1 バイト）
	LineAt(off int64) int     // その逆
	Reader() LineReader       // ほかの goroutine から読むためのもの
}
type LineReader interface {
	ReadLines(from, to int, fn func(i int, line string) bool) error
	Close() error
}
type LineWriter interface { // 任意。元の改行のまま書く
	WriteLines(w io.Writer, from, to int) error
}
```

`Controller` には次を置く。

- `SetSource(src)`、`Source()`
- `SourceGrew(from)`: ソースの from 行目から先が新しく増えた、または変わった（末尾に限る）。読み込み中のファイルにも、続きが書き足された最後の行にも使える。
- `WriteTo(w, nl)`: ピースを順に書く。ソースの行は `LineWriter` があればそれで、無ければメモリの行と同じく `nl` でつなぐ。
- `Edits() []SourceEdit`: ソースに対する編集の塊。置き換えた行、前後 2 行、入れた行を持つ。両端の変わっていない行は削る。
- `Rebase(src, []PlacedEdit)`: 新しいソースに、アプリが置き場所を決めた編集だけを載せる。置き場所を探す処理（patch）はアプリの仕事にした。

`Offset`/`LineAt` をソースに持たせたのは、Editor が自分でバイト位置を数えると、全部の行の長さを読むことになるからである。索引を持つソースが答える。

### 消したもの

`OpenFile`、`Save`、`SaveAs`、`Close`、`Loading`、`LoadErr`、`FilePath`、`FileOptions`、`WatchPeriod`、`FileChange`、`CheckFile`、`Reload`、`ReloadKeepingEdits`、`EditConflict`、`ErrLoading`、`ErrNoFile`、`ErrChangedOnDisk`。未公開なので、互換の層は作らない。実装は、コミット 9ca9b90 の `widgets/editor/editor_file.go` などに残っており、Nople に移すときの元にする。

### 置き場所

- **gowaku の examples:** 読み取り専用の簡単なファイルのソース（索引とキャッシュ）と、それで巨大なファイルを開く実演。README とサイトの数字はこれで取り直す。
- **Nople:** 見張り、追記の追いかけ、Windows でファイルを開いたままにしないこと、安全な保存、編集を残した読み直し。

## 結果

- テストは、メモリの上のソース（`memSource`）で書き直した。行とオフセット、見えている行だけを読む（100 万行で末尾へ飛んでも 2,000 行以下）、少しずつ増える、最後の行に続きができる、増えている途中と後のランダムな編集を文字列と比べる、`WriteTo` で元の改行が残る、`LineWriter` の無いソース、ソースの両端の編集、ソースが変わった後も `Edits` が覚えておいた行を返す、`Rebase` で編集がずれて当たり、重なった置き場所は捨てる、Editor に表示したまま増えながら検索する。race 検出器あり。
- 残り: examples の簡単なソース、README とサイトの数字の取り直し、Nople への移植。
