package golden

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/shader/shaders"
)

// shaderTime is the moment of the custom shaders that the scene shows.
const shaderTime = 1250 * time.Millisecond

// shaderScene draws the picture of examples/shader at a fixed time: custom
// shaders written in HLSL, by CustomPaint, turned, and by ShaderMask over text
// and a picture. The CPU draws their Go fallbacks, so that the picture holds
// D3D11, Vulkan on Windows and Vulkan on Linux to the same thing. See
// docs/ADR/0045-shaders-svg-animated-images.md.
func shaderScene() core.Widget {
	return shaders.Scene(shaderTime, false, shaders.Picture())
}
