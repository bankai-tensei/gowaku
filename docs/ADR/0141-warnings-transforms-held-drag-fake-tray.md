# 0141: Warnings は描画の変形をたどる、ドラッグを途中で止める、中ボタンと条件で探す、テスト用のトレイと通知

決定日: 2026-11-17

状態: 採用

## 背景

初めて使う人に `gowakutest` を試してもらったところ、次のことが見つかった。

1. **変形の下での誤った警告：** `Warnings` は、RenderObject の `Offset` を足し合わせた配置の位置で文字を探していた（ADR 0137 の結果に書いたとおり、変形は無視していた）。ズームした `interact.InteractiveViewer` の中では、文字をズーム前の位置で測り、そこに実際に描かれていた別のものと比べたので、1.4〜4.3:1 の誤ったコントラストの警告が出た。`basic.Translate` や `basic.Scale` のように、レイアウトもポインターも動かさず絵だけを動かす部品でも、同じことが起こる。
2. **Tester に足りない操作：** `Drag` と `DragAt` は押下から離すまでを一度に行うので、ドラッグの途中の画面を撮ったり、途中の状態を `Find` で調べたりできなかった。中ボタンのタップが無かった。ラベルを問わず、ある役割のノードをすべて探す手段が無かった（`FindAllRole` はラベルが必須）。ガイドの Tester の表には戻り値が無く、`FindAll` がスライスを返すことや、`SaveScreenshot` が何も返さず失敗するとテストを落とすことが分からなかった。
3. **トレイと通知がテストできない：** `gowaku.ShowTray` と `gowaku.Notify` は `currentApp` から窓のアプリを探し、`Run` の外では panic した。ADR 0137 でクリップボードは `ctx.Host()` を通すようにしたが、トレイと通知は引数に `BuildContext` が無いので、同じ経路を通せなかった。
4. **説明の不足：** ダイアログは閉じた後、120 ms ほどかけて消えていき、それが終わるまでオーバーレイに残る。どこで `Advance` が要るかが書かれていなかった。木を壊すときは、親の State が先にツリーから外れた後で子の `Dispose` が呼ばれることがあり、子の `Dispose` から親のコールバックを通して `SetState` を呼ぶと panic する（「not in the tree」）が、`Dispose` の godoc に書かれていなかった。

## 選択肢

### 変形の扱い

- **A. 祖先の変形をたどって、描かれた位置で調べる。** `core.Transformer`（`InteractiveViewer`、`Transform`、`DockLayout` の箱）の `ChildTransform` は今もある。絵だけを動かす `RenderTranslate` と `RenderScale` には、その変形を外から読む手段が無いので、`core` に読むための小さなインターフェースを足す。
- **B. 変形の下にあるものは調べない。** 実装は簡単だが、ズームしたビューアーの中の本当に読みにくい文字も見逃す。
- **C. `gowakutest` で `RenderTranslate` と `RenderScale` の型を個別に見る。** `core` は変わらないが、絵だけを動かす部品を足すたびに `gowakutest` を直す必要がある。

### 途中で止めるドラッグの形

- **A. `Press(label)`、`PressAt(pos)`、`MoveTo(pos)`、`Release()` を Tester のメソッドにする。** `Tap` と `TapAt` と同じ並びで、押下中かどうかは Tester が覚える。
- **B. `PressAt` がドラッグの値（`*Drag`）を返し、その `MoveTo` と `Release` を呼ぶ。** 押下を 2 つ同時に持てるように見えるが、ポインターは 1 つしか無い。
- **C. `DragAt` にコールバックの引数を足し、途中で呼ぶ。** 引数で振る舞いを変える形になり、パッケージの流儀に合わない。

### 条件で探す

- **A. `FindFunc(match)` と `FindAllFunc(match)` を足す。** 標準ライブラリの `strings.IndexFunc` と同じ名前の形で、役割だけで探すことも、値を持つ入力欄を探すことも 1 つで書ける。
- **B. `FindAllRole(role)` のラベルを省ける形にする。** Go には省略できる引数が無く、空のラベルはラベルの無いノードを探す意味と区別できない。

### トレイと通知の経路

- **A. `ShowTray(ctx, opts)` と `Notify(ctx, title, body)` にし、`ctx.Host()` を通す。** `Host` が `platform.Trays` を実装していれば使い、していなければ `ErrNoTray` / `ErrNoNotifications` を返す。窓の `windowHost` は OS のトレイへ取り次ぎ、`gowakutest` の `host` は記録する。クリップボードと同じ経路になり、`Run` の外でも panic しない。
- **B. 引数はそのままにし、`gowakutest` が `currentApp` に偽物のアプリを置く。** パッケージの変数を書き換えることになり、テストを並列に走らせられない。

## 決定

