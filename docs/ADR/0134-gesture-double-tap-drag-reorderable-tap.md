# 0134: GestureDetector のダブルタップとドラッグ、ReorderableList のタップ、押下とフォーカスの説明

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次のことが分かった。

1. **ダブルタップとドラッグが無い：** `interact.GestureDetector` にはタップ、右クリック、長押し、ピンチしか無い。ダブルクリックを取るには `interact.Listener` で `PointerEvent.Clicks == 2` を調べるしかなく、アプリの中で物を動かすドラッグ（キャンバスの図形、ハンドル）は、`Listener` で移動量を自分で計算していた。
2. **OS のドラッグ＆ドロップへの誤解：** `interact.Draggable` / `DragTarget` を、アプリ内でゴーストを描きながら並べ替える仕組みだと思って使い、ドラッグ中の位置が取れないと戸惑った。
3. **並べ替えとクリックの両立：** `collection.ReorderableList` の項目に `Pressable` や `OnTap` 付きの `GestureDetector`、`OnPress` 付きの `SidebarItem` を置くと、押下をそちらが取るので項目を持ち上げられない。クリックでき、かつドラッグで並べ替えられる行を作る方法が分からなかった。
4. **フォーカスと押下の誤解：** `interact.Focus` はクリックではフォーカスを取らないと思われていた。また、祖先の `Listener` に子のボタンの押下が届くかどうかが godoc から読めなかった。

コードを確かめた結果は次のとおり。

- 押下（`PointerDown`）は、ヒットテストの経路を深い方から順に渡り、最初に `Handled` にしたもの 1 つだけが取る。その後の移動と離す操作は、取ったものにだけ届く（`core.Tree.pointerDown` / `captured`）。ボタンを持たない移動（ホバー）は、どの `PointerTarget` にも届かない。ホイールとピンチは、誰かが `Handled` にするまで経路を上る。
- 押下を取ったものから上へたどり、最初に見つかった `FocusTarget` がフォーカスを取る（`focusOnPress`）。したがって `Focus` は、中に押下を取るもの（`GestureDetector`、`Listener` など）があればクリックでフォーカスを取る。中が `Container` だけなら取らず、何も無い所の押下はフォーカスを外す。
- タッチでは、指が `panSlop`（8）を越えて動くと、押下を取ったものが `core.PanYielder` で許せば、押下を取り消してスクロールに変える。`RenderGesture` は常に許していた。

## 選択肢

### ダブルタップと OnTap

- **A. OnTap を待たせない。** 1 回目の押下は即座にタップ、2 回目（`Clicks` が偶数）を離したときに OnTap の代わりに OnDoubleTap を呼ぶ。ダブルクリックは「OnTap、OnDoubleTap」の順になる。ファイルの一覧で「選択してから開く」と同じで、Windows・macOS のデスクトップの作法に合う。`collection.Tree` や `GridView` の行も同じ動きである。
- **B. OnDoubleTap があるときは OnTap をダブルクリックの間隔だけ遅らせ、ダブルクリックでなければ呼ぶ。** Flutter の作法。シングルクリックの反応が 0.5 秒ほど遅れ、ボタンのような部品では重く感じる。
- **C. 両方を選べる設定を足す。** 設定が増えるわりに、B を要る場面は少ない。

### ドラッグの名前と形

- **A. `OnDragStart(pos)`、`OnDragUpdate(pos, delta)`、`OnDragEnd(pos)`。** 位置は子からの相対、`delta` は前回からの変化。開始の位置は押した場所で、閾値を越えた直後に、そこまでの移動を 1 回の OnDragUpdate で渡す。`Draggable` の `OnDragStart` / `OnDragEnd` と名前は同じだが、`GestureDetector` の godoc に「アプリ内のドラッグで、システムのドラッグ＆ドロップは `Draggable`」と書く。
- **B. Flutter の `OnPanStart` などにする。** gowaku の利用者には「pan」は `InteractiveViewer` の意味で通っており、紛らわしい。
- **C. イベントの構造体（位置、変化、開始からの距離、デバイス）を渡す。** 引数は 2 つまでなので、CODING_STYLE §5.1 では関数の引数で足りる。足りなくなったら構造体に変える（未公開なので変えられる）。

### ドラッグと他の操作

