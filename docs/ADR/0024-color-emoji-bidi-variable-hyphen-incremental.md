# 0024: カラー絵文字、BiDi、可変フォント、ハイフネーション、増分レイアウト（Phase 11b）

決定日: 2026-11-08

状態: 採用

## 背景

Phase 11 の後半（ADR 0023 で分けた 11b）。ヒアリングで次を決めた。

- 絵文字は COLRv0 / COLRv1 / CBDT。sbix は macOS の Phase 12。Linux の CBDT は Ubuntu の `fonts-noto-color-emoji` を WSL で取得して確かめる（リポジトリには入れない）。
- BiDi の TextField の矢印キーは視覚順。
- ハイフネーションは英語だけ同梱（hyph-utf8 の hyph-en-us）。既定はオフ。
- 見本: BiDi は Word と、絵文字は Chrome と比べる。→ Word は使えなかった（下記）ので、BiDi も Chrome と比べた。

## 決定

### 公開 API（追加のみ）

```go
text.Style{
    Direction:  text.DirectionAuto, // 既定。各ブロックの最初の強い文字で決める（UAX #9 P2）
    Variations: [4]text.Variation{{Axis: "wdth", Value: 75}},
    Hyphenate:  true,
}
text.DirectionLTR, text.DirectionRTL
(*text.Layout).Direction(), VisualMove(offset, dir)
(*text.Shaper).Reshape(old, content, style), (*text.Paragraph).Relayout(old, maxLength)
text.Atlas: ColorPageCount, TakeColorDirty, GlyphImage.Color
text/hyphen: English(), Parse, (*Patterns).Points / Hyphenate
jlreq.Unit.Hyphen, jlreq.Line.Hyphen
```

`Style` を比較可能に保つため、`Variations` は固定長の配列にした。

### カラー絵文字

- **フォントの選び方:** 書記素クラスタが絵文字として表示されるもの（Emoji_Presentation の文字、VS16 付き、国旗、キーキャップ）を見分け、その範囲だけ `Segoe UI Emoji`、`Noto Color Emoji`、`Apple Color Emoji` を先に引く。fontscan に任せると Segoe UI Symbol（単色）や Meiryo が先に当たる。VS15 は文字表示のまま。Emoji_Presentation の表は UTS #51 から BMP の散在する文字と U+1F000 以降のブロックを抜き書きした（go-text の表は internal で使えない）。
- **ラスタライズ:** go-text は COLR のペイントグラフと CPAL、CBDT の PNG を読むが、描かない。COLR のペイントグラフ（レイヤー、PaintGlyph のクリップ、単色、線形・放射・スイープのグラデーション、変換、PaintColrGlyph、PaintComposite の Porter-Duff と分離可能なブレンド）を CPU で premultiplied の float に描く（`text/colr.go`）。非分離ブレンド（hue など）は source over に退化。可変の Var* は既定のインスタンスで描く。前景色（パレット 0xFFFF）はアトラスでは文字色が分からないので黒。
- CBDT は ppem を合わせて PNG を取り出し、`GlyphExtents` の箱に Catmull-Rom で縮小する。
- **アトラス:** RGBA のカラーページを別に持つ（最大 2 枚、32MB）。描画は既存の `InstanceImage` でページのテクスチャを読む。**シェーダーの変更なし**で D3D11 / Vulkan / soft に出る。色は絵文字自身のもので、文字色からは不透明度だけ取る。

### BiDi

- 埋め込みレベルは **go-text の `bidi` パッケージ**（公開 API でレベルを返す）。`x/text/unicode/bidi` はレベルを返さず、`Line` も方向を自動にしてしまうので使わない。新しい依存は無い。
- 方向はブロック（改行で区切った段落）ごと。`DirectionAuto` は最初の強い文字（`x/text/unicode/bidi.LookupRune`）。RTL の文字が無く LTR なら何もしない（レベルを持たない）ので、既存の文章のコストは増えない。
- シェイピングは RTL の run を RTL で HarfBuzz に渡す（アラビア文字の連結と括弧のミラー）。出力は視覚順なので逆から読んで論理順のクラスタにする。
- 行分割は論理順のまま jlreq。行ごとに UAX #9 の **L1**（行末の空白を段落のレベルに）と **L2**（最高レベルから最低の奇数レベルまで反転）で並べ替え、各クラスタは jlreq が与えた幅（後ろの空きを含む）を持って移動する。
- RTL の段落は右揃えが始端。行の長さが有限なら、レイアウトの幅を行の長さにする（最長行の幅ではなく）。LTR の既存の挙動は変えない。単一行の TextField は右に寄せる。
- キャレット: RTL のクラスタの手前は右端。ヒットテストは端の近い方。選択は画面上でつながる範囲ごとの矩形。`VisualMove` は画面上の左右で、行の端を越えると論理順で次の行へ。

