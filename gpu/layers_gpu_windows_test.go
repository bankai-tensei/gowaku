//go:build gpu

package gpu_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu/d3d11"
	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform/win32"
	"github.com/bankai-tensei/gowaku/text"
)

// TestParity_LayersLookLikeDirectDrawing draws each scene directly and through a
// layer. Colors pass through 8 bits once more on the way, and nothing else may
// change.
func TestParity_LayersLookLikeDirectDrawing(t *testing.T) {
	p, err := win32.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Destroy)
	shaper, err := text.NewSystemShaper()
	if err != nil {
		t.Fatal(err)
	}
	direct, err := d3d11.New(d3d11.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(direct.Destroy)
	explicit, err := vulkan.New(vulkan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(explicit.Destroy)

	for name, b := range map[string]reader{"d3d11": direct, "vulkan": explicit} {
		s := open(t, p, b, shaper)
		s.onDemand = true
		for _, scale := range []float32{1, 1.5, 2} {
			for scene, draw := range scenes(shaper) {
				if scene == "filtered" {
					// A blurred layer inside another layer is drawn sharp:
					// layers do not nest.
					continue
				}
				t.Run(name+"/"+scene, func(t *testing.T) {
					want := s.render(t, scale, draw)
					var l paint.Layer
					draw(l.Begin())
					l.End()
					got := s.render(t, scale, func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{}) })
					r := compare(want, got)
					t.Logf("scale %v: %v", scale, r)
					if r.failed() {
						save(t, "layers", name+"-"+scene+"-want", want)
						save(t, "layers", name+"-"+scene+"-got", got)
						t.Errorf("scale %v: %v", scale, r)
					}
				})
			}
		}
	}
}
