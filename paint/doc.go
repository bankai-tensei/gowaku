// Package paint records drawing commands. A Canvas turns calls such as
// DrawRRect into a DisplayList that the render package later hands to the GPU.
//
// paint imports geom, and the image decoders. It must not import gpu, platform or render, which
// is what lets everything above it run without a GPU.
package paint
