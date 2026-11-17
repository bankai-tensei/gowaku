package field

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// paletteRig is a window with a CommandPalette over a page with a button,
// whose actions record what they ran.
type paletteRig struct {
	t       *testing.T
	tree    *core.Tree
	host    *wtest.FakeHost
	swap    func(core.Widget)
	palette CommandPalette
	ran     []string
	now     time.Duration
	size    geom.Size
	ctx     core.BuildContext
}

func (r *paletteRig) act(label string) interact.Action {
	return interact.Action{Label: label, OnInvoke: func() { r.ran = append(r.ran, paletteLabel(label)) }}
}

// demoActions are the actions of an editor, 書き出し disabled.
func (r *paletteRig) demoActions() []interact.Action {
	ctrl := input.ModControl
	acts := []interact.Action{
		r.act("新規作成(&N)"),
		r.act("ファイルを開く(&O)…"),
		r.act("上書き保存(&S)"),
		r.act("名前を付けて保存(&A)…"),
		{Label: "書き出し(&E)…"},
		r.act("縦書き(&V)"),
		r.act("Toggle Sidebar"),
	}
	acts[2].Shortcut = interact.Shortcut{Key: input.KeyS, Modifiers: ctrl}
	acts[5].Checkable, acts[5].Checked = true, true
	return acts
}

func newPaletteRig(t *testing.T, configure func(r *paletteRig, p *CommandPalette)) *paletteRig {
	t.Helper()
	r := &paletteRig{t: t, host: &wtest.FakeHost{}, now: time.Second, size: geom.Size{W: 800, H: 600}}
	r.palette = CommandPalette{Actions: r.demoActions()}
	if configure != nil {
		configure(r, &r.palette)
	}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.tree.SetHost(r.host)
	r.frame()
	return r
}

func (r *paletteRig) widget() core.Widget {
	p := r.palette
	p.Child = wtest.Capture{Got: &r.ctx, Child: basic.Align{Alignment: layout.AlignTopLeft, Child: control.Button{
		Label: "ページのボタン", OnPress: func() { r.ran = append(r.ran, "ページのボタン") },
	}}}
	return overlay.Overlay{Child: p}
}

// set rebuilds the window with p as the palette.
func (r *paletteRig) set(p CommandPalette) {
	r.palette = p
	r.swap(r.widget())
	r.frame()
}

func (r *paletteRig) frame() { testutil.RenderTree(r.tree, r.size) }

func (r *paletteRig) settle() { wtest.Settled(r.t, r.tree, r.size, &r.now) }

func (r *paletteRig) key(k input.KeyCode, mods input.Modifiers) {
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	r.frame()
}

func (r *paletteRig) typeText(s string) {
	r.tree.DispatchText(s)
	r.frame()
}

// open opens the palette with its shortcut and lets it come in.
func (r *paletteRig) open() {
	r.t.Helper()
	r.key(input.KeyP, input.ModControl|input.ModShift)
	r.settle()
	if len(r.rows()) == 0 && r.find("No results") == nil {
		r.t.Fatal("the palette did not open")
	}
}

// rows returns the labels of the results, in order.
func (r *paletteRig) rows() []string {
	root, _ := r.tree.SemanticsTree()
	var out []string
	for _, n := range wtest.RolesOf(root, semantics.RoleListItem) {
		out = append(out, n.Label)
	}
	return out
}

func (r *paletteRig) row(label string) *semantics.Node {
	root, _ := r.tree.SemanticsTree()
	for _, n := range wtest.RolesOf(root, semantics.RoleListItem) {
		if n.Label == label {
			return n
		}
	}
	return nil
}

// selected returns the label of the selected result.
func (r *paletteRig) selected() string {
	root, _ := r.tree.SemanticsTree()
	for _, n := range wtest.RolesOf(root, semantics.RoleListItem) {
		if n.State&semantics.Selected != 0 {
			return n.Label
		}
	}
	return ""
}

