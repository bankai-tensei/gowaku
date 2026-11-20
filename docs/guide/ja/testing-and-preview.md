# テストとプレビュー

[English](../en/testing-and-preview.md)

gowaku のツリーはウィンドウが無くても動く。`gowakutest` パッケージはテストの中でツリーをマウントし、CPU で描画し、偽の時計を進める。そのため UI のテストは、GPU もディスプレイも要らない普通の `go test` として書ける。`gowakupreview` パッケージと `gowaku-preview` コマンドは、テストファイルに書いた小さな関数を、ファイルを保存するたびに PNG として描画する。デバッグビルドでは、実行中のアプリに Inspector のウィンドウが加わる。

## gowakutest

`gowakutest.New` はウィジェットをマウントしてレイアウトする。テストでは、支援技術に伝わる名前で部品を探し、利用者と同じようにタップや入力を行い、その結果を確かめる。

```go
package app

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gowakutest"
)

func TestCounter(t *testing.T) {
	tt := gowakutest.New(t, counter{}, gowakutest.Options{})
	tt.Tap("Add one")
	if _, ok := tt.Find("count: 1"); !ok {
		t.Errorf("the count did not change:\n%s", tt.Dump())
	}
}
```

`Tester` のメソッド:

| メソッド | 戻り値 | 働き |
|---|---|---|
| `Find(label)`、`FindAll(label)` | `(*Node, bool)`、`[]*Node` | そのラベルを持つノードを返す。ボタン・入力欄・スライダーは `Label` で、文字は表示内容で探す。ラベルは完全に一致しなければならない。同じラベルが複数あるときは、操作できるもの（タップやフォーカスの操作を持つもの、入力欄）をただの文字より優先し、その次は読む順に選ぶ。入力欄が上に描く同じ名前の見出しではなく、入力欄そのものが見つかる |
| `FindRole(role, label)`、`FindAllRole(role, label)` | `(*Node, bool)`、`[]*Node` | その役割とラベルを持つノードを返す。同じラベルのノードを区別するときに使う |
| `FindFunc(match)`、`FindAllFunc(match)` | `(*Node, bool)`、`[]*Node` | `match(*Node) bool` が真になるノードを読む順に返す。すべてのボタンや、ラベルを問わずある役割のノードを探すときに使う |
| `Tap(label)`、`TapAt(pos)` | なし | ノードの上、または指定した座標で主ボタンを押して離す |
| `SecondaryTap(label)`、`SecondaryTapAt(pos)` | なし | 副ボタン（右ボタン）でクリックする。コンテキストメニューを開くときに使う |
| `MiddleTap(label)`、`MiddleTapAt(pos)` | なし | 中ボタン（ホイール）でクリックする |
| `DoubleTap(label)`、`DoubleTapAt(pos)` | なし | 主ボタンでダブルクリックする。`Tree` の行を開くときや、単語を選ぶときに使う |
| `Hover(label)`、`HoverAt(pos)`、`Unhover()` | なし | ボタンを押さずにポインターをノードの上や指定した座標へ動かす。`Unhover` はウィンドウの外へ出す |
| `Scroll(label, dx, dy, mods)`、`ScrollAt(pos, dx, dy, mods)` | なし | ホイールをノッチ単位で回す。符号は `core.PointerEvent.Scroll` と同じで、`dy` が正ならホイールを奥へ回し、リストの上の方へ進む。負なら下の方へ進む。`dx` は正が右。`input.ModControl` を付けるとズームになる |
| `Drag(label, by)`、`DragAt(from, to)` | なし | 押して、1 フレームに 1 歩ずつ動かして、離す。スライダーや分割線に使う |
| `Press(label)`、`PressAt(pos)`、`MoveTo(pos)`、`Release()` | なし | ドラッグを途中で止めておく。主ボタンを押したままにし、1 フレームに 1 歩ずつ動かし、離す。途中でスクリーンショットを撮ったり `Find` で調べたりできる |
| `DropFiles(label, paths...)`、`DropText(label, s)` とその `At` 形 | `input.DropEffect` | 他のプログラムからファイルや文字をドロップする。ドロップの結果を返す |
| `Type(s)` | なし | フォーカスのある部品に、キーボードまたは IME で確定した文字列として `s` を送る |
| `Key(k, mods)` | なし | フォーカスのある部品にキーの押下を送る。文字は入力しない。`Key(input.KeySpace, 0)` はボタンを押すが、入力欄に空白は入らない。空白を入れるには `Type(" ")` を使う |
| `Hold(mods)` | なし | 後に続くタップ、ドラッグ、ホバー、ドロップ、`Scroll`、`Key` の間、`Hold(0)` まで修飾キーを押したままにする。Shift クリック、Ctrl クリック、Ctrl ドラッグに使う |
| `Advance(d)` | なし | 時計を `d` だけ進め、その間のフレームを描画する |
| `Settle()` | なし | 何も動かなくなるまでフレームを描画する |
| `Pump()` | なし | 現在の時刻でフレームを 1 枚描画する |
| `Screenshot()`、`SaveScreenshot(path)` | `*image.NRGBA`、なし | フレームを CPU で描画する。`SaveScreenshot` は PNG で書き出す |
| `Warnings()` | `[]Warning` | フレームに潜んでいそうな問題を検査する。はみ出し（親の端を越えて配置された子を含む）、ラベルの無い部品、低いコントラストなど。ビューアーのズームなどの変形をたどり、実際に描かれた位置で調べる |
| `Semantics()`、`Dump()` | `*Node`、`string` | アクセシビリティのツリーを、値または文字列で返す。矩形は `Find` と同じくウィンドウの座標で、リストが表示域のすぐ外に作った行も含む |
| `Clipboard()`、`SetClipboard(s)`、`ClipboardContent()`、`SetClipboardContent(c)` | `string`、なし、`gowaku.ClipboardContent`、なし | Tester 専用のクリップボード。入力欄のコピーと貼り付け、`gowaku.WriteClipboard` と `ReadClipboard` はここを使う |
| `FileDialogs()`、`FileDialog()` | `[]*FileDialog`、`*FileDialog` | `gowaku.ShowFileDialog` が表示し、テストがまだ答えていないファイルダイアログ。`FileDialog` には `Options()`、`Choose(paths...)`、`Cancel()`、`Fail(err)` がある |
| `Trays()`、`Notifications()`、`ClickNotification()` | `[]*Tray`、`[]Notification`、なし | `gowaku.ShowTray` と `gowaku.Notify` が表示したもの。`Tray` には `Options()`、`Activate()`、`Choose(label)` がある |
| `Tree()` | `*core.Tree` | `core.Tree` を返す。Tester に無い操作を行うためのもの |

