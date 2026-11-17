package field

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
)

// The text of the ruby tests: 前 (3 bytes), then a mark from 3 to 27 whose
// base 漢字 runs from 6 to 12, then 後.
const rubyValue = "前｜漢字《かんじ》後"

const (
	markStart = 3
	markBase  = 6
	markEnd   = 27
)

func rubyField(t testing.TB, f Input) *fieldHarness {
	t.Helper()
	f.Ruby = aozora.Marks
	return newField(t, f)
}

// shownText returns the text the field lays out.
func (h *fieldHarness) shownText() string { return h.field().shaped.Text() }

func (h *fieldHarness) expectShown(want string) {
	h.t.Helper()
	if got := h.shownText(); got != want {
		h.t.Errorf("shown %q, want %q", got, want)
	}
}

func TestTextField_RubyHidesTheNotation(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue})
	h.expectShown("前漢字後")
	h.expect(rubyValue)
	// The base carries the ruby: one more run than the plain text has.
	plain := newField(t, Input{Value: "前漢字後", Style: text.Style{RubyRoom: true}})
	if got, want := len(h.field().laidOut.Runs()), len(plain.field().laidOut.Runs())+1; got != want {
		t.Errorf("%d runs, want %d", got, want)
	}
}

func TestRubyView_MapsOffsets(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue + rubyValue})
	v := &h.field().ruby
	// Shown: 前漢字後前漢字後. The second mark starts at 30+3 in the text.
	for _, tc := range []struct{ display, shown int }{
		{0, 0}, {3, 3}, {6, 3}, {9, 6}, {12, 9}, {20, 9}, {27, 9}, {30, 12},
		{33, 15}, {36, 15}, {42, 21}, {57, 21}, {60, 24},
	} {
		if got := v.toShown(tc.display); got != tc.shown {
			t.Errorf("toShown(%d) = %d, want %d", tc.display, got, tc.shown)
		}
	}
	// A shown offset at hidden notation stands for the outside of the mark.
	for _, tc := range []struct{ shown, display int }{
		{0, 0}, {3, markStart}, {6, 9}, {9, markEnd}, {12, 30}, {15, 33}, {21, 57}, {24, 60},
	} {
		if got := v.fromShown(tc.shown); got != tc.display {
			t.Errorf("fromShown(%d) = %d, want %d", tc.shown, got, tc.display)
		}
	}
}

func TestRubyView_NeighboringMarks(t *testing.T) {
	h := rubyField(t, Input{Value: "漢《かん》字《じ》"})
	h.expectShown("漢字")
	v := &h.field().ruby
	// Between the two bases is the end of the first and the start of the
	// second, which are one offset.
	if got := v.fromShown(3); got != len("漢《かん》") {
		t.Errorf("got %d", got)
	}
	h.Key(input.KeyRight, 0) // into the first
	h.expectSelection(3, 3)
	h.expectShown("漢《かん》字")
}

func TestTextField_RubyArrowsStepIntoAMark(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue})
	h.Key(input.KeyRight, 0)
	h.expectSelection(markStart, markStart)
	h.expectShown("前漢字後")
	// The next step goes into the notation, which shows.
	h.Key(input.KeyRight, 0)
	h.expectSelection(markBase, markBase)
	h.expectShown(rubyValue)
	// Out at the other end, the ruby comes back.
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyLeft, 0)
	h.expectSelection(markEnd, markEnd)
	h.expectShown("前漢字後")
	h.Key(input.KeyLeft, 0)
	h.expectSelection(markEnd-len("》"), markEnd-len("》"))
	h.expectShown(rubyValue)
	h.Key(input.KeyRight, 0)
	h.expectShown("前漢字後")
	// Shift steps over the mark as a whole, and copy takes the notation.
	h.Key(input.KeyHome, 0)
	h.Key(input.KeyRight, 0)
	h.Key(input.KeyRight, input.ModShift)
	h.expectSelection(markStart, markEnd)
	h.expectShown("前漢字後")
	h.Key(input.KeyC, input.ModControl)
	if h.host.Clipboard != "｜漢字《かんじ》" {
		t.Errorf("copied %q", h.host.Clipboard)
	}
}

