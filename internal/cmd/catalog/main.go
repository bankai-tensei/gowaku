// Command catalog writes the widget catalog of docs/catalog from
// internal/catalog: the pictures, drawn on the CPU, the Markdown pages in
// Japanese and English, and the static site. Run it from the root of the
// module:
//
//	go run ./internal/cmd/catalog
//	go run ./internal/cmd/catalog -pictures=false
//	go run ./internal/cmd/catalog -only control.Button,control.Slider
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/internal/catalog"
)

func main() {
	out := flag.String("out", filepath.Join("docs", "catalog"), "the directory of the catalog")
	src := flag.String("src", filepath.Join("internal", "catalog"), "the source of the entries, where the examples are read")
	pictures := flag.Bool("pictures", true, "draw the pictures; false writes the pages from the pictures there are")
	only := flag.String("only", "", "draw only the pictures of these entries, separated by commas: names such as control.Button, or a bare type for every package that has it")
	flag.Parse()

	opts := catalog.Options{Out: *out, Source: *src, Render: *pictures}
	if *only != "" {
		opts.Only = strings.Split(*only, ",")
	}
	start := time.Now()
	if err := catalog.Generate(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("catalog: %d entries written to %s in %v\n", len(catalog.Entries()), *out, time.Since(start).Round(time.Millisecond))
}
