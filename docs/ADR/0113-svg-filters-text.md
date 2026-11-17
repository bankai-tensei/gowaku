# 0113: SVG のフィルターと文字

決定日: 2026-11-14

状態: 一部を ADR 0114 で置き換え

## 背景

[ADR 0112](0112-svg-structure-paint-clip.md) の続き（段 5〜6）。描く形をグループのある操作の列にし、グループのある図を CPU で合成するようにしたので、フィルターはその合成の一段として、文字は形として載せる。

## 決定

### 1. フィルター（段 5）

- **（追記 2026-11-14：[ADR 0114](0114-gpu-layers-masks-filters.md) で、フィルターは GPU のパスになった。読むものと意味はこのままで、CPU の実行は無くなった。）**
- **フィルターはグループの効果。** `filter` のある要素はグループになり、合成のとき、層にフィルターを掛けてからクリップ、マスク、不透明度を掛ける（SVG の順）。GPU のブラーとブレンド（ADR 0044）は使わない。理由は ADR 0112 §4 と同じで、フィルターのグラフは中間の結果と名前付きの入力を持ち、層は入れ子にできないため。
- **読むもの：** `filterUnits`（既定 objectBoundingBox、領域の既定 −10%、120%）、`primitiveUnits`（objectBoundingBox なら長さを外接矩形の幅と高さで掛ける）、`x` / `y` / `width` / `height`、プリミティブの部分領域、`in` / `in2` / `result`（`SourceGraphic`、`SourceAlpha`、名前。名前が無い・知らない名前は前の結果、最初は SourceGraphic）、`color-interpolation-filters`（既定 linearRGB、sRGB）、フィルターの `href`（プリミティブを持つものまでたどる）。
- **プリミティブ：**
  - `feGaussianBlur`：標準偏差 2 未満は正確なガウス、2 以上は仕様が勧める 3 回の箱のブラー（偶数の幅は左右に寄せた 2 回と幅 + 1 の 1 回）。Chrome と比べて合った。
  - `feOffset`（端数は双一次）、`feDropShadow`（入力のアルファのブラー、ずらし、`flood-color` と `flood-opacity`、入力を上に重ねる）、`feFlood`。
  - `feColorMatrix`：`matrix`、`saturate`、`hueRotate`、`luminanceToAlpha`。premultiplied を外して掛ける。
  - `feComposite`：over、in、out、atop、xor、arithmetic（k1〜k4、色はアルファ以下に詰める）、lighter。
  - `feMerge`、`feBlend`（normal と、分離可能・不可能なすべてのモード）。
  - 安いので足した：`feComponentTransfer`（identity、table、discrete、linear、gamma）、`feMorphology`（erode / dilate。半径は物理ピクセルで 64 まで）。
- **描けないプリミティブ**（`feTurbulence`、ライティング、`feDisplacementMap`、`feImage`、`feConvolveMatrix`、`feTile`）を含むフィルターは、要素をフィルター無しで描き、`Skipped` に数える。存在しない参照はフィルター無し、子の無い `filter` は何も描かない（仕様どおり）。CSS のフィルター関数（`blur()` など）は読まず数える。`BackgroundImage` などの入力は透明。
- **座標：** フィルターは合成の絵のピクセルで走る。領域は（図からピクセルへの行列 × 要素の ctm）で写した外接矩形。標準偏差とずらしは、その行列の x 軸と y 軸の長さで掛ける（回転や歪みの下では近似）。図のビューポートの外にあるものはブラーで入ってこない。
- **色空間：** 結果ごとに linearRGB か sRGB かを持ち、使う側の色空間に変換して読む。最後に sRGB に戻す。

### 2. 文字（段 6）

