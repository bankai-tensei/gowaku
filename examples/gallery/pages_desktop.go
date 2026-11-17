package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/editor"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// The desktop page: a small file manager. A tree of folders on
// the left, tabs with a table of 100,000 rows and two notes in the middle, and
// an inspector of the chosen row on the right, under a breadcrumb.

// folders maps each folder of the tree to what is inside it.
var folders = map[string][]string{
	"":                      {"プロジェクト", "書類", "写真"},
	"プロジェクト":                {"プロジェクト/gowaku", "プロジェクト/ウェブサイト"},
	"プロジェクト/gowaku":         {"プロジェクト/gowaku/widgets", "プロジェクト/gowaku/docs", "プロジェクト/gowaku/go.mod"},
	"プロジェクト/gowaku/widgets": {"プロジェクト/gowaku/widgets/table.go", "プロジェクト/gowaku/widgets/tree.go"},
	"プロジェクト/gowaku/docs":    {"プロジェクト/gowaku/docs/ARCHITECTURE.md"},
	"プロジェクト/ウェブサイト":         {"プロジェクト/ウェブサイト/index.html"},
	"書類":                    {"書類/請求書", "書類/議事録.txt"},
	"書類/請求書":                {"書類/請求書/2026-10.pdf", "書類/請求書/2026-11.pdf"},
	"写真":                    {"写真/旅行"},
	"写真/旅行":                 {"写真/旅行/京都.jpg"},
	"プロジェクト/gowaku/go.mod":  nil,
	"書類/議事録.txt":            nil,
	"プロジェクト/ウェブサイト/index.html": nil,
}

// base returns the last part of a path of the tree.
func base(p string) string { return p[strings.LastIndexByte(p, '/')+1:] }

// record is one row of the table.
type record struct {
	name, kind, owner string
	size              int
	shared            bool
	priority          int
	// state is an index of recordStates, and synced how much of the file is
	// synced, in percent.
	state, synced int
}

var (
	recordKinds  = []string{"書類", "画像", "表計算", "フォルダ"}
	recordOwners = []string{"佐藤", "鈴木", "高橋", "田中"}
	// recordStates are shown as badges in recordTones.
	recordStates = []string{"同期済み", "同期中", "エラー", "ローカル"}
	recordTones  = []collection.CellTone{collection.CellToneSuccess, collection.CellToneAccent, collection.CellToneDanger, collection.CellToneNeutral}
	// recordPriorities are shown as dots in priorityTones.
	recordPriorities = []string{"なし", "低", "普通", "高", "至急"}
	priorityTones    = []collection.CellTone{collection.CellToneNeutral, collection.CellToneNeutral, collection.CellToneAccent,
		collection.CellToneSuccess, collection.CellToneDanger}
	// percents spells 0 to 100 percent, ready for the cells of the table.
	percents = func() []string {
		out := make([]string, 101)
		for i := range out {
			out[i] = strconv.Itoa(i) + "%"
		}
		return out
	}()
)

// records makes the 100,000 rows of the table. The names repeat every 500
// rows, so that the shaped texts stay in the cache of the table.
func records() []record {
	out := make([]record, 100_000)
	for i := range out {
		out[i] = record{
			name: "ファイル " + strconv.Itoa(i%500+1), kind: recordKinds[i%len(recordKinds)],
			owner: recordOwners[i/3%len(recordOwners)], size: (i*7919)%9000 + 12, priority: i % 5,
			shared: i%3 == 1, state: i * 7 % 11 % len(recordStates),
		}
		switch out[i].state {
		case 0:
			out[i].synced = 100
		case 1, 2:
			out[i].synced = (i * 37) % 100
		}
	}
	return out
}

const memo = `会議のメモ

TODO: 表の列の幅を保存する
TODO: ツリーの並べ替えを試す

決まったこと:
- 新しい部品は gallery の「デスクトップ」に置く
- タブの引き剥がしは examples/tabs で見せる
- 編集面にはコード専用の機能をいれない

次の会議は 11 月 16 日。`

// desktopState is what the desktop page keeps.
type desktopState struct {
	tree     collection.TreeController[string]
	place    string
	tabs     []nav.TabItem
	tab      int
	rows     []record
	order    []int // the rows in the order the table shows them
	table    collection.TableController
	sort     collection.TableSort
	kinds    collection.TableFilter
	notes    editor.Controller
	nameW    float32
	initDone bool
}

func (d *desktopState) init() {
	if d.initDone {
		return
	}
	d.initDone = true
	d.tree.Expand("プロジェクト")
	d.tree.Expand("プロジェクト/gowaku")
	d.tree.Expand("プロジェクト/gowaku/widgets")
	d.place = "プロジェクト/gowaku/widgets"
	d.tree.Select(d.place)
	d.tabs = []nav.TabItem{{Key: 1, Label: "ファイル一覧"}, {Key: 2, Label: "会議のメモ.txt"}, {Key: 3, Label: "README.md"}}
	d.rows = records()
	d.refilter()
	d.table.SetSelection(collection.RowRange{Start: 1, End: 2})
	d.table.SetFocusedCell(1, 0)
	d.notes.SetText(memo)
}

