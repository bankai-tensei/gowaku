// Package soft implements gpu.Backend on the CPU, as the reference that the GPU
// backends are compared to. It serves tests, headless screenshots and generated
// documentation, not applications: it makes no attempt at speed.
//
// It mirrors the GPU rather than approximating it. The pixel shader of gpu/shaders
// runs in float32, once per pixel at the pixel center. Its results become 8-bit
// unorm values before they blend with the 8-bit values that the target stores,
// which is what measurements of real GPUs show. Coverage, depth and stencil work
// per sample on vertices snapped to the 1/256 pixel grid of Direct3D, with its
// top-left fill rule. See docs/ADR/0009-paths-and-clips.md and
// docs/ADR/0017-multisample-on-demand.md for the steps that it reproduces.
//
// soft imports gpu, geom, and platform for NativeHandle only. It uses neither
// unsafe nor syscall, so that it builds and runs on every OS without a build tag.
package soft
