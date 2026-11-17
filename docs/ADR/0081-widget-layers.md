# 0081: widgets を意味の層に分け、ルートの窓口を絞る

決定日: 2026-11-10

状態: 採用

## 背景

Phase 16 までで `widgets` は 195 ファイル・約 4.9 万行の 1 パッケージになった。ルートの `gowaku` は 1 import で UI を書けるように 284 個を再エクスポートし、`phase15b_table_aliases.go` のようにフェーズの名前が付いたファイルが並んでいた。ファイル名が構造ではなく歴史を表していて、どこに何があるか、何が何に依っているかが読めない。

非公開のうちに、API とソースを意味の単位で整理する。ヒアリングで次を決めた。

- `widgets` は全部カテゴリに分ける。層は多段にするが、細かくしすぎない。
- ルートの窓口は絞る。部品は各サブパッケージから import してもらう。
- 名前の不揃いは直す（互換は気にしない）。

## 決定

### 1. widgets を 6 段・8 パッケージに分ける

ファイル単位の依存（go/types で取った参照）から、循環しない分け方を作った。上の段は下の段だけを import する。同じ段の間も import しない（規約 §3.1 と同じ）。

| 段 | パッケージ | 中身 |
|---|---|---|
| 1 | `widgets/basic` | テーマ・メディア・文字列（`Theme`, `MediaQuery`, `Localizations`, `Strings`）、箱と並べ（`Container`, `Padding`, `SizedBox`, `Align`, `Column`/`Row`, `Stack`, `Wrap`, `Grid`, `CustomLayout`, `Builder`, `Divider`）、効果と変形（`Opacity`, `Blend`, `BackdropFilter`, `ShaderMask`, `Transform`, `Translate`）、暗黙アニメーション（`Animated*`）、文字と画像（`Text`, `RichText`, `Icon`, `Image`, `SVG`）、`Scroll` |
| 2 | `widgets/interact` | 入力を受ける側: `GestureDetector`, `MouseRegion`, `Listener`, `Focus`, `FocusRing`, `Pressable`, `Shortcut`/`Action`, `Draggable`/`DragTarget`, `Semantics` |
| 3 | `widgets/control` | 自前の見た目を持つコントロール: `Button`, `Checkbox`, `Switch`, `Slider`, `SegmentedControl`, `Radio`, `Chip`, `Card`, `Stepper`, `Tabs`, `SplitPane`, `Badge`, `Progress*` |
| 4 | `widgets/overlay` | ページの上に出るもの: `Overlay`, `Dialog`, `Sheet`, `Toast`, `Tooltip`、メニュー（`ShowPopupMenu`, `MenuBar`, `ContextMenu`, `PopupList`） |
| 5 | `widgets/field` | 打つ・選ぶで値をもらうもの: テキストフィールド（ルビ込み）、検索、フォーム、オートコンプリート、ドロップダウン、カレンダー、日付と時刻のピッカー |
| 5 | `widgets/nav` | 画面と場所を移るもの: `Navigator` と経路、`Hero`、戻るボタン、パンくず、タブバー、ツールバー |
| 6 | `widgets/collection` | 多くの項目を見せるもの: リスト、ツリー、表、プロパティグリッド |
| 6 | `widgets/editor` | 長い文書のエディタ |

共有する中身は外に出さない。

- `widgets/internal/kit`（段 0）: コントロールの時間、undo の履歴、タイプアヘッド、小さなグリフ、`Rebuild` などの補助。widgets のどのパッケージも import しない。
- `widgets/internal/behavior`（`interact` と `control` の間）: コントロールが共有するホバー・フォーカス・押下の振る舞い `Control`。`Focus` と `GestureDetector` を組み合わせるので `interact` より上にある。
- `widgets/internal/wtest`: 複数のパッケージのテストが使う補助。

`widgets` 自体はコードを持たず、doc.go に層の図だけを置く。

分け方の判断:

- `Scroll` は表・ツリー・メニュー・エディタのすべてが使うので一番下に置いた。ホイールを受けるが、`interact` に置くと `basic` の効果やアニメーションと循環する。
- ツールバーはあふれたボタンをメニューで出すので `control` ではなく `nav` に置いた（`control` はメニューより下）。
- 日付の補助のうち、`nav` と `collection` も使う山形のパスは `kit` に降ろし、`nav` が `field` に依らないようにした。
- `ContextMenu` と `MenuBar` は `overlay` に置いた。ツールチップがメニューの位置の計算を、オーバーレイがトーストとツールチップの置き場を持つので、分けると循環する。

