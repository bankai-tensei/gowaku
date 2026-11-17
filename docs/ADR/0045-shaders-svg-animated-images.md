# 0045: 描画拡張（13b: SVG、ミップマップ、動く画像、カスタムシェーダー）

決定日: 2026-11-09

状態: 一部を ADR 0112、ADR 0113 で置き換え

## 背景

描画の拡張（Phase 13）のうち、後半の 13b。前半（Affine、グラデーション、ブレンドモード、ブラー）は [ADR 0044](0044-drawing-extensions.md)。完了条件は当初の計画の次の 2 つ。

- 一般的なアイコンセットの SVG がそのまま出る。
- カスタムシェーダーの例が、Windows と Linux で同じ絵になる。

ヒアリングで次を決めた。

- **カスタムシェーダーは HLSL 1 本から全部作る。** `gowaku-shader` が、DXBC（fxc）、SPIR-V（dxc）、MSL（spirv-cross）を作る。当初の計画では「HLSL・GLSL・MSL を利用者が 3 本書く」としていたが、これを変える。spirv-cross はビルドのときに使う道具なので、実行時の依存は増えない（当初の計画が避けたかったのは実行時の C++ 依存だった）。
- **シェーダーへの入力：** 決まった値（大きさ、時刻、スケール）に、float を 16 個（float4 を 4 本）とテクスチャを 2 枚まで足す。テクスチャには、子の描画か画像を渡す。
- **SVG の確かめ方：** Lucide（ISC）と Material Symbols（Apache 2.0）を全部手元に落とし、すべてのアイコンを読んで描けることを確かめる。golden に使う数十個だけを、ライセンスの表記と一緒に testdata に入れる。
- **動く画像：** 当初の計画どおり、GIF、APNG、アニメーション WebP の 3 つ。

## 決定

### 1. SVG

- `paint.ParseSVG(data []byte) (*SVG, error)`。読める要素は当初の計画の範囲に合わせる：`svg`、`g`、`path`、`rect`（rx / ry を含む）、`circle`、`ellipse`、`line`、`polyline`、`polygon`。
- 読める属性：
  - 塗りと線：`fill`、`stroke`、`stroke-width`、`stroke-linecap`、`stroke-linejoin`、`stroke-miterlimit`、`fill-rule`
  - 不透明度：`opacity`、`fill-opacity`、`stroke-opacity`
  - 配置：`transform`（matrix / translate / scale / rotate / skewX / skewY）、`viewBox`、`width` / `height`
  - `style` 属性に書かれた同じプロパティ
  - 継承と `currentColor`
- **読めない要素は飛ばし、数を `SVG.Skipped` に残す。** text、filter、mask、gradient、use、アニメーションがそれに当たる。（2026-11-14：text、filter、mask、gradient、use などは描くようになった。今の範囲は [ADR 0112](0112-svg-structure-paint-clip.md) と [ADR 0113](0113-svg-filters-text.md)。）エラーにすると、アイコン 1 つのために画面全体が出なくなるため。壊れた XML や数値はエラーにする。
- **線は CPU で輪郭のパスに変える。** `Path.Stroke(StrokeStyle) *Path` を足す。端の形は butt / round / square、角の形は miter / round / bevel。結果は non-zero で塗るパスになる。SVG を読んだときに一度だけ作り、描くたびには作らない。こうすれば、Phase 5 のパスの道（小さいものは CPU でアトラスに、大きいものはステンシル）を、そのまま使える。
- **ウィジェット：** `widgets.SVG{Data, Size, Color}`。`Color` が `currentColor` になり、ゼロ値のときはテーマの Ink を使う。`Icon` は今のまま残す。
- **Chrome と見比べる：** 全アイコンを Chrome（headless）と gowaku の soft で同じ大きさに描き、アイコンごとの差が閾値以内かを見るテストを置く（ローカルのみ、`//go:build svgsets`）。「そのまま出る」の判定をこのテストに任せる。

段 1 を実装したときに決めた細部（2026-11-09）：

- **線の輪郭は「部品の和」にする：** 線分ごとの四角形、外側の角だけを埋める join（miter は四角形、bevel は三角形、round は扇形）、cap（square は四角形、round は半円の扇形）を、すべて同じ向きに巻く別々の輪郭として出す。non-zero で塗れば和になる。左右のオフセットを 1 本の輪郭につなぐ作り方（Skia 型）は、曲率半径が線の半分の幅より小さいところで内側が裏返り、閉じた輪郭の穴と打ち消し合って、穴が開く（Lucide の `r=".5"` に幅 2 の円がそうなる。ブラウザでは塗りつぶしの丸）。部品の和ならこの心配が無い。SVG 2 の線の形の定義（線分の形、join、cap の和）とも同じ。
  - 部品どうしは、隣と共有する辺の点をビットまで同じにする（線分の法線を一度だけ計算して使い回す）。覆いのアトラスでも、共有する辺で被覆が重ならない。
  - 曲線は平らにしてから線分として扱う。曲線の途中の点は、`stroke-linejoin` に関わらず round で継ぐ（曲線の線はそこで丸い）。曲がりが小さくて、弦と弧の差が許容誤差の半分以下なら、三角形で済ませる。round の join と cap は 3 次曲線のまま出すので、塗るときの平坦化が描く大きさで効く。
  - 長さ 0 の部分パス（`M x y Z`、`M x y L x y`）は、round なら円、square なら軸に沿った正方形、butt なら何も描かない（SVG と同じ）。`M` だけのものは描かない。180° の折り返しは、round なら前に半円、miter と bevel は何も足さない。
  - `StrokeStyle` のゼロ値は SVG の既定（butt、miter、miter limit 4）。`Tolerance` で曲線の平坦化の誤差を選べる（ゼロは塗りと同じ 0.1）。
