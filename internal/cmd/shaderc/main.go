// Command shaderc compiles the HLSL shaders of a directory for one backend.
//
// A file named x.vs.hlsl compiles as a vertex shader and x.ps.hlsl as a pixel
// shader. With -target dxbc the output is x.vs.dxbc, made by fxc from the
// Windows SDK. With -target spirv it is x.vs.spv, made by dxc from the Vulkan
// SDK. The output is checked in, so only people who change a shader need these.
// See docs/ADR/0012-shaders-for-vulkan.md.
package main

import (
	"flag"
	"log"
	"path/filepath"
	"strings"

	"github.com/bankai-tensei/gowaku/internal/shadertool"
)

func main() {
	log.SetFlags(0)
	src := flag.String("src", ".", "directory holding the .hlsl files")
	out := flag.String("out", ".", "directory to write to")
	target := flag.String("target", "dxbc", "dxbc or spirv")
	flag.Parse()

	find := shadertool.FindFXC
	if *target == "spirv" {
		find = shadertool.FindDXC
	}
	tool, err := find()
	if err != nil {
		log.Fatal(err)
	}
	sources, err := filepath.Glob(filepath.Join(*src, "*.hlsl"))
	if err != nil {
		log.Fatal(err)
	}
	for _, source := range sources {
		stem := strings.TrimSuffix(filepath.Base(source), ".hlsl")
		kind := filepath.Ext(stem)
		if kind != ".vs" && kind != ".ps" {
			log.Fatalf("%s: name must end in .vs.hlsl or .ps.hlsl", source)
		}
		dst := filepath.Join(*out, stem)
		args := shadertool.DXBCArgs(source, dst+".dxbc", kind[1:])
		if *target == "spirv" {
			args = shadertool.SPIRVArgs(source, dst+".spv", kind[1:], shadertool.SPIRVBindings)
		}
		if err := shadertool.Run(tool, args...); err != nil {
			log.Fatalf("%s: %v", source, err)
		}
	}
}
