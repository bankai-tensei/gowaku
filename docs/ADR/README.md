# Architecture Decision Records

This directory holds gowaku's architecture decision records (ADRs). **The ADRs are written in Japanese.** The titles below are translated so you can find the record you need; the bodies are not.

An ADR here records one design decision at the time it was made: why the question came up, what the options were, which one was chosen, and what happened afterwards. ADRs are records, not living documentation. When a later decision replaces an earlier one, the old ADR stays as it was and its status line points to the new one. For the current shape of the library, read [ARCHITECTURE.md](../ARCHITECTURE.md) first.

Each ADR has the same layout:

- a title line `# NNNN: ...`, the decision date (`決定日`) and the status (`状態`)
- `背景` (context): the problem and the constraints
- `選択肢` (options): the alternatives that were considered
- `決定` (decision): what was chosen and why
- `結果` (consequences): what changed, what was measured, and what was left open

Some ADRs merge options and decision into one section (`選択肢と決定`), and some add sections such as `残したもの` (left for later).

# 設計判断の記録（ADR）

このディレクトリには、gowaku の設計判断の記録（ADR）を置いている。**ADR は日本語で書いている。** 下の表には英語のタイトルも併記した。

ここでいう ADR とは、1 つの設計判断を、判断した時点の形のまま残したものである。なぜ問題になったのか、どのような選択肢があったのか、どれを選んだのか、その後どうなったのかを記す。ADR は記録であって、最新の状態を説明する文書ではない。後の判断が以前の判断を置き換えた場合も、古い ADR はそのまま残し、状態の行から新しい ADR を参照する。現在のライブラリの構成を知るには、まず [ARCHITECTURE.ja.md](../ARCHITECTURE.ja.md) を読んでほしい。

ADR はどれも同じ構成をとる。

- タイトルの行 `# NNNN: ...`、決定日、状態
- 背景：問題と制約
- 選択肢：検討した案
- 決定：選んだ案とその理由
- 結果：変わったこと、計測したこと、残した課題

選択肢と決定を 1 つの節（選択肢と決定）にまとめたものや、「残したもの」などの節を加えたものもある。

## Index / 一覧

Status / 状態: Accepted = 採用, Partly superseded by NNNN = 一部を ADR NNNN で置き換え, Superseded by NNNN = ADR NNNN で置き換え.

Number 0104 is unused (the number was skipped). / 0104 は欠番（番号を飛ばした）。

