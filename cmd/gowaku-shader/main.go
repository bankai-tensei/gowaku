// Command gowaku-shader compiles a custom shader for gowaku.
//
// A custom shader is one HLSL function in a file of its own:
//
//	float4 shade(ShaderInput i) {
//	    float wave = 0.5 + 0.5 * sin(i.pos.x * 0.1 + i.time);
//	    return float4(wave, 0.2, 0.6, 1.0); // premultiplied
//	}
//
// where
//
//	struct ShaderInput {
//	    float2 pos;       // the pixel center from the top left, in logical pixels
//	    float2 size;      // the size of the rectangle, in logical pixels
//	    float2 uv;        // pos / size
//	    float time;       // seconds
//	    float scale;      // physical pixels per logical pixel
//	    float4 params[4]; // paint.ShaderParams
//	};
//
// and float4 sample0(float2 uv) and sample1 read the two textures, with uv from
// 0 to 1 across the rectangle, transparent outside the picture; size0() and
// size1() say how many texels span the rectangle. Names that start with gowaku,
// blend or BLEND_ belong to gowaku.
//
// Usage:
//
//	gowaku-shader [flags] ripple.hlsl
//
// writes ripple.dxbc with fxc of the Windows SDK, ripple.spv with dxc of the
// Vulkan SDK, ripple.metal with spirv-cross of the Vulkan SDK, and
// ripple_shader.go, which embeds them and declares
//
//	var Ripple = paint.NewShader(paint.Shader{...})
//
// Check all four in: building the program needs none of the tools. A
// //go:generate line next to the source keeps them up to date. The flags:
//
//	-pkg name       the package of the Go file; by default that of the .go files
//	                in the output directory, or its name
//	-name Name      the variable; by default the file name, as in Ripple
//	-fallback fn    a paint.ShaderFallback in the package that paints the
//	                shader on the CPU, as gowakutest does
//	-out dir        where to write; by default next to the source
//	-keep           also write the wrapped source, ripple.wrapped.hlsl
//
// Errors in the source point at its own lines. See
// docs/ADR/0045-shaders-svg-animated-images.md for the register layout.
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/bankai-tensei/gowaku/internal/shadertool"
)

func main() {
	pkg := flag.String("pkg", "", "package of the Go file")
	name := flag.String("name", "", "name of the shader variable")
	fallback := flag.String("fallback", "", "paint.ShaderFallback that paints the shader on the CPU")
	out := flag.String("out", "", "directory to write to")
	keep := flag.Bool("keep", false, "also write the wrapped HLSL")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "usage: gowaku-shader [flags] shader.hlsl") //nolint:errcheck // nothing is left to do if the usage cannot be written
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *pkg, *name, *fallback, *out, *keep); err != nil {
		fmt.Fprintln(os.Stderr, "gowaku-shader:", err)
		os.Exit(1)
	}
}

func run(src, pkg, name, fallback, out string, keep bool) error {
	if filepath.Ext(src) != ".hlsl" {
		return fmt.Errorf("%s: the source must end in .hlsl", src)
	}
	stem := strings.TrimSuffix(filepath.Base(src), ".hlsl")
	if out == "" {
		out = filepath.Dir(src)
	}
	var err error
	if name == "" {
		if name, err = shadertool.ExportedName(stem); err != nil {
			return err
		}
	}
	if pkg == "" {
		if pkg, err = packageOf(out); err != nil {
			return err
		}
	}
	file := shadertool.GoFile{Package: pkg, Name: name, Stem: stem, Fallback: fallback}
	code, err := file.Generate()
	if err != nil {
		return err
	}
	opts := shadertool.CompileOptions{Stem: filepath.Join(out, stem), Keep: keep}
	if opts.FXC, err = shadertool.FindFXC(); err != nil {
		return err
	}
	if opts.DXC, err = shadertool.FindDXC(); err != nil {
		return err
	}
	if opts.SPIRVCross, err = shadertool.FindSPIRVCross(); err != nil {
		return err
	}
	if _, err := shadertool.CompileShader(src, opts); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, stem+"_shader.go"), code, 0o644)
}

// packageOf returns the package of the Go files in dir, leaving out tests and
// generated shader files, or else the name of dir.
func packageOf(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, f, nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}
		return parsed.Name.Name, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	base := strings.ReplaceAll(filepath.Base(abs), "-", "")
	if !token.IsIdentifier(base) {
		return "", errors.New("cannot tell the package; give it with -pkg")
	}
	return base, nil
}
