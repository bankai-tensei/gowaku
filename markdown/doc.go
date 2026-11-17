// Package markdown shows Markdown text as gowaku widgets.
//
//	markdown.Markdown{Source: readme, OnLinkTap: openURL}
//
// It is a module of its own, github.com/bankai-tensei/gowaku/markdown, so
// that the main module keeps its short list of dependencies. It
// has its own parser and needs nothing beyond gowaku and golang.org/x/text,
// which gowaku already uses.
//
// The syntax is CommonMark with the GitHub extensions for tables, task lists,
// strikethrough and bare links: ATX and setext headings, paragraphs with soft
// and hard line breaks, emphasis, strong emphasis, strikethrough, code spans,
// fenced and indented code blocks, block quotes, ordered, bullet and task
// lists, links (inline, reference and autolinks), images, thematic breaks and
// pipe tables. A soft line break between two wide characters, as in Japanese,
// adds no space, and the library's own line breaking wraps the text.
//
// Raw HTML is read as GitHub reads a README, within a safe subset: p, div and
// h1 to h6 with align, details and summary, img with width and height,
// picture with sources for the light and the dark theme, a, b, strong, i, em,
// code, kbd, sub, sup, s, del, br, hr, pre and tables. Other tags show as the
// text they are; comments show nothing. Scripts, styles and event handlers
// never run, links to javascript: are text, and nothing is fetched from a
// network (docs/ADR/0119-markdown-html-subset.md).
//
// Inside a field.SelectionArea the text can be selected and copied
// (docs/ADR/0105-markdown-selection.md). Headings get GitHub's anchors, links
// to them scroll the document, and Headings and Controller serve an outline,
// with where each heading is in the source for a source editor beside it
// (docs/ADR/0106-markdown-headings.md).
//
// The package imports gowaku's widgets and the layers below them; nothing in
// gowaku imports it. It uses no unsafe, no syscall and no cgo. See
// docs/ADR/0099-markdown-module.md.
package markdown
