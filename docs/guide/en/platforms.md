# Platforms

[日本語](../ja/platforms.md)

gowaku runs on Windows, Linux and macOS (Apple Silicon), with no cgo on any of them. This page lists what each OS uses, what has been verified and where, and the known limits. "Verified" means checked on a running system; "tested by code" means automated tests drive the OS API, but nobody has yet checked the result by hand.

## Overview

| | Windows | Linux | macOS |
|---|---|---|---|
| Architectures | amd64, arm64 | amd64, arm64 | arm64 only |
| GPU | Direct3D 11 | Vulkan 1.0 | Metal |
| Window system | Win32 | Wayland, or X11 | AppKit |
| IME, inline with clauses | Yes (IMM32) | Wayland: yes, target clause only. X11: no | Yes (tested by code) |
| Screen reader | UI Automation | AT-SPI2 | NSAccessibility (tested by code) |
| Menu bar | Drawn by gowaku | Drawn by gowaku | System menu bar (`NSMenu`) |
| File dialog | `IFileOpenDialog`, `IFileSaveDialog` | xdg-desktop-portal | `NSOpenPanel`, `NSSavePanel` |
| Tray, notifications | `Shell_NotifyIcon`, toast | StatusNotifierItem, `org.freedesktop.Notifications` | `NSStatusItem`, `UNUserNotificationCenter` (bundle only) |
| Drag and drop with other programs | OLE | XDND, `wl_data_device` | Finder (tested by code) |
| Pinch, touch, pen | DirectManipulation, `WM_POINTER` | XInput 2, `wl_touch`, tablet-v2 (Wayland only for pens) | Trackpad pinch |

Other systems, and Intel Macs, build but `gowaku.Run` returns `gowaku.ErrUnsupported`.

## Windows

- **GPU**: Direct3D 11, feature level 11.0 or 11.1. A hardware GPU is required: the software renderer WARP is skipped, and without a GPU `Run` fails. On machines with several GPUs gowaku asks DXGI for the power-saving one, usually the integrated GPU, since a UI is idle most of the time (Windows 10 1803 and later; earlier versions take the OS default). Windows uses Direct3D 11 only; gowaku's Vulkan backend is not used on Windows except in its own comparison tests.
- **Window system**: Win32, with the standard title bar or a custom one (`CustomFrame`), where Windows 11 shows its snap layouts over the maximize button.
- **IME**: IMM32. IMEs built on TSF, such as the Microsoft IME of Windows 11 and Mozc, work through the system's compatibility layer. The composition shows inline with its clauses and the target clause, and the candidate window follows the caret, also in vertical text.
- **Accessibility**: a UI Automation provider. Verified with Narrator by hand and with NVDA through its speech log, and with automated tests through a UI Automation client.
- **Verified on**: Windows 11, on an Intel Core i7-12700H with Iris Xe graphics and on an NVIDIA RTX 3050 Ti. The benchmarks in the README were measured there.

## Linux

gowaku picks the window system at start: Wayland if `WAYLAND_DISPLAY` is set, X11 otherwise. If Wayland fails and `DISPLAY` is set, it falls back to X11 (XWayland usually runs). `GOWAKU_BACKEND=wayland` or `GOWAKU_BACKEND=x11` forces one. Drawing is Vulkan in both cases.

The libraries are loaded at start with purego: `libvulkan.so.1`; `libwayland-client.so.0`, `libwayland-cursor.so.0` and `libxkbcommon.so.0` for Wayland; `libX11.so.6` (and `libXi.so.6` if present) for X11. File dialogs, the desktop theme, the tray, notifications and the screen reader bridge go through D-Bus with a pure-Go client; without a session bus they are off and the application still runs.

### Wayland

