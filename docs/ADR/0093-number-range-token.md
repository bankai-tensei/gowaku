# 0093: NumberField、RangeSlider、TokenField

決定日: 2026-11-13

状態: 採用

## 背景

当初の計画の「小さく作れる」部品のうち 3 つを足す。どれも今は利用者が自作している。

- `NumberField`: `Stepper` は整数を ± ボタンで変えるだけで、`PropertyNumber` は `PropertyGrid` の中にしか無い。数値を直接打ち、キーとドラッグで増減でき、書式がロケールに従う欄が要る。
- `RangeSlider`: `Slider` はつまみが 1 個。価格帯や期間のような範囲を選べない。
- `TokenField`: タグや宛先の入力。確定した文字列が `Chip` になる。IME の確定と Enter の区別、Backspace で前のトークンを消す動きは、利用者が組むと必ず間違えるので部品が持つ。

どれも追加だけで、当初の計画の互換方針に収まる。既存の公開 API と振る舞いは変えない。

## 決定

### 1. NumberField（`widgets/field`）

```go
type NumberField struct {
	Value, Min, Max float64
	Step            float64 // 0 は 1
	Precision       int     // 0 は Step の桁数、負は整数
	Grouping        bool
	Label, Placeholder string
	OnChange        func(float64)
}
```

- **値は float64。** Slider（float32）ではなく PropertyNumber に揃えた。金額や座標を打つ欄なので、float32 の 7 桁では足りない。
- **Precision のゼロ値は Step の桁数。** Step 0.25 なら 2 桁、Step 1（ゼロ値）なら整数。ゼロ値の `NumberField{}` は整数の欄になる。値は確定のたびにこの桁に丸める。
- **確定は Enter とフォーカスを失うとき。** 打っている途中の文字は値ではない（PropertyNumber と同じ）。
  - 数でない文字は、**打っている間は枠を赤くし**（Semantics は Invalid、ヒントは `Strings.InvalidNumber`「数値を入力してください」）、**確定すると元の値に戻す**。空にして確定しても戻す。値の無い状態を作らないため。エラーのまま残す方式は、フォームの外で使うと抜け出せない欄になるので採らなかった。
  - 範囲外の数は Min / Max に丸めて確定する（Stepper と同じく Max > Min のときだけ上限がある）。
  - Escape は打った文字を捨てて値に戻す。IME の変換中の Enter と Escape は IME のもの。
- **書式はロケールから。** 小数点と桁区切りは、`basic.FormatDecimal(ctx, 1234567.5, 1)` の出力から読み取る（`1,234,567.5` / `1.234.567,5` / `1 234 567,5`）。x/text の CLDR の表をそのまま使い、表を二重に持たない。ロケールが変わったときだけ読み直す。
  - 書くときの数字は ASCII。アラビア数字の地域でも ASCII で書く（読むときはアラビア・インド数字も受け付ける）。
  - `Grouping` は既定で切る。年やコードの欄で `2,026` と出るのを避ける。Stepper（FormatInt で区切る）とは既定が逆だが、Stepper は表示だけで、こちらは編集する欄なので区切りがあると打ち直しにくい。
- **読むときは寛容に。** 次を受け付ける。
  - 全角の数字・記号（日本語 IME の全角入力）、和文の空白、`ー`（日本語 IME で `-` キーを押すと出る長音符）を負号として。
  - その地域の桁区切りが空白なら、どの空白も（U+0020、U+00A0、U+202F、U+2009）。スイスのアポストロフィも。
  - 地域の小数点が無く、もう一方の記号（`.` か `,`）が 1 つだけなら、それを小数点とみなす。ドイツ語で `1.5` は 1.5、英語で `1,5` は 1.5。
  - 桁区切りは 3 桁ごとでなければならない。区切りが 1 つだけで後ろが 3 桁でないとき、それが `.` か `,` なら小数点とみなす。ドイツ語で `1.500` は 1500、`1.5` は 1.5。
  - 指数（`1e5`）、途中の符号、小数点 2 つは数ではない。
