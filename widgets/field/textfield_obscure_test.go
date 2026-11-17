package field

import (
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// secureFakeHost is a FakeHost that also records secure input, in order with
// the IME, as a window that has it would get them.
type secureFakeHost struct {
	wtest.FakeHost
	secure []bool
	calls  []string
}

func (h *secureFakeHost) SetIMEEnabled(enabled bool) {
	h.FakeHost.SetIMEEnabled(enabled)
	h.calls = append(h.calls, map[bool]string{true: "ime on", false: "ime off"}[enabled])
}

func (h *secureFakeHost) SetSecureInput(secure bool) {
	h.secure = append(h.secure, secure)
	h.calls = append(h.calls, map[bool]string{true: "secure on", false: "secure off"}[secure])
}

var _ core.SecureInputHost = (*secureFakeHost)(nil)

// obscureRig drives fields in a column through a real tree.
type obscureRig struct {
	t    testing.TB
	tree *core.Tree
	host *secureFakeHost
	size geom.Size
}

func newObscureRig(t testing.TB, w core.Widget) *obscureRig {
	t.Helper()
	r := &obscureRig{t: t, host: &secureFakeHost{}, size: geom.Size{W: 400, H: 300}}
	r.tree = wtest.TextTree(t, basic.Align{Alignment: layout.AlignTopLeft, Child: w})
	r.tree.SetHost(r.host)
	r.frame()
	return r
}

// newObscured is a rig of one field that already has the keyboard.
func newObscured(t testing.TB, f Input) *obscureRig {
	t.Helper()
	r := newObscureRig(t, f)
	r.key(input.KeyTab, 0)
	return r
}

func (r *obscureRig) frame() string { return testutil.RenderTree(r.tree, r.size) }

func (r *obscureRig) key(k input.KeyCode, mods input.Modifiers) {
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	r.frame()
}

func (r *obscureRig) typeText(s string) {
	r.tree.DispatchText(s)
	r.frame()
}

func (r *obscureRig) field() *RenderEditable {
	r.t.Helper()
	e, ok := r.tree.Focused().(*RenderEditable)
	if !ok {
		r.t.Fatalf("the focus is on %T, not a text field", r.tree.Focused())
	}
	return e
}

func (r *obscureRig) expect(want string) {
	r.t.Helper()
	if got := r.field().Text(); got != want {
		r.t.Errorf("text %q, want %q", got, want)
	}
}

func (r *obscureRig) expectSelection(start, end int) {
	r.t.Helper()
	if a, b := r.field().Selection(); a != start || b != end {
		r.t.Errorf("selection %d-%d, want %d-%d", a, b, start, end)
	}
}

// node returns the semantics of the text field labeled label.
func (r *obscureRig) node(label string) *semantics.Node {
	r.t.Helper()
	root, _ := r.tree.SemanticsTree()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleTextField && (label == "" || n.Label == label) {
			found = n
		}
		return found == nil
	})
	if found == nil {
		r.t.Fatalf("no text field %q", label)
	}
	return found
}

func bullets(n int) string { return strings.Repeat(obscureBullet, n) }

func TestInputObscure_DrawsBulletsOnly(t *testing.T) {
	// Two passwords of as many characters draw the same frame: nothing of
	// what was typed reaches the canvas.
	frames := make([]string, 0, 2)
	for _, s := range []string{"hunter2", "パスワード12"} {
		r := newObscured(t, Input{Obscure: true})
		r.typeText(s)
		r.expect(s)
		frames = append(frames, r.frame())
		if got := r.field().laidOut.LineCount(); got != 1 {
			t.Errorf("%q: %d lines", s, got)
		}
	}
	if frames[0] != frames[1] {
		t.Errorf("the frames differ by content:\n%s\n---\n%s", frames[0], frames[1])
	}
	// The same text without Obscure draws something else.
	r := newObscured(t, Input{})
	r.typeText("hunter2")
	if r.frame() == frames[0] {
		t.Error("a field without Obscure must draw its text")
	}
}

