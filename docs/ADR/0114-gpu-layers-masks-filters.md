# 0114: GPU のグループ、マスク、フィルター

決定日: 2026-11-14

状態: 採用

## 背景

[ADR 0112](0112-svg-structure-paint-clip.md) と [0113](0113-svg-filters-text.md) で、合成の要る SVG（グループの不透明度、clip-path、mask、filter）を CPU で合成して画像にした。GPU のレイヤー（ADR 0019、0044）は入れ子にできない、`RepaintBoundary` の中で使えない、インラインに落ちるとまとめての合成が失われる、輝度のマスクやフィルターのグラフを表せない、が理由だった。オーナーの要望は「GPU使いたい」。SVG 専用の道ではなく、paint / render / gpu の一般の仕組みとして入れ子のレイヤー、パスのクリップ、マスク、フィルターのグラフを作り、ほかのウィジェットも使えるようにする。そのうえで SVG をそれに載せ替え、CPU の合成を消す。

## 決定

### 1. paint：キャンバスのグループ

- **API：** `Canvas.PushGroup(GroupStyle)`、`Canvas.BeginMask()`、`Canvas.PopGroup()`。`GroupStyle{Blend, Mask, Filter}`。`MaskMode` は `MaskNone`（ゼロ値）、`MaskAlpha`、`MaskLuminance`。
  - グループの中に描いたものは、透明から始まる自分の絵になる。閉じるとき、絵はフィルター → マスク → 不透明度 → `Blend` の順で下に重なる（SVG と CSS の順）。
  - **不透明度と Blend はキャンバスの状態から取る：** `PushOpacity` の値がグループ全体の不透明度になり、中は不透明度 1、source-over から始まる（`DrawLayerBlended` と同じ考え）。`GroupStyle.Blend` がゼロ値なら `PushBlend` のモード。
  - 変換とクリップはそのまま中に続く。`PopGroup` は `PushGroup` の時の状態に戻す（`Restore` と同じ）。グループより前の `Save` に当たる `Restore` は panic（グループが壊れるので）。
  - マスクは `BeginMask` から `PopGroup` までに描いたもの。`PushGroup` の時の変換とクリップ、不透明度 1 で描く。マスクがあるのに `BeginMask` しなかったグループは、空のマスクとして何も見せない。
  - 不透明度 0 や平らな変換の下のグループは、何も記録しない。
- **記録：** `OpGroupPush` / `OpGroupMask` / `OpGroupPop`。大きな中身は `DisplayList.Groups []GroupCall{Style, Mask, End}` に置き、op は `ShaderCall` でその添字を指す（OpShader と同じ横の表。`Op` の大きさは変わらない）。`Mask` と `End` は op の添字なので、render は入れ子を探さずに飛べる。`OpGroupPush` は `Color.A` に不透明度、`Origin` / `Scale` / `Transform` にフィルターの座標の置き方、`Clip` / `ClipLevel` に絵に掛かるクリップを持つ。
- **フィルター：** `Filter{Region, Transform, Primitives}` と `FilterPrimitive`。種類は `FilterBlur`、`FilterOffset`、`FilterDropShadow`、`FilterFlood`、`FilterColorMatrix`、`FilterComposite`（`CompositeOperator`、arithmetic の `K`）、`FilterMerge`、`FilterBlend`、`FilterComponentTransfer`（`TransferFunc`）、`FilterMorphology`。入力は `FilterInput`：ゼロ値が「前の結果、最初なら SourceGraphic」（SVG で in を省いたとき）、`FilterSourceGraphic`、`FilterSourceAlpha`、`FilterTransparent`、`FilterResult(i)`。`SRGB` のゼロ値は linearRGB（SVG の既定）。`Subregion` のゼロ値は「なし」。長さと領域は `Transform` の座標で、回転や歪みの下では領域は外接矩形、標準偏差と半径は軸の伸びで掛ける（近似）。`Filter` はポインターで持ち、Reset までは変えない約束（グリフと同じ）。
- **パスのクリップ：** 今の `ClipPath`（ステンシルと深度の段、ADR 0009）がそのままグループにも効く。グループの外の `ClipPath` は合成した絵に、中の `ClipPath` は中身に効く。グループの絵はマルチサンプルのターゲットに描けるようにしたので（§3）、中でもステンシルのクリップが使える。和や入れ子の規則が要る SVG の clip-path は、アルファのマスクとして描く（§5）。