- **平坦化の許容誤差：** 輪郭は読むときに一度だけ作るので、描く大きさが分からない。view box の長辺を 256 論理 px に描いたときに 0.1 px（512 px までで 0.25 px 以内）になるように、`0.1 × 長辺 / 256` を許容誤差にする。`transform` で拡大されている要素は、その最大の拡大率で割る。round の join と cap は曲線のまま残るので、この制限を受けない。
- **`transform` は読むときに焼き込む：** 線は要素の座標で輪郭にしてから、変換を点（制御点も）に掛ける。拡大や歪みのある線も SVG のとおり変形する。`SVG` が持つのは view box の座標の塗りのパスだけになる。
- **`Draw` は大きさごとに写しを作る：** `Canvas.Scale` で拡大すると `Zoom != 1` になり、アトラスの道（64 物理 px 以下）を通らない。`Icon` と同じく、描く大きさに拡大したパスの写しを作って `Translate` だけで置く。写しは最近使った 4 つの大きさの分を `SVG` に持つ（同じ SVG を 16 px と 24 px で並べても作り直さない）。アトラスはパスのポインターで引くので、写しは使い回さずに新しく作る。同じ大きさなら、描くたびのアロケーションは 0。
- **ビューポートで切る：** ブラウザは root の svg の外を切る。Material Symbols には view box の外へ出るパスがある（`sharp/handshake` など）。読むときに全部の形の外接矩形を持っておき、`dst` からはみ出すときだけ `ClipRect(dst)` する（クリップはシザーでも 1 つ費用がかかる）。`slice` も同じ道で切れる。
- **`preserveAspectRatio`：** 既定の xMidYMid meet だけでなく、9 つの位置、`none`、`slice` も読む（少しの式で済む）。入れ子の `svg` は、x / y / width / height / viewBox を変換として扱い、切らない。
- **`Skipped` に数えるもの：**（2026-11-14 に変わった。今の一覧は [ADR 0112](0112-svg-structure-paint-clip.md) と [ADR 0113](0113-svg-filters-text.md)。以下は段 1 のときのもの）描けない要素（その子は数えない）、塗りの `url(#…)`（後ろに色があればそれで塗る）、`stroke-dasharray`、`clip-path`、`mask`、`filter`、`marker`。描かないのが正しい `title`、`desc`、`metadata`、ほかの名前空間の要素（エディターのメタデータ）は数えない。`display="none"` は子ごと描かない。
- **色：** CSS の名前の色（148 個、定数の文字列から引く）、`#rgb` / `#rgba` / `#rrggbb` / `#rrggbbaa`、`rgb()` / `rgba()`、`transparent`。分からない色は CSS と同じく無視して、継いだ値のまま。`color` プロパティがあれば `currentColor` はそれ、無ければ `Draw` に渡す色。
- **グループの不透明度は形ごとに掛ける：**（2026-11-14 に変わり、ブラウザと同じくまとめて合成する。[ADR 0112](0112-svg-structure-paint-clip.md) §4） `opacity` を子の形の色に掛ける。重なる子を持つ `g` の `opacity`、塗りと線のある形の `opacity` は、ブラウザ（まとめて合成）と重なりの部分だけ違う。両方のアイコンセットには、どちらも無い。
- **ウィジェット：** `widgets.SVG{Data, Size, Color, Label}`。`Size` のゼロ値は `Icon` と同じ 24。`Label` があれば image、無ければ飾り（`Icon` と同じ）。ルートには `SVG`（ウィジェット）、`SVGDrawing`（`paint.SVG`）、`ParseSVG`、`StrokeStyle` を置いた。
- **アトラスのラスタライザーを直した：** `x/image/vector` は符号つきの面積を足して切り詰めるので、同じ向きの輪郭が重なる辺（Material Symbols の `hdr_off_select` は文字を 2 回描く）、逆向きの輪郭が重なるところで、縁が太くなったり欠けたりする。部品の和にした線の輪郭も重なる。そこで、パスのアトラスに入れる前に、平坦化した多角形を 1 px あたり 16 本の走査線で調べ、巻き数が 2 以上か、正と負の両方があるときだけ、正確な non-zero のラスタライザー（`text/nonzero.go`、走査線の上では被覆が正確）で描く。重ならないパスは今までどおり `x/image/vector` を通るので、今までの golden はビットまで同じ。
- **全アイコンの結果（`svgsets_test.go`）：** Lucide 1.70.0 の 2322 個と Material Symbols（svg-400 0.48.3）の outlined / rounded / sharp の 23640 個、計 25962 個を、48 px で Chrome と比べる。どれもエラー 0、`Skipped` 0。合格は「インクに対する被覆の差の合計が 0.15 未満」かつ「被覆が 0.5 以上違うピクセルが 4 個以下」。アンチエイリアスの違いは縁に薄く広がり、欠けや余計な部品はほぼ 1.0 違うピクセルの塊になるので、後者が本当の判定になる。前者は細い線ばかりのアイコンで大きくなる（`fingerprint` が 0.114、見比べてアンチエイリアスだけの違い）ので、0.15 にした。
  - 結果：25962 個すべてと手書きの 6 個が合格。差の中央値 0.011、p90 0.029、p99 0.040、最大 0.114（`fingerprint`）。0.5 以上違うピクセルは最大 3 個（Lucide の `wheat`、細い隙間の縁）。
  - 直す前（差の閾値が 0.10）は 39 個が落ちた。6 個は `fingerprint` の差だけで、残りの原因は 2 つ、どちらも上で直した：アトラスのラスタライザーが重なる輪郭で縁を太らせる（`hdr_off_select`、`contactless_off` などの `_off` の斜線）、view box の外へ出る部分を切っていない（`sharp/handshake`）。
  - 1 回りは、このマシン（i7-12700H）で約 7 分（Chrome の起動が 400 個ごと）。

### 2. ミップマップと `FilterQuality`

- `paint.FilterQuality`：
  - `FilterNearest`：最近傍
  - `FilterLinear`：双一次。今までの描き方
  - `FilterMipmap`：ミップマップと三線形
  - ゼロ値は「既定」で、`FilterMipmap` として扱う
- `Image` ウィジェットと `Canvas.DrawImage` に、この品質を渡せるようにする。拡大と等倍のときは、ミップマップがあっても見た目は変わらない。縮小した画像は今までより滑らかになるので、そういう golden は描き直してから見比べる。
- **ミップマップは CPU で作る。** premultiplied のまま、2×2 の平均で作る。soft と GPU で同じ段を使えるので、golden がずれない。作るのはテクスチャを上げる最初の 1 回だけ。
- 読むときは、glyph と同じく手書きのフィルタで Load する。サンプラーに任せると、GPU によって結果が変わるため。

段 2 を実装したときに決めた細部（2026-11-09）：

