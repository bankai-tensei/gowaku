# 0042: アクセシビリティ（12a: Windows の UIA、フォーカス、動きを減らす）

決定日: 2026-11-08

状態: 採用

## 背景

アクセシビリティ（Phase 12）の前半。v1 で骨格だけ置いた Semantics ツリーを、OS のスクリーンリーダーにつなぐ。検討して次を決めた。

- **12a（Windows）と 12b（Linux）に分け、2 回レビューする。** 12a は `semantics` の差分、`platform` のブリッジ、`platform/win32/com`、UIA、`FocusTraversalOrder`、ReduceMotion、クリック時のフォーカス修正。12b は AT-SPI2 と Orca。
- **確認方法：** Go で書いた UIA クライアントの自動テスト、Narrator（手で確認）、NVDA（ログに残る読み上げで確認）。
- **ボタンをクリックすると、押したボタンがフォーカスを取る。** Windows のネイティブと同じ挙動。マウスで取ったフォーカスには輪を出さない（Web の `:focus-visible`）。何も無い所を押したら、今と同じでフォーカスを外す。
- **UIA の Text パターンは最小限：** 文書全体・選択・キャレットの範囲と、文字・語・行の単位での移動。当初の計画で「アクセシビリティの終わりに決める」としていた点を、ここで決めた。
- **ReduceMotion で止めるのは、位置や大きさが変わる動きだけ。** 短いフェードとスクロールの慣性は残す。明示的な Ticker（進捗の回転など）は止めない。
- **ロールは、今あるウィジェットが必要とする分を足す。** NSAccessibility（Phase 17）と AT-SPI（12b）にも写せるかを確かめる。

## 決定

### 1. `semantics` パッケージを新設する

`platform` は `core` を import できない（CODING_STYLE §3.1）。ブリッジに渡すノードの型を両方から見えるように、`geom` だけに依存する `semantics` を新しく作り、ノード、ロール、状態、アクション、差分（`Diff`）をそこに置く。

```
core     ─→ semantics, paint, text, input, geom
platform ─→ semantics, input, geom
semantics ─→ geom
```

`core.SemanticsNode`、`core.SemanticsRole`、`core.SemanticsActions` と既存の定数は、`semantics` の型の別名として残す。別名なので型は同一で、v1 の API は壊れない（当初の計画の互換方針）。

**ノード**は、v1 と同じく子をポインターで持つ入れ子の木にする。追加したフィールドは `ID`、`Hint`、`State`、`Live`、`Range`（最小・最大・現在値）、`Text`（TextField の文字列、選択、行の区切り）、`Order`。

**ID** は RenderObject ごとに 1 回だけ振る連番（`uint64`、0 は使わない）。Element ではなく RenderObject に振るのは、ノードを開くのが RenderObject だから。同じ RenderObject が残る限り ID は変わらない。

**差分** `semantics.Compare(old, new, &diff)` は ID で突き合わせ、`Added`、`Removed`、`Changed`（どの属性が変わったかのビット付き）と、フォーカスの移動を返す。木の構造の変化は、子の ID の並びが変わったノードの `ChangedChildren` で表す。

### 2. ロール、状態、アクション

| semantics | UIA ControlType | AT-SPI | NSAccessibility | 使うウィジェット |
|---|---|---|---|---|
| Button | Button | push button | button | Button, Pressable |
| Checkbox | CheckBox | check box | checkBox | Checkbox |
| Switch | Button + Toggle（Win11 のトグルと同じ） | toggle button | switch（checkBox の subrole） | Switch |
| Radio | RadioButton | radio button | radioButton | SegmentedControl の各項目 |
| RadioGroup | Group | panel | radioGroup | SegmentedControl |
| Tab / TabList | TabItem / Tab | page tab / page tab list | tab の radioButton / tabGroup | （Phase 15 の TabBar 用。型だけ置く） |
| List / ListItem | List / ListItem | list / list item | list / cell | ListView、Sidebar |
| Dialog | Window（`IsDialog`） | dialog | window の dialog subrole | Dialog |
| Group | Group | panel | group | Group |
| Heading | Text（`HeadingLevel`） | heading | heading | `Semantics{Role: RoleHeading}` |
| Link | Hyperlink | link | link | （型だけ置く） |
| Slider | Slider | slider | slider | Slider |
| Stepper | Spinner | spin button | incrementor | Stepper |
| Progress | ProgressBar | progress bar | progressIndicator | ProgressBar, ProgressRing |
| Text / TextField / Image | Text / Edit / Image | label / entry / image | staticText / textField / image | |

