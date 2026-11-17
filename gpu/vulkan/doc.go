// Package vulkan implements gpu.Backend on Vulkan 1.0 without cgo. The loader is
// opened with purego on Linux and with LoadLibrary on Windows, and every
// function is resolved at startup through vkGetInstanceProcAddr.
//
// Linux draws with this backend. On Windows it exists so that tests can compare
// it with gpu/d3d11 on the same machine.
//
// This is one of the few packages allowed to use unsafe and purego.
//
// The ABI types are exported only because they keep their header names. This
// package is not part of the public API of gowaku, which makes no promise about
// them. See docs/ADR/0016-public-api.md.
//
// The variable kept is an exception to CODING_STYLE §3.2. See ptr.
package vulkan
