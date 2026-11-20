# Testing and previews

[日本語](../ja/testing-and-preview.md)

A gowaku tree runs without a window. Package `gowakutest` mounts it in a test, draws it on the CPU, and moves a fake clock, so UI tests are ordinary `go test` tests that need no GPU and no display. Package `gowakupreview` and the command `gowaku-preview` draw small functions of your test files to PNG files on every save. A debug build adds an Inspector window to a running application.

## gowakutest

`gowakutest.New` mounts a widget and lays it out. The test then finds widgets by what they say to assistive technology, taps and types like a user, and checks the result.

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

What a `Tester` does:

| Method | Returns | What it does |
|---|---|---|
| `Find(label)`, `FindAll(label)` | `(*Node, bool)`; `[]*Node` | The node with that label: a button, field or slider by its `Label`, a text by its content. The label must match whole. Of nodes that share it, one that can act (a tap or focus action, a text field) wins over plain text, so the caption a field draws above itself finds the field; then reading order |
| `FindRole(role, label)`, `FindAllRole(role, label)` | `(*Node, bool)`; `[]*Node` | The node of that role with that label, to tell apart nodes that share a label |
| `FindFunc(match)`, `FindAllFunc(match)` | `(*Node, bool)`; `[]*Node` | The nodes for which `match(*Node) bool` is true, in reading order: every button, a role with any label |
| `Tap(label)`, `TapAt(pos)` | — | Press and release the primary button over the node, or at a point |
| `SecondaryTap(label)`, `SecondaryTapAt(pos)` | — | Click the secondary (right) button, as to open a context menu |
| `MiddleTap(label)`, `MiddleTapAt(pos)` | — | Click the middle button (the wheel) |
| `DoubleTap(label)`, `DoubleTapAt(pos)` | — | Double-click the primary button, as to open a row of a `Tree` or select a word |
| `Hover(label)`, `HoverAt(pos)`, `Unhover()` | — | Move the pointer over the node or to a point without pressing, or out of the window |
| `Scroll(label, dx, dy, mods)`, `ScrollAt(pos, dx, dy, mods)` | — | Turn the wheel by notches, with the signs of `core.PointerEvent.Scroll`: positive `dy` is the wheel turned away from you, toward the top of a list, negative `dy` toward the bottom, positive `dx` to the right; with `input.ModControl`, a zoom |
| `Drag(label, by)`, `DragAt(from, to)` | — | Press, move in steps, one a frame, and release, as on a slider or a divider |
| `Press(label)`, `PressAt(pos)`, `MoveTo(pos)`, `Release()` | — | Hold a drag in progress: press the primary button and keep it down, move one step a frame, and let go, to take a screenshot or `Find` on the way |
| `DropFiles(label, paths...)`, `DropText(label, s)` and their `At` forms | `input.DropEffect` | Drop files or text from another program; they return what the drop did |
| `Type(s)` | — | Send `s` to the focused widget, as committed text from the keyboard or an IME |
| `Key(k, mods)` | — | Send a key press to the focused widget. It types nothing: `Key(input.KeySpace, 0)` presses a button, `Type(" ")` puts a space in a field |
| `Hold(mods)` | — | Keep modifier keys down for the taps, drags, hovers, drops, `Scroll` and `Key` that follow, until `Hold(0)`: a Shift-click, a Ctrl-click or a Ctrl-drag |
| `Advance(d)` | — | Move the clock by `d`, drawing the frames on the way |
| `Settle()` | — | Draw frames until nothing animates |
| `Pump()` | — | Run one frame at the current time |
| `Screenshot()`, `SaveScreenshot(path)` | `*image.NRGBA`; — | Draw the frame on the CPU; `SaveScreenshot` writes it as PNG |
| `Warnings()` | `[]Warning` | Check the frame for likely problems: overflow, including a child laid out past the edge of its parent, unlabeled controls and low contrast, among others. It looks where things are painted, through the zoom of a viewer and other transforms |
| `Semantics()`, `Dump()` | `*Node`; `string` | The accessibility tree, as a value or as text. Its rects, like those of `Find`, are in window coordinates, and it holds the rows a list builds just outside its view |
| `Clipboard()`, `SetClipboard(s)`, `ClipboardContent()`, `SetClipboardContent(c)` | `string`; —; `gowaku.ClipboardContent`; — | The Tester's own clipboard, which the copy and paste of fields and `gowaku.WriteClipboard` and `ReadClipboard` use |
| `FileDialogs()`, `FileDialog()` | `[]*FileDialog`; `*FileDialog` | The file dialogs that `gowaku.ShowFileDialog` showed and the test has not answered; a `FileDialog` has `Options()`, `Choose(paths...)`, `Cancel()` and `Fail(err)` |
| `Trays()`, `Notifications()`, `ClickNotification()` | `[]*Tray`; `[]Notification`; — | What `gowaku.ShowTray` and `gowaku.Notify` showed; a `Tray` has `Options()`, `Activate()` and `Choose(label)` |
| `Tree()` | `*core.Tree` | The `core.Tree`, for what the Tester does not offer |