- **増減**:
  - ↑↓ で Step、Shift と一緒で 10 倍、Ctrl と一緒で 1/10（ただし Precision より細かくはしない）。PageUp / PageDown で 10 倍。Home / End はキャレットの移動に残した（欄の中のキーを奪わない）。
  - ↑↓ は単一行の Input が使わないので、Input の外側の `Focus.OnKey` で受ける。Input には手を入れていない。
  - 打った途中の文字が数なら、そこから増減する。
  - **ホイールはフォーカスがあるときだけ。** ページをスクロールしている途中で欄の上を通っても値が変わらないように。端数のノッチ（タッチパッド）はためて 1 段ずつ進める。
  - **ドラッグは右端のつまみ（◂▸）で。** 欄の本体のドラッグは文字の選択に使うので、ぶつけない。4px で 1 段、右が増。Shift / Ctrl の倍率はキーと同じ。押すと欄にフォーカスを移し、打っていた文字は先に確定する。
- **Semantics** はスピンボタン（`RoleStepper`）。Range、Increase / Decrease / SetValue を持ち、中に Input のテキスト欄がある（Autocomplete のコンボボックスと同じ入れ子）。
- **制御された部品。** 親が OnChange で受けた値を返さずに作り直したら、親の値を表示し直す。このため、表示の文字列を差し替えたときに `RenderEditable.value` も合わせている（同じパッケージの内部。Input の約束「Value が前回と違えば従う」を、コードで入れた文字にも当てはめる）。
- 無効（OnChange なし）は地の色の箱に淡い文字だけを出す。Input に無効の状態が無いため。
- **PropertyNumber とは共有しない。** PropertyNumber は `strconv.ParseFloat` で読み、`'g'` で書く。ロケールの書式に替えると、PropertyGrid の表示と読み取りが変わる。振る舞いを保つ共有はできないので、そのままにした。PropertyNumber を NumberField に置き換えるかは別に決める。

### 2. RangeSlider（`widgets/control`、描画は `layout.RenderRangeSlider`）

```go
type RangeSlider struct {
	Low, High, Min, Max float32
	Step   float32 // 0 は範囲の 1/20
	Snap   bool
	MinGap float32
	RTL    bool
	Label  string
	OnChange func(low, high float32)
}
```

- **Slider と同じ見た目。** `layout.RenderSlider` と同じパッケージに `RenderRangeSlider` を置き、トラック・つまみの寸法、影、フォーカスリングの太さ、`circle` を共有した。テーマのトークン（Accent、FillStrong、Hair、LiftKnob、ホバーで太る量と押して縮む量）も Slider の定数をそのまま使う。既存の `RenderSlider` は触っていない（つまみの描画 8 行は写した）。
- **つまみは交差しない。** 入れ替える方式は、キーボードで動かしているつまみが途中で別の値になり、スクリーンリーダーの読み上げも入れ替わるので採らなかった。ドラッグもキーも、相手から MinGap 手前で止まる。端の近くで MinGap を守れないとき（Min=0, Low=0.05, High=0.1, MinGap=0.2 など）は、動かしている側が Min / Max で止まり、相手は押さない。値は常に [Min, Max] に収まり、間隔は相手が離れるまで MinGap より狭いまま（2026-11-13、レビューでドラッグが −0.1 を返していたのを直したときに、キーと同じこの決まりに揃えた）。
  - 2 つのつまみが同じ場所にあるときは、押した後に動いた向きで決める（上へなら High、下へなら Low）。動く前は Low を持つ。
