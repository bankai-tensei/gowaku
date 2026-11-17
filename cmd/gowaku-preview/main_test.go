package main

import (
	"encoding/json"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/gowakupreview"
)

const samplePath = "github.com/bankai-tensei/gowaku/cmd/gowaku-preview/testdata/sample"

func TestScan_FindsPreviewsAndTheTheme(t *testing.T) {
	groups, err := scan(filepath.Join("testdata", "sample"))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups", len(groups))
	}
	g := groups[0]
	if g.name != "sample" || !g.theme || strings.Join(g.previews, ",") != "PreviewBadge,PreviewTooWide" {
		t.Errorf("got %+v", g)
	}
}

func TestSource_IsGoCode(t *testing.T) {
	p := pkg{importPath: samplePath, groups: []group{{name: "sample", previews: []string{"PreviewBadge"}, theme: true}}}
	src := source(p, p.groups[0])
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	for _, want := range []string{`Name: "Badge"`, "PreviewTheme, []gowakupreview.Preview", "package sample"} {
		if !strings.Contains(src, want) {
			t.Errorf("no %q in\n%s", want, src)
		}
	}
}

// TestDraw_EndToEnd runs go test on the sample package, as the command does.
func TestDraw_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs another test binary")
	}
	out := t.TempDir()
	p := &previewer{out: out, args: []string{"./testdata/sample"}}
	if !p.draw() {
		// The overflow of TooWide is a warning, not a failure.
		t.Fatalf("draw failed:\n%s", p.summary())
	}
	dir := filepath.Join(out, gowakupreview.DirOf(samplePath))
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r gowakupreview.Report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	files := map[string]gowakupreview.Image{}
	for _, img := range r.Images {
		files[img.File] = img
		if _, err := os.Stat(filepath.Join(dir, img.File)); err != nil {
			t.Errorf("no picture: %v", err)
		}
	}
	if len(r.Images) != 3 {
		t.Errorf("got %d pictures, want Badge light and dark and TooWide light", len(r.Images))
	}
	if w := files["TooWide@light.png"].Warnings; len(w) != 1 || w[0].Kind != "overflow" {
		t.Errorf("TooWide warnings %v, want one overflow", w)
	}
	if b := files["Badge@light.png"]; b.Width < 20 || b.Width > 120 || len(b.Warnings) != 0 {
		t.Errorf("Badge %+v: want a picture cut to the badge, without warnings", b)
	}
	if !strings.Contains(files["Badge@light.png"].Semantics, "New") {
		t.Errorf("semantics %q lack the label", files["Badge@light.png"].Semantics)
	}
	// Nothing written into the package.
	if left, err := filepath.Glob(filepath.Join("testdata", "sample", "zz_*")); err != nil || len(left) != 0 {
		t.Errorf("left %v in the package", left)
	}
	// A second run sees no change.
	if !p.draw() {
		t.Fatal(p.summary())
	}
	if data, err = os.ReadFile(filepath.Join(dir, "report.json")); err != nil {
		t.Fatal(err)
	}
	var again gowakupreview.Report
	if err := json.Unmarshal(data, &again); err != nil {
		t.Fatal(err)
	}
	for _, img := range again.Images {
		if img.Changed {
			t.Errorf("%s changed between two runs of the same code", img.File)
		}
	}
}
