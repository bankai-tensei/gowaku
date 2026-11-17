# モーション

[ガイドの目次](README.md) · [English](../en/motion.md)

gowaku にはアニメーションの仕組みが組み込まれており、新しい値へ自動で遷移するウィジェットから、自分で駆動するコントローラやスプリングまでそろっている。このガイドでは簡単なものから順に、手動で制御するものへと進む。カーブ、スプリング、コントローラは `motion` パッケージに、アニメーションするウィジェットは `widgets/basic` に、ページ遷移は `widgets/nav` にある。

## 時間の流れはどこから来るか

アニメーションはすべて `core.Ticker` の上で動く。Ticker は動作中にフレームを要求し、動きが終点に達すると停止するので、ウィンドウは再び何も描画しない状態に戻る。`motion` パッケージ自身は時計を読まず、時間はすべて Ticker から受け取る。そのため `gowakutest` では、テストの中で偽の時計を進められる（[テストとプレビュー](testing-and-preview.md)）。

以下で紹介するものはすべて、次の 2 つのルールに従う。

- 新しい目標値が与えられると、その時点の値から動き始める。動きの途中で割り込んでも値が飛ぶことはない。
- ユーザーが OS で動きを減らす設定にしていると、`basic.ReduceMotionOf(ctx)` が true になる。標準ウィジェットはこのとき移動やサイズ変化のアニメーションをやめて即座に切り替え、フェードだけを残す。独自のアニメーションでも同じように対応するとよい。

## 暗黙アニメーション

暗黙アニメーションは、フィールドが変わったときに値を一気に切り替えず、時間をかけて新しい値へ遷移するウィジェットである。いつもどおり `SetState` の中でフィールドを変更すれば、あとはウィジェットが処理する。

| ウィジェット | アニメーションする属性 |
|---|---|
| `AnimatedContainer` | サイズ、余白、色、角丸、枠線、影。幅や高さが 0 のときはアニメーションしない。 |
| `AnimatedOpacity` | 不透明度。0 のとき、子はヒットテスト、Semantics、フォーカスの対象から外れる。 |
| `AnimatedAlign` | 子の配置の位置。 |
| `AnimatedPositioned` | `Stack` の子の位置。 |

いずれも `Duration`（0 なら 200 ms）と `Curve`（nil なら `motion.Standard`）のフィールドを持つ。

```go
type panel struct{ gowaku.StatefulBase }

type panelState struct {
	gowaku.StateBase
	open bool
}

func (panel) CreateState() gowaku.State { return &panelState{} }

func (s *panelState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	height, color := float32(48), th.Color.Fill
	if s.open {
		height, color = 160, th.Color.AccentTint
	}
	return basic.Column{Size: layout.MainMin, Spacing: theme.Space8, Children: []gowaku.Widget{
		control.Button{Label: "Toggle", OnPress: func() { s.SetState(func() { s.open = !s.open }) }},
		basic.AnimatedContainer{
			Duration: 250 * time.Millisecond,
			Curve:    motion.EaseOutCubic,
			Height:   height,
			Color:    color,
			Radius:   th.Radius.Group,
		},
		basic.AnimatedOpacity{Opacity: boolTo1(s.open), Child: basic.Text{Content: "Details"}},
	}}
}

func boolTo1(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
```

標準のウィジェットも同じ仕組みで動いている。スイッチはつまみがスライドし、ボタンは押すと沈み込み、ダイアログはフェードしながら拡大して現れる。

## 子を切り替える

`AnimatedSwitcher` は、古い子から新しい子へアニメーションしながら切り替える。型かキーが異なる子が新しい子と見なされるので、同じ型の子を切り替えるときは `gowaku.Keyed` でキーを付ける。

```go
type tabsState struct {
	gowaku.StateBase
	page, dir int // the page shown, and +1 or -1 for the way it went
}

func (s *tabsState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.AnimatedSwitcher{
		Duration:   250 * time.Millisecond,
		Transition: basic.SlideTransition(0.04*float32(s.dir), 0),
		Child:      gowaku.Keyed(s.page, s.body(s.page)),
	}
}
```

動き方は `Transition` で指定する。

- 既定の `basic.FadeTransition` はクロスフェードする。
- `basic.SlideTransition(dx, dy)` は、新しい子を、子のサイズに対する割合 `dx, dy` だけずれた位置からフェードインさせながらスライドさせ、古い子を逆方向に送り出す。0.03〜0.06 程度にすると控えめなずれになる。戻るときは `dx` を負にする。
- `basic.ScaleTransition(from)` は、ウィンドウが開くときのように、新しい子を `from` の倍率から元のサイズへ拡大する。0.96 にすると穏やかな動きになる。

独自の遷移は `basic.SwitcherTransition` として書く。これは子、進行度 `t`、入ってくる側かどうかを受け取り、そのフレーム用に子を包んだウィジェットを返す関数である。どの `t` に対しても同じ型のウィジェットを返さないと、動きの途中で子の State が失われてしまう。後述する描画だけの変形ウィジェットを組み合わせて作るとよい。

