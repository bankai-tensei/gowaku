# 0140: 見出しを描かない入力欄の名前、ホバー中のポインターの位置、日付の書き方、フォーカスの外し方

決定日: 2026-11-17

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 4 つが見つかった。

1. **見出しを描かずに名前だけ付けられない：** `field.Input`、`field.Search`、`field.DropdownMenu`、`field.NumberField`、`field.DatePicker`、`field.TimePicker`、`field.Autocomplete`、`field.TokenField` は、`Label` を欄の上に小さな見出しとして描き、同じ文字列をスクリーンリーダーに名前として渡す。設定の行や表のセルの編集欄のように、行の側が欄の意味をすでに示している所では、見出しは要らない。しかし `Label` を空にすると名前も無くなり、`gowakutest` の Warnings が名前の無い欄（unlabeled）として報告する。`collection.Table` のセルの編集欄も、名前の無い `field.Input` だった。`field.TokenField` は、`Label` だけを設定すると、トークンのリストには名前が付くが、その後ろの入力欄には名前が付かなかった。
2. **ホバー中のポインターの位置が分からない：** `interact.MouseRegion` には `OnEnter` と `OnExit` しかない。`interact.Listener` は自分が受け取った押下の間しか移動を受け取らない（`core/input.go` の `dispatchPointer` は、押下を受け取った物 `captured` にだけ移動を配る）。ポインターについて動くグラフのツールチップを作った人は、見えない細い `MouseRegion` を 24 本並べていた。
3. **日付の書き方が足りない：** `basic.Strings.Weekdays` は英語で 2 文字（「Mo」）のカレンダーの列見出しで、3 文字の曜日（「Mon」）も、年を省いた短い日付を書く手段も無かった。カレンダーのアプリと天気のアプリが、どちらも書式を自分で書いていた。週の始まりの曜日は `field.Calendar` の中に `weekStartOf` として隠れていて、週の表示を自分で作る人には使えなかった。
4. **説明が足りない：** テキスト欄をタップしても文字列は選択されないので、テストで文字列を置き換えるには Ctrl+A を送ってから打つ必要があるが、どこにも書いていなかった。テキスト欄にフォーカスがある間は、修飾なしの `interact.Actions` のショートカットが動かない（ADR 0133）。そこからフォーカスを外す方法が分からず、コードから外す公開の手段も無かった。`go mod tidy` は、gowaku を import するファイルがまだ無いと gowaku の require を消してしまうので、始めたばかりの人がつまずいた。

## 選択肢

### 見出しを描かない名前

- **A. `HideLabel bool` を各部品に足す。** `Label` は名前のまま変わらず、`HideLabel` は見出しを描かないことだけを意味する。ゼロ値では今まで通り見出しを描く。名前の元は `Label` の 1 つだけで、`Bare` や `Obscure` と同じく、見た目を変える動詞の形のフィールドになる。
- **B. 名前だけの `Name string` を足す。** `Label` が空のときに `Name` を名前に使う。名前の元が 2 つになり、両方を設定したときの規則が要る。`Placeholder` も名前の代わりになる部品があるので、3 つになる。
- **C. `ShowLabel bool` にする。** 肯定形の名前の規則（CODING_STYLE §4.1）には合うが、ゼロ値で見出しが消え、今までのコードの見た目が変わる。

### ホバー中の位置

- **A. `MouseRegion.OnHover func(pos geom.Offset)` を足し、`core` に `HoverMoveTarget`（`HandleHoverMove(pos)`）を足す。** `Tree.updateHover` は、当たり判定の経路（`hits.Path`）をすでに毎回たどっているので、その上の物に位置を渡すのは型の確認と座標の変換だけで済み、割り当ては無い。押下の有無にかかわらず、ポインターがその上にある間のすべての移動を受け取る。
- **B. `HoverTarget.HandleHover` に位置を足す。** 今あるすべての実装を変える必要があり、出入りだけを知りたい物にも位置を渡すことになる。
- **C. `Listener` に押下の無い移動も渡す。** `Listener` の意味（押下を受け取り、その押下の移動を追う）が変わり、ドラッグの実装が移動の種類を見分ける必要が出る。

### 曜日の名前

- **A. 3 つの組にする。** `Months` と `ShortMonths` に合わせ、`Weekdays` を正式な名前（Sunday、日曜日）、`ShortWeekdays` を短い名前（Sun、日）、`NarrowWeekdays` をカレンダーの列見出し（Su、日）とする。今の `WeekdayNames` は `Weekdays` に、今の `Weekdays` は `NarrowWeekdays` になる。CLDR では「Su」を short、「S」を narrow と呼ぶが、gowaku の `ShortMonths` が CLDR の abbreviated（Jan）に当たるので、それに揃えた。
- **B. `WeekdaysShort` を足すだけにする。** 名前が増えるが、`Weekdays`（2 文字）と `WeekdayNames`（正式）との関係が名前から分からない。

### 日付の書式

- **A. `Strings` に `ShortDate`（曜日付きで年の無い日付：「Mon, Nov 16」、「11月16日（月）」）と `MonthDay`（「Nov 16」、「11月16日」）を足す。** 今の `Date` や `SpokenDate` と同じく、`FormatShortDate` と `FormatMonthDay` で表ごとに書き換えられ、nil なら英語の語順に表の言葉を入れる。
- **B. `time.Time.Format` のレイアウト文字列を表に持たせる。** 日本語の「（月）」や、ドイツ語の「16. Nov.」のような語順を、Go のレイアウトでは名前の表と組み合わせて書けない。

### 週の始まり