### 2. render：グループをジョブにする

- **ジョブ：** グループは、ターゲット 1 枚に `DrawToTarget` で描くバッチ（ジョブ）の列になる。中身 → フィルターの各パス → マスク → の順で、どのジョブも読むターゲットを描くジョブより後。最後に、グループのあった場所に絵を `InstanceImage` として描く（クリップ、不透明度、ブレンドはそこで掛かる。下地を読むブレンドは呼び出しを分ける）。入れ子は再帰で、子のジョブが親の中身のジョブより先に並ぶ。`Finish` はページ（ADR 0019）より先にジョブを流す。ページはグループの絵を持てる。
- **RepaintBoundary とインライン：** ページに載せるレイヤーの中のグループは、ラスタライズのときにジョブになり、ページに絵が残る（ページが描き直されるまでジョブは走らない）。インラインのレイヤーの中のグループは毎フレーム走る。グループの中の `OpLayer` は、ページがジョブより後に描かれるので、インラインにする（カスタムシェーダーのレイヤーのテクスチャも同じ理由で素のまま描く）。どちらの場合も、グループのまとめての合成は失われない。
- **格子：** 絵のピクセルは面の物理ピクセルにそろえる（左上を床に、右下を天井に）。フィルターの無いグループは中身の範囲（`extent`）とクリップの交わり、フィルターのあるグループは `Region` を置いたものと、クリップをフィルターの届く距離（ブラー 3σ、オフセット、膨張の和）だけ広げたものの交わり。辺は 2048 まで（Vulkan のターゲットが共有する深度が 2048）。それより大きいと解像度を落として引き伸ばす。中身は描く倍率でもう一度エンコードし直す（グリフとアトラスのパスは倍率に依る）。最初のエンコードのジョブとターゲットは巻き戻す。
- **クリップの段：** 中身の `ClipLevel` はグループの段からの差にする（ターゲットの深度は 0 から）。フィルターのあるグループでは、中身のうちグループ自身のシザーと同じクリップを外す。オフセットやブラーがクリップの外のものを中へ運べるように（ブラウザと同じく、フィルターの後で切る）。
- **ターゲットの使い回し：** 大きさを 64 の倍数に丸めた `CreateTarget` を持ち回る。フレームの初めにすべて返し、120 フレーム使われないものは捨てる。フレームの中でも、フィルターの途中の結果、中身、マスクは最後に読まれるジョブの後で返し、後のジョブが使う（ジョブは作った順に GPU で走るので安全）。グループの最終の絵だけはフレームの終わりまで持つ。定常状態では Encode と Finish のアロケーションは 0（`TestGroup_EncodeDoesNotAllocate`）。
- **マルチサンプルのジョブで出来ないこと：** 下地を読むブレンドは source-over に、ぼかしたレイヤーはぼかさずに、背景のぼかしは描かずに落とす（`keepSingleCopy`）。マルチサンプルでない中身なら、今までどおりターゲットの下地を読める。

### 3. gpu：効果のパスとマルチサンプルのターゲット

