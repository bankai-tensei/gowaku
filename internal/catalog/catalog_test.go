package catalog

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/internal/golden"
)

// exclusions are the exported widgets of the packages under widgets that the
// catalog leaves out, by package-qualified name, with the reason. A widget
// that stops being exported loses its line here and nothing else.
var exclusions = map[string]string{
	"basic.ScrollFrame": split,
	"basic.Viewport":    split,
	"overlay.PopupList": split,
}

// split is the reason for the building blocks that one widget package
// exports only for the packages above it.
const split = "a building block shared between the widget packages"

// widgetTypes returns the exported types of the packages under widgets, by
// package-qualified name such as "control.Button", and those of them that are
// widgets: the types with a Key method of their own or that embed
// core.StatelessBase or core.StatefulBase. pkgs are the packages it read.
func widgetTypes(t *testing.T) (all, widgets, pkgs map[string]bool) {
	t.Helper()
	fset := token.NewFileSet()
	paths, err := filepath.Glob(filepath.Join("..", "..", "widgets", "*", "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no widgets source: %v", err)
	}
	all, widgets, pkgs = map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range paths {
		dir := filepath.Base(filepath.Dir(p))
		if strings.HasSuffix(p, "_test.go") || dir == "internal" {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		if f.Name.Name != dir {
			t.Fatalf("%s: package %s in directory %s", p, f.Name.Name, dir)
		}
		pkgs[dir] = true
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil && d.Name.Name == "Key" {
					if name := receiverName(d.Recv.List[0].Type); ast.IsExported(name) {
						widgets[dir+"."+name] = true
					}
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok || !ts.Name.IsExported() {
						continue
					}
					all[dir+"."+ts.Name.Name] = true
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, fl := range st.Fields.List {
						if se, ok := fl.Type.(*ast.SelectorExpr); ok && len(fl.Names) == 0 &&
							(se.Sel.Name == "StatelessBase" || se.Sel.Name == "StatefulBase") {
							widgets[dir+"."+ts.Name.Name] = true
						}
					}
				}
			}
		}
	}
	return all, widgets, pkgs
}

