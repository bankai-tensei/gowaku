# 0128: 無効な部品のコントラスト、Tester のポインター操作とロケール、待つ規則、等幅の文字スタイル

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 5 つが見つかった。

1. **無効な部品のコントラスト：** `gowakutest.Tester.Warnings` が、無効にした `control.Button` の文字を 2.43:1 の低コントラストとして報告した。WCAG 1.4.3 は、操作できない部品（inactive user interface component）の文字をコントラストの要件から除外している。無効な部品は薄く描くのが慣例であり、警告は誤報だった。
2. **ポインター操作の不足：** Tester にはタップ（`Tap`、`SecondaryTap`、`DoubleTap` とその `At` 形）しかなく、ホバーで止まるカルーセル、ツールチップ、ホイール、Ctrl+ホイールのズーム、スライダーや分割線のドラッグ、ファイルのドロップを試すには、`tt.Tree().DispatchPointer` や `DispatchDrag` でイベントを自分で組み立てる必要があった。同じラベルを持つノード（タブとページの見出しなど）を区別する方法も無かった。
3. **ロケール：** `gowakutest.Options` にロケールが無く、テストの中のカレンダーや日付は英語にしかならなかった。
4. **待つ規則：** `Tap` が `StateBase.After(16ms)` を実行し終えるまで待つと期待した人がいた。`Settle` が休止中の Ticker の時刻まで時計を進めるのかどうかが、どこにも正確に書かれていなかった。
5. **等幅の文字：** テーマに等幅の文字スタイルが無く、利用者は `text.Style{Family: "Consolas"}` と推測して書いていた。これは Windows でしか当たらない。`editor.Editor` の godoc の例も同じ書き方で、markdown は `runtime.GOOS` で書体を 1 つ選んでいた。

## 選択肢

### 無効な部品の見分け方

- **A. 意味ツリーで無効なノードの矩形を集め、その中にある文字を除く。** 実装は短いが、重なった部品や変形の下では位置で判定を誤る。
- **B. 意味ツリーのノードを開いた RenderObject を引き、その下の RenderObject を「無効の中」として歩く。** `core.Tree` は `SemanticsTree` のときにノードと RenderObject の対応（owners）を既に持っている。これを `SemanticsOwner(id)` として公開すれば、Warnings の描画順の走査で祖先の状態を引き継げる。位置に頼らない。
- **C. 部品ごとに RenderParagraph へ「無効」の印を付ける。** 部品の数だけ変更が要り、利用者の部品では漏れる。

### ポインター操作の形

- 名前は既存のタップに揃え、ラベルを取る形と `At` 形の組にする: `Hover`/`HoverAt`、`Scroll`/`ScrollAt`、`Drag`/`DragAt`、`DropFiles`/`DropFilesAt`、`DropText`/`DropTextAt`。ポインターをウィンドウの外へ出す `Unhover` を加える。
- ホイールの向き: `core.PointerEvent.Scroll.Y` はホイールを奥へ回すと正（OS の値のまま）だが、テストでは「下へ 3 ノッチ」を正の数で書ける方が読みやすい。Tester の `dy` は下を正とし、イベントには符号を反転して渡す。CSS の `scrollBy` や DOM の `deltaY` と同じ向きである。
- ドラッグの `Drag(label, by)` は、ノードの中央から `by` だけ動かす。スライダーや分割線では「どれだけ動かすか」で書く方が自然だからである。途中の移動は 10 歩、1 歩ごとに 1 フレーム（`FrameTime`）時計を進める。実際のウィンドウでもドラッグの間にフレームが進み、ドラッグ中に端で自動スクロールするような Ticker もそれで動く。
- ドロップは OS が他のプログラムからのドラッグを届ける順序（種類だけを持った `DragOver`、次にデータを持った `DragDrop`）をなぞり、許す効果はファイルマネージャーと同じくコピー・移動・リンクのすべてとする。結果の `input.DropEffect` を返し、拒否されたことをテストできるようにする。
- 役割で探す `FindRole(role, label)` と `FindAllRole(role, label)` を加える。ラベルの一致は `Find` と同じく完全一致とする。

### ロケール

- `Options.Locale`（`language.Tag`）を加え、`basic.MediaQueryData.Locale` に渡す。`Run` と同じく `basic.Localizations` も根に置き、`App.Strings` に当たる `Options.Strings` も加える。ゼロは英語で、OS のロケールには従わない。テストの結果が機械によって変わらないようにするためである。

