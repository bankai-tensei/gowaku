package catalog

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Options says what Generate writes.
type Options struct {
	// Out is the directory of the catalog, docs/catalog.
	Out string
	// Source is the directory of this package, where the examples are read.
	Source string
	// Render draws the pictures into Out/img first and removes the ones of no
	// entry. Without it the pages take the sizes of the pictures in Images.
	Render bool
	// Images is where the pictures are when Render is false. Empty selects
	// Out/img.
	Images string
	// Only, if not empty, renders only the entries of these names, such as
	// "control.Button", or of these types in any package; the pages are
	// written in full.
	Only []string
}

// page is what the writers know of an entry.
type page struct {
	Entry
	Example string
	// Width and Height are the size of the pictures in logical pixels, zero
	// for an entry without them.
	Width, Height int
}

// Generate writes the Markdown pages, catalog.json and, with Render, the
// pictures of the catalog.
func Generate(opts Options) error {
	examples, err := Examples(opts.Source)
	if err != nil {
		return err
	}
	entries := Entries()
	images := opts.Images
	if opts.Render || images == "" {
		images = filepath.Join(opts.Out, "img")
	}
	if opts.Render {
		if err := renderAll(entries, images, opts.Only); err != nil {
			return err
		}
	}
	pages := make([]page, len(entries))
	for i := range entries {
		e := &entries[i]
		p := page{Entry: *e, Example: examples[e.Func()]}
		if p.Example == "" {
			return fmt.Errorf("catalog: %s has no function %s", e.Name, e.Func())
		}
		if !e.NoPicture {
			w, h, err := pictureSize(filepath.Join(images, e.File()+"-light.png"))
			if err != nil {
				return fmt.Errorf("catalog: %w; render the pictures first", err)
			}
			p.Width, p.Height = w/Scale, h/Scale
		}
		pages[i] = p
	}
	files := map[string]string{"README.md": readme(pages)}
	for _, lang := range []string{"ja", "en"} {
		files[lang+"/README.md"] = index(lang, pages)
		for _, c := range Categories() {
			files[lang+"/"+c.Slug+".md"] = categoryPage(lang, c, pages)
		}
	}
	data, err := siteJSON(pages)
	if err != nil {
		return err
	}
	files["catalog.json"] = data
	for name, content := range files {
		path := filepath.Join(opts.Out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// renderAll draws the pictures of entries into dir, a few at a time.
func renderAll(entries []Entry, dir string, only []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	want := map[string]bool{}
	for i := range entries {
		if !entries[i].NoPicture {
			want[entries[i].File()+"-light.png"] = true
			want[entries[i].File()+"-dark.png"] = true
		}
	}
	if len(only) == 0 {
		old, err := filepath.Glob(filepath.Join(dir, "*.png"))
		if err != nil {
			return err
		}
		for _, p := range old {
			if !want[filepath.Base(p)] {
				if err := os.Remove(p); err != nil {
					return err
				}
			}
		}
	}
	jobs := make(chan *Entry)
	errs := make(chan error, len(entries))
	var wg sync.WaitGroup
	for range max(runtime.NumCPU()/2, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range jobs {
				errs <- renderEntry(e, dir)
			}
		}()
	}
	for i := range entries {
		e := &entries[i]
		if e.NoPicture || len(only) > 0 && !contains(only, e) {
			continue
		}
		jobs <- e
	}
	close(jobs)
	wg.Wait()
	close(errs)
	var all []error
	for err := range errs {
		if err != nil {
			all = append(all, err)
		}
	}
	return errors.Join(all...)
}

// contains reports whether names has the name of e, or its type alone.
func contains(names []string, e *Entry) bool {
	for _, n := range names {
		if strings.EqualFold(n, e.Name) || strings.EqualFold(n, e.Type()) {
			return true
		}
	}
	return false
}

func renderEntry(e *Entry, dir string) error {
	shot, err := Render(e)
	if err != nil {
		return err
	}
	for _, p := range shot.files(e) {
		data, err := EncodePNG(p.img)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, p.name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// pictureSize returns the size of the PNG at path in physical pixels.
func pictureSize(path string) (w, h int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close() //nolint:errcheck // read only; nothing to flush
	c, err := png.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", path, err)
	}
	return c.Width, c.Height, nil
}
