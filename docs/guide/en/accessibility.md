# Accessibility

[日本語](../ja/accessibility.md)

Every gowaku window keeps an accessibility tree next to its widget tree: what each control is, what it is called, its value and state, and what a screen reader may do with it. gowaku hands that tree to the screen reader of the OS: UI Automation on Windows, AT-SPI2 on Linux, NSAccessibility on macOS. The standard widgets describe themselves. Your part is to give controls their labels, to describe what you build from smaller parts, and to keep the keyboard working.

The same tree is what `gowakutest` searches when a test calls `Find` or `Tap`, so a well-labeled application is also easy to test.

## Labels

A control without a name is read as "button" and nothing more. Give every control a label:

- `control.Button` reads its `Label`. A button whose label is not enough, such as the same 削除 on every row of a list, adds a `Hint`, which a screen reader says after the label.
- `field.Input` reads its `Label`, or its `Placeholder` when it has no label. A field in a group row that has its name there needs neither.
- `basic.Icon` and `basic.Image` read their `Label`. Leave it empty for pictures that are only decoration.
- `interact.Pressable` reads its `Label`, `Role`, `Value` and `State`. A pressable without a label is named by all of its texts joined with spaces.

A text inside a control that says the same as the control's label is left out of the tree, so a button is not read twice.

A `basic.Text` is always read. Wrap text that only decorates, such as the initial in an avatar next to the name, in `interact.ExcludeSemantics`. It leaves its subtree out of the tree, as `aria-hidden` does, and keeps drawing it and letting it take the pointer. Do not put controls inside: Tab still reaches them, but a screen reader cannot see them.

```go
package app

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// person shows an avatar with an initial; only the name is read.
func person(ctx gowaku.BuildContext, initial, name string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
		interact.ExcludeSemantics{Child: basic.Container{
			Width: 32, Height: 32, Radius: 16, Color: th.Color.AccentTint,
			Child: basic.Center{Child: basic.Text{Content: initial, Style: th.Text.Headline}},
		}},
		basic.Text{Content: name},
	}}
}
```

```go
control.Button{Label: "削除", Hint: "請求書 2026-10", OnPress: remove}
```

## Describing your own widgets

`interact.Semantics` describes its child: a role, a label, a value, a hint, states, a live region, and the callbacks that do what a screen reader asks.

```go
package app

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// heading marks a title as a heading of level 1.
func heading(title string) gowaku.Widget {
	return interact.Semantics{
		Role:  semantics.RoleHeading,
		Level: 1,
		Label: title,
		Child: basic.Text{Content: title},
	}
}

// status announces its text whenever it changes, once the reader is idle.
func status(msg string) gowaku.Widget {
	return interact.Semantics{
		Live:  semantics.LivePolite,
		Label: msg,
		Child: basic.Text{Content: msg},
	}
}

// volume is a range that the screen reader can step up and down.
func volume(v float64, up, down func()) gowaku.Widget {
	return interact.Semantics{
		Role:       semantics.RoleSlider,
		Label:      "音量",
		Range:      semantics.Range{Min: 0, Max: 100, Value: v},
		OnIncrease: up,
		OnDecrease: down,
		Child:      basic.SizedBox{Width: 200, Height: 24},
	}
}
```

The roles are in package `semantics`: buttons, checkboxes, switches, radios and groups, tabs, lists, trees, grids, menus, dialogs, headings, links, sliders, steppers, progress indicators, combo boxes, tooltips and more (`semantics.Role`). The states are bits of `semantics.State`: `Checked`, `Mixed`, `Selected`, `Expanded`, `Collapsed`, `Disabled`, `ReadOnly`, `Multiline`, `Multiselectable`, `Invalid`, `Required`, `Obscured`. `Focusable` and `Focused` are set by gowaku; leave them alone.

The callbacks are `OnTap`, `OnIncrease`, `OnDecrease`, `OnSetValue`, `OnExpand` and `OnCollapse`. A nil one leaves its action out, and none runs while `State` has `Disabled`. They run on the UI goroutine, like a click.

Live regions (`LivePolite`, `LiveAssertive`) are read when their label or value changes, not when they first appear.

Package `semantics` also has the tree itself (`semantics.Node`) and the difference between two trees (`semantics.Compare`, `semantics.Diff`). Applications rarely need them; tests read the tree through `gowakutest`.

## Focus and the keyboard

Tab and Shift+Tab move the focus between the controls of a window, in the order they are painted. Space and Enter press a focused button. Arrow keys move within a group, a list, a menu or a grid.

