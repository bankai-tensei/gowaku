# Theme

[Guides](README.md) · [日本語](../ja/theme.md)

gowaku has a design of its own: quiet neutral surfaces, one accent color, hairlines instead of heavy shadows, and a type scale set from Japanese. Package `theme` holds its tokens. This guide shows how to pick the accent, follow light and dark, change the theme while running, and read the tokens in your widgets. The design itself is described in [docs/DESIGN.md](../../DESIGN.md) (Japanese).

## Theme and Set

- A `*theme.Theme` is the full set of tokens for one brightness: `Color` (`theme.Colors`), `Text` (`theme.TextTheme`), `Radius` (`theme.Radii`) and `Brightness`.
- A `theme.Set` is a light and a dark `*theme.Theme`, so that the application can follow the OS.

`theme.Default()` is the set an application gets when it asks for nothing: an indigo accent, `#2f5fd0`. Its themes are shared, so treat them, and any theme, as read-only.

## One accent color

Every color follows from one accent. Give it to `theme.FromAccent` and put the set in `App.Theme`:

```go
func app() gowaku.App {
	return gowaku.App{
		Title:     "notes",
		Theme:     theme.FromAccent(geom.Hex(0x2f8f5b)),
		ThemeMode: gowaku.ThemeSystem,
		Home:      home,
	}
}
```

`FromAccent` derives the accent fill, the text on it, the accent as text, and the tint of a selection, for both brightnesses, in the HCT color space. The surfaces stay neutral whatever the accent. Contrast is checked: `Ink` and `Ink2` read at 4.5:1 or more on every surface ([ADR 0041](../../ADR/0041-ink-contrast.md), Japanese).

`theme.New(accent, b)` makes a single `*theme.Theme` of one brightness, `theme.BrightnessLight` or `theme.BrightnessDark`.

## Light and dark

`App.ThemeMode` says which half of the set to use:

| Mode | Does |
|---|---|
| `gowaku.ThemeSystem` (zero value) | Follows the OS, also while running. |
| `gowaku.ThemeLight` | Stays light. |
| `gowaku.ThemeDark` | Stays dark. |

The title bars and backgrounds that the OS draws follow the theme too.

## Changing the theme while running

`gowaku.SetTheme(ctx, set, mode)` changes the look of every window, as `App.Theme` and `App.ThemeMode` set it at the start. `gowaku.CurrentTheme(ctx)` returns what is in use. Both go through the window of `ctx`, so under a `gowakutest.Tester` they change the theme of its tree instead. A settings page might offer the mode:

```go
func modeButtons(ctx gowaku.BuildContext) gowaku.Widget {
	set := func(mode gowaku.ThemeMode) func() {
		return func() {
			s, _ := gowaku.CurrentTheme(ctx)
			gowaku.SetTheme(ctx, s, mode)
		}
	}
	return basic.Row{Spacing: theme.Space8, Children: []gowaku.Widget{
		control.Button{Label: "System", OnPress: set(gowaku.ThemeSystem)},
		control.Button{Label: "Light", OnPress: set(gowaku.ThemeLight)},
		control.Button{Label: "Dark", OnPress: set(gowaku.ThemeDark)},
	}}
}
```

and the accent:

```go
func accentButton(ctx gowaku.BuildContext, accent geom.Color) gowaku.Widget {
	return control.Button{Label: "Use this color", OnPress: func() {
		_, mode := gowaku.CurrentTheme(ctx)
		gowaku.SetTheme(ctx, theme.FromAccent(accent), mode)
	}}
}
```

Every widget that reads the theme rebuilds. Windows opened later take the new theme too. Call `SetTheme` on the UI goroutine, outside `Build`, as from a callback. See [ADR 0067](../../ADR/0067-runtime-theme.md) (Japanese).

## Reading the theme

`basic.ThemeOf(ctx)` returns the nearest theme above a widget and rebuilds the widget when it changes:

