# 0064: ローカライズ（15c）

決定日: 2026-11-09

状態: 採用

## 背景

ADR 0053 §2 で、次を決めた。

- 日本語と英語の文言を同梱する。
- OS のロケールを既定にし、`MediaQuery.Locale` で上書きできる。
- 利用者は文言の表を差し替えて、言語を足せる。
- 日付と数値の書式は `x/text`。

15a では、非公開の `localStrings` に日英の表を置いていた。

## 決定

### 1. 公開の文言の表 `Strings`

- 言語ごとに平たい 1 つの表にした。`Language` と、標準の部品が出すすべての言葉を持つ。
- 語順や複数形が言語で変わるものは、関数の欄にした。
  - 欄: `FormatMonthYear`、`FormatDate`、`FormatSpokenDate`、`FormatMatchCount`、`FormatRemoveNamed`、`FormatCloseNamed`
  - テンプレートではなく関数にしたのは、どんな語順にも複数形にも対応できるため。
- `EnglishStrings()` と `JapaneseStrings()` は写しを返す。新しい言語の出発点にできる。
- **足りない欄は英語で埋める。ライブラリの中ではリフレクションを使わない。**
  - 利用者の表は 1 回だけ写し、明示的な `fillFrom` で埋める。埋めるのは、空の文字列と、最初の要素が空の配列。
  - 関数の欄が nil なら、その表の言葉を使って英語の語順で書く。だから英語の表は関数を持たず、日本語の表は 6 つすべてを持つ。
  - bool の欄は、ゼロ値が英語のふるまい。

### 2. グローバルな登録簿は作らない

- CODING_STYLE §3.2 は、パッケージ変数の書き換えを禁じている。足す表はアプリごとに `App.Strings` で渡す。
- `Run` は、各ウィンドウの `MediaQuery` と `Theme` の間に `Localizations` を置く。部分木ごとに `Localizations` を置くこともでき、内側が外側に代わる（混ぜない）。
- **照合。**
  - 候補は「英語、日本語、利用者の表」。同じ言語タグの利用者の表は、その候補を置き換える。
  - `language.NewMatcher` で照らし、確信が No なら英語（いつも先頭）にする。
  - 例: ja-JP → ja、de-AT → de、pt-BR → en（pt を足していなければ）。
- **キャッシュ。**
  - 照合器と埋めた表は、表のスライスが変わったとき（ポインタで比べる）だけ作り直す。ロケールの照合も、ロケールが変わったときだけする。
  - 木には `*localized` のポインタを 1 つ配る。中身は、ロケール、表、`message.Printer`。
  - `StringsOf` は Inherited を 2 回引くだけで、34ns、0 allocs/op。
- `Localizations` の下の `MediaQuery` でも、ロケールを変えられる。`StringsOf` は、いちばん近い `MediaQuery.Locale` をキャッシュと比べ、違えば照らし直す。gallery と golden は、これで部分木ごとにロケールを変えている。
- DatePicker のポップオーバーは、欄の `*localized` を包み直して渡す。オーバーレイは、部分木の `Localizations` より上にあることがあるため（`locale-de-date-picker-open` の golden で確かめた）。

### 3. 週の始まりは表から外す

- CLDR の地域の表で決める。タグに地域が無ければ、ありそうな地域を補う（en → US、ja → JP、de → DE）。
- `FirstWeekday` の欄は消した。en と ja の結果は変わらない。

### 4. 数値はロケールに、言葉は照らした表に従う

- 数値は `x/text/message` の `number.Decimal` で書く。英語の表のままでも、de-CH ならスイスの桁区切りになる。
- `FormatInt(ctx, v)` と `FormatDecimal(ctx, v, digits)` を公開した。`Stepper` は、`Format` が無ければ `FormatInt` を使う。
- 日付は表のまま書く。x/text には CLDR の日付の書式が無いため。

### 5. OS のロケール

- `Platform.Locale()` は BCP 47 のタグを文字列で返す。platform が x/text を import しないで済むよう文字列にし、ルートで解釈する。
- **Windows**
  - `GetUserDefaultLocaleName`（地域の書式のロケール）を読む。並べ替えの接尾辞（`_phoneb` など）は落とし、不変ロケールや私用のロケールは空にする。
  - `WM_SETTINGCHANGE` の "intl" で読み直し、変わっていたときだけ作り直す。
- **Linux（X11、Wayland 共通）**
  - gettext と同じ順で読む: LANGUAGE（ロケールが C でなければ）、LC_ALL、LC_MESSAGES、LANG。
  - "ja_JP.UTF-8" は ja-JP、`@latin` は文字体系、C / POSIX / 空は en。
  - 起動中の変化は見ない。
- `App.Locale` で上書きできる（ゼロ値なら OS に従う）。

### 6. gallery

- OS のロケールに従い、`-locale ja|en` で変えられる。
- `-png` は既定で日本語にした。レビューのスクリーンショットが変わらないようにするため。
- ページそのものの文は日本語のまま。標準の部品の言葉だけが変わる。

## 結果

- golden は `locale-en`、`locale-de`、`locale-de-dark`、`locale-de-date-picker-open`。
  - ドイツ語の表は、わざと一部を空にしたものをシーンの中で作り、英語で埋まることを確かめた。
  - 映っているもの: 月曜始まり、「9. Nov. 2026」、24 時間制、「12.345」、「Suchen」、ポップオーバーの「Heute」/「Löschen」。
  - 日本語の golden は無い（golden のフォントに仮名が無いため）。
- 残り:
  - Windows は UI の言語（`GetUserPreferredUILanguages`）ではなく、地域の書式のロケールを読む。`MediaQueryData.Locale` は言葉と書式で 1 つのタグを共有している。
  - Linux は起動中のロケールの変化を拾わない。
  - `Hour24` が表にあるが、12 / 24 時間制は本来は地域の選択（英語の表の en-GB は 12 時間制になる）。
  - 入れ子の `Localizations` は外側と混ざらない。
  - x/text の CLDR データが古く、fr-FR の桁区切りが狭い空白にならない。
