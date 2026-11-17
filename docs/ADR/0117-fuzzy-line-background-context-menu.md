# 0117: あいまい一致の公開、行の幅いっぱいの背景、右クリックで選ぶ行と OnContextMenu

決定日: 2026-11-14

状態: 採用

## 背景

gowaku で書いたコードと Markdown の閲覧アプリからの要望 3 つ。

- **#17:** コマンドパレットの一致（`kit.Fuzzy`）が internal で、アプリのクイックオープンは自前の一致を使っている。同じアプリの中で一致の仕方と強調の見た目が違う。
- **#18:** `editor.Decoration.Background` は文字の後ろだけを塗る。差分の追加・削除の行の色が文字の終わりで切れ、空行には色が付かない。アプリはガターの帯で代わりにしている。
- **#20:** Tree の行を右クリックしても行が選ばれない。コンテキストメニューが前に選んでいた行に効いてしまう。

## 1. あいまい一致の公開（#17）

### 選択肢

| | 置き場所 | 判断 |
|---|---|---|
| A. `widgets/field` の `field.Fuzzy` | パレットと同じパッケージ。`collection` / `editor`（段 6）からも使える | **採る** |
| B. `text` に `text.Fuzzy` | 組版の層に文字列の照合を混ぜる。`text` の持ち主はほかの作業者で、この作業の範囲外 | 採らない |
| C. 新しいトップレベルの `fuzzy` パッケージ | depguard の規則と規約 §3.1 の層の図を足すことになる。中身は型 1 つ | 採らない |
| D. `kit` に残して別名だけ公開 | `kit` は internal。別名を置くと同じ型が 2 か所に出る | 採らない |

`field` は「打って値を選ぶ」部品の段で、照合はその仕事に当たる。パレットを使うアプリはすでに `field` を import している。`nav` は `field` と同じ段なので使えないが、`nav` に一致で絞る部品は無い。`kit.Fuzzy` はパレットだけが使っていたので、`kit` から `field` に移した（別名や互換の層は置かない。gowaku は未公開）。

### 決定（API）

```go
type Fuzzy struct{ /* バッファだけ */ }
func (f *Fuzzy) SetQuery(q string)
func (f *Fuzzy) Empty() bool
func (f *Fuzzy) Match(label string) (score int, ok bool)
func (f *Fuzzy) Ranges() []int
const FuzzyMaxQuery = 64
const FuzzyMaxLabel = 256

func FuzzySpans(dst []text.Span, label string, ranges []int, style, hit text.Style) []text.Span
func FuzzyHighlight(style text.Style, mark geom.Color) text.Style
```

- **点数**（ADR 0097 と同じ。doc コメントに数字まで書いた）: 一致した文字 16、語頭 +10、ラベルの頭 +6、直前の一致に続けば +8。間を空けると 3、空けた文字ごとに 1 を引く。最初の一致より前の文字は 1 つ 1 点、8 点まで引く。点数は同じ問い合わせの間で比べるもので、絶対値に意味は無い。
- **畳み込み:** 大文字と小文字（`unicode.ToLower`）と文字幅（全角英数字・記号 ↔ ASCII、半角カナ → 全角）。ひらがなとカタカナは別、読みは引かない。問い合わせの空白・全角空白・タブは無視する。
- **Ranges の意味:** 渡したラベル（畳み込む前）の**バイト**オフセットの組 `[start, end)` を順に並べたもの。どれも rune の境界にあり、隣り合った一致は 1 つの範囲にまとまるので、範囲どうしは接しない。空の問い合わせと一致しなかったときは空。スライスは次の `Match` で使い回すので、残すならコピーする。`basic.RichText` の `text.Span` もバイトで切るので、そのまま使える。
- **強調の補助:** `FuzzySpans` は範囲を `hit`、残りを `style` にした `text.Span` を `dst` に足す。文字列はラベルを切り出すだけなので、`dst` に余裕があればアロケーションしない。ラベルに収まらない範囲（別のラベルの結果を渡した）は飛ばす。`FuzzyHighlight` はパレットと同じ見た目（色 `mark`、太さ 600、もとが 600 以上なら 700）を返す。パレットもこの 2 つで描くようにした。
- **アロケーション:** `Match` はバッファが最長の問い合わせとラベルまで育ったあとは 0（既存の `TestFuzzy_DoesNotAllocate`）。`FuzzySpans` も `dst` に余裕があれば 0（`TestFuzzySpans_DoesNotAllocate`）。
- **直したこと:** ラベルが 256 文字を超えるとき、256 文字目で一致した範囲がラベルの終わりまで伸びていた。探した最後の文字の終わりで止める。

