package gowakutest_test

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

// notes is a field labeled Notes that keeps its text in *value.
type notes struct {
	gowaku.StatefulBase
	value *string
}

type notesState struct{ gowaku.StateBase }

func (notes) CreateState() gowaku.State { return &notesState{} }

func (s *notesState) Build(gowaku.BuildContext) gowaku.Widget {
	w := s.Widget().(notes)
	return basic.Column{Children: []gowaku.Widget{field.Input{
		Label: "Notes", Value: *w.value,
		OnChange: func(v string) { s.SetState(func() { *w.value = v }) },
	}}}
}

func TestClipboard_CopyAndPasteOfAField(t *testing.T) {
	value := "hello"
	tt := gowakutest.New(t, notes{value: &value}, goFonts)
	if got := tt.Clipboard(); got != "" {
		t.Fatalf("a new Tester's clipboard holds %q, want nothing", got)
	}
	tt.Tap("Notes")
	tt.Key(input.KeyA, input.ModControl)
	tt.Key(input.KeyC, input.ModControl)
	if got := tt.Clipboard(); got != "hello" {
		t.Errorf("the clipboard holds %q after a copy, want %q", got, "hello")
	}

	tt.SetClipboard("pasted")
	tt.Key(input.KeyA, input.ModControl)
	tt.Key(input.KeyV, input.ModControl)
	if value != "pasted" {
		t.Errorf("the field holds %q after a paste, want %q", value, "pasted")
	}
}

// copier writes rich content with gowaku.WriteClipboard on a press, and reads
// it back with gowaku.ReadClipboard on another.
type copier struct {
	gowaku.StatelessBase
	read *gowaku.ClipboardContent
}

func (c copier) Build(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.VStack(
		control.Button{Label: "Copy", OnPress: func() {
			gowaku.WriteClipboard(ctx, gowaku.ClipboardContent{Text: "gowaku", HTML: "<b>gowaku</b>", Files: []string{"/tmp/a.txt"}})
		}},
		control.Button{Label: "Paste", OnPress: func() {
			if gowaku.ClipboardHas(ctx)&gowaku.ClipboardFiles != 0 {
				*c.read = gowaku.ReadClipboard(ctx, gowaku.ClipboardText|gowaku.ClipboardFiles)
			}
		}},
	)
}

func TestClipboard_WriteClipboardReachesTheTester(t *testing.T) {
	var read gowaku.ClipboardContent
	tt := gowakutest.New(t, copier{read: &read}, goFonts)
	tt.Tap("Copy")
	if got := tt.ClipboardContent(); got.Text != "gowaku" || got.HTML != "<b>gowaku</b>" || !slices.Equal(got.Files, []string{"/tmp/a.txt"}) {
		t.Errorf("the clipboard holds %+v", got)
	}
	if got := tt.Clipboard(); got != "gowaku" {
		t.Errorf("Clipboard is %q, want the text of the copy", got)
	}

	tt.SetClipboardContent(gowaku.ClipboardContent{Text: "list", HTML: "<i>no</i>", Files: []string{"/tmp/b.txt"}})
	tt.Tap("Paste")
	if read.Text != "list" || read.HTML != "" || !slices.Equal(read.Files, []string{"/tmp/b.txt"}) {
		t.Errorf("ReadClipboard of text and files got %+v", read)
	}

	tt.SetClipboard("plain")
	if got := tt.ClipboardContent(); got.HTML != "" || got.Files != nil {
		t.Errorf("SetClipboard must replace every kind, got %+v", got)
	}
}
