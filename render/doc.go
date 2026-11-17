// Package render turns a paint.DisplayList into the gpu.Batch that a backend
// draws. It is the only package that knows both sides.
//
// render imports paint, text, gpu and geom. It must not import platform, and nothing
// below the root package imports it.
//
// The passes of the filters and masks of groups draw with effectShader, a
// custom shader that cmd/gowaku-shader compiles from effect.hlsl into
// effect_shader.go: a package-level variable that is set once and never
// changes, like the shaders of an application. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
package render

//go:generate go run ../cmd/gowaku-shader -name effectShader effect.hlsl
