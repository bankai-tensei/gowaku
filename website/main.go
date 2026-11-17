// Command website builds the website of gowaku: the landing page, the guides
// and the widget catalog, in English and Japanese, as static files for GitHub
// Pages. It reads the Markdown of the repository and docs/catalog/catalog.json,
// so the site never disagrees with the docs.
//
//	go run . -out out                    # build into out/
//	go run . -out out -serve :7901       # build, serve, and rebuild on reload
//
// Run it from website/. It is a module of its own so that the Markdown and
// highlighting libraries stay out of the go.mod of gowaku.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	root := flag.String("root", "..", "the root of the gowaku repository")
	out := flag.String("out", "out", "the directory to write the site into")
	base := flag.String("base", "/gowaku/", "the path the site is served under")
	serve := flag.String("serve", "", "serve the site on this address and rebuild it on every page load")
	flag.Parse()

	if !strings.HasSuffix(*base, "/") {
		*base += "/"
	}
	if *serve != "" {
		*base = "/"
		files = os.DirFS(".")
	}
	b := &builder{root: *root, out: *out, base: *base}
	start := time.Now()
	if err := b.build(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("website: %d pages in %s in %v\n", b.pages, *out, time.Since(start).Round(time.Millisecond))
	if *serve == "" {
		return
	}
	var mu sync.Mutex
	files := http.FileServer(http.Dir(*out))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || strings.HasSuffix(r.URL.Path, ".html") {
			mu.Lock()
			err := (&builder{root: *root, out: *out, base: *base}).build()
			mu.Unlock()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
	fmt.Printf("website: serving on http://%s/\n", strings.TrimPrefix(*serve, ":"))
	if strings.HasPrefix(*serve, ":") {
		*serve = "127.0.0.1" + *serve
	}
	log.Fatal(http.ListenAndServe(*serve, nil))
}

// writeFile writes data to path under the output directory.
func (b *builder) writeFile(rel string, data []byte) error {
	path := filepath.Join(b.out, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// copyAsset copies a file of the repository to assets/ in the output, unless an
// up-to-date copy is there, and returns its URL.
func (b *builder) copyAsset(repoPath string) (string, error) {
	src := filepath.Join(b.root, filepath.FromSlash(repoPath))
	dst := filepath.Join(b.out, "assets", filepath.FromSlash(repoPath))
	si, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if di, err := os.Stat(dst); err != nil || di.ModTime().Before(si.ModTime()) || di.Size() != si.Size() {
		data, err := os.ReadFile(src)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return "", err
		}
	}
	return b.base + "assets/" + repoPath, nil
}
