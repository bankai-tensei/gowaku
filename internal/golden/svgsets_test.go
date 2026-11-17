//go:build svgsets

package golden

// This test reads every icon of whole icon sets and compares how gowaku draws
// them with headless Chrome. It needs the sets on disk and Chrome, so it runs
// only by hand:
//
//	GOWAKU_SVGSETS="<dir>;<dir>" GOWAKU_SVGSETS_OUT=<dir> go test -tags svgsets -run SVGSets -v -timeout 2h ./internal/golden
//
// GOWAKU_SVGSETS lists directories of .svg files, apart by the list separator
// of the OS. GOWAKU_SVGSETS_OUT, if set, receives a report and pictures of the
// worst icons. GOWAKU_CHROME names chrome.exe if it is not in the usual place.
// See docs/ADR/0045-shaders-svg-animated-images.md.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu/soft"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

const (
	setIcon  = 48 // the size an icon is drawn at
	setCell  = 64 // the cell around it, which holds strokes that stick out
	setCols  = 20
	setRows  = 20
	setBatch = setCols * setRows

	// An icon passes when the coverage it draws differs from Chrome's by
	// less than setMaxDiff of its ink, and at most setMaxStrong pixels differ
	// by half or more. Antialiasing moves coverage a little along every edge;
	// a missing or extra piece makes a clump of pixels that differ fully. See
	// the ADR for the numbers behind the thresholds.
	setMaxDiff   = 0.15
	setMaxStrong = 4
)

type setIconFile struct {
	path string
	svg  *paint.SVG
}

type setResult struct {
	path      string
	diff, iou float64
	strong    int
	chrome    *image.Gray
	gowaku    *image.Gray
}

func TestSVGSets(t *testing.T) {
	dirs := filepath.SplitList(os.Getenv("GOWAKU_SVGSETS"))
	if len(dirs) == 0 {
		t.Skip("set GOWAKU_SVGSETS to directories of icons")
	}
	chrome := os.Getenv("GOWAKU_CHROME")
	if chrome == "" {
		chrome = `C:\Program Files\Google\Chrome\Application\chrome.exe`
	}
	out := os.Getenv("GOWAKU_SVGSETS_OUT")
	var icons []setIconFile
	failed, skipped := 0, 0
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.svg"))
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(files)
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			s, err := paint.ParseSVG(data)
			if err != nil {
				failed++
				t.Errorf("%s: %v", f, err)
				continue
			}
			if s.Skipped > 0 {
				skipped++
				t.Errorf("%s: %d skipped", f, s.Skipped)
			}
			icons = append(icons, setIconFile{path: f, svg: s})
		}
	}
	t.Logf("parsed %d icons: %d errors, %d with skipped parts", len(icons), failed, skipped)

	device := soft.New()
	t.Cleanup(device.Destroy)
	r, err := NewRenderer(device)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Destroy)
	work := t.TempDir()
	var results []setResult
	for start := 0; start < len(icons); start += setBatch {
		batch := icons[start:min(start+setBatch, len(icons))]
		want, err := chromeGrid(chrome, work, batch, start/setBatch)
		if err != nil {
			t.Fatal(err)
		}
		got, err := gowakuGrid(r, device, batch)
		if err != nil {
			t.Fatal(err)
		}
		for i, ic := range batch {
			cell := image.Rect(i%setCols*setCell, i/setCols*setCell, (i%setCols+1)*setCell, (i/setCols+1)*setCell)
			a, b := coverage(want, cell), coverage(got, cell)
			diff, iou, strong := compareCoverage(a, b)
			results = append(results, setResult{path: ic.path, diff: diff, iou: iou, strong: strong, chrome: a, gowaku: b})
		}
		t.Logf("batch %d: %d icons", start/setBatch, len(batch))
	}
	pass := 0
	for _, res := range results {
		if res.diff < setMaxDiff && res.strong <= setMaxStrong {
			pass++
		}
	}
	slices.SortFunc(results, func(a, b setResult) int {
		return -cmpFloat(a.diff+float64(a.strong)/100, b.diff+float64(b.strong)/100)
	})
	var report strings.Builder
	fmt.Fprintf(&report, "%d of %d icons pass (diff < %v of ink, at most %d pixels off by half)\n", pass, len(results), setMaxDiff, setMaxStrong)
	diffs := make([]float64, len(results))
	for i, res := range results {
		diffs[i] = res.diff
	}
	slices.Sort(diffs)
	if n := len(diffs); n > 0 {
		fmt.Fprintf(&report, "diff median %.4f, p90 %.4f, p99 %.4f, max %.4f\n", diffs[n/2], diffs[n*9/10], diffs[n*99/100], diffs[n-1])
	}
	for i, res := range results[:min(60, len(results))] {
		fmt.Fprintf(&report, "%3d diff %.4f strong %3d iou %.4f %s\n", i, res.diff, res.strong, res.iou,
			filepath.Base(filepath.Dir(res.path))+"/"+filepath.Base(res.path))
	}
	t.Log("\n" + report.String())
	if out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "report.txt"), []byte(report.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		for i, res := range results[:min(40, len(results))] {
			name := fmt.Sprintf("%02d-%s.png", i, strings.TrimSuffix(filepath.Base(res.path), ".svg"))
			if err := Save(filepath.Join(out, name), sideBySide(res.chrome, res.gowaku)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if pass < len(results) {
		t.Errorf("%d icons differ from Chrome", len(results)-pass)
	}
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// chromeGrid draws the icons with Chrome, in black on white.
func chromeGrid(chrome, work string, batch []setIconFile, n int) (*image.NRGBA, error) {
	var page strings.Builder
	fmt.Fprintf(&page, `<!doctype html><html><head><style>html,body{margin:0;background:#fff}`+
		`div{display:grid;grid-template-columns:repeat(%d,%dpx);grid-auto-rows:%dpx}`+
		`img{width:%dpx;height:%dpx;margin:%dpx;display:block}</style></head><body><div>`,
		setCols, setCell, setCell, setIcon, setIcon, (setCell-setIcon)/2)
	for _, ic := range batch {
		abs, err := filepath.Abs(ic.path)
		if err != nil {
			return nil, err
		}
		u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(abs)}
		fmt.Fprintf(&page, `<img src="%s">`, u.String())
	}
	page.WriteString(`</div></body></html>`)
	html := filepath.Join(work, fmt.Sprintf("page%d.html", n))
	if err := os.WriteFile(html, []byte(page.String()), 0o644); err != nil {
		return nil, err
	}
	shot := filepath.Join(work, fmt.Sprintf("page%d.png", n))
	cmd := exec.Command(chrome, "--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
		"--disable-extensions", "--force-device-scale-factor=1", "--allow-file-access-from-files",
		"--user-data-dir="+filepath.Join(work, "profile"),
		fmt.Sprintf("--window-size=%d,%d", setCols*setCell, setRows*setCell),
		"--screenshot="+shot, "file:///"+filepath.ToSlash(html))
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("chrome: %w: %s", err, b)
	}
	return Load(shot)
}

// gowakuGrid draws the icons with the soft backend, in black on white.
func gowakuGrid(r *Renderer, device *soft.Device, batch []setIconFile) (*image.NRGBA, error) {
	rows := make([]core.Widget, 0, setRows)
	for y := 0; y*setCols < len(batch); y++ {
		cells := make([]core.Widget, 0, setCols)
		for x := 0; x < setCols && y*setCols+x < len(batch); x++ {
			cells = append(cells, basic.Padding{
				Insets: geom.All((setCell - setIcon) / 2),
				Child:  basic.SVG{Data: batch[y*setCols+x].svg, Size: setIcon, Color: geom.Color{A: 1}},
			})
		}
		rows = append(rows, basic.Row{Children: cells})
	}
	s := Scene{
		Name: "svgsets", Width: setCols * setCell, Height: setRows * setCell, Scale: 1,
		Root: func() core.Widget {
			return basic.Container{Color: geom.Color{R: 1, G: 1, B: 1, A: 1}, Child: basic.Align{
				Alignment: layout.AlignTopLeft, Child: basic.Column{Children: rows},
			}}
		},
	}
	surface, err := device.CreateSurface(nil, s.Width, s.Height, s.Scale)
	if err != nil {
		return nil, err
	}
	defer surface.Destroy()
	return r.Render(&s, surface)
}

// coverage returns how much ink each pixel of cell has, from black on white.
func coverage(img *image.NRGBA, cell image.Rectangle) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, cell.Dx(), cell.Dy()))
	for y := range cell.Dy() {
		for x := range cell.Dx() {
			c := img.NRGBAAt(cell.Min.X+x, cell.Min.Y+y)
			lum := (int(c.R) + int(c.G) + int(c.B)) / 3
			g.SetGray(x, y, color.Gray{Y: uint8(255 - lum)})
		}
	}
	return g
}

