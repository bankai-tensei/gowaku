# 0143: 入力欄の幅とプレースホルダー、Input の無効と読み取り専用と行数、閉じるのを止められるダイアログ、アイコンだけのボタン、メッセージの日付

決定日: 2026-11-17

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 5 つが見つかった。

1. **`field.DropdownMenu` だけ扱いが違う：** プレースホルダーを `Ink3` の `basic.Text` で描いていたので、`gowakutest` の Warnings がコントラスト不足（contrast）として報告した。`field.Input` は `RenderEditable` が、`field.DatePicker` は `face` がプレースホルダーを自分で描くので、`RenderParagraph` を調べる Warnings には見えず、報告されない。また、`DropdownMenu` は最も長い選択肢の幅の `SizedBox` で包まれていたので、幅いっぱいに広がる `Input` や `DatePicker` と並べると、フォームの右端がそろわなかった。調べると `DatePicker` も、`Label` のあるときは見出しの `Column`（`CrossStretch`）のおかげで広がるが、`Label` の無いときや `HideLabel` のときは 160 の固定幅だった。
2. **`field.Input` に無効と読み取り専用が無い：** 表示だけの欄（API キー、生成された ID）や、権限の無い人に見せる欄を作れなかった。また、複数行の欄は、高さを決めなければ文字に合わせて際限なく伸び、決めると最初からその高さになる。チャットの入力欄のように「N 行までは伸び、その先はスクロールする」欄を作れなかった。
3. **ダイアログを閉じるのを止められない：** `overlay.ShowDialog` のダイアログは、Escape と外側のクリックで閉じ、アプリに知らせる手段が無い。「下書きを破棄しますか？」と聞くことができなかった。
4. **アイコンだけのボタンが無い：** `control.Button` は必ずラベルを描く。`nav.Toolbar` の外でアイコンだけのボタンを作るには、`interact.Pressable` から自分で組む必要があった。
5. **メッセージの一覧の日付：** 「今日なら時刻、この 1 週間なら曜日、それより前なら短い日付」は、メールやチャットの一覧の決まった書き方だが、`basic.Strings` に無く、各アプリが英語と日本語で書いていた。時刻を書く手段も無かった。

## 選択肢

### プレースホルダー

- **A. `Input` と `DatePicker` と同じく、プレースホルダーを `face` で描く。** `face` はアクセシビリティのノードを出さない描画だけの部品で、プレースホルダーは欄のセマンティクスが運ぶ。`Input` と同じく、`Label` があればヒントに、無ければ名前にする。
- **B. Warnings がセマンティクスの役割を見て、欄の中の `Ink3` の文字を除く。** `gowakutest` が部品の内部を知ることになり、ほかの部品の本当に読めない文字も見逃す。
- **C. プレースホルダーを `Ink2` にする。** プレースホルダーと入力済みの値の見分けがつきにくくなり、ほかの欄とも違う。

### 欄の幅

- **A. `Input` の規則に合わせる。** 親が幅を決めていれば（`MaxW` が有限なら）その幅いっぱいに、`Row` の中のように決めていなければ自分の幅（`DropdownMenu` は最も長い選択肢、`DatePicker` は 160）になる。`field` の中に `fieldWidth` という小さな描画物を置き、`DropdownMenu` と `DatePicker` の外側（見出しを含む）を包む。見出しの有無で幅が変わらなくなる。
- **B. `DropdownMenu` に `Expand bool` を足す。** ゼロ値の見た目は変わらないが、フォームを作る人がそれぞれ設定する必要があり、`Input` と違う規則が残る。
- **C. `CrossStretch` のときだけ広げる。** 描画物は親の揃え方を知らない。制約から見えるのは最大幅だけである。

### 無効と読み取り専用

- **A. `Disabled bool` と `ReadOnly bool` を `Input` に足す。** `Input` は自分で文字列を持つので、`Button` のように「`OnChange` が nil なら無効」にはできない（`OnChange` の無い `Input` は今も普通に使われる）。`Enabled bool` はゼロ値で無効になる。そこで、選択肢の型（ADR 0054）と同じく、ゼロ値が動く部品になるよう否定形の `Disabled` にする。`ReadOnly` は `editor.Editor` と同じ名前である。
- **B. `Mode` の列挙（編集可、読み取り専用、無効）にする。** 1 つのフィールドで済むが、ほかの部品の書き方（`Editor.ReadOnly`、選択肢の `Disabled`）と合わない。

