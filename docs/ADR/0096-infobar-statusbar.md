# 0096: InfoBar と StatusBar

決定日: 2026-11-13

状態: 採用

## 背景

当初の計画に「小さく作れる」部品として `InfoBar`（ページ内に残る通知帯）と `StatusBar`（ウィンドウ下端の帯）がある。どちらも今は利用者が毎回 `Container` と `Row` で組んでいる。`Toast` は数秒で消えるので、「オフラインです」「保存できませんでした」のように状態が続くあいだ見せておく通知には使えない。notes のサンプルも下端の帯を自前で描いている。

## 選択肢

### InfoBar

- 表示の持ち方
  - A. 親が `Open` を持ち、閉じるボタンは `OnClose` で知らせる（制御された部品）。
  - B. 部品が自分で開閉の状態を持ち、`ShowInfoBar` のような命令で出す。
- 警告の色
  - A. `theme.Colors` に `Warning` を足す。
  - B. 部品の中で `theme.TonalPalette` から琥珀色を作る。
- 地の色
  - A. 重要度の色を薄く敷く。
  - B. 地は `Card` のまま、アイコンと枠線だけに色を付ける。

### StatusBar

- 狭いときのあふれ
  - A. 優先度の低い項目から丸ごと隠す。
  - B. 文字列を省略記号（…）で詰める。
  - C. Toolbar のように「…」メニューに逃がす。
- Semantics の役割: 専用の `RoleStatus` を足すか、既存の `RoleGroup` で名前を付けるか。

## 決定

### 1. InfoBar（`widgets/control`）

```go
type InfoSeverity uint8 // SeverityInfo, SeveritySuccess, SeverityWarning, SeverityError
type InfoBar struct {
	Open     bool
	Severity InfoSeverity
	Title, Message string
	Action   string
	OnAction func()
	OnClose  func()
}
```

- **制御された部品（A）**。ページの状態（オフラインか、保存に失敗したか）は親が持っているので、出すかどうかも親が決める。閉じるボタンは `OnClose` を呼ぶだけで、`Open` を下ろすのは親。ゼロ値は閉じた帯で、何も描かず場所も取らない。否定形の bool（`Hidden`）は規約で作らないので `Open` にした。
- 置き場所は `control`。ボタンを使い、メニューやオーバーレイは要らない。閉じるボタンにツールチップは付けない（`overlay` は上の段）。読み上げ名は `Strings.Close`。
- 中身: 重要度のアイコン（18）、太字の `Title`、その後に `Message`（`Body` / `Ink2`、幅に合わせて折り返す）、操作ボタンは **1 つまで**（`ButtonSmall`）、`OnClose` があれば × ボタン。操作ボタンは帯を閉じない。閉じたいなら `OnAction` の中で親が閉じる。
- **見た目（B, B）**: 地は `Card`、角は `Radius.Group`、枠線は重要度の色を 45% にしたもの。アイコンは色の付いた円（警告は三角）に、地の色で i・!・✓ を抜いたもの。色を面に敷くと、琥珀を薄めた地がベージュに見える（DESIGN の禁止事項）ので、色は線とアイコンだけにした。情報は `AccentInk`、成功は `Success`、エラーは `Danger`。警告は `theme.TonalPalette{Hue: 70, Chroma: 64}` のトーン 55（ダークは 78）。`theme.Colors` に `Warning` を足すと、同時に作業している他の部品とテーマの共有ファイルでぶつかるので、今は部品の中で作る。トークンに上げるのは後からでも追加だけで済む。
- **開閉の動き**: 開くと高さが 0 から伸び（下の中身を押し下げる）、同時にフェードイン。閉じるときは逆。`ReduceMotion` では高さは一度に変わり、フェードだけ残す（ADR 0042 の方針どおり）。最初から `Open` で作られた帯は動かずにそのまま出る。
- **読み上げ**: 帯全体が Live 領域。エラーは `LiveAssertive`、それ以外は `LivePolite`。ラベルは「重要度の語: タイトル 本文」（例: "エラー: 保存できませんでした ネットワークに…"）。重要度の語は `Strings` に足した（`Information`, `Success`, `Warning`, `Error`）。Toast と同じく、Live のノードは最初のフレームでは空のラベルで出し、次のフレームで言葉を入れる。読み上げソフトは「新しく現れた Live ノード」ではなく「Live ノードの変化」を読むからである。親が同じ帯で作り直してもラベルは変わらないので、二度は読まれない。閉じて開き直すとまた読まれる。言葉を変えれば、その変化が読まれる。

