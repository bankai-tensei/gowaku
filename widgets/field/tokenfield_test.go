package field

import (
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// tokenPick records what a TokenField asks for, and lets the test change the
// tokens as the parent.
type tokenPick struct {
	changes [][]string
	set     func([]string)
}

func tokenRig(t *testing.T, w TokenField, p *tokenPick) *fieldRig {
	t.Helper()
	if w.Label == "" {
		w.Label = "タグ"
	}
	r := newFieldRig(t, basic.SizedBox{Width: 300, Child: formHolder[[]string]{
		Initial: w.Tokens,
		Build: func(v []string, set func([]string)) core.Widget {
			p.set = set
			f := w
			f.Tokens = v
			f.OnChange = func(n []string) {
				p.changes = append(p.changes, n)
				set(n)
			}
			return f
		},
	}})
	r.act(r.must(semantics.RoleTextField, ""), semantics.ActionFocus)
	return r
}

// tokens returns the labels of the items of the list, and the selected one.
func (r *fieldRig) tokens() (labels []string, selected string) {
	for _, n := range r.all(semantics.RoleListItem) {
		labels = append(labels, n.Label)
		if n.State&semantics.Selected != 0 {
			selected = n.Label
		}
	}
	return labels, selected
}

func (r *fieldRig) wantTokens(selected string, want ...string) {
	r.T.Helper()
	got, sel := r.tokens()
	if strings.Join(got, "|") != strings.Join(want, "|") || sel != selected {
		r.T.Errorf("tokens %q selected %q, want %q selected %q", got, sel, want, selected)
	}
}

func (p *tokenPick) last() []string {
	if len(p.changes) == 0 {
		return nil
	}
	return p.changes[len(p.changes)-1]
}

func TestTokenField_EnterMakesAToken(t *testing.T) {
	var p tokenPick
	given := []string{"go"}
	r := tokenRig(t, TokenField{Tokens: given}, &p)
	r.typeText(" 東京 ")
	r.Key(input.KeyEnter, 0)
	if !slices.Equal(p.last(), []string{"go", "東京"}) {
		t.Errorf("got %q", p.changes)
	}
	if given[0] != "go" || len(given) != 1 {
		t.Error("the parent's slice must not change")
	}
	r.wantText("")
	r.wantTokens("", "go", "東京")
	// Enter with nothing typed makes no token.
	r.Key(input.KeyEnter, 0)
	if len(p.changes) != 1 {
		t.Errorf("got %q", p.changes)
	}
}

func TestTokenField_EnterDuringCompositionCommitsTheIME(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{}, &p)
	r.compose(&input.Composition{Text: "とうきょう", Cursor: len("とうきょう")})
	r.compose(&input.Composition{Text: "東京", Cursor: len("東京")})
	// The Enter that confirms the conversion: a platform that passed it on
	// must still not make a token of it.
	r.Key(input.KeyEnter, 0)
	if len(p.changes) != 0 {
		t.Fatalf("Enter during composition made %q", p.changes)
	}
	// The IME commits.
	r.compose(nil)
	r.typeText("東京")
	if len(p.changes) != 0 {
		t.Fatalf("the commit made %q", p.changes)
	}
	r.wantText("東京")
	r.Key(input.KeyEnter, 0)
	if !slices.Equal(p.last(), []string{"東京"}) {
		t.Errorf("got %q", p.changes)
	}
	// Backspace and Left during composition are the IME's too.
	r.compose(&input.Composition{Text: "お", Cursor: len("お")})
	r.Key(input.KeyBackspace, 0)
	r.Key(input.KeyLeft, 0)
	r.wantTokens("", "東京")
}

func TestTokenField_SeparatorsEndTokens(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{}, &p)
	r.typeText("a")
	r.typeText(",")
	r.typeText("b")
	r.wantTokens("", "a")
	r.wantText("b")
	// A Japanese IME commits the comma key as 、.
	r.typeText("、")
	r.wantTokens("", "a", "b")
	r.wantText("")

	var q tokenPick
	r = tokenRig(t, TokenField{Separators: ";"}, &q)
	r.typeText("x,y;")
	r.wantTokens("", "x,y")
}