func TestTextField_RubyWordKeysSkipTheMark(t *testing.T) {
	h := rubyField(t, Input{Value: "ab 漢字《かんじ》 cd"})
	h.Key(input.KeyRight, input.ModControl)
	h.expectSelection(3, 3)
	h.Key(input.KeyRight, input.ModControl)
	end := len("ab 漢字《かんじ》 ")
	h.expectSelection(end, end)
	h.expectShown("ab 漢字 cd")
}

func TestTextField_RubyClickLeavesTheMarkHidden(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue})
	f := h.field()
	origin := f.textOrigin()
	box := core.LocalRectToGlobal(f, geom.Rect{W: f.Size().W, H: f.Size().H})
	// The middle of the base, as the layout of the shown text puts it.
	l := f.laidOut
	x := (l.CaretPosition(3) + l.CaretPosition(9)) / 2
	pos := geom.Offset{X: box.X + origin.X + x + 1, Y: box.Y + f.Size().H/2}
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: input.ButtonPrimary, Clicks: 1})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: input.ButtonPrimary})
	h.frame()
	if a, b := f.Selection(); a != b || (a != markStart && a != markEnd) {
		t.Errorf("selection %d-%d, want an end of the mark", a, b)
	}
	h.expectShown("前漢字後")
}

func TestTextField_RubyBackspaceOpensTheMark(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue})
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyLeft, 0)
	h.expectSelection(markEnd, markEnd)
	h.Key(input.KeyBackspace, 0)
	h.expect("前｜漢字《かんじ後")
	h.expectShown("前｜漢字《かんじ後")
	h.Key(input.KeyZ, input.ModControl)
	h.expect(rubyValue)
	h.expectShown("前漢字後")
	// Delete before the mark takes its ｜.
	h.Key(input.KeyHome, 0)
	h.Key(input.KeyRight, 0)
	h.Key(input.KeyDelete, 0)
	h.expect("前漢字《かんじ》後")
	// Without the bar, the base is all the kanji, 前 too, and the caret is
	// inside it.
	h.expectShown("前漢字《かんじ》後")
	h.Key(input.KeyHome, 0)
	h.expectShown("前漢字後")
}

func TestTextField_RubyTypingAMark(t *testing.T) {
	h := rubyField(t, Input{})
	h.typeRunes("｜漢字《かんじ")
	h.expectShown("｜漢字《かんじ")
	h.typeText("》")
	h.expect("｜漢字《かんじ》")
	h.expectShown("漢字")
	h.typeText("を")
	h.expectShown("漢字を")
	// Undo takes the typing back, notation and all.
	h.Key(input.KeyZ, input.ModControl)
	h.expect("")
	h.expectShown("")
}

func TestTextField_RubyStaysWhileTypingElsewhere(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue, Multiline: true})
	h.Key(input.KeyEnd, input.ModControl)
	h.Key(input.KeyEnter, 0)
	for _, c := range "ここで打つ" {
		h.typeText(string(c))
		if got := h.shownText(); !strings.HasPrefix(got, "前漢字後\n") {
			t.Fatalf("shown %q", got)
		}
	}
	if a := h.field().ruby.active; a != [2]int{-1, -1} {
		t.Errorf("active %v", a)
	}
}

func TestTextField_RubyUpAndDownLandOutsideMarks(t *testing.T) {
	h := rubyField(t, Input{Value: "あいうえお\n" + rubyValue + "\nかきくけこ", Multiline: true})
	for range 2 {
		h.Key(input.KeyRight, 0)
	}
	h.Key(input.KeyDown, 0)
	a, _ := h.field().Selection()
	line := len("あいうえお\n")
	if a != line+markStart && a != line+markEnd {
		t.Errorf("caret %d, want an end of the mark", a)
	}
	h.expectShown("あいうえお\n前漢字後\nかきくけこ")
	h.Key(input.KeyDown, 0)
	h.expectShown("あいうえお\n前漢字後\nかきくけこ")
}

