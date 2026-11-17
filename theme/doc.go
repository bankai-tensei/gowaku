// Package theme holds the tokens of the design of docs/DESIGN.md: the colors that
// follow from one accent, the type scale, the corners and the shadows. Below them
// lie the HCT color space and tonal palettes.
//
// The tokens, by name:
//
//   - spaces, in logical pixels: Space2, Space4, Space6, Space8, Space10,
//     Space12, Space16, Space20, Space24 and Space28, and ControlHeight (30);
//   - heights for Colors.Shadows (type Lift): LiftNone, LiftControl, LiftKnob
//     and LiftFloat;
//   - corners (Theme.Radius, type Radii): Check, Control, Group and Dialog;
//   - text styles (Theme.Text, type TextTheme, each a text.Style):
//     LargeTitle, Title, Headline, Body, Control, Caption, Section, Reading
//     and Code;
//   - colors (Theme.Color, type Colors), named for what they do.
//
// theme sits above text, paint (for the type of its shadows) and geom, and
// imports nothing else from gowaku. It must not import core, layout, widgets,
// gpu or platform.
//
// A Theme is built once, not per frame, so the color math favors precision and
// clarity over speed and works in float64 instead of the float32 used elsewhere.
//
// The HCT math is a port of material-color-utilities, Copyright 2021 Google LLC,
// under the Apache License 2.0. The license is in
// LICENSE.material-color-utilities in this directory.
//
// The only package-level variable is the lazily computed result of Default. It is
// written once under a sync.OnceValue and never changes afterwards.
package theme