A method that cannot do what it is asked, such as a tap on a label that is not there or a screenshot that cannot be written, fails the test, so none of them returns an error. An action on a label also fails when a user could not reach the node: outside the window, clipped away, as a row scrolled out of a list, or covered by something drawn above it; of a node that shows in part, it uses the part that shows. Like the clipboard, the tray, the notifications and the file dialogs are the Tester's own: `gowaku.ShowTray` and `gowaku.Notify` record into `Trays()` and `Notifications()`, where a test reads a menu, chooses an item or clicks a notification, and `gowaku.ShowFileDialog` waits in `FileDialogs()` for the test to answer. `gowaku.SetTheme` changes the theme of the tree; `ThemeSystem` follows `Options.Dark`.

The methods that act (the taps, hovers, scrolls, drags and drops, `Type` and `Key`) settle after they act, so a test reads the state after the transitions have ended. Two `Tap`s in a row are two single clicks, however near; use `DoubleTap` for a double click. What nobody can see is not in the tree: an `Opacity` of 0, the pages under the top of a `Navigator`, a collapsed pane. A text inside a control that says the same as the control is not in the tree either, so `Find` returns the control.

### Typing into a field

Tap the field by its label to focus it, then type:

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

`Type` sends committed text, as an IME does when the user confirms a conversion. The composition before that is not simulated.

### What a control reports

`Find` returns a `gowakutest.Node` (the same type as `semantics.Node`). Its `Role`, `Value`, `State` and `Range` are what a screen reader hears, so a test reads the state of a control from them:

| Widget | `Role` | `Value` | Also |
|---|---|---|---|
| `control.Switch` | `RoleSwitch` | `"on"` or `"off"` | `State&semantics.Checked != 0` when on |
| `control.Checkbox`, a toggling `control.Chip` | `RoleCheckbox` | `"checked"` or `"unchecked"` | `State&semantics.Checked` |
| `control.Slider` | `RoleSlider` | what `Format` spells, or the value with two decimals: `"50.00"` | `Range.Value` is the number, `Range.Min`, `Range.Max` the limits |
| `control.NumberStepper` | `RoleStepper` | the value as shown | `Range.Value` |
| `control.SegmentedControl`, `control.RadioGroup` | `RoleRadioGroup` | the label of the selected option | |
| `field.DropdownMenu` | `RoleComboBox` | the label of the selected option | tap it, then tap an option by its label to choose |
| `field.Input`, `field.Search` | `RoleTextField` | the text | `Text` holds the text, selection and lines; a tap puts the caret in and selects nothing, so to replace the text send `Key(input.KeyA, input.ModControl)` before `Type` |
| `control.ProgressBar`, `control.ProgressRing` | `RoleProgress` | a percentage: `"40%"` | `Range.Value` from 0 to 100 |
| `collection.Table` | `RoleGrid`, named by its `Label` | | its rows are `RoleRow` children, named by all their cells; a selected row has `State&semantics.Selected`; each cell is a `RoleGridCell` named by its text, so `Tap("budget.xlsx")` taps that cell |
| `collection.Tree` | `RoleTree` | | its items are `RoleTreeItem`s with `Level`, `State&semantics.Expanded` and `Selected` |
| `collection.SidebarItem` | `RoleListItem` | the `Count`, if any | `Hint` holds the `Detail`; `State&semantics.Selected` |

Compare numbers through `Range.Value` rather than the string. When unsure, print `tt.Dump()`: it shows every node with its role, label and value.

### Testing a dialog

While a dialog or a sheet is open, the page under it is out of the accessibility tree, as a screen reader sees it. `Find` and `Tap` then look only inside the dialog, so a confirm button may share its label with the button that opened it:

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

