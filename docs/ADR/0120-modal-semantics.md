# 0120: モーダルの下を Semantics ツリーから外す

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人にライブラリを試してもらったところ、`overlay.ShowDialog` でダイアログを開いても、下のページが Semantics ツリーに残っていた。

- `gowakutest` の `Find` / `Tap` は読む順で最初に合うものを選ぶ。ページとダイアログの両方に「Reset」があると、`tt.Tap("Reset")` がダイアログの Reset ではなく、幕（スクリム）の下のページの Reset の位置を押す。
- UIA / AT-SPI / NSAccessibility は同じツリーを読む。スクリーンリーダーはダイアログの外の、Tab でもポインタでも届かない部品を読み、操作もできた（`PerformSemanticsAction`）。Web の `aria-modal="true"` は外側を支援技術から隠す。

Tab はすでに閉じ込めていた。`core.FocusScope` の `Modal()` が真の最後のもの（`layout.RenderModal`、`kit.ModalScope`）だけを Tab とショートカットの範囲にする（ADR 0010、0046）。ツリーだけがこれを見ていなかった。

`kit.ModalScope{Active: true}` を使う場所は 8 つある。

- 幕のあるモーダル: `ShowDialog`、`ShowSheet`。
- コントロールから開くポップアップ: メニュー（メニューバー、コンテキストメニュー）、`DropdownMenu` の一覧、`DatePicker`、`PropertyGrid` の色、`DataTable` の絞り込み。これらは開いている間も元のコンボボックスやメニューの見出しがツリーにあり、`Expanded` を言う（既存のテストがそれを確かめている）。ARIA でもコンボボックスやメニューのポップアップはモーダルにしない。

OS のモーダルウィンドウ（`gowaku.ShowModal`、ADR 0047）は別のウィンドウで、持ち主のウィンドウは OS が無効にする。ここでは扱わない。

## 選択肢

1. **`FocusScope.Modal()` をそのまま使い、モーダルスコープの外をすべて外す**。ポップアップの元のコンボボックスまで消え、`Expanded` を言う相手がいなくなる。ポップアップを開いたままコンボボックスを探すテストが 10 個ほど落ちた。
2. **ダイアログのウィジェットに `ExcludeSemantics` を利用者が巻く**。ページ側を巻く必要があり、ダイアログを開く側からは届かない。忘れると今と同じになる。
3. **core に `SemanticsBlocker` interface を置き、ツリーを作る `SemanticsBuilder` が、最後に塗られて隠れていないブロッカーより前に塗られたものを外す**（Flutter の `BlockSemantics` と同じ規則）。`RenderModal` にフラグ `BlockSemantics` を足し、ダイアログとシートだけが立てる。

3 を採った。

## 決定

### 1. core.SemanticsBlocker

```go
type SemanticsBlocker interface {
	RenderObject
	BlocksSemantics() bool
}
```

`Tree.Semantics` と `Tree.SemanticsTree` は、まず根から、`BlocksSemantics()` が真で隠れていない（`Hider`）最後のブロッカーを探す（`modalScope` と同じ探し方）。見つかれば、根からそこまでの道を `Tree` が持つバッファに入れる（2 回目からはアロケーションしない）。

`SemanticsBuilder.Visit` は、今の親がその道の上にあるとき、子が道の次のものより前に塗られる兄弟なら飛ばす。道の上の子と、後に塗られる兄弟はそのままたどる。

- ダイアログの下のページ（Overlay の Stack でダイアログより前の子）とタイトルバー（Overlay の中の Column の前の子）は外れる。
- ダイアログより後に塗られるもの、たとえばダイアログから開いたトースト、ドロップダウンの一覧、ツールチップは残る。
- ダイアログの上のダイアログは、後のものが勝つ。

どちらの関数も同じ `build` を通る。`SemanticsTree` の `owners` には外れたノードが入らないので、`PerformSemanticsAction` で外れたボタンを押すことはできない。

### 2. layout.RenderModal.BlockSemantics

`RenderModal` に `BlockSemantics bool` を足し、`BlocksSemantics()` は `Active && BlockSemantics` を返す。`kit.ModalScope` も同じ名前のフィールドで渡す。`ShowDialog` と `ShowSheet` だけが立てる。閉じ始めると `Active` が偽になるので、ページは閉じるアニメーションの最初の一枚から戻る。Tab が戻るのと同じ時点。

コントロールから開くポップアップは立てない。元のコントロールがツリーに残り、開いていることを言う。

### 3. gowakutest

`Find` / `FindAll` / `Tap` は `Tree.Semantics` を読むので、手を入れずにダイアログの中だけを探すようになる。doc コメントにそう書いた。

## 結果

- `tt.Tap("Reset")` はダイアログの Reset を押す。ダイアログが閉じると、同じ呼び出しでページの Reset が見つかる。
- スクリーンリーダーはダイアログとシートの外を読まない。プラットフォームのブリッジは同じツリーを読むので変更は要らない。外れたノードは、ナビゲーターのページが替わるときと同じく、差分で消える。
- ホットパス（Layout、Paint、ヒットテスト）には触れない。ツリーを作るときに道を 1 回探す分だけ増える。Visit で道の上の兄弟の順を調べるのは、Overlay の Stack のような道の上の親だけ。
- 未解決: トーストの層は最初のトーストのときに Overlay の項目として 1 つ足され、ずっと残る。そのあとで開いたダイアログはトーストの層より後に塗られるので、ダイアログを開いている間に出したトーストは（幕の下に描かれるのと同じく）ツリーから外れ、読み上げられない。ダイアログより前にトーストを 1 度も出していなければ、トーストの層はダイアログより後になり、残る。トーストの層を常に最前にするかは別に決める。
