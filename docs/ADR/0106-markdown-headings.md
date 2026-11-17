# 0106: Markdown の見出しの一覧と、見出しへ移ること

決定日: 2026-11-13

状態: 採用

## 背景

Markdown を表示するアプリからの要望（その 4）。

- 文書の中の `[x](#見出し)` を押したら、その見出しへ移りたい。README の目次のリンクがそのまま動いてほしい。
- 文書の横に目次（アウトライン）を出し、押した見出しへ移りたい。そのために見出しの一覧（段・文字・アンカー）が要る。

ADR 0099 では、`#…` のリンクは `OnLinkTap` にそのまま渡すだけで、見出しにアンカーは無かった。

調べたこと:

- `Markdown` は自分ではスクロールしない。長い文書はアプリが `basic.Scroll` に入れる（`Markdown` の doc コメント）。
- `basic.Scroll` には外から動かす口が無い。`collection.List` には `ListController.JumpToIndex` があるが、`RenderList` の行の番号で動くもので、任意の RenderObject を見せる口ではない。`core` にも「この RenderObject を見える所へ」の仕組み（Flutter の `showOnScreen` / `Scrollable.ensureVisible`）は無かった。semantics の `ActionScrollIntoView` は `collection` の行だけが実装している。
- `core` には「レイアウトの後に呼ぶ」口（post-frame callback）も無い。

## 決定

### 1. アンカー（slug）の規則は GitHub と同じ

github-slugger と同じ手順にした。README を GitHub と gowaku の両方で読むので、同じリンクが同じ見出しへ行くことが一番大事。

1. 見出しの文字（強調・コード・リンクの記号を除いた、表示される文字）を小文字にする（`strings.ToLower`。全角の英字も小文字になる）。
2. ASCII の空白 1 つを `-` 1 つにする（続く空白はまとめない。`a  b` は `a--b`）。
3. 文字（`unicode.IsLetter`）、数字（`unicode.IsNumber`）、結合文字（`unicode.IsMark`）、`-`、`_` のほかは捨てる。約物・記号・絵文字・全角の空白（U+3000）は消え、漢字・かな・ハングルなどはそのまま残る。
4. 同じ slug が前に出ていたら、`-1`、`-2`… を付ける。数えるのは元の slug ごと。付けた結果がさらに既存の slug と重なるときは次の番号へ進む（`## faq-1` のあとの重なりなど。github-slugger と同じ）。
5. 引用やリストの中の見出しも、文書順に数える。

| 見出し | slug |
|---|---|
| `# Getting Started` | `getting-started` |
| `## API: v2.0 (beta)` | `api-v20-beta` |
| `## 見出し` | `見出し` |
| `## はじめに「概要」` | `はじめに概要` |
| `## Go 言語の UI` | `go-言語の-ui` |
| `## 見出し`（2 つ目） | `見出し-1` |
| `## FAQ!` / `## FAQ?` | `faq` / `faq-1` |
| `## Emoji 🎉 party` | `emoji--party` |
| `## ＡＢＣ　全角` | `ａｂｃ全角` |
| `## Café Ünïcode` | `café-ünïcode` |

リンクの宛先との突き合わせ:

- `#` で始まる宛先の後ろを、`%E8%A6%8B` のようなエスケープを戻してから slug と比べる（`[x](#%E8%A6%8B%E5%87%BA%E3%81%97)` も `[x](#見出し)` も同じ見出しへ）。
- そのままで無ければ小文字にして比べる（GitHub の画面も大文字の `#Install` を受ける）。
- 空の `#` はどの見出しにも当たらない（見出しの文字がすべて記号で slug が空のときも、`#` だけのリンクで飛ばない）。

### 2. 公開 API

```go
type Heading struct {
	Level int    // 1〜6
	Text  string // 表示される文字
	Slug  string
}

func Headings(source string) []Heading // 解析して見出しを文書順に返す

type Controller struct{ /* 非公開 */ }
func (c *Controller) Headings() []Heading
func (c *Controller) ShowHeading(anchor string) bool

type Markdown struct {
	...
	OnAnchorTap func(h Heading) // 新規
	Controller  *Controller     // 新規
}
```

