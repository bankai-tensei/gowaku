// Command gowaku-preview draws the previews of Go packages built with gowaku
// to PNG files, with a report of what each shows and what may be wrong with
// it: overflow, controls without a label, text of low contrast, and what
// changed since the last run. It is meant for people and for AI agents alike:
// everything it shows is also in files.
//
// Usage:
//
//	gowaku-preview [flags] [packages]
//
// A preview is a function of a _test.go file whose name begins with Preview;
// see package gowakupreview. With no flags the command draws once, prints the
// report and where the files are, and exits with status 1 if the build or a
// preview failed. -watch draws again whenever a .go file changes, and -http
// also serves a page that shows the pictures and updates itself:
//
//	go run github.com/bankai-tensei/gowaku/cmd/gowaku-preview -http localhost:7878 .
//
// The command writes nothing into the packages: it adds the test that draws
// the previews through go test -overlay.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/gowakupreview"
)

//go:embed page.html
var page embed.FS

func main() {
	out := flag.String("out", filepath.Join(os.TempDir(), "gowaku-preview"), "directory for the pictures and reports")
	run := flag.String("run", "", "draw only the previews whose name matches this regular expression")
	at := flag.String("at", "", "also draw the frames at these times, such as 0,70ms,150ms")
	watch := flag.Bool("watch", false, "draw again whenever a .go file of the packages changes")
	addr := flag.String("http", "", "serve a page of the pictures at this address, such as localhost:7878 (implies -watch)")
	liveName := flag.String("live", "", "open the preview of this name in a window instead of drawing")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}
	absOut, err := filepath.Abs(*out)
	if err != nil {
		fail(err)
	}
	p := &previewer{out: absOut, run: *run, at: *at, args: args}
	if *liveName != "" {
		if err := p.live(*liveName); err != nil {
			fail(err)
		}
		return
	}
	if *addr == "" && !*watch {
		ok := p.draw()
		fmt.Print(p.summary())
		if !ok {
			os.Exit(1)
		}
		return
	}
	if *addr != "" {
		go p.serve(*addr)
	}
	p.loop()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gowaku-preview:", err)
	os.Exit(1)
}

// previewer draws the previews of some packages and keeps the last result.
type previewer struct {
	out, run, at string
	args         []string

	mu       sync.Mutex
	version  int
	building bool
	ok       bool
	output   string // of go test, when it failed
	reports  []served
	// dirs are the directories of the packages, listed at each drawing, so
	// that watching only looks at files.
	dirs []string
}

// served is a report with the URL of its directory.
type served struct {
	gowakupreview.Report
	Base string `json:"base"`
}

// pkg is a package with previews in its test files.
type pkg struct {
	importPath, dir string
	groups          []group
}

// group is the previews of one package clause: the package itself, or its
// external test package with the _test suffix.
type group struct {
	name     string // the package clause
	previews []string
	theme    bool // whether PreviewTheme exists
}

func (g group) importPath(p pkg) string {
	if strings.HasSuffix(g.name, "_test") {
		return p.importPath + "_test"
	}
	return p.importPath
}

// draw runs the previews of every package once and records the result.
func (p *previewer) draw() bool {
	p.mu.Lock()
	p.building = true
	p.mu.Unlock()
	ok, output, reports := p.drawAll()
	dirs := listDirs(p.args)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.building, p.ok, p.output, p.reports, p.dirs = false, ok, output, reports, dirs
	p.version++
	return ok
}

func (p *previewer) drawAll() (bool, string, []served) {
	pkgs, err := findPreviews(p.args)
	if err != nil {
		return false, err.Error(), nil
	}
	if len(pkgs) == 0 {
		return false, "no previews found: write a func PreviewName() gowaku.Widget in a _test.go file", nil
	}
	ok := true
	var log strings.Builder
	var reports []served
	for _, pk := range pkgs {
		res, err := goTest(pk, p.out, map[string]string{
			gowakupreview.EnvOut: p.out, gowakupreview.EnvFilter: p.run, gowakupreview.EnvAt: p.at,
		}, false)
		if err != nil {
			ok = false
			fmt.Fprintf(&log, "%s:\n%s\n", pk.importPath, res)
		}
		for _, g := range pk.groups {
			path := g.importPath(pk)
			data, err := os.ReadFile(filepath.Join(p.out, gowakupreview.DirOf(path), "report.json"))
			if err != nil {
				continue
			}
			var r gowakupreview.Report
			if json.Unmarshal(data, &r) == nil {
				if len(r.Errors) > 0 {
					ok = false
				}
				reports = append(reports, served{Report: r, Base: "/files/" + gowakupreview.DirOf(path) + "/"})
			}
		}
	}
	return ok, log.String(), reports
}

