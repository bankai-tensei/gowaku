# gowaku design specification

[日本語](DESIGN.ja.md)

`widgets` and `theme` are built to this specification (ADR 0011). For examples, see `examples/gallery` and the [widget catalog](catalog/en/README.md). Only the continuous-curvature corners (§4) are not implemented.

---

## 1. Position

gowaku has its own default design. It has no name yet.

In one line: **a quiet desktop tool, made for Japanese text.** It borrows "subtraction" and the way materials are built from Apple's aesthetic, and its typography starts from Japanese.

### What we borrow from Apple

| What we borrow | How it looks in gowaku |
|---|---|
| The courage not to use color | Surfaces are neutral. There is a single accent color, used only for things you can operate and for where you are now |
| Hairline rules | Separators are 1px translucent lines, not shadows |
| Inset groups | Settings and forms place rows in a rounded card, and separators stop short of the left edge |
| Two-layer shadows | A thin contact shadow plus a wide, faint shadow. Never one dark blur |
| Physical feel | Pressing sinks to 0.97, the switch knob is a white ball with a shadow, motion uses springs |
| Hierarchy through weight | Weight and ink strength create contrast before size does |
| Focus ring | The keyboard position, and only that, is shown clearly with a soft outer ring |

### What we do not borrow

- The San Francisco (SF) font. Its license does not allow it either.
- Anything meant to look like an Apple product: traffic-light buttons, the macOS title bar, icons, Liquid Glass-style effects. The default title bar is the one of the OS (the title bar gowaku draws is opt-in, ADR 0048).
- Materials that assume blur. The default design works with opaque surfaces. Blur is available as opt-in (`BackdropFilter`, `Dialog{Glass: true}`: the background is blurred at σ 20 and a `Glass` surface is laid on top. The see-through effect is kept subtle, and text legibility comes first. ADR 0044).

### What is gowaku's own

- **Colorless surfaces**: the base is `f5f5f7`, which looks achromatic. Never beige or cream. Those look like typical AI-generated screens.
- **Typography based on Japanese text**: sizes and line heights are chosen for the legibility of Japanese (§3).
- **Indigo accent**: the default color is a strongly blue indigo (ai). It can be replaced with any color.

---

## 2. Color

HCT is used as is. The 36 Material color roles are dropped and narrowed to about 15 colors named by use. The user picks one accent color, and the rest is derived.

```go
Theme: theme.FromAccent(geom.Hex(0x2f5fd0))   // indigo when omitted
```

| Token | Light | Dark | Use |
|---|---|---|---|
| `Canvas` | N 96.5 | N 10 | Window background |
| `Sidebar` | N 93 | N 14 | Sidebar |
| `Card` | White | N 15 | Groups, cards |
| `Raised` | White | N 20 | Dialogs, menus |
| `Glass` | White 85% | N 20 at 78% | Glass dialog (`Dialog{Glass: true}`). Ink2 reaches 4.5:1 on top of any surface under the scrim (ADR 0044) |
| `Field` | White | N 19 | Input fields |
| `Control` | White | White 14% | Normal buttons |
| `Ink` | N 13 | N 95 | Body text |
| `Ink2` | N 42 | N 72 | Secondary text. At least 4.5:1 on every surface (ADR 0041) |
| `Ink3` | N 62 | N 50 | Placeholders, disabled, decoration. Deliberately below 4.5:1, so do not use it for text that carries information such as times or counts; use `Ink2` |
| `Hair` | Black 9% | White 10% | Lines |
| `Fill` | Black 5% | White 8% | Subtle fills, segment background |
| `FillStrong` | Black 9% | White 14% | Switch when off, tracks |
| `Accent` | A 45 | A 45 | Fills. White text sits on top, at 4.5:1 also under hover (ADR 0123) |
| `AccentInk` | A 41 | A 74 | Text, lines, focus ring |
| `AccentTint` | A 45 at 12% | A 62 at 22% | Selected row. `Ink` reads on it; `Ink2`, `AccentInk`, `Danger` and `Success` fall below 4.5:1 there, so what sits on it takes the colors of `Colors.OnTint`, which mixes each toward `Ink` by the least that reads (ADR 0138) |
| `Danger` | Red 41 | Red 72 | Destructive actions, errors |
| `Success` | Green 41 | Green 72 | Positive states, such as something finished or a break in progress |
| `Warning` | Amber 70 | Amber 76 | What needs attention before it goes wrong, such as the icon and border of `SeverityWarning` of `InfoBar`. For icons, dots, borders and tints, not for text (ADR 0124) |