## 2. 行の幅いっぱいの背景（#18）

### 選択肢

| | 内容 | 判断 |
|---|---|---|
| A. `Decoration` に「行全体」の印を足す | `Start` / `End` が意味を失う値が混ざる。行ごとに 1 つで足りるものをスライスで返すことになる | 採らない |
| B. 新しいコールバック `LineStyle func(i int, s string) LineStyle` | 行ごとに値を 1 つ返す。スライスも比較も要らない | **採る** |

### 決定（API）

```go
type LineStyle struct {
	Background geom.Color // テキスト領域の幅いっぱい
	Gutter     geom.Color // 行の横のガター
	Bar        geom.Color // テキスト領域の左端の帯（幅 LineBarWidth = 3）
}
// Editor のフィールド
LineStyle func(i int, s string) LineStyle
```

- **塗る範囲:** ガターの右端からエディターの右端まで、行のすべての表示行（折り返しのとき）。空行も塗る。横にスクロールしても範囲は画面に対して決まり、文字と一緒に動かない（現在行の強調と同じ計算）。
- **重なり:** 行の背景 → 現在行の強調 → 検索の一致 → `Decoration` の背景 → 選択 → 文字。行の背景は一番下で、選択と一致はその上に見える。`Bar` は文字の後に描く。横にスクロールした文字は帯の下を通る（テキスト領域の左の余白は 8 で、スクロールしていなければ帯は文字に掛からない）。`Gutter` は行番号の下。
- **呼ぶ時:** `Decorations` と同じく行を組むときに呼び、`lineEntry` に値を持つ。描画では呼ばない。だから `Redecorate` / `RedecorateLines` がそのまま効き、ADR 0103 の複数のビューでもビューごとのキャッシュにそれぞれ届く（各ビューで 1 回ずつ呼ばれる。テストで数えた）。行の内容が変われば勝手に呼び直す。
- **IME の変換中** は `Decorations` を呼ばない（位置がずれる）が、`LineStyle` は変換中の文字を除いた行の文字で呼ぶ。行の色が変換の間だけ消えるのを避けた。
- **アロケーション:** 描画は今までどおり 0（`TestEditor_LineStylePaintAllocatesNothing`、折り返しの有無の両方）。

## 3. 右クリックで行を選ぶ、OnContextMenu（#20）

### 何があったか

- `Tree` は副ボタンの押下を見ていなかった（主ボタン以外は素通り）。外側の `overlay.ContextMenu` は開くが、行は選ばれない。
- `Table` は副ボタンで「選ばれていない行は選び、選択の中なら保つ」をすでにしていた。ただし押下を全部 `Handled` にしていたので、外側の `overlay.ContextMenu` には届かず、表にはメニューを付けられなかった。選択の中の行を右クリックしたときにフォーカスも動かなかった。
- `GridView` は副ボタンを見ていなかった。
- `List` は選択もフォーカスする項目も持たない（項目はアプリのウィジェット）。項目を `overlay.ContextMenu` で包めば済むので、何も足さない。

### 決定

**選び方（Windows のエクスプローラーと VS Code に合わせる）:**