// summary is the report of the last run, for the terminal.
func (p *previewer) summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var b strings.Builder
	if p.output != "" {
		b.WriteString(p.output)
	}
	for _, r := range p.reports {
		dir := filepath.Join(p.out, gowakupreview.DirOf(r.Package))
		data, err := os.ReadFile(filepath.Join(dir, "report.txt"))
		if err == nil {
			b.Write(data)
		}
		fmt.Fprintf(&b, "  in %s\n", dir)
	}
	return b.String()
}

// findPreviews lists the packages of args that have previews.
func findPreviews(args []string) ([]pkg, error) {
	cmd := exec.Command("go", append([]string{"list", "-f", "{{.ImportPath}}\t{{.Dir}}"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	listed, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w\n%s", err, stderr.String())
	}
	var pkgs []pkg
	for _, line := range strings.Split(strings.TrimSpace(string(listed)), "\n") {
		path, dir, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		groups, err := scan(dir)
		if err != nil {
			return nil, err
		}
		if len(groups) > 0 {
			pkgs = append(pkgs, pkg{importPath: path, dir: dir, groups: groups})
		}
	}
	return pkgs, nil
}

// scan finds the previews in the test files of dir.
func scan(dir string) ([]group, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	byName := map[string]*group{}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), "zz_gowaku_preview_") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !isPreview(fn) {
				continue
			}
			g := byName[file.Name.Name]
			if g == nil {
				g = &group{name: file.Name.Name}
				byName[file.Name.Name] = g
			}
			if fn.Name.Name == "PreviewTheme" {
				g.theme = true
			} else {
				g.previews = append(g.previews, fn.Name.Name)
			}
		}
	}
	var groups []group
	for _, g := range byName {
		if len(g.previews) > 0 {
			sort.Strings(g.previews)
			groups = append(groups, *g)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })
	return groups, nil
}

// isPreview reports whether fn is PreviewXxx() with one result, named as go
// test names tests: Preview, then nothing or a character that is not lower case.
func isPreview(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	rest, ok := strings.CutPrefix(name, "Preview")
	if !ok {
		return false
	}
	if r, _ := utf8.DecodeRuneInString(rest); rest != "" && unicode.IsLower(r) {
		return false
	}
	return fn.Type.Params.NumFields() == 0 && fn.Type.Results.NumFields() == 1 && fn.Type.TypeParams == nil
}

// source is the test that the command adds to a group.
func source(p pkg, g group) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by gowaku-preview. DO NOT EDIT.\n\npackage %s\n\n", g.name)
	b.WriteString("import (\n\t\"testing\"\n\n\tgowakupreview \"github.com/bankai-tensei/gowaku/gowakupreview\"\n)\n\n")
	fmt.Fprintf(&b, "func TestGowakuPreview_%s(t *testing.T) {\n", g.name)
	themeArg := "nil"
	if g.theme {
		themeArg = "PreviewTheme"
	}
	fmt.Fprintf(&b, "\tgowakupreview.Run(t, %q, %s, []gowakupreview.Preview{\n", g.importPath(p), themeArg)
	for _, name := range g.previews {
		fmt.Fprintf(&b, "\t\t{Name: %q, Func: func() any { return %s() }},\n", strings.TrimPrefix(name, "Preview"), name)
	}
	b.WriteString("\t})\n}\n")
	return b.String()
}

