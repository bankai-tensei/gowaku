# 0006: core の API は当初の案から 5 点変える

決定日: 2026-11-07

状態: 採用

## 背景

Phase 2 で当初の設計と CODING_STYLE §15 のコード片を実装すると、Go の型の制約で成り立たない箇所があった。

## 決定

### Widget は `Key()` だけを持ち、core が型スイッチで振り分ける

当初の案の `createElement()` は非公開メソッドなので、`widgets` パッケージの値型は実装できない。埋め込みで渡す案も、埋め込まれた側が外側の型を知らないので成り立たない。そこで `Widget` は `Key()` だけにして、`inflate` が `StatefulWidget` / `StatelessWidget` / `RenderWidget` を型スイッチで見分ける。どれでもなければ panic する。型スイッチが走るのは Element の生成時だけで、reconcile の経路には入らない。

子は `SingleChildWidget.ChildWidget()` と `MultiChildWidget.ChildWidgets()` で取る。Element は `*element` 1 種類で、`kind` で分岐する。

### `Keyed` は普通の StatelessWidget

`Keyed(k, w)` は、キーを持ち `Build` で `w` を返すだけの非公開の型を作る。reconcile に特別扱いが要らない。同じキーで中身の型が変わると、ラッパーの Element は残り、その下だけが作り直される。

### 親は `core.LayoutChild(child, c)` を呼ぶ

規約 §15 の `r.child.Layout(inner)` では、キャッシュと relayout boundary を挟む場所が無い。Go には仮想呼び出しが無いので、埋め込みの `RenderBase` からも割り込めない。`Layout` は実装者が書くメソッドのままとして、親からの呼び出しを `LayoutChild` に通す。クリーンで制約が同じならレイアウトをせず前回のサイズを返す。規約 §15 の例はこの ADR が上書きする。

### Constraints と Insets は geom に置く

`RenderObject.Layout` のシグネチャが `Constraints` を使うが、`core` は `layout` を import できない。依存の無い値型なので `geom` に置き、`layout.Constraints` と `layout.Insets` は型別名として残す。規約の対応表はそのまま成り立つ。

### ParentData の代わりにマーカーの RenderObject

Flutter の `ParentData` は全 RenderObject に型無しの枠を持たせる（規約 §5.2 の `any` 禁止に触れる）。`Flexible` と `Positioned` は薄い `RenderFlexible` / `RenderPositioned` になり、親は子の型アサーションで読む。RenderObject が 1 つ増えるが、型は失わない。Stateless な Widget で包んでも RenderObject の親子関係は変わらないので動く。

## その他の小さな決定

- `Alignment` は 0〜1 の割合で、ゼロ値が左上。Flutter の -1〜1 だとゼロ値が中央になり、`Stack{}` が左上にならない。
- `Inherited[T comparable]`。値が変わったときだけ依存先を再ビルドするため `comparable` を要求する。`DependOn[T](ctx)` は Go にジェネリックメソッドが無いので関数にした（当初の案の `ctx.DependOn[T]()` から変更）。
- `SizedBox` の 0 は「その軸は自由」で、`ConstrainedBox` の最大値 0 は「上限なし」。ゼロ値で使えることを優先した。
- Widget の同一性による再ビルド省略（Flutter の `identical`）は無い。値型の比較は `Children` のスライスで panic するため。親が再ビルドすると下は全部 reconcile する。1 万ノードで 0.19ms なので今は問題にならない。このため `Inherited` の変更通知は今のところ効果が見えない。親の再ビルドが下を全部更新するので、依存先は通知が無くても再ビルドされる。通知は省略を入れるときに要る部分として残してある。
- `gowaku.Post` は `currentApp` を通す。パッケージ変数の例外は `doc.go` に列挙してある。
- `-tags gowakudebug` の SetState ゴルーチン検査は入れていない。規約 §9 の `goroutineToken` は、呼び出し元のゴルーチンを知る手段が無いと比較できない。方式を決め直す必要がある。

## 結果

イベントループ `run` は `platform.Platform` と `gpu.Backend` のインターフェースだけに依存するので、`app_test.go` は偽物の OS と GPU を使って `go test` で「アイドル時に描かない」ことを検証できる。
