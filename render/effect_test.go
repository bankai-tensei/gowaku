package render

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bankai-tensei/gowaku/internal/shadertool"
)

// The compiled effect shader comes from effect.hlsl as it is. It needs the
// shader compilers, and skips without them.
func TestEffectShader_IsUpToDate(t *testing.T) {
	var opts shadertool.CompileOptions
	var err error
	if opts.FXC, err = shadertool.FindFXC(); err != nil {
		t.Skip(err)
	}
	if opts.DXC, err = shadertool.FindDXC(); err != nil {
		t.Skip(err)
	}
	if opts.SPIRVCross, err = shadertool.FindSPIRVCross(); err != nil {
		t.Skip(err)
	}
	opts.Stem = filepath.Join(t.TempDir(), "effect")
	got, err := shadertool.CompileShader("effect.hlsl", opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{got.DXBC, "effect.dxbc"}, {got.SPIRV, "effect.spv"}, {got.MSL, "effect.metal"}} {
		made, err := os.ReadFile(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(made, want) {
			t.Errorf("%s is out of date; run go generate ./render", pair[1])
		}
	}
}
