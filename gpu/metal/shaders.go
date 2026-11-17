//go:build darwin && arm64

package metal

import _ "embed" // for the shader source below

// sdfSource is the MSL of the SDF pipeline, a port of gpu/shaders. Metal
// compiles it at run time, so that there is nothing to generate.
//
//go:embed shaders/sdf.metal
var sdfSource string
