# 0097: CommandPalette

決定日: 2026-11-13

状態: 採用

## 背景

当初の計画に `CommandPalette` があり、「notes の実装を `widgets` に上げるか。登録済みの `Action` をあいまい検索する薄い部品に留める」と範囲を先に決めてある。`examples/notes/palette.go` は、ダイアログ・入力欄・行の一覧・上下キーをサンプルの中で自前で持ち、検索は前方一致と部分一致だけだった。Actions（ADR 0046）がラベル・ショートカット・有効かどうかを 1 か所にまとめたので、それをそのまま探せる部品にする。

## 選択肢

- 何を探すか
  - A. `Action` の一覧だけ。
  - B. `Action` に加えて、任意の項目（最近のファイル、ノート）を足せる。
  - C. 項目の提供者（provider）、ページ、サブメニューを持つ、VS Code 並みのパレット。
- 「今の文脈の Action」をどう得るか
  - A. 祖先の `interact.Actions` を辿って集める。
  - B. アプリが同じ `[]Action` を渡す。
- あいまい一致
  - A. 先頭から貪欲に部分列を探す。
  - B. 動的計画法で最良の置き方を探す（fzf v2 と同じ考え方）。

## 決定

### 1. API（`widgets/field`）

```go
type PaletteItem struct {
	Label, Detail string
	OnInvoke      func()
}
type CommandPalette struct {
	Actions     []interact.Action
	Extra       []PaletteItem
	Shortcut    interact.Shortcut // ゼロ値は Ctrl+Shift+P
	Placeholder string
	RecentFirst bool
	Child       core.Widget
}
func OpenCommandPalette(ctx core.BuildContext)
func CommandPaletteAction(ctx core.BuildContext) interact.Action
```

- **探すもの（B）**: `Actions` と、追加の `Extra`。`Extra` は「ラベル・説明・実行」の 3 つだけの値で、notes の最近のファイルとノートがこれで書ける。提供者・ページ・サブメニュー（C）は作らない。当初の計画の範囲どおり。
- **文脈の Action（B）**: アプリが `interact.Actions` とメニューに渡すのと同じスライスを `Actions` に渡す。`interact.Actions` は RenderObject で、祖先を辿って集める口が無い。集める仕組みを足すと `Actions` の作りを変えることになり、当初の計画の互換方針に触れる。どの Action をパレットに出すか（無効なものを隠すか、内部用を外すか）をアプリが選べる利点もある。
- **置き場所は `field`**。検索欄（`field.Input`）とオーバーレイ（`overlay.ShowDialog`）の両方を使うので、`overlay` より上で、入力欄を持つ段に置いた。`nav` は `field` を import できない。
- **開くのも Action**: `CommandPalette` は自分の `Child` を、開く Action だけを持つ `interact.Actions` で包む。既定のショートカットは Ctrl+Shift+P（macOS では Cmd が `ModControl` になるので Cmd+Shift+P）。`Shortcut` で変えられる。ゼロ値が既定なので「ショートカット無し」は選べない。`CommandPaletteAction(ctx)` はメニューやツールバーに置くための同じ Action を返し、`OpenCommandPalette(ctx)` はボタンなどから開く。どちらも ctx の上に `CommandPalette` が無ければ panic（契約違反）。開いているときにもう一度開いても何もしない。

### 2. 動き

- `overlay.ShowDialog` の中に、ガラスの `Dialog`（幅 560）で出す。フォーカスの退避と復帰、Escape、外側のクリック、Tab の閉じ込めは `ShowDialog` のものをそのまま使う。
- 検索欄は、開いた次のフレームにキーボードを取る。ダイアログは最初のレイアウトで（子より後に）自分でフォーカスを取るので、同じフレームで欄に渡しても奪われ、さらに `ShowDialog` が「前のフォーカス」として欄を覚えてしまう。notes が `post` で後回しにしていたのと同じ理由で、`Ticker` の最初の 1 回で渡す。
- 行: ラベル（ニーモニックと末尾の「…」を除く。`"開く(&O)…"` は「開く」）、右端にショートカットのヒント（`Extra` は `Detail`）、`Checkable` で `Checked` の Action にはチェックマーク。一致した文字はアクセントの色と太さ 600 で示す。選んでいる行は `AccentTint`、ポインタの下は `Fill`。
- **無効な Action**: 灰色で出す。上下キーは飛ばし、Enter もクリックも何もしない。出しておくのは「その操作はあるが、今は使えない」と分かる方が探す人に親切だから。隠したいアプリは渡す前に除く（notes はそうしている）。
- キー: ↑↓ で選び、Enter で実行（パレットを閉じてから実行する）、Escape で閉じる。**IME が変換中**は Enter・↑↓ は IME のものなので何もしない。Escape も変換中はパレットを閉じない（`Autocomplete` と同じ）。
- 一度に見せるのは 8 行。選択が見える窓をずらし、8 行を超えるときは下に「3 / 20 件」（`Strings.MatchCount`）を出す。一覧の高さは固定で、絞り込んでも中央のダイアログが跳ねない（notes と同じ）。一致が無ければ `Strings.NoResults`。
- **開いている間も widget に従う**: 親が別の `Actions` で作り直すと、`DidUpdateWidget` で結果を作り直す。選択は同じ位置に留め、無効になっていれば次の有効な行に移す。
- `RecentFirst`: 実行した項目のラベルを新しい順に 8 個覚える（パレットの State の中。アプリをまたいで保存はしない）。問い合わせが空のときは最近のものを先頭に、空でないときは同点の中で最近のものを先に並べる。
- 並び順: 点数の高い順。同点なら（`RecentFirst` のとき）最近の順、その次は `Actions`、`Extra` の元の順。`Extra` の `Detail` にも一致させるが、ラベルで一致したものより後ろに置く。
- Semantics: 中身は `RoleGroup`（名前は「コマンドパレット」）、一覧は `RoleList`、行は `RoleListItem` で、選んでいる行に `Selected`、無効な行に `Disabled`、チェックに `Checked`、ショートカットは `Value`。行は Tab で止まらない（`SkipTab`）。

