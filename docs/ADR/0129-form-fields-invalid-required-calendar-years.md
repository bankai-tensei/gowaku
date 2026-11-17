# 0129: 入力部品の不正状態の統一、選択部品の必須エラー、カレンダーの月と年の一覧、NumberField の Enter

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に、日本語の会員登録ウィザードを `widgets/field` で作ってもらったところ、次の 5 つが見つかった。

1. **ColorPicker の「HEX」のコントラスト：** hex の欄の先頭に置く「HEX」（数値の欄の R、G、B なども同じ）は `Ink3` で描かれ、コントラストは 2.94:1 だった。`gowakutest` の `Warnings` が指摘するが、アプリの側では直せない。
2. **不正状態の不統一：** `FormItem` の中の `Input` と `DropdownMenu` はエラーで枠が赤くなるが、`DatePicker` はエラーの文字だけが出て枠は変わらなかった。`TimePicker`、`ColorPicker` も `FormItem` を知らず、スクリーンリーダーにも Invalid が伝わらなかった。`Autocomplete` はエラーを Hint で伝えるが、Invalid の状態を立てていなかった。
3. **生年月日が選べない：** `Calendar` には月を 1 つずつ動かす ‹ › しかなく、2026 年から 1990 年まで行くには 400 回以上押す必要があった。
4. **選択の部品の必須エラー：** `FormItem{Required: true}` のエラーは部品にかかわらず「入力してください」だった。`DropdownMenu`、`DatePicker`、`RadioGroup`、同意の `Checkbox` には合わない。
5. **NumberField の Enter：** `FormItem` の中でだけ Enter がフォームを送信することは、ソースのコメントにしか書かれていなかった。さらに、Enter で確定した新しい値は親が作り直されるまで `FormItem` に届かないため、フォームは古い値を検証していた。

## 選択肢

### 1. HEX の色

- **A. `Ink2` にする。** `Ink2` はどの面の上でも 4.5:1 を満たす（ADR 0041）。欄の中身を説明する文字なので、情報を持つ文字として扱うのが正しい。
- **B. 文字をやめてアイコンにする。** 「HEX」「R」「G」「B」に合うアイコンは無く、かえって分かりにくい。

### 2. 部品の不正状態

- **A. 各部品が `fieldScope` を読み、`Input` と同じ規則で描く。** エラーがあれば枠とフォーカスの輪を `Danger` にし、Semantics に Hint と Invalid、Required を載せる。
- **B. `FormItem` が子の周りに赤い枠を描く。** 部品ごとの角丸や大きさに合わず、`Input` と二重の枠になる。

`ColorPicker` には欄としての枠が 1 つではないので、彩度と明度の平面の枠と hex の欄の枠を赤くし、グループの Semantics でエラーを伝える。中の hex と数値の欄は、`FormItem` へのフォーカスの通知も Enter による送信もしない（ピッカー全体が 1 つの欄である）。

フォーカスの扱いでは、`TimePicker` は部分ごとにフォーカスを持つので、時から分へ移るたびに「欄を離れた」と判定されてしまう。そこで `FormItem` は、フォーカスが外れた通知の後、検証する build より前にフォーカスを得た通知が来たら、外れた通知を取り消す。コアは古いフォーカスの false と新しいフォーカスの true を続けて同期的に呼ぶので、部分の間の移動は検証されない。`DatePicker` は `DropdownMenu` と同じく、カレンダーがフォーカスを取ったことは離脱とみなさず、Escape などで閉じてフィールドに戻ったときも離脱としない。

### 3. 遠い日付への移動

- **A. 見出しを押すと月の一覧、もう一度押すと 10 年分の年の一覧に切り替える（Windows の CalendarView、macOS のカレンダーと同じ）。** 年を選ぶとその年の月、月を選ぶとその月の日に戻る。
- **B. 年のドロップダウンを置く。** 100 年分のリストは長く、見出しの静かな見た目も崩れる。
- **C. 開く月を指定する `InitialMonth` だけを足す。** 生年月日のように見当が付く場合は十分だが、見当が外れたときにやはり遠い。

A と C の両方を採る。`InitialMonth` は `Value` が空のときに開く月で、`Value`、`InitialMonth`、`Today`、`Min` の順に使う。`Min` と `Max` は既にある。

月と年の一覧は 4 列 3 行で、日の一覧（曜日の行を含めて 216px）と同じ高さにし、切り替えてもカードの大きさが変わらないようにした。年の一覧は 10 年の前後の 1 年ずつを薄く表示する（月の前後の日を薄くするのと同じ）。選んだ月や年は日と同じアクセントの丸、今日を含む月や年は `AccentInk` の太字、`Min` と `Max` の外は取り消し線である。見出しのボタンには小さな下向きの山形を付け、押せることを示す。10 年分の見出し（「2020–2029」）は最後の段階なので押せない。

