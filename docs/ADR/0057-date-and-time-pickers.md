# 0057: カレンダー、日付と時刻の入力（15a）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 で DatePicker / TimePicker を作ることにした。週の始まり、月と曜日の名前、12 / 24 時間制はロケールから取る。部品は時計を読まない（CODING_STYLE §9）。

## 決定

### 1. API

- `Calendar{Value, Today, Min, Max time.Time; Selectable func(time.Time) bool; OnChange}`。「今日」は `Today` で受け取る。`OnChange` に渡すのは、Value のゾーン（無ければ Today、それも無ければ UTC）での 0 時。
- `DatePicker{Value, Today, Min, Max, Selectable, Label, Placeholder, ShowToday, ShowClear, OnChange}`。クリック、Enter、Space、Alt+↓、F4 で開く。Escape と外側のクリックで閉じ、フォーカスは欄に戻る。クリアは `OnChange(time.Time{})`。
- `TimePicker{Value time.Duration（0 時からの経過）, HourCycle, Seconds, MinuteStep, Label, OnChange}`。`HourCycle` のゼロ値はロケールに従う。

### 2. カレンダーのキーボードとフォーカス

ARIA の日付グリッドに合わせた。矢印で日と週、PageUp / PageDown で月、Shift 付きで年、Home / End で週の始めと終わりへ移る。Tab で止まる日は 1 つだけで、スクリーンリーダーにはセルそのものにフォーカスがあるように見せる。42 個のセルがそれぞれ RenderFocus を持ち、有効なのはフォーカス中の日だけにした。

### 3. セルの数字は自前で描く

セルのラベル（「2026年11月9日 月曜日」）と、見えている「9」は違う。Text で描くとテキストのノードが残り、二重に読まれる。そこで、Semantics を出さない葉の RenderObject（`face`）で描く。時刻の各部分と日付欄の表示にも使う。

### 4. Semantics

- カレンダーは `RoleGrid`（名前は「2026年11月」）、曜日は `RoleColumnHeader`（「月曜日」）、週は `RoleRow`、日は `RoleGridCell`（名前は曜日まで含めた日付、`Selected`、`ActionTap`）。UIA では DataGrid / HeaderItem / DataItem になる。
- **日のセルが選択状態を伝えていなかった。** UIA の SelectionItem は radio、tab、list item にしか付けていなかった。grid cell にも付け、AT-SPI では selectable / selected にした。gallery を UIA から操作し、選択中の日が `IsSelected`、別の日を `Select()` すると値が変わってポップオーバーが閉じることを確かめた。
- DatePicker は `RoleComboBox`（Value は日付の文字列、Expanded / Collapsed）。名前の無いときは「日付を選択」。
- TimePicker は全体を `RoleGroup`（値は整形した時刻）、時、分、秒、午前 / 午後をそれぞれ `RoleStepper` にした。Chromium の `<input type=time>` と同じ構成で、各部分が Range と増減、値の設定を持つ。

### 5. 12 時間制と刻み

時を上下すると時刻そのものが 1 時間動き、11 AM → 12 PM のように午前と午後をまたぐ。分と秒は繰り上がらずに一周する。`MinuteStep` は上下キーのときだけ効き、次の倍数へ寄せる。

### 6. 週の始まり

ロケールに地域があれば CLDR の表で決める（en-GB は月曜、ar-EG は土曜、US と JP は日曜）。地域が無ければ、文言の表の `FirstWeekday` を使う。

### 7. ポップオーバーのテーマとロケール

DatePicker は、自分の Build で取った Theme と MediaQuery を、オーバーレイ側のカードに包み直して渡す。欄の周りで上書きしたテーマやロケールが、カレンダーにも届くようにするため（メニューはまだこれをしていない）。

### 8. 見た目

選択中の日は Accent の円に白文字。今日は AccentInk の文字にウェイト 600。選べない日は Ink3 に取り消し線、前後の月の日は Ink3。ポップオーバーは Raised、角丸 12、Lift Float、余白 12。TimePicker でフォーカス中の部分は Accent の塗りに白文字で、フォーカスがある間は TextField と同じく欄全体に輪が出る。

## 結果

- golden は `calendar`、`date-picker-open`（ライトとダーク）、`time-pickers-ja`、`time-pickers-12h`（ライトとダーク）。golden の文字は欧文フォントだけなので、日本語ロケールのシーンは数字で済むものにした。和文の見た目は gallery のスクリーンショットで確かめた。
- 残り: 窓がフォーカスを失っても、ポップオーバーは閉じない（メニューは閉じる）。TimePicker で 1 桁目を打った後の待ち状態に時間切れが無い（時計を持たないため。フォーカスの移動か矢印で解ける）。NVDA での読み上げは耳で確かめていない。
