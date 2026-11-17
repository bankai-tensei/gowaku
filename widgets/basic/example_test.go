package basic_test

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// A value that changes between builds moves there on its own.
func ExampleAnimatedContainer() {
	on := true
	color := geom.Hex(0xeaddff)
	if on {
		color = geom.Hex(0x6750a4)
	}
	var w core.Widget = basic.AnimatedContainer{
		Duration: 300 * time.Millisecond,
		Curve:    motion.EaseOutCubic,
		Color:    color,
		Radius:   16,
		Padding:  geom.All(16),
		Child:    basic.Text{Content: "こんにちは"},
	}
	_ = w
}

// Text wraps by the rules of Japanese composition without being asked to.
func ExampleText() {
	var w core.Widget = basic.Text{Content: "禁則処理も約物の詰めも、既定で効く。"}
	_ = w
}
