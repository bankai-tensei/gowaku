package gowaku

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// tabApp is a browser in miniature: windows of tabs that tear off and drop
// back.
type tabApp struct {
	group nav.TabGroup
	// lists holds the tabs of each open window, by the window's first tab.
	states []*tabWinState
}

type tabWin struct {
	StatefulBase
	app   *tabApp
	first []int
}

type tabWinState struct {
	StateOf[tabWin]
	tabs     []int
	selected int
}

func (tabWin) CreateState() State { return &tabWinState{} }

func (s *tabWinState) InitState() {
	s.tabs = slices.Clone(s.Widget().first)
	s.Widget().app.states = append(s.Widget().app.states, s)
}

func (s *tabWinState) Dispose() {
	a := s.Widget().app
	a.states = slices.DeleteFunc(a.states, func(o *tabWinState) bool { return o == s })
}

func (s *tabWinState) Build(ctx BuildContext) Widget {
	a := s.Widget().app
	h := WindowOf(ctx)
	items := make([]nav.TabItem, len(s.tabs))
	for i, k := range s.tabs {
		items[i] = nav.TabItem{Key: k, Label: string(rune('A' + k))}
	}
	remove := func(i int) {
		s.SetState(func() {
			s.tabs = slices.Delete(s.tabs, i, i+1)
			s.selected = max(min(s.selected, len(s.tabs)-1), 0)
		})
		if len(s.tabs) == 0 {
			h.Close()
		}
	}
	return basic.Column{Children: []Widget{nav.TabBar{
		Tabs: items, Selected: s.selected, Group: &a.group,
		OnSelect: func(i int) { s.SetState(func() { s.selected = i }) },
		OnReceive: func(k Key, i int) {
			s.SetState(func() { s.tabs = slices.Insert(s.tabs, i, k.(int)) })
		},
		OnDetach: remove,
		OnTearOff: func(e nav.TabTearOff) {
			if e.Alone {
				h.MoveTo(e)
				return
			}
			remove(e.Index)
			if _, err := TearOffWindow(e, WindowOptions{Width: 300, Height: 200}, tabWin{app: a, first: []int{e.Key.(int)}}); err != nil {
				panic(err)
			}
		},
	}}}
}

func TestTearOffWindow_OpensUnderThePointerAndDropsBack(t *testing.T) {
	a := &tabApp{}
	p, b := newFakes(t)
	pointer := func(kind platform.EventKind, id platform.WindowID, x, y float32) {
		p.send(platform.Event{Kind: kind, Window: id, X: x, Y: y})
	}
	p.script = []func(){
		func() {
			p.windows[1].MoveClientTo(1000, 500)
			// Three tabs share 400 less the new tab button... there is none,
			// so 400 / 3: 133 each, at scale 2. The middle one is torn off.
			pointer(platform.EventPointerMove, 1, 400, 34)
			pointer(platform.EventPointerDown, 1, 400, 34)
			pointer(platform.EventPointerMove, 1, 410, 40)
			pointer(platform.EventPointerMove, 1, 400, 400)
			pointer(platform.EventPointerUp, 1, 400, 400)
		},
		func() {
			w2 := p.windows[2]
			if w2 == nil {
				t.Fatal("the tab must get a window")
			}
			// The pointer let go at 1400, 900 on the screen, holding the tab
			// 200 - 133 = 67 logical pixels from its left edge and 17 from its top.
			if o := w2.opts; !o.Placed || o.ClientX != 1400-67*2 || o.ClientY != 900-17*2 {
				t.Errorf("got %+v", o)
			}
			if got := a.states[0].tabs; !slices.Equal(got, []int{0, 2}) {
				t.Errorf("the first window keeps %v", got)
			}
			// Back onto the end of the first bar: two tabs of 200 there.
			// Window 2 is at 1266, 866; its tab is held where it was.
			x, y := float32(1000+380*2-w2.origin[0]), float32(500+17*2-w2.origin[1])
			pointer(platform.EventPointerMove, 2, 134, 34)
			pointer(platform.EventPointerDown, 2, 134, 34)
			pointer(platform.EventPointerMove, 2, 134, 300)
			pointer(platform.EventPointerMove, 2, x, y)
			pointer(platform.EventPointerUp, 2, x, y)
		},
		func() {
			if !p.windows[2].closed {
				t.Error("the empty window must close")
			}
			if len(a.states) != 1 || !slices.Equal(a.states[0].tabs, []int{0, 2, 1}) {
				t.Errorf("got %v", a.states[0].tabs)
			}
			if p.windows[1].activations == 0 {
				t.Error("the window that took the tab must come to the front")
			}
			Quit()
		},
	}
	app := App{Home: tabWin{app: a, first: []int{0, 1, 2}}, Width: 400, Height: 200}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
}
