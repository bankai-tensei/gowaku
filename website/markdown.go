package main

import (
	"bytes"
	"html"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// doc is a rendered Markdown document.
type doc struct {
	Title   string
	Summary string
	HTML    template.HTML
	TOC     []heading
}

type heading struct {
	Level int
	ID    string
	Text  string
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

// slugIDs makes heading IDs as GitHub does, so that links written for GitHub
// work on the site: lower case, letters of any script, digits, - and _ kept,
// spaces turned into -, and -1, -2 for repeats.
type slugIDs struct{ seen map[string]int }

func (s *slugIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	id := slug(string(value))
	n := s.seen[id]
	s.seen[id] = n + 1
	if n > 0 {
		id += "-" + itoa(n)
	}
	return []byte(id)
}

func (s *slugIDs) Put(value []byte) { s.seen[string(value)]++ }

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

var langLine = regexp.MustCompile(`^\s*\[[^\]]+\]\([^)]*\)(\s*·\s*\[[^\]]+\]\([^)]*\))*\s*$`)

// readDoc reads and renders a document of the repository. The first heading
// becomes the title and leaves the body, and so does a line of links right
// under it (to the index and the other language), which the site shows in its
// own frame.
func (b *builder) readDoc(src string) (*doc, error) {
	data, err := os.ReadFile(filepath.Join(b.root, filepath.FromSlash(src)))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	d := &doc{}
	for i, l := range lines {
		if strings.HasPrefix(l, "# ") {
			d.Title = strings.TrimSpace(l[2:])
			lines = append(lines[:i:i], lines[i+1:]...)
			for j := i; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if langLine.MatchString(lines[j]) {
					lines = append(lines[:j:j], lines[j+1:]...)
				}
				break
			}
			break
		}
	}
	body := []byte(strings.Join(lines, "\n"))
	ctx := parser.NewContext(parser.WithIDs(&slugIDs{seen: map[string]int{}}))
	tree := md.Parser().Parse(text.NewReader(body), parser.WithContext(ctx))
	_ = ast.Walk(tree, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			if n.Level == 2 || n.Level == 3 {
				id, _ := n.AttributeString("id")
				ids, _ := id.([]byte)
				d.TOC = append(d.TOC, heading{Level: n.Level, ID: string(ids), Text: plain(n, body)})
			}
		case *ast.Paragraph:
			if d.Summary == "" && n.Parent() == tree {
				d.Summary = plain(n, body)
			}
		}
		return ast.WalkContinue, nil
	})
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, body, tree); err != nil {
		return nil, err
	}
	out := b.finish(src, buf.String())
	d.HTML = template.HTML(out)
	if r := []rune(d.Summary); len(r) > 120 {
		d.Summary = string(r[:120]) + "…"
	}
	return d, nil
}