// refilter rebuilds the order of the rows from the filter and the sort.
func (d *desktopState) refilter() {
	d.order = d.order[:0]
	for i, r := range d.rows {
		if !d.kinds.Active() || slices.Contains(d.kinds.Values, r.kind) {
			d.order = append(d.order, i)
		}
	}
	if d.sort.Order == collection.SortNone {
		return
	}
	less := func(a, b int) int {
		ra, rb := d.rows[a], d.rows[b]
		switch d.sort.Column {
		case 0:
			return strings.Compare(ra.name, rb.name)
		case 2:
			return ra.size - rb.size
		}
		return a - b
	}
	slices.SortStableFunc(d.order, func(a, b int) int {
		if d.sort.Order == collection.SortDescending {
			return less(b, a)
		}
		return less(a, b)
	})
}

// focused returns the row the inspector shows: the focused row of the table.
func (d *desktopState) focused() *record {
	row, _ := d.table.FocusedCell()
	if row < 0 || row >= len(d.order) {
		return nil
	}
	return &d.rows[d.order[row]]
}

func (s *galleryState) desktop(th *theme.Theme) gowaku.Widget {
	d := &s.desk
	d.init()
	return basic.Column{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		basic.Pad(theme.Space12, s.trail()),
		basic.Expanded{Child: control.SplitPane{
			First: basic.Pad(theme.Space8, collection.Tree[string]{
				Label: "フォルダ", Controller: &d.tree, Roots: folders[""],
				ChildrenOf:  func(k string) []string { return folders[k] },
				HasChildren: func(k string) bool { return len(folders[k]) > 0 },
				LabelOf:     base,
				OnSelectionChange: func(sel []string) {
					if len(sel) > 0 {
						s.set(func() { d.place = sel[len(sel)-1] })
					}
				},
				OnRename: func(string, string) {},
			}),
			Second: control.SplitPane{
				First:    s.documents(th),
				Second:   s.inspector(),
				Position: 0.66, MinFirst: 280, MinSecond: 200,
				Collapsible: control.SplitCollapseSecond, Label: "インスペクタの区切り",
			},
			Position: 200, Pixels: true, Default: 200, MinFirst: 140, MinSecond: 400,
			Collapsible: control.SplitCollapseFirst, Label: "フォルダの区切り",
		}},
	}}
}

// trail is the breadcrumb of the folder chosen in the tree.
func (s *galleryState) trail() gowaku.Widget {
	d := &s.desk
	parts := strings.Split(d.place, "/")
	items := make([]nav.BreadcrumbItem, len(parts))
	for i, p := range parts {
		items[i] = nav.BreadcrumbItem{Label: p}
	}
	return nav.Breadcrumb{Items: items, OnSelect: func(i int) {
		place := strings.Join(parts[:i+1], "/")
		s.set(func() {
			d.place = place
			d.tree.Select(place)
			d.tree.Focus(place)
		})
	}}
}

// documents is the tab bar and the page of the chosen tab.
func (s *galleryState) documents(th *theme.Theme) gowaku.Widget {
	d := &s.desk
	var page gowaku.Widget = basic.Center{Child: basic.Text{Content: "タブがありません"}}
	if d.tab < len(d.tabs) {
		switch d.tabs[d.tab].Key {
		case 1:
			page = s.fileTable()
		case 2:
			page = editor.Editor{Controller: &d.notes, Label: "会議のメモ", LineNumbers: true, Decorations: todos(th)}
		default:
			page = editor.Editor{Label: "README", Wrap: true}
		}
	}
	return basic.Column{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		nav.TabBar{
			Tabs: d.tabs, Selected: d.tab, Label: "開いている書類",
			OnSelect: func(i int) { s.set(func() { d.tab = i }) },
			OnClose: func(i int) {
				s.set(func() {
					d.tabs = slices.Delete(d.tabs, i, i+1)
					if d.tab >= len(d.tabs) {
						d.tab = max(len(d.tabs)-1, 0)
					}
				})
			},
			OnMove: func(from, to int) {
				s.set(func() {
					t := d.tabs[from]
					d.tabs = slices.Insert(slices.Delete(d.tabs, from, from+1), to, t)
				})
			},
		},
		basic.Expanded{Child: page},
	}}
}

