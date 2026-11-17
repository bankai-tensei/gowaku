// Command listview scrolls 10,000 rows of an icon, text and a button.
//
//	listview                       opens the window
//	listview -measured             100,000 rows of wrapping text, whose
//	                               heights the list measures as they come
//	                               into view, with buttons that jump
//	listview -png out.png          draws one frame to a file and exits
//	listview -bench 3s             scrolls without waiting for vertical blank
//	                               and prints frame rates
package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/shot"
)

const rows = 10000

type page struct {
	gowaku.StatelessBase
	icon *paint.Path
}

func (p page) row(th *theme.Theme, i int) gowaku.Widget {
	return basic.Padding{Insets: geom.Symmetric(16, 4), Child: basic.Row{
		Spacing: 12,
		Cross:   layout.CrossCenter,
		Children: []gowaku.Widget{
			basic.Icon{Path: p.icon, Color: th.Color.Accent},
			basic.Expanded{Child: basic.Text{
				Content: fmt.Sprintf("%d 行目の項目 Item number %d", i+1, i+1),
				Style:   th.Text.Body,
			}},
			control.Button{Label: "開く", OnPress: func() {}},
		},
	}}
}

// Build implements core.StatelessWidget.
func (p page) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return collection.List{
		ItemCount: rows, ItemExtent: 52,
		ItemBuilder: func(i int) gowaku.Widget { return p.row(th, i) },
	}
}

// measuredPage shows rows of text that wrap to different heights, which no one
// knows before the list lays them out, and jumps to rows nobody measured.
type measuredPage struct{ gowaku.StatefulBase }

type measuredState struct {
	gowaku.StateBase
	list collection.ListController
}

// CreateState implements core.StatefulWidget.
func (measuredPage) CreateState() gowaku.State { return &measuredState{} }

var sentence = []string{
	"Rows of wrapping text", "take the height", "their words need,", "which the list learns",
	"only by laying them out.", "Rows not laid out yet", "count as the mean", "of those it has seen.",
}

// note returns the text of row i: one to eight phrases, so up to a few lines.
func note(i int) string {
	n := 1 + i*7%8
	s := fmt.Sprintf("%d.", i+1)
	for k := range n {
		s += " " + sentence[(i+k)%len(sentence)]
	}
	return s
}

// Build implements core.State.
func (s *measuredState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	jump := func(label string, fn func()) gowaku.Widget {
		return control.Button{Label: label, OnPress: fn}
	}
	return basic.Column{Children: []gowaku.Widget{
		basic.Padding{Insets: geom.All(8), Child: basic.Row{Spacing: 8, Children: []gowaku.Widget{
			jump("先頭", func() { s.list.JumpToIndex(0, 0) }),
			jump("50,000 行目", func() { s.list.JumpToIndex(49999, 0.5) }),
			jump("末尾", s.list.JumpToEnd),
		}}},
		basic.Expanded{Child: collection.List{
			ItemCount: measuredRows, MeasureItems: true, Controller: &s.list,
			ItemBuilder: func(i int) gowaku.Widget {
				return basic.Padding{Insets: geom.Symmetric(16, 8), Child: basic.Text{Content: note(i), Style: th.Text.Body}}
			},
		}},
	}}
}

const measuredRows = 100000

// bench scrolls down the list for a while, a few notches per wheel event.
func bench(root gowaku.Widget, length time.Duration) error {
	h, err := shot.Open(root, shot.Options{Width: 800, Height: 900, Immediate: true, Theme: shot.DefaultTheme()})
	if err != nil {
		return err
	}
	defer h.Close()
	fmt.Println("adapter:", h.AdapterName)
	if _, err := h.Frame(); err != nil {
		return err
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	frames := 0
	var worst, busy time.Duration
	var slow []int
	instances := 0
	for time.Since(start) < length {
		begin := time.Now()
		if frames%6 == 0 {
			h.Tree.DispatchPointer(core.PointerEvent{
				Kind: core.PointerScroll, Pos: geom.Offset{X: 200, Y: 200}, Scroll: geom.Offset{Y: -3},
			})
		}
		if instances, err = h.Frame(); err != nil {
			return err
		}
		h.Pump()
		took := time.Since(begin)
		busy += took
		worst = max(worst, took)
		if took > time.Second/120 {
			slow = append(slow, frames)
		}
		frames++
	}
	runtime.ReadMemStats(&after)
	total := time.Since(start)
	fmt.Printf("%d frames, %.1f fps, mean %.2f ms, worst %.2f ms, over 8.3 ms: frames %v, %d instances, %.1f allocs/frame\n",
		frames, float64(frames)/total.Seconds(), float64(busy.Microseconds())/1000/float64(frames),
		float64(worst.Microseconds())/1000, slow, instances, float64(after.Mallocs-before.Mallocs)/float64(frames))
	return nil
}

func screenshot(root gowaku.Widget, path string) error {
	h, err := shot.Open(root, shot.Options{Width: 480, Height: 360, Theme: shot.DefaultTheme()})
	if err != nil {
		return err
	}
	defer h.Close()
	return h.Screenshot(path)
}

func main() {
	pngPath := flag.String("png", "", "write one frame to this file and exit")
	length := flag.Duration("bench", 0, "scroll for this long and print frame rates")
	measured := flag.Bool("measured", false, "show rows of wrapping text whose heights the list measures")
	flag.Parse()
	icon, err := paint.ParseSVGPath("M12 2l3 7h7l-5.5 4.5L18.5 21 12 16.8 5.5 21l2-7.5L2 9h7z")
	if err != nil {
		log.Fatal(err)
	}
	var root gowaku.Widget = page{icon: icon}
	if *measured {
		root = measuredPage{}
	}
	switch {
	case *length > 0:
		err = bench(root, *length)
	case *pngPath != "":
		err = screenshot(root, *pngPath)
	default:
		err = gowaku.Run(gowaku.App{Title: "gowaku: listview", Width: 480, Height: 640, Home: root})
	}
	if err != nil {
		log.Fatal(err)
	}
}
