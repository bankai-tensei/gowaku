# よく使う形

[ガイドの目次](README.md) · [English](../en/recipes.md)

デスクトップアプリの多くがとる 4 つの形を、それぞれ完成したプログラムとして載せる。コピーして書き換えて使うとよい。最後の節には、つまずきやすい点をまとめた。どのプログラムもコンパイルでき、`gowakutest` のテストでスクリーンショットを撮れる（[はじめに](getting-started.md#ウィンドウを開かずに見る)を参照）。

## 一覧と詳細

上に検索欄、下にボタンを置いたメモのサイドバーと、選択中のメモを編集する入力欄。入力欄はウィンドウの残りいっぱいに広がる。

```go
package main

import (
	"log"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

type note struct {
	id   int
	body string
	ctl  field.InputController // one per note, so that a new note can take the focus
}

func (n *note) title() string {
	t, _, _ := strings.Cut(n.body, "\n")
	if t == "" {
		return "New note"
	}
	return t
}

// preview is the start of the text under the title.
func (n *note) preview() string {
	_, rest, _ := strings.Cut(n.body, "\n")
	return strings.ReplaceAll(rest, "\n", " ")
}

type notes struct{ gowaku.StatefulBase }

type notesState struct {
	gowaku.StateBase
	all      []*note
	selected int // index into all, -1 for none
	query    string
	next     int
}

func (notes) CreateState() gowaku.State {
	return &notesState{all: []*note{{id: 1, body: "Groceries\nmilk, eggs"}, {id: 2, body: "Ideas\na quiet app"}}, next: 3}
}

func (s *notesState) add() {
	s.SetState(func() {
		n := &note{id: s.next}
		s.next++
		s.all = append(s.all, n)
		s.selected = len(s.all) - 1
		s.query = ""
		n.ctl.Focus() // waits for the field that the next build creates
	})
}

func (s *notesState) remove() {
	s.SetState(func() {
		if s.selected < 0 {
			return
		}
		s.all = append(s.all[:s.selected], s.all[s.selected+1:]...)
		s.selected = min(s.selected, len(s.all)-1)
	})
}

func (s *notesState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	var rows []gowaku.Widget
	for i, n := range s.all {
		if s.query != "" && !strings.Contains(strings.ToLower(n.body), strings.ToLower(s.query)) {
			continue
		}
		rows = append(rows, gowaku.Keyed(n.id, collection.SidebarItem{
			Label: n.title(), Detail: n.preview(), Selected: i == s.selected,
			OnPress: func() { s.SetState(func() { s.selected = i }) },
		}))
	}
	sidebar := collection.Sidebar{
		Title:    "Notes",
		Header:   field.Search{Value: s.query, OnChange: func(q string) { s.SetState(func() { s.query = q }) }},
		Children: rows,
		Footer: basic.Row{Spacing: theme.Space8, Children: []gowaku.Widget{
			control.Button{Label: "Add", Style: control.ButtonPrimary, OnPress: s.add},
			control.Button{Label: "Delete", Style: control.ButtonQuiet, OnPress: s.remove},
		}},
	}
	var detail gowaku.Widget = basic.Center{Child: basic.Text{Content: "No note selected"}}
	if s.selected >= 0 {
		n := s.all[s.selected]
		// Keyed by the note, so that each note gets a field of its own: Input
		// keeps its text and follows Value only when Value changes.
		detail = gowaku.Keyed(n.id, field.Input{
			Label: "Note " + strconv.Itoa(n.id), Multiline: true, Value: n.body, Controller: &n.ctl,
			OnChange: func(v string) { s.SetState(func() { n.body = v }) },
		})
	}
	return basic.Row{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		sidebar,
		basic.Expanded{Child: basic.Padding{Insets: geom.All(theme.Space16), Child: detail}},
	}}
}

func main() {
	if err := gowaku.Run(gowaku.App{Title: "Notes", Width: 900, Height: 600, Home: notes{}}); err != nil {
		log.Fatal(err)
	}
}
```

ポイント:

- `collection.Sidebar` はパネル全体を受け持つ。背景と区切り線を自分で描き、行が収まらなければスクロールし、`Header` と `Footer` は固定したまま残す。`CrossStretch` の `Row` の先頭に置き、隣の内容は `Expanded` で包む。
- `SidebarItem.Detail` は行の 2 行目に小さく表示する文字で、ここではメモの書き出しを出している。1 行に収まらない分は省略記号で切られ、スクリーンリーダーには全文が 1 度だけ伝わる。
- 複数行の `field.Input` は、ラベルの有無にかかわらず与えられた高さいっぱいに広がる。ここでは `Padding` を包む `Expanded` がウィンドウの残りを渡している。
- `field.Input` は自分で文字列を保持する。`gowaku.Keyed(n.id, ...)` でメモごとに別の入力欄にしているので、メモを切り替えると正しい内容が表示される。
- 次のビルドで作られる入力欄にフォーカスを移すには、項目ごとに `InputController` を持ち、入力欄ができる前に `Focus` を呼ぶ。呼び出しは入力欄ができるまで待たれ、入力欄がダイアログの中にあっても、ダイアログ自身のフォーカスより優先される。

## 確認付きの設定画面

スイッチ、スライダー、ドロップダウン、セグメントをグループにまとめた設定画面と、実行前に確認するリセットボタン。

```go
package main

import (
	"log"
	"strconv"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

type prefs struct {
	notify  bool
	volume  float32
	quality string
	theme   int
}

var defaults = prefs{notify: true, volume: 50, quality: "medium"}

type settings struct{ gowaku.StatefulBase }

type settingsState struct {
	gowaku.StateBase
	p prefs
}

func (settings) CreateState() gowaku.State { return &settingsState{p: defaults} }

func (s *settingsState) set(f func(*prefs)) { s.SetState(func() { f(&s.p) }) }

func (s *settingsState) confirmReset() {
	overlay.ShowDialog(s.Context(), func(dismiss func()) gowaku.Widget {
		return overlay.Dialog{
			Title:   "Reset all settings?",
			Message: "Every setting goes back to its default.",
			Actions: []gowaku.Widget{
				control.Button{Label: "Cancel", OnPress: dismiss},
				control.Button{Label: "Reset", Style: control.ButtonDanger, OnPress: func() {
					s.set(func(p *prefs) { *p = defaults })
					dismiss()
				}},
			},
		}
	})
}

func (s *settingsState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	p := s.p
	return basic.Scroll{Child: basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space24, Children: []gowaku.Widget{
		collection.Group{Title: "General", Children: []gowaku.Widget{
			collection.GroupRow{Label: "Notifications", Child: control.Switch{Label: "Notifications", Value: p.notify,
				OnChange: func(v bool) { s.set(func(p *prefs) { p.notify = v }) }}},
			collection.GroupRow{Label: "Volume", Detail: strconv.Itoa(int(p.volume)), Child: basic.SizedBox{Width: 200,
				Child: control.Slider{Label: "Volume", Value: p.volume, Max: 100,
					OnChange: func(v float32) { s.set(func(p *prefs) { p.volume = v }) }}}},
			// GroupRow names the dropdown; a Label on it would show a caption above it.
			collection.GroupRow{Label: "Quality", Child: field.DropdownMenu[string]{Value: p.quality,
				Options: []field.DropdownOption[string]{{Value: "low", Label: "Low"}, {Value: "medium", Label: "Medium"}, {Value: "high", Label: "High"}},
				OnChange: func(v string) { s.set(func(p *prefs) { p.quality = v }) }}},
			collection.GroupRow{Label: "Theme", Child: control.SegmentedControl{Label: "Theme", Options: []string{"Light", "Dark", "Auto"}, Selected: p.theme,
				OnChange: func(i int) { s.set(func(p *prefs) { p.theme = i }) }}},
		}},
		control.Button{Label: "Reset", OnPress: s.confirmReset},
	}}}}
}

func main() {
	if err := gowaku.Run(gowaku.App{Title: "Settings", Width: 560, Height: 480, Home: settings{}}); err != nil {
		log.Fatal(err)
	}
}
```

ポイント:

- `collection.Group` と `GroupRow` で設定画面を組む。左にラベル、右にコントロールが並ぶ。行のラベルは、自身のラベルを持たないコントロールの名前にもなる。
- ページは、部分の間にセクション間の間隔 `theme.Space24` を置いた `Column` である。`basic.VStack` ではコントロール間の間隔 `theme.Space8` しか空かない。
- `overlay.ShowDialog` には State の `BuildContext`（`s.Context()`）を渡し、コールバックの中から呼ぶ。必要な `Overlay` は `gowaku.Run` と `gowakutest.New` が用意している。
- ダイアログが開いている間、ページはアクセシビリティのツリーから外れるので、テストでは `tt.Tap("Reset")` でダイアログのリセットボタンを押せる。[ダイアログをテストする](testing-and-preview.md#ダイアログをテストする)を参照。

## 長い縦書きの文章

右から左へ列を並べる縦組み。ルビは青空文庫の記法で書き、文字の大きさはスライダーで変える。スクロールは右端の最初の列から始まる。

```go
package main

import (
	"log"
	"strings"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

const book = `吾輩《わがはい》は猫である。名前はまだ無い。
どこで生れたかとんと見当《けんとう》がつかぬ。何でも薄暗いじめじめした所でニャーニャー泣いていた事だけは記憶している。`

type reader struct{ gowaku.StatefulBase }

type readerState struct {
	gowaku.StateBase
	size float32
}

func (reader) CreateState() gowaku.State { return &readerState{size: 20} }

func (s *readerState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	style := text.Style{Family: "Yu Mincho", Size: s.size, Orientation: text.Vertical, Align: text.AlignJustify, Indent: s.size} // one em
	// Vertical paragraphs run right to left: put them in a Row in reverse order.
	paras := strings.Split(book, "\n")
	cols := make([]gowaku.Widget, 0, len(paras))
	for i := len(paras) - 1; i >= 0; i-- {
		cols = append(cols, basic.RichText{Spans: aozora.Spans(paras[i], text.Style{}), Style: style})
	}
	return basic.Column{Children: []gowaku.Widget{
		basic.Padding{Insets: geom.All(theme.Space16), Child: control.Slider{Label: "Size", Value: s.size, Min: 14, Max: 36,
			OnChange: func(v float32) { s.SetState(func() { s.size = v }) }}},
		// Reverse: the scroll starts at the right end, at the first column, and
		// a short text sits against the right edge.
		basic.Expanded{Child: basic.Scroll{Horizontal: true, Reverse: true, Child: basic.Padding{Insets: geom.All(theme.Space24),
			Child: basic.Row{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: s.size, Children: cols}}}},
	}}
}

func main() {
	if err := gowaku.Run(gowaku.App{Title: "Reader", Width: 900, Height: 600, Home: reader{}}); err != nil {
		log.Fatal(err)
	}
}
```

ポイント:

- `text.Style{Orientation: text.Vertical}` で段落を縦組みにする。縦書きは右から左へ進むので、段落は逆順にして `Row` に入れる。
- `aozora.Spans` は `漢字《かんじ》` の記法を、`basic.RichText` 用のルビ付きスパンに変換する。ルビが行頭や行末からはみ出すことはない。
- `basic.Scroll{Horizontal: true, Reverse: true}` は右端から始まり、ウィンドウの大きさが変わっても右端を保つ。`Controller` に `basic.ScrollController` を渡せば位置を読み書きできるので、読みかけの位置から本を開き直せる。
- 指定したフォントがその環境に無ければ、システムの既定のフォントで表示する。OS ごとのフォントは[文字と日本語入力](text-and-ime.md)を参照。

## バックグラウンドで処理する

時間のかかる処理をゴルーチンで動かし、進み具合を表示して、終わったらトーストを出す。

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// worker runs the slow part. The app passes gowaku.Post and time.Sleep; a test
// passes functions it controls, because Post drops its function outside Run.
type worker struct {
	Post  func(func())
	Sleep func(time.Duration)
}

type exporter struct {
	gowaku.StatefulBase
	Work worker
}

type exporterState struct {
	gowaku.StateOf[exporter] // Widget() returns the exporter, with its Work
	running  bool
	progress float32 // 0 to 1
	runs     int
}

func (exporter) CreateState() gowaku.State { return &exporterState{} }

func (s *exporterState) start() {
	w := s.Widget().Work
	s.SetState(func() { s.running, s.progress = true, 0 })
	go func() { // never touch the State here: only through Post
		for i := 1; i <= 10; i++ {
			w.Sleep(300 * time.Millisecond)
			p := float32(i) / 10
			w.Post(func() {
				if s.Mounted() { // the window may have closed meanwhile
					s.SetState(func() { s.progress = p })
				}
			})
		}
		w.Post(func() {
			if !s.Mounted() {
				return
			}
			s.SetState(func() { s.running = false; s.runs++ })
			overlay.ShowToast(s.Context(), overlay.Toast{Message: fmt.Sprintf("Export %d finished", s.runs)})
		})
	}()
}

func (s *exporterState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	var onPress func()
	if !s.running {
		onPress = s.start // a Button without OnPress is disabled
	}
	return basic.Center{Child: basic.SizedBox{Width: 320, Child: basic.Column{
		Size: layout.MainMin, Spacing: theme.Space16, Children: []gowaku.Widget{
			control.ProgressBar{Label: "Progress", Value: s.progress},
			control.Button{Label: "Export", Style: control.ButtonPrimary, OnPress: onPress},
		}}}}
}

func main() {
	app := exporter{Work: worker{Post: gowaku.Post, Sleep: time.Sleep}}
	if err := gowaku.Run(gowaku.App{Title: "Export", Width: 480, Height: 320, Home: basic.Padding{Insets: geom.All(theme.Space24), Child: app}}); err != nil {
		log.Fatal(err)
	}
}
```

ポイント:

- `SetState` を呼べるのは UI のゴルーチンだけだ。ゴルーチンは変更を `gowaku.Post` に渡し、渡された関数は `Mounted()` を確かめる。処理の途中でウィンドウが閉じられているかもしれないからだ。
- `overlay.ShowToast` には、ダイアログと同じく State の `BuildContext`（`s.Context()`）を渡す。トーストはウィンドウの下中央に内容の上へ重ねて表示され、スクリーンリーダーが読み上げる。
- `gowaku.Post` は `Run` の外では関数を捨てるので、テストでは頼れない。`worker` のように予定の立て方を外から渡せば、テストから処理を 1 段ずつ進められる。

```go
func TestExport(t *testing.T) {
	steps := make(chan struct{})
	posts := make(chan func(), 1)
	w := worker{
		Sleep: func(time.Duration) { <-steps }, // each sleep waits for the test
		Post:  func(fn func()) { posts <- fn },  // posted work waits for the test too
	}
	tt := gowakutest.New(t, exporter{Work: w}, gowakutest.Options{})
	tt.Tap("Export")
	for i := 0; i < 10; i++ {
		steps <- struct{}{}
		(<-posts)() // run it on the test's goroutine, which is the UI goroutine of tt
	}
	(<-posts)() // the end: the toast
	tt.Settle()
	if n, _ := tt.Find("Progress"); n.Range.Value != 100 {
		t.Errorf("progress = %v", n.Range.Value)
	}
	if _, ok := tt.Find("Export 1 finished"); !ok {
		t.Errorf("no toast:\n%s", tt.Dump())
	}
}
```

トーストもほかの文字と同じく `Find` で見つかる。

## つまずきやすい点

- **入力欄は自分で文字列を保持する。** `field.Input` は、最初に表示されたときと、`Value` が変わったときにだけ `Value` を取り込む。そのため `OnChange` の中で `SetState` を使って `Value` を更新し続け、入力欄を空にするときは `Value` を `""` にする。同じ場所に別の項目を表示するには、項目ごとにキーを付ける：`gowaku.Keyed(id, field.Input{...})`。
- **キーを付けた入力欄には、項目ごとにコントローラーを用意する。** `InputController` は 1 つの入力欄に結び付く。入力欄に項目ごとのキーを付けているのにコントローラーを共有すると、`Focus` を呼んだ時点ではまだ、これから消える古い入力欄を指している。
- **入力欄の名前は Label、無ければ Placeholder になる。** スクリーンリーダーが読み上げるのも、`tt.Tap` や `tt.Find` が照合するのもこの名前だ。ラベルは全体が一致する必要がある。
- **ラベルは重ならないようにする。** 2 つのコントロールが同じラベルだと、`Find` と `Tap` は読み上げ順で先のほうを選ぶ。「Export」というラベルの進捗バーの隣に「Export」ボタンがあると、タップは進捗バーに当たる。
- **Row と Column は使える領域をすべて取る。** 子と同じ大きさにするには `Size: layout.MainMin` を、1 つの子に残りを渡すには `basic.Expanded` を使う。
- **Scroll には有限の大きさが要る。** `Column` の中では `Expanded` か `SizedBox` で大きさを与える。
- **画像ビューアでは矢印キーをアプリに残す。** フォーカスを持つ `interact.InteractiveViewer` は、動かせる余地があるときだけ矢印キーでパンし、それ以外は矢印キーを先へ渡す。`Keys: interact.ViewerKeysZoom` にすると常に先へ渡すので、左右キーで前後の画像を表示できる。`ViewerKeysNone` にするとフォーカスを受け取らない。`WheelZooms: true` にすると、修飾キーなしのホイールで拡大縮小する。中に置いた `interact.Listener` は押下を受け取るので、その上をドラッグしてもパンしない。パンさせるには、`OnPointer` で `PointerDown` の `e.Handled` を false に戻す。
- **ダイアログとトーストには State のコンテキストを渡す。** `overlay.ShowDialog(s.Context(), ...)` はコールバックの中で呼び、`Build` の中では呼ばない。
- **`gowaku.Post` は `Run` の外では何もしない。** テストでは予定の立て方を外から渡す（[バックグラウンドで処理する](#バックグラウンドで処理する)を参照）か、ティッカーと `tt.Advance` で時間を進める。