func TestInputObscure_ClustersAreOneBullet(t *testing.T) {
	// e + combining acute, a ZWJ family and a flag are one character each.
	const s = "aé👨‍👩🇯🇵b"
	r := newObscured(t, Input{Obscure: true})
	r.typeText(s)
	f := r.field()
	if got := f.obscure.bounds; !slices.Equal(got, []int{0, 1, 4, 15, 23, 24}) {
		t.Fatalf("bounds %v", got)
	}
	n := r.node("")
	if n.Value != bullets(5) || n.Text == nil || n.Text.Text != bullets(5) {
		t.Errorf("semantics got %q / %+v, want 5 bullets", n.Value, n.Text)
	}
	// The caret steps over a cluster at a time, on the real text.
	steps := []int{23, 15, 4, 1, 0, 0}
	for _, want := range steps {
		r.key(input.KeyLeft, 0)
		r.expectSelection(want, want)
	}
	for _, want := range []int{1, 4, 15, 23, 24, 24} {
		r.key(input.KeyRight, 0)
		r.expectSelection(want, want)
	}
	// Shift extends by clusters too.
	r.key(input.KeyLeft, input.ModShift)
	r.key(input.KeyLeft, input.ModShift)
	r.expectSelection(15, 24)
	// Backspace takes the whole cluster before the caret.
	r.key(input.KeyRight, 0)
	r.key(input.KeyLeft, 0)
	r.key(input.KeyBackspace, 0)
	r.expect("aé👨‍👩b")
	r.key(input.KeyHome, 0)
	r.key(input.KeyRight, 0)
	r.key(input.KeyDelete, 0)
	r.expect("a👨‍👩b")
}

func TestInputObscure_WordMovesGoToTheEnds(t *testing.T) {
	r := newObscured(t, Input{Obscure: true, Value: "two words here"})
	r.key(input.KeyEnd, 0)
	r.key(input.KeyLeft, input.ModControl)
	r.expectSelection(0, 0)
	r.key(input.KeyRight, input.ModControl)
	r.expectSelection(14, 14)
	r.key(input.KeyLeft, 0)
	r.key(input.KeyLeft, 0)
	r.key(input.KeyBackspace, input.ModControl)
	r.expect("re")
	r.key(input.KeyDelete, input.ModControl)
	r.expect("")
}

func TestInputObscure_Pointer(t *testing.T) {
	r := newObscured(t, Input{Obscure: true, Value: "secret"})
	n := r.node("")
	left := geom.Offset{X: n.Rect.X + 2, Y: n.Rect.Y + n.Rect.H/2}
	right := geom.Offset{X: n.Rect.X + n.Rect.W - 2, Y: left.Y}
	wtest.Tap(r.tree, right)
	r.frame()
	r.expectSelection(6, 6)
	wtest.Tap(r.tree, left)
	r.frame()
	r.expectSelection(0, 0)
	// A press in the middle of the bullets lands between two characters.
	f := r.field()
	mid := f.shown().CaretRect(3)
	at := core.LocalRectToGlobal(f, mid.Translate(f.textOrigin()))
	wtest.Tap(r.tree, geom.Offset{X: at.X, Y: at.Y + at.H/2})
	r.frame()
	r.expectSelection(3, 3)
	// A double click takes the whole password, not a word of it.
	wtest.Tap(r.tree, right)
	wtest.Tap(r.tree, right)
	r.frame()
	r.expectSelection(0, 6)
}

