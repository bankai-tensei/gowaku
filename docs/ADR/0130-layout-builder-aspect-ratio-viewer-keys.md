# 0130: 制約で構築する LayoutBuilder、ウィンドウの大きさ、AspectRatio、画像ビューアのための InteractiveViewer の設定

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人が画像ビューアを作ったところ、次の 4 つが見つかった。

1. **InteractiveViewer の設定不足：** godoc は `interact.InteractiveViewer` を「画像ビューアの土台」と呼んでいるが、画像ビューアに要る設定が無かった。
   - ホイールはパンに使われ、拡大縮小には Ctrl が要る。画像ビューアの多くは修飾キーなしのホイールで拡大縮小する。
   - フォーカスを持つと矢印キーを常に使う（ADR 0092）。拡大していない画像、つまりパンする余地が無いときでも矢印キーを取るので、アプリの「左右キーで前後の画像」のショートカットが動かない。
   - Tab キーでのフォーカス移動から外す方法が無い。
   - 中に置いた `interact.Listener` が押下を取り、ビューアのドラッグが始まらない。この振る舞いはどこにも書かれていなかった。
2. **大きさに応じて構築する手段が無い：** ウィジェットはレイアウトより前に構築されるので、自分の大きさを知らない。利用者は自分の大きさを知るためだけに `CustomLayout` の Delegate を書いていた。Flutter の `LayoutBuilder` にあたるものが無い。
3. **AspectRatio が無い：** 16:9 の動画や正方形のサムネイルの枠を作るには、`CustomLayout` か `LayoutBuilder` で計算するしかなかった。
4. **制約の型が分かりにくい：** `layout.Constraints` が `geom.Constraints` の別名であることがガイドに書かれておらず、利用者は `c.Max` や `layout.Tight` を推測で書いていた。
5. **ウィンドウの大きさが読めない：** 「ウィンドウが狭いときはサイドバーを畳む」という単純な切り替えにも、LayoutBuilder（または Delegate）が要った。オーナーから、`basic.MediaQueryData` にウィンドウの大きさを足すよう求められた。条件は、Run が設定して大きさの変化に追従すること、大きさの変化で再構築されるのは MediaQuery に依存するウィジェットだけであること、何も起きていないときの負荷がゼロであること、gowakutest が `Options.Width`/`Height` から設定すること。

## 選択肢

### ホイールでの拡大縮小

- **A. `WheelZooms bool` を足す。** 修飾キーなしの縦のホイールで、Ctrl+ホイールと同じ倍率（1 ノッチで 1.1 倍）でポインタを中心に拡大縮小する。Shift+ホイールと横のスクロールはパンのまま残す。
- **B. 整数ノッチのホイールだけ拡大縮小し、タッチパッドの細かいスクロールはパンにする。** 高精度ホイールのマウスもタッチパッドと同じく端数を送るので、両者を確実には区別できない。マウスによって振る舞いが変わってしまう。

### 矢印キーとフォーカス

- **A. 動かせないときは矢印キーを取らない（既定の振る舞いを変える）。** 子が収まっていてパンの余地が無いとき、または端に着いていてその方向へ動けないとき、キーを処理済みにせず、祖先の `Focus` やアプリの `Actions` に渡す。ノードエディタのような境界の無いビューアは常に動けるので、これまでと変わらない。
- **B. 矢印キーを使わない設定と、フォーカスを取らない設定を別々の bool にする。** 規約（§4.1）で真偽値は肯定形にするが、ゼロ値で今の振る舞いを保つには `NoArrows` や `SkipTab` のような否定形になる。
- **C. 1 つの列挙型 `ViewerKeys` にする。** `ViewerKeysAll`（ゼロ値。今の振る舞いに A を加えたもの）、`ViewerKeysZoom`（矢印キーは常にアプリに渡し、+、-、0 だけ使う）、`ViewerKeysNone`（キーを使わず、フォーカスも取らない）。キーを使わないビューアがフォーカスを持つ理由は無いので、「Tab から外す」は `ViewerKeysNone` に含める。
- **D. core に「押せばフォーカスを取るが Tab では止まらない」仕組みを足す。** HTML の `tabindex="-1"` にあたる。core の `FocusTarget` とフォーカス順の収集に手を入れることになり、今回の需要（画像ビューアのショートカット）には大きすぎる。

### Listener と押下

