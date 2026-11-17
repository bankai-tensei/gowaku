# 0159: 文書の写しを取り、UI を止めずに別の goroutine で書き出す

決定日: 2026-11-18

状態: 採用

## 背景

Nople は 8 MB を超えるファイルを `LineSource`（ADR 0157）で開き、`Controller.WriteTo` で保存している。`WriteTo` は UI の goroutine で動き、ソースの行をすべて読むので、保存のあいだウィンドウが固まる。Nople の計測では 1 GB で約 1 秒、10 GB で約 50 秒かかり、Windows では「応答なし」になる。

保存を別の goroutine に移したいが、今のままではできない。

- `Controller` は goroutine に対して安全ではない。書き出しの途中で編集されると、ピースの並びとメモリ上の行が変わる。
- `WriteTo` はソースの `Line` と `WriteLines` を呼ぶ。どちらも UI の goroutine のもので、ソースのキャッシュを描画と同時に触ることになる。goroutine をまたいで読んでよいのは `LineSource.Reader` が返す `LineReader` だけ。

## 決定

### `Controller.Snapshot`

```go
func (c *Controller) Snapshot() *Snapshot           // UI の goroutine で
func (s *Snapshot) WriteTo(w io.Writer, nl string) error // どの goroutine でも
func (s *Snapshot) Size(nl string) int64
func (s *Snapshot) Lines() int
func (s *Snapshot) Close() error
func (c *Controller) Changed(s *Snapshot) bool      // UI の goroutine で
```

- **写しに持つもの**: ピースの並びの写し、メモリ上のピースの行の一覧の写し、ソースの `Reader()`。メモリ上の行は、編集が配列をその場で書き換えるので一覧ごと写す。写すのは文字列のヘッダー（1 行 16 バイト）で、本文ではない。ソースの行は 1 行も読まない。`SetText` で入れた 100 万行の文書なら 16 MB の複写になる。
- **`Reader()` は `Snapshot` の中で呼ぶ**。`LineSource` のメソッドは UI の goroutine のものなので、書き出す goroutine から `src.Reader()` を呼ばせない。リーダーは「`Reader` を呼んだときにソースが持っていた行」を読む約束なので、写しを取った時点の行がそのまま読める。リーダーを持つので、写しは `Close` で閉じる。
- **`Snapshot.WriteTo` は `Controller.WriteTo` と同じものを書く**。ソースの行は `LineReader` を通してだけ読む。
  - その `LineReader` が `LineWriter` も満たすなら、編集していない行を `WriteLines` で元の改行のまま書く。
  - 満たさなければ `ReadLines` で読み、行の間に `nl` を書く。
  - **ソース自身の `LineWriter` は使わない**。UI の goroutine のものだから。ソースだけが `LineWriter` でリーダーがそうでない場合、`Controller.WriteTo` は元の改行を保つが、`Snapshot.WriteTo` は `nl` に揃える。元の改行を保ちたいソースは、リーダーにも `WriteLines` を持たせる。`LineWriter` のコメントにこの約束を書いた。
- **`Size(nl)`** は書き出すバイト数。「保存中 42%」のような表示に使う。`LineWriter` が `nl` と長さの違う改行（`nl` が `\n` のときの `\r\n`）で書く行の分だけずれるので、目安として使う。書いたバイト数は、呼び出し側が `io.Writer` を包んで数える。
- **`WriteTo` は `recover` を使わない**。最初の書き込みの失敗を覚えて返す小さな書き手（`lineOut`）にした。

### 写しを取った後の編集と `ReplaceSource`

写しは、その後の編集、元に戻す、`SetText`、`SetSource`、`Rebase`、`SourceGrew` のどれにも影響されない。写しを取った後に文書を編集してもよく、書き出されるのは写しを取った時点の本文になる。

保存の後に `ReplaceSource` を呼ぶときの注意が一つある。`ReplaceSource` は行数とバイト数しか比べない（ADR 0158）。写しを取った後に、行数もバイト数も変えない編集（1 文字の置き換え）があると、保存したファイルは今の本文と違うのに `ErrNotSameText` にならず、通ってしまう。

そこで `Controller.Changed(snapshot)` を足した。バッファの編集の世代を写しに覚えておき、変わっていれば true を返す。`ReplaceSource` 自体は本文を変えないので、世代は進まない。

- 保存中に編集を止めるアプリ（Nople は ReadOnly にする）: 保存が終わったら `Changed` が false のはずなので、そのまま `ReplaceSource` を呼べばよい。念のため `Changed` を見て、true なら `ReplaceSource` を呼ばずに「保存後に変更あり」として扱う。
- 保存中も編集させるアプリ: `Changed` が true なら `ReplaceSource` は呼ばない。文書は古いソースと編集のまま残り、保存したファイルとの差は次の保存で書かれる。

`ReplaceSource` の中で写しを受け取って確かめる形（`ReplaceSource(src, snapshot)`）も考えたが、既存の呼び出しを変えることになり、同期の保存では写しが要らないので、別の関数にした。

## 使い方

```go
// UI の goroutine
snap := ctl.Snapshot()
total := snap.Size(nl)
go func() {
	err := snap.WriteTo(countingWriter{f, progress}, nl) // 別の goroutine
	post(func() {                                        // UI の goroutine に戻る
		defer snap.Close()
		if err != nil || ctl.Changed(snap) {
			return
		}
		_ = ctl.ReplaceSource(open(path))
	})
}()
```

## 確かめたこと

- 15 通りの本文と編集（未編集、空、末尾が改行、途中の編集、最後の行への追記、最後の行の削除、ソースの途中で終わる本文、複数の編集など）を、改行 2 種類、ソース 3 種類（書けるリーダー、読むだけのリーダー、ソース無し）で書き、`Controller.WriteTo` と 1 バイトも違わないこと。
- 写しを取った後の編集、元に戻す、`SetText` の後も、同じ本文が書かれること。ソースが伸びた後も、写しを取った時点の行までが書かれること。
- 書き手の失敗とソースの失敗が、そのまま返ること。
- 2 万行の文書を別の goroutine で書きながら、UI の goroutine で編集、行の読み出し、元に戻すを続け、race detector が何も言わず、書かれた本文が写しを取った時点のものであること。

## 入れなかったもの

- 書き出しの中断。`io.Writer` が失敗を返せば `WriteTo` は止まるので、呼び出し側が書き手を包んで止められる。
- 進み具合の通知。上と同じく、書き手を包んで数える。
- 1 つの写しを同時に複数の goroutine で書くこと。リーダーが 1 つなので、1 回ずつ。