func TestInputObscure_IMEAndSecureInput(t *testing.T) {
	r := newObscureRig(t, basic.Column{Size: layout.MainMin, Children: []core.Widget{
		Input{Label: "名前"},
		Input{Label: "パスワード", Obscure: true},
		Input{Label: "メモ"},
	}})
	r.key(input.KeyTab, 0)
	if r.node("名前") == nil || !slices.Equal(r.host.Ime, []bool{true}) || len(r.host.secure) != 0 {
		t.Fatalf("a plain field turns the IME on alone: ime %v secure %v", r.host.Ime, r.host.secure)
	}
	r.host.calls = nil
	r.key(input.KeyTab, 0)
	if want := []string{"ime off", "secure on", "ime off"}; !slices.Equal(r.host.calls, want) {
		t.Errorf("into the password: got %v, want %v", r.host.calls, want)
	}
	if r.node("パスワード").State&semantics.Focused == 0 {
		t.Fatal("the password field must have the keyboard")
	}
	r.host.calls = nil
	r.key(input.KeyTab, 0)
	if want := []string{"ime off", "secure off", "ime on"}; !slices.Equal(r.host.calls, want) {
		t.Errorf("out of the password: got %v, want %v", r.host.calls, want)
	}
	// Back with Shift+Tab.
	r.host.calls = nil
	r.key(input.KeyTab, input.ModShift)
	if want := []string{"ime off", "secure on", "ime off"}; !slices.Equal(r.host.calls, want) {
		t.Errorf("back into the password: got %v, want %v", r.host.calls, want)
	}
}

func TestInputObscure_HostWithoutSecureInput(t *testing.T) {
	// A host that cannot do secure input still gets the IME turned off.
	host := &wtest.FakeHost{}
	tree := wtest.TextTree(t, basic.Align{Child: Input{Obscure: true}})
	tree.SetHost(host)
	size := geom.Size{W: 400, H: 300}
	testutil.RenderTree(tree, size)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	testutil.RenderTree(tree, size)
	if !slices.Equal(host.Ime, []bool{false}) {
		t.Errorf("got %v", host.Ime)
	}
}

func TestInputObscure_RemovedWithTheKeyboard(t *testing.T) {
	var set func(bool)
	r := newObscureRig(t, formHolder[bool]{Initial: true, Build: func(shown bool, s func(bool)) core.Widget {
		set = s
		if !shown {
			return basic.SizedBox{}
		}
		return Input{Obscure: true}
	}})
	r.key(input.KeyTab, 0)
	set(false)
	r.frame()
	if !slices.Equal(r.host.secure, []bool{true, false}) {
		t.Errorf("secure input must end with the field: %v", r.host.secure)
	}
}

func TestInputObscure_Clipboard(t *testing.T) {
	r := newObscured(t, Input{Obscure: true, Value: "secret"})
	r.host.Clipboard = "before"
	r.key(input.KeyA, input.ModControl)
	r.key(input.KeyC, input.ModControl)
	if r.host.Clipboard != "before" {
		t.Errorf("copy must do nothing, clipboard %q", r.host.Clipboard)
	}
	r.key(input.KeyX, input.ModControl)
	if r.host.Clipboard != "before" {
		t.Errorf("cut must do nothing, clipboard %q", r.host.Clipboard)
	}
	r.expect("secret")
	r.expectSelection(0, 6)
	// Paste works, and a line break in it becomes a space.
	r.host.Clipboard = "pa\nste"
	r.key(input.KeyV, input.ModControl)
	r.expect("pa ste")
}

func TestInputObscure_ControllerClipboard(t *testing.T) {
	var c InputController
	r := newObscured(t, Input{Obscure: true, Value: "secret", Controller: &c})
	r.host.Clipboard = "before"
	c.SelectAll()
	c.Copy()
	c.Cut()
	if r.host.Clipboard != "before" {
		t.Errorf("the edit menu must not copy either, clipboard %q", r.host.Clipboard)
	}
	r.expect("secret")
	r.host.Clipboard = "new"
	c.Paste()
	r.frame()
	r.expect("new")
	// What the application reads stays the text.
	if c.Text() != "new" {
		t.Errorf("Text %q", c.Text())
	}
}

