# 0020: gpu/soft とピクセル golden

決定日: 2026-11-08

状態: 採用

## 背景

Phase 8 までのピクセルの確認は D3D11 と Vulkan の相互比較だけで、GPU の無い CI では回らなかった。当初の計画の `gpu/soft` を実装し、同じ PNG に全バックエンドを合わせる。

## 決定

### gpu/soft はシェーダーを同じ式で写す

`gpu.Backend` を Go だけで実装した（`unsafe`、`syscall`、purego は使わない）。ピクセルシェーダーの式を float32 で評価し、MSAA はサンプルごとの色、深度レベル、ステンシルを持つ。D3D の標準 4x 位置を使う。

当初の計画は「パスは `x/image/vector`、出力は `*image.RGBA`」としていたが、やめた。GPU はステンシルと 4 サンプルで塗るので、別のラスタライザでは縁の値が合わない。出力は `ReadPixels` と同じ `*image.NRGBA` にした。

実機に合わせて決めた点:

| 項目 | 実機で分かったこと | soft の実装 |
|---|---|---|
| ブレンド | シェーダーの出力は 8 ビットに丸めてから混ざる。float のまま混ぜると、最大 19% のピクセルが 1 レベルずれた | 整数で `src + (dst*(255-a)+127)/255` |
| resolve | 半分は切り下げる | `(sum+1)/4` |
| 丁度 x.5 の丸め | Iris Xe は切り上げ、RTX は切り下げ。両方には合わせられない | Iris Xe に合わせた |

### golden は soft で作り、全バックエンドをそれに合わせる

`internal/golden` が 6 シーン（部品の明暗、段落、リスト、スクロール後のリスト、ダイアログ）を `gowaku.Run` と同じ手順で描く。倍率は 1、1.5、2。フォントは Go フォントだけなので、マシンのフォントに左右されない。

`go test ./internal/golden -update` が `testdata/*.png` を書き直す。絵が変わる変更をしたら、差分の PNG を見てからコミットする。

許容は比較テストと同じ: チャンネル差 2 を超えるピクセルが 0.2% まで、12 を超えるものが 8 個まで。

| バックエンド | 環境 | golden との差（6 シーンの最大） |
|---|---|---|
| soft | Windows、Linux | 0（両 OS でバイト単位で同じ結果） |
| D3D11 | Iris Xe | 1 |
| Vulkan | RTX 3050 Ti | 1 |
| Vulkan | lavapipe（WSL） | 2 |

### CI

`.github/workflows/ci.yml` を置いた。GPU の無いジョブで soft の golden、lavapipe と Xvfb のジョブで Vulkan の golden を回す。

- D3D11 は CI で回らない。gowaku はソフトウェアアダプタを選ばないので、WARP が使えない。テストのためだけに選ばせる変更はしていない。`go test -tags gpu ./internal/golden` をローカルで回す。
- **このワークフローはまだ一度も GitHub で動いていない。** リポジトリをプッシュしていないため。同じコマンドはローカルと WSL で確認した。

### Iris Xe の Vulkan を golden から外した

golden を作って初めて見つかった。Windows の Iris Xe の Vulkan では、影の上に描いた文字とつまみが壊れる（ADR 0013）。Windows の製品経路は D3D11 なので、Windows の golden の Vulkan は RTX で回す。