- `Headings(source)` は純粋な関数。アウトラインを同じ Build の中で作れる（`Markdown` が先に解析している必要が無い）。解析を 1 回余分にするが、ホットパスではなく、`Source` が変わったときにアプリが持っておけばよい。
- `Controller` は `ListController` と同じ形: ゼロ値で使え、State に置いて `Markdown.Controller` に渡す。`Headings()` は今出している文書の一覧を、解析し直さずに返す（複製）。
- `ShowHeading(anchor)` は `#` の有無もエスケープも、リンクと同じ規則で受ける。文書にその見出しが無ければ false。`Markdown` がまだそのコントローラーを出していないとき（親の `InitState` など）は覚えておき、出たときに移って true を返す。関数オプションやメソッドチェーンは無い（規約）。

### 3. 文書の中の `#` のリンク

- `#` と見出しの slug を宛先に持つリンクは、**既定で見出しへスクロールする**。`OnLinkTap` が nil でも押せ、Tab で止まり、スクリーンリーダーにはリンクとして出る（今までは `OnLinkTap` が nil だとすべてのリンクが押せなかった。`#` 以外のリンクは今までどおり）。
- アプリが上書きしたいときは `OnAnchorTap` を設定する。設定すると、見出しへのリンクは `OnAnchorTap(Heading)` を呼ぶだけで、スクロールはしない。アプリは中で `Controller.ShowHeading` を呼んでもよい（アウトラインの選択を合わせる、履歴に積む、など）。
- 見出しへのリンクは `OnLinkTap` には来ない。見出しに当たらない `#…` は今までどおり `OnLinkTap` へ渡す（アプリが別の意味に使っているかもしれない）。
- 画像のリンク（`[![絵](a.png)](#見出し)`）も同じ。

`OnLinkTap` の型を `func(url string) bool`（true で処理済み）に変える案もあったが、すべてのアプリの呼び出しが変わるのでやめた。

### 4. スクロールの仕組み: `basic.ScrollIntoView`

```go
func ScrollIntoView(ro core.RenderObject, rect geom.Rect, align float32)
```

- `ro` の座標の `rect` が見えるように、`ro` を囲む `basic.Scroll`（`layout.RenderViewport`）を近い方から順にすぐ動かす。`align` は `ListController.JumpToIndex` と同じ（0 で始まりを view の始まりに、1 で終わりを終わりに、0.5 で中央）。各 Scroll は内容の中で止まるので、末尾近くの見出しは先頭まで来ない。入れ子の Scroll（ページ全体の Scroll の中の、高さのある Scroll の中の文書）もすべて動く。
- `layout.RenderViewport` に `OnJump func(offset float32)` を足し、`basic.Viewport` は `ScrollState` の非公開の `jumpTo` を渡す。物理（`Scroller`）を通して動かすので、ホイールの動きの途中でも止まり、スクロールバーも出る。`OnUser`（利用者が動かしたとき）は呼ばない。
- 動きはアニメーションさせず一度に移す。ブラウザのアンカーの既定と同じで、`ReduceMotion` を考えなくてよい。
- `core` ではなく `basic` に置いた。動かすのは `basic.Scroll` の State で、`core` は `ScrollState` を知らない。`List` / `Tree` / `Table` / エディタは自分のコントローラーで動くので対象外（doc コメントに書いた）。

`Markdown` は見出しの `renderRich` に slug を持たせ、`ShowHeading` のときに自分の RenderObject の下から探して `ScrollIntoView(見出し, 見出しの上 8px を含む矩形, 0)` を呼ぶ。見出しは view の上端から 8px 下に来る。

### 5. レイアウトの前と、あとから高さが変わるとき