### 待つ規則

- **A. `Settle` が休止中の Ticker の時刻まで時計を飛ばす。** `After(16ms)` は待てるが、周期的な Ticker があると永遠に終わらず、5 分後の自動保存のような遅延まで勝手に進んでしまう。
- **B. `Settle` は何かが動いている間だけフレームを描き、休止中の Ticker の時刻へは進めない。遅延は `Advance` で進める。** 現在の実装のとおりであり、テストが時間を明示的に支配できる。ただし、タップ自身のアニメーションの途中で時刻が来た遅延は、その間に実行される。

### 等幅の書体の指定

- **A. テーマで `runtime.GOOS` を見て書体を 1 つ選ぶ。** テーマの値が OS によって変わり、ゴールデンや比較の結果が OS に依存する。Linux のように何が入っているか分からない環境では 1 つでは足りない。
- **B. `text.Style.Family` に CSS の font-family と同じくカンマ区切りの一覧を許し、テーマは全 OS の等幅書体を並べた 1 つの一覧を持つ。** 値は OS によらず同じで、各システムは自分にある最初の書体を使う。fontscan の汎用名 `monospace` を最後に置けば、どれも無い環境でも等幅が選ばれる。

## 決定

1. `core.Tree.SemanticsOwner(id)` を加え、`Warnings` は `SemanticsTree` の無効なノード（`semantics.Disabled`）を開いた RenderObject の下にある文字のコントラストを調べない（選択肢 B）。はみ出し、ラベルの無い部品、シェーダーの検査はそのまま行う。
2. Tester に `Hover`、`HoverAt`、`Unhover`、`Scroll`、`ScrollAt`、`Drag`、`DragAt`、`DropFiles`、`DropFilesAt`、`DropText`、`DropTextAt`、`FindRole`、`FindAllRole` を加える。操作系のメソッドはタップと同じく操作の後で `Settle` する。
3. `Options.Locale` と `Options.Strings` を加え、根に `basic.Localizations` を置く。
4. 待つ規則は選択肢 B とし、パッケージの doc、`Settle` と `Tap` の godoc、テストのガイドに書く: `Settle` と操作系のメソッドは動いている間だけフレームを描き、休止中の Ticker（周期的な Ticker、`After`、ツールチップの遅延）の時刻へは進めない。`Advance(d)` はその間に来た時刻をすべて実行する。`Options.Settle` を設定したときは、その時間だけ `Advance` する。
5. `text.Style.Family` をカンマ区切りの一覧として扱う（選択肢 B）。`theme.TextTheme` に `Code`（13px、400、行送り 20、`Family` は `theme.MonoFamilies`）を加える。`theme.MonoFamilies` は "Cascadia Mono, Consolas, SF Mono, Menlo, DejaVu Sans Mono, Noto Sans Mono, Liberation Mono, Go Mono, monospace" である。Go Mono はカタログの図と多くのテストが使う Go フォントの等幅で、これを入れておくとカタログの図でも等幅になる。`editor.Editor` は Style の Family とサイズが空なら `Code` を使い、markdown のコードは Family が空なら `Code` の Family を使う（`monospaceFamily` は削除）。

## 結果

- 無効な部品を薄く描いても `Warnings` は報告しない。有効な部品の文字と、部品の外の文字は今までどおり調べる。`SemanticsOwner` は直前の `SemanticsTree` の対応を返すので、`Warnings` は自分で `SemanticsTree` を作り直してから引く。
- ホバー、ホイール、ドラッグ、ドロップ、同じラベルの区別が 1 行で書ける。`Drag` は時計を約 160ms 進めるので、その間に時刻の来た遅延は実行される。
- テストで日本語などのロケールを指定できる。ゼロのロケールは英語のままなので、既存のテストは変わらない。
- `Tap` の後に `After` の遅延を待つには `Advance` が要ることが明文化された。
- `editor.Editor` の既定が等幅になり、カタログの `editor.Editor` の図が変わった（Go Mono）。`Family` に一覧を書けるようになったので、利用者は OS ごとに書体を切り替える必要がない。カンマを含む名前の書体は指定できなくなるが、そのような書体名は実際には無い。
