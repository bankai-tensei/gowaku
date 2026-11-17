//go:build !windows && !linux && !(darwin && arm64)

package gowaku

// urlOpeners is empty, and OpenURL reports ErrUnsupported.
func urlOpeners() openers { return nil }
