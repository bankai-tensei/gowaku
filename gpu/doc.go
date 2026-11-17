// Package gpu defines the rendering backend interface. Implementations live in
// gpu/d3d11, gpu/vulkan and gpu/metal, and gpu/soft draws on the CPU for tests.
//
// gpu imports geom, and platform for NativeHandle only. Only render and the
// root package import gpu; core, layout, paint, text, theme, motion and
// widgets must not.
package gpu
