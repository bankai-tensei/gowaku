# 0084: sbix のカラー絵文字（Apple Color Emoji）

決定日: 2026-11-10

状態: 採用

## 背景

Phase 17 の残り（ADR 0082 の表の「sbix 絵文字」）。ADR 0024 は COLRv0 / COLRv1 / CBDT までで、sbix は macOS の段階に回していた。

調べると、`text` の側では sbix はすでに通っていた。go-text（v0.3.5）の `Face.GlyphDataBitmap` は sbix を CBDT より先に引き、`SetPpem` に合う strike（要求以上で最小のもの、無ければ最大）の PNG を `font.GlyphBitmap{Format: PNG}` で返す。`Face.GlyphExtents` も同じ strike から、origin offset を含めた箱をフォント単位で返す。CBDT 用に書いた `rasterizeBitmap` がそのまま sbix を描き、`NewSystemShaper` の絵文字のフォールバック（`emojiFamilies` の 3 番目）も `/System/Library/Fonts/Apple Color Emoji.ttc` を見つけていた。

Apple Color Emoji の実データ: upem 800、strike は 20 / 26 / 32 / 40 / 48 / 52 / 64 / 96 / 160 ppem、各グリフは 1em 四方の PNG で origin offset は 0、glyf には (−1, 0)–(799, 800) の矩形がある。つまりベースラインから 1em 上までの正方形で、ascender（800）と一致する。

## 決定

- sbix 専用のコードは足さない。`rasterizeBitmap` が sbix と CBDT の両方を描く。
- **ビットマップはサブピクセル位置で伸ばさない。** これまでは箱を整数ピクセルに丸めた範囲へ縮小していたので、`Subpixel` が 0 以外だと 48px の絵文字が 49px に横に伸びていた（COLR はベクターなので問題にならない）。箱を小数のまま求め（`pixelBox`）、`draw.CatmullRom.Transform` で倍率を保ったまま小数の位置へ置く。strike と同じ大きさで `Subpixel` 0 なら strike の画素がそのまま入る。
- 縮小先は `a.colorScratch` を直接 `image.RGBA` として使い、グリフごとの `image.NewRGBA` をやめた（PNG の復号のアロケーションは残る。アトラスのキャッシュミスのときだけ）。

## 含めなかったもの

- glyf の bbox を origin offset に足す扱い（FreeType の一部の版が行う）。Apple Color Emoji は yMin が 0 なので結果が変わらない。
- sbix の JPEG / TIFF の strike（Apple のフォントは PNG だけ）。`Format != PNG` は輪郭の経路に落ちる。
- `Face.SetPpem` が共有の Face を書き換える点（ADR 0024 の CBDT から同じ）。

## 結果

`text/sbix_test.go`（フォントが無ければ skip）:

- 12 / 16 / 26 / 32 / 48 / 100 / 300px の 😀 🇯🇵 👍🏽 ❤️ 🍣 がカラーで、1em 四方、下端がベースライン。
- 48px（strike あり）・`Subpixel` 0 で strike と画素が一致。`Subpixel` 1〜3 で総被覆が ±0.5% 以内（伸ばすと約 2% 増える）。
- `NewSystemShaper` で「あ😀」「寿司🍣です」「☀️」「🇯🇵」の絵文字が Apple Color Emoji に当たり、カラーになる。