- **層の分け方：** `paint` はフォントを知らず、`text` は `paint` を import する。そこで paint にインターフェースを置き、呼ぶ側が渡す：

  ```go
  type TextOutliner interface {
      OutlineText(content string, font SVGFont, glyph func(outline *Path)) TextMetrics
  }
  ```

  `*text.Shaper` がこれを実装する（`text/svgoutline.go`）。1 行に組み、グリフごとの輪郭（ベースラインが原点、y は下向き、non-zero）を渡し、送り幅（末尾の空白と字間を含む）とアセント、ディセントを返す。輪郭の無いグリフ（カラー絵文字など）は描かない。合成の斜体はアトラスと同じ傾き。
- **`ParseSVG` は文字を飛ばして数える。** `SVG.SetText(o)` が、覚えておいた文書を `o` と一緒に読み直す。同じ `o` なら何もしない（だから `o` は比べられる値、たとえばポインター）。`SVG.HasText()` で文字があるかを聞ける。SVG ウィジェットは `Build` で、上に `text.Shaper` があれば `SetText(shaper)` を呼ぶ。読み直しなので、描くときには何もしない（Paint はアロケーション 0 のまま）。文字の数や属性の誤りは、`ParseSVG` の時点でエラーになるよう、輪郭が無くても文字は一度読む。
- **文字は形になる。** グリフを 1 つずつ形にするので、GPU の道ではグリフごとにアトラスを通り（大きな 1 本のパスにしてステンシルの MSAA に落とさない）、塗りと線、グラデーション（外接矩形は文字の要素全体）、破線、クリップパスの中の文字、マスク、フィルターがそのまま効く。
- **読むもの：** `text`、`tspan`、`a`。`x` / `y` / `dx` / `dy` は文字ごとのリスト（内側の要素の値が勝つ）。位置を持つ文字から新しい片になり、`x` か `y` を持つ片から新しいチャンクになる。`text-anchor` はチャンクごと。空白は `xml:space="default"` の扱い（改行を消し、タブを空白に、続く空白を 1 つに、先頭と末尾を落とす）。`font-family`（最初の名前、引用符を外す。総称はそのまま渡す）、`font-size`（長さ、%、em、キーワード）、`font-weight`（数、bold、bolder、lighter）、`font-style`、`letter-spacing`、`dominant-baseline` / `alignment-baseline` の基本（middle は x ハイトの半分を 0.26 em とみなす、central、hanging、text-before-edge / text-top、text-after-edge / text-bottom / ideographic、mathematical）。
- **読まないもの：** `textPath`（数える）、`rotate`、`textLength` / `lengthAdjust`、`text-decoration`、縦書き、`font` の一括指定、tspan の不透明度などの効果。リストで位置を持つ文字は 1 文字ずつ組むので、そこではカーニングが効かない（ブラウザもおおむね同じ）。`font-family` が無いときは gowaku の既定（日本語のフォント）で、ブラウザの既定（serif）とは違う。

### 3. Chrome との比較

ADR 0112 §5 と同じテスト。段 5〜6 の分（2026-11-14）：

| ファイル | 平均の差 | 1/4 以上違うピクセル |
|---|---|---|
| filter-blur-shadow | 0.0017 | 0% |
| filter-color | 0.0011 | 0.02% |
| text-basic | 0.0120 | 3.88% |
| text-effects | 0.0030 | 0.39% |

文字の差は、位置と送り幅は合っていて（並べて確かめた）、残りはグリフのラスタライズの違い（Chrome は Windows でヒンティングとガンマを掛ける）。文字の合格の閾値はこのため緩くした（平均 0.03、4%）。

## 結果

- golden `svg-features` に、フィルター 2 つと文字 2 つが入る。golden の Renderer は Go のフォントだけを持つので、文字はそのフォントで組まれる（太字は無い）。
- ギャラリーに「SVG の描画」のページ（`-state svg`）を足した。グラデーション、破線とマーカー、クリップとマスク、フィルター、文字。
- 費用：合成は大きさと色が変わったときだけ走る。`BenchmarkSVG_Composite`（200 px、グループ、ブラー、マスク）を置いたが、並行して別の作業が走っていたので測っていない（メインのセッションで測る）。フィルターのある大きな図をアニメーションで拡大縮小すると、毎フレーム合成し直すので重い。