- **A. `basic.WeekStart(ctx)` と `basic.WeekStartOf(locale)` を公開する。** 週の始まりは言葉の表ではなく、ロケールの地域から決まる（`Strings` の godoc に書いてある通り）。ロケールを読む `basic.LocaleOf` と、数字を書く `basic.FormatInt` と同じ場所に置く。
- **B. `Strings` のフィールドにする。** 同じ英語でも米国（日曜）と英国（月曜）で違うので、表の言葉としては持てない。

### フォーカスの外し方

調べたところ、今の動作は次の通りである。押下を受け取る物の無い所をクリックすると、フォーカスが外れる（`Tree.pointerDown` の `clearFocus`）。Escape は `field.Input` ではフォーカスを外さない（`field.Search` では文字列を消す）。Tab は次の部品にフォーカスを移す。コードからフォーカスを外す公開の手段は無かった。

- **A. `interact.Unfocus(ctx)` を足す。** `core.RenderBase.ClearFocus()` を足し、`ctx.RenderObject()` から呼ぶ。`Actions` と同じパッケージに置き、検索欄の `OnSubmit` や Escape から呼べる。
- **B. Escape でテキスト欄のフォーカスを外す。** ブラウザやプラットフォームのテキスト欄はそうしないし、ダイアログの中では Escape はダイアログを閉じる。
- **C. `FocusScope` のような部品を作る。** フォーカスを外すだけには大きすぎる。

## 決定

- 案 A。`HideLabel bool` を `field.Input`、`field.Search`、`field.DropdownMenu`、`field.NumberField`、`field.DatePicker`、`field.TimePicker`、`field.Autocomplete`、`field.TokenField` に足す。見出しを描かず、そのための場所も取らない。`Label` は名前としてスクリーンリーダーに渡る。`field.ColorPicker`、`editor.Editor`、`control.NumberStepper` の `Label` はもともと見出しを描かないので、足さない。`collection.Table` のセルの編集欄は、列の見出しを `Label` にして `HideLabel` を設定する。`field.TokenField` の入力欄は、リストと同じ名前を持つ。
- 案 A。`core.HoverMoveTarget` と `interact.MouseRegion.OnHover` を足す。位置は領域の中の座標で、入ったときは `OnEnter` の後に来る。
- 案 A。`basic.Strings` の曜日を `Weekdays`、`ShortWeekdays`、`NarrowWeekdays` にし、`WeekdayNames` は無くす。`field.Calendar` の列見出しは `NarrowWeekdays`、読み上げは `Weekdays` を使う。
- 案 A。`Strings.ShortDate` と `Strings.MonthDay`、`FormatShortDate` と `FormatMonthDay` を足す。日本語の表は「11月16日（月）」「11月16日」と書き、数字と漢字の間に空白を入れない。間の空きは組版が入れる（JLReq）。曜日を囲む括弧は、日本語の文の中に置くので全角の「（）」にした。
- 案 A。`basic.WeekStart` と `basic.WeekStartOf` を公開し、`field.Calendar` の `weekStartOf` を消してそれを使う。
- 案 A。`interact.Unfocus` と `core.RenderBase.ClearFocus` を足し、`Actions` の godoc にフォーカスの外し方（押下を受け取る物の無い所のクリック、`Unfocus`、Escape だけでは外れないこと）を書く。`field.Input` の godoc に、タップは選択しないこと、置き換えるには Ctrl+A（macOS では Cmd+A）の後に打つことを書き、テストのガイド（en、ja）の表の行にも書く。始め方のガイド（en、ja）に、`go mod tidy` の前に import を書くことを 1 文で書く。

## 結果

- 設定の行や表のセルで、見出しを描かずに名前の付いた欄を置ける。テストは、8 つの部品（`Input` は 1 行と複数行、`NumberField` は無効のときも）それぞれで、`HideLabel` のとき `Label` の名前の付いた部品があり、見出しの文字が無く、unlabeled の警告が出ず、欄の高さが `Label` の無いときと同じで、見出しのあるときより低いことを確かめる。`Label` も `Placeholder` も無い `Input` は、`HideLabel` でも警告が出る。`TokenField` の入力欄は、名前が `Label`、`Placeholder` がヒントになり、テストを直した。
- `OnHover` で、ポインターについて動くツールチップを 1 つの `MouseRegion` で作れる。テストは、`gowakutest` の `HoverAt` で 2 か所に動かし、領域の中の座標（10, 10 と 180, 80）が順に届き、`OnEnter` が先に来て、領域の外に出ると `OnExit` の後は届かないこと、中の `GestureDetector` が今まで通りタップを受け取ることを確かめる。`core` のテストは、`HoverMoveTarget` へのホバーの移動が割り当てをしないことを確かめる。
- 曜日の名前の変更で、`Weekdays` の意味が変わった（2 文字から正式な名前へ）。gowaku はまだ公開していないので、互換のための別名は置かない。中の使い手（`field.Calendar`、golden のドイツ語の表）は直した。`ShortWeekdays` を設定しない表は、ほかの言葉と同じく英語の「Sun」を使う。
- テストは、英語と日本語で `ShortDate`、`MonthDay`、3 つの曜日の名前を確かめ、日本語の日付に空白が無いこと、書式の無い表が英語の語順で書くこと、`WeekStartOf` が地域から週の始まりを決めること（`field` から移した）を確かめる。
- `interact.Unfocus` のテストは、検索欄で打った Space がショートカットを動かさず、`OnSubmit` から `Unfocus` を呼んだ後は Space で動くこと、欄の外のクリックでも同じになることを確かめる。
