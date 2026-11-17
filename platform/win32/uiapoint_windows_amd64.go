package win32

import (
	"syscall"

	"github.com/bankai-tensei/gowaku/platform/win32/com"
)

// uiaPoint is UiaPoint.
type uiaPoint struct{ x, y float64 }

// textRangeFromPointMethod returns ITextProvider::RangeFromPoint. On x64 a
// 16-byte struct is passed as a pointer to a copy, so no thunk is needed.
func textRangeFromPointMethod() (uintptr, error) {
	return syscall.NewCallback(func(this *com.Unknown, pt *uiaPoint, ret **com.Unknown) uintptr {
		if pt == nil {
			return com.E_INVALIDARG
		}
		return rangeFromPoint(this, pt.x, pt.y, ret)
	}), nil
}