- **`InstanceEffect`：** フィルターとマスクのパスは 1 種類のインスタンスで、`gpu.EffectOp`（`EffectCopy`、`EffectFlood`、`EffectMatrix`、`EffectTransfer`、`EffectComposite`、`EffectBlend`、`EffectMorphology`、`EffectGaussian`、`EffectBoxes`、`EffectMask`、`EffectShadow`）と旗（`EffectLinear`、`EffectAlpha0/1`、`EffectEmpty0/1`、`EffectDilate`、`EffectLuminance`）を持つ。入力は描画コールの `Image`（In0）と `Texture`（In1）。パラメーターは 8 つの float4（`EffectParams`、`SetEffect`）を、カスタムシェーダーのインスタンスと同じ場所に入れる。render は倍率 1、`Rect` を原点から領域の大きさで描くので、ピクセルとテクセルが一対一になる。領域の外（プールのテクスチャは大きい）は透明として読む。部分領域はシザー。
- **シェーダーはカスタムシェーダーの道：** `render/effect.hlsl` を `cmd/gowaku-shader` でコンパイルする（`go generate ./render`）。DXBC、SPIR-V、MSL（spirv-cross）がそろい、各バックエンドの `PrepareShader` と既存のパイプラインがそのまま使える。D3D11 と Vulkan のバックエンドには、効果のためのコードは 1 行も要らなかった。`TestEffectShader_IsUpToDate` が、生成物がソースと合っているかを見る（コンパイラが無ければ飛ばす）。soft はシェーダーを走らせられないので、`InstanceEffect` を自分で計算する（`gpu/soft/effect.go`、HLSL と関数ごとに対応）。
- **色の保存：** 途中の結果はすべて premultiplied の sRGB の RGBA8。linearRGB のプリミティブは、読むときに sRGB → linear、書くときに linear → sRGB へ直す（`EffectLinear`）。結果ごとに色空間を持つより単純で、8 ビットでは暗いところの精度が sRGB の方がよい。
- **ブラー：** 標準偏差 2 未満はガウス（片側 ceil(3σ) テクセル、重みは正規化）、2 以上は仕様の 3 回の箱のブラーを、軸ごとに 1 パスの核にした（`gpu.BoxSizes`、`gpu.BoxWeight`：3 つの箱の畳み込みの重みを包除で数える）。横と縦の 2 パス。標準偏差は 256 ピクセルで頭打ち。13a のブラー（`PlanBlur`）は使わなかった。1/2 以下に縮めてからぼかすので小さい σ が粗くなる、等方でしか無い、描画コールの中で走る、の 3 つが SVG（Chrome との一致）に合わない。
- **そのほか：** オフセットは端数を双一次で、ドロップシャドウは入力のアルファをぼかしてから `EffectShadow`（ずらし、色付け、入力を上に）、マージは 2 つずつの合成（linearRGB で重ねるため）、`feComponentTransfer` は 4 チャンネルの表をランプの行（ADR 0044 の 256 テクセル）に焼いて In1 に渡す、モルフォロジーは横と縦の 2 パス（半径は 64 まで、領域の中だけ見る）、`feBlend` は screen と分離可能・不可能なモード、マスクは `0.2125·R + 0.7154·G + 0.0721·B`（premultiplied）。
- **`DrawToTarget` とマルチサンプル：** マルチサンプルの要るバッチ（パスやパスのクリップ）を受けるようにした。マルチサンプルのスクラッチ（透明、深度とステンシル 0 から）に描き、`Clears` の外接矩形へ resolve する。その範囲の前の中身は失われ、下地の読みとぼかしはできない（gpu の契約に書いた）。
  - **D3D11：** RGBA8 のマルチサンプルの色と深度、同じ大きさの 1 サンプルのテクスチャ（D3D11 は部分 resolve が無い）を最大のターゲットまで伸ばして持つ。描いて、全体を resolve し、範囲を `CopySubresourceRegion`。
  - **Vulkan：** フレームのマルチサンプルのパスと互換のパス（同じ添付、同じ依存、resolve 先がターゲット）を作り、そのパイプラインを使う。描画の範囲は `Clears` の外接矩形（シザーもそこに切る）。スクラッチは伸びると作り直し、ターゲットごとのフレームバッファは世代で作り直す。MSAA の無い装置では、ターゲットのパス（共有の深度を持つ）にそのまま描く。
  - **Vulkan のターゲットの定数：** 今まではフレームに 1 組で、「ターゲットはどれも同じ大きさ」を前提にしていた（ページは 2048 四方、倍率も同じ）。グループの絵とフィルターのパスは大きさも倍率も違い、記録した後でまとめて走るので、最後に書いた定数で全部が描かれた。フレームごとに定数の組を必要なだけ持ち（同じ値は共有、定常状態では作らない）、ターゲットごとに束ねるようにした。
  - **Metal：** D3D11 と同じ形（マルチサンプルのスクラッチ、resolve 先、blit で写す）。**このマシンでは動かしていない**（darwin/arm64 のビルドと vet だけ）。効果のシェーダーの MSL は gowaku-shader（spirv-cross）が出したもの。
  - **soft：** 4 サンプルのキャンバスに描き、フレームの resolve と同じ丸めで範囲に書く。