- Clicking a control gives it the focus, as native Windows controls do. Clicking where nothing takes focus clears it, but dragging a scroll bar leaves the focus where it was.
- `interact.FocusTraversalOrder{Order: n}` changes the Tab order, as `tabindex` does in HTML: controls with an order above zero come first, lowest first, and the rest follow in painting order.
- `interact.Focus` lets any widget take the keyboard: `OnKey` gets the key presses, `Autofocus` takes the focus when the widget first appears.
- `Pressable` takes `SkipTab` for an area that only the pointer should reach.
- A modal dialog keeps the focus inside itself while it is open.

```go
interact.FocusTraversalOrder{Order: 1, Child: field.Search{OnChange: s.search}}
```

### The focus ring

The standard controls show a pale ring when they have the focus, but only when the keyboard gave it (like `:focus-visible` on the web): a mouse click does not light the ring. For your own controls, draw the same ring with `interact.FocusRing` and the `FocusVisible` flag of `Pressable`:

```go
interact.Pressable{Label: "Album", OnPress: open, Build: func(s interact.PressState) gowaku.Widget {
	return interact.FocusRing{Visible: s.FocusVisible, Radius: 8, Child: cover}
}}
```

`interact.Focus` reports the same flag through `OnShowFocus`. The ring is drawn outside the child and takes no room, so leave a few pixels around it.

## Reduced motion

When the user asks the OS for less animation, `basic.ReduceMotionOf(ctx)` is true. The standard widgets then jump instead of moving or resizing, and keep their short fades. Explicit tickers, such as a spinning progress ring, keep running. gowaku reads the setting on each OS and follows its changes:

- Windows: Settings > Accessibility > Visual effects > Animation effects.
- macOS: Reduce motion in the accessibility settings.
- Linux: `enable-animations` of `org.gnome.desktop.interface`, read through xdg-desktop-portal. Without a portal, and on KDE, motion is not reduced.

## Checking in tests

`gowakutest.Tester.Warnings` checks the current frame and reports, among other things:

- `WarnUnlabeled`: a button, checkbox, slider or text field that tells assistive technology nothing.
- `WarnContrast`: text whose color is too close to what is behind it, below 4.5:1, or 3:1 for large text, as WCAG 2 asks.

```go
package app

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

func TestLabels(t *testing.T) {
	tt := gowakutest.New(t, control.Button{Label: "保存", OnPress: func() {}}, gowakutest.Options{})
	for _, w := range tt.Warnings() {
		t.Error(w)
	}
	if _, ok := tt.Find("保存"); !ok {
		t.Errorf("no button named 保存:\n%s", tt.Dump())
	}
}
```

`Dump` prints the accessibility tree as a screen reader would see it, and `Options.ReduceMotion` stands for the OS setting. `gowaku-preview` runs the same checks on every preview. See [Testing and previews](testing-and-preview.md).

## Per OS

| | Windows | Linux | macOS |
|---|---|---|---|
| Bridge | UI Automation provider | AT-SPI2 over D-Bus (X11 and Wayland) | NSAccessibility |
| Screen readers verified | Narrator by hand, NVDA by its speech log | Orca by its debug log | VoiceOver tested by code only |
| Text fields | Text pattern: document, selection, caret; moves by character, word, line | Text and EditableText: the same range | Text, selection, lines, ranges |

The bridge does nothing until a screen reader asks, so an application without one pays nothing for the tree.

**Windows.** The provider answers the patterns Invoke, Toggle, Value, RangeValue, SelectionItem, ExpandCollapse, ScrollItem and Text. It raises focus, property, structure and live region events. Automated tests drive the windows through a UI Automation client written in Go.

**Linux.** The bridge connects to the accessibility bus when the desktop says a screen reader is on (`org.a11y.Status`), and otherwise waits for one to start. `GOWAKU_A11Y=1` forces the connection, and `NO_AT_BRIDGE=1` turns it off, as in GTK. Without a session bus there is no bridge. The tree was checked on X11 and on Wayland, and Orca was driven by keyboard under Xvfb and on a headless sway. On Wayland a node's position is relative to its window, as the protocol gives no screen coordinates. Live regions use `Object:Announcement`, which Orca 45 and later read.

**macOS.** Each node becomes an `NSAccessibilityElement`. Tests query the elements and their actions directly; reading with VoiceOver has not been done by hand yet.

Design notes, in Japanese: [ADR 0042](../../ADR/0042-accessibility-windows.md) (Windows, focus, reduced motion), [ADR 0043](../../ADR/0043-accessibility-linux.md) (Linux), [ADR 0085](../../ADR/0085-accessibility-macos.md) (macOS).