// compareCoverage returns the summed difference of coverage as a share of the
// ink of both, and the IoU of the pixels more than half covered, and how many pixels differ by
// half or more.
func compareCoverage(a, b *image.Gray) (diff, iou float64, strong int) {
	var sum, ink float64
	var both, either int
	for i := range a.Pix {
		x, y := float64(a.Pix[i])/255, float64(b.Pix[i])/255
		sum += math.Abs(x - y)
		ink += math.Max(x, y)
		if math.Abs(x-y) >= 0.5 {
			strong++
		}
		ha, hb := a.Pix[i] >= 128, b.Pix[i] >= 128
		if ha && hb {
			both++
		}
		if ha || hb {
			either++
		}
	}
	if ink == 0 {
		return 0, 1, strong
	}
	if either == 0 {
		return sum / ink, 1, strong
	}
	return sum / ink, float64(both) / float64(either), strong
}

// sideBySide puts Chrome, gowaku and their difference next to each other,
// enlarged four times.
func sideBySide(a, b *image.Gray) *image.NRGBA {
	const zoom = 4
	w, h := a.Rect.Dx(), a.Rect.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, 3*w*zoom+2*zoom, h*zoom))
	draw.Draw(out, out.Rect, image.NewUniform(color.NRGBA{0x80, 0x80, 0xff, 0xff}), image.Point{}, draw.Src)
	for y := range h {
		for x := range w {
			ca, cb := a.GrayAt(x, y).Y, b.GrayAt(x, y).Y
			cols := [3]color.NRGBA{
				{255 - ca, 255 - ca, 255 - ca, 255},
				{255 - cb, 255 - cb, 255 - cb, 255},
				diffColor(ca, cb),
			}
			for k, c := range cols {
				for dy := range zoom {
					for dx := range zoom {
						out.SetNRGBA(k*(w*zoom+zoom)+x*zoom+dx, y*zoom+dy, c)
					}
				}
			}
		}
	}
	return out
}

// diffColor shows where gowaku has more ink in red and less in blue.
func diffColor(chrome, gowaku uint8) color.NRGBA {
	d := int(gowaku) - int(chrome)
	switch {
	case d > 0:
		return color.NRGBA{255, uint8(255 - d), uint8(255 - d), 255}
	case d < 0:
		return color.NRGBA{uint8(255 + d), uint8(255 + d), 255, 255}
	}
	return color.NRGBA{255, 255, 255, 255}
}
