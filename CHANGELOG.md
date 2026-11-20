# Changelog

All notable changes to gowaku are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). Before v1.0, a minor version may change the public API; each such change is listed under **Changed** or **Removed**.

## [Unreleased]

## [0.2.0] - 2026-11-21

### Changed

- `gowaku.FileDialog`, `FileFilter`, `ClipboardContent` and `ClipboardKinds` no longer come from `platform`, which promises no compatibility: they are defined in `core` and named in gowaku, as `Widget` and `BuildContext` are. They have the same fields, so code that uses them through gowaku still builds; code that passed them to or from `platform` must convert. `gowaku.ThemeMode` is likewise `theme.Mode`, with `ThemeSystem`, `ThemeLight` and `ThemeDark` as before.
- `gowaku.ShowFileDialog`, `SetTheme` and `CurrentTheme` take a `BuildContext` first, as `ShowTray` and the clipboard functions do: `ShowFileDialog(ctx, d, done)`, `SetTheme(ctx, set, mode)`, `CurrentTheme(ctx)`. They go through the window of `ctx`, so they work under a `gowakutest.Tester`, and the file dialog opens in front of that window instead of the focused one, or of another open window once that one has closed. Code with no context of its own, as a callback of `Post`, keeps the `BuildContext` of a widget, or uses `WindowHandle.ShowFileDialog`. Outside a window they no longer panic: `ShowFileDialog` calls `done` with `ErrNoFileDialog`, `SetTheme` does nothing, and `CurrentTheme` returns the zero `theme.Set`, which stands for `theme.Default()`, and `ThemeSystem`.
- `gowakutest.Tester.ClipboardContent` and `SetClipboardContent` take a `core.ClipboardContent`, which is `gowaku.ClipboardContent`.
- `ClipboardContent.Kinds` has a value receiver, so that `gowaku.ReadClipboard(ctx, k).Kinds()` builds.
- `field.Input` takes `OnFocus func(focused bool)` as a field of its own, and `field.Extras` and `Input.Extras` are gone. Replace `Extras: field.Extras{OnFocus: f}` with `OnFocus: f`.
- `overlay.Dialog.OnDismissRequest` and `overlay.Sheet.OnDismissRequest` are now `func() bool`, as `OnCloseRequest` of a window is: Escape or a click beside the dialog or sheet closes it when the function returns true and leaves it open when it returns false. A function that kept the dialog open and called `dismiss` itself to close it should now `return false`; one that only looked on can `return true` and need not call `dismiss`.
- `overlay.ShowDialog` and `overlay.ShowSheet` return `dismiss func()`, as `overlay.ShowToast` does: the same function that `build` gets, which does nothing once the dialog or sheet is closing or gone. Code that called them as a statement compiles as before; code that saved the `dismiss` of `build` in a variable only to close the dialog from outside can take the return value instead.

### Added

- `core.FileDialogHost`, which a host implements to show file dialogs.
- `theme.Mode`, with `ModeSystem`, `ModeLight`, `ModeDark` and `Mode.For`, which `gowaku.ThemeMode` names.
- `gowakutest`: file dialogs that the test answers, through `Tester.FileDialogs`, `Tester.FileDialog` and `FileDialog.Options`, `Choose`, `Cancel` and `Fail`; `gowaku.SetTheme` changes the theme of the Tester's tree, and `ThemeSystem` follows `Options.Dark`.

## [0.1.2] - 2026-11-20

### Fixed