`.golangci.yml` の depguard に段ごとの規則を足し、上や横の import を lint で止める。

### 2. ルートの窓口を絞る

ルートの `gowaku` は、アプリとウィンドウの入り口だけにする。

- 残す: `App`, `Run`, `ThemeMode`, `WindowOptions`, `OpenWindow`, `WindowHandle`, `WindowOf`, `ShowModal`, `Post`, `Quit`、トレイ・タイトルバー・クリップボード・ファイルダイアログ・Inspector、起動とフォント。どのアプリも書く `Widget`, `BuildContext`, `State`, `StateBase`, `StateOf`, `StatelessBase`, `StatefulBase`, `Key`, `Keyed` の別名。
- 消す: ウィジェット、配置の定数、幾何・描画・曲線の型、`VStack` などの補助、`ThemeOf`、`ShowDialog` の再エクスポート。フェーズの名前の付いた `*_aliases.go` は全部消す。

UI のファイルは、使う層のパッケージを import する。`basic` だけで足りるファイルが多い。

ADR 0016 の「ルートだけで書ける」は取り下げる。表の `widgets` は、上の 8 パッケージに置き換える。

### 3. 名前の付け方

- パッケージ名を識別子で繰り返さない（規約 §3.2）。
- 分けたときに道具が外に出した、パッケージをまたいで使うだけの補助（`ShadowsOf`, `GlobalRect` など）は公開しない。同じパッケージだけで使うものは小文字に戻し、複数のパッケージが使うものは `internal/kit` に移す。

改名の対応表は下の「改名」に置く。

## 結果

- `widgets` の 1 パッケージにあったテストは、テストする部品の層に移った。上の層の部品を使うテストは上の層に置く（下の層のテストが上を import すると循環する）。
- アプリの import は増える。代わりに、どのファイルがどの層を使うかが import で読める。
- 公開 API は v1 のタグの前に大きく変わる。サンプル、カタログ、README、当初の計画のコード例を合わせて直した。

## 改名

パッケージ名を繰り返していた型と、意味が紛らわしかった名前:

| 前 | 後 |
|---|---|
| `widgets.DataTable` / `DataColumn` / `DataTableController` | `collection.Table` / `TableColumn` / `TableController` |
| `widgets.ListView` | `collection.List` |
| `widgets.TreeView` | `collection.Tree` |
| `widgets.TextEditor` / `TextEditorController` | `editor.Editor` / `editor.Controller` |
| `widgets.EditorDecoration` / `EditorGutterLine` / `EditorUnderline` | `editor.Decoration` / `GutterLine` / `Underline` |
| `widgets.TextField` / `TextFieldController` | `field.Input` / `field.InputController` |
| `widgets.SearchField` | `field.Search` |
| `widgets.FormField` | `field.FormItem` |
| `widgets.OverlayEntry` / `OverlayState` / `OverlayOf` | `overlay.Entry` / `overlay.Host` / `overlay.Of` |

- `field.Text` にしなかったのは、表示だけの `basic.Text` と並ぶと区別がつかないから。`overlay.State` にしなかったのは、`core.State`（ウィジェットの状態）と読み違えるから。
- `basic.Theme` と `theme.Theme`、`basic.Align` と `text.Align` は分ける前から同じ関係なので、そのままにした。

分けたときに道具が公開した補助の扱い:

- 同じパッケージだけで使うもの（`reduceMotion`、`openMenu`、`weekStartOf` など、未公開の型のメソッド約 60）は小文字に戻した。
- 複数のパッケージが使う補助（区切り線の太さ、`ShadowsOf`、`GlobalRect`、`Popup` と `PopupLayer`、`ModalScope`、`Barrier`、ニーモニックの解析、`SplitHandle`、`PaneSlot`、キャレットの寸法など）は `internal/kit` に移した。`kit` には補助だけでなく、層をまたいで共有する小さな RenderObject の部品も入る。
- 自作のウィジェットの部品として公開のまま残したもの: `basic.ScrollState` / `Scroller` / `ScrollFrame` / `Viewport`（リスト・エディタ・ポップアップが埋め込む）、`basic.Localized` / `LookupStrings` / `CarryLocalizations`、`overlay.PopupList` / `PopupWidth`、`field.FindEditable` と `RenderEditable` の選択範囲。
- 未公開の型が署名に出ないよう、`overlay.ShowMenuBelow`、`Scroller.ShiftGlide`、`ScrollState.ShowBar` を足した。`FieldExtras` は `field.Extras` にした。
