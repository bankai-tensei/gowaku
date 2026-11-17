# 0085: アクセシビリティ（macOS の NSAccessibility と VoiceOver）

決定日: 2026-11-10

状態: 採用

## 背景

ADR 0082 の「まだ無いもの」の一つ。ADR 0042 の `semantics` の木と `platform.Accessibility` を、macOS の NSAccessibility につなぎ、VoiceOver が gowaku の窓を読んで操作できるようにする。ADR 0042 §3 で見たとおり、NSAccessibility も「引く」問い合わせ（`accessibilityChildren` など）と「押す」通知（`NSAccessibilityPostNotification`）の組み合わせなので、インターフェースは変えない。

## 決定

### 1. 形

- **根（ID 0、窓）は content view が答える。** view に `accessibilityChildren`、`accessibilityHitTest:`、`accessibilityFocusedUIElement` を足す（`accessibilityViewMethods`）。view そのものは NSView の既定どおり無視される要素のままにし、子は窓の直下に見える。
- **その下の各ノードは `NSAccessibilityElement` のサブクラスのインスタンス。** クラスは view と同じく Platform ごとに `objc.RegisterClass` で作り、メソッドは Platform の状態を閉じ込めたクロージャにする。作るのは最初の問い合わせのとき（AppKit のシンボルもそこで読む）。失敗したら `slog.Debug` に残し、木を出さない。
- **要素はノードの ID ごとに 1 つ、問い合わせで初めて作って保持する。** 次の `Update` で ID が木から消えたら `NSAccessibilityUIElementDestroyedNotification` を出して release する。同じノードには同じオブジェクトを返し続ける（VoiceOver はオブジェクトの同一性でフォーカスを追う）。
- **プロパティは要素のメソッドで答える。** 役割、ラベル、値、ヘルプ（Hint）、frame、親、子、有効、フォーカス、選択、展開、範囲の最小・最大、テキスト（文字数、選択、行、範囲の文字列と矩形）。NSAccessibilityElement はクラスにあるメソッドを全部属性・アクションとして見せるので、`isAccessibilitySelectorAllowed:` でノードに無いもの（Tap の無いノードの `AXPress` など）を隠す。
- **frame は画面座標。** ノードの矩形は窓の論理ピクセルで、それは flipped な content view のポイントと同じ。`convertRect:toView:nil` と `convertRectToScreen:` で画面に移す（IME の `caretRect` と同じ）。ヒットテストは逆向きに変換し、UIA と同じく重なる兄弟は後ろ（上に描かれた方）を取る。

### 2. 役割の対応

ADR 0042 §2 の NSAccessibility 列を基本にし、表に無いものと VoiceOver の読み方に合わないものは Chromium の対応に合わせた。

| semantics | role / subrole |
|---|---|
| Button / Checkbox / Radio / Slider / Image / Link | AXButton / AXCheckBox / AXRadioButton / AXSlider / AXImage / AXLink |
| Switch | AXCheckBox / AXSwitch |
| Tab / TabList | AXRadioButton / AXTabButton、AXTabGroup |
| RadioGroup | AXRadioGroup |
| List / ListItem | AXList / AXContentList、AXGroup（表の「cell」ではなく Chromium と同じ） |
| Dialog | AXGroup / AXApplicationDialog（窓の中の窓を AXWindow にすると VoiceOver が別の窓と扱うため） |
| Group / RoleNone で子あり | AXGroup |
| Text / RoleNone でラベルだけ | AXStaticText（文字は値で返す。VoiceOver が値を読むため） |
| TextField | AXTextField、Multiline なら AXTextArea |
| Heading | AXHeading。値は Level（WebKit と同じ） |
| Stepper / Progress | AXIncrementor / AXProgressIndicator |
| MenuBar / Menu / MenuItem / MenuItemCheckbox | AXMenuBar / AXMenu / AXMenuItem / AXMenuItem（値 0・1） |
| Separator / Toolbar / Tooltip | AXSplitter / AXToolbar / AXGroup + AXUserInterfaceTooltip |
| ComboBox | 入力できる（Text あり）なら AXComboBox、そうでなければ AXPopUpButton |
| Grid / Row / GridCell / ColumnHeader | AXTable / AXRow / AXCell / AXCell |
| Tree / TreeItem | AXOutline / AXRow + AXOutlineRow（`AXDisclosing`、`AXDisclosureLevel`、`AXDisclosedByRow`） |

役割の説明（"button" など）は AppKit の `NSAccessibilityRoleDescription` に任せる。AXHeading は空なので "heading" を返す。

### 3. 値

チェックボックスとスイッチは 0・1・2（Mixed）、選ばれた Radio と Tab は 1、範囲は `Range.Value`、テキストフィールドは文字列、見出しは Level を NSNumber か NSString で返す。テキストのオフセットは NSString と同じ UTF-16 なので、`semantics.TextInfo` をそのまま使える（AT-SPI のような変換は要らない）。

