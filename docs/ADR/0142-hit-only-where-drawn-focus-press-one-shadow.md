# 0142: 配置だけの物は子のある所でしか当たらない、押下のフォーカスは取れない物を飛ばす、影の型を 1 つにする

決定日: 2026-11-17

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 4 つが見つかった。

1. **押下のフォーカスが外側の `Focus` に届かない：** `core/input.go` の `focusOnPress` は、押下を受け取った物から外へたどり、最初に見つけた `FocusTarget` で止まっていた。その物がフォーカスを取れない（`CanFocus` が false）ときも止まるので、`Keys: ViewerKeysNone` の `interact.InteractiveViewer` を `Focus` で囲み、中の `Listener` が押下を受け取ると、`Focus` にフォーカスが来なかった。
2. **層の空いた所が押下を吸い込む（別々の 2 人が報告）：** `basic.Stack` の層を `PositionedFill{Child: Align{AlignBottomCenter, Child: Button}}` や、小さな子を持つ全面の `Column` で作ると、何も描いていない所も含めて層の全体が押下を受け取り、その下のコントロールを押せなかった。`core.RenderBase.HitTest` は、子が当たらなくても自分の範囲の中なら自分を当たりとして `true` を返すので、`Stack` は最も手前の層で探すのをやめていた。`PositionedFill{Child: IgnorePointer{...}}` と書いた人は、`IgnorePointer` は通しても、その外側の `PositionedFill` が当たっていた。`IgnorePointer` を `PositionedFill` の外に置くと、`Stack` が位置を指定した子と見なさなくなる。
3. **説明が足りない：** 子も `Width` も `Height` も無い `basic.Container` は、緩い制約の下では大きさが 0 になる（Flutter の `Container` は広がる）。`interact.GestureDetector` のドラッグの位置は子からの相対で、ウィンドウや別のウィジェットの座標への変え方が書いていなかった。`control.CustomPaint` で描いた項目をスクリーンリーダーとキーボードで使えるようにする方法も無かった。
4. **影の型が 2 つある：** `theme.Shadow`（`Offset`、`Blur`、`Color`）と `layout.BoxShadow`（`Color`、`Offset`、`Blur`、`Spread`）は同じ CSS の box-shadow を表すのに、フィールドの順が違い、`Spread` は片方にしか無い。`th.Color.Shadows(...)` を `Container` に渡すには詰め替えが要り、ガイドにその関数の例が載っていた。中でも `internal/kit` の `ShadowsOf` が同じ詰め替えをしていた。

## 選択肢

### 押下のフォーカス

- **A. フォーカスを取れない `FocusTarget` を飛ばして外へ進む。** フォーカスを取れる最も近い物が取る。どれも無ければ今まで通りフォーカスは動かない（スクロールバーのドラッグで入力欄のフォーカスが残る）。
- **B. 今のまま。** `InteractiveViewer` の側で `FocusTarget` を実装しない形にする必要があり、`Keys` を後から変える場合に作り直しが要る。

### 当たり判定

- **A. 配置だけの物は子のある所でしか当たらないことにする（Flutter と同じ）。** `core` に `SelfHitTester`（`HitTestSelf(pos) bool`）を足し、`RenderBase.HitTest` は子が当たらなかったときだけこれに聞く。実装しない物は今まで通り範囲の全体で当たる。`layout` の配置と見た目だけの物が false を返す。
- **B. すべての物を既定で透過にし、当たる物が明示する。** 文字、画像、描くコントロール、各パッケージの RenderObject（60 以上）のすべてに手を入れる必要があり、漏れがあると押せない部品ができる。
- **C. `Stack` だけが、層の中で何かが押下を受け取るかを見る。** `Stack` の外（オーバーレイの層、アプリの自作の重ね合わせ）では直らず、規則が 2 つになる。

### 影の型

- **A. `paint.BoxShadow` を 1 つの型にする。** `paint.Gradient` と `paint.Stroke` と同じ所で、`Canvas.DrawShadow` と並ぶ。`theme` は `paint` を型のためだけに import する（`text` も同じ形で `paint` を使っている）。
- **B. `geom` に置く。** `geom` は大きさと位置と色の基本の型で、影の意味（CSS の box-shadow）を持ち込むことになる。
- **C. `theme.Shadow` を残し、`layout` が `theme` を import する。** `layout` は `theme` を知らない層であり（CODING_STYLE §3.1）、許されない。

## 決定

