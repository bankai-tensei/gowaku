# 0087: Disclosure（アコーディオン）と Link

決定日: 2026-11-13

状態: 採用

## 背景

当初の計画に挙げた 2 つを足す。どちらも、無いと利用者が毎回自作するもの。

- `Disclosure`: 見出しを押すと中身が開閉する。設定・プロパティ・FAQ で毎回要る。今あるものは、`collection.Tree` は重すぎ、`collection.Group` は閉じない。`PropertyGrid` のカテゴリの見出し（`propertyHeader`）は同じ形だが、表の中だけで公開していない。複数を並べて 1 つだけ開く使い方も、同じ部品で書けることが条件。
- `Link`: 本文の中の「開く」「詳しくは」。`semantics.RoleLink` はあり、`nav.Breadcrumb` が内部で使っているが、公開ウィジェットが無い。今は毎回 `Pressable` で書くことになる。

## 決定

### 1. 置き場所: どちらも `widgets/control`

自前の見た目を持ち、`behavior.Control`（ホバー・フォーカス・押下）を使うので `control`（段 3、ADR 0081）に置く。`collection` に置くと `Group` の隣で見つけやすいが、`Disclosure` は表やリストに依らないので、下の段に置けるものは下に置く。`nav.Breadcrumb` も `control` より上なので、後で `Link` を使える。

### 2. Disclosure の API: 制御するかしないかを `OnChange` で分ける

```go
type Disclosure struct {
	core.StatefulBase
	Title    string
	Header   core.Widget // 見出しの文字を置き換える（バッジ付きなど）。読み上げは Title のまま
	Open     bool
	OnChange func(open bool)
	Child    core.Widget
}
```

- `OnChange` があれば親が状態を持つ（制御）。Disclosure は `Open` を見せるだけで、押されたら `OnChange(!Open)` で頼む。親が `Open` を変えなければ開かない。`Switch` や `Chip` と同じ形。
- `OnChange` が無ければ自分で状態を持つ（非制御）。`Open` は始めの状態。その後も、親が `Open` を変えたときはそれに従う（「すべて開く」ボタンが書ける）。親が同じ `Open` で作り直すだけなら、利用者が開いた状態を保つ。
- 1 つだけ開くアコーディオンは、親が開いている添字を 1 つ持ち、各 Disclosure に `Open: s.open == i` と `OnChange` を渡して書く。

```go
Disclosure{Title: sec.Title, Child: sec.Body, Open: s.open == i,
	OnChange: func(open bool) { s.SetState(func() { s.open = -1; if open { s.open = i } }) }}
```

なぜこの形か:

- 制御の形にすれば「1 つだけ」も「少なくとも 1 つは開く」も「2 つまで」も、親の数行で書ける。規則ごとにフラグや `AccordionGroup` を作ると、規則の数だけ API が増える。
- 1 つの部品で単独とアコーディオンの両方をこなすことが当初の計画の条件。グループ用の別ウィジェット（`Accordion{Items: ...}`）を足すと、中身の型（タイトルと子の組）を新しく決めることになり、`Header` の差し替えや項目ごとの設定を二重に持つ。
- 単独で使うときに親へ状態を書かせるのは重い。設定画面の「詳細」1 つのために State を作らせたくないので、`OnChange` 無しの既定は自分で持つ。
- `Flutter` の `ExpansionTile` は `initiallyExpanded` と `controller` を持つ。gowaku では関数オプションもコントローラも足さず、既にあるコントロールと同じ「`OnChange` の有無で決まる」に揃えた。名前を `InitiallyOpen` にしなかったのは、非制御でも親の変更に従うので「始めだけ」ではないから。

採らなかった:

- 開閉の状態を外に出すコントローラ（`DisclosureController`）。`Open` と `OnChange` で足りる。
- `Enabled` / 無効の状態。見出しが開閉以外をしないので、無効にしたいときは中身を空にするか、置かなければよい。要るとわかったら `Enabled` ではなく、ゼロ値を壊さない名前（たとえば `Locked`）で足す。

### 3. 見た目と動き

- 見出しはコントロールの高さ（30）、幅いっぱい。シェブロン 12 とタイトル（`Text.Headline`）。ホバーで `Fill` の地、押下で濃く、キーボードのフォーカスでリング。`propertyHeader` と同じ。
- シェブロンは閉じて右、開いて下。開閉の割合に合わせて 90° 回る。
- 中身はタイトルの頭にそろえて字下げする（左 26、上 4、下 8）。
- 開閉は暗黙アニメーション（`motion.Implicit`、200ms、`motion.Standard`）。中身の高さに割合を掛け、整数に丸めた高さだけを見せ、残りはクリップする。下に続くものが小数の位置で揺れないように丸める（積は ADR 0086 のとおり明示的に丸める）。`ReduceMotion` では跳ぶ。ホバーのフェードは ADR 0042 どおり残る。
- 途中で反対を頼まれたら、今の位置から向きを変える（`Implicit.Commit` の性質）。

