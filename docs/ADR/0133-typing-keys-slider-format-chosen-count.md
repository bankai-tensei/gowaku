# 0133: 入力中のキーとショートカット、スライダーの値の読み方、選択中の行の件数

決定日: 2026-11-16

状態: 採用

## 背景

初めて使う人に gowaku を試してもらったところ、次の 3 つが見つかった。

1. **入力中に Space のショートカットが動く：** 音楽プレーヤーで、`interact.Actions` に Space だけの「再生／一時停止」を置いたところ、検索欄に空白を打つたびに再生が切り替わった。キーを押すと、プラットフォームはキーの押下（`DispatchKey`）と文字（`DispatchText`）を別々に渡す（`core/input.go`）。`field.RenderEditable.HandleKey` は、修飾なしの文字のキーを、文字が後から来るので使わずに `false` を返していた。押下はフォーカスの上の部品を順にたどった後、ショートカットに回り、Space の Action が動いた。`editor.Editor` も同じだった。テキスト欄を含むリストの行が、欄に打った Space で切り替わることも、同じ理由で起こりえた。
2. **スライダーの値が数字だけ：** `control.Slider` と `control.RangeSlider` は、スクリーンリーダーに値を小数第 2 位までの数字（「0.50」）で渡す。音量の「50%」、時間の「3 分」、「大」「中」「小」のような段階を伝える手段が無かった。
3. **選択中の行の件数が読みにくい：** `collection.SidebarItem` の `Count` は控えめな丸い枠（`control.Badge` の `Quiet`。`Fill` の上に `Ink2`）で描く。選択中の行では、その枠が `AccentTint` の上に乗り、コントラストはライトで 3.84:1、ダークで 4.00:1 しかなかった。文字は 4.5:1 という `theme.Colors` の約束を破っている。同じ理由で、選択中の行の `Detail` は直前に `Ink` に変えてあった。

## 選択肢

### 入力中のキー

- **A. テキスト欄が、文字を打つキーの押下を受け取ったことにする。** 修飾なしか Shift だけのとき、Space、英字、数字、記号のキー（`KeyCode` の 32〜126）は、文字が別に来るので押下だけを「使った」とする。IME の変換中は、修飾にかかわらず押下は変換のものとする。Ctrl や Alt のショートカットのうち、欄が使わないもの（Ctrl+S など）は今まで通り Actions に届く。ブラウザの input で、keydown が既定の動作（文字の入力）に使われるのと同じ考え方である。
- **B. Actions の側で、フォーカスがテキスト欄にあるときは修飾なしのショートカットを無視する。** Actions がフォーカスの種類を知る必要があり、テキスト欄の外（リストの行の Space など）では直らない。
- **C. 文字を打つキーでは、押下を送らず文字だけを送る。** プラットフォームの層を全部変える必要があり、押下で動くもの（リストの先頭文字での移動など）が壊れる。

### スライダーの値

- **A. `Format func(float32) string` を足す。** `control.NumberStepper` の `Format func(int) string`、`collection` のプロパティの `Format` と同じ名前と形で、パッケージの中で揃う。スライダーは数字を表示しないので、godoc に「読み上げにだけ使う」と書く。
- **B. `ValueText func(float32) string` とする。** ARIA の aria-valuetext に近い名前だが、同じパッケージの `NumberStepper.Format` と並べると別の概念に見える。
- **C. 単位の文字列（`Unit string`）と小数の桁数を足す。** 「大」「中」「小」のような段階は表せない。

### 選択中の行の件数

- **A. 選択中の行では、枠の中の件数を `Ink` にする。** `Detail` と同じ規則である。`Fill` を重ねた `AccentTint` の上の `Ink` は、どのアクセントでもライトとダークで 4.5:1 を超える。枠の色（`Fill`）はそのままで、控えめな見た目は変わらない。
- **B. 選択中の行では枠を `Accent` にし、白い文字にする。** 読めるが、選択中の行の中で件数だけが目立ち、「控えめな件数」という ADR 0127 の意図に反する。
- **C. `Ink2` を濃くする。** すべての補助の文字が濃くなり、`Ink` との差が無くなる（ADR 0041）。

## 決定

- 案 A。`input.KeyCode.Printable()` を足す（Space から `~` まで）。`widgets/internal/kit.TypingKey(e, composing)` が、文字を打つキーの押下（修飾なしか Shift だけ、または変換中なら修飾にかかわらず）かどうかを返す。`field.RenderEditable.HandleKey`（`field.Input`、`field.Search`、複数行の入力、`NumberField` などすべての入力欄）と `editor.Editor` の `HandleKey` は、それまでの処理で使わなかった押下のうち、これに当たるものを使ったとして `true` を返す。読み取り専用の `editor.Editor` は文字を受け取らないので、検索バーを開いた直後に打った文字が検索語になる間を除き、押下を使わない。Tab、Escape、Enter、矢印などは文字を打つキーではないので、今まで通りである。
- 案 A。`control.Slider.Format` と `control.RangeSlider.Format`（`func(float32) string`）を足す。セマンティクスの `Value` に入り、スクリーンリーダーはそれを値として読む。`Range.Value` には数値が入ったままである。nil なら今まで通り小数第 2 位まで。
- 案 A。`collection.SidebarItem` は、選択中の行の件数の枠を、`Ink2` を `Ink` に置き換えたテーマ（`basic.Theme`）の中で描く。`control.Badge` に色を指定するフィールドは足さない。

## 結果

- 入力欄にフォーカスがある間、Space や英字だけのショートカットは動かない。Ctrl+S のような欄が使わないショートカットは動く。テスト（`gowakutest`）は、Space、Shift+P、Ctrl+S の Action を置き、`field.Input`、`field.Search`、複数行の入力、`editor.Editor` のそれぞれで、打った文字が欄に入り、Space と Shift+P の Action が動かず、Ctrl+S が 1 度だけ動くことを確かめる。変換中は Space も Ctrl+S も動かないこと、読み取り専用のエディタやボタンでは Space と Shift+P が動くことも確かめる。
- AltGr（Windows では Ctrl+Alt として届く）や macOS の Option で打つ文字のキーは、Ctrl や Alt が付くので押下を使わない。それらに当たるショートカットを置くアプリはまれなので、そのままにした。ASCII の外の文字を打つキー（`KeyNone`）も同じである。
- データ表やツリーの先頭文字での移動（`HandleText`）は、テキスト欄ではないので変えていない。
- スライダーは「50%」のような言葉で値を伝えられる。テストは、`Format` のある `Slider` が「50%」を読み、右矢印の後に「55%」を読むこと、無い `Slider` が「0.25」を読むこと、`RangeSlider` の両方のつまみが `Format` で読まれることを確かめる。
- 選択中の行の件数は、どのアクセントでもライトとダークで 4.5:1 を超える。テストは、`gowakutest` の警告（コントラスト）が、8 つのアクセントのライトとダークで、選択中の行の件数と `Detail` について出ないことを確かめ、`theme` のテストは `Fill` を重ねた `AccentTint` の上で `Ink` が 4.5:1 を超えることを約束として固定する。カタログの Sidebar の例は、選択中の行を件数のある「Documents」に変え、絵を描き直した。