- **A. Listener は既定で押下を取らず、求められたときだけ取る。** 描画のパッドのように、押下から離すまでの動きを受け取る使い方が主であり、既定で取らないとそれらがすべて壊れる。
- **B. 今の振る舞い（PointerDown を処理済みにしてから `OnPointer` を呼ぶ）を保ち、文書にする。** `OnPointer` が `e.Handled` を false に戻せば、押下は外側（ビューアのドラッグなど）へ渡る。この仕組みは既に動いていたが、書かれていなかった。

### LayoutBuilder の構築の時機

- **A. core にレイアウト中の構築を足す。** Flutter の `invokeLayoutCallback` にあたる。RenderObject の `Layout` の中から要素の再構築を呼べるようにする。構築中の RenderObject の付け替え、`MarkNeedsLayout` の扱い、`inBuild` の判定などに手を入れることになる。
- **B. collection.List の仕組みを再利用する。** List は、レイアウトで自分の高さを知るとコールバックで State に伝え、State が `SetState` する。フレームは描画の前に `NeedsBuild` を見て、構築とレイアウトをやり直す（最大 3 回）。LayoutBuilder も同じく、RenderObject が `Layout` で受け取った制約を State に渡し、前回と違えば再構築を求める。core に変更は要らない。最初のフレームと制約が変わったフレームでは、構築とレイアウトが 1 往復増える。入れ子の LayoutBuilder は 1 段ごとに 1 往復増えるので、4 段以上入れ子にすると、内側が 1 フレーム遅れて表示される。

### AspectRatio の置き場所

- **A. `basic.AspectRatio` を `basic.LayoutBuilder` などで組み立てる。** 余計な往復が増える。
- **B. `layout.RenderAspectRatio` を足す。** Flutter の `RenderAspectRatio` と同じ手順で大きさを決める。使える幅いっぱい（幅が無限なら高さいっぱい）から始め、最大値に収まるよう縮め、最小値に届くよう広げる。tight な制約の下では比を反映しない。

### ウィンドウの大きさ

調べると、次の 2 点が問題になった。

- ルートの再構築は木全体の再構築になる。これまで明るさやロケールが変わると、ルートの State が `SetState` し、Home 以下のすべての要素が更新されていた（要素の更新は、ウィジェットが前と同じ値でも必ず Build を呼ぶ）。めったに起きない変化なら構わないが、ウィンドウの大きさはドラッグの間、毎フレーム変わる。
- `MediaQueryOf` に依存するウィジェットが多い。標準のウィジェットの 37 か所が、`ReduceMotion` や `Locale` を読むためだけに `MediaQueryOf` を呼んでいる。その中にはルートの直下にある `Localizations` も含まれ、これが再構築されると、やはり木全体が更新される。

選択肢:

- **A. `MediaQueryData.Size` を足し、1 つの Inherited で配る。** 大きさが変わるたびに、上の 37 か所すべてと、`Localizations` 以下の木全体が再構築される。
- **B. 大きさを別のウィジェット（`basic.WindowSizeOf` だけ）で配り、`MediaQueryData` には入れない。** オーナーの求めた形と違い、MediaQuery を差し替えて小さな画面を試す、といった使い方で大きさだけ別扱いになる。
- **C. `MediaQueryData.Size` を足すが、MediaQuery はそれを別の Inherited で配る（Flutter の `MediaQuery.sizeOf` と同じ考え方）。** `Inherited[MediaQueryData]` には Size を除いたものを、非公開の `Inherited[mediaSize]` に Size を入れる。`MediaQueryOf` は両方に依存して完全な値を返す（`MediaQueryOf(ctx).Size` が正しく動く）。一部だけを読む `WindowSizeOf`、`ReduceMotionOf`、`LocaleOf` を足し、標準のウィジェットの 37 か所は後の 2 つに置き換える。加えて、ルートの State は MediaQuery より下の部分（Localizations、Theme、Overlay、タイトルバー、Home）をポインタのウィジェット `*rootBody` として持ち、大きさが変わったときは同じポインタを渡す。core は「同じポインタのウィジェットは更新しない」とする（Flutter が同一のウィジェットのインスタンスで再構築を省くのと同じ）。値のウィジェットはこれまでどおりで、ポインタのウィジェットを使っているコードは無い。

## 決定

