# 0025: アプリを 1 本書いて見つかった穴を埋める

決定日: 2026-11-08

状態: 採用

## 背景

リポジトリの外に、公開 API だけでポモドーロタイマー（`../gowaku-pomodoro`）を書いた。画面を組む部分は困らなかったが、次の 6 点で回り道をした。

1. `Center` の中の `Column` が上に詰まる。`Column` は既定で縦いっぱいに広がるので、`Center` には寄せる余白が残らない。README と `examples/counter` の `Center{VStack(...)}` も同じ理由で上に寄っていた。
2. 1 秒ごとの更新を `time.AfterFunc` → `gowaku.Post` → 世代番号で古い予約を捨てる、という形で自前で書いた。`core.Ticker` の `SleepUntil` で書けるが、毎回の計算と、`Dispose` での `Stop` の書き忘れが利用者に残る。
3. 円形の進捗を描く手段がなく、`Path` に点を並べて `Icon` に流し込んだ。
4. 整数を増減する部品が無く、ボタン 2 つで代用した。（`Slider` の `Min` / `Max` / `Step` / `Snap` は既にあり、こちらは私の見落としだった。）
5. `internal/shot` が internal なので、外のアプリからは見た目を確かめられない。実際の窓に `PostMessage` を送り、`PrintWindow` で撮った。DPI の扱いで一度クリックが外れた。
6. テーマに前向きな状態の色が無く、休憩の緑を決め打ちした。窓を閉じる直前に保存する場所も無かった。

## 選択肢と決定

### 1. Column と Center

- (a) `Column` の既定を `MainMin` にする。`Expanded` を含む既存のコードがすべて壊れる。
- (b) `Center` が子に高さを渡さない。Flutter と違う規則が増え、`Center` の中で `Expanded` が使えなくなる。
- (c) `VStack` / `HStack` を中身の大きさに縮める。SwiftUI の `VStack` と同じ意味になり、名前から期待される動きに揃う。`Column` / `Row` / `Center` の doc に理由と回避策を書く。

**(c) を採る。** `VStack` はこれまで `Column{Children: ...}` と同じだったので挙動の変更になるが、使っていたのはリポジトリ内では counter の例とテストだけで、どちらも縮むことを期待していた。

### 2. 周期的な Ticker

- (a) `widgets` に `Timer` ウィジェットを足す。`core` / `widgets` でゴルーチンを起動しない規約（§9）に沿うには結局 Ticker で書くことになり、ウィジェットにする理由が無い。
- (b) `core.StateBase.NewPeriodicTicker(period, fn)` を足し、`Tree.Tick` の中で次の呼び出しまで自動で `SleepUntil` する。

**(b) を採る。** 窓は呼び出しの間ずっと眠り、待ち時間は既存の `Tree.NextWake` から `app.waitTime` に届く。2 回目以降は最初の呼び出しを起点にした格子に乗るので、フレームが遅れてもずれが積み重ならない。止まっていた間の呼び出しは追いかけずに飛ばす（最小化から戻った時計が 100 回鳴らないように）。

あわせて、**State が作った Ticker は、その State がツリーから外れたときに `Dispose` の直後に止まる**ようにした。`Ticker` に作った要素を持たせ、`element.unmount` で外す。`Dispose` で `Stop` を呼ぶ既存のコードはそのまま動く。リストはスクロールのたびに行を外すので、ここはクロージャを使わずに書き、`BenchmarkListView_ScrollFrame` のアロケーション数（22）が変わらないことを確かめた。

### 3. 自由な描画と進捗表示

`layout.Painter` インターフェース（`Paint(cv, sz)`）と `widgets.CustomPaint` を足した。`layout.Delegate` / `CustomLayout` と同じ形で、`PainterFunc` で関数も渡せる。Painter は状態を隠せるので、更新のたびに描き直す。

その上に `ProgressBar` と `ProgressRing` を足した。リングは 4 分の 1 周以下の 3 次ベジェで円弧を近似し、端は丸く、満タンのときは内周を逆回りにした 2 本の輪で穴を開ける（非ゼロ規則）。パスは値・直径・太さが変わったときだけ作り直し、Canvas が次の Reset まで指す間は手を付けない。Paint はアロケーションゼロ（`BenchmarkRenderProgressRing_Paint`）。値の変化は `SnappySpring` で滑らせる。アクセシビリティには新しい役割 `RoleProgress` で百分率を出す。