- **API：** `paint.FilterQuality` は `FilterDefault`（ゼロ値、`FilterMipmap` と同じ）、`FilterNearest`、`FilterLinear`、`FilterMipmap`。`Canvas.DrawImage` の形は変えず（既定の品質で描く）、`Canvas.DrawImageQuality(img, dst, opacity, quality)` を足した。`Op.Filter` は `Blend` の隣の詰め物に入るので、`unsafe.Sizeof(paint.Op{})` は 384 バイトのまま。`layout.RenderImage.Quality`、`widgets.Image.Quality`、ルートの `FilterQuality` と `FilterDefault` などの定数を足した。
- **`Backend` のメソッドは増やさない：** テクスチャの形式に `gpu.TextureRGBA8Mipmapped` を足した。`UpdateTexture` は level 0 を全体で受け取り（一部だけの更新は契約違反として panic）、バックエンドが `gpu.BuildMips` で下の段を作って上げる。全バックエンドが同じ関数で作るので、段のバイトはどこでも同じ。D3D11 は `MipLevels` と段ごとの `UpdateSubresource`、Vulkan は `mipLevels`、全段を見るビュー、段ごとの `vkCmdCopyBufferToImage`。Vulkan のバリアとクリアの範囲は、全テクスチャで `VK_REMAINING_MIP_LEVELS` にした。
- **段の作り方：** 段の大きさは D3D11 と Vulkan の決まりどおり、上の段の半分を切り捨て、最小 1（`gpu.MipSize`）。1 テクセルまで全部作る（`gpu.MipLevels`）。下の 1 テクセルは、覆う上の段のテクセルの面積で重みを付けた平均。偶数の辺は 2×2 の平均そのもので、整数で計算する（0.5 は切り上げ）。奇数の辺（2n+1 → n）は、1 テクセルが上の段の 3 テクセルにまたがり、端のテクセルは一部だけの重みになる。捨てるテクセルが無いので、一様な絵はどの段でも一様のまま。横、縦の順に分けて計算し、各段は 1 つ上の段から作る。2048×2048 は約 14 ms、全段が奇数の 2047×2047 は約 120 ms（最初のアップロードの 1 回だけ）。
- **render は画像を常にミップマップ付きで上げる：** 最初に描いた品質に関係なく作る。あとで品質が変わったときに、描きかけのバッチが指すテクスチャを作り直さずに済む。予算の計算では 4/3 倍にした。
- **フィルタの運び方：** `gpu.ImageFilter`（Linear がゼロ値、Nearest、Mipmap）を、画像には無いグラデーションの欄 `Instance.Gradient` に入れる（`SetImageFilter`）。インスタンスは増えない。レイヤーのページと色付きのグリフのページ（1 段）はゼロ値の Linear のまま。
- **読み方：**
  - Linear は今までどおりサンプラーで読むが、`Sample` を `SampleLevel(…, 0)` にした（段のあるテクスチャで暗黙の LOD を使わないため）。
  - Nearest は `Load` で `floor(uv·大きさ + 1/256)` のテクセル。1/2 や 1/4 の縮小では画素の中心がちょうどテクセルの境目に乗るので、1/256 テクセルだけ後ろへ寄せて、丸めの差で GPU と soft が割れないようにした。
  - Mipmap は LOD が 1/256 以下なら Linear と同じ道を通る。だから拡大と等倍は今までとビットまで同じで、既存の golden は変わらない。それより上は、LOD の上下の 2 段をそれぞれ `Load` の 4 テクセルと正確な重みのバイリニアで読み、LOD の端数で混ぜる（三線形）。最後の段で頭打ち。soft は同じ式。
- **LOD：** `log2(σ / scale)`。σ は面からテクセルへの写像 `diag(論理ピクセルあたりのテクセル)·T⁻¹` の大きい方の特異値（T は 13a の Affine の線形部分）。回転では変わらず、斜めに歪めても一番縮む向きで決まる。片方の軸だけ縮めた絵は、縮む軸の段を使うので、もう一方の軸はぼける（異方性のフィルタはしない）。LOD はピクセルシェーダーがインスタンスの値から計算する。render で計算すると、インラインのレイヤーで変換を足す `appendMoved` のあとに合わなくなるため。
- **回した画像の uv：** 回した画像は、uv を補間した varying ではなく、画素の中心を逆変換して求める（`exactLogical`。そのために `translation` を varying に足した）。補間はラスタライザーがサブピクセルの格子に丸めた頂点に対して線形で、縮めた画像では 1/100 テクセルほどずれる。Nearest では選ぶテクセルが変わり、回した Nearest で soft と両 GPU が 10 画素ほど違った（golden の 8 画素を超える）。soft はもともと逆変換で求めている。縁のカバレッジは今までどおり補間した値で、Linear と Mipmap も同じ uv を使う。
- **golden：** `mipmaps`（580×430、倍率 1）。コードで作った 256×256 の細かい絵（1 テクセルの白黒の市松、色の zone plate、3 テクセルおきの斜線、文字のような棒の行）を、1/2、1/3、1/5、1/8 と、70 の大きさで 0.5 ラジアン回したもので、Nearest、Linear、Mipmap（`FilterDefault`）の 3 行に描く。D3D11（Iris Xe）は最大 1、Vulkan（RTX 3050 Ti）は最大 2、閾値 2 を超える画素は 0 で、閾値は広げていない。既存の golden で画像を縮めているものは無く、すべてビットまで同じ（`controls-dark` は既知の再生成のずれなので戻した）。`gpu` のパリティのテストに `mipmaps` の場面（3 品質、1/2、1/3、1/4.8、片方の軸だけの縮小、回したもの、半透明のテクセル）を足した。D3D11 と Vulkan の差は最大 1。
- **費用：** HEAD と一時的な worktree で交互に 6 回ずつ測った（`-benchtime 2s`、中央値）。`BenchmarkCanvas_10kRects` は 1.00 ms 対 0.98 ms、`BenchmarkEncode_10kRects` は 0.76 ms 対 0.74 ms、`BenchmarkEncode_10kGlyphs` は 1.28 ms 対 1.27 ms（HEAD 対 段 2）で、差は揺れの中。どれもアロケーション 0。`BenchmarkEncode_1kImages`（64×64 のミップマップ付き画像 1000 個、5 個に 1 個は回す、`Finish` 込み）は約 0.07 ms、アロケーション 0。D3D11 と Vulkan のテストで、ミップマップ付きの画像を毎フレーム描いても、最初のアップロードのあとはアロケーション 0 を確かめた。
- **残した穴：** 異方性のフィルタは無い。回した Nearest は、逆変換の float の丸めの差で、ごくまれに soft と GPU が隣のテクセルを取る（パリティのテストで場面あたり 0〜1 画素。golden の `mipmaps` では 0）。段は CPU で作るので、大きな画像の最初のアップロードが重くなる（上の数字）。レイヤーのページと色付きのグリフは段を持たないので、縮めて描くと今までどおり Linear のちらつきが出る。
- **lavapipe で `mipmaps` が合わなかった原因（2026-11-09）：** ミップマップの道ではなく、lavapipe（Mesa 25.2.8 の llvmpipe）のシザーだった。差は 3 行とも 1/8 のセルの左の 1 列（x = 396）と、回した画像の上の角の 1 画素だけで、Nearest と Linear の行にも同じだけ出ていた。画像はそれぞれ自分の矩形でクリップされ（`DrawCall.Clip`）、1/8 のセルの左端は 396.53 なので、シザーの左は `PhysicalBounds` で 397 になる。この場面はクリップのパスがあるので MSAA（4x）で描く。lavapipe は多重サンプルのパスで、シザーの外の 1 列（1 行）のサンプルを通してしまい、x = 396 に画像の端のテクセルが 2/4 のカバレッジで乗っていた（123 = 245 と黒の半分、146 = 245 と青の半分。soft、D3D11、RTX はシザーどおり 0）。矩形とクリップの小数部を変えた小さな場面で、MSAA のときだけ、頂点の位置によって起きたり起きなかったりすることを確かめた。シザーは画素ごとの検査なので、lavapipe の誤り。直し方：Vulkan で、Mesa の CPU のデバイス（vendorID 0x10005、`VK_PHYSICAL_DEVICE_TYPE_CPU`）の多重サンプルのパスでだけ、`sdf.ps.hlsl` がシザーを画素で繰り返して `discard` する。矩形は push constant（`int4`、パイプラインレイアウトに全デバイスで宣言）で渡し、入れるかどうかは特殊化定数 `SCISSOR_IN_SHADER`（既定 false）で決めるので、ほかのデバイスでは検査ごと消える。`#ifdef __spirv__` の中なので DXBC はバイトまで同じ。lavapipe はパイプラインを結び直すと前に送った push constant を捨てるようなので、シザーを変えたときと、パイプラインを結んだときの両方で送る。結果：lavapipe の `mipmaps` は最大 123、閾値 2 を超える画素 133、12 を超える画素 83 から、最大 4、34、0 になった（残りは Linear のサンプラーの重みの丸め。閾値は広げていない）。ほかの golden も lavapipe で全部通る。Windows の D3D11 と Vulkan（RTX）の golden の数字は、変える前の SPIR-V と比べて全場面で同じで、`mipmaps.png` は作り直していない。カスタムシェーダーのパイプラインはこの検査を持たないので、lavapipe の MSAA でクリップの端に同じずれが出うる（`shader` の golden では出ていない）。

