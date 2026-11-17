package main

import (
	"encoding/json"
	"html/template"
	"os"
	"strings"
)

// catalogData is docs/catalog/catalog.json, which internal/cmd/catalog writes.
type catalogData struct {
	Categories []struct {
		Slug    string `json:"slug"`
		JA      string `json:"ja"`
		EN      string `json:"en"`
		AboutJA string `json:"aboutJa"`
		AboutEN string `json:"aboutEn"`
	} `json:"categories"`
	Entries []struct {
		Name     string   `json:"name"`
		Import   string   `json:"import"`
		Also     []string `json:"also"`
		Category string   `json:"category"`
		JA       string   `json:"ja"`
		EN       string   `json:"en"`
		Code     string   `json:"code"`
		Picture  string   `json:"picture"`
		Width    int      `json:"width"`
		Height   int      `json:"height"`
	} `json:"entries"`
}

func loadCatalog(path string) (*catalogData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c catalogData
	return &c, json.Unmarshal(data, &c)
}

// catEntry and catCategory are what the catalog templates show.
type catEntry struct {
	Name, Type, Package, ID string
	Import                  string
	Also                    []string
	Category, CategoryTitle string
	URL                     string
	Text                    template.HTML
	Short                   string
	Search                  string
	Code                    template.HTML
	Light, Dark             string
	Width, Height           int
}

type catCategory struct {
	Slug, Title, URL string
	About            template.HTML
	Entries          []catEntry
	On               bool
}

func entryID(name string) string { return strings.ToLower(strings.ReplaceAll(name, ".", "")) }

// firstSentence returns the first sentence of a description, without its
// Markdown, for the cards of the index.
func firstSentence(s, lang string) string {
	s = strings.ReplaceAll(s, "`", "")
	end := ". "
	if lang == "ja" {
		end = "。"
	}
	if i := strings.Index(s, end); i >= 0 {
		s = s[:i+len(end)]
	}
	return strings.TrimSpace(s)
}

func (b *builder) catalogModel(lang string) ([]catCategory, error) {
	src := "docs/catalog/" + lang + "/README.md"
	var cats []catCategory
	for _, c := range b.cat.Categories {
		title, about := c.EN, c.AboutEN
		if lang == "ja" {
			title, about = c.JA, c.AboutJA
		}
		cats = append(cats, catCategory{Slug: c.Slug, Title: title, URL: b.route("catalog/"+c.Slug, lang), About: b.inline(src, about)})
	}
	for _, e := range b.cat.Entries {
		for i := range cats {
			if cats[i].Slug != e.Category {
				continue
			}
			desc := e.EN
			if lang == "ja" {
				desc = e.JA
			}
			pkg, typ, _ := strings.Cut(e.Name, ".")
			ce := catEntry{
				Name: e.Name, Type: typ, Package: pkg, ID: entryID(e.Name), Import: e.Import, Also: e.Also,
				Category: e.Category, CategoryTitle: cats[i].Title,
				URL:  cats[i].URL + "#" + entryID(e.Name),
				Text: b.inline(src, desc), Short: firstSentence(desc, lang),
				Search: strings.ToLower(e.Name + " " + strings.Join(e.Also, " ") + " " + e.EN + " " + e.JA),
				Code:   template.HTML(highlight(e.Code, "go")),
				Width:  e.Width, Height: e.Height,
			}
			if e.Picture != "" {
				var err error
				if ce.Light, err = b.copyAsset("docs/catalog/img/" + e.Picture + "-light.png"); err != nil {
					return nil, err
				}
				if ce.Dark, err = b.copyAsset("docs/catalog/img/" + e.Picture + "-dark.png"); err != nil {
					return nil, err
				}
			}
			cats[i].Entries = append(cats[i].Entries, ce)
		}
	}
	return cats, nil
}

type catalogView struct {
	Categories []catCategory
	Category   *catCategory
	Count      int
	Pictures   int
}

func (b *builder) catalogPages(lang string) error {
	cats, err := b.catalogModel(lang)
	if err != nil {
		return err
	}
	view := catalogView{Categories: cats}
	for _, c := range cats {
		for _, e := range c.Entries {
			view.Count++
			if e.Light != "" {
				view.Pictures++
			}
		}
	}
	p := &page{
		Lang: lang, Title: word(lang, "catalog.title"), Description: word(lang, "catalog.lede"),
		Section: "catalog", URL: b.route("catalog", lang), Alt: b.route("catalog", other(lang)), Data: view,
	}
	if err := b.render("catalog", b.outPath(p.URL), p); err != nil {
		return err
	}
	for i := range cats {
		v := view
		v.Categories = append([]catCategory(nil), cats...)
		v.Categories[i].On = true
		v.Category = &v.Categories[i]
		p := &page{
			Lang: lang, Title: cats[i].Title, Description: strings.ReplaceAll(b.aboutText(i, lang), "`", ""), Section: "catalog",
			URL: cats[i].URL, Alt: b.route("catalog/"+cats[i].Slug, other(lang)), Data: v,
		}
		for _, e := range cats[i].Entries {
			p.TOC = append(p.TOC, heading{Level: 2, ID: e.ID, Text: e.Name})
		}
		if err := b.render("category", b.outPath(p.URL), p); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) aboutText(i int, lang string) string {
	if lang == "ja" {
		return b.cat.Categories[i].AboutJA
	}
	return b.cat.Categories[i].AboutEN
}
