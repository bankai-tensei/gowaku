package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// searchPlayer is a player whose search field gives the keyboard back on
// Enter, so that bare Space plays again.
func searchPlayer(plays *int) gowaku.Widget {
	return interact.Actions{
		Actions: []interact.Action{
			{Label: "Play", Shortcut: interact.Shortcut{Key: input.KeySpace}, OnInvoke: func() { *plays++ }},
		},
		Child: basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Builder{Content: func(ctx gowaku.BuildContext) gowaku.Widget {
			return basic.SizedBox{Width: 240, Child: field.Search{Label: "Find", OnSubmit: func(string) { interact.Unfocus(ctx) }}}
		}}},
	}
}

func TestUnfocus_GivesTheKeysBackToActions(t *testing.T) {
	plays := 0
	tt := gowakutest.New(t, searchPlayer(&plays), goFonts)
	tt.Tap("Find")
	typeKey(tt, input.KeySpace, 0, " ")
	if plays != 0 {
		t.Fatalf("Space in the field played %d times", plays)
	}
	tt.Key(input.KeyEnter, 0)
	if tt.Tree().Focused() != nil {
		t.Fatalf("Unfocus left %T focused", tt.Tree().Focused())
	}
	tt.Key(input.KeySpace, 0)
	if plays != 1 {
		t.Errorf("Space after Unfocus played %d times, want 1", plays)
	}
}

func TestUnfocus_AClickOnNothingDoesTheSame(t *testing.T) {
	plays := 0
	tt := gowakutest.New(t, searchPlayer(&plays), goFonts)
	tt.Tap("Find")
	tt.TapAt(geom.Offset{X: 600, Y: 400})
	tt.Key(input.KeySpace, 0)
	if plays != 1 {
		t.Errorf("Space after a click beside the field played %d times, want 1", plays)
	}
}