func TestTokenField_PasteSplits(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{}, &p)
	r.host.Clipboard = "x, y\nz,, w"
	r.Key(input.KeyV, input.ModControl)
	r.wantTokens("", "x", "y", "z")
	r.wantText("w")
	if len(p.changes) != 1 {
		t.Errorf("a paste is one change, got %q", p.changes)
	}
}

func TestTokenField_BackspaceSelectsThenRemoves(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Tokens: []string{"a", "b", "c"}}, &p)
	r.typeText("hi")
	r.Key(input.KeyBackspace, 0)
	r.Key(input.KeyBackspace, 0)
	r.wantText("")
	r.wantTokens("", "a", "b", "c")
	r.Key(input.KeyBackspace, 0)
	r.wantTokens("c", "a", "b", "c")
	if len(p.changes) != 0 {
		t.Fatal("the first Backspace only selects")
	}
	r.Key(input.KeyBackspace, 0)
	r.wantTokens("", "a", "b")
	r.Key(input.KeyBackspace, 0)
	r.wantTokens("b", "a", "b")
	// Typing leaves the token.
	r.typeText("z")
	r.wantTokens("", "a", "b")
	r.wantText("z")
}

func TestTokenField_ArrowsMoveThroughTheTokens(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Tokens: []string{"a", "b", "c"}}, &p)
	r.Key(input.KeyLeft, 0)
	r.wantTokens("c", "a", "b", "c")
	r.Key(input.KeyLeft, 0)
	r.Key(input.KeyLeft, 0)
	r.Key(input.KeyLeft, 0)
	r.wantTokens("a", "a", "b", "c")
	r.Key(input.KeyRight, 0)
	r.wantTokens("b", "a", "b", "c")
	r.Key(input.KeyDelete, 0)
	r.wantTokens("", "a", "c")
	r.Key(input.KeyLeft, 0)
	r.Key(input.KeyRight, 0)
	r.wantTokens("", "a", "c")
	r.typeText("d")
	r.wantText("d")
	// Left inside the text moves the caret.
	r.Key(input.KeyLeft, 0)
	r.wantTokens("", "a", "c")
	r.Key(input.KeyLeft, 0)
	r.wantTokens("c", "a", "c")
	r.Key(input.KeyEscape, 0)
	r.wantTokens("", "a", "c")
}

func TestTokenField_Duplicates(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Tokens: []string{"a"}}, &p)
	r.typeText("a")
	r.Key(input.KeyEnter, 0)
	if len(p.changes) != 0 {
		t.Errorf("a duplicate must be dropped, got %q", p.changes)
	}
	r.wantText("")

	var q tokenPick
	r = tokenRig(t, TokenField{Tokens: []string{"a"}, AllowDuplicates: true}, &q)
	r.typeText("a,a,")
	r.wantTokens("", "a", "a", "a")
	// Removing the middle one keeps the others.
	r.Key(input.KeyLeft, 0)
	r.Key(input.KeyLeft, 0)
	r.Key(input.KeyBackspace, 0)
	r.wantTokens("", "a", "a")
}

func TestTokenField_MaxTokens(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Tokens: []string{"a"}, MaxTokens: 2}, &p)
	r.typeText("b,c,d")
	r.wantTokens("", "a", "b")
	r.wantText("c,d")
	r.Key(input.KeyEnter, 0)
	r.wantTokens("", "a", "b")
}

func TestTokenField_Validate(t *testing.T) {
	var p tokenPick
	validate := func(s string) string {
		if !strings.Contains(s, "@") {
			return "メールアドレスではありません"
		}
		return ""
	}
	r := tokenRig(t, TokenField{Validate: validate}, &p)
	r.typeText("a@x, bad, c@x")
	r.wantTokens("", "a@x")
	r.wantText("bad,c@x")
	n := r.must(semantics.RoleTextField, "")
	if n.State&semantics.Invalid == 0 || n.Hint != "メールアドレスではありません" {
		t.Errorf("got %+v", n)
	}
	if l := r.must(semantics.RoleList, "タグ"); l.Hint != "メールアドレスではありません" {
		t.Errorf("got %+v", l)
	}
	r.Key(input.KeyA, input.ModControl)
	r.typeText("b@x")
	if n := r.must(semantics.RoleTextField, ""); n.State&semantics.Invalid != 0 {
		t.Errorf("typing clears the error, got %+v", n)
	}
	r.Key(input.KeyEnter, 0)
	r.wantTokens("", "a@x", "b@x")
}

