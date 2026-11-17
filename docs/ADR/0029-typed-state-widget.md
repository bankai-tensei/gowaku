# 0029: State から自分の Widget を型付きで読む `StateOf[W]`

決定日: 2026-11-08

状態: 採用

## 背景

公開 API だけで音楽プレーヤーの見本アプリを書いたところ、どの State も次の 1 行を何度も書いていた。

```go
w := s.Widget().(visualizer)
```

- `Build`、`InitState`、`DidUpdateWidget`、tick のハンドラーと、State のメソッドごとに同じ型アサーションが要る。ライブラリの `widgets` でも約 60 か所あった。
- 型を間違えても、そのメソッドが初めて走るまで分からない。tick のハンドラーのように、まれにしか通らない道で落ちる。
- `StateBase.Widget()` は `core.Widget` を返すので、エディタの補完も効かない。

Go のジェネリクスで、`StateBase` と `Widget()` を壊さずに型付きの道を足したい。

## 制約

- **メソッドは型パラメータを持てない。** `func (b *StateBase) WidgetAs[W]() W` は書けない。型パラメータを置けるのは、関数か型。
- **`CreateState` は `core.State` を返す。** State インタフェースは全部の State で共通。`DidUpdateWidget(old Widget)` の引数の型は変えられない。
- **ホットパスでアロケーションしない。** `Build` のたびに読むので、interface から値型の widget を取り出すときに確保してはいけない。
  - 型アサーションで値を取り出すのは、箱の中身をスタックへコピーするだけ。確保は起きない。
  - これをベンチで確かめる。
- **既存のコードはそのまま動く。** `StateBase` を埋め込んで `s.Widget().(T)` と書くコードは変えない。

## 選択肢

### (a) ジェネリックな関数 `core.WidgetOf[W](s)`

```go
w := core.WidgetOf[visualizer](&s.StateBase)
```

- `core` に関数を 1 つ足すだけで済む。
- 呼ぶたびに型引数と `&s.StateBase` を書く。今の `s.Widget().(visualizer)` より長く、繰り返しは減らない。
- 型の誤りは、今と同じく呼んだときまで分からない。

### (b) 埋め込むジェネリックな基底 `StateOf[W]`

```go
type visualizerState struct {
	gowaku.StateOf[visualizer]
	...
}

w := s.Widget() // visualizer
```

- 型を書くのは、State の定義の 1 か所だけ。Flutter の `State<T extends StatefulWidget>` の `widget` と同じ形。
- `StateOf[W]` は `StateBase` を埋め込み、`Widget() W` で `StateBase.Widget()` を覆う。
  - `SetState` / `Context` / `NewTicker` などは、`StateBase` からそのまま昇格する。
  - `&s.StateBase` を受け取る既存の補助（`motion.Implicit.Attach` など）も、そのまま渡せる。
- State がツリーに入るとき（`attach`）に、widget の型が `W` かを 1 回だけ確かめる。
  - 違えば、`gowaku: X.CreateState returned a State embedding StateOf[Y]; ...` で panic する。
  - 誤りは、どの道を通るかに関係なく、最初のマウントで必ず見つかる。
  - その後の `Widget()` は失敗しない。
- メソッド名は `Widget` のままにする。
  - `W()` や `Props()` のように名前を変えると、`StateBase.Widget()` と並んで 2 つになり、どちらを使うか迷う。
  - 同じ名前で型だけが変わるほうが、Flutter の `widget` に近い。
  - 型の無い値が要るときは、`s.StateBase.Widget()` で取れる。

### (c) `DidUpdateWidget` の古い widget も型付きにする

- State インタフェースの `DidUpdateWidget(old Widget)` を変えると、既存の State がすべて壊れる。
- 壊さずにやるには、`StateOf[W]` が `DidUpdateWidget` を実装する。そのうえで、外側の State にある `DidUpdate(old W)` のような別名のメソッドを、interface のアサーションで探して呼ぶ。
  - しかし、同じ出来事に 2 つの名前ができる。
  - 外側が `DidUpdateWidget` を定義すると、`DidUpdate` は黙って呼ばれなくなる。
  - 更新のたびに、動的な検索が 1 回増える。
- 一方で、reconciler は同じ型の widget どうしでしか更新しない。だから `old.(W)` は決して失敗しない。

## 決定

**(b) の `StateOf[W]` を採る。** (c) は採らず、`old` は `Widget` のままにする。`old.(W)` は失敗しないことを doc に書く。

- `core/stateof.go` に `StateOf[W Widget]` を置く。
  - `attach` を覆って、型を確かめる。
  - `Widget() W` は `s.el.widget.(W)` を返す。
- ルートの `gowaku` パッケージに、ジェネリックな型エイリアス `StateOf[W Widget] = core.StateOf[W]` を足す（Go 1.24 以降）。
- 見本として `widgets.AnimatedOpacity` / `widgets.AnimatedAlign` の State と、`examples/multiwindow` の 2 つの State を書き換えた。

### 計測

i7-12700H、`go test ./core -bench 'StateOf|StateBase_' -benchmem`。他の作業と並行して測ったので、時間は揺れる。

| ベンチ | ns/op | allocs/op |
|---|---|---|
| `StateOf_Widget`（`s.Widget()` で 48 バイトの widget を読む） | 2.3 | 0 |
| `StateBase_WidgetAssert`（従来の `s.Widget().(T)`） | 2.5 | 0 |
| `StateOf_Rebuild`（`SetState` → `Build` で widget を読む） | 51〜250 | 0 |

`Widget()` は、従来の型アサーションと同じ速さで、確保もしない。`TestStateOf_WidgetDoesNotAllocate` でも `testing.AllocsPerRun` が 0 になることを確かめている。

## 影響

- 既存の `StateBase` と `Widget()` は変えていない。書き換えは任意で、少しずつ移れる。
- `StateOf[W]` を埋め込んだ State を、別の型の widget の `CreateState` から返すと、マウントの時点で panic する。これまでは、そのメソッドが走るまで落ちなかった。
- `widgets` の残りの State（Button、Slider、Scroll など）と 見本アプリ は、まだ `StateBase` のまま。必要に応じて移す。
