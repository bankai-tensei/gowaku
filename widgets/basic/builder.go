package basic

import "github.com/bankai-tensei/gowaku/core"

// Builder builds its content with a function at its own place in the tree, so
// that the function can read what is above it, such as the theme:
//
//	Builder{Content: func(ctx core.BuildContext) core.Widget {
//		return Text{Content: "Hi", Style: ThemeOf(ctx).Text.Title}
//	}}
//
// A nil Content builds nothing.
type Builder struct {
	core.StatelessBase
	Content func(ctx core.BuildContext) core.Widget
}

// Build implements core.StatelessWidget.
func (b Builder) Build(ctx core.BuildContext) core.Widget {
	if b.Content == nil {
		return SizedBox{}
	}
	return b.Content(ctx)
}
