//go:build darwin

package main

import "path/filepath"

// pathKey is the name of a file as the file system compares names.
func pathKey(p string) string { return filepath.Clean(p) }

// The families of the editor on macOS: Hiragino, which the system ships.
const (
	minchoFamily = "Hiragino Mincho ProN"
	gothicFamily = "Hiragino Sans"
)