### 4. アクション

すべて `EventAccessibilityAction` として `enqueue` で積む（UIA、AT-SPI と同じ道）。Disabled のノードには積まない。

| NSAccessibility | semantics |
|---|---|
| `accessibilityPerformPress` | Tap |
| `accessibilityPerformIncrement` / `Decrement` | Increase / Decrease |
| `setAccessibilityValue:`（範囲） | SetValue（Number）。SetValue が無く Increase / Decrease だけなら、その向きに 1 段（AT-SPI と同じ） |
| `setAccessibilityValue:`（文字） | SetValue（Text） |
| `setAccessibilitySelectedTextRange:` | SetSelection |
| `setAccessibilityFocused:` | Focus |
| `setAccessibilitySelected:` | まだ選ばれていなければ Tap（UIA の Select と同じ） |
| `setAccessibilityExpanded:` / `setAccessibilityDisclosed:` | Expand / Collapse、無ければ Tap（UIA と同じ） |

### 5. 通知

`Update` の差分から、聞き手がいるときだけ出す（`axNotes` は純粋な関数で、OS 無しでテストする）。

- 追加・削除・子の並びの変化 → `AXLayoutChanged`（窓に）。
- ラベル → `AXTitleChanged`、値（範囲、チェック、文字）→ `AXValueChanged`、選択の移動 → `AXSelectedTextChanged`。
- 選択項目の Selected の変化 → コンテナに `AXSelectedChildrenChanged`。木の項目の開閉 → `AXRowExpanded` / `AXRowCollapsed`。
- フォーカスの移動 → 新しいフォーカスの要素に `AXFocusedUIElementChanged`（要素がまだ無ければ作る）。
- ライブ領域 → NSApp に `AXAnnouncementRequested`（Polite は優先度 Medium、Assertive は High）。変わったときだけで、現れたときは読まない（ADR 0043）。

通知を出すのは、読み手が見たことのある要素（作ってある要素）だけ。見ていない要素の変化は、読み手が次に問い合わせたときに分かる。

### 6. `Listening()`

UIA と同じく、**view が初めて `accessibilityChildren`（かヒットテスト）を聞かれたとき** に真にし、`EventRedraw` を積む。`accessibilityFocusedUIElement` だけでは真にしない。AppKit は読み手以外のためにも first responder にそれを聞くので。

## 実装で決めたこと

- **ブリッジの置き場所。** ブリッジは view のクラスの状態（クロージャ）に view ごとに持つ。`Window.Accessibility()` は view に自前のメソッド `gowakuAccessibilityBridge:` を送り、ヒープに置いたポインター 1 個に書き込ませて受け取る。`Platform` と `Window` の構造体に手を入れずに済み、パッケージ変数も増やさない（AppKit のシンボルの `sync.OnceValues` だけ）。
- **閉じた窓。** `Window.Close` からは呼ばれないので、次の問い合わせか `Update` で、閉じた窓（または同じアドレスの別の view に変わったもの）のブリッジを見つけたら要素を全部 release する。
- **スレッド。** AppKit は問い合わせをメインスレッドの run loop、つまり `WaitEvent` の中で呼ぶ。`Update` も UI ゴルーチン（メインスレッド）なので、ロックは要らない。

## まだ無いもの

- **本物の VoiceOver での確認。** gpu タグのテストは、窓を作って `Update` を渡し、view と要素に objc で直接問い合わせ、アクションがイベントになることを確かめるだけ。AX API（`AXUIElement`）越しの確認と、VoiceOver の読み上げは手で行う。
- **表の細部：** `AXColumnCount`、`AXRowIndexRange` / `AXColumnIndexRange`、列見出しの対応、`RowCount` で仮想化された表の行数。VoiceOver の表の移動（VO+矢印）は行と子の並びで動くが、「n 行目 m 列」は読まれない。
- **文字の属性**（`accessibilityAttributedStringForRange:`）、`AXScrollToVisible`（ScrollIntoView）、メニュー項目のショートカット表示（`AXMenuItemCmdChar`）。
- **無視する要素。** 役割もラベルも無い入れ子のグループも要素として出す。VoiceOver で余計なグループに入る操作が要るなら、`isAccessibilityElement` を偽にして子を持ち上げる。

## 却下した案

- **view のサブクラスに Swift / ObjC のコードを足す：** cgo を使わない方針（当初の計画）に反する。
- **NSAccessibility の古い API（`accessibilityAttributeValue:` など）：** 文字列キーの分岐を全部自分で書くことになり、10.10 以降の新しい API より大きく、非推奨。
- **ノードごとに NSView を置く：** 描画と無関係な view を大量に持つことになり、レイアウトの変化のたびに view の木を直す必要がある。
