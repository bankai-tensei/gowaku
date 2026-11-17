# レイアウト

[ガイドの目次](README.md) · [English](../en/layout.md)

gowaku が各ウィジェットのサイズと位置をどのように決めるかを説明し、レイアウト用のウィジェットをひととおり紹介したうえで、独自のレイアウトの書き方を示す。パッケージ名を明記していないウィジェットは `widgets/basic` にある。配置と軸の定数は `layout` に、幾何の型は `geom` にある。

## 制約のプロトコル

レイアウトは、ツリーを下ってから戻ってくる 1 回の走査で行われる。

- **制約は下へ。** 親は各子に、取ってよいサイズの範囲を `geom.Constraints{MinW, MaxW, MinH, MaxH}` で伝える。最大値が `geom.Inf` なら上限はない。
- **サイズは上へ。** 子はその範囲内でサイズを決めて返す。
- **配置は親が決める。** 子のサイズが決まった後、親が子の位置を決める。

子は自分の位置を選べず、親が許す範囲より大きくなることもできない。よく使われる制約は次の 2 種類である。

- **tight**: 最小値と最大値が等しく、子のサイズが 1 つに決まる。ウィンドウはルートのウィジェットに、ウィンドウサイズの tight な制約を渡す。
- **loose**: 最小値が 0 で、子は小さくなってもよい。`Center` と `Align` は受け取った制約を緩めてから子に渡す。

長さの単位は論理ピクセルで、gowaku が画面の表示倍率に合わせて変換する。

`layout.Delegate` や `basic.LayoutBuilder` が受け取る `layout.Constraints` は `geom.Constraints` と同じ型である。そのため補助の関数とメソッドは `geom` にある。制約は `geom.Tight(size)` と `geom.Loose(size)` で作る。値 `c` に対しては、`c.Biggest()` が許される最大のサイズ、`c.Constrain(size)` がサイズを範囲内に収めたもの、`c.Loosen()` が最小値を 0 にしたもの、`c.Deflate(insets)` が余白の分だけ縮めたものを返し、`c.IsTight()` はサイズが 1 つに決まるかどうかを返す。`layout.Tight` は存在しない。

## ボックス

| ウィジェット | 役割 |
|---|---|
| `Padding{Insets, Child}` | 子の周りに余白を取る。余白は `geom.All(16)` や `geom.Symmetric(16, 8)` で作る。`basic.Pad(16, child)` は前者の短縮形。 |
| `Center{Child}` | 与えられた領域の中央に子を置く。 |
| `Align{Alignment, Child}` | 指定した配置の位置に子を置く。 |
| `SizedBox{Width, Height, Child}` | 幅か高さ、またはその両方を固定する。長さに 0 を指定した軸は固定しない。 |
| `ConstrainedBox{MinWidth, MaxWidth, MinHeight, MaxHeight, Child}` | 制約を追加する。最大値が 0 なら制限しない。 |
| `Container{...}` | サイズ、余白、色、グラデーション、枠線、角丸、影をまとめて指定する。 |
| `DecoratedBox{Decoration, Child}` | 子の背後に `layout.Decoration` を描画する。 |
| `AspectRatio{Ratio, Child}` | 子を `Ratio`（幅 ÷ 高さ。`16.0 / 9` など）の比の大きさにする。使える幅いっぱいに広がり、それでは高すぎる場合は使える高さいっぱいにする。0 は 1（正方形）として扱う。 |

`AspectRatio` は、両方の長さが決まっている tight な制約（ウィンドウや、`CrossStretch`、`Expanded` が渡すもの）の下では比を反映できない。そのような場所では `Center` か `Align` の中に置く。そうすれば、収まる範囲で最大のその比の矩形になる。

```go
func player(video gowaku.Widget) gowaku.Widget {
	return basic.Center{Child: basic.AspectRatio{Ratio: 16.0 / 9, Child: video}}
}
```

`Alignment` は各軸を 0 から 1 で表し、`{X: 0, Y: 0}` が左上、`{X: 0.5, Y: 0.5}` が中央になる。Flutter のような -1〜1 **ではない**。`layout.AlignTopLeft` から `layout.AlignBottomRight` まで名前付きの値が用意されているので、`layout.AlignCenterLeft` のようにそれを使うとよい。

## 行と列

`Row` は子を左から右へ、`Column` は上から下へ並べる。どちらも同じフィールドを持つ。