func (r *paletteRig) find(label string) *semantics.Node {
	root, _ := r.tree.SemanticsTree()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == label {
			found = n
		}
		return found == nil
	})
	return found
}

func TestCommandPalette_OpensFiltersAndRuns(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.open()
	want := []string{"新規作成", "ファイルを開く", "上書き保存", "名前を付けて保存", "書き出し", "縦書き", "Toggle Sidebar"}
	if got := r.rows(); !slices.Equal(got, want) {
		t.Fatalf("the palette lists %v, want the actions in order without mnemonics: %v", got, want)
	}
	if n := r.row("上書き保存"); n.Value != "Ctrl+S" {
		t.Errorf("the shortcut hint is %q", n.Value)
	}
	if n := r.row("縦書き"); n.State&semantics.Checked == 0 {
		t.Error("a checked action does not say so")
	}
	if n := r.row("書き出し"); n.State&semantics.Disabled == 0 {
		t.Error("a disabled action is not marked disabled")
	}
	r.typeText("保存")
	if got := r.rows(); !slices.Equal(got, []string{"上書き保存", "名前を付けて保存"}) {
		t.Fatalf("保存 finds %v", got)
	}
	r.key(input.KeyDown, 0)
	r.key(input.KeyEnter, 0)
	if !slices.Equal(r.ran, []string{"名前を付けて保存"}) {
		t.Fatalf("Enter ran %v", r.ran)
	}
	r.settle()
	if len(r.rows()) != 0 {
		t.Error("the palette stays open after running a command")
	}
}

func TestCommandPalette_JapaneseAndFuzzyQueries(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"ファイル", []string{"ファイルを開く"}},
		{"ﾌｧｲﾙ", []string{"ファイルを開く"}},
		{"開く", []string{"ファイルを開く"}},
		{"新作", []string{"新規作成"}},
		{"tsb", []string{"Toggle Sidebar"}},
		{"ＴＯＧ", []string{"Toggle Sidebar"}},
		{"書", []string{"書き出し", "上書き保存", "縦書き"}},
		{"存保", nil},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			r := newPaletteRig(t, nil)
			r.open()
			r.typeText(c.query)
			if got := r.rows(); !slices.Equal(got, c.want) {
				t.Errorf("%q finds %v, want %v", c.query, got, c.want)
			}
			if len(c.want) == 0 && r.find("No results") == nil {
				t.Error("no results says nothing")
			}
		})
	}
}

func TestCommandPalette_EscapeRestoresTheFocus(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.key(input.KeyTab, 0)
	if got := wtest.FocusedLabel(r.tree); got != "ページのボタン" {
		t.Fatalf("the focus starts on %q", got)
	}
	r.open()
	r.typeText("縦")
	if got := r.rows(); !slices.Equal(got, []string{"縦書き"}) {
		t.Fatalf("typing did not reach the field: %v", got)
	}
	r.key(input.KeyEscape, 0)
	r.settle()
	if len(r.rows()) != 0 {
		t.Fatal("Escape did not close the palette")
	}
	if got := wtest.FocusedLabel(r.tree); got != "ページのボタン" {
		t.Errorf("the focus went back to %q", got)
	}
	if len(r.ran) != 0 {
		t.Errorf("closing ran %v", r.ran)
	}
	// It opens again, empty.
	r.open()
	if got := r.rows(); len(got) != 7 {
		t.Errorf("reopened, it lists %v", got)
	}
}

func TestCommandPalette_DisabledActionsAreNotInvoked(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.open()
	r.typeText("書き出")
	if got := r.rows(); !slices.Equal(got, []string{"書き出し"}) {
		t.Fatalf("got %v", got)
	}
	r.key(input.KeyEnter, 0)
	wtest.Tap(r.tree, wtest.Center(r.row("書き出し").Rect))
	r.frame()
	if len(r.ran) != 0 {
		t.Errorf("a disabled action ran: %v", r.ran)
	}
	if len(r.rows()) == 0 {
		t.Error("Enter on a disabled action closed the palette")
	}
}