- InteractiveViewer に `WheelZooms bool`（A）と `Keys ViewerKeys`（C）を足す。既定の矢印キーは、動かせないときに取らない（矢印キーの A）。`ViewerKeysNone` では `CanFocus` が false になり、Tab で止まらず、押してもフォーカスは元の場所に残る。スクリーンリーダーからの拡大縮小は引き続き使える。
- Listener の振る舞いは変えず（B）、`interact.Listener`、`layout.RenderListener`、`InteractiveViewer` の godoc に、押下を取ることと、`e.Handled` を false に戻せば外側へ渡ることを書く。
- `basic.LayoutBuilder{Content func(ctx core.BuildContext, c layout.Constraints) core.Widget}` を足す。名前とフィールドは `basic.Builder{Content}` にそろえる。構築の時機は B（List の仕組みの再利用）とする。中身を構築し直すのは、制約が変わったとき、親が構築し直したとき、`ctx` から読んだもの（テーマなど）が変わったときだけである。制約が変わらないフレームではメモリを割り当てない。
- `basic.AspectRatio{Ratio, Child}` と `layout.RenderAspectRatio`、計算を切り出した `layout.AspectRatioSize` を足す（B）。`Ratio` のゼロ値は 1（正方形）とし、負の値と、幅も高さも無限の制約では panic する。
- ウィンドウの大きさは C とする。`MediaQueryData.Size` は内容の領域（gowaku が描くタイトルバーを除く）の論理ピクセルである。Run はウィンドウを開いたときと大きさが変わったときに設定し、大きさが変わっただけなら `rootState.resized` で同じ `*rootBody` を渡す。表示倍率やタイトルバーの状態など、それ以外の変化はこれまでどおり `refresh` で木全体を構築し直す。gowakutest は `Options.Width`/`Height` を Size に入れる（1 フィールドの追加だけにとどめた）。オーバーレイの入口（callerScope）は Size を含まない MediaQueryData だけを運び、大きさはオーバーレイより上の MediaQuery から届く。
- 使い分けは、ウィンドウ全体に関わる判断（サイドバーを畳むなど）には `WindowSizeOf`、ウィンドウの一部の大きさ（ドラッグで幅を変えるペイン、どこにでも置けるカードなど）には `LayoutBuilder` とし、レイアウトのガイドに書く。
- レイアウトのガイドに、`layout.Constraints` が `geom.Constraints` であること、`Biggest`、`Constrain`、`Loosen`、`Deflate`、`IsTight`、`geom.Tight`、`geom.Loose` を書く。

## 結果

- 画像ビューアは `WheelZooms: true, Keys: interact.ViewerKeysZoom` で、ホイールで拡大縮小し、左右キーをアプリのショートカットに残せる。
- 既定の `ViewerKeysAll` でも、拡大していない画像では矢印キーがアプリに届く。境界のあるビューアでは、端までパンした後の矢印キーもアプリに届く。キーを押し続けたまま端に着くと、そこからアプリのショートカットが動き始める。これを避けたいアプリは `ViewerKeysZoom` を使う。
- Tab では止まらないが押せばフォーカスを取る、という中間の設定は無い。必要になったら core に D の仕組みを足す。
- LayoutBuilder の中身は、レイアウトが届くまで構築されない。最初の `Build` と `Layout` だけを 1 回ずつ呼ぶテスト（`wtest.Rects` など）では中身が空になる。`NeedsBuild` を見て繰り返すフレーム（アプリ、gowakutest、testutil.RenderTree）では、描画の前に中身がそろう。
- 中身を構築し直すと、そのフレームで子のレイアウトがもう 1 度行われる。子は一度、前の制約向けに構築されたまま新しい制約でレイアウトされるので、その間の描画は行われないが、子の `Layout` が 2 回走ることはある。
- ウィンドウの大きさが変わったフレームで再構築されるのは、ルートの数個の要素と、`WindowSizeOf` または `MediaQueryOf` を呼んだウィジェットだけになる。何も起きていないウィンドウでは何も構築しない。
- `MediaQueryOf` は大きさの変化でも再構築される。MediaQuery を差し替えるために `MediaQueryOf(ctx)` を写して一部を変えるコード（examples/gallery と examples/notes のロケールの差し替えなど）は、大きさが変わるたびにその下を再構築する。正しさを優先してそのままにし、一部だけ要るウィジェットには `ReduceMotionOf` と `LocaleOf` を勧める。
- core に「同じポインタのウィジェットは更新しない」という規則が加わった。`core.Widget` の godoc に書いた。
