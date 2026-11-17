# 0030: CustomPaint を、作り直しもレイアウトもせずに描き直す RepaintSignal

決定日: 2026-11-08

状態: 採用

## 背景

音楽プレーヤーの見本アプリのスペクトル表示は、描く数値だけを 1 秒に 30 回変える。描き直させる手段が `SetState` しか無かったので、`s.SetState(func(){})` を呼んでいた。

- `SetState` は部分木を作り直し、`CustomPaint.UpdateRenderObject` が通るので描き直しにはなる。
- ただ、Build・reconcile・（寸法が同じでも）painter の付け替えが毎回走る。描画の範囲を絞るために `RepaintBoundary` で包む工夫も要った。
- Flutter は `CustomPainter(repaint: Listenable)` でこれを解く。render object が Listenable を聞き、`markNeedsPaint` だけを呼ぶ。

gowaku にはすでに同じ形がある。`layout.ScrollOffset` は State が持って動かし、`RenderViewport` が `offsetChanged` で直接 `MarkNeedsPaint` する。作り直しは起きない。

## 選択肢と決定

### 1. 何を State に持たせるか

- (a) 汎用の `Listenable`（コールバックの登録・解除）を `core` に置く。
  - 何にでも使えるが、登録のたびに関数値を持つことになる。比較できない関数値を外すための ID も要る。
  - 今の用途は「描き直す」1 つだけで、汎用にする理由がまだ無い。
- (b) `layout.RepaintSignal` を置く。持つのは聞いている render object の一覧だけで、`Notify` はその全部に `MarkNeedsPaint` を呼ぶ。

**(b) を採る。** `ScrollOffset` と同じく `layout` に置き、State が値で持ってアドレスを渡す。ゼロ値で使える。関数値を持たないので、`Notify` はアロケーションしない（ベンチで 0 allocs/op を確認した）。用途が増えたら、そのとき (a) を考える。

### 2. 1 つの signal を何個の render object が聞けるか

- (a) `ScrollOffset` のように 1 個だけ。
- (b) 何個でも。左右のチャンネルを別々の `CustomPaint` で描くような使い方がある。

**(b) を採る。** 一覧はスライスで持つ。伸びるのは付けたときだけで、`Notify` では伸びない。

### 3. 外すこと

- 作り直しで別の signal や nil を渡されたら、`UpdateRenderObject` から呼ぶ `RenderCustomPaint.SetRepaint` が古い signal から外し、新しい signal に付ける。同じ signal なら何もしない。
- render object が木から外れたことを知らせる口は `core` に無い。口を足すと `core` の `RenderObject` の約束が広がる。
  - 代わりに、`Notify` のときに `Attached()` でない render object を一覧から落とす。外れた render object への `MarkNeedsPaint` はもともと何もしないので、落とすのはメモリを残さないためだけ。
  - 生成直後は `CreateRenderObject` と木への attach の間に何も走らないので、まだ attach されていない render object を誤って落とすことは無い。

### 4. スレッド

`Notify` は UI ゴルーチンからだけ呼ぶ。ticker やイベントの中が想定。ほかのゴルーチンの値は、`SetState` と同じく App を通して State に届けてから `Notify` する。gowaku の木はどこもロックを持たないので、`RepaintSignal` だけを安全にはしない。この決まりは doc に書く。

## 影響

- `widgets.CustomPaint` に `Repaint *layout.RepaintSignal` が増えた。ゼロ値（nil）なら今までと同じ。
- `layout.RenderCustomPaint` に `SetRepaint` / `Repaint` が増えた。
- ルートの `gowaku` パッケージに `RepaintSignal` を再エクスポートした。
- 見本アプリ の `visualizer` は `SetState(func(){})` をやめ、`Notify` に置き換えられる。
- 今のところ signal を聞けるのは `RenderCustomPaint` だけ。ほかの render object にも要るようになったら、未公開のインターフェース `repaintView` を満たせば付けられる。