無効は、キーボードもポインターも受け取らず（`CanFocus` が false）、`Fill` の地と `Ink3` の文字で描き、セマンティクスは `Disabled` で操作が無い。フォーカスを持っているときに無効になると、木がフレームの後にフォーカスを外す（`dropHiddenFocus`）。読み取り専用は、フォーカスを受け取り、キャレットの移動、選択、コピー、全選択、Enter での送信ができ、入力、IME、貼り付け、切り取り、元に戻す、削除、スクリーンリーダーからの値の設定は何もしない。IME はオフにする。セマンティクスは `ReadOnly` で、操作は選択の設定だけである。`Value` と `InputController` からの変更は今まで通り効く（利用者の編集だけを止める）。

### 行数

- **A. `MinLines` と `MaxLines` を足す。** 高さが決まっていない複数行の欄は、文字の行数を `MinLines` と `MaxLines` の間に収めた高さになり、それを超えるとスクロールする。1 行の高さは、文字のレイアウトの高さを行数で割ったもので、ルビの余白も含めてスタイルから決まる。親が高さを決めればそちらが勝ち、1 行の欄では無視する。
- **B. `MaxHeight float32` にする。** 行の途中で切れる高さになりやすく、文字の大きさを変えると合わなくなる。

### ダイアログを閉じるのを止める

- **A. `Dialog.OnDismissRequest func()` を足す。** 設定すると、Escape と外側のクリックはダイアログを閉じずにそれを呼ぶ。アプリはそこで自分の確認を出し、閉じると決めたら `ShowDialog` の dismiss を呼ぶ。dismiss はいつでも閉じる。ルートは `core.Inherited[*dialogGate]` を `build` の結果の上に置き、`Dialog` は Build でそこに自分の `OnDismissRequest` を書く。`Dialog` が何かに包まれていても届く。`Sheet` にも同じフィールドを足す（`Persistent` のシートでは、外側のクリックは今まで通り無視する）。
- **B. `OnDismiss func() bool` で、false を返すと閉じない。** 確認のダイアログを開いて答えを待つ間、関数は返さなければならないので、非同期の確認には結局 A の形が要る。
- **C. `ShowDialog` の引数にオプションの構造体を足す（`ShowSheet` のように）。** すべての呼び出しが変わる。閉じ方はダイアログの中身（下書きがあるかどうか）で決まることが多く、中身を作る `Dialog` に置く方が自然である。

押しっぱなしの Escape（キーリピート）は、確認を何度も出さないよう、最初の 1 回だけを扱う。

### アイコンだけのボタン

- **A. `Button.HideLabel bool` を足す。** 入力欄の `HideLabel`（ADR 0140）と同じく、`Label` は名前のまま変わらず、見出しを描かないことだけを意味する。ボタンの高さの正方形に `Icon` だけを描き、アイコンは標準で 16（`nav.Toolbar` と同じ。小 14、大 20）。`ButtonQuiet` のときは `nav.Toolbar` のボタンと同じ見た目（地はホバーで出る、アイコンは `Ink2` でホバーで `Ink`）にする。`Icon` が無ければラベルを描く。
- **B. `IconButton` という別の部品を作る。** スタイル、大きさ、無効、フォーカスの扱いがすべて `Button` と重なる。

ツールチップ：`overlay.Tooltip` は `control` より上の層にあり、`control` からは import できない（ADR 0081）。godoc に `overlay.Tooltip` で包む書き方を 1 行で示すにとどめた。

### メッセージの日付

- **A. `Strings.MessageDate(t, now time.Time)` と `Strings.Time(t)`（`FormatTime`）を足す。** 今日なら `Time`（「3:04 PM」「15:04」）、その前の 6 日なら曜日の正式な名前（「Monday」「月曜日」）、今年のそれより前なら `MonthDay`（「Nov 9」「11月9日」）、ほかの年なら `Date`（「Nov 9, 2025」「2025/11/09」）。今日より後は日付で書く。日は `now` のタイムゾーンで数える。`now` は呼ぶ側が渡す（テストで固定できるように）。`Time` は、`FormatTime` が無い表では `Hour24` なら 0〜23 時、そうでなければ 1〜12 時に `AM`/`PM` を付け、`PeriodFirst` なら前に置く。
- **B. 「この週」を `WeekStart` から決める。** 週の始まりはロケールの地域で決まり（ADR 0140）、`Strings` には地域が無い。月曜の朝に、前日の日曜が「日曜日」でなく日付になるのも一覧として読みにくい。直前の 6 日なら曜日が重ならない。
- **C. 「昨日」を入れる。** 多くのメールのアプリはそうするが、表に言葉が増え、ほかの言語の表を作る人の負担になる。必要になったら足す。

