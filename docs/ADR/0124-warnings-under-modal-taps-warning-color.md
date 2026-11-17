# 0124: モーダルの下を Warnings で調べない、副ボタンとダブルクリックのタップ、Warning の色

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 3 つが見つかった。

1. **`Tester.Warnings` がダイアログの下のページを調べていた。** `collection.Group{Title: "General"}` と `Detail` 付きの `GroupRow` のページで `overlay.ShowDialog` を開き、`Warnings` を呼ぶと、ページの文字 2 つに「contrast below 4.5:1」が出た。ページは幕（スクリム）でわざと暗くしてあるので、コントラストが落ちるのは当然である。ADR 0120 でダイアログやシートの下は Semantics ツリーから外れ、`Find` も `Tap` もそこを見ない。`Warnings` のうち、ラベルの無い部品の検査は Semantics ツリーを読むので外れていたが、はみ出しとコントラストはレンダーオブジェクトを根からなめていて、`Hider` しか見ていなかった。
2. **`gowakutest` に `Tap` / `TapAt` しかなかった。** コンテキストメニューやダブルクリックを試すには、`widgets/collection/contextmenu_test.go` の `secondaryClick` を写し、`tt.Tree().DispatchPointer` に `input.ButtonSecondary` を渡すしかなかった。また `TapAt` は押す・離すの `Time` をいつも 0 にしていたので、同じ所を 2 回 `Tap` すると、木の数え方（500ms 以内、4px 以内）ではダブルクリックになっていた。
3. **テーマに警告の色が無かった。** `Accent`、`Danger`、`Success` はあるが、「注意」を表す色が無く、状態を表示する画面で使う色が無かった。`control.InfoBar` の `SeverityWarning` は、`infobar.go` の中で琥珀色（色相 70、トーン 55 / 78）を自前で作っていた。これは文字として 4.5:1 に届かず、他から使えない。`DataTable` の `CellTone` には Success と Danger しかなかった。

## 選択肢

### 1. Warnings が飛ばすもの

1. **Semantics ツリーに無いものをすべて飛ばす**（`ExcludeSemantics` の中も）。`ExcludeSemantics` は、同じことを言う文字を読み上げから外すためのもので、画面には見えている。見えている文字のコントラストを調べないのは誤り。
2. **Semantics ツリーと同じ規則で、隠れたもの（`Hider`）とブロッカーの下だけを飛ばす。** ブロッカーを探す規則は core の `semanticsBlocker` にある。`gowakutest` に写すと、規則が 2 か所になる。

2 を採り、規則は core に 1 つだけ置く。

### 2. 副ボタンとダブルクリックの名前

1. `RightClick` / `DoubleClick`。gowaku の他の名前（`GestureDetector.OnSecondaryTapAt`、`interact.Viewer.DoubleTapZoom`）と合わない。
2. **`SecondaryTap(label)` / `SecondaryTapAt(pos)` / `DoubleTap(label)` / `DoubleTapAt(pos)`。** `Tap` / `TapAt` と同じ形で、`OnSecondaryTapAt` と同じ言葉。
3. `Tap` にボタンと回数のオプション構造体を渡す。よく使う 3 つの操作に構造体は重い。

2 を採った。

### 3. 警告の色

1. **`Warning` を 1 色足す（`Danger`、`Success` と同じ形）。** 文字として 4.5:1、塗りとしても使い、ライトテーマでは白い文字を載せられる。そのためライトでは深い琥珀（茶に寄る）になる。
2. **塗りの `Warning`（明るい琥珀）と文字の `WarningInk` の 2 色。** 明るい黄色の塗りには黒い文字が要り、`Success` / `Danger` と使い方が変わる。色が 1 つ増える。
3. 足さず、アプリに `TonalPalette` で作ってもらう。今回の指摘そのもの。

1 を採った。

## 決定

### 1. core.Tree.BlockingSemantics と Warnings

`core.Tree` に `BlockingSemantics() RenderObject` を足した。Semantics ツリーが従うブロッカー（開いているダイアログやシートの `RenderModal`）を返し、無ければ nil。`semanticsBlocker` をそのまま呼ぶので、規則は 1 か所のまま。