**状態**（ビット）：`Checked`、`Mixed`、`Selected`、`Expanded`、`Collapsed`、`Disabled`、`Focusable`、`Focused`、`ReadOnly`、`Multiline`。`Focusable` と `Focused` は core が付ける（ウィジェットは書かない）。

**アクション**：`Tap`、`Focus`、`Increase`、`Decrease`、`SetValue`、`Expand`、`Collapse`、`SetSelection`、`ScrollIntoView`。ウィジェットは `Semantics` にコールバック（`OnTap`、`OnIncrease` など）を渡し、`RenderSemantics` がそれを持つ。`Focus` は core が処理する。ノード、またはその中で最初に見つかったフォーカス可能な部品に `RequestFocus` する。

**ライブ領域**：`Live` は `Off` / `Polite` / `Assertive` の 3 つ。ラベルか値が変わると、UIA の `LiveRegionChanged` を出す。`Semantics{Live: LivePolite}` で付ける。

### 3. `platform.Accessibility`

```go
type Accessibility interface {
    Listening() bool                                    // 支援技術が窓に問い合わせたか
    Update(root *semantics.Node, focus semantics.ID, d *semantics.Diff)
}
Window.Accessibility() Accessibility                    // 無い OS では nil
```

- **引く方向（OS → アプリ）は、イベントで返す。** UIA の `Invoke` などは `EventAccessibilityAction`（`Node`、`Action`、`Text`、`Number`、`Selection`）としてイベントキューに積み、アプリのループが `core.Tree.PerformSemanticsAction` で実行する。キーやマウスと同じ道なので、`SetState` の規約（UI ゴルーチン）を破らない。
- **押す方向（アプリ → OS）は、フレームの後に送る。** `Listening()` が真の窓だけ、frame の最後（present の後）に Semantics を作り直し、前回との差分を `Update` に渡す。誰も聞いていない窓では作らないので、ホットパスのアロケーション 0 は守られる。聞き始めた最初の問い合わせで、ブリッジは `EventRedraw` を積み、最初の木を作らせる。
- **NSAccessibility に写せるか：** Cocoa も、`accessibilityChildren` などを問い合わせる「引く」方式と、`NSAccessibilityPostNotification` で知らせる「押す」方式の組み合わせなので、同じインターフェースで足りる。アクション（`accessibilityPerformPress` など）は同じイベントにする。

### 4. UIA プロバイダ（`platform/win32`）

- `WM_GETOBJECT` で `lParam == UiaRootObjectId` のとき `UiaReturnRawElementProvider` で根のプロバイダを返す。根は `IRawElementProviderFragmentRoot`。子ノードは `IRawElementProviderFragment`。
- `ProviderOptions_ServerSideProvider` だけを立てる（`UseComThreading` は立てない）。この場合、UIA は HWND のスレッド、つまり UI ゴルーチンでプロバイダを呼ぶ。念のため、スナップショットは mutex で守る。
- パターン：`Invoke`（Tap）、`Toggle`（Checkbox / Switch）、`Value`（TextField、Slider 以外の値）、`RangeValue`（Slider、Stepper、Progress）、`SelectionItem`（Radio、Tab、ListItem の Selected）、`ExpandCollapse`、`ScrollItem`、`Text` / `Text2`（TextField）。
- イベント：`UiaRaiseAutomationEvent`（フォーカス、Invoke、テキスト選択）、`UiaRaiseAutomationPropertyChangedEvent`（Name、Value、ToggleState、ExpandCollapseState、IsEnabled、RangeValue）、`UiaRaiseStructureChangedEvent`（子の変化）、ライブ領域の `LiveRegionChanged`。
- 窓が閉じるときは `UiaReturnRawElementProvider(hwnd, 0, 0, nil)` と `UiaDisconnectAllProviders` を呼ばずに、自分の作ったプロバイダだけを `UiaDisconnectProvider` で切る。消えたノードのプロバイダは、以後 `UIA_E_ELEMENTNOTAVAILABLE` を返す。

### 5. COM オブジェクトの Go 実装（`platform/win32/com`）

当初の計画は `runtime.Pinner` で固定するとしていた。**それはやめて、COM オブジェクトの本体（vtable へのポインター、参照カウント、Go 側の番号）を `CoTaskMemAlloc` の Go 外メモリに置く。** Go のポインターを Go の外に渡し続けることがなくなり、GC と無関係になる。Go 側の実体は番号から引く表に置き、参照カウントが 0 になったら表から消して、メモリを `CoTaskMemFree` で返す。

