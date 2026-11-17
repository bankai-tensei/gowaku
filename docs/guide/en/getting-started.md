# Getting started

[Guides](README.md) · [日本語](../ja/getting-started.md)

This guide installs gowaku, opens a window, adds a button that counts, and runs the samples in the repository.

## Requirements

- **Go 1.27.3 or later**, as `go.mod` asks.
- **No C compiler.** gowaku does not use cgo. It calls the OS and the GPU from pure Go, so `CGO_ENABLED=0` builds work, and each OS cross-compiles for the others.
- **One of these systems:**

| OS | GPU API | What must be installed |
|---|---|---|
| Windows | Direct3D 11 | A hardware GPU. The software adapter (WARP) is not used. |
| Linux | Vulkan | A Vulkan driver (`libvulkan.so.1` and Mesa or the vendor driver), and either X11 (`libX11.so.6`) or Wayland (`libwayland-client.so.0`, `libxkbcommon.so.0`). |
| macOS | Metal | Apple Silicon. On an Intel Mac, `gowaku.Run` returns `gowaku.ErrUnsupported`. |

On Linux, the libraries are opened when the program starts. You need the libraries a desktop already has, not their `-dev` packages. gowaku picks Wayland when `WAYLAND_DISPLAY` is set and X11 otherwise; `GOWAKU_BACKEND=x11` or `GOWAKU_BACKEND=wayland` forces one. See [Platforms](platforms.md) for the details and known limits of each system.

## Install

In your module:

```bash
go get github.com/bankai-tensei/gowaku
```

The Markdown view is a module of its own, so that the main module keeps its short list of dependencies. Add it when you need it:

```bash
go get github.com/bankai-tensei/gowaku/markdown
```

To try a local checkout of gowaku instead of a release, point your `go.mod` at it and let Go fill in the rest:

```bash
go mod edit -require=github.com/bankai-tensei/gowaku@v0.0.0 -replace=github.com/bankai-tensei/gowaku=../gowaku
go mod tidy
```

`go mod tidy` adds the modules gowaku depends on and their checksums; without it, the first build stops with "missing go.sum entry". It also drops the require of gowaku when no file of your module imports gowaku yet, so write the import first, as in the program below, and run it after.

## The first window

```go
package main

import (
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

type hello struct{ gowaku.StatelessBase }

func (hello) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: basic.Text{Content: "Hello, gowaku"}}
}

func main() {
	err := gowaku.Run(gowaku.App{Title: "hello", Width: 400, Height: 300, Home: hello{}})
	if err != nil {
		log.Fatal(err)
	}
}
```

```bash
go run .
```

What happens here:

- `hello` is a widget: a plain struct. Embedding `gowaku.StatelessBase` gives it the `Key` method every widget has, and `Build` returns what it looks like.
- `basic.Center` and `basic.Text` are standard widgets from `widgets/basic`. They are struct literals too, and their zero values work.
- `gowaku.Run` opens the window, shows `Home`, and blocks until the last window closes. The goroutine that calls it becomes the UI goroutine. Its error reports only a failed start, such as no usable GPU.

The window follows the light or dark mode of the OS. Nothing runs while nothing changes: an idle window draws no frames.

## A counter

A widget that remembers something is stateful. The widget itself stays a value; what it remembers lives in a `State`, which survives rebuilds.

```go
package main

import (
	"fmt"
	"log"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

type counter struct{ gowaku.StatefulBase }

type counterState struct {
	gowaku.StateBase
	n int
}

func (counter) CreateState() gowaku.State { return &counterState{} }

func (s *counterState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Center{Child: basic.Column{
		Size:    layout.MainMin,
		Spacing: theme.Space12,
		Children: []gowaku.Widget{
			basic.Text{Content: fmt.Sprintf("Count: %d", s.n), Style: th.Text.LargeTitle},
			control.Button{
				Label:   "Add one",
				Style:   control.ButtonPrimary,
				OnPress: func() { s.SetState(func() { s.n++ }) },
			},
		},
	}}
}

func main() {
	err := gowaku.Run(gowaku.App{Title: "counter", Width: 400, Height: 300, Home: counter{}})
	if err != nil {
		log.Fatal(err)
	}
}
```