// goTest runs the generated tests of a package with env added, and returns the
// output of go test.
func goTest(p pkg, out string, env map[string]string, live bool) (string, error) {
	tmp, err := os.MkdirTemp("", "gowaku-preview-overlay")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // a leftover temporary directory does no harm
	replace := map[string]string{}
	for _, g := range p.groups {
		gen := filepath.Join(tmp, "zz_gowaku_preview_"+g.name+"_test.go")
		if err := os.WriteFile(gen, []byte(source(p, g)), 0o644); err != nil {
			return "", err
		}
		replace[filepath.Join(p.dir, "zz_gowaku_preview_"+g.name+"_test.go")] = gen
	}
	overlay := filepath.Join(tmp, "overlay.json")
	data, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(overlay, data, 0o644); err != nil {
		return "", err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	args := []string{"test", "-count=1", "-run", "^TestGowakuPreview_", "-overlay", overlay}
	if live {
		args = append(args, "-timeout=0")
	}
	cmd := exec.Command("go", append(args, p.importPath)...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	res, err := cmd.CombinedOutput()
	return string(res), err
}

// live opens one preview in a window and waits until it closes.
func (p *previewer) live(name string) error {
	name = strings.TrimPrefix(name, "Preview")
	pkgs, err := findPreviews(p.args)
	if err != nil {
		return err
	}
	for _, pk := range pkgs {
		for _, g := range pk.groups {
			for _, n := range g.previews {
				if strings.TrimPrefix(n, "Preview") != name {
					continue
				}
				only := pk
				only.groups = []group{g}
				res, err := goTest(only, p.out, map[string]string{gowakupreview.EnvLive: name}, true)
				if err != nil {
					return fmt.Errorf("%w\n%s", err, res)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("no preview named %q", name)
}

// loop draws whenever a .go file of the packages changes, until killed.
func (p *previewer) loop() {
	last := ""
	for {
		stamp := p.stamp()
		if stamp != last {
			// Wait for the editor to finish writing.
			time.Sleep(150 * time.Millisecond)
			last = p.stamp()
			start := time.Now()
			p.draw()
			fmt.Printf("%s drawn in %v\n%s", time.Now().Format("15:04:05"), time.Since(start).Round(10*time.Millisecond), p.summary())
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// stamp sums up the names, sizes and times of the .go files and go.mod of the
// packages, to see a change. It lists the packages only before the first
// drawing; each drawing lists them again.
func (p *previewer) stamp() string {
	p.mu.Lock()
	dirs := p.dirs
	p.mu.Unlock()
	if dirs == nil {
		dirs = listDirs(p.args)
	}
	var b strings.Builder
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go")) //nolint:errcheck // the pattern is fixed and valid
		files = append(files, filepath.Join(dir, "go.mod"))
		for _, f := range files {
			if st, err := os.Stat(f); err == nil {
				fmt.Fprintf(&b, "%s %d %d\n", f, st.Size(), st.ModTime().UnixNano())
			}
		}
	}
	return b.String()
}

// listDirs returns the directories of the packages of args, or nil.
func listDirs(args []string) []string {
	cmd := exec.Command("go", append([]string{"list", "-f", "{{.Dir}}"}, args...)...)
	listed, err := cmd.Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(listed))
}

// serve shows the page, the pictures and the state.
func (p *previewer) serve(addr string) {
	mux := http.NewServeMux()
	sub, err := fs.Sub(page, ".")
	if err != nil {
		fail(err)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "page.html")
	})
	mux.Handle("GET /files/", http.StripPrefix("/files/", noCache(http.FileServer(http.Dir(p.out)))))
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, _ *http.Request) {
		p.mu.Lock()
		state := map[string]any{"version": p.version, "building": p.building, "ok": p.ok, "output": p.output, "reports": p.reports, "out": p.out}
		data, err := json.Marshal(state)
		p.mu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data) //nolint:errcheck // a client that went away needs no answer
	})
	mux.HandleFunc("POST /api/live", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		go func() {
			if err := p.live(name); err != nil {
				fmt.Fprintln(os.Stderr, "gowaku-preview:", err)
			}
		}()
		w.WriteHeader(http.StatusAccepted)
	})
	fmt.Printf("serving http://%s/\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
