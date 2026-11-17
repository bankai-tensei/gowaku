# Text and Japanese input

[日本語](../ja/text-and-ime.md)

gowaku sets text itself: it shapes with HarfBuzz (through go-text), breaks lines by the rules of the W3C "Requirements for Japanese Text Layout" (JLReq), and draws the glyphs on the GPU. The same rules apply on every OS, so a paragraph breaks at the same places on Windows, Linux and macOS when the fonts are the same.

This guide covers the widgets that show text, the fonts, the Japanese typesetting rules and how to switch them, vertical text, and text input with an IME.

## Showing text

`basic.Text` shows a string. `basic.RichText` shows spans of different looks in one paragraph, with ruby. Both take a `text.Style`.

```go
package main

import (
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

func main() {
	body := text.Style{Size: 18}
	page := basic.Pad(24, basic.Column{Spacing: 12, Children: []gowaku.Widget{
		basic.Text{Content: "吾輩は猫である。名前はまだ無い。", Style: body},
		basic.RichText{Style: body, Spans: []text.Span{
			{Text: "吾輩", Ruby: "わがはい"},
			{Text: "は"},
			{Text: "猫", Ruby: "ねこ"},
			{Text: "である。"},
			{Text: "名前", Style: body.WithWeight(700)},
			{Text: "はまだ無い。"},
		}},
	}})
	err := gowaku.Run(gowaku.App{Title: "text", Width: 480, Height: 200, Home: page})
	if err != nil {
		log.Fatal(err)
	}
}
```

The zero `text.Style` is regular black text of 14 logical pixels, set by the JLReq defaults. Zero fields of a span's `Style` take the paragraph's value. The theme has ready styles for each role (`th.Text.Body`, `th.Text.Title`, `th.Text.LargeTitle` and so on, from `basic.ThemeOf(ctx)`); the `With` methods (`WithSize`, `WithWeight`, `WithColor`, `WithAlign`, `WithMaxLines`) return a copy with one field changed.

Text is not selectable by default. Wrap it in `field.SelectionArea` to let the user select and copy across paragraphs; ruby copies as its base text.

### Cutting text short

`Style.MaxLines` limits a text to that many lines and ends the last one with an ellipsis "…": a one-line preview in a list row, a two-line description on a card. Do not cut strings by guessing their width.

```go
package app

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// mailRow shows the subject on one line and two lines of the body.
func mailRow(ctx gowaku.BuildContext, subject, body string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Column{Size: layout.MainMin, Spacing: theme.Space4, Children: []gowaku.Widget{
		basic.Text{Content: subject, Style: th.Text.Headline.WithMaxLines(1)},
		basic.Text{Content: body, Style: th.Text.Body.WithColor(th.Color.Ink2).WithMaxLines(2)},
	}}
}
```

- The lines before the last break as usual. The last line is filled up to the width, cut at grapheme clusters, so an emoji or a letter with its accents never splits. With `MaxLines: 1`, text that does not fit is cut at the width.
- The ellipsis always fits inside the width: characters come off the end until it does. Spaces and opening brackets such as 「 do not stay in front of it.
- The ellipsis comes at the logical end of the text: on the right in a left-to-right paragraph, on the left in a right-to-left one. In vertical text it is the vertical ellipsis, at the foot of the last column.
- What a screen reader reads, and `gowakutest`'s `Find`, stay the whole text. `Layout.Truncated` reports whether anything was cut, for a tooltip with the rest.

The main fields of `text.Style`:

| Field | What it does |
|---|---|
| `Family` | A font family tried before the default Japanese families, or several separated by commas, tried in order as in CSS: `"Cascadia Mono, Menlo, monospace"`. Families the system lacks are skipped |
| `Size`, `Weight`, `Italic` | Size in logical pixels (zero is 14), CSS weight (zero is 400), italic |
| `LineHeight` | Distance between baselines as a multiple of `Size` (zero is 1.5). Fixed, so a fallback font cannot make lines uneven |
| `Color` | Glyph color. The zero value is opaque black |
| `Align` | `AlignStart`, `AlignCenter`, `AlignEnd`, `AlignJustify` |
| `Direction` | Base direction for BiDi; zero follows the first strong character |
| `Orientation` | `text.Horizontal` or `text.Vertical` |
| `MaxLines` | The most lines (columns in vertical text) to show; zero is unlimited. Text cut short ends with "…" |
| `Indent`, `HangingIndent`, `TabStop` | First-line indent, indent of the other lines, tab stops |
| `LineBreak`, `Kinsoku`, `Punctuation`, `WesternSpacing`, `Hanging` | The JLReq rules, below |
| `Hyphenate` | Break English words with a hyphen |
| `Variations` | Axes of a variable font |
| `RubyRoom` | Keep room for ruby even where no span has ruby |