- `Main` は主軸方向の配置。`layout.MainStart`、`MainEnd`、`MainCenter`、`MainSpaceBetween`、`MainSpaceAround`、`MainSpaceEvenly`。
- `Cross` は主軸と直交する方向の配置。`layout.CrossStart`、`CrossEnd`、`CrossCenter`、`CrossStretch`。
- `Size` は主軸方向の長さ。ゼロ値の `layout.MainMax` は使える長さをすべて使い、`layout.MainMin` は子に合わせて縮む。
- `Spacing` は子と子の間隔。

`Row` と `Column` は既定で領域をすべて使うので、`Center` の中に置くと中央に収まらず、`Center` いっぱいに広がってしまう。`Size: layout.MainMin` を指定するか、それを指定済みの `basic.VStack(...)` と `basic.HStack(...)` を使う。`Flex` は、軸を `Axis` フィールドで指定する同等のウィジェットである。

`VStack` と `HStack` は、子と子の間に `theme.Space8` も空ける。SwiftUI のスタックが子の間を空けるのと同じで、8 はデザインの指針でコントロールの間に置く間隔である。ほかの間隔にするには、`Column` か `Row` を書いて `Spacing` を指定する。ページのセクションの間なら `theme.Space24`、間を空けないならゼロである。

```go
basic.VStack(title, field, button) // 間隔は 8
basic.Column{Size: layout.MainMin, Spacing: theme.Space24, Children: []gowaku.Widget{general, advanced, reset}}
```

### 余った領域を分配する

`Expanded` と `Flexible` は、`Row` や `Column` の他の子が使わずに残した領域から、子に取り分を割り当てる。`Flex` は重みで、0 は 1 と同じに扱われる。`Expanded` の子は取り分をすべて埋める必要があるが、`Flexible` の子はそれより小さくてもよい。

```go
func toolbar(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
		basic.Expanded{Child: basic.Text{Content: "Title", Style: th.Text.Headline}},
		basic.Flexible{Child: basic.Text{Content: "A note that may be cut short"}},
		control.Button{Label: "Share"},
	}}
}
```

よくあるページの構成では、`Expanded` で包んだ本体の上にヘッダーを、下にフッターを置く。

```go
func page(ctx gowaku.BuildContext, items []string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Column{
		Spacing: theme.Space12,
		Children: []gowaku.Widget{
			basic.Text{Content: "Inbox", Style: th.Text.LargeTitle},
			basic.Expanded{Child: collection.List{
				ItemCount:  len(items),
				ItemExtent: 36,
				ItemBuilder: func(i int) gowaku.Widget {
					return basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.Text{Content: items[i]}}
				},
			}},
			basic.Row{
				Main:    layout.MainEnd,
				Spacing: theme.Space8,
				Children: []gowaku.Widget{
					control.Button{Label: "Cancel"},
					control.Button{Label: "OK", Style: control.ButtonPrimary},
				},
			},
		},
	}}
}
```

## 重ね合わせ

`Stack` は子を重ねて描画し、最後の子が最も手前に来る。位置を指定していない子は `Alignment` に従って配置され、Stack のサイズはその中で最も大きい子に合わせて決まる。`Fit: layout.StackExpand` を指定すると、使える領域をすべて使う。

- `Positioned` は子を決まった位置に置く。位置は左上から測るが、`AnchorRight` と `AnchorBottom` を指定すると右端と下端から測る。`Width` と `Height` でサイズを固定することもできる。
- `PositionedFill` は子を Stack 全体に広げ、`Insets` の分だけ内側に縮める。背景によく使う。

位置を指定した子は、Stack のサイズの計算には含まれない。

押下は、その位置で当たる最も手前の子に届く。配置だけのウィジェット（`Align`、`Center`、`Padding`、`Row`、`Column`、`Stack`、`SizedBox`、`PositionedFill` など）は子のある所でしか当たらないので、全面の層の空いた所では、ポインターはその下の層に届く。`Color`、`Gradient`、`Border`、`Shadows` のどれかを持つ `Container` は箱の全体で当たり、文字や画像、ポインターを受け取るウィジェット（`GestureDetector`、`Pressable`、`Listener`）も同じである。`IgnorePointer` は、子の全体でポインターを通す。

```go
func card(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.SizedBox{Width: 240, Height: 140, Child: basic.Stack{Children: []gowaku.Widget{
		basic.PositionedFill{Child: basic.Container{Color: th.Color.Card, Radius: th.Radius.Group}},
		basic.Positioned{Left: 16, Top: 16, Child: basic.Text{Content: "Album", Style: th.Text.Title}},
		basic.Positioned{Right: 12, Bottom: 12, AnchorRight: true, AnchorBottom: true,
			Child: control.Button{Label: "Play", Style: control.ButtonPrimary}},
	}}}
}
```

## 折り返しとグリッド