頼まれたことができないとき、たとえばタップするラベルが無いときや、スクリーンショットを書き出せないときは、テストを失敗させる。そのため、どのメソッドもエラーを返さない。ラベルで指す操作は、利用者が届かないノード、つまりウィンドウの外にあるもの、リストの外へスクロールした行のようにクリップで隠れたもの、上に描かれたものに覆われたものに対しても、テストを失敗させる。一部だけ見えるノードには、見えている部分で操作する。

クリップボードと同じく、トレイと通知とファイルダイアログも Tester 専用のものになる。`gowaku.ShowTray` と `gowaku.Notify` は `Trays()` と `Notifications()` に記録されるので、テストからメニューを読んだり、項目を選んだり、通知をクリックしたりできる。`gowaku.ShowFileDialog` が表示したダイアログは `FileDialogs()` に記録され、テストが答えるまで残る。`gowaku.SetTheme` は Tester のツリーのテーマを切り替え、`ThemeSystem` は `Options.Dark` に従う。

操作系のメソッド（タップ、ホバー、スクロール、ドラッグ、ドロップ、`Type`、`Key`）は操作の後で状態が落ち着くまで待つので、テストは遷移が終わった後の状態を読み取れる。`Tap` を 2 回続けても、位置が近くてもシングルクリック 2 回として扱う。ダブルクリックには `DoubleTap` を使う。誰からも見えないもの、たとえば `Opacity` が 0 のもの、`Navigator` の最前面より下にあるページ、折りたたまれたペインなどはツリーに含まれない。操作部品の中にあって、その部品と同じ内容を示す文字もツリーに含まれないので、`Find` は部品の方を返す。

### 入力欄に文字を入力する

入力欄をラベルでタップしてフォーカスを移してから、文字を入力する。

```go
func TestRename(t *testing.T) {
	tt := gowakutest.New(t, renameForm{}, gowakutest.Options{})
	tt.Tap("名前")
	tt.Type("山田")
	tt.Key(input.KeyEnter, 0)
	if _, ok := tt.Find("保存しました"); !ok {
		t.Error(tt.Dump())
	}
}
```

