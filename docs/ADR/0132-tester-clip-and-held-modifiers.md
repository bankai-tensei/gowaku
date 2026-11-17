# 0132: Warnings はクリップで隠れたものを調べない、Tester で修飾キーを押したままにする

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に `gowakutest` を試してもらったところ、次の 3 つが見つかった。

1. **クリップの外の警告：** `Tester.Warnings` が、`Scroll` の表示域の外へスクロールし、上に残る見出しの下に入った行の文字を、コントラスト 1.19:1 として報告した。誰にも見えない行である。`Warnings` の走査は RenderObject の矩形をウィンドウと重ねるだけで、祖先がクリップしているかを知らなかった。コントラストも文字の箱全体で測るので、半分だけ見える行は、隠れた側に重なる見出しの色を背景として測っていた。はみ出しとラベルの無い部品も同じく、見えない所のものまで報告していた。
2. **修飾キー：** `TapAt`、`DragAt` などは修飾キーを取らず、Shift クリック、Ctrl クリック、Ctrl ドラッグを試すには `tt.Tree().DispatchPointer` でイベントを自分で組み立てる必要があった。`Scroll` と `Key` だけが `mods` を引数に持っていた。
3. **ドラッグの元：** `interact.Draggable` を `Drag` で動かしても何も起きないことに戸惑った人がいた。`Draggable` は `core.DragHost` を通して OS のドラッグを始めるが、Tester にはその受け手が無い。

## 選択肢

### クリップの知り方

- **A. 表示リストを見る。** `Canvas.DrawGlyphs` はクリップの外の文字を記録しないので、RenderParagraph のグリフの列が表示リストに無ければ見えない、と判定できる。部品の手書きのクリップにも自動で効く。ただし文字にしか使えず、`Row` のはみ出しやラベルの無い部品は描画命令と RenderObject を結び付けられない。レイヤーの中の座標の扱いも要る。`GridView` や `Tree` の本体のように、表示域の外の子をそもそも描かない部品もある。
- **B. 子をクリップする RenderObject が `core.Clipper`（`ChildClip() (geom.Rect, bool)`）を実装し、`Warnings` は走査で祖先のクリップを重ねていく。** RenderObject ごとに判定できるので、3 種類の警告すべてに同じ規則が効く。クリップする RenderObject ごとに短いメソッドが要り、実装を忘れた部品では今までどおり報告される（誤報の側に倒れる）。
- **C. 意味ツリーに「画面外」の印を持たせる。** UIA の IsOffscreen に当たり、将来は役に立ちうるが、意味ツリーを作る側にクリップの知識が要るのは B と同じで、今回の範囲を超える。

### 修飾キーの持たせ方

- **A. すべてのポインターのメソッドに `mods` 引数を足す。** `Tap(label)` のよく使う形が `Tap(label, 0)` になり、ほとんどのテストで読みにくくなる。
- **B. `TapWith` などの別名を足す。** メソッドの数が倍になる。
- **C. `Hold(mods)` で修飾キーを押したままにし、後に続く操作がそれを運ぶ。** 実際の操作（Shift を押しながらクリックする）と同じ順に書け、既存の呼び出しは変わらない。関数オプションでもメソッドチェーンでもない。

## 決定

1. `core.Clipper` を加える（選択肢 B）。`ChildClip` は子を切り取る矩形を自分の座標で返し、今は切り取らないときは false を返す。フレームの外でしか呼ばれないので速さは問わない。子をクリップする次の RenderObject が実装する: `layout.RenderViewport`、`layout.RenderList`、`layout.RenderTranslate`（`Clip` で動いているとき）、`GridView`・`ReorderableList`・`Tree` の本体、`Disclosure` の本体（開閉の途中、閉じていれば空の矩形）、`InfoBar` の枠（伸びる途中）、`InteractiveViewer`、分割ペインの枠、`DockLayout` の行と枠、`Navigator` のページの枠（ずれているとき）、`StatusBar` の行、`TabBar` のタブの列。
2. `Warnings` は根からウィンドウの矩形をクリップとして持ち、`Clipper` の祖先ごとに重ねる。クリップの外に丸ごとある RenderObject は、はみ出しもコントラストも調べず、その RenderObject が開いた意味ツリーのノードはラベルの無い部品として報告しない。一部だけ見える文字は、見えている部分だけで背景を測る。大きさの無い RenderObject は、その位置がクリップの中なら見えているとする。
3. `Tester.Hold(mods input.Modifiers)` を加える（選択肢 C）。次の `Hold` まで、タップ、`Hover`、`Drag`、ドロップのイベントがその修飾キーを運び、`Scroll` と `Key` は自分の `mods` に足す。`Hold(0)` で離す。キーの押下そのものは送らない。
4. `Draggable` が Tester では何もしないこと、代わりに `DropText`・`DropFiles` とその `At` 形で `DragTarget` にドロップすることを、`DropFiles` の godoc とテストのガイドに書く。

## 結果

- 表示域の外へスクロールした行、見出しの下に入った行、閉じた `Disclosure` の中身は警告に出ない。表示域に戻れば、また報告される。
- 新しい部品が子をクリップするときは `core.Clipper` を実装する。忘れても警告が増えるだけで、見える問題を見落とすことはない。部品の中で子を移動・変形して描く場合（`Navigator` のずれ、`InteractiveViewer` の拡大）は、`Warnings` が今までどおり変形を無視して配置の位置で判定する。
- Shift クリックの範囲選択、Ctrl クリックの追加選択、Ctrl ドラッグを、`tt.Hold(input.ModShift)` の 1 行を足すだけで書ける。`Hold` の状態はテストの中で続くので、使い終えたら `Hold(0)` で戻す。
