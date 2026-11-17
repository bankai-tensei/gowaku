# 0065: テーマの拡張、行の名前、全部品の網羅（15c）

決定日: 2026-11-09

状態: 採用

## 背景

15c の残りは 3 つあった。

- 当初の計画の「`Theme.Extension[T]` で利用者定義のトークン」。
- ADR 0056 に残した「GroupRow のラベルが、隣の欄の名前にならない」。
- Phase 15 の完了条件「gallery が全コンポーネントを網羅。全ウィジェットに Semantics と golden がある」。

## 決定

### 1. テーマの拡張

- Go のメソッドは型引数を取れないので、`Theme.Extension[T]` は関数にした。
  - `theme.WithExtension[T](th, v) *Theme`: v を持つ写しを返す。同じ型の値は置き換え、th は変えない（共有の `Default()` は読み取り専用のまま）。
  - `theme.WithExtensionPair[T](set, light, dark) Set`: ライトとダークに別々の値を持たせる。テーマは明るさを 1 つしか持たないので、拡張もそれに従う。
  - `theme.Extension[T](th) (T, bool)` / `theme.ExtensionOr[T](th, fallback) T`: 読み出す。インターフェースの T は、それを満たす最初の値を返す。
- 置き場は非公開の `extensions []any`。1 つの型に 1 つの値。非公開にしたのは、写して足す以外の方法を無くすため。
- 読み出しは型アサーションで数個をなめるだけで、9.2ns、0 allocs/op。リフレクションは使わない（depguard が core の外で禁じている）。
- テーマを補間で動かす場所が gowaku には無いので、`Lerp` は作っていない。動くテーマができたら、任意の `Lerp(T, float32) T` を尊重すればよい。
- ルートには転送する関数を置いた: `ThemeExtension`、`ThemeExtensionOr`、`ThemeWithExtension`、`ThemeWithExtensionPair`。Go にはジェネリック関数の別名が無いため。
- gallery の「そのほか」の「テーマの拡張」の行で、拡張の色を持つテーマと持たないテーマを並べた。

### 2. GroupRow のラベルが欄の名前になる

- `core.SemanticsBuilder.Caption()` を足した。次に開くノードを「後の `Name` が名付ける部品の、見えている名前」として印を付ける。
  - その部品が名前を受けたとき、またはすでに同じラベルを持つときは、印の付いたノードを木から外す。
  - 違うラベルを持つときは、文字として残す。
- GroupRow は、Label と Child の両方があるときだけ、ラベルの文字を caption で包み、子を PropertyGrid と同じ `propertyName` で包む。
  - これで、ラベルの無い TextField、DropdownMenu、Switch、Autocomplete が、行の名前で 1 回だけ読まれる。
  - 部品が行と同じラベルを持つときも 1 回で済む（gallery の Slider「音量」）。
- ついでに直したもの: Sidebar の題が、文字としてとリストの名前として 2 回読まれていた。題の文字は読ませないようにした。
- 15a で gallery のフォームのページに残っていた「行の中のドロップダウンと候補に名前が無い」は、これで解けた。

### 3. 全部品の網羅

- **点検した**: 全部品について、golden、Semantics のテスト、gallery のページがあるかを一覧にした。
- **golden の無かった部品に足した**（ライトとダーク）。

  | シーン | 部品 |
  |---|---|
  | `coverage-settings` | Group、GroupRow、Sidebar、SegmentedControl、Stepper、ProgressBar、ProgressRing |
  | `coverage-menubar` | 編集のメニューを開いた MenuBar |
  | `coverage-context-menu` | ContextMenu |
  | `coverage-misc` | RichText、DecoratedBox、Grid、FocusRing、AnimatedContainer、AnimatedOpacity |

- **Semantics のテストが無かった部品に足した**: Group、GroupRow、Sidebar、Icon、Image、TitleBar。
- **点検で見つけたバグ。** TitleBar の最小化、最大化、閉じるのボタンは、名前はあるのにタップの操作が無く、スクリーンリーダーから押せなかった。直した。
- **対象外にしたもの。** それぞれ理由がある。
  - レイアウト: Row、Padding など。
  - 変形
  - 効果: Opacity、Blend など。既存の golden がすでに描いている。
  - 入力だけの部品: GestureDetector など。
  - 木の配管: Builder、Theme など。
  - データの型
  - Pressable: 見た目を利用者が作る。Semantics のテストはある。
- **gallery** に「そのほか」と「ナビゲーション」のページを足し、ほかのページに出てこない部品を全部載せた。
  - 「そのほか」に載せたもの: MenuBar、ContextMenu、ProgressBar / Ring、Stepper、単独の Radio、RichText（ルビ付き）、SVG、テーマの拡張、AnimatedOpacity、Grid、1 万行の ListView、Calendar。
  - TitleBar は `CustomFrame` の窓が要るので、gallery ではなく `examples/menus` で見る。
- **見せ方を直したもの。** gallery の UIA で、ProgressBar の値が「30.0000019」と読まれた。float32 の 0.3 を float64 にしたためで、0.01% 単位に丸めた。

## 結果

- Phase 15 の完了条件を満たした。
  - gallery は全部品を網羅した（TitleBar だけは例のプログラムで見る）。
  - DataTable は、100 万行でスクロール 1 段 4.9µs（ADR 0058）。
  - 全部品に Semantics のテストと golden がある（対象外は上のとおり）。
- 残り:
  - プレースホルダーしか持たない TextField / Autocomplete は、プレースホルダーが名前になり、行のラベルが別に読まれる（ARIA ならラベルが勝つ）。
  - Label を持つ TextField は、見えているラベルと Semantics で 2 回読まれる（15c より前からのふるまい）。
  - TitleBar のピクセルの golden（本物の窓の枠の状態が要る）。