## 決定

- 案 A。`DropdownMenu` のプレースホルダーを `face` で描き、`Label` があればヒントに、無ければ名前にする。`face` は幅より長い文字を端で切るようにした。
- 案 A。`fieldWidth` で `DropdownMenu` と `DatePicker` に `Input` と同じ幅の規則を与える。`TimePicker` は時と分の部品を並べた欄で、広げても中身が広がらないので、今まで通り自分の幅のままにした。
- 案 A。`Input.Disabled`、`Input.ReadOnly`、`Input.MinLines`、`Input.MaxLines` を足す。CODING_STYLE §4.1 の否定形の例外に `Input.Disabled` を書き足した（en、ja）。
- 案 A。`Dialog.OnDismissRequest` と `Sheet.OnDismissRequest` を足す。
- 案 A。`Button.HideLabel` を足す。
- 案 A。`Strings.MessageDate`、`Strings.Time`、`Strings.FormatTime` を足し、日本語の表に `FormatTime` を書いた。日本語の文字列には、数字と漢字の間に空白を入れない（ADR 0140）。

## 結果

- フォームの欄の右端がそろう。`DropdownMenu` が広がるようになったので、`collection.PropertyGrid` の選択の欄もほかの編集欄と同じ幅になった。カタログの絵（`control.Button`、`field.DropdownMenu`、`collection.PropertyGrid`）と、golden の `form-error`、`time-pickers-ja`、`pane`、`locale-de-date-picker-open`（ライトとダーク）を作り直した。カタログの `field.DropdownMenu` の場面は、幅 200 の `SizedBox` に入れた。`Row` の中の `DropdownMenu` と `DatePicker` は今まで通り自分の幅である。
- テストは、`Input`、`DatePicker`、`DropdownMenu` が、見出しの有無と `CrossStart`/`CrossStretch` にかかわらず親の幅（400）になり、`Row` の中では自分の幅になること、`DropdownMenu` のプレースホルダーが警告を出さず、ヒントまたは名前になり、文字のノードにならないことを確かめる。
- `Input` のテストは、無効の欄がセマンティクスで `Disabled` を持ち、操作が無く、タップでフォーカスを取らず、入力も削除も効かず、`OnChange` が呼ばれず、コントラストの警告が出ないこと、フォーカス中に無効になるとフォーカスが外れることを確かめる。読み取り専用の欄は、`ReadOnly` を持ち、操作は選択だけで、入力、IME、削除、貼り付け、切り取り、元に戻すが効かず、全選択とコピー、Shift+矢印での選択、Enter での送信ができることを確かめる。行数は、`MaxLines` 3 で 1、3、4、9 行のとき高さが 1、3、3、3 行分になり、`MinLines` 3 と `MaxLines` 5 で 0、1、4、7 行のとき 3、3、4、5 行分になり、1 行の欄は無視すること、`MaxLines` 2 で 7 行打つと、キャレットの行が欄の中に見え、最初の行がスクロールで外に出ることを確かめる。
- ダイアログのテストは、`OnDismissRequest` が無ければ Escape と外側のクリックで閉じること、あれば閉じずに 1 回ずつ呼ばれ（リピートの Escape は数えない）、ダイアログの上のクリックでは呼ばれず、dismiss で閉じること、閉じるボタン（dismiss）は聞かずに閉じることを確かめる。シートは、`Persistent` でなければ Escape とクリックで 2 回、`Persistent` なら Escape の 1 回だけ呼ばれることを確かめる。
- `Button.HideLabel` のテストは、4 つのスタイルと 3 つの大きさで、ボタンが 30、24、38 の正方形になり、名前が `Label` で、ラベルの文字が無く、警告が無く、押せること、アイコンが無ければラベルを描くこと、`OnPress` が無ければ無効であることを確かめる。
- `MessageDate` のテストは、英語と日本語で、今日の朝と午後、0 時、正午、昨日、6 日前、7 日前、今年、去年、明日を確かめ、日本語に空白が無いこと、日を `now` のタイムゾーンで数えること、書式の無い表の `Time` が `Hour24` と `PeriodFirst` に従うことを確かめる。
- `Search`、`NumberField`、`Autocomplete`、`TokenField` には無効と読み取り専用を足していない（`NumberField` は今まで通り `OnChange` が nil なら無効）。必要になったら `Input` の上に足す。→ ADR 0145 で足した。「昨日」も ADR 0145 で入れた。
