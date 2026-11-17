package main

import (
	"cmp"
	"math"
	"slices"
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The page of what is seen rather than typed: the color picker,
// skeletons while something loads, and sparklines in the cells of a table.

// pageVisual returns the page of the color picker, the skeletons and the
// sparklines.
func pageVisual() core.Widget { return visualPage{} }

// visualPage is the page; it keeps the color and the loading switch.
type visualPage struct{ core.StatefulBase }

type visualPageState struct {
	core.StateBase
	color, tint geom.Color
	loaded      bool
	series      []visualSeries
	// order is the rows of the table of sparklines, indexes of series in the
	// order of sort.
	order []int
	sort  collection.TableSort
}

// visualSeries is a row of the table of sparklines. Its sparklines and the
// texts that name them are made once, as the table asks of what it shows.
type visualSeries struct {
	name, last          string
	now                 float64
	trend               []float64
	change              []float64
	trendText, diffText string
	trendCell, diffCell core.Widget
}

func (visualPage) CreateState() core.State { return &visualPageState{} }

func (s *visualPageState) InitState() {
	s.color, s.tint = geom.Hex(0x3a7bf0), geom.Color{R: 0.2, G: 0.7, B: 0.45, A: 0.8}
	names := [...]string{"東京", "大阪", "名古屋", "札幌", "福岡"}
	s.series = make([]visualSeries, len(names))
	for k, name := range names {
		trend, change := make([]float64, 24), make([]float64, 12)
		for i := range trend {
			trend[i] = 20 + 6*math.Sin(float64(i+k*3)/3.5) + float64(i*k%7)/2
		}
		// A sensor that dropped out leaves a gap in one series.
		if k == 2 {
			trend[9], trend[10] = math.NaN(), math.NaN()
		}
		for i := range change {
			change[i] = math.Sin(float64(i*(k+1))/2.3) * float64(3+k)
		}
		v := visualSeries{name: name, trend: trend, change: change, now: trend[len(trend)-1]}
		v.last = formatOne(v.now) + "℃"
		low, high := spanOf(trend)
		v.trendText = "最低 " + formatOne(low) + "℃、最高 " + formatOne(high) + "℃"
		low, high = spanOf(change)
		v.diffText = "前日との差 " + formatOne(low) + "〜" + formatOne(high) + "℃"
		v.trendCell = control.Sparkline{Data: trend, Height: 20, Fill: true, MarkLast: true, Label: name + "の気温、現在 " + v.last}
		v.diffCell = control.Sparkline{Kind: control.SparklineBars, Data: change, Height: 20, Label: name + "の" + v.diffText}
		s.series[k] = v
		s.order = append(s.order, k)
	}
}

// spanOf returns the smallest and the largest number of data, past the gaps.
func spanOf(data []float64) (low, high float64) {
	low, high = math.Inf(1), math.Inf(-1)
	for _, v := range data {
		if !math.IsNaN(v) {
			low, high = min(low, v), max(high, v)
		}
	}
	return low, high
}

// sortSeries puts the rows of the table of sparklines in the order of o: by
// name, or by the temperature now.
func (s *visualPageState) sortSeries(o collection.TableSort) {
	s.sort = o
	s.order = s.order[:0]
	for k := range s.series {
		s.order = append(s.order, k)
	}
	if o.Order == collection.SortNone {
		return
	}
	slices.SortStableFunc(s.order, func(a, b int) int {
		x, y := s.series[a], s.series[b]
		c := cmp.Compare(x.now, y.now)
		if o.Column == 0 {
			c = cmp.Compare(x.name, y.name)
		}
		if o.Order == collection.SortDescending {
			c = -c
		}
		return c
	})
}

// formatOne spells v with one decimal.
func formatOne(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

func (s *visualPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Scroll{Child: basic.Pad(theme.Space28, basic.Column{
		Size: layout.MainMin, Spacing: theme.Space24, Cross: layout.CrossStretch, Children: []core.Widget{
			heading(th, "色と読み込み", "色の選択、読み込み中の表示、表のセルに入る小さなグラフ。"),
			s.pickers(th),
			s.skeletons(th),
			s.sparklines(th),
		},
	})}
}

// section is a titled group of the page.
func section(th *theme.Theme, title string, child core.Widget) core.Widget {
	style := th.Text.Headline
	style.Color = th.Color.Ink
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		basic.Text{Content: title, Style: style},
		child,
	}}
}

// caption is the style of a small note.
func caption(th *theme.Theme) text.Style {
	style := th.Text.Caption
	style.Color = th.Color.Ink2
	return style
}