func TestTextField_RubyComposition(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue})
	f := h.field()
	// After the mark, the composition sits after the hidden notation.
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyLeft, 0)
	h.tree.DispatchComposition(&input.Composition{Text: "かな", Cursor: 6})
	h.frame()
	h.expectShown("前漢字かな後")
	want := f.laidOut.CaretRect(len("前漢字かな")).Translate(f.textOrigin())
	if got := f.shown().CaretRect(f.displayCaret()).Translate(f.textOrigin()); got != want {
		t.Errorf("caret %v, want %v", got, want)
	}
	h.tree.DispatchComposition(nil)
	h.typeText("仮名")
	h.expect("前｜漢字《かんじ》仮名後")
	h.expectShown("前漢字仮名後")
	// Inside the ruby, the mark shows while the IME converts.
	h.Key(input.KeyHome, 0)
	h.Key(input.KeyRight, 0)
	h.Key(input.KeyRight, 0)
	h.tree.DispatchComposition(&input.Composition{Text: "大", Cursor: 3})
	h.frame()
	h.expectShown("前｜大漢字《かんじ》仮名後")
	// Over a selection that holds the mark, the mark goes with it.
	h.tree.DispatchComposition(nil)
	h.frame()
	h.Key(input.KeyHome, 0)
	h.Key(input.KeyEnd, input.ModShift)
	h.tree.DispatchComposition(&input.Composition{Text: "x", Cursor: 1})
	h.frame()
	h.expectShown("x")
}

func TestTextField_RubyVertical(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue, Multiline: true, Style: text.Style{Size: 10, Orientation: text.Vertical}})
	h.expectShown("前漢字後")
	// The ruby stands right of its base.
	runs := h.field().laidOut.Runs()
	var base, ruby float32
	for _, r := range runs {
		if r.Size == 10 {
			base = r.Origin.X
		} else {
			ruby = r.Origin.X
		}
	}
	if ruby <= base {
		t.Errorf("ruby at x %v, base at %v", ruby, base)
	}
	// Down goes along the column, and into the mark.
	h.Key(input.KeyDown, 0)
	h.Key(input.KeyDown, 0)
	h.expectSelection(markBase, markBase)
	h.expectShown(rubyValue)
}

func TestTextField_RubySemanticsGetTheValue(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue + "\nb", Multiline: true})
	h.Key(input.KeyEnd, input.ModControl)
	n := h.tree.Semantics().Children[0]
	if n.Value != rubyValue+"\nb" || n.Text.Text != n.Value {
		t.Errorf("value %q", n.Value)
	}
	if got := n.Text.LineStarts; len(got) != 1 || got[0] != kit.Utf16Len(rubyValue+"\n") {
		t.Errorf("line starts %v", got)
	}
	if n.Text.SelectionEnd != kit.Utf16Len(rubyValue+"\nb") {
		t.Errorf("selection %d", n.Text.SelectionEnd)
	}
	// A selection set into the ruby shows the mark.
	f := h.field()
	f.PerformSemanticsAction(semantics.ActionSetSelection, core.SemanticsArg{SelectionStart: 3, SelectionEnd: 3})
	h.frame()
	h.expectShown(rubyValue + "\nb")
}

func TestTextField_RubyValueFromOutside(t *testing.T) {
	h := rubyField(t, Input{Value: "plain"})
	f := h.field()
	f.setText(rubyValue)
	f.invalidate()
	h.frame()
	h.expectShown("前漢字後")
}

func TestTextField_RubyPaintDoesNotAllocate(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue + rubyValue})
	h.Key(input.KeyA, input.ModControl)
	var cv paint.Canvas
	cv.Reset()
	h.tree.Paint(&cv)
	allocs := testing.AllocsPerRun(10, func() {
		cv.Reset()
		h.tree.Paint(&cv)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func TestTextField_RubyBadMarksPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("marks out of order must panic")
		}
	}()
	newField(t, Input{Value: "abc", Ruby: func(string, []text.RubyMark) []text.RubyMark {
		return []text.RubyMark{{Start: 0, End: 9, BaseStart: 0, BaseEnd: 1, RubyStart: 1, RubyEnd: 2}}
	}})
}

// rubyText is about 100,000 characters with a mark every hundred or so.
func rubyText() string {
	line := "親譲りの無鉄砲《むてっぽう》で小供の時から損ばかりしている。小学校に居る時分学校の二階から飛び降りて一週間ほど腰を抜かした事がある。\n"
	return strings.Repeat(line, 100000/len([]rune(line))+1)
}

// BenchmarkTextField_Keystroke100kRuby is BenchmarkTextField_Keystroke100k
// with Ruby: a keystroke and its frame in 100,000 characters with about 1,000
// marks. "notation" is the same text in a field without Ruby.
func BenchmarkTextField_Keystroke100kRuby(b *testing.B) {
	b.Run("ruby", func(b *testing.B) { benchmarkKeystroke(b, aozora.Marks) })
	b.Run("notation", func(b *testing.B) { benchmarkKeystroke(b, nil) })
}