- **間のバーをドラッグすると範囲ごと動く。** 幅を保ったまま、端で止まる。範囲の外を押すと、近い方のつまみがそこへ来る（Slider と同じ）。
- **キー**: つまみごとに Tab の止まり場所がある（Low が先）。←→↑↓ で Step、PageUp / PageDown で 10 段、Home / End は動ける端まで。押したつまみにフォーカスが移る。
- **RTL は `RTL bool` で明示する。** gowaku にはまだ方向の Inherited が無い（当初の計画で全体の RTL ミラーリングは対象外）。ロケールから推すと、アラビア語の UI で左から右の数直線を出したい場合に困る。RTL では Min が右端に来て、←→ は画面の向きに動く（→ で値が下がる）。↑ と PageUp はいつも上げる。
- **Semantics** はグループ（Label）の中に、つまみごとのスライダー（`Strings.RangeMinimum` / `RangeMaximum`「最小」「最大」）。各つまみの Range は動ける範囲（Low は Min から High − MinGap まで）。
- **ドラッグはアロケーションしない。** ドラッグ中はつまみの位置を render object が自分で動かし（レイアウトで子のフォーカス枠も動かす）、値が変わったときだけ OnChange を呼ぶ。部品は作り直さない。`TestRangeSlider_DragDoesNotAllocate` と `BenchmarkRangeSlider_Drag` で、移動・レイアウト・描画の 1 フレームが 0 allocs/op であることを確かめた。
  - ただし `core.Tree.DispatchPointer` は配るイベントを毎回ヒープに写す（`deliver` の `local := *e` が interface のメソッドに渡って逃げる）。これはどのポインタ対象でも同じで、Slider も 1 イベント 1 回アロケーションする。core の問題なのでここでは直していない。
  - 親が OnChange のたびに作り直せば、その Build の分はアロケーションする（Slider と同じ）。
  - 親の値がいつも勝つ。作り直しのたびに render object の位置は Low / High に合わせ直す。

### 3. TokenField（`widgets/field`）

```go
type TokenField struct {
	Tokens          []string
	Label, Placeholder string
	Separators      string // 空は「, ， 、」
	Validate        func(token string) string
	AllowDuplicates bool
	MaxTokens       int
	OnChange        func(tokens []string)
}
```

- **確定**: Enter、区切り文字（既定は `,`、全角 `，`、和文の `、`。日本語 IME はカンマキーを `、` で確定する）、フォーカスを失うとき。前後の空白は削る。
- **IME の確定と Enter の区別。** 変換中の Enter は IME のもので、トークンにしない。
  - 各プラットフォームは IME が使ったキーをアプリに送らない（Windows の VK_PROCESSKEY、macOS と Wayland も同じ。ADR 0008、0022、0082）。確定した文字は `EventText` で届き、それだけではトークンにならない。
  - それでも送ってくる環境に備え、変換中（`RenderEditable.Composition != nil`）は Enter・Backspace・←・区切りの処理をすべて IME に任せる。Autocomplete（ADR 0056）と同じ考え方。
  - 変換中の文字列は OnChange に来ないので、変換中の「、」で区切られることもない。確定して初めて区切る。
- **貼り付け**: 区切り文字と改行で分けて、まとめて 1 回の OnChange にする。最後の区切りの後ろは打ちかけとして欄に残す。単一行の Input は改行を空白にしてしまうので、Ctrl+V だけはトークン側で受けて、改行を区切り文字に替えてから入れる。
- **Backspace は 2 段階。** 文字の先頭で Backspace を押すと直前のトークンを選び、もう一度押すと消す。メールの宛先欄と同じ。いきなり消すと、文字を消しているつもりで宛先を消してしまう。
  - 文字の先頭の ← も直前のトークンを選ぶ。選んだ後は ←→ でトークンを移り、最後のトークンの先の → で文字に戻る。Home は先頭のトークン、End と Escape は文字に戻る。
  - 選んだトークンは Backspace / Delete で消える。文字を打つと選択は外れる。
  - キーボードは最後まで文字の欄に置く。トークンごとに Tab の止まり場所は作らない（Tab で欄を抜けられるように）。チップの × は control.Chip のまま、ポインタとスクリーンリーダー用に残る。
  - 選んだトークンはフォーカスリングと同じ輪で示し、Semantics の Selected を立てる。
