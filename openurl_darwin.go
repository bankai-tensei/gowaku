//go:build darwin && arm64

package gowaku

import "github.com/bankai-tensei/gowaku/platform/cocoa"

func urlOpeners() openers { return openers{cocoa.OpenURL} }