func benchmarkKeystroke(b *testing.B, ruby func(string, []text.RubyMark) []text.RubyMark) {
	h := newField(b, Input{Value: rubyText(), Multiline: true, Ruby: ruby})
	h.Key(input.KeyHome, input.ModControl)
	for range 20 {
		h.Key(input.KeyDown, 0)
	}
	var cv paint.Canvas
	frame := func() {
		h.tree.Build()
		h.tree.Layout(geom.Tight(h.size))
		cv.Reset()
		h.tree.Paint(&cv)
	}
	b.ReportMetric(float64(len(h.field().ruby.marks)), "marks")
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		h.tree.DispatchText(string(rune('a' + i%26)))
		frame()
	}
}

// BenchmarkTextField_RubyRevealMark100k moves the caret into a mark and out
// again in 100,000 characters: each move shows or hides one mark.
func BenchmarkTextField_RubyRevealMark100k(b *testing.B) {
	h := rubyField(b, Input{Value: rubyText(), Multiline: true})
	h.Key(input.KeyHome, input.ModControl)
	for range 20 {
		h.Key(input.KeyDown, 0)
	}
	h.Key(input.KeyHome, 0)
	for range 4 {
		h.Key(input.KeyRight, 0)
	}
	var cv paint.Canvas
	frame := func() {
		h.tree.Build()
		h.tree.Layout(geom.Tight(h.size))
		cv.Reset()
		h.tree.Paint(&cv)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		k := input.KeyRight
		if i%2 == 1 {
			k = input.KeyLeft
		}
		h.tree.DispatchKey(core.KeyEvent{Key: k})
		frame()
	}
}

func TestTextField_RubyMarksFollowEdits(t *testing.T) {
	h := rubyField(t, Input{Value: rubyValue + "\n漢《かん》と字《じ》\nend", Multiline: true})
	f := h.field()
	rng := rand.New(rand.NewPCG(5, 6))
	pieces := []string{"漢", "《", "》", "｜", "\n", "か", "", "字《じ》"}
	for step := range 300 {
		n := len(f.Text())
		a := rng.IntN(n + 1)
		b := min(a+rng.IntN(4), n)
		for a > 0 && a < n && !utf8.RuneStart(f.Text()[a]) {
			a--
		}
		for b < n && !utf8.RuneStart(f.Text()[b]) {
			b++
		}
		f.replace(a, b, pieces[rng.IntN(len(pieces))], kit.EditOther)
		h.frame()
		want := aozora.Marks(f.Text(), nil)
		if !slices.Equal(f.ruby.marks, want) {
			t.Fatalf("step %d, %q: marks %v, want %v", step, f.Text(), f.ruby.marks, want)
		}
		if got, want := h.shownText(), shownOf(f.Text(), want, f.ruby.active); got != want {
			t.Fatalf("step %d: shown %q, want %q", step, got, want)
		}
	}
}

// shownOf returns what a field shows of s with marks, but for the active ones.
func shownOf(s string, marks []text.RubyMark, active [2]int) string {
	var b strings.Builder
	at := 0
	for i, m := range marks {
		if i == active[0] || i == active[1] {
			continue
		}
		b.WriteString(s[at:m.Start])
		b.WriteString(s[m.BaseStart:m.BaseEnd])
		at = m.End
	}
	b.WriteString(s[at:])
	return b.String()
}

// A note of emphasis hides as ruby does, and its text gets the marks.
func TestTextField_RubyEmphasisNote(t *testing.T) {
	const value = "前対象［＃「対象」に傍点］後"
	h := rubyField(t, Input{Value: value})
	h.expectShown("前対象後")
	// The marks are one more run than the plain text has.
	plain := newField(t, Input{Value: "前対象後", Style: text.Style{RubyRoom: true}})
	if got, want := len(h.field().laidOut.Runs()), len(plain.field().laidOut.Runs())+1; got != want {
		t.Errorf("%d runs, want %d", got, want)
	}
	// Into the note from its end, it shows as written.
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyLeft, 0)
	h.expectShown("前対象後")
	h.Key(input.KeyLeft, 0)
	h.expectShown(value)
}
