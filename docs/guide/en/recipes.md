# Recipes

[Guide index](README.md) · [日本語](../ja/recipes.md)

Four shapes that most desktop applications take, each as a complete program. Copy one and change it. The last section lists the mistakes that are easy to make. Every program here compiles, and a `gowakutest` test can take its screenshot (see [Getting started](getting-started.md#see-it-without-a-window)).

## A list and a detail

A sidebar of notes with a search field above it and buttons below it, and the selected note in an editor that fills the rest of the window.

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

What makes it work:

- `collection.Sidebar` is a whole panel: it draws its background and divider, scrolls its rows when they do not fit, and keeps `Header` and `Footer` in place. Put it first in a `Row` with `CrossStretch`, and the content in an `Expanded`.
- `SidebarItem.Detail` is the smaller second line of a row, here the start of the note. It is cut to one line with an ellipsis, and a screen reader hears it whole, once.
- A multiline `field.Input` fills the height it is given, label or not; the `Expanded` around the `Padding` gives it the rest of the window.
- `field.Input` keeps its own text. `gowaku.Keyed(n.id, ...)` gives each note its own field, so switching notes shows the right text.
- To focus a field that the next build creates, keep an `InputController` per item and call `Focus` before the field exists: it waits for the field, and wins over the focus of a dialog the field opens in.

## A settings window with a confirmation

Grouped rows of switches, a slider, a dropdown and a segmented control, and a Reset button that asks first.

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

What makes it work:

- `collection.Group` and `GroupRow` lay out a settings page: a label on the left, the control on the right. The row's label also names a control that has none of its own.
- The page is a `Column` with `theme.Space24` between its parts, the gap between sections. `basic.VStack` would put only `theme.Space8` between them, the gap between controls.
- `overlay.ShowDialog` takes the `BuildContext` of the State, `s.Context()`; call it from a callback. `gowaku.Run` and `gowakutest.New` already provide the `Overlay` it needs.
- While the dialog is open, the page is out of the accessibility tree, so a test can press the dialog's Reset with `tt.Tap("Reset")`: see [Testing a dialog](testing-and-preview.md#testing-a-dialog).

## A long vertical text

Japanese set in columns from right to left, with ruby written in Aozora Bunko notation, a size slider, and a scroll that starts at the first column on the right.

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

What makes it work:

- `text.Style{Orientation: text.Vertical}` sets a paragraph in columns; the paragraphs go into a `Row` in reverse order, because vertical text runs right to left.
- `aozora.Spans` turns `漢字《かんじ》` notation into spans with ruby for `basic.RichText`. Ruby never goes past the head or end of a line.
- `basic.Scroll{Horizontal: true, Reverse: true}` starts at the right end and keeps it in place when the window is resized. A `basic.ScrollController` in `Controller` reads and sets the position, to reopen a book where the reader left it.
- The font family falls back to the system default when the machine lacks it; see [Text and Japanese input](text-and-ime.md) for fonts per OS.

## Work in the background

A slow job on a goroutine that reports its progress, and a toast when it is done.

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

What makes it work:

- Only the UI goroutine may call `SetState`. The goroutine hands each change to `gowaku.Post`, and the posted function checks `Mounted()`, because the window may have closed while the job ran.
- `overlay.ShowToast` takes the State's `BuildContext`, `s.Context()`, like a dialog. A toast sits at the bottom center of the window, over the content, and is announced by screen readers.
- `gowaku.Post` drops its function outside `Run`, so a test cannot rely on it. Passing the scheduling in, as `worker` does, lets the test step the job by hand:

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

`Find` finds a toast like any other text.

## Pitfalls

- **A field keeps its own text.** `field.Input` takes `Value` when it first appears and again only when `Value` changes. So keep `Value` in step: set it from `OnChange` inside `SetState`, and clear the field by setting `Value` to `""`. To show a different item in the same place, key the field by the item: `gowaku.Keyed(id, field.Input{...})`.
- **One controller per keyed field.** An `InputController` is attached to one field. If the field is keyed by item, keep a controller per item; a shared controller still points at the field that is about to go away when you call `Focus`.
- **A field is named by its Label, or else its Placeholder.** That name is what a screen reader says and what `tt.Tap` and `tt.Find` match. Labels match whole.
- **Labels should differ.** When two controls say the same, `Find` and `Tap` take the first in reading order: a progress bar labelled "Export" next to an "Export" button takes the tap.
- **Rows and columns take all the room they may.** Use `Size: layout.MainMin` to make a `Row` or `Column` as small as its children, and `basic.Expanded` to give one child the rest.
- **A Scroll needs bounded lengths.** Inside a `Column`, give it an `Expanded` or a `SizedBox`.
- **An image viewer leaves the arrows to the app.** `interact.InteractiveViewer` with the focus pans with the arrow keys while there is room to pan, and passes them on otherwise. `Keys: interact.ViewerKeysZoom` always passes them on, so that Left and Right can show the previous and next image; `ViewerKeysNone` keeps the viewer out of the focus. `WheelZooms: true` zooms with the plain wheel. An `interact.Listener` inside it takes the presses, so dragging there does not pan unless its `OnPointer` clears `e.Handled` on the `PointerDown`.
- **Dialogs and toasts take the State's context.** Call `overlay.ShowDialog(s.Context(), ...)` from a callback, not during `Build`.
- **`gowaku.Post` does nothing outside `Run`.** In tests, pass the scheduling in (see [Work in the background](#work-in-the-background)) or drive time with a ticker and `tt.Advance`.
