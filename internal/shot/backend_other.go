//go:build !windows && !linux && !(darwin && arm64)

package shot

import (
	"errors"
	"time"

	"github.com/bankai-tensei/gowaku/platform"
)

// open reports that this system has no backend.
func open(*Options) (platform.Platform, device, error) {
	return nil, nil, errors.New("shot: this platform is not supported yet")
}

// origin is where clock counts from.
var origin = time.Now()

// clock is a monotonic time fine enough to time parts of a frame.
func clock() time.Duration { return time.Since(origin) }