func receiverName(e ast.Expr) string {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	switch x := e.(type) {
	case *ast.IndexExpr:
		e = x.X
	case *ast.IndexListExpr:
		e = x.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func TestEntries_CoverEveryWidget(t *testing.T) {
	all, widgets, _ := widgetTypes(t)
	shown := map[string]string{}
	for _, e := range Entries() {
		for _, n := range append([]string{e.Name}, e.Also...) {
			if !all[n] {
				t.Errorf("%s: %s is not an exported type of a package under widgets", e.Name, n)
			}
			if prev, ok := shown[n]; ok {
				t.Errorf("%s is in both %s and %s", n, prev, e.Name)
			}
			shown[n] = e.Name
		}
	}
	var missing []string
	for n := range widgets {
		if _, ok := shown[n]; !ok && exclusions[n] == "" {
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("widgets without an entry or an exclusion: %s", strings.Join(missing, ", "))
	}
	for n := range exclusions {
		if _, ok := shown[n]; ok {
			t.Errorf("%s has an entry and an exclusion", n)
		}
		if !widgets[n] {
			t.Errorf("exclusion %s is no widget of a package under widgets", n)
		}
	}
	t.Logf("%d widgets: %d with an entry or in one, %d excluded", len(widgets), len(widgets)-len(exclusions), len(exclusions))
}

func TestPackages_MatchTheWidgetPackages(t *testing.T) {
	_, _, pkgs := widgetTypes(t)
	listed := map[string]bool{}
	for _, p := range Packages() {
		if !pkgs[p.Name] {
			t.Errorf("package %s is not under widgets", p.Name)
		}
		if p.JA == "" || p.EN == "" {
			t.Errorf("package %s: needs JA and EN", p.Name)
		}
		listed[p.Name] = true
	}
	for p := range pkgs {
		if !listed[p] {
			t.Errorf("widgets/%s is missing from Packages", p)
		}
	}
	for _, e := range Entries() {
		for _, n := range append([]string{e.Name}, e.Also...) {
			if pkg, _ := splitName(n); !listed[pkg] {
				t.Errorf("%s: %s names no widget package", e.Name, n)
			}
		}
	}
}

func TestEntries_AreWellFormed(t *testing.T) {
	examples, err := Examples(".")
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string]bool{}
	for _, c := range Categories() {
		slugs[c.Slug] = true
	}
	names := map[string]bool{}
	for _, e := range Entries() {
		if names[e.Name] {
			t.Errorf("%s twice", e.Name)
		}
		names[e.Name] = true
		if !slugs[e.Category] {
			t.Errorf("%s: no category %q", e.Name, e.Category)
		}
		if e.JA == "" || e.EN == "" || e.Scene == nil {
			t.Errorf("%s: needs JA, EN and Scene", e.Name)
		}
		if examples[e.Func()] == "" {
			t.Errorf("%s: no function %s in the source", e.Name, e.Func())
		}
	}
	used := map[string]bool{}
	for _, e := range Entries() {
		used[e.Func()] = true
	}
	for name := range examples {
		if !used[name] {
			t.Errorf("%s is the scene of no entry", name)
		}
	}
}

func TestExample(t *testing.T) {
	body := "\n\tx := 1\n\t// catalog:begin\n\treturn basic.Row{Children: []core.Widget{\n\t\ta,\n\t}}\n\t// catalog:end\n"
	got, err := example(body)
	if err != nil {
		t.Fatal(err)
	}
	if want := "basic.Row{Children: []core.Widget{\n\ta,\n}}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	got, err = example("\n\tx := 1\n\treturn x\n")
	if err != nil || got != "x := 1\nreturn x" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := example("\n\t// catalog:end\n\tx\n\t// catalog:begin\n"); err == nil {
		t.Error("markers out of order must fail")
	}
}

// TestGenerate_DocsAreUpToDate writes the pages again, with the sizes of the
// pictures in docs/catalog/img, and compares them with the committed ones. Run
// go run ./internal/cmd/catalog after changing an entry.
func TestGenerate_DocsAreUpToDate(t *testing.T) {
	out := filepath.Join("..", "..", "docs", "catalog")
	tmp := t.TempDir()
	if err := Generate(Options{Out: tmp, Source: ".", Images: filepath.Join(out, "img")}); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(tmp, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(tmp, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		want, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Errorf("%s: %v; run go run ./internal/cmd/catalog", rel, err)
			return nil
		}
		if string(got) != string(want) {
			t.Errorf("docs/catalog/%s is out of date; run go run ./internal/cmd/catalog", filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRender_PicturesAreUpToDate draws every entry again and compares it with
// its committed pictures, with the tolerance of internal/golden, so that a
// change of a widget that the catalog shows asks for new pictures. It also
// runs the scenes without pictures.
func TestRender_PicturesAreUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("draws every scene on the CPU")
	}
	dir := filepath.Join("..", "..", "docs", "catalog", "img")
	for _, e := range Entries() {
		t.Run(e.Name, func(t *testing.T) {
			t.Parallel()
			if e.NoPicture {
				if e.Scene() == nil {
					t.Error("the scene builds nothing")
				}
				return
			}
			shot, err := Render(&e)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range shot.files(&e) {
				path := filepath.Join(dir, p.name)
				want, err := golden.Load(path)
				if err != nil {
					t.Fatalf("%v; run go run ./internal/cmd/catalog", err)
				}
				d, err := golden.Compare(p.img, want)
				if err != nil || d.Failed() {
					t.Errorf("%s: %v %v; run go run ./internal/cmd/catalog", p.name, d, err)
				}
			}
		})
	}
}

func TestCategories_HaveEntries(t *testing.T) {
	for _, c := range Categories() {
		if !slices.ContainsFunc(Entries(), func(e Entry) bool { return e.Category == c.Slug }) {
			t.Errorf("category %s has no entry", c.Slug)
		}
	}
}
