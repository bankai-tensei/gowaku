# 0086: FMA の融合とアーキテクチャ間で同じ丸め

決定日: 2026-11-10

状態: 採用

## 背景

macOS arm64 で `TestGolden_CPU` の `vertical` と `svg-icons` が落ちた（svg-icons は最大差 63、2 を超える画素が約 670。vertical は最大差 54、約 93）。golden は amd64 で作ってあり、同じマシンでも `GOARCH=amd64`（Rosetta）なら通る。システムフォントは使わない（同梱の goregular だけ）ので、差は計算そのもの。

Go の仕様は `x*y + z` を一つの融合演算（FMA）にまとめることを許す。arm64 のコンパイラは `FMADDS` / `FNMSUBS` などへ融合し、amd64 は（GOAMD64=v1 では）融合しない。融合すると積が丸められずに足されるので、結果が最後の 1 ビットずれる。ずれが `Floor` や整数への変換の境目にかかると、画素やサブピクセルの段が変わる。仕様上、明示的な型変換 `float32(x*y)` はその型の精度へ丸めるので融合を止める（同じ型への変換でも効く）。

原因の場所は推測でなく `-d=fmahash` で二分探索して決めた:

```
go test -gcflags=all=-d=fmahash=n ...   # 融合を全部止めると arm64 でも通る
go run golang.org/x/tools/cmd/bisect@latest -compile=fma go test -run 'TestGolden_CPU/vertical$' ./internal/golden/
```

見つかったのはそれぞれ 1 か所だけだった。

- `render/glyphs.go` の `encodeRun`: `x := (Origin.X + gl.X*zoom) * scale` と `step := uint8((x - pixel) * SubpixelSteps)`。`x - pixel` が `x` の積と融合し、丸める前の `x` から段を求めていた。4 分の 1 画素の境目にいるグリフ（vertical の両端揃えの段落の 1 文字）で段が変わり、別のビットマップが出た。
- `paint/svgdoc.go` の `SVG.Draw`: `off = dst + (dst.W - vb.W*sx) * align`。`vb.W*sx` がちょうど `dst.W` に丸まる場合（例: 3 × (10/3) = 10）、融合すると差が 0 でなく 1.2e-7 になり、アイコン全体が 0 でない小さな位置へ動く。パスのアトラスと被覆の計算では小数の位置がそのまま効くので、輪郭の画素が変わった。

## 決定

- **画素の判断（`Floor` / `Ceil` / 整数変換 / 比較による分岐）へ流れる積は、明示的に `float32(...)`（`float64` の式なら `float64(...)`）で丸める。** 書く場所には「arm64 での FMA の融合を止め、どのアーキテクチャでも同じに丸めるため（ADR 0086）」と短いコメントを付ける。
- 直したのは golden が捕まえた 2 か所と、同じ式の中で同じ形をしている隣（`encodeRun` の `gl.X*zoom` / `gl.Y*zoom` / `y` の `*scale + 0.5`、`SVG.Draw` のクリップ判定の `drawn`）。amd64 では元から融合しないので、amd64 の結果は 1 ビットも変わらない。
- コード全体へ機械的に変換を撒くことはしない。読みにくくなるうえ、多くの場所は差が画素に届かない。golden が arm64 でも走るので、新しい場所は golden が見つけ、上の bisect の手順で 1 行まで絞れる。

## 結果

- `go test -run TestGolden_CPU ./internal/golden/` が arm64 でそのまま通り、`GOARCH=amd64` でも通る。
- `paint/svgdoc_test.go` の `TestSVG_DrawOffsetRoundsAlikeOnEveryArch` が 1 か所を固定する: 1 × 3 の viewBox を 10 × 10 に置くと Y は厳密に 0。変換を外すと arm64 で 1.1920929e-07 になって落ちる。
