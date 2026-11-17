package field_test

import (
	"slices"
	"testing"

	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
)

// chatBox is the box of a chat: Enter sends, Shift+Enter starts a new line.
type chatBox struct {
	core.StatefulBase
	multiline, submitOnEnter bool
	sent                     *[]string
}

type chatBoxState struct {
	core.StateBase
	draft string
}

func (chatBox) CreateState() core.State { return &chatBoxState{} }

func (s *chatBoxState) Build(core.BuildContext) core.Widget {
	b := s.Widget().(chatBox)
	return fieldw.Input{
		Label: "Message", Value: s.draft, Multiline: b.multiline, SubmitOnEnter: b.submitOnEnter, Autofocus: true,
		OnChange: func(v string) { s.SetState(func() { s.draft = v }) },
		OnSubmit: func(v string) {
			*b.sent = append(*b.sent, v)
			s.SetState(func() { s.draft = "" })
		},
	}
}

func TestInput_SubmitOnEnterSendsAndShiftEnterBreaksTheLine(t *testing.T) {
	var sent []string
	tt := gowakutest.New(t, chatBox{multiline: true, submitOnEnter: true, sent: &sent}, pressableFonts)
	tt.Type("one")
	tt.Key(input.KeyEnter, input.ModShift)
	tt.Type("two")
	if len(sent) != 0 {
		t.Fatalf("Shift+Enter sent %q", sent)
	}
	tt.Key(input.KeyEnter, 0)
	if want := []string{"one\ntwo"}; !slices.Equal(sent, want) {
		t.Errorf("Enter sent %q, want %q", sent, want)
	}
	if n, _ := tt.Find("Message"); n.Value != "" {
		t.Errorf("the field holds %q after sending, want it empty", n.Value)
	}
}

// Enter while an IME converts confirms the conversion, should a platform pass
// the key on at all, and sends nothing; text the IME commits keeps its line
// breaks.
func TestInput_SubmitOnEnterLeavesTheIMEAlone(t *testing.T) {
	var sent []string
	tt := gowakutest.New(t, chatBox{multiline: true, submitOnEnter: true, sent: &sent}, pressableFonts)
	tt.Tree().DispatchComposition(&input.Composition{Text: "にほん", Cursor: len("にほん")})
	tt.Key(input.KeyEnter, 0)
	if len(sent) != 0 {
		t.Fatalf("Enter during a composition sent %q", sent)
	}
	tt.Tree().DispatchComposition(nil)
	tt.Type("日本\n語")
	if n, _ := tt.Find("Message"); n.Value != "日本\n語" {
		t.Errorf("the field holds %q, want the committed line break kept", n.Value)
	}
	tt.Key(input.KeyEnter, 0)
	if want := []string{"日本\n語"}; !slices.Equal(sent, want) {
		t.Errorf("Enter sent %q, want %q", sent, want)
	}
}

// Without SubmitOnEnter a multiline field breaks the line on Enter, with or
// without Shift, and submits nothing; a single-line field submits on both, as
// the fields of browsers and of the platforms do.
func TestInput_EnterWithoutSubmitOnEnter(t *testing.T) {
	var sent []string
	tt := gowakutest.New(t, chatBox{multiline: true, sent: &sent}, pressableFonts)
	tt.Type("a")
	tt.Key(input.KeyEnter, 0)
	tt.Key(input.KeyEnter, input.ModShift)
	if n, _ := tt.Find("Message"); n.Value != "a\n\n" || len(sent) != 0 {
		t.Errorf("multiline: text %q, sent %q; want %q and nothing sent", n.Value, sent, "a\n\n")
	}

	sent = nil
	tt = gowakutest.New(t, chatBox{sent: &sent}, pressableFonts)
	tt.Type("b")
	tt.Key(input.KeyEnter, 0)
	tt.Type("c")
	tt.Key(input.KeyEnter, input.ModShift)
	if want := []string{"b", "c"}; !slices.Equal(sent, want) {
		t.Errorf("single line: sent %q, want %q", sent, want)
	}
}
