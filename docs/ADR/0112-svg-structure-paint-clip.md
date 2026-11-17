# 0112: SVG の構造、塗り、線、クリップとマスク

決定日: 2026-11-14

状態: 一部を ADR 0114 で置き換え

## 背景

ADR 0045 §1 の `paint.ParseSVG` は、アイコンセットに要る範囲だけを読み、text、グラデーション、フィルター、use、clip-path、mask、stroke-dasharray、marker を飛ばして `Skipped` に数えていた。利用者から「gowaku の SVG は、文字・グラデーション・フィルターなどの要素を描きません」という報告があった。アイコンだけでなく、ほぼすべての静的な SVG を描けるようにする。段 1〜4（構造、塗り、線とマーカー、クリップとマスク）をこの ADR に、段 5〜6（フィルターと文字）を [ADR 0113](0113-svg-filters-text.md) に書く。

守ること：アイコンの速い道（アトラス、描くたびのアロケーション 0）は変えない。壊れた入力で panic しない。

## 決定

### 1. 構造（段 1）

- **木を先に読む。** 今までは XML のトークンを流しながら描いていたが、`use` や `url(#…)` は後ろを指せ、`style` 要素は文書のどこにあっても全体に効く。そこで、まず軽い DOM（`svgNode`：名前、属性、子、親、文字）を作り、id の表を引けるようにしてから、根から歩いて描く（`svgxml.go`、`svgdoc.go`）。
- **`style` 要素は CSS の小さな部分集合：** 型（`rect`）、クラス（`.a`）、id（`#x`）、全称（`*`）、その組み合わせ（`rect.a.b#x`）、セレクターの並び（`a, b`）、子孫結合子と子結合子（`g .a`、`g > .a`）。属性セレクター、疑似クラス、兄弟結合子を含むセレクターは何にも当たらない。`@media` などの at-rule はブロックごと読み飛ばす。コメントは消す。`type` が無いか `text/css` のものだけを読む（`svgcss.go`）。
- **カスケードの順：** 表示属性 → 規則（詳細度、同じなら後のもの）→ `style` 属性 → `!important` の規則。結果は各要素の `decls` に、効く順で並べる。`class` 属性で当てる。
- **`use`：** `href` と `xlink:href`、`x` / `y`（`transform` の後に平行移動）、要素、`g`、入れ子の `svg`、`symbol` を指せる。`symbol` と `svg` は `use` の `width` / `height`（無ければ symbol 自身のもの、それも無ければ 100%）をビューポートにし、`viewBox` と `preserveAspectRatio` を当てる。中身は `use` の状態を継ぐ（ブラウザの影の木と同じ）。CSS の規則は元の木で当てる。
- **`defs`、`symbol`、グラデーション、`clipPath`、`mask`、`marker`、`pattern`、`filter`、`style`** はその場では描かない（数えない）。`a` は `g` として描く。`switch` は最初の子要素だけを描く（条件は評価しない）。
- **壊れた入力：** 参照のループ（`use`、`clipPath`、`mask`、`marker`、グラデーションの `href`）は、描いている最中の要素の列（`active`）で見つけて `Skipped` に数える。`use` の指数的な展開（各段が前の段を 10 回使う、など）は、訪ねる要素の数の予算（`svgVisitBudget` = 2^17）で止め、超えた分を一度だけ数える。入れ子が 256 段より深い要素は読まずに数える（再帰の深さの上限）。CSS は規則を 2048 個まで、1 回のセレクターの照合は 4096 歩までにする。NaN と無限大の数はエラーにする（後の計算を全部壊すため）。`FuzzParseSVG` が、読む、描く、`SetText` してもう一度描くを試す（このマシンで 90 秒、エラー 0）。

### 2. 塗り（段 2）

