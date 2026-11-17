# 基本の考え方

[ガイドの目次](README.md) · [English](../en/concepts.md)

gowaku では、Flutter と同じように UI を宣言的に書く。UI をウィジェットで記述すると、ライブラリが画面をその記述どおりに保つ。このガイドでは、それを構成する要素と、変更が画面に反映されるまでの流れを説明する。

## 3 つのツリー

| ツリー | 寿命 | 役割 |
|---|---|---|
| **Widget** | 1 回の `Build` で作られ、捨てられる | UI の一部を記述する。ただの値。 |
| **Element** | ツリーのその位置にある間 | `State` を保持し、ウィジェットと RenderObject を結び付け、差分比較の単位になる。gowaku 内部の型。 |
| **RenderObject** | Element と同じ | ウィジェットのレイアウト、描画、ヒットテスト、アクセシビリティを担当する。 |

アプリの作者が書くのはウィジェットで、`Build` のたびに新しいウィジェットの値を返す。gowaku はそれを既存の Element と比較し、古いウィジェットと新しいウィジェットで型とキーが同じなら、Element をそのまま残して State を保持したまま新しい値を渡す。異なる場合は古い Element を取り外し、新しい Element を作る。

これらの型は `core` パッケージにあるが、UI のコードでよく使うものはルートの `gowaku` パッケージに別名が用意されているので、UI のファイルで `core` を import することはあまりない。別名があるのは `gowaku.Widget`、`gowaku.BuildContext`、`gowaku.StatelessBase`、`gowaku.StatefulBase`、`gowaku.State`、`gowaku.StateBase`、`gowaku.StateOf`、`gowaku.Key`、`gowaku.Keyed` である。

## ウィジェットは値

ウィジェットは構造体で、リテラルで書く。フィールドがそのウィジェットの API のすべてになる。

```go
control.Button{Label: "Save", Style: control.ButtonPrimary, OnPress: save}
```

- **ゼロ値で動く。** `basic.Padding{}` は余白なしで子を描画し、`basic.Text{}` は何も表示せず、`OnPress` のない `control.Button` は無効状態になる。既定値は各フィールドの doc コメントに書いてある。
- **ビルダーもオプションもない。** `NewButton(...)` のような関数、関数オプション、メソッドチェーンは用意していない。最もよく使う形に限り、`basic.VStack`、`basic.HStack`、`basic.Layers`、`basic.Pad` という 1 行の補助関数がある。
- **フィールドを reflect で走査しない。** gowaku が比較するのはウィジェットの型とキーだけである。

ウィジェットの生成コストは小さいので、`Build` のたびに気にせず作ってよい。

## Stateless なウィジェット

Stateless なウィジェットは、自身のフィールドだけから子を組み立てる。`gowaku.StatelessBase` を埋め込み、`Build` を実装する。

```go
type greeting struct {
	gowaku.StatelessBase
	Name string
}

func (g greeting) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Text{Content: "Hello, " + g.Name, Style: th.Text.Title}
}
```

使うときは `greeting{Name: "Hanako"}` と書く。

## Stateful なウィジェット

Stateful なウィジェットは、再ビルドをまたいで保持される `State` を持つ。ウィジェットは `gowaku.StatefulBase` を埋め込み、`CreateState` で新しい State を返す。State 側は `gowaku.StateBase` を埋め込む。ウィジェットのフィールドを型アサーションなしで読みたい場合は、代わりに `gowaku.StateOf[W]` を埋め込む。

```go
type stepper struct {
	gowaku.StatefulBase
	Step int
}

type stepperState struct {
	gowaku.StateOf[stepper]
	n int
}

func (stepper) CreateState() gowaku.State { return &stepperState{} }

func (s *stepperState) Build(gowaku.BuildContext) gowaku.Widget {
	step := s.Widget().Step // a stepper, no type assertion
	return control.Button{
		Label:   fmt.Sprintf("%d (+%d)", s.n, step),
		OnPress: func() { s.SetState(func() { s.n += step }) },
	}
}
```

State のライフサイクル:

| メソッド | 呼び出されるタイミング |
|---|---|
| `InitState()` | 最初の `Build` の前に 1 回。Ticker やコントローラはここで開始する。 |
| `Build(ctx)` | State がダーティになったとき、または親が再ビルドされたとき。 |
| `DidUpdateWidget(old)` | 親の再ビルドによって Element に新しいウィジェットの値が渡されたとき。`Widget()` は新しいほうを返す。 |
| `Dispose()` | State がツリーから取り外されるときに 1 回。`InitState` で開始したものをここで止める。 |

`StateBase` が `Build` 以外のメソッドの空実装を提供しているので、必要なものだけ書けばよい。

`SetState(fn)` は `fn` を実行し、この State の再ビルドを予約する。守るべきルールが 2 つあり、どちらも違反すると panic する。

