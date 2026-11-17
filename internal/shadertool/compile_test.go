package shadertool

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tools returns fxc, dxc and spirv-cross, or skips the test without them.
func tools(t *testing.T) CompileOptions {
	t.Helper()
	var opts CompileOptions
	var err error
	if opts.FXC, err = FindFXC(); err != nil {
		t.Skip(err)
	}
	if opts.DXC, err = FindDXC(); err != nil {
		t.Skip(err)
	}
	if opts.SPIRVCross, err = FindSPIRVCross(); err != nil {
		t.Skip(err)
	}
	return opts
}

// The checked-in output of the example comes from its sources as they are.
// go generate compiles each from its own directory, which names it in the
// #line of the wrapped source; so does the test.
func TestCompileShader_ExampleIsUpToDate(t *testing.T) {
	opts := tools(t)
	dir, err := filepath.Abs("../../examples/shader/shaders")
	if err != nil {
		t.Fatal(err)
	}
	sources, err := filepath.Glob(filepath.Join(dir, "*.hlsl"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no sources: %v", err)
	}
	out := t.TempDir()
	t.Chdir(dir)
	for _, src := range sources {
		stem := strings.TrimSuffix(filepath.Base(src), ".hlsl")
		opts.Stem = filepath.Join(out, stem)
		got, err := CompileShader(stem+".hlsl", opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, pair := range [][2]string{{got.DXBC, ".dxbc"}, {got.SPIRV, ".spv"}, {got.MSL, ".metal"}} {
			made, err := os.ReadFile(pair[0])
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(stem + pair[1])
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(made, want) {
				t.Errorf("%s%s is out of date; run go generate ./examples/shader/...", stem, pair[1])
			}
		}
	}
}

// Errors point at the lines of the source, not at those of the prelude.
func TestCompileShader_ErrorsNameTheSourceLine(t *testing.T) {
	opts := tools(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.hlsl")
	code := "// comment\nfloat4 shade(ShaderInput i) {\n    float x = i.pos.x;\n    return float4(x, y, 0.0, 1.0);\n}\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.Stem = filepath.Join(dir, "bad")
	_, err := CompileShader(src, opts)
	if err == nil {
		t.Fatal("no error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bad.hlsl(4,") || strings.Contains(msg, "wrapped") {
		t.Errorf("the error does not point at line 4 of bad.hlsl:\n%s", msg)
	}
}