`Type` は、利用者が変換を確定したときの IME と同じように、確定済みの文字列を送る。確定前の変換中の状態は再現しない。

### コントロールが報告する値

`Find` は `gowakutest.Node`（`semantics.Node` と同じ型）を返す。`Role`、`Value`、`State`、`Range` はスクリーンリーダーが読み上げる内容そのものなので、テストではここからコントロールの状態を読み取る。

| ウィジェット | `Role` | `Value` | ほかに |
|---|---|---|---|
| `control.Switch` | `RoleSwitch` | `"on"` または `"off"` | オンのとき `State&semantics.Checked != 0` |
| `control.Checkbox`、切り替え式の `control.Chip` | `RoleCheckbox` | `"checked"` または `"unchecked"` | `State&semantics.Checked` |
| `control.Slider` | `RoleSlider` | `Format` が返す文字列。無ければ小数第 2 位までの値：`"50.00"` | `Range.Value` が数値、`Range.Min` と `Range.Max` が範囲 |
| `control.NumberStepper` | `RoleStepper` | 表示されている値 | `Range.Value` |
| `control.SegmentedControl`、`control.RadioGroup` | `RoleRadioGroup` | 選択中の選択肢のラベル | |
| `field.DropdownMenu` | `RoleComboBox` | 選択中の選択肢のラベル | 選ぶときは、これをタップしてから選択肢をラベルでタップする |
| `field.Input`、`field.Search` | `RoleTextField` | 入力されている文字列 | `Text` に文字列、選択範囲、行の情報が入る。タップはキャレットを置くだけで文字列を選択しないので、置き換えるときは `Type` の前に `Key(input.KeyA, input.ModControl)` を送る |
| `control.ProgressBar`、`control.ProgressRing` | `RoleProgress` | パーセント表記：`"40%"` | `Range.Value` は 0 から 100 |
| `collection.Table` | `RoleGrid`（名前は `Label`） | | 行は `RoleRow` の子で、名前はすべてのセルをつないだもの。選択中の行は `State&semantics.Selected`。セルは文字列を名前に持つ `RoleGridCell` なので、`Tap("budget.xlsx")` でそのセルをタップできる |
| `collection.Tree` | `RoleTree` | | 項目は `RoleTreeItem` で、`Level`、`State&semantics.Expanded`、`Selected` を持つ |
| `collection.SidebarItem` | `RoleListItem` | `Count`（あれば） | `Hint` に `Detail` が入る。`State&semantics.Selected` |

数値は文字列ではなく `Range.Value` で比べるとよい。迷ったら `tt.Dump()` を出力すれば、すべてのノードのロール、ラベル、値を確認できる。

### ダイアログをテストする

ダイアログやシートが開いている間、その下のページはアクセシビリティのツリーから外れる。スクリーンリーダーから見える状態と同じだ。そのため `Find` と `Tap` はダイアログの中だけを探すので、確認ボタンが、ダイアログを開いたボタンと同じラベルでも問題ない。

```go
func TestResetAsksFirst(t *testing.T) {
	tt := gowakutest.New(t, settings{}, gowakutest.Options{})
	tt.Tap("Notifications") // turn a switch off
	tt.Tap("Reset")         // the page's button opens the dialog
	tt.Tap("Reset")         // the dialog's button: the page is hidden now
	if n, _ := tt.Find("Notifications"); n.Value != "on" {
		t.Errorf("Reset did not restore the switch:\n%s", tt.Dump())
	}
}
```

`overlay.ShowDialog` が必要とする `Overlay` は、`gowakutest.New` も `gowaku.Run` も用意している。ダイアログはコールバックの中から、State の `BuildContext`（`s.Context()`）を渡して開く。メニュー、ドロップダウンの一覧などのポップアップはページを隠さない。開いたコントロールは、展開された状態としてツリーに残る。閉じたダイアログはタイマーで 120 ミリ秒ほどかけて消えていき、画面とオーバーレイから外れるのはそれが終わってからである。閉じるボタンの `Tap` はこの間も待つが、`After` やテストから直接呼んだコードで閉じたときや、`Options.Settle` が短いときは、消えたことを確かめる前に `tt.Advance(200 * time.Millisecond)` を呼ぶ。