func TestTokenField_RebuildWithFewerTokensWhileSelected(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Tokens: []string{"a", "b", "c"}}, &p)
	r.Key(input.KeyLeft, 0)
	r.Key(input.KeyLeft, 0)
	r.wantTokens("b", "a", "b", "c")
	// The parent takes away a: the selection follows b.
	p.set([]string{"b", "c"})
	r.frame()
	r.wantTokens("b", "b", "c")
	r.Key(input.KeyBackspace, 0)
	if !slices.Equal(p.last(), []string{"c"}) {
		t.Errorf("got %q", p.changes)
	}
	// The parent takes away the selected token: the selection goes.
	r.Key(input.KeyLeft, 0)
	r.wantTokens("c", "c")
	p.set(nil)
	r.frame()
	r.wantTokens("")
	r.Key(input.KeyBackspace, 0)
	r.Key(input.KeyDelete, 0)
	if len(p.changes) != 1 {
		t.Errorf("got %q", p.changes)
	}
}

func TestTokenField_RemoveButtons(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Tokens: []string{"a", "b", "c"}}, &p)
	r.click(r.must(semantics.RoleButton, "Remove b"))
	if !slices.Equal(p.last(), []string{"a", "c"}) {
		t.Errorf("got %q", p.changes)
	}
	r.wantFocus(semantics.RoleTextField, "")
}

func TestTokenField_LeavingCommits(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{}, &p)
	r.typeText("draft")
	r.tree.RequestFocus(nil)
	r.frame()
	r.wantTokens("", "draft")
}

func TestTokenField_WrapsOntoLines(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Tokens: []string{"いちばん", "にばんめ", "さんばんめ", "よんばんめ", "ごばんめ", "ろくばんめ"}}, &p)
	items := r.all(semantics.RoleListItem)
	if len(items) != 6 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].Rect.Y == items[5].Rect.Y {
		t.Errorf("six chips in 300 must wrap: %v %v", items[0].Rect, items[5].Rect)
	}
	field := r.must(semantics.RoleTextField, "").Rect
	box := r.must(semantics.RoleList, "タグ").Rect
	if field.Y+field.H > box.Y+box.H || field.X+field.W > box.X+box.W {
		t.Errorf("the text %v must stay in the box %v", field, box)
	}
}

func TestTokenField_ZeroValueAndDisabled(t *testing.T) {
	r := newFieldRig(t, basic.SizedBox{Width: 300, Child: TokenField{}})
	if l := r.must(semantics.RoleList, ""); l.State&semantics.Disabled == 0 {
		t.Errorf("got %+v", l)
	}
	if r.Find(semantics.RoleTextField, "") != nil {
		t.Error("a disabled field has no text to type")
	}
	r = newFieldRig(t, basic.SizedBox{Width: 300, Child: TokenField{Tokens: []string{"a"}}})
	r.wantTokens("", "a")
	if r.Find(semantics.RoleButton, "") != nil {
		t.Error("a disabled field has no remove buttons")
	}
}

func TestTokenField_Placeholder(t *testing.T) {
	var p tokenPick
	r := tokenRig(t, TokenField{Placeholder: "タグを追加"}, &p)
	// The text field is named as the list is, and the placeholder is its hint.
	if n := r.Find(semantics.RoleTextField, "タグ"); n == nil || n.Hint != "タグを追加" {
		t.Errorf("the placeholder must show without tokens: %+v", n)
	}
	r.typeText("x,")
	if n := r.Find(semantics.RoleTextField, "タグ"); n == nil || n.Hint != "" {
		t.Errorf("the placeholder must go with the first token: %+v", n)
	}
}
