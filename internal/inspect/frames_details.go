//go:build gowakudebug

package inspect

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// The rows of the details.
const (
	frameDetailRow = 18
	rebuiltRow     = 22
	detailNumbersW = 176
)

// frameDetails shows the numbers of the selected frame and the elements that
// it rebuilt, which select themselves in the trees when clicked.
type frameDetails struct {
	core.StatelessBase
	Session *Session
}

// Build implements core.StatelessWidget.
func (d frameDetails) Build(ctx core.BuildContext) core.Widget {
	ses := d.Session
	str, fs := stringsOf(ctx), framesStringsOf(ctx)
	th := basic.ThemeOf(ctx)
	caption := th.Text.Caption.WithColor(th.Color.Ink2)
	i := ses.SelectedFrame
	if i < 0 || i >= ses.Frames() {
		return basic.Padding{Insets: geom.All(theme.Space12), Child: basic.Center{
			Child: basic.Text{Content: str.noFrame, Style: caption},
		}}
	}
	f := ses.Frame(i)

	title := fs.closedWindow
	if f.Target != nil {
		title = windowName(f.Target, str)
	}
	header := basic.Row{
		Spacing: theme.Space8,
		Children: []core.Widget{
			basic.Text{Content: fs.frame + strconv.FormatUint(f.Seq, 10), Style: th.Text.Headline},
			basic.Expanded{Child: basic.Text{Content: title, Style: caption}},
		},
	}

	colors := stageColors(th.Brightness)
	rows := make([]core.Widget, 0, StageCount+6)
	for st := range StageCount {
		rows = append(rows, numberRow(th, colors[st], str.stageNames[st], msText(f.Stages[st], fs)))
	}
	gpu := "—"
	if f.GPU > 0 {
		gpu = msText(f.GPU, fs)
	}
	repainted := fs.no
	if f.Repainted {
		repainted = fs.yes
	}
	rows = append(rows,
		numberRow(th, geom.Color{}, fs.workShort, msText(f.Work(), fs)),
		numberRow(th, geom.Color{}, fs.cpu, msText(f.CPU(), fs)),
		numberRow(th, geom.Color{}, str.gpu, gpu),
		numberRow(th, geom.Color{}, str.allocs, strconv.FormatUint(f.Allocs, 10)),
		numberRow(th, geom.Color{}, str.laidOut, strconv.Itoa(f.LaidOut)),
		numberRow(th, geom.Color{}, str.recorded, strconv.Itoa(f.RecordedCount)),
		numberRow(th, geom.Color{}, str.repainted, repainted),
	)
	numbers := basic.SizedBox{Width: detailNumbersW, Child: basic.Scroll{Child: basic.Column{
		Cross: layout.CrossStretch, Children: rows,
	}}}

	return basic.Padding{
		Insets: geom.Symmetric(theme.Space12, theme.Space8),
		Child: basic.Column{
			Cross:   layout.CrossStretch,
			Spacing: theme.Space6,
			Children: []core.Widget{
				header,
				basic.Expanded{Child: basic.Row{
					Cross:   layout.CrossStretch,
					Spacing: theme.Space12,
					Children: []core.Widget{
						numbers,
						basic.Expanded{Child: rebuiltList(ses, f, th, str, fs)},
					},
				}},
			},
		},
	}
}

// numberRow is a line of the details: a swatch for a stage, the name and the
// number at the right.
func numberRow(th *theme.Theme, swatch geom.Color, name, value string) core.Widget {
	style := th.Text.Caption.WithColor(th.Color.Ink2)
	lead := core.Widget(basic.SizedBox{Width: 8})
	if swatch != (geom.Color{}) {
		lead = basic.Container{Width: 8, Height: 8, Radius: 2, Color: swatch}
	}
	return interact.Semantics{Label: name, Value: value, Child: basic.SizedBox{Height: frameDetailRow, Child: basic.Row{
		Spacing: theme.Space6,
		Children: []core.Widget{
			lead,
			basic.Expanded{Child: basic.Text{Content: name, Style: style}},
			basic.Text{Content: value, Style: th.Text.Caption.WithColor(th.Color.Ink)},
		},
	}}}
}

// rebuiltList lists the elements that f rebuilt, with how many more there
// were beyond those named.
func rebuiltList(ses *Session, f *Frame, th *theme.Theme, str *inspectStrings, fs *framesStrings) core.Widget {
	seq := f.Seq
	named := len(f.Rebuilt)
	more := f.RebuiltCount - named
	count := named
	if more > 0 {
		count++
	}
	selTarget, selElement, _ := ses.Selection()
	list := collection.List{
		ItemCount:  count,
		ItemExtent: rebuiltRow,
		ItemBuilder: func(i int) core.Widget {
			// The ring may have reused the record since the last build.
			if f.Seq != seq || i >= len(f.Rebuilt) {
				if i == named && more > 0 {
					return basic.Padding{Insets: geom.Symmetric(theme.Space6, 2), Child: basic.Text{
						Content: fs.more + strconv.Itoa(more) + fs.moreEnd, Style: th.Text.Caption.WithColor(th.Color.Ink2),
					}}
				}
				return basic.SizedBox{}
			}
			el := f.Rebuilt[i]
			return rebuiltItem(ses, f.Target, el, th, f.Target == selTarget && el == selElement)
		},
	}
	return basic.Column{
		Cross:   layout.CrossStretch,
		Spacing: theme.Space2,
		Children: []core.Widget{
			basic.Row{Spacing: theme.Space6, Children: []core.Widget{
				basic.Text{Content: str.rebuilt, Style: th.Text.Section},
				basic.Text{Content: strconv.Itoa(f.RebuiltCount), Style: th.Text.Caption.WithColor(th.Color.Ink2)},
			}},
			basic.Expanded{Child: list},
		},
	}
}

