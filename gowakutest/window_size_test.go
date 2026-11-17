package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
)

func TestTester_WindowSize(t *testing.T) {
	var got geom.Size
	gowakutest.New(t, basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		got = basic.WindowSizeOf(ctx)
		return basic.SizedBox{}
	}}, gowakutest.Options{Width: 320, Height: 200})
	if got != (geom.Size{W: 320, H: 200}) {
		t.Errorf("WindowSizeOf is %v, want the size of the options", got)
	}
}