// pickers shows a picker inline with a live preview, and the picker that a
// color property opens.
func (s *visualPageState) pickers(th *theme.Theme) core.Widget {
	set := func(c geom.Color) { s.SetState(func() { s.color = c }) }
	// Dark ink on light colors, white on the rest.
	ink := geom.Hex(0xffffff)
	if hsv := theme.HSVFromColor(s.color); hsv.V > 0.6 && hsv.S < 0.5 || hsv.A < 0.5 {
		ink = geom.Hex(0x1d1d1f)
	}
	title := th.Text.Title
	title.Color = ink
	body := th.Text.Body
	body.Color = ink.WithAlpha(0.85)
	preview := basic.Container{
		Width: 260, Height: 150, Radius: th.Radius.Group, Color: s.color, Padding: geom.All(theme.Space16),
		Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
		Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space4, Children: []core.Widget{
			basic.Text{Content: "プレビュー", Style: title},
			basic.Text{Content: theme.FormatHex(s.color), Style: body},
		}},
	}
	grid := basic.SizedBox{Width: 260, Height: 200, Child: collection.PropertyGrid{Label: "外観", Categories: []collection.PropertyCategory{
		{Name: "外観", Properties: []collection.Property{
			{Name: "アクセント", Description: "押すと色の選択が開きます。", Editor: collection.PropertyColor{Value: s.color, OnChange: set}},
			{Name: "強調", Editor: collection.PropertyColor{Value: s.tint, OnChange: func(c geom.Color) { s.SetState(func() { s.tint = c }) }}},
		}},
	}}}
	picker := field.ColorPicker{
		Value: s.color, Label: "アクセントカラー", OnChange: set,
		Swatches: []geom.Color{
			geom.Hex(0x3a7bf0), geom.Hex(0x5e5ce6), geom.Hex(0xbf5af2), geom.Hex(0xff375f),
			geom.Hex(0xff9f0a), geom.Hex(0x30d158), geom.Hex(0x64d2ff), geom.Hex(0x8e8e93),
		},
	}
	return section(th, "色の選択", basic.Row{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space28, Children: []core.Widget{
		control.Card{Label: "色の選択", Child: picker},
		basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space16, Children: []core.Widget{
			preview,
			basic.Text{Content: "プロパティから開く", Style: caption(th)},
			grid,
		}},
	}})
}

// skeletons shows a card while it loads, beside the same card loaded, and a
// switch that loads the first one.
func (s *visualPageState) skeletons(th *theme.Theme) core.Widget {
	loaded := func() core.Widget {
		name := th.Text.Headline
		name.Color = th.Color.Ink
		body := th.Text.Body
		body.Color = th.Color.Ink2
		return basic.Row{Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
			basic.Container{Width: 40, Height: 40, Radius: 20, Color: th.Color.AccentTint, Child: basic.Center{Child: basic.Text{Content: "佐", Style: name}}},
			basic.Expanded{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space4, Children: []core.Widget{
				basic.Text{Content: "佐藤 花子", Style: name},
				basic.Text{Content: "来週の打ち合わせの資料を共有しました。", Style: body},
				basic.Text{Content: "3 分前", Style: body},
			}}},
		}}
	}
	placeholder := func() core.Widget {
		return control.SkeletonGroup{Label: "メッセージを読み込み中", Child: basic.Row{Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
			control.Skeleton{Shape: control.SkeletonCircle, Width: 40},
			basic.Expanded{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: []core.Widget{
				control.Skeleton{Width: 90, Height: 14},
				control.SkeletonText{Lines: 2},
			}}},
		}}}
	}
	first := placeholder()
	if s.loaded {
		first = loaded()
	}
	card := func(child core.Widget) core.Widget {
		return basic.SizedBox{Width: 280, Child: control.Card{Child: child}}
	}
	return section(th, "読み込み中の表示", basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []core.Widget{
			control.Switch{Label: "読み込み完了", Value: s.loaded, OnChange: func(v bool) { s.SetState(func() { s.loaded = v }) }},
			basic.Text{Content: "読み込み完了", Style: caption(th)},
		}},
		basic.Row{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space16, Children: []core.Widget{
			card(first),
			card(loaded()),
		}},
		basic.SizedBox{Width: 576, Child: control.Card{Child: control.SkeletonGroup{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space12, Children: []core.Widget{
			control.Skeleton{Shape: control.SkeletonBlock, Height: 96},
			control.SkeletonText{Lines: 3},
		}}}}},
	}})
}

// sparklines shows a small table whose cells hold sparklines: the trend of a
// day as a line, and the change of each hour as bars. The rows sort by name
// and by the temperature now; the sparklines move with their rows.
func (s *visualPageState) sparklines(th *theme.Theme) core.Widget {
	at := func(k int) *visualSeries { return &s.series[k] }
	const rowHeight = 36
	table := collection.Table[int]{
		Label: "気温", Rows: s.order, RowHeight: rowHeight, Sort: s.sort,
		OnSort: func(o collection.TableSort) { s.SetState(func() { s.sortSeries(o) }) },
		Columns: []collection.TableColumn[int]{
			{Title: "地点", Width: 96, Text: func(k int) string { return at(k).name }, Sortable: true},
			{Title: "24 時間の気温", Width: 168, Kind: collection.CellWidget,
				Text:   func(k int) string { return at(k).trendText },
				Widget: func(k int) core.Widget { return at(k).trendCell }},
			{Title: "現在", Width: 72, Trailing: true, Text: func(k int) string { return at(k).last }, Sortable: true},
			{Title: "前日との差", Width: 144, Kind: collection.CellWidget,
				Text:   func(k int) string { return at(k).diffText },
				Widget: func(k int) core.Widget { return at(k).diffCell }},
		},
	}
	height := float32(32 + rowHeight*len(s.series))
	return section(th, "表のスパークライン", control.Card{Label: "気温", Child: basic.SizedBox{Width: 480, Height: height, Child: table}})
}
