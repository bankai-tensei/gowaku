# gowaku guides

[日本語](../ja/README.md)

These guides explain how to build a desktop application with gowaku, from the first window to packaging. Read them in order the first time; later, jump to the one you need. For each widget and its fields, see the [widget catalog](../../catalog/en/README.md) and the package documentation.

| Guide | Covers |
|---|---|
| [Getting started](getting-started.md) | Requirements, installing, the first window, a counter, and the samples. |
| [Concepts](concepts.md) | Widgets, elements and render objects; state and `SetState`; `BuildContext`, inherited data and keys; how a change reaches the screen, and why an idle window draws nothing. |
| [Layout](layout.md) | The constraints protocol; rows, columns, stacks, wraps, grids, scrolling and lists; writing a custom layout. |
| [Theme](theme.md) | One accent color, light and dark, changing the theme while running, text styles and tokens, tokens of your own. |
| [Motion](motion.md) | Implicit animations, switching children, curves, controllers, springs, page transitions and heroes. |
| [Text and input methods](text-and-ime.md) | Text styles, Japanese typesetting, ruby, vertical text, and input methods. |
| [Accessibility](accessibility.md) | Screen readers, semantics, focus and the keyboard. |
| [Windows and the OS](windows-and-os.md) | Windows, modal windows, menus, the title bar, the clipboard, file dialogs, drag and drop, the tray and notifications. |
| [Testing and previews](testing-and-preview.md) | Running a widget tree under `go test` without a window, and previews that redraw on save. |
| [Packaging](packaging.md) | The icon and metadata of an application. |
| [Platforms](platforms.md) | What each OS needs and supports, and the known limits. |
| [Recipes](recipes.md) | Complete programs for a list and a detail, a settings window with a confirmation, a long vertical text and a job in the background, and the common pitfalls. |

## Other references

- [Widget catalog](../../catalog/en/README.md): every public widget, with a picture and an example.
- [docs/DESIGN.md](../../DESIGN.md) (Japanese): the visual design.
- [docs/ADR/](../../ADR/) (Japanese): the design decisions, one file each.
- [`examples/`](../../../examples/): runnable samples.
