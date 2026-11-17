package shadertool

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files of the generated code")

// golden compares got with testdata/name, or rewrites it with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run with -update to create it", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from what is generated now:\n%s", path, got)
	}
}

func TestGoFile_Generate(t *testing.T) {
	for _, f := range []GoFile{
		{Package: "main", Name: "Ripple", Stem: "ripple", Fallback: "rippleFallback"},
		{Package: "fx", Name: "WaveMask", Stem: "wave_mask"},
	} {
		code, err := f.Generate()
		if err != nil {
			t.Fatal(err)
		}
		golden(t, f.Stem+"_shader.go.golden", code)
	}
}

func TestGoFile_RejectsWhatCannotCompile(t *testing.T) {
	for _, f := range []GoFile{
		{Package: "my-pkg", Name: "Ripple", Stem: "ripple"},
		{Package: "main", Name: "2D", Stem: "ripple"},
		{Package: "main", Name: "Ripple", Stem: "rip\"ple"},
		{Package: "main", Name: "Ripple", Stem: "ripple", Fallback: "a.b"},
	} {
		if _, err := f.Generate(); err == nil {
			t.Errorf("%+v: no error", f)
		}
	}
}

func TestExportedName(t *testing.T) {
	for stem, want := range map[string]string{
		"ripple": "Ripple", "wave_mask": "WaveMask", "glow-2": "Glow2", "3d": "Shader3d", "aurora.v2": "AuroraV2",
	} {
		got, err := ExportedName(stem)
		if err != nil || got != want {
			t.Errorf("ExportedName(%q) = %q, %v; want %q", stem, got, err, want)
		}
	}
	if _, err := ExportedName("a+b"); err == nil {
		t.Error("ExportedName(a+b): no error")
	}
}

func TestWrap(t *testing.T) {
	src := []byte("float4 shade(ShaderInput i) {\n    return float4(i.uv, 0.0, 1.0);\n}")
	golden(t, "wrapped.hlsl.golden", Wrap(src, "fx/uv.hlsl"))
	// The source follows a #line that names it, so that compilers count its
	// lines, and the epilogue after it has its own.
	w := string(Wrap(src, "fx/uv.hlsl"))
	mark := "#line 1 \"fx/uv.hlsl\"\n"
	at := strings.Index(w, mark)
	if at < 0 || !strings.HasPrefix(w[at+len(mark):], string(src)+"\n#line 1 \"gowaku-shader epilogue\"") {
		t.Error("the source does not follow its #line, or the epilogue does not follow the source")
	}
}

func TestCheckSource(t *testing.T) {
	if err := CheckSource([]byte("float4 shade( ShaderInput input ) { return 0; }")); err != nil {
		t.Error(err)
	}
	for _, src := range []string{"float3 shade(ShaderInput i)", "float4 shader(ShaderInput i)", "float4 shade(float2 p)"} {
		if CheckSource([]byte(src)) == nil {
			t.Errorf("%q: no error", src)
		}
	}
}

// members returns the members of the struct named name in an HLSL source, one
// per line, without comments or spaces at the ends.
func members(t *testing.T, src, name string) []string {
	t.Helper()
	m := regexp.MustCompile(`(?s)struct ` + name + ` \{\n(.*?)\n\};`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("no struct %s", name)
	}
	var out []string
	for _, line := range strings.Split(m[1], "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// The pixel shader of a custom shader reads what the SDF vertex shader writes,
// so the two structs must be the same, member by member.
func TestPrelude_VaryingsMatchSDF(t *testing.T) {
	sdf, err := os.ReadFile("../../gpu/shaders/sdf.hlsli")
	if err != nil {
		t.Fatal(err)
	}
	want := members(t, string(sdf), "Varyings")
	got := members(t, prelude, "GowakuVaryings")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("GowakuVaryings:\n%s\nVaryings of sdf.hlsli:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestBlend_SameAsTheSDFShader(t *testing.T) {
	sdf, err := os.ReadFile("../../gpu/shaders/blend.hlsli")
	if err != nil {
		t.Fatal(err)
	}
	if string(sdf) != blend {
		t.Error("hlsl/blend.hlsli differs from gpu/shaders/blend.hlsli; copy the latter over")
	}
}

func TestCheckMSL(t *testing.T) {
	ok := "fragment main0_out main0(main0_in in [[stage_in]], constant type_GowakuFrame& GowakuFrame [[buffer(0)]], " +
		"texture2d<float> gowakuTex0 [[texture(1)]], texture2d<float> gowakuTex1 [[texture(0)]], texture2d<float> gowakuBackdrop [[texture(3)]])"
	if err := CheckMSL([]byte(ok)); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{
		"texture2d<float> gowakuTex0 [[texture(0)]]",
		"constant type_GowakuFrame& GowakuFrame [[buffer(1)]]",
		"texture2d<float> other [[texture(2)]]",
	} {
		if CheckMSL([]byte(bad)) == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// The Metal sources of the example are checked in; their bindings must be the
// documented ones.
func TestCheckMSL_Example(t *testing.T) {
	files, err := filepath.Glob("../../examples/shader/shaders/*.metal")
	if err != nil || len(files) == 0 {
		t.Fatalf("no Metal sources: %v", err)
	}
	for _, f := range files {
		metal, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := CheckMSL(metal); err != nil {
			t.Errorf("%s: %v", f, err)
		}
		if !bytes.Contains(metal, []byte("GowakuFrame [[buffer(0)]]")) {
			t.Errorf("%s: no frame constants at buffer(0)", f)
		}
	}
}
