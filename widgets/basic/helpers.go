package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
)

// VStack returns a Column of children that is as tall as they are, lined up
// at the start (the left), not centered as in SwiftUI, with theme.Space8
// between them: a stack of SwiftUI keeps its
// children apart by default, and 8 is the gap DESIGN.md gives between
// controls. Inside a Center it stays in the middle. Write a Column for
// anything else: another Spacing (Space24 between sections, zero for none),
// the children centered (Cross: layout.CrossCenter), or all the height, for an
// Expanded child for instance.
func VStack(children ...core.Widget) Column {
	return Column{Size: layout.MainMin, Spacing: theme.Space8, Children: children}
}

// HStack returns a Row of children that is as wide as they are, lined up at
// the top, with theme.Space8 between them. Write a Row for another Spacing, to
// align the children another way or to take all the width.
func HStack(children ...core.Widget) Row {
	return Row{Size: layout.MainMin, Spacing: theme.Space8, Children: children}
}

// Layers returns a Stack of children.
func Layers(children ...core.Widget) Stack { return Stack{Children: children} }

// Pad returns child with the same padding on every edge.
func Pad(all float32, child core.Widget) Padding { return Padding{Insets: geom.All(all), Child: child} }
