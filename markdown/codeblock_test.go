package markdown

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

func TestMarkdown_CopyButtonCopiesTheCodeAsWritten(t *testing.T) {
	src := "Two blocks:\n\n```go\nfunc main() {\n\tx()\n}\n```\n\n    indented\n"
	tt := gowakutest.New(t, Markdown{Source: src, CopyButton: true}, gowakutest.Options{Width: 480})
	buttons := tt.FindAllRole(semantics.RoleButton, "Copy")
	if len(buttons) != 2 {
		t.Fatalf("%d Copy buttons, want one per code block\n%s", len(buttons), tt.Dump())
	}
	tt.Tap("Copy")
	if got, want := tt.Clipboard(), "func main() {\n\tx()\n}"; got != want {
		t.Errorf("copied %q, want %q with its tab", got, want)
	}
	if _, ok := tt.Find("go"); !ok {
		t.Error("the language is gone from the head of the block")
	}
	if _, ok := tt.FindRole(semantics.RoleButton, "Copied"); !ok {
		t.Errorf("the pressed button should say Copied for a moment\n%s", tt.Dump())
	}
	tt.Advance(copiedFor + 100*time.Millisecond)
	if _, ok := tt.FindRole(semantics.RoleButton, "Copied"); ok {
		t.Error("the button still says Copied after the moment")
	}
	if n := len(tt.FindAllRole(semantics.RoleButton, "Copy")); n != 2 {
		t.Errorf("%d Copy buttons after the moment, want 2", n)
	}
}

func TestMarkdown_NoCopyButtonByDefault(t *testing.T) {
	tt := gowakutest.New(t, Markdown{Source: "```\nx\n```"}, gowakutest.Options{})
	if n := len(tt.FindAllRole(semantics.RoleButton, "Copy")); n != 0 {
		t.Errorf("%d Copy buttons without CopyButton", n)
	}
}

func TestMarkdown_CodeBlockWrapsTheDefault(t *testing.T) {
	var got []CodeBlock
	ran := ""
	tt := gowakutest.New(t, Markdown{
		Source: "```go title=main.go\npackage main\n```\n\n```\nkeep\n```",
		CodeBlock: func(c CodeBlock) core.Widget {
			got = append(got, c)
			if c.Lang == "" {
				return nil
			}
			return basic.Column{Children: []core.Widget{
				c.Default,
				control.Button{Label: "Run", OnPress: func() { ran = c.Code }},
			}}
		},
	}, gowakutest.Options{Width: 480})
	if len(got) != 2 {
		t.Fatalf("CodeBlock ran for %d blocks", len(got))
	}
	if c := got[0]; c.Lang != "go" || c.Info != "go title=main.go" || c.Code != "package main" || c.Open {
		t.Errorf("first block %+v", c)
	}
	tt.Tap("Run")
	if ran != "package main" {
		t.Errorf("the button of the app got %q", ran)
	}
	if _, ok := tt.Find("package main"); !ok {
		t.Error("the default drawing is not inside the wrapper")
	}
	if _, ok := tt.Find("keep"); !ok {
		t.Error("a nil result does not keep the default drawing")
	}
}

func TestMarkdown_UnclosedFenceIsCodeInProgress(t *testing.T) {
	cases := []struct {
		name, src  string
		lang, code string
		open       bool
	}{
		{"fence alone", "```", "", "", true},
		{"fence and language", "Here:\n\n```go", "go", "", true},
		{"partial line", "```go\nfmt.Println(", "go", "fmt.Println(", true},
		{"lines so far", "text\n```py\nx = 1\ny = ", "py", "x = 1\ny = ", true},
		{"in a list item", "- item\n\n  ```sh\n  ls", "sh", "ls", true},
		{"closed", "```go\na\n```", "go", "a", false},
		{"closed by tildes", "~~~\na\n~~~\nafter", "", "a", false},
		{"indented", "    code", "", "code", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []CodeBlock
			tt := gowakutest.New(t, Markdown{Source: c.src, CodeBlock: func(b CodeBlock) core.Widget {
				got = append(got, b)
				return nil
			}}, gowakutest.Options{Width: 480})
			if len(got) != 1 {
				t.Fatalf("%d code blocks\n%s", len(got), tt.Dump())
			}
			if b := got[0]; b.Lang != c.lang || b.Code != c.code || b.Open != c.open {
				t.Errorf("got lang %q code %q open %v, want %q %q %v", b.Lang, b.Code, b.Open, c.lang, c.code, c.open)
			}
			if c.code != "" {
				if _, ok := tt.Find(c.code); !ok {
					t.Errorf("the code does not show\n%s", tt.Dump())
				}
			}
		})
	}
}