### 可変フォント

- `Weight` は `wght`、`Size` は `opsz`（CSS の font-optical-sizing: auto と同じ）に自動で当て、`Variations` が最後に勝つ。持たない軸は無視される。
- go-text の `Face.SetVariations` は Face の座標を変える。fontscan の Face は共有なので、**軸の組ごとに Face を作り直す**。HarfBuzz のシェイパーはフォントファイル単位でキャッシュするので、別の座標の Face と混ざらないよう、組ごとにシェイパーも分けた。可変でないフォントは一度調べて覚える。

### ハイフネーション

- `text/hyphen`: Liang のアルゴリズムと TeX のパターン（標準ライブラリだけ、jlreq と同じくフォント無しでテストできる）。パターンは hyph-utf8 の hyph-en-us（Kuiken、Knuth の hyphen.tex に追加。自由配布、著作権表示は `patterns/hyph-en-us.license.txt`）。左 2 字、右 3 字。
- 段落は西欧文字の単語にハイフン位置を付け、`jlreq.Unit.Hyphen` にハイフンの幅を入れる。Breaker は、最後の空白より後ろでハイフンを足しても行に収まる位置があればそこで切り、`Line.Hyphen` を立てる。幅にはハイフンを含める（両端揃えがその幅で揃えるため）。ハイフンは描くときに行末に足し、テキストには入れない。
- パッケージ変数は `sync.OnceValue` で、初回に一度パターンを解析する（書き換えない）。

### 増分レイアウト（10 万字の TextField）

- `Shaper.Reshape(old, content, style)`: 前後の共通部分から変わったブロックを求め、その範囲だけを別の Paragraph に整形して配列に差し込む。後ろのブロックのオフセットとグリフ番号をずらす。スパンが 1 つでルビの無い段落だけ。それ以外は全部整形し直す（ADR 0074 で、ルビを持つスパンの段落にも広げた: `ReshapeSpans`）。
- `Paragraph.Relayout(old, maxLength)`: 変わったブロックの行だけ分割し直し、`xs` / `adv` / グリフ / スパンを差し込み、後ろの行を `top` だけずらす。最長行が変わって揃えが動くとき（中央・右揃え、RTL）は全部やり直す。
- 古い Paragraph と Layout はその場で書き換えて返す（呼んだ側は古いものを使わない）。TextField は古いレイアウトを `stale` として残し、次の Layout で `Relayout` に渡す。
- 正しさは、ランダムな 300 回の編集（文字、改行、アラビア文字、削除）の後ごとに、新しく整形した結果と行・位置・グリフ・ランが一致することで確かめた（横組、縦組、中央揃え）。

## 含めなかったもの

- sbix（Phase 12）、SVG 絵文字、COLR の可変（Var*）の差分。
- BiDi の明示的な埋め込み制御文字の表示、RTL のレイアウトのミラーリング（当初の計画で対象外）。縦書きの BiDi（縦組は論理順）。
- 英語以外のハイフネーションのパターン（`hyphen.Parse` で利用者が読める。段落から差し替える口はまだ無い）。
- 増分レイアウトの O(n) の残り: 差し込みで後ろの配列を詰め直すメモリ移動と、後ろのクラスタのオフセットのずらし。ブロックごとに配列を持つ構造にすれば消せるが、Paragraph の全体に及ぶ。

## 結果

- BiDi: 9 行の混在文で、画面上の文字の並びが Chrome（`<p dir="auto">`、ICU）と全行一致した（`text/testdata/bidi_chrome.txt`）。Word は `Information(wdHorizontalPositionRelativeToPage)` が RTL の段落で同じ行の中でも一貫しない値を返し、比較に使えなかった。
- 絵文字: Segoe UI Emoji（COLRv1、Fluent のグラデーション）と Noto Color Emoji（CBDT）の両方がカラーで出る。Chrome も同じ字形を選び、Windows では 🇯🇵 が「JP」になる点も同じ。
- 1 打鍵: テキスト層（日本語 10 万字、中ほどの行を編集）0.86ms（全部やり直すと 64ms）。TextField のフレーム全体（ビルド・レイアウト・描画、先頭近くを編集する最悪の位置）1.9〜2.2ms。
- 描画のホットパス（Encode）は 0 allocs のまま。