func TestCommandPalette_ArrowsSkipDisabled(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.open()
	if got := r.selected(); got != "新規作成" {
		t.Fatalf("the first selection is %q", got)
	}
	for range 3 {
		r.key(input.KeyDown, 0)
	}
	if got := r.selected(); got != "名前を付けて保存" {
		t.Fatalf("three downs select %q", got)
	}
	r.key(input.KeyDown, 0)
	if got := r.selected(); got != "縦書き" {
		t.Errorf("down passes the disabled action to %q", got)
	}
	r.key(input.KeyUp, 0)
	if got := r.selected(); got != "名前を付けて保存" {
		t.Errorf("up passes the disabled action to %q", got)
	}
	for range 10 {
		r.key(input.KeyUp, 0)
	}
	if got := r.selected(); got != "新規作成" {
		t.Errorf("up stops at the first: %q", got)
	}
}

func TestCommandPalette_EnterDuringCompositionBelongsToTheIME(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.open()
	r.typeText("縦")
	r.tree.DispatchComposition(&input.Composition{Text: "がき", Cursor: 6})
	r.frame()
	r.key(input.KeyEnter, 0)
	r.key(input.KeyDown, 0)
	r.key(input.KeyEscape, 0)
	if len(r.ran) != 0 {
		t.Fatalf("Enter during the composition ran %v", r.ran)
	}
	r.settle()
	if len(r.rows()) == 0 {
		t.Fatal("Escape during the composition closed the palette")
	}
	// The composition commits, and then Enter runs.
	r.tree.DispatchComposition(nil)
	r.typeText("書")
	if got := r.rows(); !slices.Equal(got, []string{"縦書き"}) {
		t.Fatalf("after the composition: %v", got)
	}
	r.key(input.KeyEnter, 0)
	if !slices.Equal(r.ran, []string{"縦書き"}) {
		t.Errorf("Enter ran %v", r.ran)
	}
}

func TestCommandPalette_FollowsActionsWhileOpen(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.open()
	r.typeText("保存")
	if got := r.selected(); got != "上書き保存" {
		t.Fatalf("selected %q", got)
	}
	// The window disables 上書き保存 and adds a command while the palette is
	// open.
	p := r.palette
	acts := r.demoActions()
	acts[2].OnInvoke = nil
	acts = append(acts, r.act("すべて保存"))
	p.Actions = acts
	r.set(p)
	if got := r.rows(); !slices.Equal(got, []string{"上書き保存", "すべて保存", "名前を付けて保存"}) &&
		!slices.Equal(got, []string{"上書き保存", "名前を付けて保存", "すべて保存"}) {
		t.Fatalf("after the change: %v", got)
	}
	if n := r.row("上書き保存"); n.State&semantics.Disabled == 0 {
		t.Error("the disabled action is not marked")
	}
	if got := r.selected(); got == "上書き保存" || got == "" {
		t.Errorf("the selection stays on a disabled action: %q", got)
	}
	r.key(input.KeyEnter, 0)
	if len(r.ran) != 1 || r.ran[0] == "上書き保存" {
		t.Errorf("Enter ran %v", r.ran)
	}
	// Removing every action leaves the palette empty, not broken.
	r.settle()
	r.open()
	p.Actions = nil
	r.set(p)
	if len(r.rows()) != 0 || r.find("No results") == nil {
		t.Errorf("without actions: %v", r.rows())
	}
}

func TestCommandPalette_ClickRuns(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.open()
	wtest.Tap(r.tree, wtest.Center(r.row("Toggle Sidebar").Rect))
	r.frame()
	if !slices.Equal(r.ran, []string{"Toggle Sidebar"}) {
		t.Errorf("a click ran %v", r.ran)
	}
}