// rebuiltItem is a pressable line naming el, which selects it. It is
// disabled when el is gone or its window closed.
func rebuiltItem(ses *Session, t Target, el core.ElementView, th *theme.Theme, selected bool) core.Widget {
	name := elementName(el)
	var press func()
	if t != nil && !el.IsZero() && el.Mounted() {
		press = func() { ses.Select(t, el) }
	}
	var state semantics.State
	if selected {
		state = semantics.Selected
	}
	return interact.Pressable{
		Label: name, State: state, OnPress: press,
		Build: func(ps interact.PressState) core.Widget {
			var bg geom.Color
			switch {
			case selected:
				bg = th.Color.AccentTint
			case ps.Hovered && press != nil:
				bg = th.Color.Fill
			}
			ink := th.Color.Ink
			if press == nil {
				ink = th.Color.Ink3
			}
			var border geom.Color
			if ps.FocusVisible {
				border = th.Color.AccentInk
			}
			return basic.Container{
				Color: bg, Radius: 5, Border: strokeOf(border),
				Padding: geom.Symmetric(theme.Space6, 2),
				Child:   basic.Text{Content: name, Style: th.Text.Caption.WithColor(ink)},
			}
		},
	}
}

// elementName is the type of the widget of el, with its key if it has one.
func elementName(el core.ElementView) string {
	if el.IsZero() {
		return "<nil>"
	}
	name := TypeName(el.Widget())
	if k := el.Key(); k != nil {
		name += " (key " + keyText(k) + ")"
	}
	return name
}

// keyText spells a key.
func keyText(k core.Key) string {
	switch k := k.(type) {
	case string:
		return strconv.Quote(k)
	case int:
		return strconv.Itoa(k)
	}
	return fmt.Sprint(k)
}

// scaleOf returns the top of the graph and the most allocations of a frame.
// The scale follows the work and the GPU time, not present: one long wait,
// such as while the window is dragged, would flatten every other bar. A bar
// taller than the top is cut at it, with a mark.
func scaleOf(ses *Session) (time.Duration, uint64) {
	var tallest time.Duration
	var allocs uint64
	for i := range ses.Frames() {
		f := ses.Frame(i)
		tallest = max(tallest, f.Work(), f.GPU)
		allocs = max(allocs, f.Allocs)
	}
	return graphTop(tallest), allocs
}

// summaryOf sums up the frames kept: the average and the longest work, and
// how many frames went over the budget of 60 frames a second. The wait in
// present is left out: a frame that waits for vsync is not slow.
func summaryOf(ses *Session, fs *framesStrings) string {
	n := ses.Frames()
	if n == 0 {
		return ""
	}
	var sum, longest time.Duration
	over := 0
	for i := range n {
		cpu := ses.Frame(i).Work()
		sum += cpu
		longest = max(longest, cpu)
		if cpu > budget60 {
			over++
		}
	}
	return fs.workShort + " " + fs.average + " " + msText(sum/time.Duration(n), fs) +
		"  ·  " + fs.maximum + " " + msText(longest, fs) +
		"  ·  " + fs.overBudget + " " + strconv.Itoa(over) + " / " + strconv.Itoa(n)
}

// frameReadout reads out the numbers of the hovered frame.
func frameReadout(f *Frame, str *inspectStrings, fs *framesStrings) string {
	b := make([]byte, 0, 160)
	b = append(b, '#')
	b = strconv.AppendUint(b, f.Seq, 10)
	b = append(b, "   "...)
	b = append(b, fs.workShort...)
	b = append(b, ' ')
	b = appendMS(b, f.Work())
	b = append(b, fs.ms...)
	for st := range StageCount {
		b = append(b, "  ·  "...)
		b = append(b, str.stageNames[st]...)
		b = append(b, ' ')
		b = appendMS(b, f.Stages[st])
	}
	if f.GPU > 0 {
		b = append(b, "   GPU "...)
		b = appendMS(b, f.GPU)
		b = append(b, fs.ms...)
	}
	b = append(b, "   "...)
	b = strconv.AppendUint(b, f.Allocs, 10)
	b = append(b, ' ')
	b = append(b, fs.allocsAxis...)
	return string(b)
}

// appendMS appends d in milliseconds, with two decimals.
func appendMS(b []byte, d time.Duration) []byte {
	return strconv.AppendFloat(b, float64(d)/float64(time.Millisecond), 'f', 2, 64)
}

// msText spells d in milliseconds, with two decimals.
func msText(d time.Duration, fs *framesStrings) string {
	return string(appendMS(make([]byte, 0, 12), d)) + fs.ms
}

// msLabel spells a line of the graph: one decimal, none for whole numbers.
func msLabel(d time.Duration, fs *framesStrings) string {
	ms := float64(d) / float64(time.Millisecond)
	prec := 1
	if ms == float64(int64(ms)) {
		prec = 0
	}
	return strconv.FormatFloat(ms, 'f', prec, 64) + fs.ms
}

// strokeOf is a border of col, or none for the zero color.
func strokeOf(col geom.Color) (s paint.Stroke) {
	if col != (geom.Color{}) {
		s = paint.Stroke{Width: 1.5, Color: col}
	}
	return s
}