func TestInputObscure_NoUndoHistory(t *testing.T) {
	var changes []string
	r := newObscured(t, Input{Obscure: true, OnChange: func(s string) { changes = append(changes, s) }})
	r.typeText("old")
	r.key(input.KeyA, input.ModControl)
	r.key(input.KeyBackspace, 0)
	r.typeText("new")
	before := len(changes)
	r.key(input.KeyZ, input.ModControl)
	r.key(input.KeyY, input.ModControl)
	r.key(input.KeyZ, input.ModControl|input.ModShift)
	r.expect("new")
	if len(changes) != before {
		t.Errorf("undo must change nothing: %v", changes[before:])
	}
	if _, ok := r.field().history.TakeUndo(); ok {
		t.Error("the history must hold nothing")
	}
}

func TestInputObscure_Semantics(t *testing.T) {
	r := newObscured(t, Input{Obscure: true, Label: "パスワード", Value: "あいう"})
	r.key(input.KeyEnd, 0)
	r.key(input.KeyLeft, input.ModShift)
	n := r.node("パスワード")
	if n.Value != bullets(3) || n.Value == "あいう" {
		t.Errorf("value %q, want bullets", n.Value)
	}
	if n.State&semantics.Obscured == 0 {
		t.Error("the node must be marked obscured")
	}
	if n.Text == nil || n.Text.Text != bullets(3) || n.Text.SelectionStart != 3 || n.Text.SelectionEnd != 2 {
		t.Errorf("text %+v", n.Text)
	}
	if n.Label != "パスワード" {
		t.Errorf("label %q", n.Label)
	}
	// A reader selects by bullets, and sets the value as typing would.
	if !r.tree.PerformSemanticsAction(n.ID, semantics.ActionSetSelection, core.SemanticsArg{SelectionStart: 1, SelectionEnd: 2}) {
		t.Fatal("set selection failed")
	}
	r.frame()
	r.expectSelection(3, 6)
	if !r.tree.PerformSemanticsAction(n.ID, semantics.ActionSetValue, core.SemanticsArg{Text: "pw"}) {
		t.Fatal("set value failed")
	}
	r.frame()
	r.expect("pw")
	if n := r.node("パスワード"); n.Value != bullets(2) {
		t.Errorf("value after set %q", n.Value)
	}
	// Without Obscure the reader gets the text, as before.
	plain := newObscured(t, Input{Label: "名前", Value: "あいう"})
	if n := plain.node("名前"); n.Value != "あいう" || n.State&semantics.Obscured != 0 {
		t.Errorf("plain field %+v", n)
	}
}

func TestInputObscure_ShowPasswordToggle(t *testing.T) {
	var set func(bool)
	r := newObscureRig(t, formHolder[bool]{Initial: true, Build: func(hidden bool, s func(bool)) core.Widget {
		set = s
		return Input{Label: "パスワード", Obscure: hidden}
	}})
	r.key(input.KeyTab, 0)
	r.typeText("abc")
	r.key(input.KeyLeft, 0)
	hiddenFrame := r.frame()
	r.host.calls = nil
	set(false) // show
	shownFrame := r.frame()
	if want := []string{"ime on", "secure off"}; !slices.Equal(r.host.calls, want) {
		t.Errorf("show: got %v, want %v", r.host.calls, want)
	}
	if shownFrame == hiddenFrame {
		t.Error("showing must draw the text")
	}
	if n := r.node("パスワード"); n.Value != "abc" || n.State&semantics.Obscured != 0 {
		t.Errorf("shown node %+v", n)
	}
	// The text, the caret and the focus stay.
	r.expect("abc")
	r.expectSelection(2, 2)
	// The history starts afresh in the shown field and works there.
	r.typeText("X")
	r.key(input.KeyZ, input.ModControl)
	r.expect("abc")
	r.host.calls = nil
	set(true) // hide again
	r.frame()
	if want := []string{"secure on", "ime off"}; !slices.Equal(r.host.calls, want) {
		t.Errorf("hide: got %v, want %v", r.host.calls, want)
	}
	if n := r.node("パスワード"); n.Value != bullets(3) {
		t.Errorf("hidden node %q", n.Value)
	}
	// What the shown field could undo is gone.
	r.key(input.KeyY, input.ModControl)
	r.expect("abc")
	if got := r.field().history; func() bool { _, ok := got.TakeRedo(); return ok }() {
		t.Error("the history must be cleared")
	}
}

