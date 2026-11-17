# 0003: 既定の GPU は省電力側を選ぶ

決定日: 2026-11-07

状態: 採用

## 背景

開発機は内蔵 GPU（Iris Xe）と外部 GPU（RTX 3050 Ti）を持つハイブリッド構成。Phase 0 の完了条件は両方で動くことで、利用者のノート PC でも同じ構成が多い。どちらを既定にするかを決める必要がある。

## 選択肢

1. OS の既定アダプタ（`EnumAdapters1(0)`）。OS 任せで、機種によって結果が変わる。
2. `IDXGIFactory6::EnumAdapterByGpuPreference` で明示的に選ぶ。既定は省電力。
3. 同じ API で既定を高性能にする。

## 決定

選択肢 2。`gpu.PowerPreference` のゼロ値を `LowPower` とし、`d3d11.Options{Power: gpu.HighPerformance}` で切り替える。

UI はほとんどの時間アイドルで、当初の計画の主用途で省電力が重要とされている。外部 GPU を起こすとアイドルでも電力を使い、デバイス作成も遅い（開発機で内蔵 0.37 秒、外部 2.30 秒。どちらもウィンドウ作成と数フレームの描画を含むテスト全体の時間）。1 万矩形の描画程度なら内蔵 GPU で足りる見込みで、Phase 1 のベンチで確認する。

`IDXGIFactory6` が無い Windows 10 1803 より前では OS の既定に落とす。ソフトウェアアダプタ（WARP）は飛ばし、ハードウェアが無ければ `gpu.ErrNoGPU` を返す。

## 結果

Vulkan バックエンドにも同じ `PowerPreference` を渡す。`VkPhysicalDeviceType` の INTEGRATED / DISCRETE で選ぶ想定。
