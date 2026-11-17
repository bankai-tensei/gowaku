# Layout

[Guides](README.md) · [日本語](../ja/layout.md)

This guide explains how gowaku decides the size and place of each widget, walks through the layout widgets, and shows how to write a layout of your own. The widgets here are in `widgets/basic` unless another package is named; the alignment and axis constants are in `layout`, and the geometry types in `geom`.

## The constraints protocol

Layout is one pass down the tree and back:

- **Constraints go down.** A parent tells each child the sizes it may take, as a `geom.Constraints{MinW, MaxW, MinH, MaxH}`. A maximum of `geom.Inf` means unbounded.
- **Sizes go up.** The child picks a size within those limits and returns it.
- **The parent places the child.** Only after the child has its size does the parent decide where it goes.

A child cannot pick its own position, and it cannot be larger than its parent allows. Two kinds of constraints come up all the time:

- **Tight**: the minimum equals the maximum, so the child has exactly one size. The window gives its root widget tight constraints of the window's size.
- **Loose**: the minimum is zero, so the child may be smaller. `Center` and `Align` loosen what they get before they pass it on.

Lengths are in logical pixels. gowaku scales them to the display.

`layout.Constraints`, which a `layout.Delegate` and `basic.LayoutBuilder` receive, is the same type as `geom.Constraints`, so its helpers are in `geom`: `geom.Tight(size)` and `geom.Loose(size)` make constraints, and on a value `c`, `c.Biggest()` is the largest size allowed, `c.Constrain(size)` clamps a size into it, `c.Loosen()` drops the minimums, `c.Deflate(insets)` takes insets off, and `c.IsTight()` reports one allowed size. There is no `layout.Tight`.

## Boxes

| Widget | Does |
|---|---|
| `Padding{Insets, Child}` | Insets the child. `geom.All(16)` and `geom.Symmetric(16, 8)` make the insets; `basic.Pad(16, child)` is short for the first. |
| `Center{Child}` | Places the child in the middle of the space it gets. |
| `Align{Alignment, Child}` | Places the child at an alignment. |
| `SizedBox{Width, Height, Child}` | Forces a width, a height or both. A zero length leaves that axis free. |
| `ConstrainedBox{MinWidth, MaxWidth, MinHeight, MaxHeight, Child}` | Adds limits. A zero maximum means no limit. |
| `Container{...}` | Size, padding, color, gradient, border, corners and shadows in one widget. |
| `DecoratedBox{Decoration, Child}` | Paints a `layout.Decoration` behind the child. |
| `AspectRatio{Ratio, Child}` | Sizes the child to a width over height of `Ratio`, such as `16.0 / 9`: all the width allowed, or all the height where that is too tall. Zero is 1, a square. |

`AspectRatio` has no say under tight constraints, which fix both lengths: the window's, or those of `CrossStretch` and `Expanded`. Put it in a `Center` or an `Align` there, and it becomes the largest rectangle of that shape that fits:

```go
func player(video gowaku.Widget) gowaku.Widget {
	return basic.Center{Child: basic.AspectRatio{Ratio: 16.0 / 9, Child: video}}
}
```

An `Alignment` runs from 0 to 1 on each axis: `{X: 0, Y: 0}` is the top left and `{X: 0.5, Y: 0.5}` the middle. This is **not** the -1..1 of Flutter. Use the named values, `layout.AlignTopLeft` through `layout.AlignBottomRight`, such as `layout.AlignCenterLeft`.

## Rows and columns

`Row` lays out its children from left to right, `Column` from top to bottom. Both have the same fields:

- `Main` places the children along the axis: `layout.MainStart`, `MainEnd`, `MainCenter`, `MainSpaceBetween`, `MainSpaceAround`, `MainSpaceEvenly`.
- `Cross` places each child across it: `layout.CrossStart`, `CrossEnd`, `CrossCenter`, `CrossStretch`.
- `Size` is the length along the axis. `layout.MainMax`, the zero value, takes all the length allowed. `layout.MainMin` shrinks to the children.
- `Spacing` is the gap between children.

Because a `Row` or `Column` takes all the room by default, one inside a `Center` fills it instead of sitting in its middle. Write `Size: layout.MainMin`, or use `basic.VStack(...)` and `basic.HStack(...)`, which do that for you. `Flex` is the same widget with an `Axis` field.

`VStack` and `HStack` also put `theme.Space8` between their children, the gap between controls in the design guide, as a stack of SwiftUI keeps its children apart. For any other gap, write the `Column` or `Row` with its `Spacing`: `theme.Space24` between the sections of a page, zero for none.

```go
basic.VStack(title, field, button) // 8 between them
basic.Column{Size: layout.MainMin, Spacing: theme.Space24, Children: []gowaku.Widget{general, advanced, reset}}
```

### Sharing the free space

`Expanded` and `Flexible` give a child of a `Row` or `Column` a share of the room the other children leave. `Flex` is the weight, where zero counts as one. An `Expanded` child must fill its share; a `Flexible` child may stay smaller.