// todos colors the word TODO at the start of a line of the notes.
func todos(th *theme.Theme) func(int, string, []editor.Decoration) []editor.Decoration {
	accent, danger := th.Color.AccentInk, th.Color.Danger
	return func(_ int, line string, dst []editor.Decoration) []editor.Decoration {
		if strings.HasPrefix(line, "TODO:") {
			dst = append(dst, editor.Decoration{Start: 0, End: 5, Color: accent, Bold: true})
		}
		// A spelling checker would mark this mistake of okurigana.
		if i := strings.Index(line, "いれない"); i >= 0 {
			dst = append(dst, editor.Decoration{Start: i, End: i + len("いれない"),
				Underline: editor.UnderlineSquiggle, UnderlineColor: danger})
		}
		return dst
	}
}

// fileTable is the table of 100,000 rows.
func (s *galleryState) fileTable() gowaku.Widget {
	d := &s.desk
	at := func(i int) *record { return &d.rows[i] }
	return collection.Table[int]{
		Label: "ファイル", Rows: d.order, Controller: &d.table, Frozen: 1, Sort: d.sort,
		OnSort: func(o collection.TableSort) {
			s.set(func() { d.sort = o; d.refilter(); d.table.ClearSelection() })
		},
		OnFilter: func(_ int, f collection.TableFilter) {
			s.set(func() { d.kinds = f; d.refilter(); d.table.ClearSelection() })
		},
		OnCommit: func(e collection.TableEdit) bool {
			if strings.TrimSpace(e.Text) == "" {
				return false
			}
			s.set(func() { at(d.order[e.Row]).name = e.Text })
			return true
		},
		OnSelectionChange: func() { s.set(func() {}) },
		Columns: []collection.TableColumn[int]{
			{Title: "名前", Width: 160, Text: func(i int) string { return at(i).name }, Sortable: true, Editable: true},
			{Title: "種類", Width: 110, Text: func(i int) string { return at(i).kind }, Filterable: true,
				FilterValues: recordKinds, Filter: d.kinds},
			{Title: "サイズ", Width: 100, Trailing: true, Sortable: true,
				Text: func(i int) string { return strconv.Itoa(at(i).size) + " KB" }},
			{Title: "所有者", Width: 100, Text: func(i int) string { return at(i).owner }},
			{Title: "状態", Width: 110, Kind: collection.CellBadge, Text: func(i int) string { return recordStates[at(i).state] },
				Tone: func(i int) collection.CellTone { return recordTones[at(i).state] }},
			{Title: "同期", Width: 150, Kind: collection.CellProgress, Text: func(i int) string { return percents[at(i).synced] },
				Value: func(i int) float32 { return float32(at(i).synced) / 100 },
				Tone:  func(i int) collection.CellTone { return recordTones[at(i).state] }},
			{Title: "共有", Width: 70, Kind: collection.CellCheck, Checked: func(i int) bool { return at(i).shared },
				OnCheck: func(row int, on bool) { s.set(func() { at(d.order[row]).shared = on }) }},
			{Title: "優先度", Width: 100, Kind: collection.CellDot, Text: func(i int) string { return recordPriorities[at(i).priority] },
				Tone: func(i int) collection.CellTone { return priorityTones[at(i).priority] }},
		},
	}
}

// inspector is the property grid of the focused row.
func (s *galleryState) inspector() gowaku.Widget {
	d := &s.desk
	r := d.focused()
	if r == nil {
		return basic.Center{Child: basic.Text{Content: "行を選んでください"}}
	}
	kinds := make([]field.DropdownOption[string], len(recordKinds))
	for i, k := range recordKinds {
		kinds[i] = field.DropdownOption[string]{Value: k, Label: k}
	}
	edit := func(fn func()) { s.set(fn) }
	return collection.PropertyGrid{
		Label: "プロパティ", Filter: true, NameWidth: d.nameW,
		OnNameWidthChange: func(w float32) { d.nameW = w },
		Categories: []collection.PropertyCategory{
			{Name: "基本", Properties: []collection.Property{
				{Name: "名前", Description: "一覧と検索に出る名前です。", Editor: collection.PropertyText{
					Value: r.name, OnChange: func(v string) { edit(func() { r.name = v }) }}},
				{Name: "種類", Editor: collection.PropertyChoice[string]{
					Value: r.kind, Options: kinds, OnChange: func(v string) { edit(func() { r.kind = v }) }}},
				{Name: "サイズ", Description: "変えられません。", Editor: collection.PropertyText{Value: strconv.Itoa(r.size) + " KB"}},
			}},
			{Name: "共有", Properties: []collection.Property{
				{Name: "共有する", Description: "チームの全員が開けるようにします。", Editor: collection.PropertyBool{
					Value: r.shared, OnChange: func(v bool) { edit(func() { r.shared = v }) }}},
				{Name: "優先度", Editor: collection.PropertyInt{
					Value: r.priority, Min: 0, Max: 4, OnChange: func(v int) { edit(func() { r.priority = v }) }}},
				{Name: "ラベルの色", Editor: collection.PropertyColor{Value: geom.Hex(0x2f5fd0)}},
			}},
		},
	}
}