- 閾値は長押しの許容量と同じ 8 とする。8 以下の動きはタップのままである。閾値を越えると長押しを止め、OnTapCancel を呼び（押下の見た目を戻せるように）、離したときに OnTap を呼ばない。長押しが発火した後の押下はドラッグにしない。
- 押下が取り消されたとき（タッチの 2 本目の指など）は、最後の位置で OnDragEnd を呼ぶ。取り消しと離した操作を分けるコールバックは足さない。要る例がまだ無い。
- ドラッグのコールバックがあるときだけ `YieldsToPan` を false にする。タップだけの `GestureDetector` を指でなぞればこれまでどおり周りの `Scroll` が動き、ドラッグを持つものは指を自分で使う。マウスのドラッグは `Scroll` が使わないので影響しない。
- 中のボタンが押下を取ったときは、`GestureDetector` にドラッグは届かない。経路の深い方が押下を取る規則のままにする。

### ReorderableList の行のクリック

- **A. `ReorderableList.OnTap func(i int)` を足す。** 動かさずに離した押下が同じ項目の上なら呼ぶ。フォーカスのある項目での Enter と Space、支援技術の `ActionTap` でも呼ぶ。押下はリストが取ったままなので、持ち上げとクリックが両立する。
- **B. `Listener` で `Handled = false` にする方法を説明するだけにする。** `Listener` には押下（`PointerDown`）しか届かず、離した位置が分からないので、クリックとドラッグを区別できない。
- **C. 新しい `GestureDetector` を行に置いて共存させる。** `OnTap` を付けると押下を取るので、持ち上げられない。押下を譲ると離す操作が届かない。共存はできない。

## 決定

- `interact.GestureDetector`（と `layout.RenderGesture`）に `OnDoubleTap func()` を足す（案 A）。`PointerEvent.Clicks` が 2、4… の押下を離したときに OnTap の代わりに呼ぶ。OnDoubleTap が無ければ、これまでどおりすべての押下がタップである。
- `OnDragStart func(pos geom.Offset)`、`OnDragUpdate func(pos, delta geom.Offset)`、`OnDragEnd func(pos geom.Offset)` を足す（案 A）。閾値は 8、超えたら OnTapCancel を呼び、タップにも長押しにもしない。ドラッグ中は子の外に出ても追う。ドラッグのコールバックがあるときは指でスクロールさせない。
- `collection.ReorderableList.OnTap func(i int)` を足す（案 A）。
- godoc に次を書く。
  - `Draggable` / `DragTarget`：アプリ内でもシステムのドラッグ＆ドロップで、画像はシステムが描き、ゴーストもホバー中の位置も無い。2 つの一覧の間で動かすなら、各項目を「その前に入れる」`DragTarget` にし、最後に「末尾に足す」ものを 1 つ置く。テストでは `gowakutest.Tester` の `DropText` / `DropTextAt` で落とす。
  - `ReorderableList`：押下を自分で取る中身（`Pressable`、`OnTap` 付きの `GestureDetector`、`OnPress` 付きの `SidebarItem`）の上では持ち上げられない。クリックもさせたいなら、中身に押下を持たせず `OnTap` を使う。
  - `Focus`：クリックでフォーカスを取るのは、中に押下を取るものがあるときだけ。クリックで取らせたいなら、何もしない `OnTapDown` を持つ `GestureDetector` を中に置く。
  - `Listener`：子（ボタン）が取った押下は、その後の移動と離す操作も含めて祖先の `Listener` に届かない。ボタンを持たない移動も届かない。ホイールとピンチは、中で処理されなければ届く。
- カタログの `GestureDetector` の説明と例を新しいコールバックに合わせる。

## 結果

- ダブルクリックとアプリ内のドラッグを `GestureDetector` だけで書ける。`Listener` で `Clicks` や移動量を数える必要は無い。テストは `gowakutest` の `DragAt`、`DoubleTapAt`、`TapAt` で、ドラッグがタップにならないこと、小さな動きはタップのままであること、子の外まで追うこと、ダブルクリックが「タップ、ダブルタップ」になること、離れた 2 回のタップがダブルタップにならないこと、中のボタンが押下を守ること、指のドラッグがコールバックの有無でスクロールかドラッグに分かれることを確かめる。
- 代償として、OnDoubleTap を持つものでは、ダブルクリックの 1 回目も OnTap を呼ぶ。1 回目を無視したい用途（シングルとダブルで全く別のことをする）は、アプリ側で遅らせる必要がある。
- ドラッグの速度（フリック）は渡さない。慣性の要る用途は `InteractiveViewer` か `Scroll` を使う。
- 並べ替えられる行をクリックでも開ける。Enter と Space は、これまで何もしなかったので、既存の動きは変わらない。
- `Focus` と `Listener` の godoc の説明を、実際の動き（`focusOnPress` と、押下を 1 つのものだけが取る規則）に合わせ、テストで確かめた。