### 3. 動く画像

- `paint.DecodeAnimation(r io.Reader) (*Animation, error)`。結果は `Frames []*Image`、`Delays []time.Duration`、`Loops` を持つ。
- **フレームは読むときに 1 枚の絵まで合成しておく。** dispose と blend（GIF の disposal、APNG の dispose_op / blend_op、WebP の dispose / blend）を適用する。描くときは、普通の画像を差し替えるだけになる。
- 形式ごとの読み方：
  - GIF：`image/gif.DecodeAll`
  - APNG：acTL / fcTL / fdAT を自前で読む。各フレームを PNG のストリームに組み直し、`image/png` で読む
  - WebP：RIFF の VP8X / ANIM / ANMF を自前で読む。各フレームを単独の WebP に包み直し、`x/image/webp` で読む
- **外から来るファイルとして扱う。** 幅と高さ、フレーム数、合計の画素数に上限を置き、超えたらエラーにする。APNG と WebP のコンテナを読む部分には fuzz テストを付ける。
- **ウィジェット：** `widgets.AnimatedImage{Source, Fit, Quality}`。Ticker で次のフレームへ進める。隠れた部分木（ADR 0027）にあるときは止める。
- **ReduceMotion のときは最初のフレームで止める。** 動きを止めるという、Phase 12 の方針に合わせる。

段 3 を実装したときに決めた細部（2026-11-09）：