### 2. StatusBar（`widgets/nav`）

```go
type StatusItem struct {
	Text     string
	Child    core.Widget
	Label    string
	Tooltip  string
	OnPress  func()
	Priority int
}
type StatusBar struct {
	Label             string
	Leading, Trailing []StatusItem
}
```

- 置き場所は `nav`。項目にツールチップ（`overlay`）を付けるので `control` には置けない。`Toolbar` と同じ段。
- ウィンドウの枠には関わらない、普通のウィジェット。アプリがページの一番下に置く。
- 項目は文字（`Caption` / `Ink2`）、小さな自前のウィジェット（`Child`、`Label` があれば読み上げ名にする）、または `OnPress` で押せる項目。押せる項目はポインタが乗ると `Fill` の地が出て、Tab で止まり、Enter で押せる。`Tooltip` は `overlay.Tooltip` で包む。
- `Leading` は始端から、`Trailing` は終端から並べる。同じ組の隣り合う項目の間に、長さ 12 の細い線（`Hair`）を自動で引く。区切りを項目として書かせる Toolbar の形より、ステータスバーでは区切りが毎回同じなので自動にした。
- 寸法: 線の下に高さ 24、上端に 1 の `Divider`、地は `Sidebar`。両端の余白 8 は Toolbar に揃えた。Toolbar（38）より低いのは、文字が `Caption` だから。
- **あふれ（A）**: 全部が入らないときは、`Priority` の低い項目から **丸ごと** 1 つずつ隠し、残りが入るまで続ける。同じ優先度なら後ろのもの（`Leading` の後に `Trailing` を数えた順）から。隠した項目は描かず、ポインタも Tab も読み上げも素通りする（Toolbar と同じ `core.Hider`）。最後の 1 つ（一番優先度の高いもの）だけは隠さず、それでも入らなければ帯の端で切る。
  - B（省略記号）は採らなかった。`basic.Text` に省略の仕組みが無く、ステータスバーのためだけにテキストの切り詰めを足すのは範囲を超える。行番号やエンコーディングは途中で切れると意味が変わるので、丸ごと隠す方が誤解が少ない。
  - C（メニュー）は採らなかった。ステータスバーの項目は大半が表示だけで、メニューに入れても押す先が無い。VS Code も優先度で隠すだけである。
- **Semantics**: `RoleGroup` に `Label`（既定は `Strings.StatusBar` の「ステータスバー」）。`RoleStatus` は無く、足すと Windows UIA・AT-SPI・macOS の 3 つのブリッジに対応表を足すことになる。「対応している所では status の役割」は、今はどこも対応していないとして見送った。Live にもしない（行と列が動くたびに読み上げられてしまう）。
- **RTL**: 帯は左右を反転しない。gowaku は RTL のミラーリングを v2 の範囲外にしている（当初の計画）。項目の中のアラビア語やヘブライ語は普段どおり右から左に組まれ、あふれの規則も同じに働く。
- **ホットパス**: 行の Layout は幅の配列を使い回し、隠す項目は優先度を毎回なめて選ぶ（項目は多くても十数個）。幅を変えながらの再レイアウトでアロケーションが 0 であることをテストで確かめた。

### 3. 共有ファイルへの追加

`widgets/basic/strings.go` と `strings_tables.go` の末尾に、`Information`, `Success`, `Warning`, `Error`, `StatusBar`（と ADR 0097 の 2 語）を足した。英語と日本語の両方を埋め、`fillFrom` にも足した。

カタログ（`internal/catalog`）はすべての公開ウィジェットに項目を求めるので、`control.InfoBar`、`nav.StatusBar`、`field.CommandPalette` の項目を `registry.go` に、場面を新しい `scenes_bars.go` に足し、`go run ./internal/cmd/catalog -only ...` で 3 つの絵とページを作った。既存の絵は変わっていない。

## 結果

- ページ内の通知と下端の帯を、利用者が自前で組まなくてよくなった。カタログに `examples/gallery/pages_bars.go` を足した（登録は統合のときに行う）。
- 警告の色はテーマのトークンではない。アプリが `theme.Colors` を差し替えても警告の色は変わらない。必要になれば `Colors.Warning` を足し、部品はそちらを使うように変える（追加だけで済む）。
- 押せる項目は地の余白（6）を持つので、隣の区切り線との間が文字だけの項目よりわずかに広く見える。
- notes のサンプルの下端の帯はまだ自前のまま。置き換えは別の作業とした。