- **vtable はインターフェースごとに 1 つ。** すべてのオブジェクトで共有する。`syscall.NewCallback` の上限（約 2000）に対し、使うのは 100 個弱。
- `QueryInterface` は、オブジェクトが実装する IID の表から引く。`AddRef` / `Release` はアトミック。
- **浮動小数点の引数は `syscall.NewCallback` が受けられない**（`IRangeValueProvider::SetValue(double)`）。そこで、XMM1（arm64 では d0）を整数レジスタに移してから Go のコールバックに飛ぶ、十数バイトの橋渡しのコードを `VirtualAlloc` で置く。書き込みの後に `VirtualProtect` で実行専用にする。
- `VARIANT`、`BSTR`、`SAFEARRAY` の組み立ても `com` に置く。DnD（Phase 14）と `IFileDialogEvents` でも使う。

### 6. クリックとフォーカス

- **ポインターの押下を処理した部品の、最も近いフォーカス対象の祖先（自分を含む）が `CanFocus` なら、その部品にフォーカスを渡す。** `Pressable`、`Slider`、`Switch` などが該当する。TextField は自分で取っていたので変わらない。押下を処理した部品にフォーカス対象の祖先が無ければ、フォーカスは動かさない（スクロールバーのドラッグで TextField のフォーカスが消えない）。
- **フォーカスを見せるかどうか（`FocusVisible`）は、最後の入力で決める。** キーを押すと真になり、ポインターを押すと偽になる。UIA の `SetFocus` でも真になる。変わったとき、フォーカスのある部品に `FocusVisibleChanged` を知らせる（任意のインターフェース）。`PressState` に `FocusVisible` を足し、標準のコントロールの輪はこちらで出す。

### 7. `FocusTraversalOrder`

`FocusTraversalOrder{Order int, Child}`。HTML の `tabindex` と同じ規則にする。`Order > 0` の部品が昇順で先に来る。同じ値のものと 0（既定）のものは描画順に並ぶ。モーダルな範囲の中だけで並べる規則は今のまま。Flutter の `FocusTraversalGroup` のような入れ子のグループは作らない。

### 8. ReduceMotion

- `platform.Platform.ReduceMotion()`。Windows は `SPI_GETCLIENTAREAANIMATION` が偽のとき真（設定 › アクセシビリティ › 視覚効果 › アニメーション効果）。変化は `WM_SETTINGCHANGE` で `EventSettings` として届く。
- `MediaQueryData.ReduceMotion` で widgets に渡す。`gowakutest` でも指定できるようにする。
- **止めるもの：** `AnimatedContainer` の大きさ・余白・角丸、`AnimatedAlign`、`AnimatedPositioned`、`Switch` のつまみの移動、`AnimatedSwitcher` の移動を含む遷移（フェードだけに置き換える）、Dialog の拡大。
- **残すもの：** 色と不透明度のフェード、フォーカスの輪、ホバー、スクロールの慣性、明示的な Ticker。

## 実装で決めたこと

- **選択肢のグループはフォーカスを選ばれた項目に渡す。** RadioGroup と TabList がフォーカスを持つとき、core は Selected の子をフォーカスとして報告する。NVDA の確認で、グループに残すと矢印で選択を変えても何も読まれなかったため。
- **ラベル付きのノードの中の、同じ文字の Text ノードは木に入れない。** ボタンの文字、グループの題、見出しを二度読ませないため。比べるのは完全一致だけにした（部分一致だと短い行の文字が消える）。
- **フォーカスを持つ部品がノードを開かないとき**（Dialog の経路の `Focus`）は、その中の最初のノードがフォーカスを代わりに持つ。
- **UIA の細部：**
  - ラベルだけの RoleNone は Text、子を持てば Group にする。
  - Paragraph は改行で区切る。Format と Page は Document と同じに扱う。
  - SmallChange は Step、0 なら範囲の 1/100 にする。
  - Disabled は読み取り専用とみなす。
  - UIAutomationCore は最初の `WM_GETOBJECT` まで読み込まない。
- **`Button.Hint`** を足した。各行に同じ「削除」が並ぶ一覧のため。

## 却下した案

- **`core` に `platform` の型を見せる：** 層の規則を破るため。
- **毎フレーム Semantics を作る：** 聞き手がいない窓でもアロケーションが起きる。
- **COM オブジェクトを Go のメモリに置いて `runtime.Pinner` で固定する：** 固定の解除を参照カウントに合わせて管理する必要があり、Go の外に Go のポインターを長く置くことになる。Go の外のメモリなら、その心配そのものが無い。
- **`ProviderOptions_UseComThreading`：** 別スレッドから木を読むことになり、UI ゴルーチン限定の規約と合わない。
- **入れ子のフォーカスグループ：** todo の操作に要らない。Phase 15 のウィジェットで必要になったら考える。
