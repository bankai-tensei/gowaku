# 0004: HLSL は fxc でコンパイルし、DXBC をリポジトリに入れる

決定日: 2026-11-07

状態: 採用

## 背景

当初の計画は「fxc / dxc で事前コンパイル」としていた。開発機には Vulkan SDK の dxc が PATH にあり、fxc は Windows SDK の中にあって PATH に無い。

## 選択肢

1. dxc。ただし出力は DXIL で、Direct3D 11 は読めない。dxc は Shader Model 6 以降専用。
2. fxc。Shader Model 5 の DXBC を出せる。Windows SDK に同梱。
3. 実行時に `d3dcompiler_47.dll` でコンパイルする。起動が遅くなり、シェーダーの誤りが実行時まで分からない。

## 決定

選択肢 2。`go generate ./gpu/...` が `internal/cmd/shaderc` を実行し、PATH か最新の Windows SDK から fxc を探す。`x.vs.hlsl` は `vs_5_0`、`x.ps.hlsl` は `ps_5_0` で `x.vs.dxbc` / `x.ps.dxbc` になる。

## 結果

ソースは ADR 0012 で `gpu/shaders/` に移った。コンパイル済みの `.dxbc` を入れるので、`go build` に Windows SDK は要らない。SDK が要るのはシェーダーを変更する人だけ。
