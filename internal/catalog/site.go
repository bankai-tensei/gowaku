package catalog

import "encoding/json"

// SiteURL is where the website of gowaku is published. The catalog pages link
// to its catalog, which can search and filter.
const SiteURL = "https://bankai-tensei.github.io/gowaku/"

// siteCatalog returns the URL of the catalog of the website in lang.
func siteCatalog(lang string) string {
	if lang == "ja" {
		return SiteURL + "ja/catalog/"
	}
	return SiteURL + "catalog/"
}

// siteCategory and siteEntry are the catalog as the website reads it, from
// catalog.json.
type siteCategory struct {
	Slug    string `json:"slug"`
	JA      string `json:"ja"`
	EN      string `json:"en"`
	AboutJA string `json:"aboutJa"`
	AboutEN string `json:"aboutEn"`
}

type siteEntry struct {
	Name     string   `json:"name"`
	Import   string   `json:"import"`
	Also     []string `json:"also,omitempty"`
	Category string   `json:"category"`
	JA       string   `json:"ja"`
	EN       string   `json:"en"`
	Code     string   `json:"code"`
	Picture  string   `json:"picture,omitempty"`
	Width    int      `json:"width,omitempty"`
	Height   int      `json:"height,omitempty"`
}

type siteData struct {
	Categories []siteCategory `json:"categories"`
	Entries    []siteEntry    `json:"entries"`
}

// siteJSON returns catalog.json, the data of pages for the website
// (website/ in the repository).
func siteJSON(pages []page) (string, error) {
	var d siteData
	for _, c := range Categories() {
		d.Categories = append(d.Categories, siteCategory(c))
	}
	for i := range pages {
		p := &pages[i]
		e := siteEntry{Name: p.Name, Import: p.ImportPath(), Also: p.Also, Category: p.Category, JA: p.JA, EN: p.EN, Code: p.Example}
		if !p.NoPicture {
			e.Picture, e.Width, e.Height = p.File(), p.Width, p.Height
		}
		d.Entries = append(d.Entries, e)
	}
	data, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}
