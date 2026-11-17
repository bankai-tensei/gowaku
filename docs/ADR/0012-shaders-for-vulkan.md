# 0012: Vulkan のシェーダーも HLSL から作り、ソースを一本にする

決定日: 2026-11-07

状態: 採用

## 背景

当初の計画は Linux のシェーダーを「GLSL → SPIR-V（glslc）」としていた。一方 Phase 7 の完了条件は、同じ DisplayList で D3D11 と同じ画像が出ること。SDF の式は 130 行あり、影の近似のように 1 行の違いが見た目に出る。

## 選択肢

1. GLSL を別に書く。当初の計画どおりだが、同じ式を 2 か所で保守する。
2. HLSL を dxc で SPIR-V にする。dxc は Vulkan SDK に入っていて、ADR 0004 で D3D11 に使えなかった dxc がここでは使える。
3. SPIRV-Cross で DXBC 由来の何かを変換する。ツールが増えるだけで得がない。

## 決定

選択肢 2。

- ソースは `gpu/shaders/*.hlsl` の一か所。`internal/cmd/shaderc -target dxbc|spirv` が `gpu/d3d11/shaders/*.dxbc` と `gpu/vulkan/shaders/*.spv` を作る。移動の前後で DXBC はバイト単位で同じ。
- `-fvk-invert-y` で頂点シェーダーの y を反転する。Vulkan のクリップ空間は下向きなので。負のビューポート高さ（1.1 以降）が要らず、Vulkan 1.0 で足りる。
- レジスタは `-fvk-bind-register` で割り当てる。b0 と s0 が set 0、t0 が set 1、t1 が set 2（理由は ADR 0013）。HLSL 側に `[[vk::...]]` は書かない。
- `SV_Position` はピクセル座標のまま `gl_FragCoord` になるので、グリフのテクセル計算も変えずに動く。

## 結果

- 当初の計画の「GLSL → SPIR-V（glslc）」はこの ADR が上書きする。
- シェーダーを変える人は fxc と dxc の両方が要る。`go build` にはどちらも要らない。
- 実測で 28 シーン中 27 シーンが差 1 以内（ほとんど差 0）。残りの 1 シーンは ADR 0013 の 3 ピクセル。