- `Build` の中で `SetState` を呼ばない。`OnPress` などのコールバックか Ticker から呼ぶ。
- ツリーから取り外された State で `SetState` を呼ばない。結果が遅れて届く可能性がある場合は `Mounted()` で確認する。

## Ticker

Ticker は、動作している間フレームごとに 1 回関数を呼び出す。State からは `NewTicker` で作成する。一定間隔で変化するものには `NewPeriodicTicker` を使う。

```go
type clock struct{ gowaku.StatefulBase }

type clockState struct {
	gowaku.StateBase
	ticker *core.Ticker
	now    time.Time
}

func (clock) CreateState() gowaku.State { return &clockState{} }

func (s *clockState) InitState() {
	s.now = time.Now()
	s.ticker = s.NewPeriodicTicker(time.Second, func(time.Duration) {
		s.SetState(func() { s.now = time.Now() })
	})
	s.ticker.Start()
}

func (s *clockState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Text{Content: s.now.Format("15:04:05")}
}
```

周期的な Ticker は、呼び出しと呼び出しの間ウィンドウをスリープさせる。最初の呼び出しは 1 周期後ではなく、`Start` の次のフレームですぐに来る。State が作成した Ticker は、その State の `Dispose` の後に自動で停止する。`motion` のアニメーションも Ticker の上に作られている。詳しくは[モーション](motion.md)を参照のこと。

### 時間をおいて実行する

「保存しました」の表示を戻すときのように、時間をおいて一度だけ呼び出すには、State の `After` を使う。戻り値は呼び出しを取り消す関数である。

```go
type saveState struct {
	gowaku.StateBase
	saved bool
}

func (s *saveState) save() {
	s.SetState(func() { s.saved = true })
	s.After(1200*time.Millisecond, func() {
		s.SetState(func() { s.saved = false })
	})
}
```

`After` は関数を UI ゴルーチンで実行するので、その中で `SetState` を呼び出してよい。Ticker の時計に従うので、`gowakutest` の `Advance` がアニメーションと同じように進め（タップ後の待ちはこれを待たない）、待っている間ウィンドウはスリープする。State がツリーを離れると、呼び出しも取り消される。`time.AfterFunc` と `gowaku.Post` の組み合わせではなく `After` を使うこと。前者は実時間で動くのでテストから進められず、`Post` は `Run` の外では関数を破棄する。

## BuildContext

`BuildContext` は、ウィジェットツリー上の位置を表す。`Build` の引数として渡され、State からは `Context()` でも取得できる。ツリーの上位にあるものを読み出す関数は、これを引数に受け取る。

- `basic.ThemeOf(ctx)`: テーマ。
- `basic.MediaQueryOf(ctx)`: 表示倍率、OS が要求する明るさ、動きを減らす設定、ロケール、ウィンドウの大きさ。`basic.WindowSizeOf`、`basic.ReduceMotionOf`、`basic.LocaleOf` はそのうち 1 つを読み、それが変わったときだけ呼び出し側を再ビルドする。
- `nav.NavigatorOf(ctx)`: 最も近い `nav.Navigator`。
- `gowaku.WindowOf(ctx)`: ウィンドウ。

コンテキストから見えるのは自分より上位にあるものだけである。同じ `Build` の中のウィジェットが提供するものを読むには、読む側を `basic.Builder` で包む。`Builder` はその位置に独自のコンテキストを持ち、それを使って中身を組み立てる。

```go
basic.Builder{Content: func(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.Text{Content: "Hi", Style: basic.ThemeOf(ctx).Text.Title}
}}
```

## 継承データ

`core.Inherited[T]` を使うと、値をその下にあるすべてのウィジェットから読めるようになる。`core.DependOn[T]` は最も近い値を探し、値が変わったときに呼び出し側を再ビルドする。`core.Lookup[T]` は再ビルドの登録をせずに探すだけなので、コールバックの中で使う。`ThemeOf` と `MediaQueryOf` もこの仕組みで実装されている。

```go
type user struct{ Name string }

func withUser(u user, child gowaku.Widget) gowaku.Widget {
	return core.Inherited[user]{Value: u, Child: child}
}

type badge struct{ gowaku.StatelessBase }

func (badge) Build(ctx gowaku.BuildContext) gowaku.Widget {
	u, ok := core.DependOn[user](ctx)
	if !ok {
		return basic.Text{Content: "Signed out"}
	}
	return basic.Text{Content: u.Name}
}
```

`T` は comparable でなければならず、変更の有無は `!=` で判定する。素の `string` や `int` ではなく独自の型を使えば、他のパッケージの値と取り違えずに済む。大きな値はポインタで渡し、データが変わったらポインタごと差し替える。

## キー

