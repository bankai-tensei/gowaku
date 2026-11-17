package sample

import (
	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakupreview"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

func PreviewTheme() theme.Set { return theme.FromAccent(geom.Hex(0x2f7d5b)) }

func PreviewBadge() gowaku.Widget { return badge("New") }

func PreviewTooWide() gowakupreview.Item {
	return gowakupreview.Item{Width: 200, Height: 40, Themes: gowakupreview.LightOnly, Widget: basic.Row{Children: []gowaku.Widget{
		basic.SizedBox{Width: 150, Height: 20}, basic.SizedBox{Width: 150, Height: 20},
	}}}
}

// Previewer is not a preview: a lower case letter follows Preview.
func Previewer() gowaku.Widget { return badge("no") }