## カーブ

`motion.Curve` は、アニメーションの進行度（0〜1）を値の進行度に変換する。

| カーブ | 動き方 |
|---|---|
| `Standard` | 画面内で始まり画面内で終わる動き。暗黙アニメーションの既定値。 |
| `Enter`, `Leave` | 入ってくるもの: 速く現れてゆっくり落ち着く。出ていくもの: ゆっくり動き出す。 |
| `Linear` | 一定の速さ。 |
| `EaseInCubic`, `EaseOutCubic`, `EaseInOutCubic` | 一般的なイージング。 |
| `EaseOutBack`, `SpringOut` | 終点を少し行き過ぎてから戻る。 |
| `ElasticOut`, `BounceOut` | はじいた弦のように揺れる。終点で跳ね返る。 |

`motion.CubicBezier(x1, y1, x2, y2)` は、CSS と同じように制御点からカーブを作る。

```go
var gentle = motion.CubicBezier(0.2, 0, 0, 1)
```

## アニメーションコントローラ

脈打つ、回転する、合図に応じて再生するなど、「新しい値へ遷移する」以外の動きには `motion.AnimationController` を使う。これは 0 から 1 までの値を時間をかけて変化させるものである。フィールドを設定し、`InitState` で `Attach` を、`Dispose` で `Dispose` を呼び出す。

```go
type pulse struct{ gowaku.StatefulBase }

type pulseState struct {
	gowaku.StateBase
	c motion.AnimationController
}

func (pulse) CreateState() gowaku.State { return &pulseState{} }

func (s *pulseState) InitState() {
	s.c.Duration = 800 * time.Millisecond
	s.c.Curve = motion.EaseInOutCubic
	s.c.OnChange = func() { s.SetState(func() {}) }
	s.c.Attach(&s.StateBase)
	s.c.Repeat(true)
}

func (s *pulseState) Dispose() { s.c.Dispose() }

func (s *pulseState) Build(gowaku.BuildContext) gowaku.Widget {
	scale := motion.LerpFloat(0.9, 1.1, s.c.Value())
	return basic.Scale{Scale: scale, Origin: layout.AlignCenter, Child: basic.Text{Content: "Live"}}
}
```

- `Forward()` は 1 へ、`Reverse()` は 0 へ向かって動かし、`Restart()` は 0 から 1 へ動かし直す。`Repeat(bounce)` は `Stop()` を呼ぶまで繰り返す。`SetValue(v)` はアニメーションせずに値を直接設定する。
- `Value()` はカーブを適用した進行度を、`Progress()` は適用前の進行度を返す。`Status()` は `motion.Dismissed`、`Forwarding`、`Reversing`、`Completed` のいずれかを返す。
- `OnChange` は毎ステップ、`OnStatus` は状態が変わったときに呼び出される。`ReverseDuration` と `ReverseCurve` を設定すると、戻るときに往路の設定の代わりに使われる。
- `Attach` は `motion.TickerSource` を受け取る。State なら `StateBase`、RenderObject なら `RenderBase` を渡す。

値を描画に使う量に変換するには、補間関数 `motion.LerpFloat`、`LerpColor`、`LerpOffset`、`LerpSize`、`LerpRect`、`LerpRRect`、`LerpInsets`、`LerpAffine` を使うか、`motion.Tween[T]{Begin, End, Lerp}` の `At(t)` を使う。

## スプリング

スプリングは、時間の長さが決まったカーブに沿うのではなく、ばねにつながれたおもりのように動く。新しい目標値が与えられてもその時点の速度を引き継ぐので、素早く押して離しても動きが途切れずに滑らかにつながる。`motion.SpringValue` は、このように目標値を追いかける数値である。

```go
type knob struct{ gowaku.StatefulBase }

type knobState struct {
	gowaku.StateBase
	x  motion.SpringValue
	on bool
}

func (knob) CreateState() gowaku.State { return &knobState{} }

func (s *knobState) InitState() {
	s.x.Spring = motion.BouncySpring()
	s.x.OnChange = func() { s.SetState(func() {}) }
	s.x.Attach(&s.StateBase, 0)
}

func (s *knobState) Dispose() { s.x.Dispose() }

func (s *knobState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Column{Size: layout.MainMin, Children: []gowaku.Widget{
		control.Button{Label: "Move", OnPress: func() {
			s.on = !s.on
			s.x.Set(boolTo1(s.on)) // starts the motion; OnChange rebuilds
		}},
		basic.SizedBox{Width: 240, Height: 32, Child: basic.Stack{Children: []gowaku.Widget{
			basic.Positioned{Left: 200 * s.x.Value(), Child: basic.Container{
				Width: 32, Height: 32, Radius: 16, Color: th.Color.Accent,
			}},
		}}},
	}}
}
```

