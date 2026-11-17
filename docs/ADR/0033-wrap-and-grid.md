# 0033: 折り返す並び（Wrap）と、列の数が幅で決まるタイル（Grid）

決定日: 2026-11-08

状態: 採用

## 背景

音楽プレーヤーの見本アプリでは、アルバムとアーティストの一覧をタイルで並べたかった。

- 窓の幅に応じて、最小の幅を割らない範囲でできるだけ多くの列を取る。
- タイルはジャケットの正方形の下に、曲名の高さを足したもの。

ライブラリに折り返しも格子も無かったので、`CustomLayout` の delegate（`grid{min, gap, extra}`）を自分で書き、二つの画面で使い回した。

- delegate は比較できないので、`CustomLayout` は更新のたびに Layout をやり直す。
- 列の数の式や、最後の行の扱いを、アプリごとに書くことになる。

タグやボタンのように長さの違う部品を、行に詰めて折り返す並び（Flutter の `Wrap`）も無かった。

## 選択肢と決定

### 1. 折り返す並び

- (a) `Row` / `Column` に `Wrap bool` を足す。
  - `Expanded` は折り返しと両立しない。
  - 行の並べ方を決めるフィールド（`RunAlignment`, `RunSpacing`）が `Row` に増えるだけで、ほとんどの `Row` では意味が無い。
- (b) Flutter と同じく、別の部品 `Wrap` にする。

**(b) を採る。** 名前は `Row` / `Column` に揃える。

- `Axis`: 並べる向き。ゼロ値は横で、行を下へ積む。
- `Main`: 行の中で子を並べる向き。`MainAxisAlignment` をそのまま使う。
- `Cross`: 行の中で、子を行の高さに対してどこに置くか。
  - `CrossStretch` は Flutter の `Wrap` には無いが、行の中で一番高い子に揃える意味で受け付ける。チップの高さを揃えるのに使える。
  - 揃えるときは、その子だけ tight な制約で Layout をやり直す。
- `RunAlignment`: 行の束を交差方向のどこに置くか。値は `MainAxisAlignment` を使い回す。
  - Flutter では `WrapAlignment` という別の型だが、中身は同じ六つなので型を増やさない。
- `Spacing`（行の中の間隔）と `RunSpacing`（行と行の間隔）。
- `Size`: `Row` と同じく、ゼロ値の `MainMax` は許された長さを全部取り、`MainMin` は一番長い行に縮む。
  - Flutter の `Wrap` は常に縮むが、ここでは `Row` / `Column` と同じ既定を優先した。そうしないと `MainCenter` や `MainEnd` が、窓ではなく一番長い行に対して効くことになる。

子には、並べる向きでは `Wrap` の長さまで、交差方向には上限なしの制約を渡す（Flutter と同じ）。幅より長い子は幅に縮み、1 つで 1 行になる。

並べる向きの長さに上限が無いとき（横の `Wrap` を `Row` の中に置いたときなど）は、折り返す位置が無い。

- panic は**しない**。1 行に並べ、`Size` に関わらず中身の長さに縮む。
- Flutter と同じ挙動で、意味の通る結果が出る。`Scroll` の「上限の無い長さを自分の大きさにする」問題（ADR 0026）とは違い、画面の外に消えることもない。

### 2. 格子

- (a) Flutter の `GridView` のように、スクロールと遅延生成を一体にする。
  - 遅延生成には、高さの決まった行の仕組み（`ListView`）を一般化する必要があり、範囲が広い。
  - 別の作業で `ListView` の行の高さを変えているところでもある。
- (b) 子をすべて並べるだけの `Grid` にし、スクロールは外の `Scroll` に任せる。
  - Flutter の `GridView(shrinkWrap: true)` に近い。
  - アルバムが数百枚までなら十分に速い。ベンチマークでは 1000 枚の Layout が 0.07〜0.14 ms、アロケーション 0。

**(b) を採る。** 遅延生成の格子は、行の高さを変えられる `ListView` が入ったあとで考える。

列の数は次のように決める。

- `Columns > 0` なら、その数に固定する。
- そうでなければ、`MinColumnWidth` を割らない範囲で最大の数にする。式は `max(1, floor((幅 + Spacing) / (MinColumnWidth + Spacing)))`。
- 両方ゼロ（ゼロ値）なら 1 列。

列の幅は、残りを等分した値になる。

- Flutter の `maxCrossAxisExtent`（列の最大幅）ではなく、最小幅で指定させる。
- 見本のアプリが必要としたのは「これより細いと読めない」という下限のほうだった。
- 下限なら、幅が足りないときに 1 列へ落ちる挙動も自然になる。

タイルの高さは、次の順に決める。

1. `TileHeight > 0` なら、その値に固定。
2. `AspectRatio > 0` なら、`列の幅 / AspectRatio + ExtraHeight`。
   - 「正方形のジャケットと曲名」は `AspectRatio: 1, ExtraHeight: 40` と書ける。
   - 見本のアプリの delegate は、`Grid{MinColumnWidth: m, Spacing: g, RunSpacing: g, AspectRatio: 1, ExtraHeight: e}` で置き換えられる。
3. どちらも無ければ、行の高さはその行で一番高い子になり、ほかの子もその高さに伸ばす（CSS grid の既定の `stretch` と同じ）。
   - ゼロ値でも、文字のカードのような高さの違う子が揃って並ぶ。

縦の `Grid`（行が下へ伸びる）だけを用意する。横に伸びる格子は、今のところ使う場面が無い。

幅に上限が無いとき（`Row` の中や横の `Scroll` の中）は、列の幅が決まらない。

- `"gowaku: Grid has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded"` で panic する。
- ADR 0026 の文面の決まり（部品の名前、決まらない長さとよくある原因、直し方）に従う。
- 高さには上限が無くてよい。縦の `Scroll` の中では、`Grid` は行の分だけの高さになる。

### 3. アロケーションしない Layout

- `Wrap` は、行ごとの長さを配列に持たない。
  - 子をすべて Layout したあと、子の大きさから行の切れ目を求める関数 `run` を、寸法を測るときと並べるときの 2 回呼ぶ。
  - 子の大きさが同じなら、切れ目も同じになる。
  - 浮動小数の足し算の誤差で、ちょうど収まる子が折り返さないよう、1/1024 の許容を置く。
- `Grid` は、行ごとに子のスライスを切り出すだけで済む。
- ベンチマーク（`BenchmarkLayout_Wrap1k`, `BenchmarkLayout_Grid1kAspect`, `BenchmarkLayout_Grid1kNatural`）で allocs/op = 0 を確かめた。

## 影響

- `widgets.Wrap` / `widgets.Grid` と、その RenderObject の `layout.RenderWrap` / `layout.RenderGrid` が増える。
  - ルートの `gowaku` から `Wrap`, `Grid` を使える。
  - 縦の `Wrap` のために、`Axis`, `Horizontal`, `Vertical` も出す。
- `layout.RenderGrid.ColumnsFor(width)` を公開する。テストと、同じ列の数を使いたい `CustomLayout` の delegate のため。
- 既存の部品の挙動は変わらない。
