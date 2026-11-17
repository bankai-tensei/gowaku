// Package sample has previews for the tests of gowaku-preview.
package sample

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// badge is a small view with an unexported name, which a preview can reach.
func badge(label string) gowaku.Widget {
	return basic.Container{Padding: geom.All(8), Color: geom.Hex(0xeeeeff), Child: basic.Text{Content: label}}
}