- 押下のフォーカスは案 A。`focusOnPress` は、`CanFocus` が false の物と隠れた `Hider` の中の物を飛ばし、外へ進む。
- 当たり判定は案 A。規則は次の通りである。
  - `layout` の `RenderAlign`（`Align`、`Center`）、`RenderPadding`、`RenderConstrainedBox`（`SizedBox`）、`RenderAspectRatio`、`RenderFlex`（`Row`、`Column`）、`RenderFlexible`、`RenderStack`、`RenderPositioned`、`RenderPositionedFill`、`RenderWrap`、`RenderGrid`、`RenderCustomLayout`、`RenderTranslate`、`RenderTransform`、`RenderOpacity`、`RenderScale`、`RenderShaderMask`、`RenderRing`、`RenderSemantics`、`RenderExcludeSemantics`、`RenderFocus`、`RenderFocusOrder`、`RenderShortcuts`、`RenderModal`、`core.RenderRepaintBoundary`、`basic.LayoutBuilder` は、子が当たる所でしか当たらない。一覧は `layout/hit.go` にまとめた。
  - `RenderDecoratedBox` は、飾りが何かを描くとき（`Color` の不透明度が 0 より大きい、`Gradient`、`Border`、`Shadows` のどれか）は箱の全体で当たり、何も描かないときは子の所でしか当たらない。色を付けた `Container` やカードは今まで通り下に押下を通さない。
  - 文字、画像、アイコン、描くコントロールと、ポインターを受け取る物（`RenderListener`、`RenderGesture`、`RenderMouseRegion`、`RenderBarrier`、`RenderViewport` など）、各パッケージの自作の RenderObject は変えない。範囲の全体で当たる。
  - `IgnorePointer` は子の全体で通す。`Stack` の中では `Positioned` や `PositionedFill` の内側に置く。これは `IgnorePointer` と `basic.Stack` の godoc に書いた。
  - 規則は `basic.Stack` と `basic.Container` の godoc と、レイアウトのガイド（en、ja）の 1 段落に書いた。
- 説明は godoc に書く。`basic.Container` には、子も大きさも無いときは緩い制約の下で `Padding` の分しか場所を取らないこと（Flutter と違う）を書いた。`interact.GestureDetector` には、`core.LocalToGlobal(ctx.RenderObject(), pos)` でウィンドウの座標に、`core.GlobalToLocal` で別のウィジェットの座標に、`core.LocalRectToGlobal` で矩形を変えることを書いた。`control.CustomPaint` には、`Stack` の中で項目ごとに `Positioned` と `interact.Semantics`（`Role`、`Label`、`Value`、`State`、`OnTap`）を重ね、その中に `interact.Focus{Enabled: true}` を置けば Tab で項目の間を移り、`OnShowFocus` でフォーカスの輪を描く項目が分かることを書いた。層は何も描かないので、キャンバスの `GestureDetector` にポインターが届く。
- 影は案 A。`paint.BoxShadow`（`Color`、`Offset`、`Blur`、`Spread`）を足し、`theme.Shadow` と `layout.BoxShadow` と `kit.ShadowsOf` を無くした。`theme.Colors.Shadows` は `[]paint.BoxShadow` を返し、`Container`、`layout.Decoration`、各コントロールにそのまま渡せる。gowaku はまだ公開していないので、互換のための別名は置かない。CODING_STYLE（en、ja）の層の図を `theme ─→ text, paint (types only), geom` にした。

## 結果

- 全面の層の空いた所で、押下が下の層に届く。テストは、`Stack` の下に全面の `GestureDetector`、上に (1) `PositionedFill` と `Align` の下中央のボタン、(2) 全面の `Column` の下端のボタン、(3) `PositionedFill` の中の `IgnorePointer` と `Center` の文字を置き、空いた所のタップが下に届き、ボタンのタップはボタンに届くことを確かめる。色を付けた `Container` の層は、その箱の上のタップを下に通さず、箱の外では通すことも確かめる。
- `core` のテストは、フォーカスを取れない `FocusTarget` を飛ばして外側の物がフォーカスを取ることを確かめる。`interact` のテストは、`Keys: ViewerKeysNone` の `InteractiveViewer` の子の `Listener` が受け取った押下で、外側の `Focus` がフォーカスを取ることを確かめる。
- `control` のテストは、`CustomPaint` の godoc の形で、項目が名前と役割と Tap の操作を持ち、項目の上のタップがキャンバスの `GestureDetector` に届き、Tab が項目を順に移ることを確かめる。
- 当たり判定の経路（`HitTestResult.Path`）から、子の当たらない配置だけの物が消える。経路を読む物（押下、ホバー、カーソル、ドロップ、戻る・進むのボタン、タイトルバーの判定）は、ポインターを受け取る物を探すので、結果は変わらない。Inspector の要素の選択は、空いた所では外側の当たる物を選び、無ければ何も選ばない。
- 当たり判定は型の確認を 1 回足すだけで、割り当ては無い。