- **キーを文字の欄より先に見る口。** Backspace と ← は Input が自分で使い切る（先頭では何も起きないが、true を返す）。部品側で先に見るため、`Extras` に非公開の `onKey` を 1 つ足し、`RenderEditable.HandleKey` の最初で呼ぶようにした（textfield.go への変更はこの 2 か所、計 6 行）。公開 API は変わらない。Input を写した独自の render object を作る案は、枠・フォーム・ルビの処理を二重に持つことになるので採らなかった。
- **重複**: 既定では、既にあるトークンは足さずに文字を消す（エラーにしない）。`AllowDuplicates` で足す。比較は完全一致。
- **検証**: `Validate` が理由を返したトークンは文字の欄に残し、理由を欄の下に赤で出し、枠も赤くする（Semantics は Invalid とヒント）。打ち直すと消える。複数を貼り付けたときは、通らなかったものから後ろを欄に残す。FormItem の Validators と同じ `func(T) string` の形にした。
- **MaxTokens**: 上限に達した後に打った文字は欄に残る。打つこと自体は止めない。
- **折り返し**: チップと文字の欄を `Wrap` に並べる。文字の欄は中身（無ければ placeholder）の幅に合わせて広がり、入らなければ次の行に回る。
- **作り直しで選択がずれない。** 選んだトークンは文字列でも覚え、新しい Tokens の中で探し直す。無くなっていれば選択を外す。チップの状態は（文字列、同じ文字列の何番目か）をキーにして、前のトークンが消えても引き継ぐ。
- **Semantics**: リスト（Label）の中に、トークンごとの項目（Selected）と、その後ろのテキスト欄。
- **OnChange にはいつも新しいスライス**を渡す。親の Tokens は書き換えない。
- **候補（Autocomplete）は今回入れない。** Autocomplete は自分で Input を作るので、TokenField が必要とする `Bare`、`Controller`、`onKey` を渡す口が無い。Autocomplete 側に手を入れないと組めず、安くないので見送った。必要になったら、候補の一覧（`overlay.PopupList`）を TokenField が直接持つ形で足す。

### 4. 共有したファイル

- `widgets/basic/strings.go` / `strings_tables.go`: `InvalidNumber`、`RangeMinimum`、`RangeMaximum` を足した（英語と日本語、`fillFrom`）。追加だけ。
- `widgets/field/textfield.go`: 上記の `Extras.onKey`。
- 新しいファイル: `layout/rangeslider.go`、`widgets/control/rangeslider.go`、`widgets/field/numberfield.go`、`numberformat.go`、`tokenfield.go`、各テスト、`examples/gallery/pages_inputs.go`（未登録。ビルドが通ることを確かめるテストを添えた）。

## 結果

- 3 部品とも GPU 無しのテストで、ポインタ・キー・IME の合成・Semantics・ロケール（ja / de / fr / en）を確かめた。
- RangeSlider のドラッグ 1 フレームは 0 allocs/op。
- 残したこと:
  - NumberField の本体のドラッグ（つまみだけにした）、単位の表示（`Format` / 接尾辞）、右寄せ。
  - RangeSlider の目盛り表示。
  - TokenField の候補、チップの並べ替え、チップのダブルクリックでの編集。
  - 全体の方向（Directionality）。RangeSlider の `RTL` は、それができたら既定をそこから取れるようにする。

## 追記（2026-11-13）

- NumberField: 打っている途中に親が別の `Value` を渡したら、打った文字を捨てて新しい値を出す（検証のエラーも消す）。`Value` が同じままの作り直しでは打った文字を残す。以前は打った文字が残り、Enter で古い値が親に返っていた。
