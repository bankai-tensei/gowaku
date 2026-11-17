package catalog

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const (
	beginMarker = "// catalog:begin"
	endMarker   = "// catalog:end"
)

// Examples reads the scene functions in the Go files of dir, the source of
// this package, and returns the example of each, by function name.
func Examples(dir string) (map[string]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(fset, p, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "scene") {
				continue
			}
			from := fset.Position(fn.Body.Lbrace).Offset + 1
			to := fset.Position(fn.Body.Rbrace).Offset
			ex, err := example(string(src[from:to]))
			if err != nil {
				return nil, fmt.Errorf("catalog: %s: %w", fn.Name.Name, err)
			}
			out[fn.Name.Name] = ex
		}
	}
	return out, nil
}

// example turns the body of a scene function into the example of the page:
// the lines between the markers if there are any, without the indent they
// share, and without the return of a body that is one return statement.
func example(body string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	begin, end := -1, -1
	for i, l := range lines {
		switch strings.TrimSpace(l) {
		case beginMarker:
			begin = i
		case endMarker:
			end = i
		}
	}
	switch {
	case begin >= 0 && end > begin:
		lines = lines[begin+1 : end]
	case begin >= 0 || end >= 0:
		return "", fmt.Errorf("markers out of order")
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("empty example")
	}
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, "\t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		if len(l) >= indent {
			lines[i] = l[indent:]
		} else {
			lines[i] = ""
		}
	}
	if strings.HasPrefix(lines[0], "return ") && !hasStatementAfter(lines) {
		lines[0] = strings.TrimPrefix(lines[0], "return ")
	}
	return strings.Join(lines, "\n"), nil
}

// hasStatementAfter reports whether a line after the first one starts at the
// indent of the first, that is, whether the return is not the only statement.
func hasStatementAfter(lines []string) bool {
	for _, l := range lines[1:] {
		if l != "" && l[0] != '\t' && l[0] != '}' && l[0] != ')' {
			return true
		}
	}
	return false
}