- **レイアウトの前**: 親の `InitState` で `ShowHeading` を呼ぶと、まだ見出しの位置が無い。`core` に post-frame callback が無いので、State の `Ticker` を 1 つ使う。次のフレームの tick（前のフレームのレイアウトが済んでいる）で移し、止まる。最大 10 フレーム待って、見つからなければやめる。
- **上の画像があとから来たとき**: `Images` から読む絵は Build の中で同期して読むので、最初のレイアウトから高さが入っている。`ImageProvider` が返すウィジェットはあとで大きさが変わりうる（読み込みが終わって絵になる、など）。そのままだと、移したあとに上が伸びて見出しが下へずれる。
  - そこで、最後に移した見出しを覚え、文書の大きさが変わったら（文書の外側の RenderObject `renderDoc` がレイアウトで大きさの変化を知らせる）、次のフレームでもう一度その見出しを上へ戻す。ブラウザのスクロールアンカリングに近い。
  - 利用者がスクロールしたら追うのをやめる。移した直後の Scroll の位置を覚えておき、大きさが変わったときに位置が違っていれば（ホイール、スクロールバー、別のジャンプ）利用者の操作とみなす。戻すのを待っている次のフレームでも同じく確かめ、間に動いていれば戻さない。
  - 待っている見出しは 1 つだけで、新しい要求が古いものを置き換える。すぐ移れた要求は待ちを捨てて Ticker を止める（古い待ちが次の tick で新しい見出しから引き戻さないように）。
  - 戻すのは次のフレームなので、1 フレームだけ見出しがずれて描かれる。レイアウトの最中に祖先の Scroll を動かすと、Scroll の大きさと限界がまだ決まっていないため。
- 毎フレームの処理は増えない。大きさが変わらなければ `renderDoc` は何もしない。Ticker は移すときだけ動き、すぐ止まる。

### 6. ホットパス

- `renderRich` に足したのは文字列のフィールド（`anchor`, `brk`）と bool。Layout と Paint の処理は増えていない（`Paint` の nil 判定は ADR 0105 の選択のもの）。
- slug は解析のとき（`Source` が変わったとき）に 1 回だけ作り、map に持つ。リンクが押せるかの判定は Build のときに map を引く。
- 確認: `TestController_RevealDoesNotAllocateAtLayout`（移したあとの静かなフレームのアロケーションが移す前と同じ 0 で、Ticker が止まっている）。ベンチは並行のエージェントがいるため走らせていない。

## 結果

- `markdown/heading.go`（`Heading`, `Headings`, slug の規則、宛先の突き合わせ）、`markdown/markdown.go`（`Controller`, `OnAnchorTap`, 待ちと追従）、`widgets/basic/scroll_reveal.go`（`ScrollIntoView`）、`layout.RenderViewport.OnJump`。
- `markdown/example` に左のアウトライン（`Headings` と `Controller.ShowHeading`）と、本文の先頭の目次（`#` のリンク）を足した。本文は `SelectionArea` の中。
- テスト: `TestSlugify`（上の表）、`TestHeadings`（引用・リストの中、重なり、setext）、`TestAnchorLink_ScrollsWithoutOnLinkTap`（日本語・エスケープ・末尾で止まる・`#` 以外は押せない）、`TestAnchorLink_AppOverrides`、`TestController`（`InitState` から、`#` 無し、無い見出し）、`TestAnchor_LandsBelowPicturesAndFollowsALatePicture`（500px の絵の下へ移る、上の絵があとで伸びても戻る、ホイールのあとは戻さない）、`widgets/basic` の `TestScrollIntoView` / `TestScrollIntoView_Nested`。
- スクリーンショットで、アウトラインの「表」で移った結果を確かめた（見出しが上端の 8px 下）。

## 残したこと

- スムーズスクロール（移るときのアニメーション）。
- `Markdown` を `List` の行の中に入れたときの `ShowHeading`（`ScrollIntoView` は `basic.Scroll` だけを動かす）。
- 見出しの横のアンカーの印（GitHub のホバーで出る鎖の絵）と、slug をコピーする UI。
- 今どの見出しを読んでいるか（アウトラインの現在地）を知らせる口。必要ならスクロールの位置から見出しを引く関数を足す。
- 本体の semantics の `ActionScrollIntoView` を `basic.Scroll` 全般で受けること。`ScrollIntoView` が下地になる。