## Fonts

gowaku ships no fonts. `gowaku.Run` reads the fonts installed on the system, on a goroutine of its own while the window and the GPU device are created. The index of the fonts is cached in the user cache directory, so later starts are faster.

For each piece of text, the families are tried in this order:

1. The families of `Style.Family`, if set, in order.
2. The default Japanese families: Yu Gothic (Windows), Noto Sans CJK JP (Linux), Hiragino Sans (macOS), Meiryo, then the system's sans-serif.
3. For text that shows as emoji: Segoe UI Emoji, Noto Color Emoji and Apple Color Emoji come first.

Yu Gothic UI is left out on purpose: its kana and punctuation are proportional, which leaves nothing for the punctuation rules to work with. On Linux, install Noto Sans CJK JP (in Debian and Ubuntu, the `fonts-noto-cjk` package) for Japanese text to look right.

Color emoji are drawn from COLRv0, COLRv1, CBDT and sbix fonts. Variable fonts take their axes from `Style.Variations`; `Weight` already sets `wght` and `Size` sets `opsz` on fonts that have them. A face without an italic, as Japanese fonts are, is slanted by about 12° when `Italic` is set, except in vertical lines, for color emoji and for right-to-left scripts.

There is no way to hand `gowaku.Run` font files of your own. Fonts given as bytes are for tests and previews (`gowakutest.Options.Fonts`, `gowakupreview.Item.Fonts`), where fixed fonts make screenshots match on every machine, and for `text.NewShaper` when you use the `text` package on its own.

## Japanese typesetting

The defaults follow JLReq. Each rule has a field in `text.Style` to change it. `examples/jptext` sets each sentence twice, with a rule on and off, side by side:

```bash
go run ./examples/jptext
```

### Line start and line end prohibitions (kinsoku)

Closing brackets, small kana, the prolonged sound mark, commas and full stops do not start a line. Opening brackets do not end one. Dashes and ellipses stay together. `LineBreak` sets how many characters the line start rule covers, like the `strict`, `normal` and `loose` of CSS `line-break`:

| `jlreq.Strictness` | Characters that may start a line |
|---|---|
| `jlreq.Strict` (default) | None of the prohibited ones |
| `jlreq.Normal` | Small kana and the prolonged sound mark |
| `jlreq.Loose` | Also iteration marks, hyphens and wave dashes |

`Kinsoku` says how a prohibited character is kept off the line start:

- `jlreq.Oidashi` (default) pushes the end of the line to the next line.
- `jlreq.Oikomi` squeezes the line to take the character in, and falls back to `Oidashi` if the line has too little to give.

### Punctuation compression

Full-width brackets and punctuation have a blank half. When two of them meet, as in `」「` or `。」`, `jlreq.PunctuationSqueeze` (default) removes the blanks that JLReq 3.1.5 removes. `jlreq.PunctuationFull` keeps every mark on its full em box. The rule applies per glyph, and only where the glyph is on a full em box, so a proportional font does not lose space it does not have.

### Space between Japanese and Western text

`jlreq.WesternQuarterEm` (default) puts a quarter em between Japanese and Latin letters or digits, as JLReq 3.2.5 describes. `jlreq.WesternNone` puts nothing. The space is spacing, not a character: it is not in the text, it is not copied, and a screen reader does not hear it.

### Hanging punctuation

With `Hanging: true`, a comma or full stop that falls just past the end of a line hangs outside the measure instead of moving to the next line with the character before it. It is off by default.

### Justification and indents

`Align: text.AlignJustify` stretches every line of a block but the last to the full width, by JLReq 3.8: Western word spaces open up first, then the gaps between Japanese characters. A Japanese paragraph usually starts one em in, which `Indent` does:

```go
st := text.Style{
	Size:    16,
	Indent:  16, // one em
	Align:   text.AlignJustify,
	Kinsoku: jlreq.Oikomi,
	Hanging: true,
}
```

