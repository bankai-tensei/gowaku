# Changelog

All notable changes to gowaku are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). Before v1.0, a minor version may change the public API; each such change is listed under **Changed** or **Removed**.

## [Unreleased]

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

[Unreleased]: https://github.com/bankai-tensei/gowaku/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/bankai-tensei/gowaku/releases/tag/v0.1.0