### 4. 使い方の例

```go
cv.Save()
cv.PushOpacity(0.8)
cv.PushGroup(paint.GroupStyle{Mask: paint.MaskLuminance, Filter: &shadow})
drawContent(cv)
cv.BeginMask()
drawMask(cv)
cv.PopGroup()
cv.Restore()
```

### 5. SVG の載せ替え

- **グループのある図は、キャンバスのグループで描く。** 形は今までどおり大きさごとの写しを `DrawPath` で描き、グループは `PushGroup` になる。不透明度は `PushOpacity`、フィルターは `Filter`（読んだときに一度 paint の形に直し、大きさごとに `Transform` だけ変えた写しを持つ）、clip-path はアルファのマスク（パスを不透明で描く。重なりは source-over なので和は 1 − Π(1 − 被覆) で、今までと同じ。clip-path の clip-path は、マスクの中の入れ子のグループ）、mask は輝度かアルファのマスク（領域は `ClipPath`）。クリップとマスクの両方があると、マスクのグループの中にクリップ（とフィルター）のグループを入れる（掛け算なので順は変わらず、フィルターだけが最初に来る）。図のビューポートで常に切る。グループの入れ子は 24 段まで。
- **CPU の合成は消した：** `paint/svgraster.go`（走査線の被覆、層、グラデーションの評価）と `paint/svgfilterrun.go`（フィルターの実行）。soft も同じグループを自分の効果で描くので、CPU の合成は soft にも要らない。合成した絵の 4 つのキャッシュも無くなった。
- **公開 API：** `SVG.DrawScaled` と `layout.RenderSVG.Scale` を消した（GPU が面の倍率で合成するので要らない）。SVG ウィジェットは `MediaQueryOf(ctx).Scale` を読まなくなった。
- **アイコンの道は変えていない：** グループの無い図は、今までと同じコード（形ごとの `DrawPath`、アトラス、アロケーション 0）。

## 結果

- **Chrome との比較**（`internal/golden` の svgsets、手書きの 13 個、2026-11-14）。soft、D3D11（Iris Xe）、Vulkan（RTX 3050 Ti）で、13 個すべて合格。`GOWAKU_SVGSETS_GPU=d3d11|vulkan` で GPU でも回せるようにした（`-tags "svgsets gpu"`）。

  | ファイル | CPU の合成（前） | soft | D3D11 | Vulkan |
  |---|---|---|---|---|
  | clip-paths | 0.0006 / 0% | 0.0010 / 0.04% | 0.0010 / 0.04% | 0.0010 / 0.04% |
  | filter-blur-shadow | 0.0017 / 0% | 0.0019 / 0.01% | 0.0019 / 0.01% | 0.0019 / 0.01% |
  | filter-color | 0.0011 / 0.02% | 0.0012 / 0.02% | 0.0012 / 0.02% | 0.0012 / 0.02% |
  | mask-opacity | 0.0007 / 0% | 0.0007 / 0% | 0.0006 / 0% | 0.0008 / 0% |
  | text-effects | 0.0030 / 0.39% | 0.0030 / 0.39% | 0.0030 / 0.39% | 0.0030 / 0.39% |

  （平均の差 / 1/4 以上違うピクセル。ほかの 8 個はグループを持たず、前と同じ。）clip-paths とブラーの図がわずかに悪いのは、縁が CPU の 16 本の走査線の正確な被覆から 4 サンプルの MSAA になったため（差の絵では縁だけ）。閾値（0.01、0.5%）には十分な余裕がある。