func TestCommandPalette_RecentFirst(t *testing.T) {
	r := newPaletteRig(t, func(_ *paletteRig, p *CommandPalette) { p.RecentFirst = true })
	r.open()
	r.typeText("縦")
	r.key(input.KeyEnter, 0)
	r.settle()
	r.open()
	r.typeText("toggle")
	r.key(input.KeyEnter, 0)
	r.settle()
	r.open()
	got := r.rows()
	if len(got) < 3 || got[0] != "Toggle Sidebar" || got[1] != "縦書き" || got[2] != "新規作成" {
		t.Errorf("with RecentFirst the palette lists %v", got)
	}
	// Without it the order stays that of the actions.
	r2 := newPaletteRig(t, nil)
	r2.open()
	r2.typeText("縦")
	r2.key(input.KeyEnter, 0)
	r2.settle()
	r2.open()
	if got := r2.rows(); got[0] != "新規作成" {
		t.Errorf("without RecentFirst: %v", got)
	}
}

func TestCommandPalette_ExtraItems(t *testing.T) {
	var opened []string
	r := newPaletteRig(t, func(_ *paletteRig, p *CommandPalette) {
		p.Extra = []PaletteItem{
			{Label: "京都の旅.txt", Detail: "最近のファイル", OnInvoke: func() { opened = append(opened, "京都") }},
			{Label: "俳句", Detail: "ノート", OnInvoke: func() { opened = append(opened, "俳句") }},
			{Label: "消えた.txt", Detail: "最近のファイル"},
		}
	})
	r.open()
	if got := r.rows(); len(got) != paletteRows || r.find("1 of 10") == nil {
		t.Fatalf("the palette lists %v", got)
	}
	r.typeText("俳句")
	r.key(input.KeyEnter, 0)
	if !slices.Equal(opened, []string{"俳句"}) {
		t.Errorf("Enter opened %v", opened)
	}
	r.settle()
	// The detail matches too, after the labels.
	r.open()
	r.typeText("最近")
	if got := r.rows(); !slices.Equal(got, []string{"京都の旅.txt", "消えた.txt"}) {
		t.Errorf("最近 finds %v", got)
	}
}

func TestCommandPalette_ShortcutAndAction(t *testing.T) {
	r := newPaletteRig(t, func(_ *paletteRig, p *CommandPalette) {
		p.Shortcut = interact.Shortcut{Key: input.KeyK, Modifiers: input.ModControl}
	})
	r.key(input.KeyP, input.ModControl|input.ModShift)
	r.settle()
	if len(r.rows()) != 0 {
		t.Fatal("the default shortcut opened a palette that has its own")
	}
	a := CommandPaletteAction(r.ctx)
	if a.Label != "Command palette" || a.Shortcut.String() != "Ctrl+K" || !a.Enabled() {
		t.Errorf("the action is %+v", a)
	}
	r.key(input.KeyK, input.ModControl)
	r.settle()
	if len(r.rows()) == 0 {
		t.Fatal("Ctrl+K did not open the palette")
	}
	r.key(input.KeyEscape, 0)
	r.settle()
	OpenCommandPalette(r.ctx)
	r.frame()
	r.settle()
	if len(r.rows()) == 0 {
		t.Fatal("OpenCommandPalette did not open it")
	}
	// Opening an open palette does nothing.
	OpenCommandPalette(r.ctx)
	r.frame()
	r.settle()
	root, _ := r.tree.SemanticsTree()
	if n := len(wtest.RolesOf(root, semantics.RoleDialog)); n != 1 {
		t.Errorf("%d dialogs", n)
	}
	r2 := newPaletteRig(t, nil)
	if got := CommandPaletteAction(r2.ctx).Shortcut.String(); got != "Ctrl+Shift+P" {
		t.Errorf("the default shortcut is %q", got)
	}
}

