# 0137: Tester のスクロールの符号、ラベルで探す順、テスト用のクリップボード、親からのはみ出し

決定日: 2026-11-17

状態: 採用

## 背景

初めて使う人に `gowakutest` を試してもらったところ、次のことが見つかった。

1. **スクロールの符号：** `core.PointerEvent.Scroll` は、Y が正のときホイールを奥へ回した（内容の上の方へ進む）ことを表す。一方 `Tester.Scroll` と `ScrollAt` は、正を下向きとして受け取り、中で符号を反転していた。イベントを自分で組み立てたテストと `Scroll` を使ったテストとで符号が逆になり、どちらが正しいか迷う。`PointerEvent.Scroll` の godoc も「ノッチ単位」としか書いておらず、X と Y の向きが分からなかった。
2. **ラベルで探す順：** `Find` は文字（`RoleText`）でない役割をすべて「部品」として文字より優先していた。見出しやグループなど、操作できない役割も部品と同じ扱いなので、入力欄と同じラベルの見出しが先にあると `Tap("Notes")` が見出しを押し、入力欄にフォーカスが移らなかった。
3. **クリップボード：** `gowaku.WriteClipboard` と `ReadClipboard` は `WindowOf` で窓を探し、`Run` の外では panic する。`gowakutest` の木には何もしない `Host` しか無く、入力欄でコピーした文字をテストで確かめられなかった。
4. **はみ出しの見落とし：** 268 px の枠に入れた `SegmentedControl`（None、Low、Medium、High）が右端から 16 px はみ出して描かれたが、`Warnings` は何も報告しなかった。はみ出しを調べていたのは `RenderFlex`（`Row` と `Column`）と `RenderParagraph` だけだった。`RenderSegmented` は自分の大きさを制約に収めるが、区切りの幅は子の最大幅から決めるので、子が自分の外に置かれる。
5. **説明の不足：** `Key(KeySpace)` が入力欄に空白を入れないこと、`Dump` と `Find(...).Rect` がウィンドウの座標で、リストが表示域のすぐ外に作った行も含むことが書かれていなかった。

## 選択肢

### スクロールの符号

- **A. Tester を `PointerEvent` に合わせる（反転しない）。** 符号の規則が一つになる。既存の呼び出しは符号を変える必要がある。
- **B. `PointerEvent` を「正が下」に変える。** 全プラットフォームの変換と、スクロールを受けるすべての部品を直すことになり、変更が大きい。
- **C. 今のまま、説明だけ足す。** 二つの規則が残る。

### クリップボードの経路

- **A. `WriteClipboard` などは `WindowOf` のままにし、godoc に「窓が要る」と書く。** テストでは入力欄のコピーしか確かめられない。
- **B. `BuildContext.Host()` を通す。** 部品が使う `RenderBase.Host()` と同じ経路になる。`Host` が `platform.Clipboard` も実装していればすべての種類を渡し、実装していなければテキストだけを `SetClipboardText` / `ClipboardText` で渡す。窓の `windowHost` は `platform.Clipboard` を OS の窓へ取り次ぐ。`core` は `platform` を知らないままで済む（型の確認は根のパッケージで行う）。

### はみ出しの調べ方

- **A. はみ出しうる RenderObject ごとに `Overflow()` を足す。** 部品を足すたびに忘れうる。
- **B. 走査で、子の矩形が親の矩形の外に出ていないかを一般に調べる。** どの RenderObject にも効く。子をクリップする親（`core.Clipper` で切り取っているとき）は、外の子が隠れるだけなので調べない。`Stack` の中で `Positioned` が置いた子（角に掛かるバッジなど）は、わざと外に置くので調べない。`RenderFlex` は自分の警告があるので重ねない。

## 決定

1. スクロールは選択肢 A。`ScrollAt` は `dx`、`dy` をそのまま `PointerEvent.Scroll` に入れる。`dy` が正ならホイールを奥へ回し、リストの上の方へ進む。負なら下の方へ進む。`dx` が正なら右へ進む。`input.ModControl` と正の `dy` はズームイン。リポジトリの呼び出し（`gowakutest` のテストとテストのガイドの例）の符号を直した。`PointerEvent.Scroll` の godoc に X と Y それぞれの向きと、どのプラットフォームでも Windows のホイールと同じ符号であることを書いた。各バックエンド（Win32 の `WM_MOUSEWHEEL` / `WM_MOUSEHWHEEL`、X11 のボタン 4〜7、Wayland、Cocoa）と、`Scroll`、`Table`、`Editor`、`InteractiveViewer` の受け方で確かめた。
2. `Find` は、同じラベルのノードを次の順に選ぶ。操作できる部品（文字でない役割で、`ActionTap`、`ActionFocus`、`ActionSetValue`、増減、開閉のいずれかを持つか、`Focusable`、または `RoleTextField`）、その他の役割のもの（見出し、グループなど）、操作できる文字（フォーカスを取る文字）、ただの文字と役割の無いもの。同じ順位の中では読む順に最初のもの。操作できる文字を部品より下にするのは、Markdown のリンクの文字が、リンクと同じラベルでフォーカスだけを持って先に来るため（`markdown` のテストで分かった）。`Tap`、`Hover`、`Scroll`、`Drag`、ドロップはすべて `Find` で探すので、同じ規則に従う。
3. クリップボードは選択肢 B。`gowakutest` の `Tester` は自分の `Host` を持ち、それが `core.Host` と `platform.Clipboard` を実装する。`Tester.Clipboard()` / `SetClipboard(s)` でテキストを、`ClipboardContent()` / `SetClipboardContent(c)` で全種類を読み書きする。`SetClipboardText` は、窓でのコピーと同じく、それまでの全種類を置き換える。`gowaku.ClipboardHas`、`ReadClipboard`、`WriteClipboard` は `ctx.Host()` を通すので、テストの中でも動く。
4. はみ出しは選択肢 B。`Warnings` は、子をクリップしていない親ごとに、子が親の端を越えた量の最大を `WarnOverflow` として 1 件報告し、どの端かと親の型を `Detail` に書く。大きさの無い子と隠れた子は調べない。
5. `Key` の godoc とガイドに、押下だけを送って文字は入力しないこと（空白は `Type(" ")`）を書いた。`Dump` と `Find` の godoc、ガイドの表に、矩形がウィンドウの座標であること、リストが表示域のすぐ外に作った行も含むことを書いた。

## 結果

- スクロールの符号は、イベントと Tester で同じになった。`Scroll(label, 0, 3, 0)` と書いていたテストは `Scroll(label, 0, -3, 0)` に直す必要がある（ライブラリは未公開なので、互換の仕組みは置かない）。
- 入力欄と同じラベルの見出しやグループがあっても、`Tap(label)` は入力欄に届く。同じラベルのボタンが二つあれば、今までどおり先のものが選ばれる。
- 入力欄のコピーと貼り付け、`gowaku.WriteClipboard` で書いた HTML やファイルをテストで確かめられる。クリップボードは Tester ごとに別なので、テスト同士や利用者の OS のクリップボードに影響しない。
- 幅の足りない `SegmentedControl` のように、子を自分の外に置く部品が警告に出る。区切りを枠に収めるかどうかは部品側の別の判断として残す。カタログの全シーンで、新しい規則による警告は `Badge`（`Positioned` を除く前）だけで、除いた後は 0 件だった。
- `Warnings` は今までどおり変形を無視して配置の位置で判定するので、動いている途中のページなどは、`Clipper` を実装していなければ報告されうる。