`Wrap` は、次の子が収まらなければ改行して新しい行を始める `Row` で、タグやチップの表示に向いている。`Spacing` は行内の間隔、`RunSpacing` は行と行の間隔を表す。

```go
func tags(names []string) gowaku.Widget {
	chips := make([]gowaku.Widget, len(names))
	for i, n := range names {
		chips[i] = control.Chip{Label: n}
	}
	return basic.Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: chips}
}
```

`Grid` は同じ幅の列にタイルを並べる。`Columns` で列数を固定でき、0 にすると `MinColumnWidth` が許す限り多くの列を入れるので、ウィンドウを広げると列が増える。タイルの高さは `TileHeight` で指定するか、列の幅を `AspectRatio` で割って `ExtraHeight` を足した値になる。Grid の高さは全行の合計になるため、縦方向の `Scroll` の中に置く。

```go
func covers(n int) gowaku.Widget {
	tiles := make([]gowaku.Widget, n)
	for i := range tiles {
		tiles[i] = basic.Text{Content: fmt.Sprintf("Cover %d", i+1)}
	}
	return basic.Scroll{Child: basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Grid{
		MinColumnWidth: 160,
		Spacing:        theme.Space12,
		RunSpacing:     theme.Space12,
		AspectRatio:    1,
		ExtraHeight:    40,
		Children:       tiles,
	}}}
}
```

`Grid` はすべての子を生成する。タイルが数千個ある場合は、見えている分だけを生成する `collection.GridView` を使う。

## スクロール

- `Scroll{Child}` は自身より背の高い子を表示し、マウスホイールか、端に現れるスクロールバーでスクロールする。`Horizontal: true` で横方向にスクロールする。`Reverse: true` でスクロールの範囲が右端（縦方向では下端）から始まり、表示はそこから開き、短い内容はその端に寄る。縦書きの文章やログに使う。`Controller` に `ScrollController` を渡すと、コードから位置を読み書きできる。子はすべて生成してレイアウトするので、1 ページ程度の内容には向いているが、1 万行のデータには向かない。
- リストビューには `collection.List` を使う。表示範囲の近くにある行だけを生成するので、行数が多くても問題にならない。行の高さは `ItemExtent`（既定値 48）でそろえる。`ItemExtentOf` を設定すれば行ごとに高さを返せ、`MeasureItems` を指定すれば各行がレイアウトした結果の高さになる。`Controller` に `collection.ListController` を渡すと、コードから指定した行や末尾までスクロールでき、表示が末尾にあるかどうかも分かる。`StickToEnd: true` にすると、ログやチャットで、ユーザーが上にスクロールするまで最新の行を表示し続ける。位置が変わるたびに `OnScroll` が呼ばれる。

```go
collection.List{
	ItemCount:   10000,
	ItemExtent:  52,
	ItemBuilder: func(i int) gowaku.Widget { return row(i) },
}
```

リストの中に置いた見出しは、行と一緒にスクロールして流れていく。見出しを固定したい場合は、前述のページの例のように見出しをリストの上に置き、リストを `Expanded` で包む。カタログの[データのウィジェット](../../catalog/ja/data.md)（`collection.Table`、`collection.Tree` など）も同じようにスクロールする。

## 上限のない領域

ウィジェットの中には、長さに上限がないと意味をなさないものがある。上限がない場合、これらは直し方を書いたメッセージとともに panic する。

- `Scroll`、`collection.List`、`Grid` は与えられた領域をすべて使う。`Column` は flexible でない子に上限のない高さを渡すので、`Expanded` か `SizedBox` で包む必要がある。
- 主軸方向に上限のない `Row` や `Column`（同じ向きの `Scroll` の中など）にある `Expanded` と `Flexible` には、分配する余りの領域がない。
- 交差軸方向に上限がないときの `CrossStretch`（縦方向の `Scroll` の中の `Row` など）には、伸ばす先がない。

たいていは、スクロールするウィジェットを `Expanded` で包むか、`SizedBox` で長さを与えれば解決する。詳しくは [ADR 0026](../../ADR/0026-unbounded-room.md) を参照のこと。

## 使える大きさに応じて構築する

ウィジェットはレイアウトより前に構築されるので、自分の大きさを知らない。使える大きさによって構築するものを変える方法は 2 つある。

**ウィンドウの大きさ。** `basic.WindowSizeOf(ctx)` は、ウィンドウの内容の領域の大きさ（論理ピクセル）で、`basic.MediaQueryData` の `Size` と同じ値である。ウィンドウが狭いときはサイドバーを畳むといった、ウィンドウ全体に関わる判断に使う。ウィンドウの大きさが変わったときに再構築されるのは大きさを読んだウィジェットだけで、何も起きていないウィンドウには負荷がかからない。