N is the neutral palette (hue 235, chroma 3.5). Red, green and amber are hues 25, 145 and 60. A is a palette with the accent's hue and chroma max(seed, 40). The numbers are tones.

### Why these colors

- **The neutral chroma must not go near 0.** HCT's achromatic point is around hue 209, chroma 2.8. Setting chroma to 0.8 actually shifts toward pink.
- **The base color was chosen by drawing four candidates side by side.** The warm `f6f5f4` looked beige and was rejected. Pure gray `f5f5f5` looks slightly muddy next to a white card, and going as blue as `f4f5f9` looks like a SaaS admin panel. Hue 265 leans purple. `f5f5f7` gives no sense of color.
- **One accent color is not enough.** In dark mode, a color bright enough to read as text makes white text unreadable when used as a fill. So there is one for fills and one for text.
- Changing the seed to green or orange does not break anything. Surfaces are neutral, so the seed only affects controls.

---

## 3. Text

The default body size is 13px. Yu Gothic on Windows is thin at 400, so labels use 500. Line heights are set in pixels, and all of them are even, so elements stacked below stay on the grid.

| Role | Size | Weight | Line height |
|---|---|---|---|
| `LargeTitle` | 24 | 600 | 32 |
| `Title` | 16 | 600 | 24 |
| `Headline` | 13 | 600 | 20 |
| `Body` | 13 | 400 | 20 |
| `Control` | 13 | 500 | 20 |
| `Caption` | 12 | 400 | 18 |
| `Section` | 12 | 500 | 18 |
| `Reading` | 15 | 400 | 26 |
| `Code` | 13 | 400 | 20 |

Line heights are fixed values based on Japanese text. Text sizes are limited to 9 roles.

`Code` is the only role with a family of its own: a monospaced font for source code, logs and values to copy, the default of `editor.Editor` and of code in markdown. Its family is a list that every OS reads in order, taking the first it has: Cascadia Mono and Consolas on Windows, SF Mono and Menlo on macOS, DejaVu Sans Mono, Noto Sans Mono and Liberation Mono on Linux (`theme.MonoFamilies`). Japanese inside code falls through to the Japanese families.

---

## 4. Shape and dimensions

The scale uses even numbers only. Even numbers become whole physical pixels at 100%, 150% and 200%. The values are `theme.Space2` to `theme.Space28`; do not write raw numbers in code.

| | Value |
|---|---|
| Corner radius | Small 4 (checkbox) / standard 8 (buttons, input fields, rows) / large 12 (groups) / extra large 16 (dialogs) |
| Control height | Small 24 / **standard 30** (buttons, input fields, segments, sidebar rows) / large 38 |
| Group row height | 44 or more |
| Sidebar row with a `Detail` | 46: the two lines (20 and 18) and 4 above and below |
| Spacing | 2, 4, 6, 8, 10, 12, 16, 20, 24, 28 |
| Which spacing where | Page 28, between sections 24, left and right inside a card 16, between controls 8, heading to body 6 |
| Sidebar width | 208, hairline included (`Sidebar.Width`) |

### Snapping to physical pixels

`render` aligns rounded rectangles to physical pixels. The same length becomes the same physical length wherever it is, so line widths stay even. Anything under 2 physical pixels is rounded down (1px at 150% is 1 physical pixel).

Not snapped: shapes being scaled (they must move by less than a pixel) and circles (so a knob does not move in steps). Only the border width stays even during scaling.

