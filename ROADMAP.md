# Roadmap

[日本語](ROADMAP.ja.md)

What gowaku does not do yet, where it is known to fall short, and what it will not do. There are no dates here. How the library is built is in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Not verified yet

- **Linux desktops.** gowaku has run on X11 (XWayland, Xvfb) and on Wayland (Weston under WSLg, a headless sway), with a software Vulkan driver. It has not been run on a real Ubuntu desktop with a real GPU, nor on GNOME or KDE.
- **Linux input methods.** On Wayland, composition shows inline through `text-input-v3`. This was checked with fcitx5 and Mozc on sway; ibus on GNOME only partly.
- **macOS.** VoiceOver, drag and drop with Finder, the IME, transparent windows and the effects of drawing groups are tested by code, not yet by hand.
- **Window decorations.** The Wayland window icon (`xdg-toplevel-icon-v1`) has not been tried on a compositor that has it. On X11, transparency and always-on-top have not been checked under a window manager that implements all of EWMH.

## Known limits

- **IME under X11** takes only committed text through XIM. The input method draws its own composition window.
- **IME clauses on Wayland.** `text-input-v3` carries no clause attributes, so the focused clause is not highlighted there as it is on Windows and macOS.
- **macOS** runs on Apple Silicon only. Intel Macs return `ErrUnsupported`. Notifications need the application inside an .app bundle.
- **OS settings.** gowaku follows the OS for light and dark, reduced motion and locale. It does not yet follow high contrast, the OS accent color or the OS text scale.
- **Drawing groups.** Inside a group that needs multisampling (one that contains paths), blend modes that read the backdrop fall back to source-over, and backdrop blur is not drawn. Groups larger than 2048 pixels are drawn at lower resolution. Filters under rotation or skew are approximated.
- **Window features not covered:** click-through on transparent areas, the AppUserModelID on Windows, and always-on-top on Wayland, which has no standard protocol for it.
- **Markdown** does not fetch images from the network (an application can supply them), does not show SVG or animated GIF images, and does not scroll wide tables sideways.
- **File dialog on Linux** needs xdg-desktop-portal. Without it, `ShowFileDialog` reports `ErrNoFileDialog`.

## Planned, not built

- **Packaging helper.** A small helper that sets the application icon and metadata for each OS. It does not exist yet.

## Out of scope

These are deliberate, not gaps.

- **Web (WASM) and mobile.** gowaku is for desktops. A WebGPU backend would fit behind `gpu.Backend`, but the platform side (DOM events, IME) is a different thing.
- **3D.** gowaku is a 2D UI library. A custom shader can draw small 3D effects.
- **Mirroring layouts for right-to-left languages.** BiDi text works. Flipping rows and icons would touch every widget.
- **Printing and PDF output.**
- **Hot reload.** Not practical in Go. Use previews (`cmd/gowaku-preview`) and the Inspector instead.
- **Video playback.** The codecs are not available without cgo.
- **Plugins and dynamic loading.** Go's `plugin` package needs cgo.
- **A file dialog widget of its own.** The OS dialog is used everywhere.
- **Charts with axes and legends.** `Sparkline` draws small lines and bars, no more.
- **Ready-made themes besides the default.** `theme.FromAccent` and `theme.Extension` are the ways to adapt it.
- **Installers, code signing and auto-update.** Distribution is left to the application.