`HangingIndent` is the indent of the other lines. `TabStop` sets tab stops in logical pixels; zero sets a tab as a space.

### Ruby

Give a span a `Ruby` and the ruby is set above it, or to its right in vertical text. A span with ruby stays on one line as a group. Ruby longer than its base overhangs neighboring kana where it may, but never past the head or the end of a line: there it lines up with its base. For mono ruby, one reading per character, make one span per base character:

```go
spans := []text.Span{
	{Text: "東", Ruby: "とう"},
	{Text: "京", Ruby: "きょう"},
	{Text: "の"},
	{Text: "日本語組版処理", Ruby: "にほんごくみはんしょり"}, // group ruby
}
```

Text with ruby has a line height of at least `text.RubyLineHeight` (1.75) times the size, and room above its first line. Set `RubyRoom` on text whose ruby comes and goes, so that its lines do not jump.

A span with `Emphasis` gets emphasis dots (傍点) beside each character, on the side of ruby, such as the sesame of `text.EmphasisSesame`. `text/aozora` reads them from notes such as `対象［＃「対象」に傍点］`.

Package `text/aozora` reads the ruby notation of Aozora Bunko, `｜漢字《かんじ》` or `漢字《かんじ》`. `aozora.Spans` turns such text into spans for `basic.RichText`, with the given style on every span (the zero style takes the paragraph's). `aozora.Marks` finds the marks, `aozora.Plain` strips them. A text field can show such ruby while the text keeps the notation; see [Ruby in a text field](#ruby-in-a-text-field).

```go
const opening = "吾輩《わがはい》は猫《ねこ》である。｜名前《なまえ》はまだ無い。"

page := basic.RichText{Spans: aozora.Spans(opening, text.Style{}), Style: text.Style{Size: 18}}
```

### Hyphenation

`Hyphenate: true` lets English words break across lines with a hyphen, by Liang's algorithm and the American English patterns of hyph-utf8. The hyphen is drawn at the line end and is not part of the text. Other languages are not built in: package `text/hyphen` can parse other TeX patterns (`hyphen.Parse`), but a paragraph cannot take them yet.

### Bidirectional text

Arabic and Hebrew mixed with other scripts are ordered by UAX #9. `Direction` is the base direction of each block (a block ends at a line feed):

- `text.DirectionAuto` (default) takes the direction of the first strong character, so an Arabic paragraph runs right to left by itself.
- `text.DirectionLTR` and `text.DirectionRTL` force one.

Carets, hit tests and selection follow the visual order on screen. The layout of widgets is not mirrored for right-to-left languages: a `Row` still runs left to right. Vertical text is always in logical order.

## Vertical text (tategaki)

Set `Orientation: text.Vertical`. Lines become columns, set top to bottom and stacked right to left. Japanese stands upright, Western text turns 90° clockwise, and punctuation takes its vertical form through the `vert` and `vrt2` features of the font. In vertical text, the width of a `Layout` is the height of a column, and `LineHeight` is the distance between the centers of columns.

Two span fields only matter in vertical text:

- `TateChuYoko` sets two or three digits horizontally within one em of the column (JLReq 3.4.3). Longer text shrinks to fit.
- `Upright` stands Western text upright, one letter below the other, as for an abbreviation.

```go
vertical := text.Style{Size: 18, Orientation: text.Vertical, Family: "Yu Mincho"}
colophon := basic.RichText{Style: vertical, Spans: []text.Span{
	{Text: "明治"},
	{Text: "39", TateChuYoko: true},
	{Text: "年、雑誌「ホトトギス」に発表。"},
	{Text: "GPU", Upright: true},
	{Text: "で縦書きを描く。"},
}}
```

Ruby goes to the right of the column. Justification, indents and the JLReq rules work as in horizontal text. A paragraph is one widget; to set several paragraphs side by side, put them in a `basic.Row` in reverse order, because vertical text starts on the right.

A long vertical text scrolls sideways in a `basic.Scroll` with `Horizontal` and `Reverse`. `Reverse` starts the scroll range at the right end: the text opens at its first column, a text narrower than the box sits against the right edge, and when the window is resized the right end stays where it is. The wheel and the scroll bar move the text the same way as in any horizontal Scroll. The Scroll takes all the room it is given, so give it a height, here with a `SizedBox`:

```go
paragraphs := []string{"親譲りの無鉄砲で小供の時から損ばかりしている。", "小学校に居る時分学校の二階から飛び降りて一週間ほど腰を抜かした事がある。"}
style := text.Style{Size: 18, Orientation: text.Vertical, Indent: 18}
columns := make([]gowaku.Widget, 0, len(paragraphs))
for i := len(paragraphs) - 1; i >= 0; i-- {
	columns = append(columns, basic.Text{Content: paragraphs[i], Style: style})
}
page := basic.SizedBox{Height: 560, Child: basic.Scroll{
	Horizontal: true, Reverse: true,
	Child: basic.Row{Size: layout.MainMin, Spacing: 18, Cross: layout.CrossStart, Children: columns},
}}
```

To move the view from code, as to open a book where the reader left it, put a `basic.ScrollController` in `Scroll.Controller`. Its offset counts from the start of the scroll range, the right end here: `JumpTo(0)` goes back to the first column, `Offset` reads where the view is, and `MaxOffset` how far it can go.

`examples/tategaki` sets the opening of *Botchan* this way, next to the same text set horizontally, with a vertical text field:

```bash
go run ./examples/tategaki
```

## Text input

`field.Input` edits text: selection, the clipboard, undo, IME composition, ruby and vertical text. It keeps its own text, so the zero value works. Set `Value` to replace the text from outside, and use `OnChange` to learn about edits. Text the IME is still converting is not an edit: `OnChange` runs once the IME commits it.

```go
package main

import (
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

type form struct{ gowaku.StatefulBase }

type formState struct {
	gowaku.StateBase
	name string
}

func (form) CreateState() gowaku.State { return &formState{} }

func (s *formState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.Pad(24, basic.Column{Spacing: 12, Children: []gowaku.Widget{
		field.Input{
			Label:       "名前",
			Placeholder: "山田花子",
			Value:       s.name,
			OnChange:    func(v string) { s.SetState(func() { s.name = v }) },
		},
		basic.Text{Content: "こんにちは、" + s.name + "さん"},
	}})
}

func main() {
	if err := gowaku.Run(gowaku.App{Title: "input", Home: form{}}); err != nil {
		log.Fatal(err)
	}
}
```

Other fields of `field.Input`:

- `Multiline` lets Enter start a new line and wraps long lines. In a single-line field, Enter runs `OnSubmit`, or, inside a `field.FormItem`, submits the `field.Form` around it (see [Forms](#forms)); Shift+Enter does the same, as in browsers.
- `SubmitOnEnter` turns a multiline field into the box of a chat: Enter runs `OnSubmit` (or submits the form), and Shift+Enter starts a new line. Enter that confirms an IME conversion only confirms it, and text the IME commits keeps its line breaks.
- `Autofocus` gives the field the keyboard when it first appears, as for the field of a dialog. It wins over the dialog's own focus.
- `Obscure` makes a password field. It draws a bullet per character and keeps the IME off, so that no input method composes or learns the text. It also asks the window for secure input while it has the keyboard. Copy and cut do nothing, undo keeps no history, and a screen reader hears a password field.
- `Controller` takes an `*field.InputController`, which reads and sets the selection, edits as the user would (`Insert`, `Replace`, `Undo`, `Copy`, `Paste`) and gives the field the keyboard (`Focus`). Offsets are bytes of the text. `Focus` and `SelectAll` may come before the field exists: the field takes them when it is built, also inside a dialog.
- `Bare` leaves out the box, for a field that is the page.

A chat box, and a Rename dialog whose field opens with its text selected:

```go
field.Input{
	Label: "Message", Multiline: true, SubmitOnEnter: true,
	Value:    s.draft,
	OnChange: func(v string) { s.SetState(func() { s.draft = v }) },
	OnSubmit: func(v string) { s.send(v); s.SetState(func() { s.draft = "" }) },
}
```

```go
func (s *filesState) rename(ctx gowaku.BuildContext) {
	s.name.SelectAll() // s.name is a field.InputController in the State
	s.name.Focus()
	overlay.ShowDialog(ctx, func(dismiss func()) gowaku.Widget {
		return overlay.Dialog{Title: "Rename", Content: field.Input{
			Value: s.file, Controller: &s.name,
			OnChange: func(v string) { s.SetState(func() { s.file = v }) },
			OnSubmit: func(string) { dismiss() },
		}}
	})
}
```

For long documents, use `editor.Editor` of `widgets/editor`. It shapes only the lines in view, so a document of 100,000 lines scrolls and takes typing without delay, and it has IME composition, find and replace, line numbers and decorations. It sets text horizontally and has no ruby: for vertical text or ruby while editing, use a multiline `field.Input`.

### Forms

`field.Form` checks and submits the `field.FormItem`s below it together. A `FormItem` holds one control and its value; it checks the value when the control loses the keyboard and on submit, shows the error below the control, and turns the border of the control red. Every field control behaves the same in a `FormItem`: `Input`, `Search`, `NumberField`, `Autocomplete`, `TokenField`, `DropdownMenu`, `DatePicker`, `TimePicker` and `ColorPicker`. A control of several parts, such as a `TimePicker`, or one that opens a popup, such as a `DatePicker`, counts as one field: moving between its parts or into its calendar is not leaving it.

The error of `Required` follows the control: 入力してください (Required) for a text field, 選択してください (Choose one) for a choice (`DropdownMenu`, `DatePicker`, `TimePicker`, `ColorPicker`, `RadioGroup`, `SegmentedControl`, a `Chip` that toggles), and チェックしてください (Check this box) for a `Checkbox` or another control of a bool. `RequiredMessage` replaces it.

Enter in a single-line field of a `FormItem` submits the form, unless the field has an `OnSubmit` of its own. A `NumberField` first takes the typed number, and the form is submitted with it; outside a `FormItem`, Enter in a `NumberField` only takes the number.

For a birth date, give the `DatePicker` an `InitialMonth` some thirty years back and a `Max` of today. Its calendar then opens near the date, and its title goes to the months of the year and the years of a decade, so another year is a few presses away. The keyboard reaches the title with Tab, and the grids of months and years take the arrows, Page Up and Page Down, and Enter.

A registration form:

```go
package main

import (
	"log"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

type signup struct{ gowaku.StatefulBase }

type signupState struct {
	gowaku.StateBase
	form   field.FormController
	name   string
	people float64
	born   time.Time
	agreed bool
}

func (signup) CreateState() gowaku.State { return &signupState{people: 1} }

func (s *signupState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	today := time.Now()
	return basic.Pad(24, field.Form{Controller: &s.form, OnSubmit: s.save, Child: basic.Column{Spacing: 12, Children: []gowaku.Widget{
		// 入力してください
		field.FormItem[string]{Value: s.name, Required: true, Child: field.Input{
			Label: "氏名", Value: s.name, OnChange: func(v string) { s.SetState(func() { s.name = v }) },
		}},
		// Enter here submits the form, with the number just typed.
		field.FormItem[float64]{Value: s.people, Child: field.NumberField{
			Label: "人数", Value: s.people, Min: 1, Max: 9,
			OnChange: func(v float64) { s.SetState(func() { s.people = v }) },
		}},
		// 選択してください. The calendar opens on 1995; its title goes to
		// the months and the years.
		field.FormItem[time.Time]{Value: s.born, Required: true, Child: field.DatePicker{
			Label: "生年月日", Value: s.born, Today: today, Max: today,
			InitialMonth: today.AddDate(-30, 0, 0),
			OnChange:     func(d time.Time) { s.SetState(func() { s.born = d }) },
		}},
		// チェックしてください, unless RequiredMessage says otherwise.
		field.FormItem[bool]{Value: s.agreed, Required: true, RequiredMessage: "利用規約への同意が必要です", Child: control.Checkbox{
			Label: "利用規約に同意する", Value: s.agreed,
			OnChange: func(v bool) { s.SetState(func() { s.agreed = v }) },
		}},
		control.Button{Label: "登録", Style: control.ButtonPrimary, OnPress: func() { s.form.Submit() }},
	}}})
}

func (s *signupState) save() { log.Printf("%s, %v, %s", s.name, s.people, s.born.Format("2006-01-02")) }

func main() {
	if err := gowaku.Run(gowaku.App{Title: "signup", Home: signup{}}); err != nil {
		log.Fatal(err)
	}
}
```

### Vertical input

A field with a vertical `Style` edits in columns. The caret, the selection and the IME follow the columns: Down moves to the next character, Left to the next column, and the IME's candidate window opens beside the caret.

```go
field.Input{
	Multiline: true,
	Style:     text.Style{Size: 16, Orientation: text.Vertical},
	OnChange:  func(v string) { s.SetState(func() { s.draft = v }) },
}
```

### Ruby in a text field

`Ruby` takes a function that finds ruby notation in the text. `aozora.Marks` has the right signature:

```go
field.Input{
	Multiline: true,
	Ruby:      aozora.Marks,
	Value:     "｜吾輩《わがはい》は猫である。",
	OnChange:  func(v string) { s.SetState(func() { s.text = v }) },
}
```

The field draws the ruby over its base and hides the notation, except in a mark that an end of the selection is in: that one shows as written, to be edited. Everything else (the value, `OnChange`, the selection, copy and undo, what a screen reader gets) works on the text as written, with the notation. Saved files keep the readings. Other notations work too: `Ruby` is any function that appends `text.RubyMark` values.

## The IME

While the user converts with an IME, the field draws the composition inline, at the caret, in the field's own font:

- Each clause has a thin underline of its own.
- The clause the candidates apply to (the target clause) gets a tinted background and a thick underline.
- The candidate window opens next to the target clause, or the caret, and keeps clear of the line; in vertical text it opens beside the column.

The IME is on only while a text field has the keyboard, so typing on a button does not start a conversion. When the field loses focus during a conversion, the field commits the composition itself, so nothing is lost. A password field (`Obscure`) keeps the IME off.

What each OS gives:

| OS | Through | Inline composition | Clauses and target clause | Candidate window at the caret |
|---|---|---|---|---|
| Windows | IMM32. IMEs built on TSF work through the system's compatibility layer | Yes | Yes | Yes |
| macOS | `NSTextInputClient` | Yes | Yes, from the underline thickness of the marked text | Yes |
| Linux, Wayland | `text-input-v3` | Yes | Target clause only (see below) | Yes |
| Linux, X11 | XIM | No: the input method draws the composition in its own window | No | No |

Notes:

- **Wayland.** The `text-input-v3` protocol carries the composition text and a cursor range, but no clauses. When the input method selects a range, as Mozc does with the clause it converts, gowaku highlights that range as the target clause and treats the rest as converted. Without a range, the whole composition is shown as unconverted. Verified with fcitx5 and Mozc on sway. ibus on GNOME has been tried only in part.
- **X11.** gowaku takes what the input method commits. The input method shows the composition and the candidates in its own windows, which gowaku cannot place at the caret.
- **macOS.** Inline composition, clauses and the candidate window are tested by code, not yet by hand.
- **Linux in general.** Text from an input method has not been verified end to end on a real desktop.

Wayland is used when `WAYLAND_DISPLAY` is set; `GOWAKU_BACKEND=x11` forces X11. See [Platforms](platforms.md).

## Using the text package directly

The `text` package works without widgets: shape a string once, then lay it out at any width. This is what the `-lines` mode of `examples/jptext` and `examples/tategaki` uses to print line breaks for comparing with other typesetting programs.

```go
package main

import (
	"fmt"
	"log"

	"github.com/bankai-tensei/gowaku/text"
)

func main() {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		log.Fatal(err)
	}
	s := "「ほら、見て。」と彼は言った。行頭に句点が来ないように組む。"
	l := shaper.Shape(s, text.Style{Size: 16}).Layout(16 * 10) // ten ems
	for i := range l.LineCount() {
		start, end := l.LineRange(i)
		fmt.Println(s[start:end])
	}
}
```

A `text.Shaper` and everything made from it belong to one goroutine. Inside an application, the widgets share the one that `Run` made; create your own only outside the widget tree, as above. A widget or a `layout.Painter` of your own gets the shared one with `core.DependOn[*text.Shaper](ctx)` in Build; the painter then draws a label at a point with `paint.Canvas.DrawText`, and `text.Labels` keeps the labels it picks while painting. Package `text/jlreq` holds the line breaking rules alone, on made-up metrics, with no fonts.

## See also

- [Accessibility](accessibility.md): what a screen reader gets from text and fields
- [Testing and previews](testing-and-preview.md): `gowakutest.Type` sends committed text to a field
- [ADR 0007](../../ADR/0007-text-architecture.md), [ADR 0023](../../ADR/0023-vertical-text.md), [ADR 0074](../../ADR/0074-inline-ruby.md) (Japanese): design decisions