### トーストと右クリックメニュー

トーストもほかの文字と同じくアクセシビリティのツリーに入るので、表示されている間は `Find("Saved")` で見つかる。`Duration` を過ぎるまで `Advance` すると閉じる。右クリックメニューは、メニューを持つ行やコントロールに `SecondaryTap` を送って開き、項目はラベルでタップする。

```go
tt.SecondaryTap("report.pdf") // the row of a Table with OnContextMenu
tt.Tap("Delete")              // an item of the menu
tt.Tap("Delete")              // the confirm button of the dialog it opened
```

### ホバー、スクロール、ドラッグ、ドロップ

ポインターのメソッドは、`Tap` と同じくラベルを受け取るか、`At` 形で座標を受け取る。

```go
tt.Hover("Now playing")                        // pauses a carousel while the pointer rests on it
tt.Unhover()                                   // the pointer leaves the window
tt.Scroll("Messages", 0, -3, 0)                // three notches down
tt.Scroll("Map", 0, 1, input.ModControl)       // Ctrl and the wheel away: zoom in
tt.Drag("Volume", geom.Offset{X: 60})          // the thumb of a slider, 60 px to the right
if tt.DropFiles("Attachments", "/tmp/report.pdf") == input.DropNone {
	t.Error("the drop was refused")
}
```

`interact.Draggable` は OS のドラッグを始めるが、`Tester` にはその受け手が無いので、ドラッグしても何も起きない。テストでは代わりに `DropText`、`DropFiles` またはその `At` 形で `interact.DragTarget` にドロップする。

2 つのノードが同じラベルを持つときは、`FindRole` で役割を指定して選び、`TapAt` で押す。

```go
tab, _ := tt.FindRole(semantics.RoleTab, "Settings") // not the heading of the page
tt.TapAt(tab.Rect.Center())
```

### 別の言語

テストの中の標準の部品は、OS にかかわらず英語で表示する。`Options.Locale` は、アプリの `App.Locale` と同じく言語を決める。これでカレンダーの月の名前が日本語になる。

```go
tt := gowakutest.New(t, field.Calendar{}, gowakutest.Options{Locale: language.Japanese})
```

`language` は `golang.org/x/text/language` である。`Options.Strings` は、アプリが `App.Strings` に渡す言葉の表を加える。

### 時間

時計はゼロから始まり、テストが進めない限り止まったままである。アニメーションと Ticker は、テストが指定した分だけ進む。

```go
tt := gowakutest.New(t, timer{}, gowakutest.Options{})
tt.Advance(3 * time.Second) // a periodic ticker fires three times
if _, ok := tt.Find("3 秒"); !ok {
	t.Error(tt.Dump())
}
```

`Advance` は、何かが動いている間は `gowakutest.FrameTime`（16ms）ごとに 1 枚、休止中の Ticker が起きるたびに 1 枚のフレームを描画する。`Settle` は、偽の時間で 1 分経ってもツリーが動き続けていればテストを失敗させる。

何を待つかの規則は次のとおりである。

- `Settle` と操作系のメソッドは、何かが動いている間だけフレームを描画し、何も動かなくなった時点で止まる。休止中の Ticker（周期的な Ticker、State の `After` の遅延、ツールチップの遅延）の時刻まで時計を進めることはない。
- `Advance(d)` は時計を `d` だけ進め、その間に時刻が来たものをすべて実行する。

したがって、`s.After(16*time.Millisecond, ...)` を始めるタップの後には `tt.Advance(16 * time.Millisecond)` が要る。タップ自身のアニメーションが動いている間に時刻が来たものは待つ間に実行されるが、テストはそれを当てにしてはならない。アニメーションが短くなることがあるからである。ツールチップも同じで、`Hover` の後に 0.5 秒の遅延を越えて `Advance` すると開く。

回り続けるスピナーのように止まらないツリーでは、`Options.Settle` を設定する（通常の遷移なら 500ms で足りる）。設定すると、タップ系のメソッド、`Type`、`Key`、`Settle` はその時間だけ時計を進めるようになる。