```go
func section(ctx gowaku.BuildContext, title, note string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{
		Color:   th.Color.Card,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Padding: geom.Symmetric(theme.Space16, theme.Space12),
		Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space6, Children: []gowaku.Widget{
			basic.Text{Content: title, Style: th.Text.Headline},
			basic.Text{Content: note, Style: th.Text.Caption.WithColor(th.Color.Ink2)},
		}},
	}
}
```

### The tokens at a glance

| Token | Names | Type |
|---|---|---|
| `theme.Space*` | `Space2`, `Space4`, `Space6`, `Space8`, `Space10`, `Space12`, `Space16`, `Space20`, `Space24`, `Space28` (the number is the size) | untyped constant, logical pixels |
| `theme.ControlHeight` | 30 | untyped constant, logical pixels |
| `theme.Lift*` | `LiftNone`, `LiftControl`, `LiftKnob`, `LiftFloat` | `theme.Lift`, for `th.Color.Shadows` |
| `th.Radius` | `Check` (4), `Control` (8), `Group` (12), `Dialog` (16) | `float32` fields of `theme.Radii` |
| `th.Text` | `LargeTitle`, `Title`, `Headline`, `Body`, `Control`, `Caption`, `Section`, `Reading`, `Code` | `text.Style` fields of `theme.TextTheme` |
| `th.Color` | the table under Colors | `geom.Color` fields of `theme.Colors` |

### Colors

Colors are named for what they do, not for their hue:

| Field | Use |
|---|---|
| `Canvas`, `Sidebar`, `Card`, `Raised` | The ground of a window, a sidebar, a group, and what floats such as a dialog. |
| `Glass` | `Raised` made translucent, for a surface over a blurred backdrop. |
| `Field`, `Control` | The ground of an input and of an ordinary button. |
| `Ink`, `Ink2`, `Ink3` | Text, supporting text, and faint marks such as placeholders. `Ink3` is below 4.5:1 on purpose: do not use it for text that tells something. |
| `Hair` | A line. |
| `Fill`, `FillStrong` | Quiet grounds that work on any surface. |
| `Accent`, `OnAccent`, `AccentInk`, `AccentTint` | The accent fill, text on it, the accent as text or a thin line, and the mark of a selection. |
| `Danger`, `Success` | Destructive actions and errors; what went well. Both read as text on every surface, and in the light theme carry white text as a fill. |
| `Warning` | What needs attention before it goes wrong. A bright amber for icons, dots, borders and tints; it is not meant for text, so set the words in `Ink` beside it. |
| `Shadow`, `Scrim` | The color of shadows; what covers a page behind a dialog. |

### Text styles

`th.Text` is the type scale. Each role sets the size, weight and line height:

| Role | Size | Weight | Line |
|---|---|---|---|
| `LargeTitle` | 24 | 600 | 32 |
| `Title` | 16 | 600 | 24 |
| `Headline` | 13 | 600 | 20 |
| `Body` | 13 | 400 | 20 |
| `Control` | 13 | 500 | 20 |
| `Caption` | 12 | 400 | 18 |
| `Section` | 12 | 500 | 18 |
| `Reading` | 15 | 400 | 26 |
| `Code` | 13 | 400 | 20 |

`Code` is monospaced: its `Family` is `theme.MonoFamilies`, a list of the monospaced fonts of Windows, macOS and Linux that each system reads in order, so `basic.Text{Content: id, Style: th.Text.Code}` needs no family of its own. `editor.Editor` and code in markdown use it unless told otherwise.

The roles leave the color unset. `basic.Text` fills an unset color with `Ink`, and an unset size with that of `Body`, so `basic.Text{Content: "..."}` already reads well. For another color, use `WithColor`, as `th.Text.Caption.WithColor(th.Color.Ink2)`. `WithSize`, `WithWeight` and `WithAlign` change the other parts. The [text guide](text-and-ime.md) covers the rest of `text.Style`.

### Spacing, corners, heights and shadows

