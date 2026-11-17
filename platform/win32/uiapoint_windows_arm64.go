package win32

import (
	"math"
	"syscall"

	"github.com/bankai-tensei/gowaku/platform/win32/com"
)

// textRangeFromPointMethod returns ITextProvider::RangeFromPoint. On arm64
// UiaPoint is a homogeneous aggregate of two doubles and comes in d0 and d1,
// so it goes through a float thunk like two double arguments.
func textRangeFromPointMethod() (uintptr, error) {
	return com.NewFloatThunk(syscall.NewCallback(func(this *com.Unknown, x, y uintptr, ret **com.Unknown) uintptr {
		return rangeFromPoint(this, math.Float64frombits(uint64(x)), math.Float64frombits(uint64(y)), ret)
	}), 2)
}