- **API：** `paint.Animation{Frames, Delays, Loops}`。`Loops` は再生する回数で、ゼロ値の `LoopForever` は終わりなく繰り返す。APNG の `num_plays` と WebP の loop count はそのまま、GIF の NETSCAPE の値 n はブラウザと同じく n+1 回（`image/gif` の -1 は 1 回）。`FrameAt(t)` が、t に出るフレーム、次のフレームの時刻、終わったかを返す（アロケーション 0、`Delays` の数が合わなければ panic）。終わったら最後のフレームのまま。静止画（PNG、GIF、WebP、JPEG）は 1 フレームの `Animation` になるので、ウィジェットはどんな画像も受け取れる。ルートには `Animation`、`AnimatedImage`、`DecodeAnimation`、`LoopForever` を置いた。
- **ブラウザに合わせたこと：** 10 ms 以下の遅延は 100 ms にする（Chrome の `ImageDecoder` が全形式に掛ける規則。0 や 1/100 秒の GIF が多い）。dispose の「背景」は透明に戻す。WebP の ANIM の背景色は仕様でも「ヒント」で、libwebp の `WebPAnimDecoder` もブラウザも使わないので、読むだけで使わない。キャンバスは透明から始まる。
- **合成：** キャンバスは premultiplied の `image.RGBA` で、`image/draw` の Src / Over で描く。`draw` は `image.NRGBA`（PNG、VP8L）のようなストレートアルファの絵を変換しながら混ぜるので、自前の式は持たない。dispose の「前の絵に戻す」は、そのフレームの範囲だけをキャンバスと同じ大きさの控えに写して戻す。各フレームはキャンバスの写しを `paint.Image` にする。GIF のフレームは Over で描く（透明のインデックスは α 0 のパレットで、ほかは不透明なので、GIF の決まりと同じになる）。APNG の最初のフレームの `APNG_DISPOSE_OP_PREVIOUS` は仕様どおり背景として扱う。
- **APNG：** チャンクをすべて CRC で確かめ、fcTL と fdAT の通し番号が 0 から順に並ぶことを求める。acTL のフレーム数と fcTL の数が違えば、エラー（静止画への逃げはしない）。フレームは、フレームの幅と高さに書き換えた IHDR、IDAT より前の PLTE と tRNS（`image/png` が使う補助チャンクはこの 2 つだけ）、fdAT の中身を並べた IDAT、IEND で単独の PNG に組み直して、`image/png` で読む。IDAT の前に fcTL があれば既定の画像が最初のフレームで、その fcTL はキャンバス全体でなければエラー。無ければ既定の画像は飛ばす。`delay_den` が 0 なら 100。
- **WebP：** RIFF のチャンクを自前で分ける。ANMF の中の ALPH と VP8 / VP8L を取り出し、VP8L はそれだけ、VP8 は単独、ALPH があれば α のフラグを立てた VP8X を前に付けた WebP に包み直して `x/image/webp` で読む。読む前に、ビットストリームだけを包んだファイルの `DecodeConfig` で大きさを取り、ANMF の大きさと合わなければエラーにする（VP8X のキャンバスを信じると、ビットストリームが 16383² を名乗ったときに 1 GB を確保しうる）。静止画の WebP も同じ道を通る。
- **VP8 の色：** `x/image/webp` は YCbCr のサンプルを返し、Go はそれを JPEG と同じフルレンジで RGB にするので、白が 235 になる（libwebp は BT.601 の限定レンジ）。動く画像の道では、libwebp の `VP8YUVToR/G/B` と同じ固定小数点の式で変換する。色差は最も近いサンプルを取り、libwebp の「fancy upsampling」はしない（色の境目だけ違いうる）。`DecodeImage` の静止画の WebP は今までどおり Go の変換のまま（段 3 の範囲の外）。
- **上限（すべて公開の定数）：** ファイル 256 MiB（`MaxAnimationBytes`）、キャンバスの辺 16384（`MaxAnimationSide`）、フレーム 4096（`MaxAnimationFrames`）、全フレームの画素の合計 2^25（`MaxAnimationPixels`、premultiplied で 128 MiB）。超えたら `ErrAnimationTooLarge` を包んだエラー。合計の上限は、ミップマップ込み（4/3 倍、約 171 MiB）で 1 つのアニメーションが render の画像の予算 256 MiB に収まるように選んだ。これより大きいと、ループのたびに古いフレームから追い出されて上げ直しになる。どの形式も、フレームの絵を読む前に数と大きさを確かめる。GIF は `gif.DecodeAll` の前にブロックを自前でたどってフレームを数える（LZW は 1 フレームあたりその範囲までしか伸びない）。APNG は acTL と fcTL、WebP は ANMF の数とヘッダー。壊れた入力はエラーで、panic しない。
- **fuzz：** `FuzzDecodeAnimation`（全形式の種）、`FuzzDecodeAPNG`（APNG の種だけ。CRC で弾かれないよう、試す前にチャンクの CRC を直す）、`FuzzDecodeAnimatedWebP`（WebP の種だけ）。それぞれこのマシンで 4 分回して失敗 0（2100 万、4150 万、1880 万回）。結果は、フレームと遅延の数が合う、全フレームが同じ大きさ、premultiplied（どの色も α 以下）、遅延が 10 ms より長い、上限の内側、を確かめる。
- **テストの素材：** GIF と APNG はテストの中で作る（GIF は `gif.EncodeAll`、APNG は zlib で行を詰めた IDAT と fcTL / fdAT を自前で書く）。WebP は Go で書けないので、ffmpeg（libwebp）で作った小さな静止画（VP8L 3 つ、ALPH + VP8 1 つ）を testdata に置き、テストで ANIM / ANMF に組む。dispose（GIF は none / background / previous と未指定、APNG は none / background / previous、WebP は none / background）と blend（APNG は source / over、WebP は blend / no blend）の組み合わせをすべて画素で確かめる。加えて、libwebp の `WebPAnimEncoder` と ffmpeg の APNG エンコーダーが同じ 6 フレームから作ったファイル（部分フレーム、dispose、blend を自分で選ぶ）を読み、可逆なので元の絵とバイトまで同じことを確かめる。作り方は `paint/testdata/anim/README.md`。
- **ウィジェット：** `widgets.AnimatedImage{Source, Width, Height, Fit, Radius, Quality, Label}`。`Image` と同じ欄にした。`Build` は自分を `RepaintBoundary` で包み、その中の `layout.RenderAnimatedImage`（`RenderImage` を埋め込み、`Source` を今のフレームに差し替える）を置く。フレームが変わると `MarkNeedsPaint` だけを呼ぶので、作り直しもレイアウトも無く、境界のレイヤーだけを記録し直す（テストで `Repaint` が窓全体を記録し直さないことを確かめた）。
- **Ticker は render object が持つ：** `RenderBase.NewTicker` を最初の `Paint` で作って動かし、毎フレームではなく、次のフレームの時刻まで `SleepUntil` で眠る。GIF が 10 fps なら窓も 10 fps しか描かない。再生した時間は、動いている間だけ数える（`anchor` から引く）。木から外れたら次に起きたときに止まる（core に外れたことを知らせる口が無いので、`Attached()` を見る。テキストのキャレットと同じ）。ループが終われば止まる。`Source` が変われば最初のフレームからやり直す。
- **隠れた部分木（ADR 0027）：** 起きるたびに祖先をたどって `core.Hider` を聞く。隠れていたら、それまでの時間を再生済みに足して止め、200 ms ごとに起きて見直す（フレームは描かない）。隠すのをやめたことを render object に知らせる口が無く、境界のレイヤーがあると親の描き直しでも `Paint` が呼ばれないため、見直すしかない。見えるようになってから再開まで最大 200 ms、隠れたことに気づくまで最大 1 フレームの遅延がかかる。透明度が 0 を超えた最初は、まだ止まった絵なので、目には付きにくい。
- **ReduceMotion：** `Build` で `MediaQueryOf(ctx).ReduceMotion` を読み、`SetStill(true)` で最初のフレームのまま Ticker を止める。設定が変われば `MediaQuery` への依存で作り直され、最初から再生する。
- **テクスチャ：** フレームは普通の `paint.Image` なので、render の画像のキャッシュ（ポインタで引く）を通り、段 2 のミップマップもそのまま付く。60 フレームのアニメーションを 1 フレームずつ描くテストで、最初の 1 周で 60 枚を上げたあとは、次の周からテクスチャを作らず、アロケーションも 0。
- **golden：** `animated`（400×150、倍率 1）。GIF（previous と background の dispose、透明のインデックス、部分フレーム）、`movie.png`、`movie.webp` の 4 番目のフレーム（前のフレームの dispose と blend の結果）を、`AnimatedImage` で 4 倍に Nearest で拡大したものと、Mipmap で縮めたもの。`Render` は眠っている Ticker を「止まった」とみなすので、途中のフレームから始まる `Animation`（`Frames[3:]`）を渡して描く。D3D11（Iris Xe）は最大 0、Vulkan（RTX 3050 Ti）は最大 1。既存の golden はビットまで同じ（`controls-dark` は既知の再生成のずれなので戻した）。
- **費用：** `BenchmarkAnimatedImage_Frame`（フレームが変わる窓の 1 フレーム：tick、build、layout、境界の記録）は約 0.3 µs、アロケーション 0。60 フレームを 5 周させても 0。既存のホットパスのベンチは段 2 と同じ（`Canvas_10kRects` 1.0 ms、`Encode_10kRects` 0.7〜0.8 ms、`Encode_10kGlyphs` 1.28 ms、`Encode_1kImages` 0.08 ms、どれも 0 allocs）。ホットパスのコードには手を入れていない。
- **残した穴：** WebP の fancy upsampling をしない（境目の色差）。`DecodeImage` の WebP は Go の色の変換のまま。上限より小さくても、たくさんのアニメーションを同時に流すと、render の予算 256 MiB を超えたところで追い出しと上げ直しが起きる（LRU で、ループでは次に要るものから追い出す最悪の形）。隠れた部分木からの再開は最大 200 ms 遅れる。画面の外にスクロールしても、木に残っている限り（`ListView` の外の行は木から外れる）動き続ける。デコードはすべてのフレームを一度に作るので、長いアニメーションはメモリを食う（上限までで 128 MiB）。

### 4. カスタムシェーダー

- **利用者が書くもの：** HLSL の関数を 1 つ書く。

  ```hlsl
  float4 shade(ShaderInput i)
  ```

  `gowaku-shader` が前置き（cbuffer、テクスチャ、補助関数）と固定の頂点シェーダーを付けて、完全な pixel shader にする。返す色は premultiplied。
- **`ShaderInput` の中身：**
  - `pos`：局所座標の論理 px
  - `size`
  - `time`
  - `scale`
  - `params[4]`：float4
  - `tex0` と `tex1`：補助関数 `sample0(uv)` と `sample1(uv)` で読む
- **`cmd/gowaku-shader`：** `x.hlsl` から次のファイルを作る。
  - `x.dxbc`
  - `x.spv`
  - `x.metal`
  - `x_shader.go`：`//go:embed` で埋め込み、`var X = paint.NewShader(...)` を置く

  `internal/cmd/shaderc` と共通の部分は、`internal/shadertool` にまとめる。MSL は作って入れておくだけで、動かすのは Phase 17。
