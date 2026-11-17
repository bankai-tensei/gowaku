// Package gowaku is a GPU-native desktop UI library for Go that needs no cgo.
//
// An application describes its UI with value-type widgets and hands the root to
// Run. See docs/ARCHITECTURE.md for the design.
//
// # What this package holds
//
// This package is the entry point of an application: App and Run, the windows
// (OpenWindow, ShowModal, WindowOf, TearOffWindow), the services of the OS
// (Post, Quit, Tray, TitleBar, the clipboard and the file dialogs), the theme
// while running (SetTheme), and the inspector. It also names the few types of
// package core that every widget of an application writes, such as Widget,
// BuildContext and StatefulBase, so that a UI file needs no import of core.
//
// The widgets are in the packages under widgets, which an application imports
// directly, each for its layer (docs/ADR/0081-widget-layers.md):
//
//	import (
//		"github.com/bankai-tensei/gowaku"
//		"github.com/bankai-tensei/gowaku/layout"
//		"github.com/bankai-tensei/gowaku/theme"
//		"github.com/bankai-tensei/gowaku/widgets/basic"
//		"github.com/bankai-tensei/gowaku/widgets/control"
//	)
//
//	func header(ctx gowaku.BuildContext, title string, onAdd func()) gowaku.Widget {
//		th := basic.ThemeOf(ctx)
//		return basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
//			basic.Expanded{Child: basic.Text{Content: title, Style: th.Text.Title}},
//			control.Button{Label: "Add", OnPress: onAdd},
//		}}
//	}
//
// The geometry is in geom, the layout constants in layout, the keys in input,
// the theme in theme and the curves in motion.
//
// The root package may import every other package. It holds the only mutable
// package-level variable of the library, the running application in
// currentApp, because Post has to find it from any goroutine.
package gowaku
