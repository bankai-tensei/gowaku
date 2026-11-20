# Windows and the OS

[日本語](../ja/windows-and-os.md)

This guide covers what lies outside the widget tree: the application and its windows, title bars, menus, the tray and notifications, the clipboard, file dialogs, drag and drop, opening URLs, and the rules for goroutines.

## The application and the UI goroutine

`gowaku.Run` opens the first window and blocks until the last window is closed. The goroutine that calls it becomes the UI goroutine. Build, layout, paint, every callback of a widget, and `SetState` run on it, and nowhere else. Because of that, the state of an application needs no locks.

```go
func main() {
	err := gowaku.Run(gowaku.App{
		Title:  "Notes",
		Width:  900,
		Height: 600,
		Home:   notes{},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

`Run` returns an error only when it cannot start: no GPU, no window system, or an OS gowaku does not support (`gowaku.ErrUnsupported`). A broken contract during a frame panics. On macOS the UI goroutine must be the main goroutine, so call `Run` from `main` itself, not from a goroutine.

`App` also takes the theme (`Theme`, `ThemeMode`), the locale and word tables of the standard widgets (`Locale`, `Strings`), the Linux application id (`AppID`), and the options of the first window: `OnClose`, `OnCloseRequest`, `CustomFrame`, `Icon`, `MinSize`, `MaxSize`, `AlwaysOnTop`, `Fullscreen` and `Transparent`, which work as the fields of `WindowOptions` below.

### Work on other goroutines

Do slow work, such as reading files or the network, on a goroutine of its own, and bring the result back with `gowaku.Post`. `Post` is safe from any goroutine and runs its function on the UI goroutine. The state may be gone by then, so check `Mounted` first:

```go
func (s *pageState) load(path string) {
	go func() {
		data, err := os.ReadFile(path)
		gowaku.Post(func() {
			if !s.Mounted() {
				return
			}
			s.SetState(func() { s.data, s.err = data, err })
		})
	}()
}
```

Outside `Run`, `Post` drops its function, so a worker can outlive the application. `gowaku.Quit` is also safe from any goroutine: it closes every window, which ends `Run`. It does not ask the windows (`OnCloseRequest` does not run), but `OnClose` does.

Calling `SetState` during a Build, or on a state that is no longer in the tree, panics.

## Multiple windows

`gowaku.OpenWindow` opens another window with a tree of its own. All windows share one GPU device and run on the one UI goroutine, so they can share state directly; `examples/multiwindow` shows this.

```go
h, err := gowaku.OpenWindow(gowaku.WindowOptions{Title: "Inspector", Width: 360, Height: 480}, inspector{})
if err != nil {
	return err
}
// later, on the UI goroutine:
h.Close()
```

`WindowOptions` has `Title`, `Width`, `Height`, `Background`, and:

| Field | What it does |
|---|---|
| `OnClose` | Runs when the window closes, by the user, by `Close`, by `Quit` or because the last window went. The tree is still whole, so save here |
| `OnCloseRequest` | Runs when the user asks to close the window. Return false to keep it open |
| `OnDrawError` | Reports what the GPU could not draw, such as a custom shader it refused |
| `CustomFrame` | Removes the OS title bar (see below) |
| `Icon` | The window icon: one image, best 256 pixels square, or the same picture in several sizes |
| `MinSize`, `MaxSize` | Bounds of the client area in logical pixels |
| `AlwaysOnTop`, `Fullscreen` | As named. Wayland ignores `AlwaysOnTop` |
| `Transparent` | Shows the desktop through the alpha of what the window draws. On X11 it needs a compositing manager |

`gowaku.WindowOf(ctx)` returns the `*WindowHandle` of the window a widget is in. The handle can `SetTitle`, `SetIcon`, `SetFullscreen`, `SetAlwaysOnTop`, `SetSizeLimits`, `Activate`, `Close` and `RequestClose`, and tell `Fullscreen()` and `Transparent()`. All handle methods run on the UI goroutine.

### Asking before closing

`OnCloseRequest` runs when the user closes the window: the close button, Alt+F4, the taskbar, the window manager, or a screen reader. It must not block. To ask about unsaved work, return false, show a dialog, and call `Close` once you have the answer. A widget that knows what the window holds can set the callback itself:

```go
func (s *docState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	win := gowaku.WindowOf(ctx)
	win.SetOnCloseRequest(func() bool {
		if !s.dirty {
			return true
		}
		overlay.ShowDialog(ctx, func(dismiss func()) gowaku.Widget {
			return overlay.Dialog{
				Title:   "保存されていない変更があります",
				Message: "閉じる前に保存しますか。",
				Actions: []gowaku.Widget{
					control.Button{Label: "保存しない", OnPress: func() { dismiss(); win.Close() }},
					control.Button{Label: "保存", Style: control.ButtonPrimary, OnPress: func() { s.save(); dismiss(); win.Close() }},
				},
			}
		})
		return false
	})
	return s.page()
}
```

Closing from the program asks nothing: `Close`, `Quit`, and closing the owner of a modal window. `RequestClose` asks as the close button does. On macOS, Cmd+Q and Quit in the Dock send a close request to every window.

### Modal windows

`gowaku.ShowModal` opens a window that belongs to the window of `ctx` and blocks input to it until the new one closes, as a dialog of the OS does. It returns at once; the result comes back through `OnClose`. Call it from a callback, not from a Build.

```go
var h *gowaku.WindowHandle
h, err := gowaku.ShowModal(ctx, gowaku.WindowOptions{Title: "設定", Width: 420, Height: 300, OnClose: s.reload},
	settings{onDone: func() { h.Close() }})