```go
func toolbar(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
		basic.Expanded{Child: basic.Text{Content: "Title", Style: th.Text.Headline}},
		basic.Flexible{Child: basic.Text{Content: "A note that may be cut short"}},
		control.Button{Label: "Share"},
	}}
}
```

A typical page puts a header and a footer around an `Expanded` body:

```go
func page(ctx gowaku.BuildContext, items []string) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Column{
		Spacing: theme.Space12,
		Children: []gowaku.Widget{
			basic.Text{Content: "Inbox", Style: th.Text.LargeTitle},
			basic.Expanded{Child: collection.List{
				ItemCount:  len(items),
				ItemExtent: 36,
				ItemBuilder: func(i int) gowaku.Widget {
					return basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.Text{Content: items[i]}}
				},
			}},
			basic.Row{
				Main:    layout.MainEnd,
				Spacing: theme.Space8,
				Children: []gowaku.Widget{
					control.Button{Label: "Cancel"},
					control.Button{Label: "OK", Style: control.ButtonPrimary},
				},
			},
		},
	}}
}
```

## Stacks

`Stack` paints its children on top of each other, the last on top. Children that are not positioned are placed by `Alignment`, and the stack is as large as the largest of them, or as large as allowed with `Fit: layout.StackExpand`.

- `Positioned` puts a child at a fixed spot, measured from the top left, or from the right and bottom edges with `AnchorRight` and `AnchorBottom`. `Width` and `Height` force its size.
- `PositionedFill` stretches a child over the whole stack, less `Insets`. Use it for a background.

Positioned children do not count toward the size of the stack.

A press goes to the top child that is hit there. The layout widgets (`Align`, `Center`, `Padding`, `Row`, `Column`, `Stack`, `SizedBox`, `PositionedFill` and the like) are hit only where one of their children is, so the empty space of a full-size layer lets the pointer through to the layer under it. A `Container` with a `Color`, a `Gradient`, a `Border` or `Shadows` is hit over its whole box, as are text, images and the widgets that take the pointer (`GestureDetector`, `Pressable`, `Listener`). `IgnorePointer` lets the pointer through its whole child.

```go
func card(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.SizedBox{Width: 240, Height: 140, Child: basic.Stack{Children: []gowaku.Widget{
		basic.PositionedFill{Child: basic.Container{Color: th.Color.Card, Radius: th.Radius.Group}},
		basic.Positioned{Left: 16, Top: 16, Child: basic.Text{Content: "Album", Style: th.Text.Title}},
		basic.Positioned{Right: 12, Bottom: 12, AnchorRight: true, AnchorBottom: true,
			Child: control.Button{Label: "Play", Style: control.ButtonPrimary}},
	}}}
}
```

## Wrap and grid

`Wrap` is a row that starts a new row when the next child does not fit. It suits tags and chips. `Spacing` is the gap within a run and `RunSpacing` the gap between runs.

```go
func tags(names []string) gowaku.Widget {
	chips := make([]gowaku.Widget, len(names))
	for i, n := range names {
		chips[i] = control.Chip{Label: n}
	}
	return basic.Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: chips}
}
```

`Grid` lays out tiles in columns of the same width. `Columns` fixes their number; when it is zero, the grid fits as many as `MinColumnWidth` allows, so it gains a column as the window grows. A tile is `TileHeight` tall, or the column width over `AspectRatio` plus `ExtraHeight`. A grid is as tall as its rows, so it goes inside a vertical `Scroll`:

```go
func covers(n int) gowaku.Widget {
	tiles := make([]gowaku.Widget, n)
	for i := range tiles {
		tiles[i] = basic.Text{Content: fmt.Sprintf("Cover %d", i+1)}
	}
	return basic.Scroll{Child: basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Grid{
		MinColumnWidth: 160,
		Spacing:        theme.Space12,
		RunSpacing:     theme.Space12,
		AspectRatio:    1,
		ExtraHeight:    40,
		Children:       tiles,
	}}}
}
```

`Grid` builds every child. For thousands of tiles, use `collection.GridView`, which builds only those in view.

## Scrolling

- `Scroll{Child}` shows a child taller than itself, moved with the wheel or the scroll bar that appears over its edge. `Horizontal: true` scrolls sideways. `Reverse: true` starts the scroll range at the right end, or the bottom: the view opens there and short content sits against that end, as for vertical Japanese text or a log. A `ScrollController` in `Controller` reads and sets the offset from code. Every child is built and laid out, so it suits a page of content, not ten thousand rows.
- `collection.List` is the list view. It builds only the rows near the view, so the number of rows does not matter. Rows have the same height, `ItemExtent` (48 by default), unless `ItemExtentOf` returns a height per row, or `MeasureItems` lets each row be as tall as it lays itself out. A `collection.ListController` in `Controller` moves the list to a row or to its end, and reads whether the view is at the end. `StickToEnd: true` keeps the view at the newest row of a log or a chat until the user scrolls up, and `OnScroll` reports each change of the position.

```go
collection.List{
	ItemCount:   10000,
	ItemExtent:  52,
	ItemBuilder: func(i int) gowaku.Widget { return row(i) },
}
```

