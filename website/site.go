package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed templates static
var embedded embed.FS

// files are the templates and static files: the embedded ones, or those on
// disk while serving, so that edits show on reload.
var files fs.FS = embedded

const repoURL = "https://github.com/bankai-tensei/gowaku"

// A docDef is a Markdown document that becomes a page in each language. An
// empty ja shows the English page in the Japanese site too.
type docDef struct {
	key    string // the route without language, as "docs/layout"
	en, ja string // the source files in the repository
	group  string // the group in the side bar: guides, reference or project
}

var guides = []string{"getting-started", "concepts", "layout", "theme", "motion", "text-and-ime",
	"accessibility", "windows-and-os", "testing-and-preview", "packaging", "platforms", "recipes"}

func docDefs() []docDef {
	d := []docDef{{key: "docs", en: "docs/guide/en/README.md", ja: "docs/guide/ja/README.md", group: "guides"}}
	for _, g := range guides {
		d = append(d, docDef{key: "docs/" + g, en: "docs/guide/en/" + g + ".md", ja: "docs/guide/ja/" + g + ".md", group: "guides"})
	}
	return append(d,
		docDef{key: "docs/architecture", en: "docs/ARCHITECTURE.md", ja: "docs/ARCHITECTURE.ja.md", group: "reference"},
		docDef{key: "docs/design", en: "docs/DESIGN.md", ja: "docs/DESIGN.ja.md", group: "reference"},
		docDef{key: "docs/benchmarks", en: "docs/BENCH.md", ja: "docs/BENCH.ja.md", group: "reference"},
		docDef{key: "docs/roadmap", en: "ROADMAP.md", ja: "ROADMAP.ja.md", group: "reference"},
		docDef{key: "docs/contributing", en: "CONTRIBUTING.md", ja: "CONTRIBUTING.ja.md", group: "project"},
		docDef{key: "docs/changelog", en: "CHANGELOG.md", group: "project"},
		docDef{key: "docs/security", en: "SECURITY.md", group: "project"},
		docDef{key: "docs/third-party", en: "THIRD_PARTY_NOTICES.md", group: "project"},
	)
}

// builder holds the state of one build.
type builder struct {
	root, out, base string
	pages           int

	routes map[string]string // repository path to URL, for the links between documents
	tmpl   *template.Template
	docs   map[string]map[string]*doc // key, then language
	cat    *catalogData
}

// route returns the URL of key in lang.
func (b *builder) route(key, lang string) string {
	u := b.base
	if lang == "ja" {
		u += "ja/"
	}
	if key != "" {
		u += key + "/"
	}
	return u
}

func (b *builder) build() error {
	b.routes = map[string]string{
		"README.md":    b.route("", "en"),
		"README.ja.md": b.route("", "ja"),
	}
	defs := docDefs()
	for _, d := range defs {
		b.routes[d.en] = b.route(d.key, "en")
		if d.ja != "" {
			b.routes[d.ja] = b.route(d.key, "ja")
		}
	}
	cat, err := loadCatalog(filepath.Join(b.root, "docs", "catalog", "catalog.json"))
	if err != nil {
		return err
	}
	b.cat = cat
	b.routes["docs/catalog/README.md"] = b.route("catalog", "en")
	for _, lang := range []string{"en", "ja"} {
		b.routes["docs/catalog/"+lang+"/README.md"] = b.route("catalog", lang)
		for _, c := range cat.Categories {
			b.routes["docs/catalog/"+lang+"/"+c.Slug+".md"] = b.route("catalog/"+c.Slug, lang)
		}
	}

	b.tmpl, err = template.New("").Funcs(template.FuncMap{
		"asset": func(p string) string { return b.base + "static/" + p },
		"img": func(p string) (string, error) {
			return b.copyAsset(p)
		},
		"t": func(lang, key string) template.HTML { return template.HTML(word(lang, key)) },
	}).ParseFS(files, "templates/*.html")
	if err != nil {
		return err
	}

	b.docs = map[string]map[string]*doc{}
	for _, d := range defs {
		b.docs[d.key] = map[string]*doc{}
		for _, lang := range []string{"en", "ja"} {
			src := d.en
			if lang == "ja" && d.ja != "" {
				src = d.ja
			}
			dc, err := b.readDoc(src)
			if err != nil {
				return err
			}
			b.docs[d.key][lang] = dc
		}
	}

	for _, lang := range []string{"en", "ja"} {
		if err := b.home(lang); err != nil {
			return err
		}
		for i, d := range defs {
			if err := b.docPage(lang, defs, i, d); err != nil {
				return fmt.Errorf("%s: %w", d.key, err)
			}
		}
		if err := b.catalogPages(lang); err != nil {
			return err
		}
	}
	if err := b.notFound(); err != nil {
		return err
	}
	if err := b.llms(); err != nil {
		return err
	}
	return b.static()
}

