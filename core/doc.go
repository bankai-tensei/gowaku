// Package core is the declarative heart of gowaku: the three trees of Widget,
// element and RenderObject, the reconciler that diffs them, and the scheduling
// of build, layout and paint.
//
// core imports paint and geom. It must not import layout, widgets, motion,
// theme, gpu or platform, which is what lets it run in tests without a GPU.
// Everything in core must be used on the UI goroutine.
package core