兄弟の中でのウィジェットの同一性は、型とキーで決まる。キーがなければ子は位置で対応付けられる。ほとんどのツリーはそれで問題ないが、項目が移動するリストでは困ったことになる。たとえば先頭の行を削除すると、その行の State（ホバー、フォーカス、入力途中の文字など）が下の行に引き継がれてしまう。

これを防ぐには、項目ごとに `gowaku.Keyed` でキーを付ける。

```go
type todo struct {
	ID    int
	Title string
	Done  bool
}

func todoRows(items []todo, toggle func(id int, done bool)) []gowaku.Widget {
	rows := make([]gowaku.Widget, len(items))
	for i, it := range items {
		rows[i] = gowaku.Keyed(it.ID, control.Checkbox{
			Label:    it.Title,
			Value:    it.Done,
			OnChange: func(v bool) { toggle(it.ID, v) },
		})
	}
	return rows
}
```

キーには ID や文字列など、comparable な値なら何でも使える。comparable でない値を渡すと `Keyed` が panic する。キーは兄弟の間で重複してはならない。`basic.AnimatedSwitcher` もキーを利用しており、キーが変わった子は新しい子と見なされ、アニメーションで入れ替わる。

## 変更が画面に届くまで

1. コールバックが `SetState` を呼び出し、State がダーティになる。この時点ではまだ何もビルドしない。
2. 次のフレームで、ダーティな State を親から順にビルドする。親が返す新しいウィジェットによって子が更新される。RenderWidget はフィールドを RenderObject に反映し、stateful な子は `DidUpdateWidget` を受け取って再ビルドする。
3. レイアウトが変わった RenderObject を再レイアウトする。起点になるのは、サイズが親に影響しない最も近いオブジェクトである。
4. 変わった部分を描画する。`basic.RepaintBoundary` は子の描画結果を周囲から切り離すので、内側が変われば子だけを再描画し、外側が変わっても子のピクセルを再利用できる。`collection.List` は行ごとにこれを置いている。
5. GPU がフレームを描画する。

フレームの前に何度 `SetState` を呼んでも、ビルドは 1 回にまとめられる。State はツリーのなるべく下のほうに置くとよい。`SetState` はその State と、その `Build` が返すものを作り直すので、変化する箇所の近くにある小さな stateful ウィジェットのほうが、ルートにあるものよりコストが小さい。

### 何もしないときは何も描かない

ウィンドウがフレームを描画するのは、何かが要求したときだけである。要求のきっかけになるのは、入力、`SetState`、ウィンドウの変化、動作中の Ticker だ。時間の経過に沿ってフレームを要求できるのは Ticker だけなので、Ticker が 1 つも動いておらずイベントも来なければ、UI ゴルーチンはイベントループで待機し、CPU を消費しない。アニメーションは終点に達すると Ticker を止めるので、ウィンドウは自然に静止状態に戻る。

## UI ゴルーチン

`gowaku.Run` を呼び出したゴルーチンが UI ゴルーチンになる。`Build`、レイアウト、描画、コールバック、`SetState` はすべてこのゴルーチンで実行されるので、State にロックは必要ない。他のゴルーチンからは `gowaku.Post` で処理を UI ゴルーチンに戻す。

```go
func (s *loaderState) InitState() {
	go func() {
		r := slowFetch() // runs on another goroutine
		gowaku.Post(func() {
			if s.Mounted() {
				s.SetState(func() { s.result = r })
			}
		})
	}()
}
```

`Post` はどのゴルーチンから呼び出しても安全である。`Run` の外で呼び出した場合、渡した関数は破棄される。時間が経つのを待つだけの処理には、テストから進められる `After`（[時間をおいて実行する](#時間をおいて実行する)）を使う。

## RenderObject

ほとんどのアプリでは RenderObject を書く必要はない。標準ウィジェットを組み合わせ、描画には `control.CustomPaint`、子の配置には `basic.CustomLayout`（[レイアウト](layout.md#独自のレイアウト)）を使えば済む。

どうしても必要な場合は、ウィジェットに `core.RenderWidget`（`CreateRenderObject` と `UpdateRenderObject`）を実装し、子を持つなら `core.SingleChildWidget` か `core.MultiChildWidget` も実装する。RenderObject は `core.RenderBase` を埋め込み、`Layout` と `Paint` を実装する。`UpdateRenderObject` では、実際に変わったフィールドについてだけダーティにする。サイズが変わるなら `MarkNeedsLayout`、見た目だけが変わるなら `MarkNeedsPaint` を呼ぶ。`Paint` の中でメモリを割り当ててはならない。`layout` パッケージの RenderObject が参考になる実装例である。

## さらに読む

- `Layout` が受け取る制約については[レイアウト](layout.md)。
- ツリーの API は [ADR 0006](../../ADR/0006-core-tree-api.md)、`StateOf` は [ADR 0029](../../ADR/0029-typed-state-widget.md)。