キーボードでは、見出しは Tab で届くボタンで、Enter で一覧が切り替わり、フォーカスは新しい一覧のその月または年に移る（元のセルは消えるため）。一覧では矢印キーで 1 つと 1 行（4 つ）、Home と End で行の端、Page Up と Page Down で 1 年または 10 年動き、Enter または Space で選ぶ。スクリーンリーダーには、見出しが見出しの中のボタンとして、月と年がグリッドのセル（「January 1990」「1990」、日本語では「1990年1月」「1990年」）として伝わる。

2026 年から 1990 年 5 月 15 日を選ぶ操作は、欄を開く、見出しを 2 回、‹ を 3 回、1990、5 月、15 日の 9 回になる。

### 4. 必須エラーの文言

- **A. `FormItem` の型 T で決める。** `DropdownMenu[string]` と `Input` はどちらも string で、区別できない。
- **B. 中の部品が自分の種類を `FormItem` に伝える。** `FormItem` は `core.Inherited[*kit.FieldReport]` を子に渡し、部品は build のときに `kit.ReportField` で種類（入力、選択、チェック）を書き込む。`Checkbox`、`RadioGroup`、`Chip` は `widgets/control` にあり `field` より下の層なので `field` の型を知れないが、`widgets/internal/kit` はどちらからも使える。
- **C. 利用者に毎回 `RequiredMessage` を書かせる。** 既定が正しくないことは変わらない。

B を採り、上書き用に `FormItem.RequiredMessage` も足す。報告が無く T が bool のとき（`Switch` やアプリの部品）は、チェックとみなす。文言は `Strings` に `RequiredChoice`（選択してください / Choose one）と `RequiredCheck`（チェックしてください / Check this box）を足した。

### 5. NumberField の Enter

- **A. `Input` のように `OnSubmit` を足す。** 検討したが、所有者の判断で採らない。部品が増やす API を最小に保つ。
- **B. 振る舞いを godoc とカタログに書く。** Enter は入力を値として確定し、`FormItem` の中ではそのうえで周りの `Form` を送信する。`FormItem` の外では確定するだけで何も送信しない。

B を採る。あわせて古い値を検証する問題を直した。Enter で値が変わったときは、送信を 2 フレーム目のティックまで遅らせる。1 回目のティックは親の build より前、2 回目はその後に来るので、`FormItem` は新しい値を検証する。値が変わらなければその場で送信する。`Form` に包まれていても `FormItem` の外の欄が Enter で送信しないのは、他の入力欄と同じである。

## 決定

- ColorPicker の欄の見出し（HEX、R、G、B、H、S）を `Ink2` にする。
- `DatePicker`、`TimePicker`、`ColorPicker` を `FormItem` に対応させ、エラーで枠を赤くし、Hint と Invalid、Required を伝える。`Autocomplete` は Invalid と Required も立てる。
- `FormItem` は、検証の前にフォーカスが戻った離脱を取り消す。
- `Calendar` の見出しから月と年の一覧に切り替えられるようにし、`Calendar.InitialMonth` と `DatePicker.InitialMonth` を足す。`Strings` に `ChooseMonth`、`ChooseYear`、`PreviousYear`、`NextYear`、`PreviousYears`、`NextYears`、`FormatYear`（メソッド `Year`、`Years`）を足す。
- `FormItem` の必須エラーを部品の種類で選び、`FormItem.RequiredMessage` で上書きできるようにする。`Strings` に `RequiredChoice`、`RequiredCheck` を足す。
- `NumberField` の Enter の振る舞いを godoc とカタログに書き、`FormItem` が新しい値で送信されるようにする。

## 結果

- 登録フォームのどの欄も、エラーの見え方とスクリーンリーダーへの伝わり方がそろった。
- 生年月日は `InitialMonth` で 2 回、そうでなくても 10 回前後の操作で選べる。
- `kit.FieldReport` を読むため、`Checkbox`、`RadioGroup`、`SegmentedControl`、切り替えの `Chip` は build のたびに祖先をたどる（`core.Lookup`）。build の処理であり、毎フレームの経路ではない。
- `Calendar` の見出しが Tab の移動先に加わった。日のセルへは Tab 1 回多くかかる。
- カタログの `field.DatePicker` の図は年の一覧を開いた状態になり、`field.Form` の図には生年月日と同意の欄が加わった。