- Ruby: a group ruby drew its whole base with the font and orientation of its first character, so that in `AI時代` the kanji took a Latin font, or a kanji from a fallback font became tofu, and in vertical text the kanji turned with the Latin letters. The base now keeps the look of each character. Line breaking and the space between Japanese and Western text after the group follow its last character, not its first; `jlreq.Unit` has a new `Last` field for that. Ruby longer than its base now overhangs small kana and the prolonged sound mark as it does other kana (JLReq 3.3.8). In `text/aozora`, a kanji with a variation selector, as in `葛󠄀《かつら》`, is part of the implicit base.
- Vertical text: dashes, leaders and curly quotes (`… ‥ ― — – ‐ “ ” ‘ ’`) took their orientation from the script around them, so between kana a dash with no vertical form in the font, such as `—` in Yu Gothic, stood upright and crossed the column, and alone or next to Latin `…` turned sideways instead of taking its vertical form. They now stand upright where the font has a `vert` form for them and turn sideways where it does not.
- JLReq Oikomi counted the space after the last character of a line, such as the quarter em before Western text, as room to squeeze, although that space goes with the line end. A character that must not start a line was then pushed in, and the line ran past its width.
- JLReq: a quarter em of Japanese–Western space was put between the ideographic space U+3000 and Western text. JLReq puts none next to a full-width space.
- Editor: a search on its own goroutine, as of a file, looked at `maxMatches` and at being stopped only every 65,536 lines, so that a frequent query over long lines gathered millions of matches before the cap applied, and a new query waited for the batch to end. It now stops at once, at exactly 100,000 matches, also within one line.
- Editor: every edit with a search open copied all the matches after the edited lines into a new slice, megabytes a keystroke with 100,000 matches; they now move in place.
- Editor: `Controller.ReplaceAll` replaced the matches one at a time, each moving the lines or the pieces of a file after it and, in an editor that wraps, counting the rows of every line again, so that 100,000 matches took from seconds to minutes. It now makes the new lines in one pass over the matched lines and changes the text in one step; Undo still takes it back at once.
- `layout.MeasuredExtents.Resize` rebuilt its trees over every row whenever the count changed, so a `List` with `MeasureItems` that gained one row walked all of them. Adding k rows at the end now costs O(k log n), and dropping rows O(log n).
- `collection.Table` with `RowKey` and `GridView` with `ItemKey` called the key for every row when one row was added or removed at the end, to find the selection again. When the selected rows, the focus and the anchor are still at their indexes, the selection now stays without looking further, and an unchanged build calls the key once per selected row instead of twice.

## [0.1.1] - 2026-11-20

### Fixed

- SVG: an arc whose numbers sweep no angle, or overflow, panicked with an index out of range; it is now drawn as a line. A large `stroke-dashoffset` over a short `stroke-dasharray` could loop forever; the offset is now reduced in float64, and a pattern whose length is not finite draws the stroke solid.
- Layers of `RepaintBoundary` that left the tree stayed in the renderer's layer pool for good, and with them the boundary and its whole subtree, so that scrolling a `List` or `GridView` leaked every row that had shown. Entries not drawn for 120 frames are now dropped. `Canvas.Reset` and the render children of an element no longer keep references past their length.
- Windows: the COM calls of the file dialog and of DirectComposition did not keep pointer arguments alive and in place during the call, and the filter specs of the file dialog were not kept until it returned. `SysAllocString` and a UI Automation call converted pointers to `uintptr` outside the call. Wayland: the locale string given to xkb compose was not kept alive during the call.

## [0.1.0] - 2026-11-18

The first public release.

- A Widget / Element / RenderObject tree with value-type widgets, states, keys and inherited data.
- GPU rendering without cgo: Direct3D 11 on Windows, Vulkan on Linux (Wayland and X11), Metal on macOS (Apple Silicon), and a CPU renderer for tests. Rounded rectangles, gradients, shadows, blur, blend modes, affine transforms, paths, clips, custom shaders, SVG and animated images.
- Text: shaping, fallback fonts, color emoji, BiDi, variable fonts, hyphenation, and Japanese typesetting after JLReq, with ruby and vertical text.
- Input: keyboard, mouse, touchpad, touch, inline IME composition on Windows, macOS and Wayland.
- Accessibility through UI Automation, AT-SPI and the macOS accessibility API.
- Over a hundred widgets in `widgets/basic`, `control`, `field`, `collection`, `nav`, `overlay`, `interact` and `editor`, and a Markdown view.
- Themes generated from one accent color, light and dark following the OS.
- Animations: implicit animations, controllers, springs, hero and page transitions.
- Lists, tables, trees and editors of any length: only the rows in view are laid out, and scroll offsets are float64, so that a hundred million rows still scroll a pixel at a time. `collection.Table` takes rows through `RowCount` and `Row`; `editor.Editor` takes lines through `editor.LineSource`, as those of a file of gigabytes, keeps only the edited lines in memory and searches on a goroutine of its own.
- Desktop: multiple windows, modal windows, menus, custom title bars, tray icons, notifications, clipboard, file dialogs, drag and drop.
- `gowakutest` for tests without a window, `gowakupreview` and `cmd/gowaku-preview` for live previews.

[Unreleased]: https://github.com/bankai-tensei/gowaku/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/bankai-tensei/gowaku/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/bankai-tensei/gowaku/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/bankai-tensei/gowaku/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/bankai-tensei/gowaku/releases/tag/v0.1.0