func TestInputObscure_ToggleWhileUnfocusedTouchesNoHost(t *testing.T) {
	var set func(bool)
	r := newObscureRig(t, formHolder[bool]{Initial: true, Build: func(hidden bool, s func(bool)) core.Widget {
		set = s
		return Input{Obscure: hidden, Value: "abc"}
	}})
	set(false)
	r.frame()
	set(true)
	r.frame()
	if len(r.host.calls) != 0 {
		t.Errorf("a field without the keyboard must leave the IME alone: %v", r.host.calls)
	}
}

func TestInputObscure_IgnoresMultilineAndRuby(t *testing.T) {
	var submitted string
	r := newObscured(t, Input{Obscure: true, Multiline: true, OnSubmit: func(s string) { submitted = s }})
	r.typeText("pw")
	r.key(input.KeyEnter, 0)
	r.expect("pw")
	if submitted != "pw" {
		t.Errorf("Enter must submit an obscured field, got %q", submitted)
	}
	r.typeText("a\nb")
	r.expect("pwa b")

	// Ruby notation is a password like any other text: every character is a
	// bullet, and nothing is hidden or drawn as ruby.
	const s = "｜漢字《かんじ》"
	rb := newObscured(t, Input{Obscure: true, Ruby: aozora.Marks, Value: s})
	rb.key(input.KeyEnd, 0)
	if n := rb.node(""); n.Value != bullets(8) {
		t.Errorf("ruby: value %q, want 8 bullets", n.Value)
	}
	if got := len(rb.field().obscure.bounds) - 1; got != 8 {
		t.Errorf("ruby: %d bullets shown", got)
	}
	rb.key(input.KeyLeft, 0)
	rb.expectSelection(len(s)-len("》"), len(s)-len("》"))
}

func TestInputObscure_Vertical(t *testing.T) {
	r := newObscured(t, Input{Obscure: true, Style: text.Style{Orientation: text.Vertical}})
	r.typeText("あいう")
	f := r.field()
	if sz := f.Size(); sz.H <= sz.W {
		t.Errorf("a vertical field stands: %v", sz)
	}
	// Up is back along a vertical line.
	r.key(input.KeyUp, 0)
	r.expectSelection(6, 6)
	if n := r.node(""); n.Value != bullets(3) {
		t.Errorf("value %q", n.Value)
	}
}

func TestInputObscure_CompositionIsBulletsToo(t *testing.T) {
	// The IME is off, but a platform that composes anyway shows bullets.
	frames := make([]string, 0, 2)
	for _, s := range []string{"かな", "漢字"} {
		r := newObscured(t, Input{Obscure: true})
		r.tree.DispatchComposition(&input.Composition{Text: s, Cursor: len(s)})
		frames = append(frames, r.frame())
		r.expect("")
	}
	if frames[0] != frames[1] {
		t.Error("a composition must not show")
	}
}

func TestInputObscure_PaintDoesNotAllocate(t *testing.T) {
	r := newObscured(t, Input{Obscure: true, Value: "correct horse battery staple"})
	r.key(input.KeyA, input.ModControl)
	var cv paint.Canvas
	cv.Reset()
	r.tree.Paint(&cv)
	allocs := testing.AllocsPerRun(10, func() {
		cv.Reset()
		r.tree.Paint(&cv)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func BenchmarkInputObscure_Paint(b *testing.B) {
	r := newObscured(b, Input{Obscure: true, Value: "correct horse battery staple"})
	var cv paint.Canvas
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		r.tree.Paint(&cv)
	}
}