`Tester.Warnings` は根からそのブロッカーまでの道を作り、道の上の親では、道の次の子より前に塗られる子を飛ばす。`SemanticsBuilder.Visit` と同じ規則で、はみ出し、コントラスト、シェーダーのどれもダイアログの下を調べない。ダイアログ自身と、その上に開いたもの（トーストなど）は調べる。ラベルの無い部品は前から Semantics ツリーを読んでいるので、もともと揃っていた。`ExcludeSemantics` の中は、見えているので今までどおり調べる。

### 2. SecondaryTap / DoubleTap

```go
func (tt *Tester) SecondaryTap(label string)
func (tt *Tester) SecondaryTapAt(pos geom.Offset)
func (tt *Tester) DoubleTap(label string)
func (tt *Tester) DoubleTapAt(pos geom.Offset)
```

ラベルの付いた形は `Tap` と同じく `Find` で見つけたノードの真ん中を押し、どれも押した後で `Settle` する。`DoubleTap` は押す・離すを 2 回送り、木が 1 回目に `Clicks` 1、2 回目に 2 を付ける。本物のマウスと同じく、1 回目は単独のクリックとして届く。

`Tester` はポインタ用のミリ秒の時計を持ち、タップのたびに 2 秒進める。1 回のタップの中では押す・離すを 40ms ずつ進める。これで別々の `Tap` がダブルクリックに数えられることはなくなった。この時計は `Advance` の偽の時計とは別にした。偽の時計は何も動いていなければ進まず、続けた `Tap` が 500ms 以内に収まってしまうからである。

### 3. theme.Colors.Warning

`Colors` に `Warning` を足した。色相 72 の明るい琥珀、彩度 80、トーンはライト 70、ダーク 76。

最初は `Danger` / `Success` と同じトーン（ライト 41）にして、文字として 4.5:1 を守らせた。ところがライトテーマで 4.5:1 に届く琥珀は `#925000` のような濃い色で、アイコンは茶色に、薄めた枠線は黄土色からベージュに見えた。「ベージュにしない」というデザインの方針に反するので、**`Warning` だけはコントラストの約束を破り、明るい琥珀にした**（2026-11-16、オーナーの判断）。

- `Warning` はアイコン、点、枠線、薄い塗りに使う色で、文字には使わない。ライトテーマの面の上では 4.5:1 に届かない。
- 警告の文言は `Ink` で書き、`Warning` はその横のアイコンや枠に使う。`InfoBar` はそうしている。
- `TestColors_InkReadsOnEverySurface` からは `Warning` を外した。ガラスの下の背景としては、`Ink` / `Ink2` が 4.5:1 を保つことを引き続き確かめる。
- 文字用の濃い `WarningInk` を別に足す案もあったが、使い道が少なく、色の役割を増やさないことを選んだ。必要になったら足す。

使う場所:

- `control.InfoBar` の `SeverityWarning` は `th.Color.Warning` を使う。自前の琥珀（`warningHue` などの定数）は消した。
- `collection.CellTone` に `CellToneWarning` を足した（`CellToneSuccess` と `CellToneDanger` の間）。バッジ、点、バーが `Warning` で描かれる。

状態を区別する列挙は、この 2 つのほかに無かった（`Badge` は数を出すだけで色の種類を持たない）。

## 結果

- ダイアログを開いたまま `Warnings` を呼んでも、幕の下のページの警告は出ない。ダイアログが閉じれば、ページは次の呼び出しからまた調べられる。
- コンテキストメニューとダブルクリックのテストが `Tester` だけで書ける。
- 同じ所を続けて `Tap` すると、前はダブルクリックとして届いていた。今は 2 回のシングルクリックになる。リポジトリのテストで、これに頼っていたものは無かった。
- `InfoBar` の警告の色は、ライトでは前より暗い琥珀（`#b97500` から `#925000`）、ダークでは少しオレンジ寄り（`#ffb14a` から `#ff972a`）になった。カタログの画像を描き直した。
- widgets 内のテスト（`contextmenu_test.go` の `secondaryClick` など）は `gowakutest` を使わない専用のリグで書かれているので、置き換えていない。
