package gowakutest_test

import (
	"fmt"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
)

// mailbox is a sidebar whose chosen row has a count and a detail, beside rows
// that are not chosen.
type mailbox struct{ gowaku.StatelessBase }

func (mailbox) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Row{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		collection.Sidebar{Title: "Mail", Children: []gowaku.Widget{
			collection.SidebarItem{Label: "Inbox", Count: 12, Selected: true, OnPress: func() {}},
			collection.SidebarItem{Label: "Aoi", Detail: "See you at seven", Count: 3, Selected: true, OnPress: func() {}},
			collection.SidebarItem{Label: "Drafts", Count: 4, OnPress: func() {}},
		}},
		basic.Expanded{Child: basic.Container{}},
	}}
}

// TestWarnings_ChosenSidebarRowReads checks that the count and the detail of a
// chosen sidebar row read at 4.5:1 on its tint, for accents of every kind.
func TestWarnings_ChosenSidebarRowReads(t *testing.T) {
	for _, accent := range []uint32{0x2f5fd0, 0x0a84ff, 0xe0457b, 0x2f7d5b, 0xb58900, 0x0f766e, 0xc2410c, 0x7c3aed} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%06x dark=%v", accent, dark), func(t *testing.T) {
				tt := gowakutest.New(t, mailbox{}, gowakutest.Options{
					Width: 500, Height: 300, Theme: theme.FromAccent(geom.Hex(accent)), Dark: dark, Fonts: goFonts.Fonts,
				})
				for _, w := range tt.Warnings() {
					t.Error(w)
				}
			})
		}
	}
}