| No. | Title | タイトル | Status |
|---|---|---|---|
| [0001](0001-win32-modal-loop.md) | Draw from a same-thread callback during the Win32 modal loop | Win32 のモーダルループ中は同一スレッドのコールバックで描画する | Accepted |
| [0002](0002-gpu-surface-api.md) | gpu.Backend takes only a NativeHandle, and Surface owns Resize | gpu.Backend は NativeHandle だけを受け取り、Resize は Surface が担う | Accepted |
| [0003](0003-adapter-selection.md) | Prefer the power-saving GPU by default | 既定の GPU は省電力側を選ぶ | Accepted |
| [0004](0004-shader-compile-with-fxc.md) | Compile HLSL with fxc and commit the DXBC to the repository | HLSL は fxc でコンパイルし、DXBC をリポジトリに入れる | Accepted |
| [0005](0005-sdf-pipeline.md) | Instance format and drawing semantics of the SDF pipeline | SDF パイプラインのインスタンス形式と描画のセマンティクス | Partly superseded by [0032](0032-gradient-stops.md) |
| [0006](0006-core-tree-api.md) | Five changes to the originally planned core API | core の API を当初の案から 5 点変更する | Accepted |
| [0007](0007-text-architecture.md) | Text architecture and changes from the original plan | テキストの構成と当初の計画からの変更点 | Accepted |
| [0008](0008-input-and-ime.md) | Input delivery and IME handling | 入力の配送と IME の扱い | Accepted |
| [0009](0009-paths-and-clips.md) | Paths, clips, scrolling and ruby | パス、クリップ、スクロール、ルビ | Partly superseded by [0017](0017-multisample-on-demand.md) |
| [0010](0010-motion-and-theme.md) | Motion, the MD3 theme and following the OS | アニメーション、MD3 テーマ、OS 設定への追従 | Partly superseded by [0011](0011-own-design.md) |
| [0011](0011-own-design.md) | Drop Material Design for an own design | マテリアルデザインをやめて独自デザインにする | Accepted |
| [0012](0012-shaders-for-vulkan.md) | Build the Vulkan shaders from the same HLSL source | Vulkan のシェーダーも同じ HLSL から生成し、ソースを一本化する | Accepted |
| [0013](0013-vulkan-backend.md) | Structure of gpu/vulkan | gpu/vulkan の構成 | Accepted |
| [0014](0014-x11-platform.md) | Structure of platform/x11 | platform/x11 の構成 | Accepted |
| [0015](0015-bench-results.md) | Benchmark method and how to handle the items we lost | ベンチマークの方法と、負けた項目の扱い | Accepted |
| [0016](0016-public-api.md) | Scope of the v1 public API | v1 の公開 API の範囲 | Partly superseded by [0081](0081-widget-layers.md) |
| [0017](0017-multisample-on-demand.md) | MSAA only for the frames that need it | MSAA は必要なフレームでだけ使う | Accepted |
| [0018](0018-parallel-startup.md) | Load fonts in parallel at startup | 起動時にフォントを並行して読み込む | Accepted |
| [0019](0019-layers.md) | Layer cache; drop partial present | レイヤーキャッシュを導入し、部分 present はやめる | Accepted |
| [0020](0020-pixel-goldens.md) | gpu/soft and pixel goldens | gpu/soft とピクセル golden | Accepted |
| [0021](0021-wayland-platform.md) | Structure of platform/wayland | platform/wayland の構成 | Accepted |
| [0022](0022-linux-ime-dbus-file-dialog.md) | Linux IME, D-Bus and file dialogs | Linux の IME、D-Bus、ファイルダイアログ | Partly superseded by [0053](0053-phase15-scope.md) |
| [0023](0023-vertical-text.md) | Vertical text and typesetting extensions (Phase 11a) | 縦書きと組版の拡張（Phase 11a） | Accepted |
| [0024](0024-color-emoji-bidi-variable-hyphen-incremental.md) | Color emoji, BiDi, variable fonts, hyphenation, incremental layout (Phase 11b) | カラー絵文字、BiDi、可変フォント、ハイフネーション、増分レイアウト（Phase 11b） | Accepted |
| [0025](0025-app-feedback.md) | Fill the gaps found by writing one app | アプリを 1 本書いて見つかった不足を埋める | Accepted |
| [0026](0026-unbounded-room.md) | Make layouts with unbounded length explicit through panics and docs | 長さが決まらない配置を、panic とドキュメントで明示する | Accepted |
| [0027](0027-hidden-subtrees.md) | Exclude fully transparent subtrees from hit testing, Semantics and Tab | 透明度 0 の部分木を、ヒットテスト・Semantics・Tab から外す | Accepted |
| [0028](0028-pressable.md) | A Pressable widget for pressable regions | 押せる領域を表すウィジェット Pressable を用意する | Accepted |
| [0029](0029-typed-state-widget.md) | `StateOf[W]`: read the State's own Widget with its type | State から自分の Widget を型付きで読む `StateOf[W]` | Accepted |
| [0030](0030-repaint-signal.md) | RepaintSignal: repaint a CustomPaint without rebuild or relayout | CustomPaint を再ビルドも再レイアウトもせずに再描画する RepaintSignal | Accepted |
| [0031](0031-style-copy-and-named-alignments.md) | Changing part of a TextStyle, and named Alignments | TextStyle の一部だけを変える方法と、名前付きの Alignment | Accepted |
| [0032](0032-gradient-stops.md) | Up to four gradient stops, stored directly in the instance | グラデーションのストップは最大 4 つとし、インスタンスに直接格納する | Accepted |
| [0033](0033-wrap-and-grid.md) | Wrapping rows (Wrap) and tiles whose column count follows the width (Grid) | 折り返す並び（Wrap）と、列の数が幅で決まるタイル（Grid） | Accepted |
| [0034](0034-varying-list-extents.md) | Per-row ListView heights from a function known in advance | ListView の行の高さを、事前に分かる関数で行ごとに変える | Accepted |
| [0035](0035-focus-and-style-followups.md) | Fix what porting the sample app to the new APIs revealed | サンプルアプリを新しい API に移行して見つかった問題を直す | Accepted |
| [0036](0036-wheel-glide-and-touchpad.md) | Exponential deceleration for the wheel, immediate movement for touchpads | ホイールは指数関数的に減速させ、タッチパッドは即座に動かす | Accepted |
| [0037](0037-switcher-transitions.md) | Selectable AnimatedSwitcher transitions, and Translate | AnimatedSwitcher の遷移を選べるようにし、Translate を追加する | Accepted |
| [0038](0038-macos-last.md) | Move macOS support to the last phase of v2 | macOS 対応を v2 の最後のフェーズに回す | Accepted |
| [0039](0039-short-ui-files.md) | UI files need only the gowaku import; recommend dot imports | UI のファイルを gowaku の import 1 つで書けるようにし、ドットインポートを推奨する | Superseded by [0081](0081-widget-layers.md) |
| [0040](0040-preview.md) | Previews built on files and a command, with a page on top | プレビューはファイルとコマンドを土台とし、その上にページを載せる | Accepted |
| [0041](0041-ink-contrast.md) | Two readable text colors, Ink and Ink2; keep Ink3 as a faint color | 読める文字色は Ink と Ink2 の 2 段にし、Ink3 は薄い色として残す | Accepted |
| [0042](0042-accessibility-windows.md) | Accessibility (12a: Windows UIA, focus, reduced motion) | アクセシビリティ（12a: Windows の UIA、フォーカス、動きを減らす） | Accepted |
| [0043](0043-accessibility-linux.md) | Accessibility (12b: Linux AT-SPI2, reduced motion) | アクセシビリティ（12b: Linux の AT-SPI2、動きを減らす） | Accepted |
| [0044](0044-drawing-extensions.md) | Drawing extensions (13a: arbitrary affine transforms, gradients, blend modes, blur) | 描画拡張（13a: 任意 Affine、グラデーション、ブレンドモード、ブラー） | Accepted |
| [0045](0045-shaders-svg-animated-images.md) | Drawing extensions (13b: SVG, mipmaps, animated images, custom shaders) | 描画拡張（13b: SVG、ミップマップ、アニメーション画像、カスタムシェーダー） | Partly superseded by [0112](0112-svg-structure-paint-clip.md), [0113](0113-svg-filters-text.md) |
| [0046](0046-menus-and-shortcuts.md) | Menus and shortcuts (14a) | メニューとショートカット（14a） | Accepted |
| [0047](0047-modal-windows.md) | Modal windows (14a) | モーダルウィンドウ（14a） | Accepted |
| [0048](0048-custom-title-bar.md) | Custom title bar (14a) | カスタムタイトルバー（14a） | Accepted |
| [0049](0049-clipboard.md) | Clipboard extensions (14a) | クリップボード拡張（14a） | Accepted |
| [0050](0050-drag-and-drop.md) | Drag and drop (14b) | ドラッグ & ドロップ（14b） | Accepted |
| [0051](0051-tray-and-notifications.md) | Tray and notifications (14b) | トレイと通知（14b） | Accepted |
| [0052](0052-pinch-touch-and-pen.md) | Pinch, touch and pen (14b) | ピンチ、タッチ、ペン（14b） | Accepted |
| [0053](0053-phase15-scope.md) | Scope and split of Phase 15 | Phase 15 の範囲と分け方 | Accepted |
| [0054](0054-display-and-choice-widgets.md) | Display and choice widgets (15a) | 表示と選択のウィジェット（15a） | Accepted |
| [0055](0055-tooltip-toast-and-sheet.md) | Tooltips, toasts and sheets (15a) | ツールチップ、トースト、シート（15a） | Accepted |
| [0056](0056-fields-and-forms.md) | Search fields, dropdowns, suggestions and forms (15a) | 検索欄、ドロップダウン、候補、フォーム（15a） | Accepted |
| [0057](0057-date-and-time-pickers.md) | Calendar, date and time input (15a) | カレンダー、日付と時刻の入力（15a） | Accepted |
| [0058](0058-data-table.md) | DataTable (15b) | DataTable（15b） | Partly superseded by [0127](0127-sort-semantics-row-keys-sidebar-count-icons.md) |
| [0059](0059-tree-view.md) | TreeView (15b) | TreeView（15b） | Accepted |
| [0060](0060-tab-bar-and-tear-off.md) | TabBar and tab tear-off (15b) | TabBar とタブの切り離し（15b） | Accepted |
| [0061](0061-text-editor.md) | TextEditor (15b) | TextEditor（15b） | Accepted |
| [0062](0062-split-pane-property-grid-breadcrumb.md) | SplitPane, PropertyGrid, Breadcrumb (15b) | SplitPane、PropertyGrid、Breadcrumb（15b） | Accepted |
| [0063](0063-navigator.md) | Navigator, PageRoute, Hero (15c) | Navigator、PageRoute、Hero（15c） | Accepted |
| [0064](0064-localization.md) | Localization (15c) | ローカライズ（15c） | Accepted |
| [0065](0065-theme-extensions-and-coverage.md) | Theme extensions, row names, and coverage of every widget (15c) | テーマの拡張、行の名前、全ウィジェットの網羅（15c） | Accepted |
| [0066](0066-canvas-copies-paths.md) | Canvas records copies of paths | Canvas はパスのコピーを記録する | Accepted |
| [0067](0067-runtime-theme.md) | Switching themes at run time, and overlays that follow the caller's theme | 実行中のテーマ切り替えと、呼び出し元のテーマに従うオーバーレイ | Accepted |
| [0068](0068-gowakutest-settle-option.md) | gowakutest Settle options, and Find looking at a stale tree | gowakutest の Settle のオプションと、Find が古いツリーを参照していた問題 | Accepted |
| [0069](0069-positioned-fill.md) | PositionedFill (a child stretched to both edges of a Stack) | PositionedFill（Stack の両端まで引き伸ばす子） | Accepted |
| [0070](0070-root-aliases.md) | Extend the root aliases to every key and the geom constructors | ルートの別名をすべてのキーと geom のコンストラクタに広げる | Superseded by [0081](0081-widget-layers.md) |
| [0071](0071-datatable-cell-kinds.md) | DataTable cell kinds | DataTable のセルの種類 | Accepted |
| [0072](0072-calendar-marks.md) | Day marks in the calendar | カレンダーの日付の印 | Accepted |
| [0073](0073-inspector.md) | Inspector | Inspector | Accepted |
| [0074](0074-inline-ruby.md) | Ruby display in TextField (Aozora Bunko notation) | TextField のルビ表示（青空文庫の記法） | Accepted |
| [0075](0075-textfield-controller.md) | TextFieldController and a frameless TextField | TextFieldController と枠のない TextField | Accepted |
| [0076](0076-close-request.md) | Refusing a window's close request | ウィンドウを閉じる要求を拒否する | Accepted |
| [0077](0077-wayland-bindgen.md) | Publishing gowaku-bindgen (Wayland binding generator) | gowaku-bindgen（Wayland のバインディング生成）の公開 | Accepted |
| [0078](0078-measured-list-extents.md) | ListView row heights measured from content | ListView の行の高さを内容から測る | Accepted |
| [0079](0079-atlas-lru.md) | Evict glyph atlas entries one by one with an LRU | グリフアトラスのエントリを LRU で 1 つずつ追い出す | Accepted |
| [0080](0080-startup.md) | Shorter startup | 起動時間を短縮する | Accepted |
| [0081](0081-widget-layers.md) | Split widgets into layers by meaning, and narrow the root package | widgets を意味ごとの層に分け、ルートパッケージを絞り込む | Accepted |
| [0082](0082-macos-cocoa-metal.md) | macOS foundation (platform/cocoa and gpu/metal) | macOS の土台（platform/cocoa と gpu/metal） | Accepted |
| [0083](0083-macos-integration.md) | macOS integration (menu bar, title bar, tray, custom shaders) | macOS の OS 統合（メニューバー、タイトルバー、トレイ、カスタムシェーダー） | Accepted |
| [0084](0084-sbix-emoji.md) | sbix color emoji (Apple Color Emoji) | sbix のカラー絵文字（Apple Color Emoji） | Accepted |
| [0085](0085-accessibility-macos.md) | Accessibility (macOS NSAccessibility and VoiceOver) | アクセシビリティ（macOS の NSAccessibility と VoiceOver） | Accepted |
| [0086](0086-fma-determinism.md) | FMA fusion and identical rounding across architectures | FMA の融合と、アーキテクチャ間で丸めを揃えること | Accepted |
| [0087](0087-disclosure-link.md) | Disclosure (accordion) and Link | Disclosure（アコーディオン）と Link | Accepted |
| [0088](0088-input-obscure.md) | Input.Obscure (password fields) and secure input | Input.Obscure（パスワード欄）と安全な入力 | Accepted |
| [0089](0089-selection-area.md) | SelectionArea (select and copy the text below it) | SelectionArea（配下のテキストを選択してコピーする） | Accepted |
| [0090](0090-gridview.md) | GridView (virtualized grid) | GridView（仮想化グリッド） | Accepted |
| [0091](0091-reorderable-list.md) | ReorderableList (a list that can be reordered) | ReorderableList（並べ替えられるリスト） | Accepted |
| [0092](0092-interactive-viewer.md) | InteractiveViewer (a pan and zoom surface) | InteractiveViewer（パンとズームができる領域） | Accepted |
| [0093](0093-number-range-token.md) | NumberField, RangeSlider, TokenField | NumberField、RangeSlider、TokenField | Accepted |
| [0094](0094-color-picker.md) | Color selection (ColorPicker, and the default target of PropertyColor) | 色の選択（ColorPicker と PropertyColor の既定の押し先） | Accepted |
| [0095](0095-skeleton-sparkline.md) | Loading placeholders (Skeleton) and small charts (Sparkline) | 読み込み中のプレースホルダー（Skeleton）と小さなグラフ（Sparkline） | Accepted |
| [0096](0096-infobar-statusbar.md) | InfoBar and StatusBar | InfoBar と StatusBar | Accepted |
| [0097](0097-command-palette.md) | CommandPalette | CommandPalette | Accepted |
| [0098](0098-dock-layout.md) | DockLayout (IDE-style panel docking) | DockLayout（IDE 風のパネルのドッキング） | Accepted |
| [0099](0099-markdown-module.md) | Markdown as a separate module with its own parser | Markdown を別モジュールにし、パーサを自前で実装する | Partly superseded by [0107](0107-table-min-content-width.md), [0119](0119-markdown-html-subset.md) |
| [0100](0100-table-widget-cells.md) | Widgets in Table cells (CellWidget) | Table のセルにウィジェットを置く（CellWidget） | Accepted |
| [0101](0101-synthetic-italic.md) | Synthetic italics (slanting fonts that have no italic face) | 合成斜体（斜体の書体がないフォントを傾けて描く） | Accepted |
| [0102](0102-dock-autohide-nesting.md) | Auto-hiding side strips and nested docks in DockLayout | DockLayout の自動で隠れるサイドの帯と、入れ子のドック | Accepted |
| [0103](0103-editor-views-and-scroll.md) | Editor views, reading and writing the scroll position, horizontal scroll bar, italic decorations | エディターのビュー、スクロール位置の読み書き、横スクロールバー、斜体の装飾 | Accepted |
| 0104 | (unused) | （欠番） | — |
| [0105](0105-markdown-selection.md) | Selecting and copying Markdown text with SelectionArea | Markdown のテキストを SelectionArea で選択してコピーする | Accepted |
| [0106](0106-markdown-headings.md) | A list of Markdown headings, and jumping to a heading | Markdown の見出しの一覧と、見出しへのジャンプ | Accepted |
| [0107](0107-table-min-content-width.md) | Table columns are never narrower than their words | 表の列を単語の幅より狭くしない | Accepted |
| [0108](0108-editor-selection-notification.md) | Editor selection notifications, and two fixes found in review | エディターの選択の通知と、レビューで見つかった 2 つの修正 | Accepted |
| [0109](0109-dock-immediate-edits-and-italic-tabs.md) | Immediate edits to the dock, and italic tabs | ドックへの即時の変更と斜体のタブ | Accepted |
| [0110](0110-fit-windows-to-the-work-area.md) | Fit opening windows to the work area | 開くウィンドウを作業領域に収める | Accepted |
| [0111](0111-window-decorations.md) | Window decorations (icon, minimum and maximum size, always on top, full screen, transparency) | ウィンドウの装飾（アイコン、最小・最大サイズ、最前面表示、フルスクリーン、透過） | Accepted |
| [0112](0112-svg-structure-paint-clip.md) | SVG structure, fill, stroke, clipping and masks | SVG の構造、塗り、線、クリップとマスク | Partly superseded by [0114](0114-gpu-layers-masks-filters.md) |
| [0113](0113-svg-filters-text.md) | SVG filters and text | SVG のフィルターとテキスト | Partly superseded by [0114](0114-gpu-layers-masks-filters.md) |
| [0114](0114-gpu-layers-masks-filters.md) | GPU groups, masks and filters | GPU のグループ、マスク、フィルター | Accepted |
| [0115](0115-cached-groups.md) | Cached groups, and grouping for existing widgets | キャッシュするグループ、既存ウィジェットのグループ化 | Accepted |
| [0116](0116-keys-and-open-url.md) | Symbol keys and OpenURL | 記号キーと OpenURL | Accepted |
| [0117](0117-fuzzy-line-background-context-menu.md) | Public fuzzy matching, full-width line backgrounds, right-click row selection and OnContextMenu | あいまい一致の公開、行の幅いっぱいの背景、右クリックによる行の選択と OnContextMenu | Accepted |
| [0118](0118-dock-close-guard-and-flyout-overlays.md) | Ask before closing a dock panel, and overlays opened from a flyout | ドックのパネルを閉じる前の確認と、フライアウトから開いたオーバーレイ | Accepted |
| [0119](0119-markdown-html-subset.md) | A safe subset of HTML in Markdown | Markdown 内の HTML は安全なサブセットだけを解釈する | Accepted |
| [0120](0120-modal-semantics.md) | Keep what lies under a dialog or sheet out of the semantics tree | モーダルの下を Semantics ツリーから外す | Accepted |
| [0121](0121-scroll-reverse-and-controller.md) | Scroll.Reverse and ScrollController | Scroll の Reverse と ScrollController | Accepted |
| [0122](0122-ruby-at-line-edges.md) | Ruby kept within the line at its head and end, overhanging kana only | 長いルビを行頭・行末からはみ出させず、仮名の側だけに掛ける | Accepted |
| [0123](0123-sidebar-input-fill-ink-contrast.md) | Sidebar width, scrolling, header and footer; multiline Input fills with a label; contrast of the text inks | Sidebar の幅・スクロール・Header/Footer、ラベル付き複数行 Input の伸び方、文字色のコントラスト | Accepted |
| [0124](0124-warnings-under-modal-taps-warning-color.md) | Warnings skip what lies under a modal; secondary and double taps in gowakutest; the Warning color | Warnings でモーダルの下を調べない、副ボタンとダブルクリックのタップ、Warning の色 | Accepted |
| [0125](0125-text-truncation-decorative-stacks.md) | Text truncation with MaxLines, ExcludeSemantics for decorative text, default spacing of VStack/HStack | 文字の切り詰め（MaxLines）、飾りの文字を外す ExcludeSemantics、VStack/HStack の既定の間隔 | Accepted |
| [0126](0126-dialog-focus-submit-on-enter-after.md) | Focus of a field in a dialog, Enter that submits a multiline field, and a call after a delay | ダイアログ内の入力欄のフォーカス、複数行の欄の Enter による送信、時間をおいた実行 | Accepted |
| [0127](0127-sort-semantics-row-keys-sidebar-count-icons.md) | Sort state of column headers, selection that follows row keys, Sidebar counts, stock icons | 列見出しのソート状態、キーで行を追う選択、Sidebar の件数、標準のアイコン | Accepted |
| [0128](0128-tester-pointer-locale-settle-mono.md) | Contrast of disabled controls; pointer methods and locale of the Tester; the settle rule; the monospaced Code text style | 無効な部品のコントラスト、Tester のポインター操作とロケール、待つ規則、等幅の文字スタイル | Accepted |
| [0129](0129-form-fields-invalid-required-calendar-years.md) | Invalid state of every field control, Required errors of choices, months and years of a calendar, Enter of NumberField | 入力部品の不正状態の統一、選択部品の必須エラー、カレンダーの月と年の一覧、NumberField の Enter | Accepted |
| [0130](0130-layout-builder-aspect-ratio-viewer-keys.md) | LayoutBuilder that builds with the constraints, the window size in MediaQuery, AspectRatio, wheel zoom and keys of InteractiveViewer | 制約で構築する LayoutBuilder、MediaQuery のウィンドウの大きさ、AspectRatio、InteractiveViewer のホイールでの拡大縮小とキー | Accepted |
| [0131](0131-find-bar-room-heading-lines-sidebar-detail-steps.md) | Room for the find bar, source lines of headings, a second line in Sidebar rows, nav.Steps and control.NumberStepper | 検索バーの分の余白、見出しのソース上の位置、Sidebar の 2 行目、手順の表示と NumberStepper への改名 | Accepted |
| [0132](0132-tester-clip-and-held-modifiers.md) | Warnings skip what an ancestor clips away; Tester.Hold keeps modifier keys down for pointer actions | Warnings はクリップで隠れたものを調べない、Tester で修飾キーを押したままにする | Accepted |
| [0133](0133-typing-keys-slider-format-chosen-count.md) | Keys that type stay in a focused text field, the spoken value of a slider, the count of a chosen Sidebar row | 入力中のキーとショートカット、スライダーの値の読み方、選択中の行の件数 | Accepted |
| [0134](0134-gesture-double-tap-drag-reorderable-tap.md) | Double taps and drags of GestureDetector, taps on a ReorderableList, how presses reach Focus and Listener | GestureDetector のダブルタップとドラッグ、ReorderableList のタップ、押下とフォーカスの説明 | Accepted |
| [0135](0135-emphasis-dots.md) | Emphasis dots and their Aozora notes, the length that Align centers in, DrawRect, notes on Positioned and on getting the Shaper | 傍点（圏点）と青空文庫の傍点の注記、そろえの幅、DrawRect、Positioned と Shaper の取り方の注意書き | Accepted |
| [0136](0136-roles-only-in-semantics.md) | Roles, actions and nodes only in package semantics; the aliases in core removed | 役割と操作の定数は semantics にだけ置く | Accepted |
| [0137](0137-tester-scroll-sign-find-clipboard-overflow.md) | The scroll sign of Tester matches PointerEvent, Find prefers what can act, a clipboard of the Tester, overflow of a child past its parent | Tester のスクロールの符号、ラベルで探す順、テスト用のクリップボード、親からのはみ出し | Accepted |
| [0138](0138-on-tint-table-cells-tap-modifiers-spinner.md) | Colors of text on AccentTint (Colors.OnTint), room and placement of Table cell widgets, modifier keys of taps, indeterminate ProgressRing and ProgressBar | 色合いの上の文字の色、表のセルのウィジェットの置き方、タップの修飾キー、処理中のスピナー | Accepted |
| [0139](0139-scroll-position-stick-to-end-span-background.md) | Scroll position and OnScroll, StickToEnd for logs and chats, jumps that land after the rebuild, Span.Background, the spoken text of ruby | スクロール位置と OnScroll、ログとチャットの末尾への追従、再構築の後にも効くジャンプ、スパンの背景色、ルビの読み上げ | Accepted |
| [0140](0140-hide-label-hover-position-dates-unfocus.md) | HideLabel of the fields, OnHover of MouseRegion, short weekdays and dates, WeekStart, Unfocus | 見出しを描かない入力欄の名前、ホバー中のポインターの位置、日付の書き方、フォーカスの外し方 | Accepted |
| [0141](0141-warnings-transforms-held-drag-fake-tray.md) | Warnings follow paint transforms; Press, MoveTo and Release hold a drag; MiddleTap and FindFunc; ShowTray and Notify through the host, with a fake tray in gowakutest | Warnings は描画の変形をたどる、ドラッグを途中で止める、中ボタンと条件で探す、テスト用のトレイと通知 | Accepted |
| [0142](0142-hit-only-where-drawn-focus-press-one-shadow.md) | Layout-only render objects are hit only where a child is (SelfHitTester), focus on press skips targets that cannot take focus, paint.BoxShadow as the one shadow type | 配置だけの物は子のある所でしか当たらない、押下のフォーカスは取れない物を飛ばす、影の型を 1 つにする | Accepted |
| [0143](0143-field-width-input-states-dismiss-icon-button-message-date.md) | Fields take the width they are given and DropdownMenu draws its placeholder as the others, Disabled, ReadOnly, MinLines and MaxLines of Input, OnDismissRequest of Dialog and Sheet, Button.HideLabel, Strings.MessageDate and Time | 入力欄の幅とプレースホルダー、Input の無効と読み取り専用と行数、閉じるのを止められるダイアログ、アイコンだけのボタン、メッセージの日付 | Accepted |
| [0144](0144-editor-reveal-focus-tab-label-modified-code-block-sidebar-leading.md) | A selection set before an Editor shows scrolls on its first layout, View.Focus, TabItem.Label and Modified, Markdown.CodeBlock and CopyButton, SidebarItem.Leading | 表示前のエディターの選択とフォーカス、タブの名前と未保存の印、コードブロックの差し替えとコピー、サイドバーの行頭 | Accepted |
| [0145](0145-field-states-yesterday-copied.md) | Disabled and ReadOnly on the other text fields, Yesterday in MessageDate, Copied after a code copy | ほかの欄の無効と読み取り専用、「昨日」、コピーした後の表示 | Accepted |
| [0146](0146-tester-unreachable-targets-table-sheet-clips.md) | Label actions of the Tester fail on a node a user cannot reach (outside the window, clipped away, covered) and use the part that shows; Table and Sheet declare their clips | Tester の操作は届かないノードでテストを落とす、表とシートのクリップ | Accepted |
| [0147](0147-ctrl-enter-shortcuts-theme-token-list.md) | Text fields and the editor use only Enter and Shift+Enter and leave Ctrl and Alt with Enter to Actions, Command is ModControl on macOS, a list of the theme tokens in the guide and godoc | 入力欄の Ctrl+Enter をショートカットに渡す、テーマのトークンの一覧 | Accepted |
| [0148](0148-clip-widgets-text-in-painters-arcs.md) | basic.ClipRect and ClipRRect cut painting and the pointer, Translate.Clip cuts also at rest, paint.Canvas.DrawText with TextAnchor and text.Labels for text in a Painter, Path.AddArc, ArcTo and AddOval | 切り取りのウィジェット、Painter の中の文字、円弧と楕円 | Accepted |
| [0149](0149-exit-motion-dock-flyout.md) | The auto-hidden panel of DockLayout slides back in when it hides, drawn but out of the input until it is in; a sweep of the exit motion of the other widgets | DockLayout の自動で隠れるパネルは閉じるときも滑って戻る、ほかの部品の閉じる動きの見直し | Accepted |
| [0150](0150-scroll-offset-float64.md) | The scroll offset, the length of the content and the tops of the rows are float64, so that lists of millions of rows still move a pixel at a time far down; positions in the view stay float32 | スクロールの位置を float64 にする | Accepted |
| [0151](0151-table-row-source.md) | Table takes its rows also as RowCount and Row, so that rows that live elsewhere, as the lines of a file of millions, need no slice | Table の行を RowCount と Row で渡す | Accepted |
| [0152](0152-state-poster.md) | StateBase.Poster returns a function that other goroutines call to run work on the UI goroutine at the next frame, dropped once the State has left; gowakutest.Tester.WaitUntil waits for it | ほかの goroutine から State に仕事を届ける Poster | Accepted |
| [0153](0153-editor-file-pieces.md) | Controller.OpenFile shows a file without reading it all: pieces of the file and of edited lines in memory, lines indexed on a goroutine, Save writes the pieces through a file that takes the old one's place | Editor でファイルを読み込まずに開き、編集して保存する | Accepted |
| [0154](0154-editor-background-find.md) | Find in a text over 4 MB or a file runs on a goroutine of its own, on a snapshot, and its matches move through the edits made meanwhile; the find bar says it is searching | 大きな文書の検索を裏で走らせ、見つかった分から出す | Accepted |
| [0155](0155-editor-file-changes.md) | A file of OpenFile that changes on disk is noticed (appended, rewritten, replaced), Save refuses to write over it, and Follow reads the lines appended to it; OpenFile takes FileOptions | 開いているファイルが外で変わったときに気づき、保存で壊さず、追記を追いかける | Accepted |
| [0156](0156-editor-file-not-held-reload-keeping-edits.md) | On Windows a file of OpenFile is opened for each read, so that other programs can replace and delete it; changes of the bytes in view are noticed; ReloadKeepingEdits places the edits in the changed file as a patch does | Windows ではファイルを開いたままにしない、表示中の行の中身の変化に気づく、編集を残して読み直す | Accepted |
| [0157](0157-editor-line-source.md) | The editor takes lines through a LineSource interface (SetSource, SourceGrew, WriteTo, Edits, Rebase); reading, watching and saving files moved out to the application | ファイルの読み書きは Editor から外し、行のソースのインターフェースで受け取る | Accepted |
| [0158](0158-replace-source-rebase-positions.md) | ReplaceSource swaps in a source of the same text, as after a save, keeping the undo history, the selections and the scroll; Rebase moves the selections and the scroll with their lines; SourceGrew panics when a source lost lines | 保存の後もソースを差し替えるだけにし、Rebase で位置を残し、ソースが縮むのを止める | Accepted |
| [0160](0160-desktop-api-alignment.md) | FileDialog and the clipboard types are defined in core, not platform; ShowFileDialog, SetTheme and CurrentTheme take ctx and work under gowakutest; Input.OnFocus; OnDismissRequest returns whether to close; ShowDialog and ShowSheet return dismiss | デスクトップの API を ctx 経由にそろえ、platform の型を公開 API から外す | Accepted |
| [0161](0161-quit-request-hidden-windows-notification-permission.md) | App.OnQuitRequest answers the quit of the OS (macOS) apart from the close requests; WindowHandle.Hide, Show and Hidden for applications that stay in the tray; RequestNotifications, and Notify waits for the leave of macOS and shows while the application is in front; the first build reaches the host | 終了の要求、ウィンドウを隠す、通知の許可、最初の Build からホストに届く | Accepted |
