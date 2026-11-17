//go:build !windows && !darwin

package main

import "path/filepath"

// pathKey is the name of a file as the file system compares names.
func pathKey(p string) string { return filepath.Clean(p) }

// The families of the editor elsewhere: Noto CJK, which Linux distributions
// ship.
const (
	minchoFamily = "Noto Serif CJK JP"
	gothicFamily = "Noto Sans CJK JP"
)