```

For a dialog inside the window, use `overlay.ShowDialog` with an `overlay.Dialog` instead.

### Tabs that become windows

A `nav.TabBar` reports a tab dropped away from every bar of its group through `OnTearOff`, with a `nav.TabTearOff`. Take the tab out of its bar and open a window for it with `gowaku.TearOffWindow`, which places the new window so the tab comes out under the pointer. If the tab was alone in its bar, move the window instead with `WindowHandle.MoveTo`. On Wayland, which does not let applications place windows, the window opens where the compositor puts it.

```go
OnTearOff: func(e nav.TabTearOff) {
	if e.Alone {
		gowaku.WindowOf(ctx).MoveTo(e)
		return
	}
	s.remove(e.Index)
	gowaku.TearOffWindow(e, gowaku.WindowOptions{Title: "文書"}, docWindow{tab: e.Key})
},
```

## Title bars

By default the OS draws the title bar. On Wayland compositors without server-side decorations (GNOME, Weston), gowaku draws a small one itself.

`CustomFrame: true` takes the OS title bar off so the application draws its own. Put a `gowaku.TitleBar` at the top of the root with what it should hold, such as a menu bar or a search field. It draws the minimize, maximize and close buttons, moves the window when dragged where nothing takes the pointer, maximizes on a double click, and opens the window menu on a secondary click. Windows 11 shows its snap layouts over the maximize button. The edges still resize the window.

```go
gowaku.App{
	Title:       "Editor",
	CustomFrame: true,
	Home: basic.Column{Children: []gowaku.Widget{
		gowaku.TitleBar{Child: menuBar},
		basic.Expanded{Child: page},
	}},
}
```

A nil `Child` shows the window title. The bar is 36 logical pixels high and takes no room in fullscreen. `examples/decorations` shows custom frames, icons, size limits, always on top, fullscreen and transparent windows.

## Menus and shortcuts

An `interact.Action` is a command: a label, a shortcut, whether it can run now, and an optional check mark. Build it once and hand the same value to the menu bar, context menus and toolbars. An action without `OnInvoke` is disabled: its menu items are grayed out and its shortcut does nothing.

```go
save := interact.Action{
	Label:    "保存(&S)",
	Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl},
	OnInvoke: s.save,
}
bar := overlay.MenuBar{Items: []overlay.MenuItem{
	{Action: interact.Action{Label: "ファイル(&F)"}, Items: []overlay.MenuItem{
		{Action: save},
		{Separator: true},
		{Action: interact.Action{Label: "終了(&X)", OnInvoke: gowaku.Quit}},
	}},
}}
```

- `&` marks the mnemonic; Japanese labels put it in parentheses, as in `ファイル(&F)`.
- `overlay.MenuBar` is an ordinary widget: put it at the top of a column or in a `TitleBar`. Alt alone or F10 moves the focus to it, and the arrow keys, Enter and Escape work as in Windows. The shortcuts of its items work anywhere in the window.
- On macOS the menu bar becomes the system menu bar (`NSMenu`), with the application menu and the Window menu added. Mnemonics are dropped there, and `Ctrl` in a shortcut is the Command key.
- `interact.Actions` runs the shortcuts of actions that are in no menu, when nothing in focus uses the key. A focused text field uses the keys that type, alone or with Shift, and Enter and Shift+Enter, so a shortcut of bare Space, a letter or Enter does not run while the user types; Ctrl and Alt shortcuts the field does not use still do, Ctrl+Enter among them (Command counts as Ctrl on macOS).
- `overlay.ContextMenu` opens a menu on a secondary click on its child, on a long press with a finger or pen, or with the menu key or Shift+F10. `overlay.ShowPopupMenu` opens one at a position from code.

```go
overlay.ContextMenu{
	Items: []overlay.MenuItem{{Action: copyAction}, {Action: pasteAction}},
	Child: row,
}
```

gowaku draws its menus itself, the same on every OS. The exceptions are the macOS menu bar and the menu of a tray icon, which the OS draws.

## Tray icons and notifications

`gowaku.ShowTray` puts an icon in the tray of the desktop. The OS draws its menu, which takes the same `overlay.MenuItem` values as a menu bar. Call it on the UI goroutine during `Run`, for example from a `Post` in `InitState`.

```go
tray, err := gowaku.ShowTray(ctx, gowaku.TrayOptions{
	Icon:       icon, // image.Image, drawn at 16 to 32 pixels
	Tooltip:    "Notes",
	OnActivate: func() { gowaku.WindowOf(ctx).Activate() },
	Menu:       []overlay.MenuItem{{Action: interact.Action{Label: "終了", OnInvoke: gowaku.Quit}}},
})
if errors.Is(err, gowaku.ErrNoTray) {
	// GNOME without the AppIndicator extension has no tray.
}
```

`tray.Update` replaces what it shows, and `tray.Close` removes it.

`gowaku.Notify(ctx, title, body)` shows a notification: a toast on Windows, the notification service on Linux, the notification center on macOS. A click on it runs `OnNotificationClick` of the first tray. Where the desktop shows no notifications, it returns `gowaku.ErrNoNotifications`. On macOS that includes every program that does not run from an .app bundle; see [Building and shipping](packaging.md).

| | Tray | Notifications |
|---|---|---|
| Windows | `Shell_NotifyIcon` | Balloon of the tray icon, shown as a toast |
| Linux | StatusNotifierItem over D-Bus | `org.freedesktop.Notifications` |
| macOS | `NSStatusItem` | `UNUserNotificationCenter`, only inside an .app bundle |

## Clipboard

Text fields handle copy and paste on their own. For other data, the clipboard holds text, an HTML fragment, an image and a list of files, each there or not. Read and write it on the UI goroutine, outside a Build:

```go
// What can be pasted?
if gowaku.ClipboardHas(ctx)&gowaku.ClipboardImage != 0 {
	c := gowaku.ReadClipboard(ctx, gowaku.ClipboardImage)
	s.SetState(func() { s.picture = c.Image })
}