A header inside a list scrolls away with the rows. To keep it in place, put it above the list and the list in an `Expanded`, as in the page above. The [data widgets](../../catalog/en/data.md) of the catalog, such as `collection.Table` and `collection.Tree`, scroll the same way.

## Unbounded space

Some widgets need a bounded length to make sense, and panic with a message that says how to fix it when they do not get one:

- `Scroll`, `collection.List` and `Grid` take all the room they are given. Inside a `Column`, which offers unbounded height to children that are not flexible, wrap them in an `Expanded` or a `SizedBox`.
- An `Expanded` or `Flexible` in a `Row` or `Column` whose length is unbounded, as inside a `Scroll` along the same axis, has no free space to share.
- `CrossStretch` with an unbounded cross axis, as a `Row` inside a vertical `Scroll`, has nothing to stretch to.

The usual fix is an `Expanded` around the scrolling widget, or a `SizedBox` that gives it a length. See [ADR 0026](../../ADR/0026-unbounded-room.md) (Japanese).

## Building for the room you have

A widget builds before the layout, so it does not know its own size. There are two ways to make what you build depend on the room.

**The size of the window.** `basic.WindowSizeOf(ctx)` is the content area of the window in logical pixels, the `Size` of `basic.MediaQueryData`. Use it for decisions about the whole window, such as folding a sidebar away when the window is narrow. When the window is resized, only the widgets that read the size rebuild, and an idle window costs nothing:

```go
func shell(ctx gowaku.BuildContext, sidebar, content gowaku.Widget) gowaku.Widget {
	if basic.WindowSizeOf(ctx).W < 700 {
		return content
	}
	return basic.Row{Children: []gowaku.Widget{basic.SizedBox{Width: 220, Child: sidebar}, basic.Expanded{Child: content}}}
}
```

`basic.MediaQueryOf(ctx).Size` is the same value, but `MediaQueryOf` also rebuilds on a resize; a widget that needs only another field reads it with `basic.ReduceMotionOf` or `basic.LocaleOf`.

**The room of a widget.** When the room is a part of the window, such as a pane whose width the user drags, or a card that may be shown anywhere, use `LayoutBuilder`. Its `Content` gets the constraints its parent gives it:

```go
func panes(list, detail gowaku.Widget) gowaku.Widget {
	return basic.LayoutBuilder{Content: func(ctx gowaku.BuildContext, c layout.Constraints) gowaku.Widget {
		if c.MaxW < 600 {
			return basic.Column{Children: []gowaku.Widget{basic.Expanded{Child: list}, basic.Expanded{Child: detail}}}
		}
		return basic.Row{Spacing: theme.Space16, Children: []gowaku.Widget{
			basic.SizedBox{Width: 240, Child: list},
			basic.Expanded{Child: detail},
		}}
	}}
}
```

- `Content` runs again only when the constraints change, when the parent rebuilds the `LayoutBuilder`, or when something it reads from `ctx`, such as the theme, changes. Frames that lay out with the same constraints build nothing.
- The `LayoutBuilder` is as large as what `Content` returns, within the constraints.
- It learns the constraints when the layout reaches it. The frame then builds `Content` and lays out again before it paints, as `collection.List` does when it learns its height. A `LayoutBuilder` inside the `Content` of another adds such a round, and a frame runs only a few, so deep nesting can show the inner content a frame late.
- To arrange children by their sizes, rather than to choose what to build, write a delegate as below.

## A custom layout

When no standard widget arranges children the way you want, write a `layout.Delegate` and hand it to `basic.CustomLayout`. The delegate gets the constraints and the render objects of the children. It lays out each child with `core.LayoutChild`, places it with `SetOffset`, and returns its own size:

```go
// diagonal places each child a step further right and down.
type diagonal struct{ step float32 }

func (d diagonal) Layout(c layout.Constraints, children []core.RenderObject) geom.Size {
	var size geom.Size
	for i, child := range children {
		sz := core.LayoutChild(child, c.Loosen())
		off := geom.Offset{X: float32(i) * d.step, Y: float32(i) * d.step}
		child.SetOffset(off)
		size.W = max(size.W, off.X+sz.W)
		size.H = max(size.H, off.Y+sz.H)
	}
	return c.Constrain(size)
}

func staircase() gowaku.Widget {
	return basic.CustomLayout{Delegate: diagonal{step: 24}, Children: []gowaku.Widget{
		basic.Text{Content: "One"}, basic.Text{Content: "Two"}, basic.Text{Content: "Three"},
	}}
}
```

Rules for a delegate:

- Lay out children only with `core.LayoutChild`, never by calling their `Layout`. It skips a child whose constraints did not change.
- Return a size within `c`. `c.Constrain` clamps one; `basic.CustomLayout` clamps the result as well.
- Layout runs on every frame that needs it. Do not allocate in `Layout`.
- Each rebuild hands the render object the new delegate and lays it out again, so a delegate can be a small value such as `diagonal{step: 24}`.

For full control over painting and hit testing as well, write a render object; see [Concepts](concepts.md#render-objects).

## Further reading

- The [layout page](../../catalog/en/layout.md) of the widget catalog, with a picture of each widget.
- [Theme](theme.md) for the spacing tokens such as `theme.Space16`.
