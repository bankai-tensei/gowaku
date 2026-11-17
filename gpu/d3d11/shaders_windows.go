package d3d11

import _ "embed" // for the compiled shaders below

// The shaders are compiled by the go:generate line of shaders.go.

//go:embed shaders/sdf.vs.dxbc
var sdfVS []byte

//go:embed shaders/sdf.ps.dxbc
var sdfPS []byte

//go:embed shaders/path.vs.dxbc
var pathVS []byte