- **`linearGradient` と `radialGradient`** を `fill` と `stroke` に使える。`gradientUnits`（objectBoundingBox / userSpaceOnUse）、`gradientTransform`、`spreadMethod`（pad / reflect / repeat）、`href` で属性と stop を継ぐ（属性は同じ種類のグラデーションからだけ、stop はどちらからも）、`fx` / `fy`、`stop-color`、`stop-opacity`、`offset`（数か %、0〜1 に詰め、減らないように）。
- **paint のグラデーションに写す：** `Gradient.Transform` に「グラデーションの空間 → 図の座標」（ctm × 外接矩形の行列 × gradientTransform）を入れる。線の外接矩形は形の幾何（線を含まない）。外接矩形の幅か高さが 0 で objectBoundingBox なら描かない（ブラウザと同じ）。stop は 4 個までも `MoreStops` に入れる（render が 4 個以下ならインスタンスに入れる）。
- **paint を広げた：** `Gradient.Spread`（`SpreadPad` がゼロ値、`SpreadReflect`、`SpreadRepeat`）と、焦点のある放射の `GradientFocal`（`Start` が円の中心、`End` が円の上の点、`Focus` が offset 0 の点）。
  - GPU は `GradientMode` にビットを足した：`GradientFocal`（形の 4 番目）、`GradientReflect`（1<<5）、`GradientRepeat`（2<<5）。`Shape()` は下位 4 ビットになった。offset を求めたあと、`sdf.ps.hlsl`、`sdf.metal`、soft の `fill` で同じ式（`gpu.SpreadOffset`、`gpu.FocalOffset`）を通す。
  - 焦点の位置は 2 つの float が要るが、インスタンスに空きが無い。焦点のあるグラデーションは必ずランプ（`GradientRamp`）で描き、使っていない `StopOffsets[1]`、`[2]` に「中心から焦点まで（グラデーションの空間）」を入れる。`Instance` の大きさは変わらない。円の上か外の焦点は半径の 0.99 倍に寄せる（SVG 1.1 の扱いに近い）。Resources の無い描画（ランプが無い）では中心からの放射に落とす。
  - `Op` は `Gradient.Focus` の分だけ 8 バイト大きくなった（`Spread` は詰め物に入る）。
  - D3D11 と Vulkan の `sdf.ps` を作り直した（`go generate`、頂点シェーダーとパスのシェーダーはバイトまで同じだった）。パリティのテストに `spread` の場面（3 つの spread、焦点、焦点 + reflect + 歪み）を足した。D3D11 と Vulkan の差は最大 1、soft との差も最大 1。Metal は手書きの `sdf.metal` に同じ式を足したが、このマシンでは試せない。
- **アルファの補間：** ブラウザは SVG のグラデーションの色を、アルファを掛ける前に補間する。paint は premultiplied で補間する。アルファと色の両方が変わる区間に、まっすぐ補間した stop を 7 つ足して近づける（`straightStops`）。
- **`pattern` は描かない。** 参照を `Skipped` に数え、後ろの代わりの色（`url(#p) red`）があればそれで塗る。stop の `currentColor` は、stop と祖先、塗る要素の `color` から取り、無ければ黒（`Draw` の色は使わない）。

### 3. 線とマーカー（段 3）

- **`stroke-dasharray`、`stroke-dashoffset`、`pathLength`：** 輪郭を作る前にパスを破線に切る（`dash`）。曲線は線の許容誤差で平らにし、各破線は開いた部分パスとして `Path.Stroke` に渡る（破線ごとに cap が付く）。閉じた輪郭で、最後の破線が始点をまたいで最初の破線につながるときは、一本につなぐ（cap ではなく join になる）。奇数個は 2 回繰り返す。負の値やすべて 0 は実線。開いた輪郭のちょうど終点で始まる長さ 0 の破線は描かない（Chrome と同じ。比べて確かめた）。破線の数は 2^16 までで、超えたら実線。
- **`marker-start` / `marker-mid` / `marker-end`、`marker`（一括）：** path、line、polyline、polygon の頂点に置く（Chrome と同じく、rect や circle には置かない）。頂点はパスの命令の端点で、`d` をもう一度読んで集める（円弧は 3 次曲線に分けて描くが、頂点は円弧の端だけ）。向きは `orient` の `auto`（中の頂点は入る向きと出る向きの二等分、開いた部分パスの始点は出る向き、終点は入る向き、閉じた部分パスの端は二等分）、`auto-start-reverse`、角度（deg / rad / grad / turn）。`markerUnits`（strokeWidth が既定）、`refX` / `refY`（left / center / right などの語も）、`viewBox`、`preserveAspectRatio`、`markerWidth` / `markerHeight`。マーカーの中身は、マーカーが定義された場所の祖先から継ぐ（参照する形からは継がない）。
- **マーカーのビューポートで切る：** 既定の `overflow: hidden` どおり切る。ただし中身がビューポートに収まっているとき（矢印などほとんど）は切らない（段 4 のグループを作ったあと、収まっていれば効果の無いグループとして畳む）。
- **`paint-order`** も読む（fill / stroke / markers の順）。

