//go:build windows && !amd64 && !arm64

package win32

import "github.com/bankai-tensei/gowaku/platform/win32/com"

// textRangeFromPointMethod fails where com has no float thunk, and with it
// the whole UI Automation bridge.
func textRangeFromPointMethod() (uintptr, error) { return 0, com.ErrNoThunk }