1. 変形は案 A。`core.PaintTransformer`（`PaintTransform() geom.Affine`）を足し、`layout.RenderTranslate` と `layout.RenderScale` が実装する。`core` 自身は使わない。`Warnings` の走査は、足し合わせた `Offset` の代わりに、窓への変形（`geom.Affine`）を持って下りる。各物の矩形はその変形で移した外接矩形、`Clipper` の切り取りも同じく変形して重ねる。子へは `ChildTransform`、次に `PaintTransform` を掛ける。文字は描かれた位置で、描かれた大きさで測る。ズームで 18.66 px 以上に見える文字は大きな文字として 3:1 で判定する。はみ出しの量は窓のピクセルで報告する。回転や傾きで文字が斜めになるときは、外接矩形に周りのものが入るので、コントラストを判定しない（誤って報告するより、測らない方を選ぶ）。
2. 途中で止めるドラッグは案 A。`PressAt` は 1 フレーム描くだけで落ち着くのを待たず、時計を進めない。押している時間はテストが決める（長押しは `Advance`）。`MoveTo` は 1 歩動かして 1 フレーム（`FrameTime`）進める。`Release` は離して落ち着くのを待つ。押下中に `PressAt`、押下の無いときに `MoveTo` や `Release` を呼ぶとテストを落とす。`DragAt` はこの 3 つで書き直し、振る舞いは前と同じ。
3. `MiddleTap(label)` と `MiddleTapAt(pos)` を足す。`click` に `input.ButtonMiddle` を渡すだけである。
4. 条件で探すのは案 A。`FindFunc` と `FindAllFunc` を足す。`Find` と同じく、ダイアログが開いている間はダイアログの中だけを見る。
5. ガイド（en、ja）の Tester の表に戻り値の列を足し、新しい行（`FindFunc`、`MiddleTap`、`Press` ほか、`Trays` ほか）を足した。どのメソッドもエラーを返さず、できないときはテストを落とすことを 1 文で書いた。
6. トレイと通知は案 A。`ShowTray(ctx, opts)` と `Notify(ctx, title, body)` にした。`windowHost` が `platform.Trays` を実装し、OS からのイベントを post の列に積み、作ったトレイを `appTray` として `Run` の終わりに閉じる（今までの `ap.trays` の役目）。`gowakutest` の `host` も `platform.Trays` を実装し、`Tester.Trays()`（`*gowakutest.Tray` のスライス、閉じたものは除く）と `Tester.Notifications()`（`gowakutest.Notification` のスライス）で読める。`Tray` には `Options()`（`platform.TrayOptions`）、`Activate()`、`Choose(label)` があり、`Tester.ClickNotification()` は最初のトレイの `OnNotificationClick` を呼ぶ。`Choose` のラベルはアプリが渡したとおり（ニーモニックの「(&O)」を含む）で、サブメニューの中も探す。どれも呼んだ後で落ち着くのを待つ。
7. ダイアログの消え方を `overlay.ShowDialog` の godoc と、テストのガイドの「ダイアログをテストする」（en、ja）に書いた。閉じるボタンの `Tap` は消えていく間も落ち着くのを待つので、ふつうは `Advance` が要らない。`After` やテストから直接呼んだコードで閉じたとき、`Options.Settle` が 120 ms より短いときは、消えたことを確かめる前に `Advance` を呼ぶ。`core.State.Dispose` と `StateBase.Dispose` の godoc に、親が先に外れていることがあるので、`Dispose` から別の State の `SetState` を呼ぶ前に `Mounted` を確かめることを書いた。

## 結果

- ズームしたビューアーの中で、誤ったコントラストの警告が出ない。テストは、暗い帯と明るい帯を縦に並べてビューアーで 2 倍にし、ズーム前の位置で測ると暗い帯の上になる明るい帯の文字が報告されないこと（変形を無視する走査では 1.10:1 で報告されることを確かめた）、パンした後も同じこと、ズームした薄い文字がセマンティクスの矩形と同じ、ズームした位置と大きさで報告されること、`Translate` で暗い帯の上に絵だけ動かした文字がそこで報告されること、回転した文字は判定しないことを確かめる。
- ドラッグの途中の画面を撮れる。テストは、`GestureDetector` で `Press` の後はドラッグが始まらず、2 回の `MoveTo` の後は始まって 40,10 動き、終わっておらず、`Release` で終わること、`Slider` の値が離す前に動くことを確かめる。中ボタンのタップと `FindAllFunc` にもテストがある。
- `ShowTray` と `Notify` の引数が変わった。gowaku はまだ公開していないので、互換の仕組みは置かない。ガイドの例と `examples/dragdrop` を直した。`Run` の外でも panic せず、`Host` がトレイを持たなければエラーを返す。テストは、ボタンでトレイを出し、メニューの項目の数と無効の項目を読み、`Activate`、`Choose` で通知を出し、`ClickNotification`、サブメニューの項目でトレイを閉じると `Trays()` から消えることを確かめ、`Host` の無い木では `ErrNoTray` と `ErrNoNotifications` が返ることを確かめる。
- `Warnings` は、`core.Transformer` と `core.PaintTransformer` を実装しない描き方の変形（`Hero` の飛行中の拡大など、自分の子ではない物を描くもの）はたどらない。回転した文字のコントラストは判定しない。