- **`paint.Shader`：** 3 つのバイト列と ID を持つ。描く操作は次のとおり。
  - `Canvas.DrawShader(rect, shader, params, tex0, tex1)`：テクスチャは画像かレイヤー
  - `widgets.CustomPaint{Shader, Params, Animate}`：`Animate` のときは Ticker が時刻を進める
  - `widgets.ShaderMask{Shader, Params, Child}`：子をレイヤーに描き、それを `tex0` として渡す。出力がそのまま最終の色になるので、Flutter の ShaderMask より一般的に使える。
- **バックエンド：**
  - パイプラインはシェーダーごとに、最初に使うときに作ってキャッシュする（D3D11 は pixel shader、Vulkan はパイプライン）。
  - 作るのに失敗したら、描画の中で panic せず、`Window` の描画エラーとして返す。当初の計画の「初期化時に error」に合わせる。
  - 違うシェーダーの間は描画呼び出しを区切る。
  - クリップ、Affine、レイヤー、ブレンドモードは、普通のインスタンスと同じように効く。
- **`gpu/soft`：** HLSL は動かせない。`paint.Shader.Fallback`（Go の関数、なくてもよい）があればそれで塗る。無ければ何も塗らず、gowakutest の警告に出す。
- **完了条件の確かめ方：** 例のシェーダーを、次の 3 つで描いて比べる。
  - Windows の D3D11
  - Windows の Vulkan（RTX）
  - Linux の Vulkan（WSL で使える Vulkan デバイス。lavapipe など）

  比べる閾値は、ほかの golden と同じにする。

段 4 を実装したときに決めた細部（2026-11-09）：

- **利用者が書くもの：** `float4 shade(ShaderInput i)` を 1 つ。返す色は premultiplied。`ShaderInput` は次のとおりで、`paint.ShaderInput` と `gpu.ShaderInput` が同じ並びの写し。

  ```hlsl
  struct ShaderInput {
      float2 pos;       // 矩形の左上からの画素の中心。キャンバスの論理 px
      float2 size;      // 矩形の大きさ。同じ単位
      float2 uv;        // pos / size
      float time;       // 秒
      float scale;      // pos の 1 単位あたりの物理 px
      float4 params[4]; // paint.ShaderParams の 16 個
  };
  float4 sample0(float2 uv);  float2 size0();  // tex0
  float4 sample1(float2 uv);  float2 size1();  // tex1
  ```

  `gowaku`、`blend`、`BLEND_` で始まる名前は gowaku が使う。`Canvas.Scale` の下では pos と size がキャンバスの単位のままで、scale にズームが入る（遷移で縮んでも模様が一緒に縮む）。
- **頂点シェーダーは新しく作らない：** SDF の頂点シェーダーをそのまま組み合わせる。前置きの `GowakuVaryings` は `sdf.hlsli` の `Varyings` と同じ並び、型、セマンティクスで、テスト（`TestPrelude_VaryingsMatchSDF`）が一致を確かめる。だから D3D11 のシグネチャも Vulkan のロケーションも合い、クリップ（深さはビューポートで決まる）、Affine、レイヤー、ブレンドモードが普通のインスタンスと同じに効く。新しい種類 `InstanceShader`（10）は、頂点シェーダーの中では画像と同じ扱い（立っていれば縁なし、回れば 1 物理 px の縁）になるので、`sdf.vs.hlsl` は変えていない。
- **定数バッファは足さず、インスタンスに載せる：** 16 個の float は `Radii`、`StopOffsets`、`GradientMatrix`、`GradientStart`/`GradientEnd` に、時刻は `BorderWidth`、ズームは `Blur`、不透明度は `Color.A`、2 枚のテクスチャの位置は `EndColor`/`BorderColor`/`MidColors` に入れる（`gpu.Instance.SetShaderParams` と `SetShaderTexture`）。描画コールごとの cbuffer にしなかった理由：同じシェーダーで続くインスタンスが 1 つのコールに入る（引数が違っても）、フレーム中にバッファを書き換えない、レイヤーのページに描いても `appendMoved` がそのまま運ぶ、アロケーションが無い。cbuffer 版は作っていないので、両者の比較の数字は無い。`appendMoved` はシェーダーのインスタンスの `GradientStart`/`GradientEnd` を動かさず、`blendEach` は `Color.A` 以外を薄めない。
- **レジスタ：** すべてのバックエンドで同じ番号にする。

  | HLSL | 中身 | Vulkan（SPIR-V） | Metal（MSL） |
  |---|---|---|---|
  | `cbuffer GowakuFrame : register(b0)` | `float2 viewport; float scale; float padding;`（`sdf.hlsli` の Frame） | set 0, binding 0 | `[[buffer(0)]]` |
  | `Texture2D<float4> gowakuTex1 : register(t0)` | tex1（`DrawCall.Texture`） | set 1 | `[[texture(0)]]` |
  | `Texture2D<float4> gowakuTex0 : register(t1)` | tex0（`DrawCall.Image`） | set 2 | `[[texture(1)]]` |
  | `Texture2D<float4> gowakuBackdrop : register(t3)` | 下地を読むブレンドモードの下地 | set 4 | `[[texture(3)]]` |

  tex1 はグリフのアトラスの t0 を借りる。シェーダーのコールにグリフは乗らないので、ぶつからない。こうすれば Vulkan のディスクリプターセットが 6 組のまま増えない。サンプラーは使わない。