- **全アイコン**（Lucide と Material Symbols、25962 個、soft）：すべて合格、差の中央値 0.0110、p90 0.0287、p99 0.0401、最大 0.1137。ADR 0112 と同じ（アイコンはグループを持たない）。
- **golden：** `groups`（新規、1281×351、倍率 1.5）：不透明度の分離、下地を読むブレンドの入れ子、アルファと輝度のマスク、全プリミティブ、`RepaintBoundary` の中、パスのクリップでインラインになるレイヤーの中、`Opacity` と回転の下。D3D11 と Vulkan はどちらも最大 2、2 を超える画素 0。`svg-features` は GPU の合成の絵に作り直した（soft）。D3D11 は最大 2・0 画素、Vulkan は最大 3・9 画素（縁、閾値内）。13a の `blends`、`blends2`、`blurs`、`glass-dialog(-dark)`、`shader` は作り直しておらず、前と同じ値で通る（`blends` は D3D11 4・9 画素、Vulkan 5・40 画素で、ADR 0044 の記録と同じ）。
- **アロケーション：** キャンバスのグループの記録、グループのある SVG の `Draw`、`RenderSVG.Paint`、グループとフィルターとマスクのフレームの Encode + Finish がどれも 0（テストあり）。新しい大きさのターゲットとランプの表の行は、初回だけ作る。
- **ベンチマーク：** `BenchmarkEncode_Groups`（render）と `BenchmarkSVG_DrawComposited`（paint）を置いたが、ほかの作業と並んで走っているので測っていない（メインのセッションで測る）。
- **費用の形：** CPU の合成は大きさか色が変わったときだけ走り、あとはキャッシュの画像だった。GPU のグループは、描くフレームごとにジョブが走る（ページに載ったものを除く）。小さな図では軽いが、フィルターの多い図を毎フレーム描き直すアニメーションでは GPU の費用になる。`RepaintBoundary` で包めばページに残る（形がすべてグループの中にある図の場合。グループの外の大きなパスはステンシルなのでページをインラインにする）。

## 残した穴

- **Metal は未検証。** コードと MSL はあるが、このマシンでは走らせていない。
- マルチサンプルのジョブ（パスを含むグループの中身）では、下地を読むブレンドが source-over に、背景のぼかしは描かれない。
- グループの縁の質は MSAA 4 サンプル。CPU の 16 本の走査線より粗い（Chrome との差の縁）。
- 2048 を超えるグループは解像度を落とす。
- キャンバスの間引き（`visible`）はグループの中でもクリップで働くので、フィルターのオフセットでクリップの外から入るはずの形が、記録の時点で落ちることがある（中身がまるごとクリップの外のとき）。
- 回転や歪みの下のフィルターは近似（領域は外接矩形、σ は軸の伸び）。グループの絵は面の空間で描くので、回転そのものは正確。
- 既存のウィジェット（`Blend`、`ImageFiltered`、`Opacity`）のフォールバック（レイヤーが使えないときの命令ごとの合成、インラインでぼかさない）は変えていない。グループに替えれば正しくなるが、既存の golden の絵が変わる（「決めてほしいこと」）。

## 却下した案

- **SVG 専用の GPU の道：** 要望どおり、一般の仕組みにした。
- **13a のレイヤー（ページ）を入れ子にする：** ページは 1 サンプルで共有、描き直しの判断も版に依る。グループは毎フレームの一時的な絵で、性質が違う。
- **CPU で被覆のマスクを作って GPU で合成：** 「GPU を使う」から外れ、大きな図ではアップロードが重い。
- **結果ごとに色空間を持つ：** 8 ビットで linear を保存すると暗いところが粗くなる。sRGB で保存し、プリミティブの中で変換する方がよい。
- **効果のために各バックエンドへパイプラインを足す：** カスタムシェーダーの道で、D3D11 と Vulkan には何も足さずに済んだ。
