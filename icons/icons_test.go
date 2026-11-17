package icons

import (
	"testing"

	"github.com/bankai-tensei/gowaku/paint"
)

var all = []struct {
	name string
	icon func() *paint.Path
}{
	{"Folder", Folder}, {"File", File}, {"Document", Document}, {"Image", Image},
	{"Search", Search}, {"Plus", Plus}, {"Minus", Minus}, {"Close", Close}, {"Check", Check},
	{"ChevronLeft", ChevronLeft}, {"ChevronRight", ChevronRight}, {"ChevronUp", ChevronUp}, {"ChevronDown", ChevronDown},
	{"Settings", Settings}, {"Trash", Trash}, {"Edit", Edit}, {"Copy", Copy}, {"Home", Home}, {"Star", Star},
	{"Info", Info}, {"Warning", Warning}, {"Error", Error}, {"Menu", Menu}, {"More", More},
}

func TestIcons_FitTheGrid(t *testing.T) {
	for _, ic := range all {
		p := ic.icon()
		if p.Empty() || p.Rule != paint.FillNonZero {
			t.Errorf("%s: empty %v, rule %v", ic.name, p.Empty(), p.Rule)
			continue
		}
		b := p.Bounds()
		// The line stays inside the grid, with room for its round ends.
		if b.X < 1.5 || b.Y < 1.5 || b.X+b.W > 22.5 || b.Y+b.H > 22.5 || b.W < 4 && b.H < 4 {
			t.Errorf("%s: bounds %v leave the grid", ic.name, b)
		}
	}
}

func TestIcons_EachCallIsANewPath(t *testing.T) {
	a, b := Folder(), Folder()
	if a == b || !a.Equal(b) {
		t.Error("each call must build an equal path of its own")
	}
}
