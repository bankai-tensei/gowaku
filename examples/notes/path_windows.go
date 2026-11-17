package main

import (
	"path/filepath"
	"strings"
)

// pathKey is the name of a file as Windows compares names: without case.
func pathKey(p string) string { return strings.ToLower(filepath.Clean(p)) }

// The families of the editor on Windows.
const (
	minchoFamily = "Yu Mincho"
	gothicFamily = "Yu Gothic"
)
