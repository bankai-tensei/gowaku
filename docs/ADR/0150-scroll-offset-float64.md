# 0150: スクロールの位置を float64 にする

決定日: 2026-11-17

状態: 採用

## 背景

巨大なログや CSV を開く閲覧アプリを gowaku で作れるかを調べたところ、描画の仮想化は数百万行でも効いていた。500 万行（443 MB）のテキストは読み取り専用の `editor.Editor` で最初のフレームまで 162 ms、`collection.Table` は 100 万行で Layout と Paint が 4.8 µs、アロケーションは 0 である。

ところが、下の方では動かなくなった。スクロールの位置（`layout.ScrollOffset`）が float32 だったためである。float32 で 1 ピクセル刻みを表せるのは 2^24（約 1,678 万）ピクセルまでで、それより先は表せる値が飛び飛びになる。行の高さ 32 の表なら、約 52 万行より先にあたる。

| 内容 | 下の方での最小の刻み | 起きたこと |
|---|---|---|
| Table 100 万行（32 px/行） | 2 px | ほぼ気にならない |
| Table 1,000 万行 | 32 px（1 行） | 11 px のスクロールでまったく動かない |
| Table 1 億行 | 256 px | 40 px のスクロールでも動かない |
| Editor 2,000 万行（20 px/行） | 32 px | 7 px のスクロールで動かない |

タッチパッドの小さなスクロールが丸めで消え、ホイールの 1 段も 1 行単位に飛ぶ。「巨大なファイルでもすっと動く」という売りの根元が崩れる。

## 選択肢

- **A. 内容の中での位置を float64 にする。** スクロールの位置、内容の長さ、行の上端、可変の行の高さの表を float64 で持ち、画面の中の位置（子の Offset、描画の座標）に直すところで float32 に落とす。画面の中は数千ピクセルなので、float32 で足りる。
- **B. 「錨にする行の番号＋そこからのずれ」で持つ。** 行の番号は int なので精度の問題は無くなる。しかし、可変の高さの表（`ExtentTable`、`MeasuredExtents`）、スクロールバー、`ScrollController.JumpTo(offset)` のように「ピクセルでいくつ」と聞く API がすべて、行とずれの組への変換を必要とする。仕組みが大きく変わり、`basic.Scroll` のような行の無い内容には合わない。
- **C. float32 のまま、内容の長さに上限を設ける。** 1,000 万行の表が作れなくなり、そもそもの目的に反する。

## 決定

案 A。ライブラリは未公開なので、型を変えたうえで互換の層は作らない。

### 型を変えたもの

- `layout.ScrollOffset` の `Value()` と `Set()` を float64 に。
- `layout.ScrollMetrics{View, Content}` を float64 に。
- `layout.ExtentTable` は上端を `[]float64` で持つ。`Offset`、`Total` は float64 を返し、`IndexAt` は float64 を受ける。`Extent` は float32 のまま。1 行につき 4 バイトから 8 バイトになる。
- `layout.MeasuredExtents` の `Offset`、`Total`、`IndexAt` も同じ。木はもともと float64 で足していたので、最後に float32 に丸めるのをやめただけである。
- `layout.RenderViewport.OnMeasured(view, content float64)`、`OnJump(float64)`、`RenderList.OnShift(float64)`、`RenderScrollBar.OnDrag(float64)`。`RenderList.OnMeasured(view float32)` は、表示の高さなのでそのまま。
- `basic.Scroller.Jump`、`SetLimit`、`ShiftGlide`、`basic.ScrollState.Jump`、`Measure` を float64 に。
- `basic.ScrollController` の `Offset`、`MaxOffset`、`JumpTo`、`basic.ScrollPosition{Offset, MaxOffset}`、`collection.ListController` の `Offset`、`MaxOffset` を float64 に。

### float32 のまま残したもの

- `geom` の座標、レイアウトの大きさ、描画。画面の中の位置であり、内容の中での絶対位置を持たないため。
- `editor.ScrollPosition{Line, Offset, X}`。`Offset` は行の箱の中でのずれ、`X` は横の位置で、どちらも小さい。
- 横のスクロール（表の列、エディタの横）も同じ `ScrollOffset` なので float64 になるが、読み出すところで float32 に戻している。

### 画面の位置への直し方

内容の中での位置どうしを float64 で引いてから、float32 に直す。たとえば行の上端は `float32(float64(i)*float64(h) - offset)`、エディタでは新しい `viewTop(i)` がこれを行う。float32 にしてから引くと、引く前に精度が失われるので意味が無い。`Table.rowY` はもともとこの形で、精度を失っていたのはスクロールの位置そのものだった。

### 範囲

仮想化して行を自分で並べる部品（`List`、`ReorderableList`、`GridView`、`Tree`、`Table`、`PopupList`、`Editor`）は、これで数億ピクセルまで 1 ピクセルで動く。`basic.Scroll` の子は、大きさが `geom.Size`（float32）なので、そこまで長くはできない。数百万行の内容は、仮想化した部品で見せる。

## 結果

- 新しいテスト: `layout/extents_far_test.go`（300 万行の表で行の上端と行の検索が正確、4 億ピクセルで 1 ピクセルを保つ）、`widgets/collection/scroll_far_test.go`（2,000 万行の List、200 万行で 4 億ピクセルの Table、1,000 万個の GridView が下の方で 1 ピクセルやタッチパッドの 1/10 段で動く）、`widgets/editor/editor_far_test.go`（200 万行の Editor が 1 ピクセルで動き、クリックが正しい行に当たる）。
- 残り:
  - `Table` は `Rows []T` を全部受け取る。外のデータを指すには行番号の配列が要り、1 億行で 800 MB になる。行の数と「i 行目」を渡す形が要る。
  - `Editor` は本文を 1 つの string で受け取る（`SetText`）。数 GB のファイルは、必要な行だけを渡す読み取り専用の外部のソースが要る。
  - スクロールバーのつまみの 1 ピクセルは、1,000 万行では 1 万行を超える。つまみは大まかに動かすためのもので、細かく動かすのはホイールとキーに任せる。
