package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// siteOrigin is where GitHub Pages serves the site; base is added to it.
const siteOrigin = "https://bankai-tensei.github.io"

// llms writes llms.txt, an index of the site for language models
// (https://llmstxt.org), and llms-full.txt, the English guides, the reference
// and the whole widget catalog as one Markdown file, so that a coding agent can
// read everything it needs in one request.
func (b *builder) llms() error {
	abs := func(u string) string { return siteOrigin + u }
	var idx strings.Builder
	idx.WriteString("# gowaku\n\n")
	idx.WriteString("> gowaku is a GPU-native desktop UI library for Go with no cgo and a declarative, Flutter-like widget tree. Widgets are value structs that work at their zero value; state lives in a State with SetState; package gowakutest runs a UI under go test without a window and saves screenshots.\n\n")
	idx.WriteString("Module: github.com/bankai-tensei/gowaku. API reference: https://pkg.go.dev/github.com/bankai-tensei/gowaku. Everything below in one file: " + abs(b.base+"llms-full.txt") + "\n\n")
	idx.WriteString("Rules that save time: `gowaku.Widget` and `core.Widget` are the same type. Widgets have no constructors, options or method chains; write struct literals. Find widgets in tests by their label (`tt.Tap(\"Save\")`). To see a UI without a window, use `gowakutest.New(t, w, gowakutest.Options{}).SaveScreenshot(\"x.png\")`.\n\n")
	idx.WriteString("## Guides\n\n")
	for _, d := range docDefs() {
		if d.group != "guides" {
			continue
		}
		dc := b.docs[d.key]["en"]
		fmt.Fprintf(&idx, "- [%s](%s): %s\n", dc.Title, abs(b.route(d.key, "en")), oneLine(dc.Summary))
	}
	idx.WriteString("\n## Reference\n\n")
	for _, d := range docDefs() {
		if d.group == "guides" {
			continue
		}
		dc := b.docs[d.key]["en"]
		fmt.Fprintf(&idx, "- [%s](%s): %s\n", dc.Title, abs(b.route(d.key, "en")), oneLine(dc.Summary))
	}
	idx.WriteString("\n## Widget catalog\n\n")
	for _, c := range b.cat.Categories {
		fmt.Fprintf(&idx, "- [%s](%s): %s\n", c.EN, abs(b.route("catalog/"+c.Slug, "en")), oneLine(strings.ReplaceAll(c.AboutEN, "`", "")))
	}
	if err := b.writeFile("llms.txt", []byte(idx.String())); err != nil {
		return err
	}

	var full strings.Builder
	full.WriteString("# gowaku: the full documentation\n\n")
	full.WriteString("This file holds the English guides, the reference and the widget catalog of gowaku (github.com/bankai-tensei/gowaku) as one document. Relative links point into the repository at https://github.com/bankai-tensei/gowaku.\n\n")
	srcs := []string{"README.md"}
	for _, d := range docDefs() {
		srcs = append(srcs, d.en)
	}
	for _, src := range srcs {
		data, err := os.ReadFile(filepath.Join(b.root, filepath.FromSlash(src)))
		if err != nil {
			return err
		}
		fmt.Fprintf(&full, "\n\n---\n\n<!-- %s -->\n\n", src)
		full.Write(demote(data))
	}
	full.WriteString("\n\n---\n\n## Widget catalog\n\nEvery public widget, with the package to import and an example. The examples use `core.Widget`, which is `gowaku.Widget`; lower-case helpers such as `themed` and `tile` are small functions of internal/catalog/helpers.go.\n")
	for _, c := range b.cat.Categories {
		fmt.Fprintf(&full, "\n### %s\n\n%s\n", c.EN, c.AboutEN)
		for _, e := range b.cat.Entries {
			if e.Category != c.Slug {
				continue
			}
			fmt.Fprintf(&full, "\n#### %s\n\nImport `%s`.", e.Name, e.Import)
			if len(e.Also) > 0 {
				fmt.Fprintf(&full, " Also covers %s.", strings.Join(e.Also, ", "))
			}
			fmt.Fprintf(&full, "\n\n%s\n\n```go\n%s\n```\n", e.EN, strings.TrimRight(e.Code, "\n"))
		}
	}
	return b.writeFile("llms-full.txt", []byte(full.String()))
}

// demote moves the headings of a document one level down, so that each
// document's title is a level-2 heading in the combined file.
func demote(data []byte) []byte {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	fence := false
	for i, l := range lines {
		if strings.HasPrefix(l, "```") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(l, "#") {
			lines[i] = "#" + l
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 160 {
		s = string(r[:157]) + "..."
	}
	return s
}