func TestCommandPalette_NeedsAPalette(t *testing.T) {
	var ctx core.BuildContext
	tree := wtest.TextTree(t, wtest.Capture{Got: &ctx, Child: basic.SizedBox{}})
	wtest.Ops(tree, wtest.Screen)
	wtest.ExpectPanic(t, "found no CommandPalette", func() { OpenCommandPalette(ctx) })
	wtest.ExpectPanic(t, "found no CommandPalette", func() { CommandPaletteAction(ctx) })
}

func TestCommandPalette_ZeroValue(t *testing.T) {
	r := newPaletteRig(t, func(_ *paletteRig, p *CommandPalette) { *p = CommandPalette{} })
	r.open()
	if r.find("No results") == nil || len(r.rows()) != 0 {
		t.Errorf("an empty palette lists %v", r.rows())
	}
	r.key(input.KeyEnter, 0)
	r.key(input.KeyDown, 0)
	if len(r.ran) != 0 {
		t.Errorf("ran %v", r.ran)
	}
	// A zero palette without a child builds as well.
	tree := wtest.TextTree(t, overlay.Overlay{Child: CommandPalette{}})
	wtest.Ops(tree, wtest.Screen)
}

func TestCommandPalette_ManyResultsScroll(t *testing.T) {
	r := newPaletteRig(t, func(r *paletteRig, p *CommandPalette) {
		p.Actions = nil
		for i := range 20 {
			p.Actions = append(p.Actions, r.act("コマンド"+strings.Repeat("あ", i)))
		}
	})
	r.open()
	if got := len(r.rows()); got != paletteRows {
		t.Fatalf("%d rows show, want %d", got, paletteRows)
	}
	if r.find("1 of 20") == nil {
		t.Error("the count does not show")
	}
	for range 12 {
		r.key(input.KeyDown, 0)
	}
	if got := r.selected(); got != "コマンド"+strings.Repeat("あ", 12) {
		t.Errorf("selected %q", got)
	}
	if r.find("13 of 20") == nil {
		t.Error("the count does not follow")
	}
	r.key(input.KeyEnter, 0)
	if len(r.ran) != 1 || r.ran[0] != "コマンド"+strings.Repeat("あ", 12) {
		t.Errorf("ran %v", r.ran)
	}
}

func TestCommandPalette_DisposedWhileOpen(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.open()
	r.swap(overlay.Overlay{Child: basic.SizedBox{}})
	r.frame()
	r.settle()
	root, _ := r.tree.SemanticsTree()
	if n := len(wtest.RolesOf(root, semantics.RoleDialog)); n != 0 {
		t.Errorf("the palette outlives its widget: %d dialogs", n)
	}
}

func TestPaletteLabel(t *testing.T) {
	cases := map[string]string{
		"保存(&S)":        "保存",
		"開く(&O)…":       "開く",
		"&Save":         "Save",
		"Save &As...":   "Save As",
		"Tom && Jerry":  "Tom & Jerry",
		"コマンドパレット(&P)…": "コマンドパレット",
		"":              "",
	}
	for in, want := range cases {
		if got := paletteLabel(in); got != want {
			t.Errorf("paletteLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFuzzySpans_MarksTheRanges(t *testing.T) {
	base := text.Style{Weight: 400}
	mark := geom.Hex(0x2f5fd0)
	spans := FuzzySpans(nil, "名前を付けて保存", []int{0, 3, 18, 24}, base, FuzzyHighlight(base, mark))
	var texts []string
	for _, s := range spans {
		texts = append(texts, s.Text)
	}
	if !slices.Equal(texts, []string{"名", "前を付けて", "保存"}) {
		t.Fatalf("spans %v", texts)
	}
	if spans[0].Style.Color != mark || spans[0].Style.Weight != 600 || spans[1].Style.Color == mark {
		t.Errorf("the matched spans are not marked: %+v", spans)
	}
	if got := FuzzySpans(nil, "abc", nil, base, FuzzyHighlight(base, mark)); len(got) != 1 || got[0].Text != "abc" {
		t.Errorf("no ranges: %+v", got)
	}
}
