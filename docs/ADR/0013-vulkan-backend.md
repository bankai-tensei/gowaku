# 0013: gpu/vulkan の構成

決定日: 2026-11-07

状態: 採用

## 背景

`gpu.Backend` は Direct3D 11 に合わせて育った。Vulkan には暗黙の同期も、状態オブジェクトの組み合わせも、自動のリソース寿命管理もない。cgo も使わない。

## 決定

### Windows でも動かす

`gpu/vulkan` は Linux 専用にせず、Windows でも `vulkan-1.dll` と `VK_KHR_win32_surface` で動かす。出荷物の Windows 版は D3D11 のまま。

理由: Linux の検証機が CPU 実装の lavapipe しかなくても、同じプロセスの中で D3D11 と Vulkan を実 GPU（Iris Xe、RTX 3050 Ti）で比べられる。`gpu/parity_gpu_windows_test.go` がそれをする。増えるコードは 60 行。

### 呼び出し: `call` と `hot`

`purego.SyscallN` は可変長引数が heap に逃げ、1 回の呼び出しで 1 回割り当てる。実測で 1 フレーム 16 回だった。毎フレームの経路は `Device.hot` を通す。引数を `Device` のフィールドの配列へコピーしてから呼ぶので、呼び出し側のスライスはスタックに残る。代わりに渡すポインタは `Device` / `Surface` のフィールドを指すものに限る。初期化の経路は `//go:uintptrescapes` 付きの `call` のままでよい。`TestFrame_DoesNotAllocate` が 0 を確認する。

`ptr` は渡すポインタを `kept` のリングに入れる。`go:uintptrescapes` が効くのは呼び出しの式に直接書いた `uintptr(unsafe.Pointer(p))` だけで、`ptr` の中の変換は対象にならない。放っておくと壊れ方が 2 つある。スタックに残った出力引数は、スタックが伸びて移動すると C が古い場所へ書く（`vulkan: no suitable memory type`）。`uintptr` にした後の構造体は Go から見て参照が切れるので、C の呼び出し中に GC が回収する（`vkCreateInstance` の失敗と SIGSEGV）。リングに入れると heap へ逃げ、後の呼び出しに押し出されるまで生きる。どちらも Phase 8 と最適化の検証で発見した。`platform/x11` の `ptr` も同じ。

### フレームとメモリ

- 飛行中のフレームは 2。`frame` がフェンス、コマンドバッファ、定数バッファ、インスタンス・頂点・ステージングのアリーナを持つ。ウィンドウごとではなくデバイスに 2 個。
- バッファは host-visible + coherent で、常時 map したまま書く。足りなければ大きいバッファへ移り、古いものは `retire` する。
- `retire` は「いま記録中のフレーム、無ければ最後に送信したフレーム」に積む。キューは順に流れるので、そのフェンスがそれ以前の全フレームを保証する。単に次のスロットへ積むと、直前のフレームの完了を待たずに解放してしまう。検証レイヤーの `VUID-vkDestroyImage-image-01000` で見つかった。

### アップロード

`CreateTexture` と `UpdateTexture` は、次のフレームと同じコマンドバッファのレンダーパスの前に記録する。レンダーパスの中ではコピーできないので、フレームの途中で呼ぶと panic する。`render` はもともと `BeginFrame` より前に済ませている。`gpu.Backend` の契約をこれに合わせて狭めた。

### ディスクリプタ

set 0 がフレーム（定数と不変サンプラー）、set 1 が DrawCall の Texture、set 2 が Image。set 1 と 2 は同じレイアウトなので、テクスチャはディスクリプタセットを 1 個だけ作れば、どちらのスロットにも束縛できる。毎フレームの `vkUpdateDescriptorSets` が無い。プールは 256 個ずつ足す。名前の無いスロットには 1×1 の `blank` を束縛する。

### パイプライン

D3D11 の「ブレンド × 深度ステンシル × シェーダー」の組み合わせのうち実際に使う 6 通りを `pipelineKind` として最初のサーフェスで作る。ビューポートとシザーは動的状態。クリップレベルの深度はビューポートの min = max = level/256 で、D3D11 と同じ。深度形式は `D24_UNORM_S8_UINT`、無ければ `D32_SFLOAT_S8_UINT`（Vulkan が保証するのはどちらか片方だけ）。level/256 は両方で正確に表せる。

### スワップチェーン

`OUT_OF_DATE` と `SUBOPTIMAL` は `stale` にして次の `BeginFrame` で作り直す。セマフォは acquire 用を画像数 + 1 個回し、render finished 用を画像ごとに持つ。形式は `B8G8R8A8_UNORM`、次に `R8G8B8A8_UNORM`。どちらも sRGB ではない（合成は sRGB のまま。ADR 0005）。

### デバイス選択

`LowPower` は統合、`HighPerformance` は専用 GPU を優先し、CPU 実装は最後の手段として認める。ADR 0003 の D3D11 はソフトウェアアダプタを除外するが、Linux の仮想マシンでは lavapipe が無いとウィンドウが出ない。

## 許容する差

D3D11 と Vulkan は実 GPU で次の結果だった（7 シーン × 2 倍率 × 2 GPU）。

- NVIDIA: 28 中すべて差 1 以内。
- Intel: 1 シーン（`clips@1.5`）のクリップ三角形の頂点近くで 3 ピクセルが最大 49 違う。ほかは差 0。
- Intel の Vulkan: 形のあるクリップを含む同じフレームを繰り返し描くと、約 2% のフレームでクリップの角の 60 ピクセルほどが抜ける。1500 フレームのうち、Iris Xe の Vulkan は 30 回、Iris Xe の D3D11 と RTX 3050 Ti の Vulkan は 0 回。負荷の有無で変わらず、同期の検証レイヤーを有効にする環境変数を付けた実行でも報告は無かった（有効になったことの確認はしていない）。原因は特定していない。製品の組み合わせは Windows が D3D11、Linux が Vulkan で、Windows の Intel ドライバの Vulkan は比較テストにしか使わない。Linux の Intel（Mesa）は別のドライバで、未確認。比較テストは 1 回だけ描き直す。
- Intel の Vulkan（2026-11-08 追記）: golden（ADR 0020）で別の症状が見つかった。影を持つ部品（ボタン、スイッチのつまみ）の上に描いた文字と図形が、影の後の同じ描画で大きく壊れる。6 シーン中 5 シーンで 5000〜25000 ピクセルが許容を超えた。影の無い段落のシーンは差 0。RTX 3050 Ti の Vulkan と lavapipe は全シーン差 2 以内で、同じ SPIR-V なのでドライバ側を疑っているが、原因は確かめていない。上の 2% の症状と同じ原因かも分からない。Windows の golden の Vulkan は RTX で回す。

三角形の辺のちょうど上に乗るサンプルの扱いは、D3D11 が規定し、Vulkan は実装に任せている。テストは 8 ピクセルまでを許す（`strayPixels`）。

## 結果

- `go test -tags gpu ./gpu/...` に比較テストが入った。
- 検証レイヤーは `VK_LOADER_LAYERS_ENABLE='*validation*'` で有効になる。全テストで VUID 0 件。
