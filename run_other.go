//go:build !windows && !linux && !(darwin && arm64)

package gowaku

// Run reports ErrUnsupported on systems without a backend.
func Run(App) error { return ErrUnsupported }