### 4. 閉じた中身

- 中身は閉じても作ったまま（状態を保つ）。フォームを閉じて開き直しても、入力が消えない。
- 閉じている間は `core.Hider` で隠し、ポインタ、Tab、スクリーンリーダーが素通りする。
- 閉じている間はレイアウトもしない。テキストの組版はレイアウトで起きるので、閉じた FAQ が 50 個あっても組むのは見出しだけ。開くときに初めて組む。
- 中身は高さ無制限で組む。`Expanded` のように高さの上限が要る中身は置けない（doc に書いた）。

採らなかった: Flutter の `maintainState: false` のように閉じたら中身を捨てる選択。捨てると入力が消えるうえ、上のとおり閉じた中身の費用は Build だけなので、選ばせる理由が薄い。

### 5. キーボードと Semantics

- 見出しは Tab の止まり。Enter と Space で開閉。押しっぱなしの繰り返しは無視。
- 左右の矢印では開閉しない。`PropertyGrid` の見出しは木と同じく左右で開閉するが、それは表の中で上下の矢印と組になっているから。単独の Disclosure で矢印を取ると、外のスクロールや横の部品へ渡らなくなる。WAI-ARIA のアコーディオンの型も Enter / Space だけを求める。
- Semantics はボタンで、名前は `Title`、状態は `Expanded` か `Collapsed`、動作はタップ・展開・折り畳み。

### 6. Link

```go
type Link struct {
	core.StatefulBase
	Label   string
	Style   text.Style // ゼロ値は本文。色がゼロならアクセントのインク
	OnPress func()     // nil で無効
}
```

- 色は `AccentInk`。ポインタが載ると下線（80ms のフェード）と手のカーソル。押下中は字が少し `Ink` に寄り、下線が残る。
- キーボード: Tab の止まりでリング、Enter で押す。Space では押さない。ウェブのリンクと同じで、Space はページのスクロールに渡す。`Breadcrumb` のリンクと同じ規則。
- `OnPress` が無ければ無効: `Ink3` で、下線も手も出ず、Tab も止まらない。Semantics は `Disabled` のリンク。ボタンと同じく「`On…` が無ければ無効」でゼロ値を保つ（`Link{}` は無効のリンク）。
- 下線は自前の render object が引く。`RenderParagraph` のレイアウトから行ごとの範囲（`SelectionRects`）とベースライン（行のランの Origin）を取り、ベースラインの少し下に、字の大きさに合わせた太さで引く。折り返した行にも、右から左の文字にも引ける。バッファは render object に持ち、描画でアロケーションしない（テストで確認）。
- `text.Style` に下線の装飾を足すことはしなかった。`text` のスタイルと組版、`render` に手が入る大きな変更になる。下線が要るのは今は Link だけなので、要る部品が増えたら考える。

採らなかった:

- `RichText` のスパンの中に置けるリンク。行の中の範囲のヒットテストとフォーカスが要り、`SelectionArea` と同じく `RenderParagraph` に手が入る。Link は `Row` や `Wrap` の中で文の隣に置く部品にした。
- 訪問済みの色。デスクトップのアプリでは「訪問」の意味が決まらない。

### 7. Breadcrumb は Link に置き換えない

`nav.Breadcrumb` の各リンク（`crumbLink`）は、下線ではなく丸い地をホバーで出し、`Ink2` の字で、… ボタンと同じ部品を兼ねている。Link に置き換えると見た目が変わるので、当初の計画の「振る舞いを変えない」に触れる。置き換えはしない。

## 結果

- `control.Disclosure`、`control.Link` を足した。既存の公開 API と振る舞いは変えていない。共有のファイル（文字列の表、テーマ）にも触れていない。
- 開閉の各フレームの Layout と Paint、下線の Paint はアロケーションしないことを `testing.AllocsPerRun` で確かめた。
- 右から左の UI でシェブロンを左に向けることはしていない。gowaku にはまだ UI 全体の書字方向（Directionality）が無いので、入ったときに合わせる。
- カタログのページは `examples/gallery/pages_disclosure.go`。