### 3. あいまい一致（`widgets/internal/kit/fuzzy.go`、B）

- 問い合わせの文字（rune）が、ラベルの中に順番どおりに現れれば一致（部分列）。空白は無視する。
- 点数: 一致した文字 16、語の頭 +10、ラベルの頭 +6、直前の文字と続いていれば +8。間を空けると 3、空けた文字ごとに 1 を引く。最初の一致より前の文字は 1 つ 1 点（8 点まで）引く。
- **語の頭**: 空白や記号の後、camelCase の大文字、そして **文字種の変わり目**（ひらがな・カタカナ・漢字・英字・数字）。日本語のラベルには空白が無いので、「ファイルを開く」の「開」、「名前を付けて保存」の「保存」の頭を文字種の境目で拾う。長音「ー」はカタカナの中に入れる。
- 最良の置き方は動的計画法（問い合わせ m × ラベル n の表）で探し、表の経路から一致した範囲を取り出す。貪欲法（A）では「Of Find Files」に "fil" を探すと最初の f と i を拾ってしまい、続いた「Fil」を見落とす。
- 大文字と小文字、**文字幅**（全角英数字 ↔ 半角、半角カナ → 全角）を同一視する。幅の畳み込みは notes の検索（`store/search.go`）が使っている `golang.org/x/text/width` と同じ方法で、新しい依存は無い。**ひらがなとカタカナの同一視はしない**。コードベースに既存の補助が無く、読み（ふりがな）からの検索もしない。「ほぞん」では「保存」は見つからない。
- アロケーション: `kit.Fuzzy` が問い合わせ・ラベル・表のバッファを持ち回し、大きくなるときだけ確保する。100 個のラベルを照らすベンチで allocs/op = 0（約 29 µs、i7-12700H）。問い合わせは 64 文字、ラベルは先頭 256 文字までを見る。

### 4. notes のサンプル

`examples/notes/palette.go` を `field.CommandPalette` に置き換えた。notes は有効なコマンドだけを渡し（以前と同じく無効なものは出さない）、最近のファイルとノートを `Extra`（説明は「最近のファイル」「ノート」）で足す。Ctrl+Shift+P とメニュー、Ctrl+K はこれまでどおりで、`openPalette` はパレットの内側の `BuildContext` から `OpenCommandPalette` を呼ぶ。サンプルの既存のテスト（絞り込み、Enter、Ctrl+K、メニューから開く）はそのまま通る。

変わった点:

- 検索が部分一致からあいまい一致になった（「縦」「俳句」は以前と同じ結果）。
- Checked のコマンドは「オン」の文字ではなくチェックマークで示す。
- 件数の行と「見つかりません」は `Strings` の言葉になった。notes はロケールを決めていないので、OS が英語なら "1 of 32" / "No results" と出る（以前は日本語の固定文で「↑↓ で選び Enter で実行」も添えていた）。

## 結果

- 登録済みの Action を探す薄い部品として、notes の 200 行が widgets の部品とアプリ側の 60 行ほどになった。
- 項目の提供者や非同期の検索、ページ、サブメニューは無い。`Extra` は値の一覧なので、毎回の Build で作り直すアプリ側の責任になる。
- ひらがな・カタカナの同一視や読みでの検索は、日本語の辞書（読み）を持たない以上できない。必要になれば `kit.Fuzzy` の畳み込みに足すだけで済む。
- 共有ファイル: `widgets/basic/strings.go` と `strings_tables.go` の末尾に `CommandPalette`（「コマンドパレット」）と `SearchCommands`（「コマンドを検索」）を足した。