`gowakutest.New` and `gowaku.Run` both provide the `Overlay` that `overlay.ShowDialog` needs; open a dialog from a callback with the `BuildContext` of the State, `s.Context()`. Menus, dropdown lists and other popups do not hide the page: the control that opened them stays in the tree, marked expanded. A closed dialog fades out on a timer of about 120 ms, and leaves the screen and the overlay only when that ends; a tap that closes it settles through the fade, but where the dialog closes from an `After`, from code the test calls, or under a short `Options.Settle`, call `tt.Advance(200 * time.Millisecond)` before checking that it is gone.

### Toasts and context menus

A toast is in the accessibility tree like any other text, so `Find("Saved")` finds it while it shows; `Advance` past its `Duration` closes it. A context menu opens with `SecondaryTap` on the row or control that has it, and its items are tapped by their labels:

```go
tt.SecondaryTap("report.pdf") // the row of a Table with OnContextMenu
tt.Tap("Delete")              // an item of the menu
tt.Tap("Delete")              // the confirm button of the dialog it opened
```

### Hovering, scrolling, dragging and dropping

The pointer methods take a label, as `Tap` does, or a point with their `At` form:

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

`interact.Draggable` starts a drag through the OS, which a `Tester` has no host for, so dragging it does nothing; a test drops onto the `interact.DragTarget` with `DropText`, `DropFiles` or their `At` forms instead.

When two nodes share a label, `FindRole` picks one by its role, and `TapAt` presses it:

```go
tab, _ := tt.FindRole(semantics.RoleTab, "Settings") // not the heading of the page
tt.TapAt(tab.Rect.Center())
```

### Another language

The standard widgets speak English in a test, whatever the OS. `Options.Locale` sets the language, as `App.Locale` does in an application, so that a calendar shows Japanese month names:

```go
tt := gowakutest.New(t, field.Calendar{}, gowakutest.Options{Locale: language.Japanese})
```

`language` is `golang.org/x/text/language`. `Options.Strings` adds the tables of words the application gives `App.Strings`.

### Time

The clock starts at zero and stands still unless the test moves it. Animations and tickers run exactly as far as the test says:

```go
tt := gowakutest.New(t, timer{}, gowakutest.Options{})
tt.Advance(3 * time.Second) // a periodic ticker fires three times
if _, ok := tt.Find("3 秒"); !ok {
	t.Error(tt.Dump())
}
```

`Advance` draws one frame every `gowakutest.FrameTime` (16 ms) while something animates, and one at each wake of a sleeping ticker. `Settle` fails the test if the tree is still moving after a minute of fake time.

The rule for what settles:

- `Settle`, and the methods that act, draw frames only while something animates, and stop as soon as nothing does. They never move the clock ahead to a sleeping ticker: a periodic ticker, the delay of a state's `After`, or that of a tooltip.
- `Advance(d)` moves the clock by `d` and runs every wake that comes due on the way.

So a tap that starts `s.After(16*time.Millisecond, ...)` needs `tt.Advance(16 * time.Millisecond)` after it. A wake that comes due while a tap's own animation runs does fire during the settle, but a test must not count on that: the animation may get shorter. A tooltip likewise opens after `Hover` and an `Advance` past its delay of half a second. A tree that never comes to rest, such as a spinner that stays, sets `Options.Settle` (500 ms suits the usual transitions); the taps, `Type`, `Key` and `Settle` then move the clock by that much instead.

`gowaku.Post` drops its function outside `Run`, and a `Tester` is not `Run`. Drive such code with tickers instead, or call what `Post` would have run directly. A delay belongs in a state's `After`, which `Advance` drives like a ticker (see [Concepts](concepts.md#running-code-after-a-delay)). A `Tester` belongs to the goroutine of its test.

### Options

| Field | What it does |
|---|---|
| `Width`, `Height` | The window in logical pixels. Zero is 800 by 600 |
| `Scale` | Physical pixels per logical pixel in screenshots. Zero is 1 |
| `Theme`, `Dark` | The look. The light half is the default, so results do not depend on the OS |
| `Fonts` | Font files to set text in. Nil uses the system fonts, which Japanese text needs |
| `ReduceMotion` | Stands for the OS setting that asks for less animation |
| `Locale` | The language and region, as `App.Locale`: the words, month and day names and first day of the week of the standard widgets. Zero is English, whatever the OS |
| `Strings` | Tables of words the application adds, as `App.Strings` |
| `Settle` | How far the acting methods move the clock, for trees that never rest |

### Warnings

`Warnings` checks the current frame and returns what it found, in painting order:

- `WarnOverflow`: content that does not fit its box, such as children past the end of a `Row` or text taller than its box.
- `WarnUnlabeled`: a button, checkbox, slider or text field that tells assistive technology nothing.
- `WarnContrast`: text below 4.5:1 against what is behind it, or 3:1 for large text. The text of a disabled control is not checked, as WCAG exempts inactive components.
- `WarnShader`: a custom shader without a `Fallback`, which the CPU renderer cannot draw.

```go
for _, w := range tt.Warnings() {
	t.Error(w)
}
```

Each `Warning` has its `Kind`, `Rect`, `Text`, `Detail` and `Amount` (the overflow in pixels, or the contrast ratio). It draws a screenshot to measure contrast, so it costs about as much as one.

What is out of the accessibility tree because nobody can see or reach it is not checked: an `Opacity` of 0, and the page under an open dialog or sheet, which the scrim dims on purpose. The dialog, and what opens over it, are checked. Nor is what an ancestor clips away whole, such as a row scrolled out of the viewport of a `Scroll`, and text cut in part is measured against what shows of its background.

## Golden images

`Screenshot` draws with the same shader code as the GPU backends, so the picture matches a window to within rounding. To compare screenshots with stored files across machines, give the test fixed fonts: with the system fonts, text depends on what is installed. The Go fonts in `golang.org/x/image` have no Japanese glyphs, so pick a font with the glyphs you need and keep it in `testdata`.

gowaku has no golden helper of its own for applications; a few lines do it:

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

Run `go test -update` to write the files, look at the PNGs, and commit them.

## Previews

A preview is a function in a `_test.go` file whose name begins with `Preview`, with no parameters, returning a widget or a `gowakupreview.Item`. Being in a test file, it reaches the package's unexported functions and stays out of the binary.

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

Then run the command in the package directory:

```bash
go run github.com/bankai-tensei/gowaku/cmd/gowaku-preview .
```

Each preview is drawn light and dark, settled, and checked with `Warnings`. The command prints a report and where the files are, and exits with status 1 if the build or a preview failed. It writes one PNG per variant, a `.diff.png` where the picture changed since the last run, and `report.json` and `report.txt` for the package, which also hold the warnings and the accessibility tree. The files are meant to be read by people and by tools, AI agents included.

Flags:

| Flag | What it does |
|---|---|
| `-http localhost:7878` | Serve a page of the pictures that updates itself on every save (implies `-watch`) |
| `-watch` | Draw again whenever a `.go` file of the packages changes |
| `-run regexp` | Draw only the previews whose name matches |
| `-at 0,70ms,150ms` | Also draw the frames at these times, to see an animation part way |
| `-live Name` | Open the preview of this name in a window instead of drawing |
| `-out dir` | Where the pictures and reports go. The default is `gowaku-preview` in the temporary directory |

`gowakupreview.Item` sets the window (`Width`, `Height`, or several `Widths` for layouts that adapt), `Scale`, `Themes` (`LightAndDark`, `LightOnly`, `DarkOnly`), `Theme`, `At` and `Fonts`. A zero `Width` or `Height` leaves that length to the content, up to 1600 by 1200. A function named `PreviewTheme` returning a `theme.Set` gives the theme of every preview of the package.

The command writes nothing into your packages: it adds the test that draws the previews through `go test -overlay`. A package with its own `TestMain` must call `m.Run` when `gowakupreview.Active()` reports true, and may skip its own setup then.

## The Inspector

Build with `-tags gowakudebug` and press F12 in any window, or call `gowaku.OpenInspector()`:

```bash
go run -tags gowakudebug .
```

The Inspector is a separate window in the same process. It shows, for the other windows:

- the Widget, Element and RenderObject trees, and a picker that selects what is under the pointer;
- layout bounds, and the layers that were painted again, tinted for a moment;
- a graph of frame times by stage (build, layout, paint, batch, present), with GPU time and allocations, and what was rebuilt in a slow frame;
- the display list and the accessibility tree.

It only reads; it does not change values. Without the tag, `OpenInspector` does nothing and no code of the Inspector is linked in.

## See also

- [Accessibility](accessibility.md): labels, which the tests and the screen readers use alike
- [ADR 0040](../../ADR/0040-preview.md) (previews), [ADR 0068](../../ADR/0068-gowakutest-settle-option.md) (`Options.Settle`), [ADR 0128](../../ADR/0128-tester-pointer-locale-settle-mono.md) (pointer methods, `Locale`, the settle rule), [ADR 0132](../../ADR/0132-tester-clip-and-held-modifiers.md) (clipped warnings, `Hold`), [ADR 0073](../../ADR/0073-inspector.md) (Inspector), in Japanese