- `counter` embeds `gowaku.StatefulBase` and returns a new state from `CreateState`.
- `counterState` embeds `gowaku.StateBase`, which gives it `SetState`, `Context` and the lifecycle methods.
- `SetState` runs the function that changes the state and schedules a rebuild. On the next frame, that state's `Build` runs again, and the rest of the tree is left alone.
- `basic.ThemeOf(ctx)` reads the theme of the window. `th.Text.LargeTitle` is a text style of the type scale, and `theme.Space12` a spacing token.
- `Size: layout.MainMin` makes the column as tall as its children, so that `Center` can center it. A `Column` takes all the height it may by default.

`Button` is in `widgets/control`. The widgets are split into packages by layer: `basic`, `interact`, `control`, `overlay`, `field`, `nav`, `collection` and `editor`. The [widget catalog](../../catalog/en/README.md) lists them all, with a picture and the code of each.

The guides write `gowaku.Widget` and `gowaku.BuildContext`; the catalog and the godoc of the widget packages write `core.Widget` and `core.BuildContext`. They are the same types: package `gowaku` gives the common names of `core` aliases, so a UI file needs no import of `core`.

## See it without a window

You do not need to open a window to see what you built. Package `gowakutest` lays out a widget and draws it on the CPU, so an ordinary test can save a screenshot. Put this next to the counter, in `main_test.go`:

```go
package main

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gowakutest"
)

func TestScreenshots(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt := gowakutest.New(t, counter{}, gowakutest.Options{Width: 400, Height: 300, Dark: dark})
		tt.Tap("Add one") // press the button by its label, as a user would
		name := "light.png"
		if dark {
			name = "dark.png"
		}
		tt.SaveScreenshot(name)
		for _, w := range tt.Warnings() {
			t.Error(w) // overflow, controls without a label, text of low contrast
		}
	}
}
```

`go test` writes `light.png` and `dark.png`. This is also the quickest way for a coding agent to look at its own work. [Testing and previews](testing-and-preview.md) explains the rest: typing, time, what each control reports, and live previews that redraw on every save.

The samples' `-png` flags use a helper inside the repository; your code uses `gowakutest` instead.

## Running the samples

The repository has small programs under [`examples/`](../../../examples/). Clone it and run them from its root:

```bash
git clone https://github.com/bankai-tensei/gowaku
cd gowaku
go run ./examples/hello
```

| Sample | Shows |
|---|---|
| `hello`, `counter` | The smallest programs. `counter` adds an implicit animation. |
| `todo` | A small application with a text field and a list. |
| `gallery` | Most widgets, as a settings window. |
| `form` | Text fields with Japanese input, a checkbox and buttons. Tab moves the focus. |
| `listview` | 10,000 rows in a list. `-measured` shows rows of wrapping text. |
| `jptext`, `tategaki`, `scripts` | Japanese typesetting and ruby, vertical text, emoji and right-to-left text. |
| `navigator` | Pages that slide in, with a Hero flight. |
| `menus`, `tabs`, `multiwindow` | A menu bar and shortcuts, tabs that tear off into windows, windows that share state. |
| `rotate`, `shader`, `glass` | Transforms with an animation controller, custom shaders, a blurred backdrop. |
| `dragdrop`, `zoom`, `decorations` | Drag and drop, tray and notifications; pinch and pen; window icon, size limits and transparency. |
| `notes` | A larger application: plain text notes with search, tabs, vertical writing and a command palette. |

## Where to go next

- [Concepts](concepts.md): widgets, elements and render objects, state, `BuildContext`, keys, and why an idle window draws nothing.
- [Layout](layout.md): how sizes are decided, and the layout widgets.
- [Theme](theme.md): the accent color, light and dark, text styles and tokens.
- [Motion](motion.md): implicit animations, controllers and springs.
- [Testing and previews](testing-and-preview.md): run a widget tree under `go test` without a window.
- The [widget catalog](../../catalog/en/README.md) for every widget.
