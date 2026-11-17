# Building and shipping

[日本語](../ja/packaging.md)

A gowaku application is one Go binary. It needs no C compiler to build and no runtime to install beside it: gowaku calls the OS and the GPU from pure Go, loading the system libraries at start. gowaku itself ships no installer, signing or updater; this guide covers the build, icons and the files each OS expects around the binary.

## Release builds

```bash
go build -trimpath -ldflags "-s -w" -o notes .
```

`-s -w` drops the symbol table and debug information, which makes the binary smaller (`examples/hello` is about 6 to 9 MB, depending on the OS). `-trimpath` keeps the paths of your machine out of it.

On Windows, add `-H windowsgui` so that the program opens no console window:

```bash
go build -trimpath -ldflags "-s -w -H windowsgui" -o notes.exe .
```

With `-H windowsgui`, the standard output and error go nowhere. Log to a file if you need the messages.

## Cross compiling

Every target builds from every host with `CGO_ENABLED=0`:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -H windowsgui" -o notes.exe .
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o notes .
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o notes .
```

In PowerShell, set the variables first: `$env:GOOS = "linux"; $env:CGO_ENABLED = "0"`.

| Target | Runs |
|---|---|
| `windows/amd64`, `windows/arm64` | Yes, Direct3D 11 |
| `linux/amd64`, `linux/arm64` | Yes, Vulkan on Wayland or X11 |
| `darwin/arm64` | Yes, Metal |
| `darwin/amd64` (Intel Mac) and other systems | Builds, but `gowaku.Run` returns `gowaku.ErrUnsupported` |

See [Platforms](platforms.md) for what each target supports.

## Icons

The window icon is set at run time. Embed the pictures with `go:embed` and give them to `App.Icon` (the first window), `WindowOptions.Icon` (other windows) or `WindowHandle.SetIcon`:

```go
//go:embed icon.png
var iconPNG []byte

func appIcon() []image.Image {
	img, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return nil // the default icon of the OS
	}
	return []image.Image{img}
}
```

One image, best 256 pixels square, is scaled down to every size the system wants. Give the same picture in several sizes to choose the smaller ones yourself. Where the icon shows:

- **Windows**: the title bar, the taskbar and the window switcher. The icon of the `.exe` file in Explorer is a resource compiled into the binary, which gowaku does not make; a resource compiler that writes a `.syso` file for the Go linker can add one.
- **macOS**: there is no icon per window. The last icon given shows in the Dock for the whole application. Inside an .app bundle, Finder shows the icon of the bundle (below).
- **Linux**: X11 shows the icon given. Wayland shows it only where the compositor has `xdg-toplevel-icon-v1`; elsewhere the compositor takes the icon of the desktop file that `App.AppID` names (below).

A tray icon is separate: `gowaku.TrayOptions.Icon`, drawn at 16 to 32 pixels.

## macOS: the .app bundle

A bare executable runs on macOS, but some things need a bundle. Notifications (`gowaku.Notify`) work only in an application with a bundle identifier, and return `gowaku.ErrNoNotifications` otherwise. A bundle also gives the application its name and icon in Finder and the Dock.

The layout:

```
Notes.app/
  Contents/
    Info.plist
    MacOS/
      notes          the binary from GOOS=darwin GOARCH=arm64
    Resources/
      notes.icns     optional
```

A minimal `Info.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>     <string>notes</string>
  <key>CFBundleIdentifier</key>     <string>com.example.notes</string>
  <key>CFBundleName</key>           <string>Notes</string>
  <key>CFBundlePackageType</key>    <string>APPL</string>
  <key>CFBundleShortVersionString</key> <string>1.0.0</string>
  <key>CFBundleIconFile</key>       <string>notes</string>
  <key>NSHighResolutionCapable</key> <true/>
</dict>
</plist>
```

`CFBundleIdentifier` is what notifications need. The first notification asks the user for permission. The Go linker signs `darwin/arm64` executables ad hoc, which lets them run on Apple Silicon. To give the application to other people, sign and notarize the bundle with Apple's tools.

## Linux: the desktop file

On Linux the desktop matches a window with its desktop file by the application id: the `app_id` on Wayland and the class of `WM_CLASS` on X11. Set `App.AppID` to the name of the desktop file without `.desktop`. Empty is `"gowaku"`.

```go
gowaku.Run(gowaku.App{Title: "Notes", AppID: "com.example.Notes", Home: notes{}})
```

Install `com.example.Notes.desktop` in `~/.local/share/applications` (or `/usr/share/applications`):

```ini
[Desktop Entry]
Type=Application
Name=Notes
Exec=/opt/notes/notes %F
Icon=com.example.Notes
Categories=Utility;
StartupWMClass=com.example.Notes
```

Put the icon under the icon theme, for example `~/.local/share/icons/hicolor/256x256/apps/com.example.Notes.png`. On Wayland this is where the title bar and the task switcher take the icon from.

What the binary loads at run time:

- Always: the Vulkan loader `libvulkan.so.1` and a Vulkan driver.
- Wayland: `libwayland-client.so.0`, `libwayland-cursor.so.0` and `libxkbcommon.so.0`.
- X11: `libX11.so.6`, and `libXi.so.6` if present, for pinches.
- Optional, through D-Bus: file dialogs (xdg-desktop-portal), the theme of the desktop, the tray, notifications and the screen reader bridge. Without a session bus these are off, and the application still runs.

Japanese text needs a Japanese font on the system, such as Noto Sans CJK JP; gowaku ships no fonts.

## Windows

Nothing to install beside the `.exe`. gowaku needs a GPU that runs Direct3D 11 (feature level 11.0 or later); it does not fall back to the software renderer WARP, and `Run` returns an error on a machine without a hardware GPU. Japanese text uses Yu Gothic, which Windows has.