- **Title bar**: when the compositor offers `xdg-decoration`, it draws the title bar. Otherwise, as on GNOME and Weston, gowaku draws a small one (minimize, maximize, close, move, resize) in the theme of the application. `GOWAKU_WAYLAND_DECORATIONS=client` makes gowaku draw it anyway, for trying it out.
- **Scaling**: `fractional-scale-v1` with `viewporter`, else the integer scale of the output.
- **IME**: `text-input-v3`. The composition shows inline and the candidate window is placed at the caret. The protocol has no clauses: when the input method selects a range, as Mozc does, that range shows as the target clause. Verified with fcitx5 and Mozc on sway. With ibus on GNOME, the protocol exchange was correct but the keys did not reach ibus in the test setup; this is not resolved.
- **Limits of the protocol**: applications cannot place windows, so a torn-off tab opens where the compositor puts it; `AlwaysOnTop` is ignored; the window icon shows only where the compositor has `xdg-toplevel-icon-v1`, and otherwise comes from the desktop file named by `App.AppID`. A screen reader gets positions relative to the window, not the screen.

### X11

- **IME**: XIM. gowaku takes the committed text; the input method draws the composition and the candidates in its own windows, which gowaku cannot place at the caret. There is no inline composition on X11.
- **Transparent windows** need a compositing manager.
- **Custom title bar**: dragging and resizing through `_NET_WM_MOVERESIZE` has not been checked with a real pointer.

### Verified environments

gowaku has been run on:

- X11 through XWayland and through Xvfb.
- Wayland on WSLg's Weston and on a headless sway.
- Vulkan through Mesa's software driver (lavapipe) in all of these.

It has **not yet** been verified on a real Ubuntu desktop with a real GPU, nor on GNOME or KDE sessions. Screen reader support was checked with Orca through its debug log, on Xvfb and on the headless sway. Text from an input method has been verified on headless sway only, not end to end on a real desktop.

Notes on desktop services:

- The tray needs a StatusNotifierItem host. GNOME has one only with the AppIndicator extension; without it, `ShowTray` returns `gowaku.ErrNoTray`.
- File dialogs need xdg-desktop-portal; without it, the result is `gowaku.ErrNoFileDialog`. With the GTK 3 portal backend, the dialog may not stay in front of its window.
- The light and dark theme follows the portal's `color-scheme`, with the GNOME setting as a fallback. Reduced motion follows GNOME's `enable-animations`; there is none on KDE.

## macOS

- **Apple Silicon only.** `darwin/arm64` builds and runs; on Intel Macs `Run` returns `gowaku.ErrUnsupported`.
- **GPU**: Metal. The shaders are kept as source in the binary and compiled when the GPU device is created, so no Xcode is needed to build.
- **Threads**: AppKit runs only on the main thread, so `Run` must be called from the main goroutine.
- **Keys**: the Command key arrives as `input.ModControl`, so shortcuts written as Ctrl+C or Ctrl+S work as Cmd+C and Cmd+S. The Control key is `ModControl` too.
- **Menu bar**: `overlay.MenuBar` becomes the system menu bar, with the application menu and the Window menu at its ends. Shortcuts with Command go to the menu; others still reach the application.
- **Notifications** need the application inside an .app bundle with a bundle identifier; see [Building and shipping](packaging.md).
- **Tested by code, not yet by hand**: VoiceOver, drag and drop with Finder, and the IME (inline composition with clauses from the marked text, candidate window at the caret).
- **Verified on**: an Apple M5 Mac, where the macOS benchmarks were measured.

## Choosing at run time

| Variable | Effect |
|---|---|
| `GOWAKU_BACKEND=wayland` / `x11` | Linux: force the window system |
| `GOWAKU_WAYLAND_DECORATIONS=client` | Wayland: draw the title bar in gowaku even when the compositor offers one |
| `GOWAKU_A11Y=1` | Linux: connect to the accessibility bus even when the desktop says no screen reader runs |
| `NO_AT_BRIDGE=1` | Linux: never connect to the accessibility bus |

Design notes, in Japanese: [ADR 0013](../../ADR/0013-vulkan-backend.md) (Vulkan), [ADR 0014](../../ADR/0014-x11-platform.md) (X11), [ADR 0021](../../ADR/0021-wayland-platform.md) (Wayland), [ADR 0022](../../ADR/0022-linux-ime-dbus-file-dialog.md) (Linux IME and D-Bus), [ADR 0082](../../ADR/0082-macos-cocoa-metal.md) (macOS).
