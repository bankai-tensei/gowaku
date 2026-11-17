//go:build svgsets && !(gpu && windows)

package golden

import (
	"errors"
	"image"
)

// featGPU needs the gpu tag and Windows.
func featGPU(string, *Scene) (*image.NRGBA, error) {
	return nil, errors.New("GOWAKU_SVGSETS_GPU needs the gpu tag on Windows")
}