- 副ボタンの押下で、その行（タイル）が選択の中に無ければ、それだけを選ぶ。修飾キーは見ない。選択の中にあれば選択を保つ（複数を選んでメニューで一括に操作できる）。
- どちらの場合もフォーカスをその行に移し、キーボードを渡す（主ボタンの押下と同じ）。
- シェブロンの上でも行を選ぶ（主ボタンのときのような開閉はしない）。
- ドラッグは始めない。

**`OnContextMenu`:**

```go
Tree[K].OnContextMenu  func(node K, pos geom.Offset)
GridView.OnContextMenu func(i int, pos geom.Offset)
Table[T].OnContextMenu func(row int, pos geom.Offset) // row は Rows の添字（OnActivate と同じ）
```

- 引数は 2 つなので、規約 §5.1 に従ってイベント構造体にしない。名前は `OnActivate(node)` に並べた。
- **呼ぶ時:** 副ボタンを離したとき（押した行の上で離したときだけ。`GestureDetector.OnSecondaryTapAt` と同じく離した位置）。Windows の WM_CONTEXTMENU も離したときに出る。押したときに開くと、離した指がメニューの項目に当たる心配もある。キーボードでは、フォーカスのある行でメニューキーか Shift+F10（`overlay.ContextMenu` と同じキー。判定は `kit.IsContextMenuKey` にまとめた）。
- **pos:** ウィジェット（Tree / GridView / Table）の左上からの位置。`GestureDetector` と `overlay.ShowPopupMenu` の「ctx のウィジェットの左上から」に揃えた。キーのときは、Tree は行のラベルの左下、GridView はタイルの左下、Table はフォーカスのあるセルの左下（固定列の下に隠れるときは固定列の右）。アプリは Tree を `basic.Builder` で包み、その ctx で `overlay.ShowPopupMenu(ctx, pos, items)` を呼ぶ（ギャラリーの例）。
- **OnContextMenu が無いとき:** 押下は `Handled` にせず、外側の `overlay.ContextMenu` に渡す。今まで Tree を `ContextMenu` で包んでいたアプリはそのまま動き、行が選ばれるようになる。Table も包めるようになった。誰も押下を取らないと core が押下でフォーカスを外す（何も無い所の押下と同じ扱い）ので、行は次のレイアウトでキーボードを取り戻す。ただしその時点で何かがフォーカスを持っていれば（開いたメニュー）、取り戻さない。
- **OnContextMenu があるとき:** 押下を `Handled` にして離すまで持つ。外側の `ContextMenu` は開かない（両方を付けると二重に開くので、どちらかにする）。

### やらなかったこと

- タッチとペンの長押しで `OnContextMenu` を呼ぶこと。Tree の主ボタンの押下はドラッグ（`OnMove`）の始まりも兼ねていて、長押しとの区別を Tree の中で持つことになる。指やペンでは、今までどおり外側の `overlay.ContextMenu` が長押しで開く。
- キーボードで開いたときに、メニューの最初の項目にフォーカスを置くこと。`overlay.ShowPopupMenu` にその口が無く（`ContextMenu` は内部の `openMenu` で置いている）、`overlay` はこの作業の範囲外。上下キーで項目に入れる。`overlay` に「キーボードから開いた」を渡す公開の口ができたら、`OnContextMenu` の引数を足すか、イベント構造体にする。
- 右から左のグリッドで、キーのメニューをタイルの右下に出すこと。

## 結果

- `field.Fuzzy` / `FuzzySpans` / `FuzzyHighlight` で、アプリのリストがパレットと同じ一致と強調になる。`kit.Fuzzy` は無くなった。
- `editor.LineStyle` で差分の行が幅いっぱいに塗れ、ガターの帯の代用は要らない。
- Tree / GridView / Table は右クリックで行を選び、`OnContextMenu` でメニューを開ける。
- ギャラリーに「差分とメニュー」のページ（`-state lines`）を足した。折り返した差分、クイックオープン、ツリーのメニューを 1 画面で見せる。
