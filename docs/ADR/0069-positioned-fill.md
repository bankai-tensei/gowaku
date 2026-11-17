# 0069: PositionedFill（Stack の両端に張る子）

決定日: 2026-11-09

状態: 採用

## 背景

- `Positioned` は 1 つの軸につき 1 つの端からしか置けない（Left か Right、Top か Bottom）。大きさは Width / Height で固定するか、子に任せるかのどちらか。
- このため、大きさを Stack の非 Positioned の子が決める Stack に、全高の線や全面のスクリムを重ねる方法が無かった。
- サンプルのアプリで、タイムラインの上に「現在」の線を引こうとして諦めた。

## 決定

- 新しいウィジェット `widgets.PositionedFill{Insets layout.Insets; Child core.Widget}` を足した。ルートには `positioned_fill.go` に別名 `gowaku.PositionedFill` を置いた。
- 子には「Stack の大きさから Insets を引いた大きさ」の tight 制約を渡し、`(Insets.Left, Insets.Top)` に置く。
  - ゼロ値は Stack 全体を覆う。
  - Insets が Stack より大きいときは、その軸の大きさを 0 にする（負の大きさは作らない）。負の Insets は外へはみ出すだけで、そのまま許す。
- `Positioned` と同じく、Stack の大きさには関わらない。Stack の大きさは非 Positioned の子（と `Fit`）で決まり、その後で PositionedFill の子を並べる。
  - 子が非 Positioned だけなら、これまでと何も変わらない。
  - PositionedFill だけの Stack は、親が許す最小の大きさになる（Positioned だけの Stack と同じ）。
- 描画側は `layout.RenderPositionedFill`。`RenderPositioned` と同じく、RenderStack が型で見分ける目印の RenderProxy にした。
  - RenderStack は 1 回目のループで両方を飛ばし、2 回目のループの型 switch で置く。新しい割り当ては無く、Layout は 0 allocs/op のまま（`TestRenderStack_LayoutWithPositionedFillAllocatesNothing`）。
  - Insets が変わったときは、Positioned と同じく親の Stack に MarkNeedsLayout を出す。
- ヒットテストは RenderBase のままで、置いた位置と大きさで当たる。

## 検討したほかの案

- **Positioned に「両端」のフラグを足す**: Left と Right が両方 0 のときに「左端」と「両端」を区別できず、ゼロ値の意味が変わる。フラグを増やすと組み合わせが読みにくくなる。Positioned の意味は変えないことにした。
- **Flutter の `Positioned.fill` / Left と Right の両方指定**: Go にはフィールドの「未指定」が無いので、ポインターかフラグが要る。値型・ゼロ値で使える方針に合わない。
- **LayoutBuilder で Stack の大きさを読んで Positioned に渡す**: 1 フレーム遅れるか、二度の Layout が要る。

## 残したこと

- 片方の軸だけ両端に張り、もう片方は端から固定の幅で置く（例: 縦は全高で、横は Right から 8px の幅 2px）は、PositionedFill の Insets で書ける（Left = 幅 − Right − 2）が、Stack の幅を知っている必要がある。要望が出たら、軸ごとの指定を考える。
