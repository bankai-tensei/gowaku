# 0056: 検索欄、ドロップダウン、候補、フォーム（15a）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 の 15a のうちフォームまわり。検証のタイミングはヒアリングで決めた。フォーカスを外したときと送信時に検証し、一度エラーを出した欄は打鍵ごとに検証し直す。初めて打っている欄は赤くしない。

## 決定

### 1. SearchField

左に虫眼鏡を置き、空のときは「検索 / Search」を出す。文字があると右に × を出す（Tab では止まらず、ラベルは「クリア」）。Escape で消す。中身が空なら Escape は外へ渡し、IME の変換中は IME に任せる。消去は 1 回の編集として扱うので、Ctrl+Z で戻せる。

### 2. DropdownMenu

- `DropdownMenu[T comparable]{Value, Options []DropdownOption[T], OnChange, Placeholder, Label}`。`OnChange` が nil なら無効。
- Space、Enter、Alt+↓、F4、クリックで開く。今の値が見える位置まで送り、矢印、Home、End、PageUp、PageDown で移る。先頭の文字でジャンプする（1 秒で入力をリセットし、同じ文字の連打は順に回る）。Enter で選び、Escape と Tab で閉じる。
- 閉じているときも、↑↓、Home、End、文字で値がその場で変わる（Windows のコンボボックスと同じ）。
- リストは下に開き、下に余地がなければ上に開く。8 行を超えるとスクロールする。
- メニュー（ADR 0046）の部品（`popup`、`barrier`、`modalScope`、`focusRef`）を使い、見た目もメニューと同じカードにした。`RoleComboBox` で Value と Expanded / Collapsed を持ち、UIA の ExpandCollapse で開ける。

### 3. Autocomplete

- `Autocomplete[T any]{Value, Options, Suggestions func(string) []T, Format, OnChange, OnSelect, OnSubmit, ShowNoResults}`。既定の絞り込みは大文字と小文字を区別せず、前方一致、続けて部分一致の順に並べる。
- **確定した文字だけで絞り込む。** 変換中は候補を開かず、↑↓、Enter、Escape も IME に渡す。
- リストに barrier は無く、modal でもない。フォーカスは欄に残る。強調中の候補は、リスト項目の `Selected` で示す。

### 4. Form と FormField

- `Form{Controller *FormController, OnSubmit, Child}`。`FormController` はゼロ値で使え、`Submit() bool` と `Reset()` を持つ。送信が失敗すると、画面上で一番上（同じ高さなら左）の誤りの欄にフォーカスを移す。`OnSubmit` を持たない 1 行の TextField で Enter を押すと、フォームを送信する。
- `FormField[T comparable]{Value, Required, Validators []func(T) string, Child}`。FormField が非公開の `core.Inherited` でエラーと欄への参照を配り、TextField、DropdownMenu、Checkbox の側がそれを読む。値は呼び出し側が持ち、`Value` で渡す。
- **`Required` は bool にした。** 「入力してください」の文はロケールで変わるが、検証の関数は ctx を持たないため。空白だけの文字列も空とみなす。
- 組み込みの検証は `MinLength(n, msg)`（文字数で数える）と `Pattern(re, msg)`（全体一致）。どちらも空文字は通す（空は Required の役目）。
- エラーは欄の下に Caption、Danger で出す。TextField と DropdownMenu は、枠と焦点の輪も Danger になる。Checkbox のようにフォーカスを報告しない部品は、送信時に初めて検証する。DropdownMenu のリストを開いたときのフォーカス移動は、欄を離れたことに数えない。

### 5. TextField に足したもの

非公開の `extras`（前と後ろの飾り、フォーカスの通知、エラー）を足した。前後の飾りがあるときだけ、中身を Row にする。公開の API は変えていない。

## 結果

- golden は `fields`、`dropdown-open`、`autocomplete-open`、`form-error`（それぞれライトとダーク）。gallery の「フォーム」のページでは、空のまま離れた欄と短い郵便番号のエラーが、離れたときに出ることを確かめた。
- **Semantics に足りないもの**（15b 以降で入れる）:
  - ~~「値が不正」の状態が無い。~~ 15b で `semantics.Invalid` と `semantics.Required` を足し、FormField が中の TextField と DropdownMenu に渡すようにした。UIA では `IsDataValidForForm` / `IsRequiredForForm`、AT-SPI では invalid-entry / required になる。エラーの文は今までどおり Hint で伝える。
  - Autocomplete は、フォーカスがテキスト欄にあるので、強調中の候補が読み上げられない。activedescendant に当たる仕組みが要る。
  - DropdownMenu の子に、今の値と同じテキストが残り、二重に読まれうる。
- 長いラベルの省略表示（…）は無い。