### 4. 描く形の作り直し、クリップ、マスク（段 4）

- **描く形：** `SVG` は形（`svgShape`：図の座標のパス、色かグラデーション）のほかに、操作の列 `ops`（形を描く、グループを開く、閉じる）と、グループの表（不透明度、クリップ、マスク、フィルター）を持つ。マスクの中身は自分の操作の列を持つ。
- **グループはブラウザが合成するところだけに作る。** 読むときは、不透明度が 1 未満、`clip-path`、`mask`、`filter` のある要素ごとにグループを開く。読み終えたら畳む（`fold`）：効果の無いグループは中身に置き換え、効果が不透明度だけで中身が形 1 つなら、その色（グラデーションなら stop）に掛けて消す。何も見せないクリップ（空の `clipPath`）や不透明度 0 は中身ごと消す。こうして、ADR 0045 が「形ごとに掛ける」で近似していたグループの不透明度は、ブラウザと同じくまとめて合成するようになり、アイコンはこれまでどおりグループ無しのまま。`opacity` は継がない値（要素自身の値）になった。
- **グループの無い図（すべてのアイコン）は、これまでの道をそのまま通る。** 形ごとに `DrawPath`、大きさごとの写し（4 つ）、アトラス。グラデーションも写しと一緒に大きさごとに変換して持つので、同じ大きさならアロケーション 0（テストあり）。全アイコンの比較（下）で変わっていないことを確かめた。
- **（追記 2026-11-14：[ADR 0114](0114-gpu-layers-masks-filters.md) で、CPU の合成を GPU のキャンバスのグループに替えた。下の CPU の合成と大きさごとの絵、`DrawScaled` は無くなった。）**
- **グループのある図は CPU で合成する。** 入れ子ごとに float32 の premultiplied RGBA の層を持ち、形は走査線ごとに正確な被覆（1 ピクセルの行に 16 本、non-zero と even-odd）で塗り、グラデーションはピクセルごとに GPU と同じ式で求める（`svgraster.go`）。グループを閉じるとき、フィルター → クリップ → マスク → 不透明度の順（SVG の順）に掛けて、下の層に重ねる。結果は `paint.Image` になり、`DrawImageQuality`（Linear）で描く。
  - なぜ GPU の層（ADR 0019、`DrawLayerBlended` / `DrawLayerFiltered`）を使わないか：層は入れ子にできない（層の中の層は溶ける）、`RepaintBoundary` の中では `CanBlendLayer` が偽になる（多くのウィジェットはその中にある）、ステンシルのパスやブラーを含む層はインラインに落ちて、まとめての合成が失われる、輝度のマスクやフィルターのグラフ（中間の結果、名前付きの入力、色空間）を表す道具が無い。どれかが欠けると絵が変わるので、まず CPU で正しく描くことにした。速くする余地は「決めてほしいこと」に書く。
  - **大きさごとの絵：** `SVG.DrawScaled(cv, dst, current, scale)` を足した（`Draw` は scale 1）。合成した絵は（物理ピクセルの幅と高さ、拡大率、位置、currentColor）ごとに最近の 4 つを持つので、同じ大きさと色ならアロケーション 0。大きさが変わる最初の 1 回だけ、合成と画像のアップロードが起きる。絵は 2^21 ピクセルまでで、それより大きいと解像度を落として引き伸ばす。層の入れ子は 24 段まで（それより深いグループはそのまま下の層に描く）。
  - SVG ウィジェットは `MediaQueryOf(ctx).Scale` を `layout.RenderSVG.Scale` に渡す。`Canvas.Scale` での拡大（遷移）には付いていかない（絵が引き伸ばされる）。