```go
func shell(ctx gowaku.BuildContext, sidebar, content gowaku.Widget) gowaku.Widget {
	if basic.WindowSizeOf(ctx).W < 700 {
		return content
	}
	return basic.Row{Children: []gowaku.Widget{basic.SizedBox{Width: 220, Child: sidebar}, basic.Expanded{Child: content}}}
}
```

`basic.MediaQueryOf(ctx).Size` も同じ値だが、`MediaQueryOf` を呼んだウィジェットも大きさの変化で再構築される。ほかの項目だけが要るウィジェットは、`basic.ReduceMotionOf` や `basic.LocaleOf` で読む。

**ウィジェットに与えられた領域。** 利用者が幅をドラッグで変えるペインや、どこに置かれるか分からないカードのように、ウィンドウの一部の大きさで決めたいときは `LayoutBuilder` を使う。`Content` は親から渡された制約を受け取る。

```go
func panes(list, detail gowaku.Widget) gowaku.Widget {
	return basic.LayoutBuilder{Content: func(ctx gowaku.BuildContext, c layout.Constraints) gowaku.Widget {
		if c.MaxW < 600 {
			return basic.Column{Children: []gowaku.Widget{basic.Expanded{Child: list}, basic.Expanded{Child: detail}}}
		}
		return basic.Row{Spacing: theme.Space16, Children: []gowaku.Widget{
			basic.SizedBox{Width: 240, Child: list},
			basic.Expanded{Child: detail},
		}}
	}}
}
```

- `Content` が再び実行されるのは、制約が変わったとき、親が `LayoutBuilder` を構築し直したとき、`ctx` から読み取ったもの（テーマなど）が変わったときだけである。同じ制約でレイアウトされるフレームでは何も構築しない。
- `LayoutBuilder` の大きさは、制約の範囲内で `Content` が返したものの大きさになる。
- 制約はレイアウトが `LayoutBuilder` に届いた時点で分かる。そこでフレームは、描画する前に `Content` を構築してもう一度レイアウトする。`collection.List` が自分の高さを知ったときと同じ仕組みである。`LayoutBuilder` の `Content` の中に別の `LayoutBuilder` があると、この往復が 1 回増える。1 フレームで行う往復は数回までなので、深く入れ子にすると内側の中身の表示が 1 フレーム遅れることがある。
- 何を構築するかを選ぶのではなく、子の大きさに合わせて子を並べたい場合は、次の節のように Delegate を書く。

## 独自のレイアウト

標準のウィジェットでは思いどおりに並べられない場合は、`layout.Delegate` を実装して `basic.CustomLayout` に渡す。Delegate は制約と子の RenderObject を受け取り、各子を `core.LayoutChild` でレイアウトして `SetOffset` で配置し、全体のサイズを返す。

```go
// diagonal places each child a step further right and down.
type diagonal struct{ step float32 }

func (d diagonal) Layout(c layout.Constraints, children []core.RenderObject) geom.Size {
	var size geom.Size
	for i, child := range children {
		sz := core.LayoutChild(child, c.Loosen())
		off := geom.Offset{X: float32(i) * d.step, Y: float32(i) * d.step}
		child.SetOffset(off)
		size.W = max(size.W, off.X+sz.W)
		size.H = max(size.H, off.Y+sz.H)
	}
	return c.Constrain(size)
}

func staircase() gowaku.Widget {
	return basic.CustomLayout{Delegate: diagonal{step: 24}, Children: []gowaku.Widget{
		basic.Text{Content: "One"}, basic.Text{Content: "Two"}, basic.Text{Content: "Three"},
	}}
}
```

Delegate のルール:

- 子のレイアウトには必ず `core.LayoutChild` を使い、子の `Layout` を直接呼び出さない。`LayoutChild` は制約が変わっていない子の処理を省略する。
- `c` の範囲に収まるサイズを返す。`c.Constrain` を使えば範囲内に収められる。`basic.CustomLayout` の側でも結果を範囲内に収める。
- レイアウトは必要になったフレームのたびに実行されるので、`Layout` の中でメモリを割り当てない。
- 再ビルドのたびに新しい Delegate が RenderObject に渡され、再レイアウトが行われる。そのため Delegate は `diagonal{step: 24}` のような小さな値で構わない。

描画やヒットテストまで自分で扱う場合は、RenderObject を書く。[基本の考え方](concepts.md#renderobject)を参照のこと。

## さらに読む

- ウィジェットカタログの[レイアウトのページ](../../catalog/ja/layout.md)。各ウィジェットのスクリーンショットがある。
- `theme.Space16` などの余白のトークンについては[テーマ](theme.md)。