gowaku.WriteClipboard(ctx, gowaku.ClipboardContent{Text: "gowaku", HTML: "<b>gowaku</b>"})
```

Another program owns the data, so a read waits for it, up to a fraction of a second on Linux. These functions go through the window of the tree, so in a `gowakutest.Tester` they reach its own clipboard.

## File dialogs

`gowaku.ShowFileDialog` shows the file dialog of the system in front of the window of `ctx`, and later calls `done` on the UI goroutine with the chosen paths, or nil if the user canceled. The zero `FileDialog` opens one existing file; `Save`, `Multiple`, `Directory`, `Name`, `Folder` and `Filters` change that.

```go
gowaku.ShowFileDialog(ctx, gowaku.FileDialog{
	Title:   "画像を開く",
	Filters: []gowaku.FileFilter{{Name: "Images", Patterns: []string{"*.png", "*.jpg"}}},
}, func(paths []string, err error) {
	if err != nil || paths == nil {
		return // gowaku.ErrNoFileDialog where the system has none
	}
	s.open(paths[0])
})
```

`WindowHandle.ShowFileDialog` does the same for code that holds a window and no context of it. Under a `gowakutest.Tester` the dialog is a fake one that the test answers with `tt.FileDialog().Choose(paths...)`. On Windows the dialog is modal. On Linux it comes from xdg-desktop-portal and the application goes on meanwhile; without a portal the result is `gowaku.ErrNoFileDialog`, and the application has to ask for a path another way. gowaku has no file dialog widget of its own. On macOS it is `NSOpenPanel` / `NSSavePanel`.

## Drag and drop

`interact.DragTarget` takes text or files dropped on its child: from the same window, from another gowaku window, or from another program such as Explorer, Finder, Nautilus or a browser. `interact.Draggable` drags data out of its child to any of those.

```go
interact.DragTarget{
	Kinds:   input.DataFiles,
	OnHover: func(over bool) { s.SetState(func() { s.lit = over }) },
	OnDrop: func(d input.DragData, pos geom.Offset) {
		s.SetState(func() { s.files = append(s.files, d.Files...) })
	},
	Child: dropArea,
}
```

```go
interact.Draggable{
	Data:      input.DragData{Files: []string{path}},
	Allowed:   input.DropCopy,
	OnDragEnd: func(effect input.DropEffect) {},
	Child:     row,
}
```

The data is text, a list of absolute file paths, or both. A move is the application's to carry out in `OnDragEnd`. Under the hood it is OLE on Windows, XDND on X11, `wl_data_device` on Wayland and `NSDraggingDestination` on macOS. `examples/dragdrop` shows both directions and a tray icon.

## Opening URLs

`gowaku.OpenURL` opens a URL with the application the system has for its scheme: a web page in the browser, `mailto:` in the mail program. It may be called from any goroutine, with or without `Run`, and returns once the system has the request. Anything but an absolute URL is `gowaku.ErrInvalidURL`.

OpenURL does not restrict the scheme, and some schemes run programs. For links you did not write, such as those in a document, allow only the schemes you mean to:

```go
if p, err := url.Parse(link); err == nil && (p.Scheme == "https" || p.Scheme == "http" || p.Scheme == "mailto") {
	err = gowaku.OpenURL(link)
}
```

It uses ShellExecute on Windows, NSWorkspace on macOS, and on Linux the OpenURI portal, then `xdg-open`. No shell sees the URL.

## Theme at run time

`gowaku.SetTheme(ctx, set, mode)` replaces the theme of every window, and `gowaku.CurrentTheme(ctx)` returns the current one. `ThemeSystem` follows the light and dark modes of the OS while running.

## Summary of goroutine rules

| Safe from any goroutine | UI goroutine only |
|---|---|
| `Post`, `Quit`, `OpenURL`, `OpenInspector` | `SetState`, `OpenWindow`, `ShowModal`, `TearOffWindow`, `WindowOf` and all `WindowHandle` methods, `ShowTray` and `Tray` methods, `Notify`, `ShowFileDialog`, the clipboard functions, `SetTheme`, `CurrentTheme` |

Design notes, in Japanese: [ADR 0046](../../ADR/0046-menus-and-shortcuts.md) (menus), [ADR 0047](../../ADR/0047-modal-windows.md) (modal windows), [ADR 0048](../../ADR/0048-custom-title-bar.md) (title bar), [ADR 0049](../../ADR/0049-clipboard.md) (clipboard), [ADR 0050](../../ADR/0050-drag-and-drop.md) (drag and drop), [ADR 0051](../../ADR/0051-tray-and-notifications.md) (tray), [ADR 0076](../../ADR/0076-close-request.md) (close requests).