// plain returns the text of n without markup.
func plain(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Segment.Value(src))
			if c.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.CodeSpan:
			for k := c.FirstChild(); k != nil; k = k.NextSibling() {
				if t, ok := k.(*ast.Text); ok {
					b.Write(t.Segment.Value(src))
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

var (
	attrRE    = regexp.MustCompile(`\b(href|src|srcset)="([^"]*)"`)
	pictureRE = regexp.MustCompile(`(?s)<picture>\s*<source media="\(prefers-color-scheme: dark\)" srcset="([^"]+)">\s*<img ([^>]*?)/?>\s*</picture>`)
	codeRE    = regexp.MustCompile(`(?s)<pre><code class="language-([\w-]+)">(.*?)</code></pre>`)
	plainRE   = regexp.MustCompile(`(?s)<pre><code>(.*?)</code></pre>`)
	tableRE   = regexp.MustCompile(`(?s)<table>(.*?)</table>`)
)

// finish rewrites the links of a rendered document of src for the site,
// splits pictures for the light and dark themes, highlights code, and lets
// wide tables scroll.
func (b *builder) finish(src, s string) string {
	s = attrRE.ReplaceAllStringFunc(s, func(m string) string {
		g := attrRE.FindStringSubmatch(m)
		return g[1] + `="` + b.resolve(src, html.UnescapeString(g[2])) + `"`
	})
	s = pictureRE.ReplaceAllString(s, `<img class="light-only" $2><img class="dark-only" src="$1" ${2}data-dark>`)
	s = regexp.MustCompile(`<img class="dark-only" src="([^"]+)" ([^>]*?)src="[^"]*"([^>]*?)data-dark>`).ReplaceAllString(s, `<img class="dark-only" src="$1" $2$3>`)
	s = codeRE.ReplaceAllStringFunc(s, func(m string) string {
		g := codeRE.FindStringSubmatch(m)
		return highlight(html.UnescapeString(g[2]), g[1])
	})
	s = plainRE.ReplaceAllString(s, `<div class="code"><pre><code>$1</code></pre></div>`)
	s = tableRE.ReplaceAllString(s, `<div class="table"><table>$1</table></div>`)
	return s
}

// resolve turns a link written in src into a link of the site: to a page when
// the target is one, to a copied asset when it is a picture, and to GitHub
// otherwise.
func (b *builder) resolve(src, ref string) string {
	if ref == "" || strings.HasPrefix(ref, "#") || strings.Contains(ref, "://") || strings.HasPrefix(ref, "mailto:") || strings.HasPrefix(ref, "/") {
		return ref
	}
	if strings.Contains(ref, ", ") { // a srcset with several candidates is not used here
		return ref
	}
	p, frag, _ := strings.Cut(ref, "#")
	if frag != "" {
		frag = "#" + frag
	}
	target := path.Clean(path.Join(path.Dir(src), p))
	if u, ok := b.routes[target]; ok {
		return u + frag
	}
	full := filepath.Join(b.root, filepath.FromSlash(target))
	fi, err := os.Stat(full)
	if err != nil {
		return ref
	}
	switch strings.ToLower(path.Ext(target)) {
	case ".png", ".svg", ".jpg", ".jpeg", ".gif", ".webp":
		if u, err := b.copyAsset(target); err == nil {
			return u
		}
	}
	if fi.IsDir() {
		return repoURL + "/tree/main/" + target
	}
	return repoURL + "/blob/main/" + target + frag
}

var formatter = chromahtml.New(chromahtml.WithClasses(true), chromahtml.PreventSurroundingPre(true))

// highlight returns code as a highlighted block with a copy button.
func highlight(code, lang string) string {
	code = strings.TrimRight(code, "\n")
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	var buf bytes.Buffer
	it, err := lexer.Tokenise(nil, code)
	if err != nil || formatter.Format(&buf, styles.Fallback, it) != nil {
		buf.Reset()
		buf.WriteString(html.EscapeString(code))
	}
	label := lang
	if lang == "bash" || lang == "sh" || lang == "shell" {
		label = "shell"
	}
	return `<div class="code" data-lang="` + label + `"><button class="copy" type="button" aria-label="Copy">` + copyIcon +
		`</button><pre><code class="chroma">` + buf.String() + `</code></pre></div>`
}

const copyIcon = `<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><rect x="5.25" y="5.25" width="8" height="8" rx="2" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10.75 3.5V3.25a1.5 1.5 0 0 0-1.5-1.5h-5a1.5 1.5 0 0 0-1.5 1.5v5a1.5 1.5 0 0 0 1.5 1.5h.25" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>`

// inline renders one paragraph of Markdown, as the descriptions of the
// catalog, without the paragraph around it.
func (b *builder) inline(src, s string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return template.HTML(html.EscapeString(s))
	}
	out := strings.TrimSpace(buf.String())
	out = strings.TrimSuffix(strings.TrimPrefix(out, "<p>"), "</p>")
	return template.HTML(b.finish(src, out))
}