- **テクスチャの読み方：** グリフやランプと同じく、`Load` の 4 テクセルを正確な重みで混ぜる（GPU ごとのサンプラーの丸めに依らない）。`ShaderTexture.Place` は矩形の左上と右下にあたるテクセルの位置（テクセルの中心は半整数）、`Texels` は絵のあるテクセルの範囲で、その外は透明として読む。画像は `Place = Texels = (0, 0, 幅, 高さ)` で矩形いっぱいに伸ばし、レベル 0 だけを読む。レイヤーはページのスロットの範囲で、矩形と同じ場所に等倍で重なるように `Place` を render が計算する（`ShaderMask` では画素がテクセルの中心にちょうど乗る）。
- **pos の求め方：** 立った矩形では補間した varying を使わず `SV_Position / scale - rect.xy`、回った矩形では画素の中心の逆変換（`exactLogical`、段 2 の画像と同じ）を矩形の中に抑えたもの。縁のカバレッジは画像と同じ式。soft も同じ式で、`Fallback` に同じ入力を渡す。
- **ブレンド：** `sdf.ps.hlsl` のブレンドの式を `gpu/shaders/blend.hlsli` に移し（関数の名前を `blend` 前置にし、下地のテクスチャは `BLEND_BACKDROP` で指す）、gowaku-shader は同じファイルの写し（`internal/shadertool/hlsl/blend.hlsli`、テストでバイトの一致を確かめる）を前置きに入れる。移したあとの `sdf.ps.dxbc` と `sdf.ps.spv` は前とバイトまで同じ。
- **`cmd/gowaku-shader`：** `gowaku-shader [-pkg p] [-name N] [-fallback fn] [-out dir] [-keep] x.hlsl` が `x.dxbc`（fxc、`ps_5_0`）、`x.spv`（dxc、`ps_6_0`、`vulkan1.0`、上の set）、`x.metal`（下の §5）、`x_shader.go` を書く。`-pkg` を省くと出力先の .go の package、無ければディレクトリ名。`-name` を省くとファイル名をつないだ名前（`wave_mask` は `WaveMask`）。前置きと後置きの間に `#line 1 "x.hlsl"` を置くので、fxc と dxc のエラーは利用者のファイルの行と桁を指す（`bad.hlsl(4,22): error X3004`）。`shade` が無いソースはコンパイラーに渡す前に分かりやすいエラーにする。`-keep` は包んだソースを残す。`internal/cmd/shaderc` は `internal/shadertool` の同じ引数の組み立てを使うようにした（出力はバイトまで同じ）。
- **生成する Go：**

  ```go
  var Ripple = paint.NewShader(paint.Shader{Name: "ripple", DXBC: rippleDXBC, SPIRV: rippleSPIRV, MSL: rippleMSL, Fallback: rippleFallback})
  ```

  3 つのバイト列は `//go:embed`。`NewShader` は `ID` に 3 つのバイト列の FNV-64a を入れる（同じコードなら同じ ID。キャッシュはポインターで引くので、ID は表示と識別のため）。ルートには `Shader`、`ShaderParams`、`ShaderInput`、`ShaderFallback`、`NewShader`、`ShaderMask` を置いた。
- **paint：** `Canvas.DrawShader(dst, paint.ShaderCall{Shader, Params, Time, Tex0, Tex1})`。ADR の 5 引数は、引数が 4 を超えたら構造体にする規約（§5.3）で 2 引数にした。`ShaderTexture{Image, Layer, Origin}` は画像かレイヤーのどちらか（両方は panic）。16 個の float と 2 枚のテクスチャは `Op` に入らないので、`DisplayList.Shaders` に置き、`Op.ShaderCall`（`int32`、`Blur` の後ろの詰め物）がその番号を持つ。`unsafe.Sizeof(paint.Op{})` は 384 バイトのまま。`Reset` で `Shaders` を空にする。
- **レイヤーを読む条件は `CanShadeLayer`（拡大が無いこと）：** `DrawLayer` と違い、レイヤーの中でも読める。Navigator のページ（`RepaintBoundary`）の中に `ShaderMask` があることは普通なので、入れ子を許した。render はシェーダーが読むレイヤーを別のページの組（`shaderLayers`、最大 4 ページ）に置き、ふつうのレイヤーのページより先に描く。だから、ページに描くレイヤーの中のシェーダーも、`shaderLayers` のページを読める。`shaderLayers` のレイヤーの中で、さらにレイヤーを読むシェーダー（`ShaderMask` の中の `ShaderMask`）は、ページの順が作れないので、内側を素のまま描く。ページに載らないレイヤー（パスや角丸のクリップでマルチサンプルが要る、ぼかしを含む、2048 を超える）も、シェーダーを描かずに子をそのまま描く。
- **描画コールの区切り：** シェーダーのコールには、同じシェーダーで同じテクスチャのインスタンスだけが続けて入る。ほかの命令は入らず、ほかの命令のコールにも入らない。下地を読むブレンドモードでは、`beginBackdrop` が作った空のコールにシェーダーが入る。
- **作るのに失敗したとき：** `gpu.Backend.PrepareShader(*gpu.Shader) error` を足した。`render.Resources.Finish`（フレームの外）が、初めて使うシェーダーごとに 1 回呼ぶ。失敗したら、そのフレームのコール（ページの分も）の `Count` を 0 にし、以後は描かない。エラーは `Resources.DrawError()` に残り、`gowaku` はフレームのあとに `WindowOptions.OnDrawError`（`App.OnDrawError`）へ 1 回だけ渡す。無ければ `slog` の既定のロガーに Error で書く。フレームは止めない（今までの `Finish` のエラーは panic のまま）。バックエンドは準備していないシェーダーのコールを飛ばし、panic しない。
  - D3D11：`CreatePixelShader`。DXBC はランタイムが検査する。ピクセルシェーダーは `pipeline.use` が段ごとに差し替える。`DrawToTarget` は t0 も外す（tex1 にページが来うる）。
  - Vulkan：モジュールと、平のパスの source-over のパイプラインを `PrepareShader` で作る（ドライバーがここでコンパイルする）。マルチサンプルのパスとほかのブレンド状態のパイプラインは、初めて使うときに作る（失敗は今のブレンドモードのパイプラインと同じく `Draw` のエラー）。ターゲットのパスは平のパスと互換なので同じパイプラインを使う。Vulkan は SPIR-V でないコードの扱いを決めておらず、Iris Xe は 0 が並んだだけのコードも受け付けたので、マジックナンバーを先に確かめる。