- **Spacing**: `theme.Space2`, `Space4`, `Space6`, `Space8`, `Space10`, `Space12`, `Space16`, `Space20`, `Space24`, `Space28`, in logical pixels. All are even, which lands them on whole physical pixels at 100, 150 and 200 percent. Prefer them to raw numbers.
- **Corners**: `th.Radius.Check` (4), `Control` (8, a button, an input or a row), `Group` (12, a card) and `Dialog` (16).
- **Height**: `theme.ControlHeight` (30) is the height of a regular control.
- **Shadows**: `th.Color.Shadows(lift)` returns the two layers of a shadow for a height above the page: `theme.LiftNone`, `LiftControl`, `LiftKnob` or `LiftFloat`. The layers are `paint.BoxShadow`, the type `basic.Container`, `layout.Decoration` and the controls take, so they go straight into `Shadows: th.Color.Shadows(theme.LiftFloat)`. Each call makes a new slice: keep it once per theme when it sits on a hot path.

## A part with a look of its own

`basic.Theme` gives the widgets below it another theme, such as a dark panel in a light window:

```go
var darkPanel = theme.New(geom.Hex(0x2f5fd0), theme.BrightnessDark)

func inspector(child gowaku.Widget) gowaku.Widget {
	return basic.Theme{Data: darkPanel, Child: child}
}
```

A theme set this way stays put when `SetTheme` changes the application's theme.

`basic.MediaQueryOf(ctx).Brightness` is the brightness the OS asks for, whatever theme is in use. `basic.ThemeOf(ctx).Brightness` is the one in use. Read the second to draw in step with the theme.

## Tokens of your own

A theme can carry values of your own type, beside the standard tokens. `theme.WithExtensionPair` adds a light and a dark value to a set, and `theme.Extension` or `theme.ExtensionOr` reads it back:

```go
type Brand struct{ Highlight geom.Color }

func brandSet() theme.Set {
	set := theme.FromAccent(geom.Hex(0x2f5fd0))
	return theme.WithExtensionPair(set,
		Brand{Highlight: geom.Hex(0xffd60a)},
		Brand{Highlight: geom.Hex(0x806b05)})
}

func highlight(ctx gowaku.BuildContext) geom.Color {
	b := theme.ExtensionOr(basic.ThemeOf(ctx), Brand{Highlight: geom.Hex(0xffd60a)})
	return b.Highlight
}
```

`theme.WithExtension` does the same for a single `*theme.Theme`. Both return copies and leave the original as it was. Reading an extension does not allocate, so it is fine in `Build`.

## Icons

Package `icons` holds 24 common icons as `*paint.Path` on the 24 unit grid of `basic.Icon`: `Folder`, `File`, `Document`, `Image`, `Search`, `Plus`, `Minus`, `Close`, `Check`, the four chevrons, `Settings`, `Trash`, `Edit`, `Copy`, `Home`, `Star`, `Info`, `Warning`, `Error`, `Menu` and `More`. They are drawn with a line 1.75 units wide with round ends, like the icons the widgets draw themselves, and take the color they are given. The same paths go into `SidebarItem.Icon`, `BreadcrumbItem.Icon`, `Button.Icon` and the `Tree.IconOf` of a file tree.

```go
// The paths are made once and kept: basic.Icon scales a path again
// whenever it gets another pointer.
type fileIcons struct {
	folder, search *paint.Path
}

func newFileIcons() fileIcons {
	return fileIcons{folder: icons.Folder(), search: icons.Search()}
}

func (ic fileIcons) header(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []gowaku.Widget{
		basic.Icon{Path: ic.folder, Size: 16, Color: th.Color.AccentInk},
		basic.Icon{Path: ic.search, Size: 16, Color: th.Color.Ink2, Label: "Search"},
	}}
}
```

Each call builds a new path, so keep them in a State or a struct like this rather than calling them in every `Build`. For a shape the set lacks, parse the path data of an icon set with `paint.ParseSVGPath`.

## Further reading

- [docs/DESIGN.md](../../DESIGN.md) (Japanese): the colors, type, shapes and components of the design, with the reasons.
- [ADR 0011](../../ADR/0011-own-design.md) (Japanese): why gowaku has its own design.
- [ADR 0065](../../ADR/0065-theme-extensions-and-coverage.md) (Japanese): theme extensions.
- `examples/gallery` shows the design, and follows the mode of the OS.
