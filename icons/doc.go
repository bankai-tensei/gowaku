// Package icons is a small set of common icons for basic.Icon and the widgets
// that take a *paint.Path, such as nav.BreadcrumbItem.Icon, Tree.IconOf,
// SidebarItem.Icon, control.Button.Icon and Toolbar items:
//
//	basic.Icon{Path: icons.Folder(), Color: th.Color.AccentInk}
//
// Each icon is drawn on the 24 unit grid that basic.Icon expects, as a line
// 1.75 units wide with round ends and corners, the line of the icons the
// widgets draw themselves (the close button of a tab, the pin of a dock
// panel). The paths are the outline of that line, to fill with the non-zero
// rule, so they take one color and scale like any other path.
//
// Every call builds a new path. Build an icon once, as in InitState, and keep
// it: basic.Icon scales a path again whenever it gets another pointer.
//
// The set is deliberately small. For more, parse the path data of an icon set
// with paint.ParseSVGPath, or draw an SVG file with basic.SVG.
//
// icons sits just above paint and imports only paint and geom. It must not
// import core, layout, the widgets or anything else above paint.
package icons