// page is what every template gets.
type page struct {
	Lang        string
	Title       string
	Description string
	Section     string // home, docs or catalog
	URL         string
	Alt         string // the same page in the other language
	Base        string
	Body        template.HTML
	TOC         []heading
	Nav         []navGroup
	Prev, Next  *navLink
	Edit        string
	Data        any
}

type navGroup struct {
	Title string
	Links []navLink
}

type navLink struct {
	Title, URL string
	On         bool
}

func (b *builder) render(name, rel string, p *page) error {
	p.Base = b.base
	var buf bytes.Buffer
	if err := b.tmpl.ExecuteTemplate(&buf, name, p); err != nil {
		return err
	}
	b.pages++
	return b.writeFile(rel, buf.Bytes())
}

// outPath returns the file of a route.
func (b *builder) outPath(url string) string {
	return path.Join(strings.TrimPrefix(url, b.base), "index.html")
}

func (b *builder) docNav(lang string, defs []docDef, current string) []navGroup {
	groups := []navGroup{{Title: word(lang, "nav.guides")}, {Title: word(lang, "nav.reference")}, {Title: word(lang, "nav.project")}}
	idx := map[string]int{"guides": 0, "reference": 1, "project": 2}
	for _, d := range defs {
		title := b.docs[d.key][lang].Title
		if w, ok := words[lang]["short."+d.key]; ok {
			title = w
		}
		g := &groups[idx[d.group]]
		g.Links = append(g.Links, navLink{Title: title, URL: b.route(d.key, lang), On: d.key == current})
	}
	return groups
}

func (b *builder) docPage(lang string, defs []docDef, i int, d docDef) error {
	dc := b.docs[d.key][lang]
	src := d.en
	if lang == "ja" && d.ja != "" {
		src = d.ja
	}
	p := &page{
		Lang: lang, Title: dc.Title, Description: dc.Summary, Section: "docs",
		URL: b.route(d.key, lang), Alt: b.route(d.key, other(lang)),
		Body: dc.HTML, TOC: dc.TOC, Nav: b.docNav(lang, defs, d.key),
		Edit: repoURL + "/edit/main/" + src,
	}
	if d.group == "guides" {
		if i > 0 && defs[i-1].group == "guides" {
			p.Prev = &navLink{Title: b.docs[defs[i-1].key][lang].Title, URL: b.route(defs[i-1].key, lang)}
		}
		if i+1 < len(defs) && defs[i+1].group == "guides" {
			p.Next = &navLink{Title: b.docs[defs[i+1].key][lang].Title, URL: b.route(defs[i+1].key, lang)}
		}
	}
	return b.render("doc", b.outPath(p.URL), p)
}

func (b *builder) notFound() error {
	p := &page{Lang: "en", Title: "Not found", Section: "home", URL: b.base, Alt: b.route("", "ja")}
	return b.render("404", "404.html", p)
}

// static writes the style sheet, the script and the files GitHub Pages wants.
func (b *builder) static() error {
	err := fs.WalkDir(files, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, p)
		if err != nil {
			return err
		}
		return b.writeFile(p, data)
	})
	if err != nil {
		return err
	}
	for _, f := range []string{"gowaku-mark.svg", "gowaku-mark-dark.svg"} {
		data, err := os.ReadFile(filepath.Join(b.root, "docs", "brand", f))
		if err != nil {
			return err
		}
		if err := b.writeFile("static/"+f, data); err != nil {
			return err
		}
	}
	return b.writeFile(".nojekyll", nil)
}

func other(lang string) string {
	if lang == "ja" {
		return "en"
	}
	return "ja"
}