`gowaku.Post` は `Run` の外では関数を破棄し、`Tester` は `Run` を使っていない。そうしたコードは Ticker で動かすか、`Post` で実行されるはずだった処理を直接呼び出す。時間をおいて実行する処理は State の `After` で書く。`After` は Ticker と同じく `Advance` で進む（[基本の考え方](concepts.md#時間をおいて実行する)を参照）。また、`Tester` はそのテストのゴルーチン専用である。

### Options

| フィールド | 働き |
|---|---|
| `Width`、`Height` | ウィンドウのサイズ（論理ピクセル）。ゼロなら 800×600 |
| `Scale` | スクリーンショットにおける、論理ピクセルあたりの物理ピクセル数。ゼロなら 1 |
| `Theme`、`Dark` | 見た目。既定はライトなので、結果が OS の設定に左右されない |
| `Fonts` | 文字を組むためのフォントファイル。nil なら OS のフォントを使う。日本語にはこちらが必要 |
| `ReduceMotion` | 動きを減らす OS の設定の代わり |
| `Locale` | 言語と地域。`App.Locale` と同じく、標準の部品の言葉、月と曜日の名前、週の始まりを決める。ゼロなら OS にかかわらず英語 |
| `Strings` | アプリが加える言葉の表。`App.Strings` と同じ |
| `Settle` | 止まらないツリーで、操作系のメソッドが進める時間 |

### Warnings

`Warnings` は現在のフレームを検査し、見つかった問題を描画順に返す。

- `WarnOverflow`: 箱に収まらない中身。`Row` の端からはみ出した子や、箱より背の高い文字など。
- `WarnUnlabeled`: 支援技術に何も伝えないボタン、チェックボックス、スライダー、入力欄。
- `WarnContrast`: 背景とのコントラスト比が 4.5:1 未満（大きな文字では 3:1 未満）の文字。WCAG は無効な部品を除外しているので、無効にした部品の中の文字は調べない。
- `WarnShader`: `Fallback` の無いカスタムシェーダー。CPU のレンダラでは描画できない。

```go
for _, w := range tt.Warnings() {
	t.Error(w)
}
```

各 `Warning` には `Kind`、`Rect`、`Text`、`Detail`、`Amount`（はみ出したピクセル数またはコントラスト比）が入る。コントラストを測るためにスクリーンショットを 1 枚描画するので、その分の時間がかかる。

誰からも見えず、届かないためにアクセシビリティのツリーから外れているものは検査しない。`Opacity` が 0 のものと、開いているダイアログやシートの下のページである。ページは幕（スクリム）でわざと暗くしてあるからだ。ダイアログ自身と、その上に開いたものは検査する。`Scroll` の表示域の外へスクロールした行のように、祖先のクリップで丸ごと切り取られたものも検査せず、一部だけ切り取られた文字は見えている部分の背景で測る。

## golden 画像

`Screenshot` は GPU バックエンドと同じシェーダーの計算式で描画するので、ウィンドウに表示される画像と丸め誤差の範囲で一致する。保存したファイルとスクリーンショットを別のマシンでも比較できるようにするには、テストに固定のフォントを渡す。OS のフォントを使うと、インストールされているフォントによって文字の見た目が変わる。`golang.org/x/image` の Go フォントには和文のグリフが無いので、必要なグリフを含むフォントを選んで `testdata` に置く。

gowaku はアプリ向けの golden テストの補助機能を提供していないが、数行で書ける。

```go
var update = flag.Bool("update", false, "rewrite the golden files")

func TestCardGolden(t *testing.T) {
	tt := gowakutest.New(t, card{}, gowakutest.Options{Width: 320, Height: 200, Fonts: [][]byte{goregular.TTF}})
	const path = "testdata/card.png"
	if *update {
		tt.SaveScreenshot(path)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	got := tt.Screenshot()
	if !got.Bounds().Eq(want.Bounds()) {
		t.Fatalf("size %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := got.Rect.Min.Y; y < got.Rect.Max.Y; y++ {
		for x := got.Rect.Min.X; x < got.Rect.Max.X; x++ {
			if !near(got.At(x, y), want.At(x, y)) {
				t.Fatalf("pixel %d,%d differs; run with -update and look at the PNG", x, y)
			}
		}
	}
}

// near allows a difference of 2 per channel, for rounding.
func near(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	d := func(p, q uint32) bool { return p>>8 <= q>>8+2 && q>>8 <= p>>8+2 }
	return d(ar, br) && d(ag, bg) && d(ab, bb) && d(aa, ba)
}
```

`go test -update` でファイルを書き出し、PNG を目で確かめてからコミットする。

## プレビュー

プレビューは `_test.go` ファイルに書く関数で、名前が `Preview` で始まり、引数を受け取らず、ウィジェットか `gowakupreview.Item` を返す。テストファイルに置くので、パッケージ内の非公開の関数を呼び出せ、しかもバイナリには含まれない。

```go
package app

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakupreview"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

func PreviewAlbumTile() gowaku.Widget {
	return basic.SizedBox{Width: 180, Child: albumTile(demoAlbum)}
}

func PreviewSongs() gowakupreview.Item {
	return gowakupreview.Item{Widget: songs(demoList), Width: 900, Height: 600}
}
```

パッケージのディレクトリでコマンドを実行する。

```bash
go run github.com/bankai-tensei/gowaku/cmd/gowaku-preview .
```

各プレビューはライトとダークの両方で描画し、状態が落ち着いてから `Warnings` で検査する。コマンドは検査結果とファイルの場所を出力し、ビルドかプレビューが失敗した場合は終了コード 1 で終了する。出力されるのは、バリエーションごとの PNG、前回から見た目が変わったものの `.diff.png`、パッケージ全体の `report.json` と `report.txt` である。レポートには警告とアクセシビリティのツリーも含まれる。これらのファイルは、人間にもツール（AI エージェントを含む）にも読めるように作られている。

フラグ:

| フラグ | 働き |
|---|---|
| `-http localhost:7878` | 保存のたびに更新される画像のページを提供する（`-watch` を含む） |
| `-watch` | パッケージの `.go` ファイルが変わるたびに描画し直す |
| `-run regexp` | 名前が一致するプレビューだけを描画する |
| `-at 0,70ms,150ms` | 指定した時刻のフレームも描画する。アニメーションの途中を確認するためのもの |
| `-live Name` | 画像を描画する代わりに、その名前のプレビューをウィンドウで開く |
| `-out dir` | 画像とレポートの出力先。既定は一時ディレクトリの下の `gowaku-preview` |

`gowakupreview.Item` では、ウィンドウのサイズ（`Width`、`Height`、または幅によって形が変わるレイアウトのための複数の `Widths`）、`Scale`、`Themes`（`LightAndDark`、`LightOnly`、`DarkOnly`）、`Theme`、`At`、`Fonts` を指定する。`Width` か `Height` がゼロなら、その方向の長さは中身に合わせる（最大 1600×1200）。`theme.Set` を返す `PreviewTheme` という関数を定義すると、それがパッケージ内のすべてのプレビューのテーマになる。

コマンドはパッケージ内にファイルを書き込まない。プレビューを描画するテストは `go test -overlay` で追加する。独自の `TestMain` を持つパッケージでは、`gowakupreview.Active()` が真のときに `m.Run` を呼び出さなければならない。その場合、自前の準備処理は飛ばしてよい。

## Inspector

`-tags gowakudebug` を付けてビルドし、いずれかのウィンドウで F12 を押すか、`gowaku.OpenInspector()` を呼び出す。

```bash
go run -tags gowakudebug .
```

Inspector は同じプロセス内の別ウィンドウとして開き、他のウィンドウについて次の情報を表示する。

- Widget、Element、RenderObject のツリーと、ポインタの下にあるものを選択するピッカー。
- レイアウトの境界線と、再描画したレイヤーを一瞬色付けする表示。
- 段階別（build、layout、paint、batch、present）のフレーム時間のグラフ。GPU の処理時間とアロケーション数、遅いフレームで再ビルドされたものも確認できる。
- 描画コマンドの列（DisplayList）とアクセシビリティのツリー。

Inspector は表示するだけで、値を書き換えることはない。タグを付けずにビルドした場合、`OpenInspector` は何もせず、Inspector のコードはリンクされない。

## 関連

- [アクセシビリティ](accessibility.md): テストとスクリーンリーダーが共通して使うラベル
- [ADR 0040](../../ADR/0040-preview.md)（プレビュー）、[ADR 0068](../../ADR/0068-gowakutest-settle-option.md)（`Options.Settle`）、[ADR 0128](../../ADR/0128-tester-pointer-locale-settle-mono.md)（ポインターのメソッド、`Locale`、待つ規則）、[ADR 0132](../../ADR/0132-tester-clip-and-held-modifiers.md)（クリップされたものの警告、`Hold`）、[ADR 0073](../../ADR/0073-inspector.md)（Inspector）