### 4. Stepper

`widgets.Stepper` を足した。整数の `Value` / `Min` / `Max` / `Step` と `Format`。`Max` は `Min` より大きいときだけ上限になる（ゼロ値で上限なし）。端を越えるボタンは無効になる。

### 5. 公開テストハーネス

- (a) `internal/shot` を公開する。実際の窓と GPU が要り、CI で動かない。
- (b) CPU の `gpu/soft` で描く新しいパッケージ `gowakutest` を足す。

**(b) を採る。** `gowakutest.New(t, root, opts)` は `Run` と同じ Theme / MediaQuery / Overlay を根に置き、窓にフォーカスがある状態で始める。時計は偽物で、`Advance` はアニメーション中は 16ms ごと、眠っている Ticker は起きる時刻ごとにフレームを回す。部品はアクセシビリティツリーのラベルで探す（`Find` / `FindAll` / `Dump`）。同じラベルのテキストと部品があれば部品を選ぶので、フィールドの見出しで `Tap` するとフィールドに届く。`Tap` / `Type` / `Key` のあとは落ち着くまで回す。`Screenshot` は GPU と同じシェーダーを CPU で動かした絵を返す。

フォントは既定でシステムのものを使う（日本語のため）。どの機械でも同じ絵が要るなら `Options.Fonts` で固定する。`gowaku.Post` は `Run` の外では捨てられる、という既存の約束はそのまま残し、doc に書いた。

`gowakutest` は公開 API に加える（ADR 0016 の表を更新）。`gpu/soft` は内部のまま。

### 6. Success と OnClose

`theme.Colors.Success` を足した。色相 145・彩度 48 のパレットで、ライト 45、ダーク 72。カード上の文字として 4.5:1 以上、ライトでは白い文字を載せても 4.5:1 以上あることをテストで押さえた。

`WindowOptions.OnClose` と `App.OnClose` を足した。利用者が閉じたとき、`Close` / `Quit` のとき、どれでも、ツリーを破棄する前に UI ゴルーチンで呼ぶ。閉じるのを取り消す仕組み（「保存しますか」）は、`Quit` との関係を決める必要があるので今回は入れない。

## 結果

- 追加した公開 API: `core.StateBase.NewPeriodicTicker`, `core.RoleProgress`, `layout.Painter`, `layout.PainterFunc`, `layout.RingPath`, `layout.RenderProgressBar`, `layout.RenderProgressRing`, `layout.RenderCustomPaint`, `widgets.CustomPaint`, `widgets.ProgressBar`, `widgets.ProgressRing`, `widgets.Stepper`, `theme.Colors.Success`, `gowaku.WindowOptions.OnClose`, `gowaku.App.OnClose`, パッケージ `gowakutest`。ルートから `Canvas`, `Paint`, `Painter`, `PainterFunc`, `CustomPaint`, `ProgressBar`, `ProgressRing`, `Stepper` を再エクスポートした。
- 挙動の変更: `VStack` / `HStack` が中身の大きさに縮む。State の Ticker はツリーから外れると止まる。ADR 0016 は公開 API を「追加のみ」と約束しているが、v1 はまだタグを打っていないので、この 2 点はその前の修正として扱う。
- ポモドーロのアプリは、自前のタイマー・リング・増減ボタン・保存のタイミングを捨てて、これらに置き換えた。

## その後

同じ日に、より大きな見本（音楽プレーヤーの見本アプリ）を書いて見つかった点を ADR 0026〜0034 で直した。長さが決まらない置き方の panic（0026）、隠れた部分木（0027）、`Pressable`（0028）、`StateOf`（0029）、`RepaintSignal`（0030）、`TextStyle.With*` と名前付きの `Alignment`（0031）、4 ストップのグラデーション（0032）、`Wrap` / `Grid`（0033）、行ごとの高さの `ListView`（0034）。