- **gpu/soft と gowakutest：** soft は `Fallback` を呼ぶ（`render` が `paint.ShaderFallback` を `gpu.Shader.Fallback` に包む。入力の構造体はクロージャーに 1 つ持ち、呼ぶたびに作らない）。無ければ何も描かない。`gowakutest.Warnings` は `Fallback` の無いシェーダーを描く `CustomPaint` と `ShaderMask` を `WarnShader` で知らせる。
- **ウィジェット：** `CustomPaint` に `Shader`、`Params`、`Textures [2]*paint.Image`、`Time`、`Animate` を足した。シェーダーが箱を塗ってから `Painter` が上に描く。`ReduceMotion` を読むために `CustomPaint` を `StatelessWidget` にした（中身は今までの render widget の `customPaintView`）。`ShaderMask{Shader, Params, Texture, Time, Animate, Child}` は `layout.RenderShaderMask` で、子をレイヤーに記録して tex0 にする。`Animate` は両方とも、自分を `RepaintBoundary` で包み、毎フレーム描き直す時計（`shaderClock`）を回す。段 3 と同じく、隠れた部分木では止めて 200 ms ごとに見直し、`ReduceMotion` では `Time` で止める、木から外れたら止まる。拡大の下の `ShaderMask` は子をそのまま描く。
- **例：** `examples/shader`。シェーダーは `examples/shader/shaders` に HLSL で 4 本：`aurora`（`CustomPaint` の、時間で動く輪とグラデーション）、`shimmer`（`ShaderMask` で文字に光の帯）、`ripple`（`ShaderMask` で画像と文字を波で曲げる）、`crossfade`（2 枚のテクスチャ。パリティのテスト用）。生成物と Go の `Fallback` も入れた。`shaders.Scene(t, animate, picture)` が例の絵で、golden も同じものを描く。`go generate ./examples/shader/...` で作り直す（`TestCompileShader_ExampleIsUpToDate` が生成物の古さを見つける）。
- **golden `shader`（1320×780、倍率 1.5、1.25 秒）：** CPU は `Fallback` で描いた絵。D3D11（Iris Xe）最大 1、Vulkan（RTX 3050 Ti、`HighPerformance`）最大 2、Linux（WSL の Mesa lavapipe = llvmpipe、XWayland）最大 2、どれも閾値 2 を超える画素 0。だから 3 つは互いに 4 以内で同じ絵になる（同じ PNG に照らした）。既存の golden はバイトまで同じ（`controls-dark` は既知の再生成のずれなので戻した）。`gpu` のパリティのテストに `shaders` の場面（`aurora`、回して薄めて Multiply で角丸のクリップの中、2 枚のテクスチャ、レイヤーを読む `ripple`）を足した。D3D11 と Vulkan の差は両 GPU で最大 1、soft との差は最大 1。レイヤー越し（`DrawToTarget`、レイヤーの中のシェーダーが `shaderLayers` を読む）も soft と最大 1。
- **lavapipe で見つかったこと（段 4 の外）：** 同じ Linux の実行で、段 2 の golden `mipmaps` が lavapipe でだけ閾値を超えた（最大 123、閾値 2 を超える画素 133、12 を超える画素 83）。段 4 はミップマップの道に触れていない。原因は lavapipe の多重サンプルのシザーで、§2 に書いた（2026-11-09）。
- **費用：** HEAD と一時的な worktree で交互に 6 回ずつ測った（`-benchtime 2s`、中央値）。`BenchmarkCanvas_10kRects` は 0.86 ms 対 0.88 ms（HEAD の 6 回が 0.80〜0.94 ms に散る揺れの中）、`BenchmarkEncode_10kRects` は 0.65 ms 対 0.65 ms、`BenchmarkEncode_10kGlyphs` は 1.20 ms 対 1.19 ms（HEAD 対 段 4）。`BenchmarkEncode_1kImages`（HEAD に無い）は 0.074 ms で段 2・3 と同じ。どれもアロケーション 0。ホットパスに足したのは、`addDraw` と `beginBackdrop` の `Shader == nil` の比較、`encodeOps` の分岐 1 つ、`Reset` の `Shaders` の片付けだけ。新しいベンチ：`BenchmarkCanvas_DrawShader`（100 個）4.4 µs、`BenchmarkEncode_1kShaders`（2 本のシェーダーを 2 個ずつ交互に 1000 個、半分は画像を読む、`Finish` 込み）0.085 ms、`BenchmarkCustomPaint_AnimatedShaderFrame`（時計が進むフレーム：tick、build、layout、境界の記録）0.11 µs、どれもアロケーション 0。毎フレーム記録し直すレイヤーを読むシェーダーを含むフレームも、render のテストで 0。D3D11 と Vulkan のテストで、シェーダーを描くフレーム（マルチサンプル、Multiply を含む）は 2 フレーム目からアロケーション 0、パイプラインもピクセルシェーダーも作り直さない。Vulkan の検証レイヤーを入れた golden とシェーダーのテストで VUID は 0。
- **残した穴：** MSL は作って番号を確かめただけで、動かしていない（Phase 17）。`sample0` は画像のレベル 0 だけを読むので、大きな画像を縮めて読むと粗くなる（ミップマップも異方性も無い）。伸ばした画像の縁は、外を透明として混ぜるので半テクセル分薄まる。`ShaderMask` の子がマルチサンプルの要るもの（アトラスに入らないパス、角丸のクリップ、`Image{Radius}`）、ぼかし、2048 px を超えるものを含むと、シェーダーを描かずに子をそのまま描く。`ShaderMask` の中の `ShaderMask` は内側が素のまま。拡大の遷移の間の `ShaderMask` も子のまま描く（遷移の終わりで効果が戻る）。時刻は float32 の秒なので、何時間も動かすと細かい動きが粗くなる。`Fallback` が HLSL と同じ式であることは、利用者が保つしかない（例では golden が確かめる）。描画コールごとの定数バッファと比べた数字は無い。

### 5. Metal（Phase 17）に写せるか

- spirv-cross の MSL は、`[[buffer(n)]]` と `[[texture(n)]]` の番号を決めて出す。前置きの cbuffer とテクスチャの番号を、Metal でも同じに保つ。
- ミップマップ、動く画像、SVG は、CPU の側だけで完結する。Metal では、テクスチャを上げる処理だけ書けばよい。

段 4 を実装したときに決めた細部（2026-11-09）：

- **MSL の作り方：** Vulkan 用の `x.spv` とは別に、MSL のためだけの SPIR-V を dxc で作る。b0 を set 0 / binding 0、t0 を set 1 / binding 0、t1 を set 2 / binding 1、t3 を set 3 / binding 3 に置き、`spirv-cross --msl --msl-version 20100 --msl-decoration-binding` で書く。binding がそのまま Metal の番号になるので、`[[buffer(0)]]`、`[[texture(0)]]`（tex1）、`[[texture(1)]]`（tex0）、`[[texture(3)]]`（下地）と、HLSL のレジスタと同じ番号になる。set を分けたのは、同じ set に同じ binding が 2 つ並ばないようにするため。
- **番号の確かめ方：** gowaku-shader は書いた `.metal` の `[[buffer(n)]]` / `[[texture(n)]]` をすべて読み、上の表と違えばエラーにする（`shadertool.CheckMSL`）。使わない資源は spirv-cross が落とすので、無いのはよい。checked in の例の `.metal` もテスト（`TestCheckMSL_Example`）で確かめる。`crossfade.metal` は tex0 と tex1 の両方を持つ。
- **Phase 17 に残すこと：** フラグメントの入力は `[[user(locnN)]]` で、N は `GowakuVaryings` の並び（SPIR-V のロケーション）なので、Metal の頂点シェーダーは同じロケーションに書く必要がある。使わない入力は MSL から消える（ロケーションの番号は残る）。spirv-cross は `fast::` の関数を使うので、精度が soft と GPU の比較に足りるかを Phase 17 で確かめる。Metal で実際に動かしてはいない。

## 進め方

段ごとに順に進める。各段で、golden、ゼロアロケーション、lint、Linux ビルドを確かめる。

1. SVG と線の輪郭化
2. ミップマップと FilterQuality
3. 動く画像
4. カスタムシェーダーと gowaku-shader

## 却下した案

- **利用者が HLSL・GLSL・MSL を 3 本書く（当初の計画の元の案）：** 書く量が 3 倍になり、絵がずれる原因も 3 つになる。
- **線をシェーダーで描く（SDF のストローク）：** 角と端の形をすべて GPU で解くのは重い。soft とも合わせにくい。
- **動く画像のフレームを、描くときに合成する：** 描画が前のフレームに依存し、途中のフレームへ飛べなくなる。
- **ミップマップを GPU で作る（GenerateMips）：** GPU によってフィルタが違い、soft と合わない。
