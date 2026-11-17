// Package widgets holds no code. Its subpackages are the standard widgets, in
// layers; a package imports only the layers below it, never one beside it:
//
//	basic       theme, media, strings, boxes and flex, effects, implicit
//	            animation, text, images, scrolling
//	interact    gestures, focus, shortcuts and actions, drag and drop,
//	            semantics
//	control     buttons, switches, radios, chips, cards, steppers, tabs,
//	            split panes, badges, progress
//	overlay     the overlay, dialogs, sheets, toasts, tooltips, menus
//	field, nav  text input, forms and pickers; navigation, tab bars and
//	            toolbars
//	collection, editor
//	            lists, trees, tables and property grids; the document editor
//
// Each widget is a value type whose zero value works, and its fields are its
// whole API. No package here imports gpu, platform or render. See
// docs/ADR/0081-widget-layers.md.
package widgets
