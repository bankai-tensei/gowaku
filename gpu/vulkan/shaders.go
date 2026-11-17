//go:build linux || windows

package vulkan

import _ "embed" // for the compiled shaders below

//go:generate go run ../../internal/cmd/shaderc -src ../shaders -out shaders -target spirv

//go:embed shaders/sdf.vs.spv
var sdfVS []byte

//go:embed shaders/sdf.ps.spv
var sdfPS []byte

//go:embed shaders/path.vs.spv
var pathVS []byte
