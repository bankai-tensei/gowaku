package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// mediaReader counts its builds and keeps what read gives.
type mediaReader struct {
	core.StatelessBase
	builds *int
	read   func(ctx core.BuildContext)
}

func (m mediaReader) Build(ctx core.BuildContext) core.Widget {
	*m.builds++
	m.read(ctx)
	return SizedBox{}
}

// mediaHost shows a MediaQuery of data over a child that stays the same.
type mediaHost struct {
	core.StatefulBase
	child core.Widget
	st    **mediaHostState
}

type mediaHostState struct {
	core.StateBase
	data MediaQueryData
	// body is held by pointer, as the root of a window holds its content.
	body *Builder
}

func (h mediaHost) CreateState() core.State {
	s := &mediaHostState{data: MediaQueryData{Scale: 1, Size: geom.Size{W: 400, H: 300}}}
	child := h.child
	s.body = &Builder{Content: func(core.BuildContext) core.Widget { return child }}
	*h.st = s
	return s
}

func (s *mediaHostState) Build(core.BuildContext) core.Widget {
	return MediaQuery{Data: s.data, Child: s.body}
}

func TestMediaQuery_ResizeRebuildsOnlyTheSizeReaders(t *testing.T) {
	var sizeBuilds, fullBuilds, motionBuilds, localeBuilds int
	var size geom.Size
	var full MediaQueryData
	var st *mediaHostState
	tree := core.NewTree(mediaHost{st: &st, child: Column{Children: []core.Widget{
		mediaReader{builds: &sizeBuilds, read: func(ctx core.BuildContext) { size = WindowSizeOf(ctx) }},
		mediaReader{builds: &fullBuilds, read: func(ctx core.BuildContext) { full = MediaQueryOf(ctx) }},
		mediaReader{builds: &motionBuilds, read: func(ctx core.BuildContext) { _ = ReduceMotionOf(ctx) }},
		mediaReader{builds: &localeBuilds, read: func(ctx core.BuildContext) { _ = LocaleOf(ctx) }},
	}}})
	t.Cleanup(tree.Dispose)
	wtest.Ops(tree, wtest.Screen)
	if size != (geom.Size{W: 400, H: 300}) || full.Size != size || full.Scale != 1 {
		t.Fatalf("read size %v and data %+v", size, full)
	}

	st.SetState(func() { st.data.Size = geom.Size{W: 200, H: 300} })
	wtest.Ops(tree, wtest.Screen)
	if sizeBuilds != 2 || fullBuilds != 2 || motionBuilds != 1 || localeBuilds != 1 {
		t.Errorf("after a resize builds are %d %d %d %d, want 2 2 1 1", sizeBuilds, fullBuilds, motionBuilds, localeBuilds)
	}
	if size.W != 200 || full.Size.W != 200 {
		t.Errorf("after a resize read %v and %v", size, full.Size)
	}

	st.SetState(func() { st.data.ReduceMotion = true })
	wtest.Ops(tree, wtest.Screen)
	if sizeBuilds != 2 || fullBuilds != 3 || motionBuilds != 2 || localeBuilds != 2 {
		t.Errorf("after ReduceMotion builds are %d %d %d %d, want 2 3 2 2", sizeBuilds, fullBuilds, motionBuilds, localeBuilds)
	}
}