**Continuous-curvature corners** (the key to Apple's rounded corners) are not implemented. The current SDF uses circular arcs, and at a radius of about 8 the difference is small. We expect that changing the corners of `roundedBoxDistance` in the shader to a p-norm would work without changing the `Instance` format, but this is not verified. It will be tried in a later step.

---

## 5. Components

**Buttons** come in 3 levels plus a destructive one, 4 kinds in all. They are one type, `Button{Label, Style}`, and the zero value is a normal button.

| `Style` | Look | When to use |
|---|---|---|
| `ButtonNormal` (zero value) | White background + line + contact shadow | Default |
| `ButtonPrimary` | Accent fill | Only one per screen |
| `ButtonQuiet` | No background, accent text | Secondary actions |
| `ButtonDanger` | No background, red text | Delete |

Buttons without a background use 8 for left and right padding. With 16, the gap to the neighbor looks too wide.

**Input fields**: the label goes above (`field.Input.Label`) or to the left of the row (`GroupRow`). No floating labels. On focus the line turns accent and a faint 3px ring appears outside. `layout.RenderRing` draws the ring as a line, so it does not affect the fill inside.

**Switch**: 38×22, a white knob of 18 with a two-layer shadow. Pressing stretches the knob sideways. **Slider**: track 4, white knob 18, grows slightly when the pointer is over it. **Checkbox**: 16, corner radius 4.5.

**New components**

- `SegmentedControl`: 2 to 4 choices, such as switching appearance. The selected plate slides on a spring.
- `Group` and `GroupRow`: an inset list. Name on the left, control on the right.
- `Sidebar` and `SidebarItem`: selection is an `AccentTint` surface, and the chosen row sets its content in the colors of `Colors.OnTint`. A `Detail` is a second line in `Caption` and `Ink2`, cut to one line.
- `Steps`: where a flow of steps is. Circles of 24 and names, joined by lines 2 thick. Done steps and the current one have `Accent` circles, the steps ahead `Fill` circles and `Ink2` names, and only the name of the current step is bold.

**Dialog**: width 340, padding 24, corner radius 16, `Raised`, left-aligned, two-layer shadow. The scrim is black 25%. With `Glass: true`, the surface becomes `Glass` and what is behind is blurred at σ 20.

### States are the same for every component

| State | How it shows |
|---|---|
| hover | Background or line changes in 80ms. All of Switch, Checkbox, Segmented, Sidebar, Slider, Input, Button |
| press | Sinks, stretches or darkens on a spring. All except Input |
| focus | A faint 3px ring. Custom components built with `Pressable` show the same ring when `PressState.Focused` is set |
| disabled | Drawn with the `Fill` and `Ink3` tokens. The whole component is not made translucent |

The `widgets` tests (`TestHover_MovesEveryControl`, `TestPress_MovesEveryControl`, `TestDisabled_UsesTokensNotOpacity`, `TestControls_ShareOneHeight`, `TestStacking_StaysOnTheGrid`) keep these promises.

---

## 6. Shadows

Light comes from directly above. Two layers: a contact shadow and a wide shadow.

| Level | Near shadow | Far shadow | Use |
|---|---|---|---|
| 1 | y 0.5 / blur 1 / 5% | None | Normal button |
| 2 | y 0.5 / blur 1 / 8% | y 1 / blur 2–3 / 16% | Segment plate, switch knob |
| 3 | y 2 / blur 4 / 8% | y 24 / blur 48 / 16% | Dialog |

In dark mode shadows show less, so steps are made with surface brightness (`Card` → `Raised`) plus a line.

---

## 7. Motion

Instead of ripples, gowaku's signature is **physical feel made with springs**. `motion.SpringSimulation` already exists, so almost no new machinery is needed.

| Motion | Value |
|---|---|
| hover | 80ms. Linear |
| Press | **Only the background** sinks to 0.97 and the fill changes. Spring (fast, no bounce) |
| Release | Returns on the same spring. Velocity carries over, so rapid presses do not jump |
| Switch, segment | Spring (slight bounce) |
| Dialog | 0.96 → 1 plus fade. 280ms `motion.SpringOut`. Closing is quick, 120ms |
| Color change | 150ms |

The press motion is applied only to the background, with `Decoration.Sink`. Shrinking the text too could make glyphs jump near 0.97 because of glyph pixel-size quantization. The text stays in place.

---

## 8. Effect on the API

- "Good-looking with no effort" means **it looks right when you specify nothing**. `Button{Label: "保存"}` ("Save") alone gives a finished look.
- `theme` has `Colors`, an 8-role `TextTheme`, `Radii` and `Lift`. It has HCT, `TonalPalette`, `Set`, `ThemeMode` and following the OS setting.
- There is one button, `Button`, and one input field, `field.Input`.
