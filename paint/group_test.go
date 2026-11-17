package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func box() geom.RRect { return geom.RRect{Rect: geom.Rect{W: 10, H: 10}} }

func TestCanvas_GroupRecordsItsRange(t *testing.T) {
	c := newCanvas()
	c.Translate(5, 6)
	c.PushOpacity(0.5)
	c.PushBlend(BlendMultiply)
	c.ClipRect(geom.Rect{W: 50, H: 50})
	filter := &Filter{Region: geom.Rect{W: 10, H: 10}, Primitives: []FilterPrimitive{{Kind: FilterFlood}}}
	c.PushGroup(GroupStyle{Mask: MaskLuminance, Filter: filter})
	c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
	c.Save()
	c.PushOpacity(0.5)
	c.ClipRRect(geom.RRectAll(geom.Rect{W: 8, H: 8}, 2))
	c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
	c.BeginMask()
	c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
	c.PopGroup()
	c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
	dl := c.DisplayList()
	kinds := ""
	for _, op := range dl.Ops {
		kinds += op.Kind.String() + " "
	}
	const want = "group-push rrect clip-push rrect clip-pop group-mask rrect group-pop rrect "
	if kinds != want {
		t.Fatalf("got %q, want %q", kinds, want)
	}
	push := &dl.Ops[0]
	call := &dl.Groups[push.ShaderCall]
	if call.Mask != 5 || call.End != 7 || call.Style.Filter != filter || call.Style.Mask != MaskLuminance {
		t.Errorf("call %+v", call)
	}
	// The group takes the opacity and the blend mode of the moment, and the
	// content starts afresh: full opacity, source-over.
	if push.Color.A != 0.5 || push.Blend != BlendMultiply || push.Origin != (geom.Offset{X: 5, Y: 6}) || !push.Clipped {
		t.Errorf("push %+v", push)
	}
	if a, b := dl.Ops[1].Color.A, dl.Ops[1].Blend; a != 1 || b != BlendSrcOver {
		t.Errorf("content: opacity %v, blend %v", a, b)
	}
	if a := dl.Ops[3].Color.A; a != 0.5 {
		t.Errorf("content opacity under its own: %v", a)
	}
	// The mask draws with the state of PushGroup, its clip popped, and so does
	// what comes after the group.
	if dl.Ops[6].Color.A != 1 || dl.Ops[6].ClipLevel != 0 || dl.Ops[8].Color.A != 0.5 || dl.Ops[8].Blend != BlendMultiply {
		t.Errorf("mask %+v, after %+v", dl.Ops[6], dl.Ops[8])
	}
}

func TestCanvas_HiddenGroupRecordsNothing(t *testing.T) {
	c := newCanvas()
	c.PushOpacity(0)
	c.PushGroup(GroupStyle{Mask: MaskAlpha})
	c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
	c.BeginMask()
	c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
	c.PopGroup()
	if n := len(c.DisplayList().Ops); n != 0 {
		t.Errorf("recorded %d ops", n)
	}
}

func TestCanvas_GroupMisuse(t *testing.T) {
	panics := func(name string, f func(c *Canvas)) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: no panic", name)
			}
		}()
		f(newCanvas())
	}
	panics("pop without push", func(c *Canvas) { c.PopGroup() })
	panics("mask without push", func(c *Canvas) { c.BeginMask() })
	panics("mask of a group without one", func(c *Canvas) {
		c.PushGroup(GroupStyle{})
		c.BeginMask()
	})
	panics("restore across the group", func(c *Canvas) {
		c.Save()
		c.PushGroup(GroupStyle{})
		c.Restore()
	})
}

func TestCanvas_GroupDoesNotAllocate(t *testing.T) {
	c := newCanvas()
	frame := func() {
		c.Reset()
		c.PushGroup(GroupStyle{Mask: MaskAlpha})
		c.PushGroup(GroupStyle{})
		c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
		c.PopGroup()
		c.BeginMask()
		c.DrawRRect(box(), Paint{Color: geom.Color{A: 1}})
		c.PopGroup()
	}
	frame()
	if n := testing.AllocsPerRun(20, frame); n != 0 {
		t.Errorf("%v allocations", n)
	}
}

func TestTransferFunc_Apply(t *testing.T) {
	cases := []struct {
		f    TransferFunc
		in   float32
		want float32
	}{
		{TransferFunc{}, 0.3, 0.3},
		{TransferFunc{Kind: TransferTable, Table: []float32{0, 1, 0}}, 0.25, 0.5},
		{TransferFunc{Kind: TransferDiscrete, Table: []float32{0.1, 0.9}}, 0.6, 0.9},
		{TransferFunc{Kind: TransferLinear, Slope: 2, Intercept: -0.5}, 0.5, 0.5},
		{TransferFunc{Kind: TransferGamma, Amplitude: 2, Exponent: 2, Offset: 0.1}, 0.5, 0.6},
	}
	for _, c := range cases {
		if got := c.f.Apply(c.in); geom.Abs32(got-c.want) > 1e-6 {
			t.Errorf("%+v at %v: got %v, want %v", c.f, c.in, got, c.want)
		}
	}
}
