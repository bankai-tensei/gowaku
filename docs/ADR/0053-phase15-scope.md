# 0053: Phase 15 の範囲と分け方

決定日: 2026-11-09

状態: 採用

## 背景

ウィジェット拡充（Phase 15、目安 6 週）は、当初の計画の「標準の残り」「デスクトップ向け」「ナビゲーション」「フォーム」「テーマ拡張」「ローカライズ」を全部含む。完了条件は「gallery が全コンポーネントを網羅。`DataTable` 100 万行が 120fps。全ウィジェットに Semantics と golden がある」。計画の判断点は「デスクトップ向けウィジェットの範囲（notes サンプルに要るものを優先）」。ヒアリングで次を決めた。

## 決定

### 1. 3 回に分けてレビューする

| | 内容 |
|---|---|
| **15a** | 標準の残り: Toolbar、Tabs、Chip、Card、Tooltip、Toast、Badge、Divider、Radio、Sheet、SearchField、DatePicker / TimePicker。フォーム: `Form` / `FormField` / バリデーション、`Autocomplete`、`DropdownMenu` |
| **15b** | デスクトップ向け: `DataTable`、`TreeView`、`SplitPane`、閉じられる `TabBar`、汎用の編集面（§3）、`PropertyGrid`、`Breadcrumb` |
| **15c** | `Navigator` + `PageRoute` の遷移 + `Hero`、`Theme.Extension[T]`、ローカライズ、gallery で全部を網羅 |

Menu と Progress（線形・円形）は Phase 14a と Phase 13 までにあるので、15a では作らない。

### 2. 各部品の範囲

- **DataTable**: 仮想化、列のリサイズ、ソートに加えて、行の複数選択（Shift / Ctrl、キーボード）、その場のセル編集（Enter / F2、確定と取消は利用者のコールバック）、列の並べ替えと左端の固定列、ヘッダーからの絞り込みの UI。100 万行で 120fps。
- **TreeView**: 開閉、キーボード操作、仮想化に加えて、子の遅延読み込み（読み込み中の表示つき）、ドラッグでの並べ替え（Phase 14 の `Draggable` / `DragTarget` の上で）、複数選択、F2 での名前変更。
- **TabBar**: 閉じる、中クリックで閉じる、ドラッグで並べ替え、溢れたらスクロール + 一覧メニュー、Ctrl+Tab に加えて、**タブを外へドラッグすると新しいウィンドウになり、別のウィンドウの TabBar に戻せる**（ブラウザと同じ）。
- **Navigator**: Push / Pop / Replace のスタックと、`"/notes/42"` のような名前付きルート。戻るは Alt+← とマウスの戻るボタン。遷移は控えめなスライド + フェード。
- **DatePicker / TimePicker**: 作る。週の始まり、月と曜日の名前、12 / 24 時間制はロケールから。
- **自前の `FileDialog` ウィジェット**: 作らない。ポータルの無い Linux は `ErrNoFileDialog` のまま。v3 の候補に移す。
- **フォームのバリデーション**: フォーカスを外したときと送信時に検証する。一度エラーを出した欄は、打鍵ごとに検証し直してエラーを消す。入力中の欄を先回りして赤くしない。
- **ローカライズ**: 日本語と英語の文言を同梱し、OS のロケールを既定に `MediaQuery.Locale` で上書きできる。利用者は文言表を差し替えて言語を足せる。日付と数値の書式は `x/text`。

### 3. CodeEditor はやめ、汎用の編集面にする

当初の計画の「`CodeEditor` 基盤（等幅・行番号・10 万行）」は、コードエディターでしか使わない機能を持たない形に変える。ヒアリングで「他のアプリでも使えそうな機能だけに絞る」と決めた。

- 作る: 10 万行の仮想化された複数行の編集、検索と置換、範囲にスタイルを載せるフック（検索結果の強調、スペルチェックの波線、構文の色付けのどれにも使える）、行の横に飾りを置くスロット（行番号を出すのは利用者）、等幅にするかどうかはフォントで決める。
- 作らない: 括弧の対応、インデントガイド、折りたたみ、複数カーソル、ミニマップ。

名前は 15b で決める（`TextEditor` を第一候補にする）。

### 4. 検証

- 全ウィジェットに `gpu/soft` のピクセル golden（ライトとダーク）と、Semantics の単体テストを付ける。
- スクリーンリーダーの実機確認は、新しいロールを持つもの（DataTable、TreeView、TabBar、DatePicker）だけにする。Narrator / NVDA は各段で、Orca は 15c でまとめて。

### 5. 15a の共通の足場

並行して作れるように、部品より先に次を入れた。

- **ロール**: `RoleToolbar`、`RoleTooltip`、`RoleComboBox`、`RoleGrid`、`RoleRow`、`RoleGridCell`、`RoleColumnHeader`。UIA は Chromium の対応（行とセルは `DataItem`、列見出しは `HeaderItem`）、AT-SPI は GTK 4 の対応（grid は `table`）に合わせた。Grid / Table パターンの実装は 15b の DataTable で入れる。
- **ロケール**: `MediaQueryData.Locale`（`language.Tag`、ゼロ値は英語）と、標準ウィジェットの文言の表 `localStrings`（日英、非公開）。15c で OS のロケールにつなぎ、利用者が言語を足せる公開の表にする。
- **golden の置き場**: 部品のまとまりごとに `internal/golden/scenes_*.go` を分け、一覧の関数が決まった順に並べる（シーンは 1 つの Renderer を共有するので順序が絵に効く）。
- 作業はまとまりごとに別の worktree でサブエージェントが進め、最後に 1 つにまとめる。gallery と `widgets/design_test.go` の共通の表（hover / press / disabled / 高さ）は、まとめるときに足す。

## 結果

- 当初の計画の該当する行を、この分け方に合わせて書き換えた。対象外の一覧に自前の `FileDialog` を足した。
- 15b の DataTable が 120fps に届かない場合の判断は、15b の ADR で書く。