- `Set(target)` は目標値を設定する。`OnChange` を呼び出さないので、`Build` の中で呼んでも安全である。
- `Jump(v)` は値をすぐにその位置へ移す。`Kick(velocity)` は値をはじいて元の位置へ戻らせる。`Moving()` は動作中かどうかを返す。
- `Spring` フィールドは物理パラメータを `motion.Spring{Mass, Stiffness, Damping}` で保持する。ゼロ値のときに使われる `motion.SnappySpring()` は行き過ぎずに素早く落ち着くので、ポインタで操作するものに向いている。`motion.BouncySpring()` は少し行き過ぎるので、位置が移動するウィジェットに向いている。`motion.CriticalSpring(stiffness)` は行き過ぎずに最短で目標に達する。

`motion.SpringSimulation` はその土台となる厳密な運動の計算で、位置と速度から目標へ向かう動きを求める。時間の管理を自前で行うコード向けである。

## 独自の暗黙アニメーション

暗黙アニメーションのウィジェットは `motion.Implicit` で作られている。State がそれを `Attach` し、アニメーションさせるフィールドごとに `motion.Value` で値を追跡し、`DidUpdateWidget` で目標値を設定してから `Commit` を呼び出す。

```go
type glow struct {
	gowaku.StatefulBase
	Level float32
	Child gowaku.Widget
}

type glowState struct {
	gowaku.StateOf[glow]
	m     motion.Implicit
	level *motion.Value[float32]
}

func (glow) CreateState() gowaku.State { return &glowState{} }

func (s *glowState) InitState() {
	s.m.Attach(&s.StateBase, func() { s.SetState(func() {}) })
	s.level = motion.Track(&s.m, s.Widget().Level, motion.LerpFloat)
}

func (s *glowState) DidUpdateWidget(gowaku.Widget) {
	s.level.Set(s.Widget().Level)
	s.m.Commit(300*time.Millisecond, motion.Standard)
}

func (s *glowState) Dispose() { s.m.Dispose() }

func (s *glowState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{
		Color: th.Color.Accent.WithAlpha(s.level.Get()),
		Child: s.Widget().Child,
	}
}
```

`motion.Track` は値を `==` で比較する。スライスなど `==` で比較できない型には、比較関数を渡せる `motion.TrackFunc` を使う。`Commit` は、どの目標値も変わっていなければ何もしない。

## 描画だけの変形

次のウィジェットは描画結果だけを動かし、レイアウトと、多くの場合はポインタの判定も、子が元の位置にあるものとして扱う。そのため、最後に静止する動きに向いており、フレームごとのレイアウトのコストもかからない。

- `basic.Opacity{Opacity, Child}`。
- `basic.Translate{X, Y, Child}`。移動量は子のサイズに対する割合で指定する。
- `basic.Scale{Scale, Origin, Child}`。
- `basic.Transform` と `basic.Rotate(radians, child)`。回転させたウィジェットは、回転後の形の内側だけがヒットテストの対象になる（`examples/rotate`）。

## ページとヒーロー

`nav.Navigator` はページ間の遷移を扱う。既定では、入ってくるページが末尾側の端から数ピクセルスライドしながらフェードインする。`nav.PageRoute.Transition` に独自の `nav.PageTransition` を渡すこともでき、`Instant: true` を指定すればアニメーションしなくなる。

`nav.Hero` は、子をページからページへ飛ばすように移動させる。リストの行にある小さなアイコンと詳細ページの大きなアイコンに同じ `Tag` を付けると、ページが遷移する間に、アイコンが一方のページから飛び立ってもう一方のページに着地する。

```go
func noteRow(ctx gowaku.BuildContext, id int, color geom.Color) gowaku.Widget {
	return basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
		nav.Hero{Tag: id, Child: basic.Container{Width: 24, Height: 24, Radius: 6, Color: color}},
		control.Button{Label: "Open", OnPress: func() {
			nav.NavigatorOf(ctx).Push(nav.PageRoute{Page: notePage{id: id, color: color}})
		}},
	}}
}

type notePage struct {
	gowaku.StatelessBase
	id    int
	color geom.Color
}

func (p notePage) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Column{Size: layout.MainMin, Children: []gowaku.Widget{
		nav.BackButton{},
		nav.Hero{Tag: p.id, Child: basic.Container{Width: 96, Height: 96, Radius: 16, Color: p.color}},
	}}
}
```

Tag は comparable な値で、ページ内で重複してはならない。動きを減らす設定のウィンドウでは、Hero のアニメーションは行われない。全体のコードは `examples/navigator` にある。

## さらに読む

- ウィジェットカタログの[アニメーションのページ](../../catalog/ja/animation.md)。
- `motion` の設計は [ADR 0010](../../ADR/0010-motion-and-theme.md)、切り替えの遷移は [ADR 0037](../../ADR/0037-switcher-transitions.md)。
