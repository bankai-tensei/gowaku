package gowaku

import "github.com/bankai-tensei/gowaku/platform/win32"

func urlOpeners() openers { return openers{win32.OpenURL} }