- **`clip-path`：** `clipPathUnits`、`clipPath` 自身の `transform`、子ごとの `clip-rule`、子の `use`（形を直接指すもの）と text、`clipPath` に付いた `clip-path`（入れ子、交わり）。被覆は各パスの和を 1 − Π(1 − 被覆) で取る（接する 2 つのパスの継ぎ目が透けない）。存在しない参照は切らない（Chrome と同じ）。CSS の基本図形（`circle()` など）は読まず数える。クリップパスの子に付いた `clip-path` は無視して数える。入れ子の `svg` のビューポートでは今までどおり切らない。
- **`mask`：** `mask-type`（luminance が既定、alpha）、`maskUnits`（既定の領域は外接矩形の −10%、120%）、`maskContentUnits`、マスクの中身にもグループや効果が効く。輝度は premultiplied の sRGB の色に SVG の係数（0.2125、0.7154、0.0721）を掛けたもの（Chrome と比べて合った）。
- **外接矩形：** クリップやマスク、フィルターが objectBoundingBox で参照する外接矩形は、グループの中で描いた形の幾何（線とマーカーを除く）を、グループを開いた座標に戻して求める（形ごとに元のパスと ctm を覚えておく）。

### 5. Chrome との比較

- `internal/golden/svgfeatures_test.go`（`//go:build svgsets`）が、`internal/golden/testdata/svg/features/` の手書きの SVG（機能ごとに 1 つ以上、13 個）を headless Chrome と soft で 200 px に描き、色を比べる。合格は「全ピクセルの全チャンネルの差の平均が 0.01 未満」かつ「どこかのチャンネルが 1/4 以上違うピクセルが 0.5% 未満」。文字は 0.03 と 4%（[ADR 0113](0113-svg-filters-text.md)）。
- 結果（2026-11-14、Chrome 安定版、このマシン）：13 個すべて合格。段 1〜4 の分は、平均の差 0.0006〜0.0020、1/4 以上違うピクセル 0〜0.16%（縁のアンチエイリアス）。

| ファイル | 平均の差 | 1/4 以上違うピクセル |
|---|---|---|
| struct-use-symbol | 0.0010 | 0.01% |
| grad-linear | 0.0006 | 0% |
| grad-spread | 0.0011 | 0.08% |
| grad-radial | 0.0018 | 0.15% |
| grad-stroke | 0.0014 | 0.12% |
| stroke-dash | 0.0020 | 0.16% |
| stroke-markers | 0.0019 | 0.11% |
| clip-paths | 0.0006 | 0% |
| mask-opacity | 0.0007 | 0% |

- 全アイコン（Lucide 1.70.0 と Material Symbols、25962 個）の比較も、変える前と後で回した。結果は下の「結果」。

## 結果

- golden：`svg-features`（1200×765、倍率 1.5）。features の 13 個を 150 px で並べる（GPU の道と CPU の合成の両方を含む）。soft、D3D11、Vulkan で通る（D3D11 と Vulkan はどの場面も最大 2、2 を超える画素 0）。`svg-icons` は soft でずれ 0。既存の golden は作り直していない（soft で 1〜2 ずれる場面と `controls-dark` の既知のずれは、SVG を含まない場面）。
- 全アイコン：変える前 25962 個すべて合格（差の中央値 0.0110、p90 0.0287、p99 0.0401、最大 0.1137）。変えた後（段 1〜6 のすべて）も 25962 個すべて合格で、`Skipped` 0、エラー 0、差の中央値・p90・p99・最大と、差の大きい順の上位 40 個の並びと値は、変える前と同じだった（アイコンはどれもグループを持たず、今までと同じ道を通る）。1 回りは約 8.5 分。
- 残した穴：`pattern`、外部の文書への参照、CSS の基本図形、属性セレクターなど CSS の多く、クリップパスの子の `clip-path`、入れ子の `svg` のビューポートのクリップ、`fr`（焦点の半径）。合成の絵は `Canvas.Scale` に付いていかず、位置が物理ピクセルの端数にあると少しぼける。
