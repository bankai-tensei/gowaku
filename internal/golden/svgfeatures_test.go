//go:build svgsets

package golden

// This test draws the hand-written SVGs of testdata/svg/features, one or more
// per feature of ADR 0112 and 0113, with headless Chrome and with gowaku's
// soft backend, and compares the colors. It needs Chrome, so it runs only by
// hand:
//
//	GOWAKU_SVGSETS_OUT=<dir> go test -tags svgsets -run SVGFeatures -v ./internal/golden
//
// GOWAKU_SVGSETS_OUT, if set, receives a report and, for every file, Chrome,
// gowaku and their difference side by side. GOWAKU_CHROME names chrome.exe if
// it is not in the usual place. GOWAKU_SVGSETS_GPU, set to d3d11 or vulkan,
// draws with that backend instead of soft, which needs the gpu tag too:
//
//	GOWAKU_SVGSETS_GPU=d3d11 go test -tags "svgsets gpu" -run SVGFeatures -v ./internal/golden

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io/fs"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu/soft"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

const (
	featSize = 200 // the size a drawing is drawn at
	featCell = 220
	featCols = 6

	// A drawing passes when its colors differ from Chrome's by less than
	// featMaxMean on average, over the channels of every pixel of its cell,
	// and fewer than featMaxStrong of the pixels differ by a quarter or more
	// in some channel. Text is held to looser limits, since the fonts and
	// their rasterizers differ (see ADR 0113).
	featMaxMean       = 0.01
	featMaxStrong     = 0.005
	featMaxMeanText   = 0.03
	featMaxStrongText = 0.04
)

type featResult struct {
	name         string
	mean, strong float64
	pass         bool
	chrome, mine *image.NRGBA
}

func TestSVGFeatures(t *testing.T) {
	chrome := os.Getenv("GOWAKU_CHROME")
	if chrome == "" {
		chrome = `C:\Program Files\Google\Chrome\Application\chrome.exe`
	}
	if _, err := os.Stat(chrome); err != nil {
		t.Skip("Chrome not found; set GOWAKU_CHROME")
	}
	out := os.Getenv("GOWAKU_SVGSETS_OUT")
	names, err := fs.Glob(svgFiles, "testdata/svg/features/*.svg")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	work := t.TempDir()
	var files []string
	var drawings []*paint.SVG
	for _, name := range names {
		data, err := svgFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s, err := paint.ParseSVG(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		shaper, err := featShaper()
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetText(shaper); err != nil {
			t.Fatal(err)
		}
		if s.Skipped > 0 {
			t.Errorf("%s: %d skipped", name, s.Skipped)
		}
		f := filepath.Join(work, path.Base(name))
		if err := os.WriteFile(f, data, 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		drawings = append(drawings, s)
	}
	rows := (len(files) + featCols - 1) / featCols
	want, err := featChrome(chrome, work, files, rows)
	if err != nil {
		t.Fatal(err)
	}
	scene, err := featScene(drawings, rows)
	if err != nil {
		t.Fatal(err)
	}
	var got *image.NRGBA
	if backend := os.Getenv("GOWAKU_SVGSETS_GPU"); backend != "" {
		got, err = featGPU(backend, &scene)
	} else {
		got, err = featSoft(&scene)
	}
	if err != nil {
		t.Fatal(err)
	}
	var results []featResult
	for i, name := range names {
		cell := image.Rect(i%featCols*featCell, i/featCols*featCell, (i%featCols+1)*featCell, (i/featCols+1)*featCell)
		a, b := crop(want, cell), crop(got, cell)
		mean, strong := compareColors(a, b)
		base := strings.TrimSuffix(path.Base(name), ".svg")
		maxMean, maxStrong := featMaxMean, featMaxStrong
		if strings.HasPrefix(base, "text") {
			maxMean, maxStrong = featMaxMeanText, featMaxStrongText
		}
		results = append(results, featResult{name: base, mean: mean, strong: strong,
			pass: mean < maxMean && strong < maxStrong, chrome: a, mine: b})
	}
	var report strings.Builder
	pass := 0
	for _, r := range results {
		mark := "ok  "
		if r.pass {
			pass++
		} else {
			mark = "FAIL"
		}
		fmt.Fprintf(&report, "%s %-28s mean %.4f strong %.4f\n", mark, r.name, r.mean, r.strong)
	}
	fmt.Fprintf(&report, "%d of %d drawings pass\n", pass, len(results))
	t.Log("\n" + report.String())
	if out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "features.txt"), []byte(report.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, r := range results {
			if err := Save(filepath.Join(out, r.name+".png"), colorsSideBySide(r.chrome, r.mine)); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, r := range results {
		if !r.pass {
			t.Errorf("%s differs from Chrome: mean %.4f, strong %.4f", r.name, r.mean, r.strong)
		}
	}
}

// featChrome draws the files with Chrome on white.
func featChrome(chrome, work string, files []string, rows int) (*image.NRGBA, error) {
	var page strings.Builder
	fmt.Fprintf(&page, `<!doctype html><html><head><style>html,body{margin:0;background:#fff}`+
		`div{display:grid;grid-template-columns:repeat(%d,%dpx);grid-auto-rows:%dpx}`+
		`img{width:%dpx;height:%dpx;margin:%dpx;display:block}</style></head><body><div>`,
		featCols, featCell, featCell, featSize, featSize, (featCell-featSize)/2)
	for _, f := range files {
		u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(f)}
		fmt.Fprintf(&page, `<img src="%s">`, u.String())
	}
	page.WriteString(`</div></body></html>`)
	html := filepath.Join(work, "features.html")
	if err := os.WriteFile(html, []byte(page.String()), 0o644); err != nil {
		return nil, err
	}
	shot := filepath.Join(work, "features.png")
	cmd := exec.Command(chrome, "--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
		"--disable-extensions", "--force-device-scale-factor=1", "--allow-file-access-from-files",
		"--user-data-dir="+filepath.Join(work, "profile"),
		fmt.Sprintf("--window-size=%d,%d", featCols*featCell, rows*featCell),
		"--screenshot="+shot, "file:///"+filepath.ToSlash(html))
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("chrome: %w: %s", err, b)
	}
	return Load(shot)
}

// featGowaku draws the drawings with the soft backend on white.
// featShaper returns the shaper of the system fonts, made once.
func featShaper() (*text.Shaper, error) {
	if sharedShaper == nil {
		s, err := text.NewSystemShaper()
		if err != nil {
			return nil, err
		}
		sharedShaper = s
	}
	return sharedShaper, nil
}

var sharedShaper *text.Shaper

// featScene lays the drawings out on white.
func featScene(drawings []*paint.SVG, rows int) (Scene, error) {
	shaper, err := featShaper()
	if err != nil {
		return Scene{}, err
	}
	lines := make([]core.Widget, 0, rows)
	for y := 0; y < rows; y++ {
		cells := make([]core.Widget, 0, featCols)
		for x := 0; x < featCols && y*featCols+x < len(drawings); x++ {
			cells = append(cells, basic.Padding{
				Insets: geom.All((featCell - featSize) / 2),
				Child:  basic.SVG{Data: drawings[y*featCols+x], Size: featSize, Color: geom.Color{A: 1}},
			})
		}
		lines = append(lines, basic.Row{Children: cells})
	}
	s := Scene{
		Name: "svgfeatures", Width: featCols * featCell, Height: rows * featCell, Scale: 1,
		Root: func() core.Widget {
			return core.Inherited[*text.Shaper]{Value: shaper, Child: basic.Container{
				Color: geom.Color{R: 1, G: 1, B: 1, A: 1},
				Child: basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Column{Children: lines}},
			}}
		},
	}
	return s, nil
}

// featSoft draws s with the soft backend.
func featSoft(s *Scene) (*image.NRGBA, error) {
	device := soft.New()
	defer device.Destroy()
	r, err := NewRenderer(device)
	if err != nil {
		return nil, err
	}
	defer r.Destroy()
	surface, err := device.CreateSurface(nil, s.Width, s.Height, s.Scale)
	if err != nil {
		return nil, err
	}
	defer surface.Destroy()
	return r.Render(s, surface)
}

func crop(img *image.NRGBA, r image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Rect, img, r.Min, draw.Src)
	return out
}

// compareColors returns the mean difference over the channels of every pixel,
// from 0 to 1, and the share of pixels that differ by a quarter or more in a
// channel.
func compareColors(a, b *image.NRGBA) (mean, strong float64) {
	var sum float64
	n := 0
	for i := 0; i+3 < len(a.Pix); i += 4 {
		worst := 0.0
		for c := range 3 {
			d := math.Abs(float64(a.Pix[i+c])-float64(b.Pix[i+c])) / 255
			sum += d
			worst = math.Max(worst, d)
		}
		if worst >= 0.25 {
			n++
		}
	}
	pixels := float64(len(a.Pix) / 4)
	return sum / (3 * pixels), float64(n) / pixels
}

// colorsSideBySide puts Chrome, gowaku and their difference, enlarged eight
// times, next to each other.
func colorsSideBySide(a, b *image.NRGBA) *image.NRGBA {
	w, h := a.Rect.Dx(), a.Rect.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, 3*w+8, h))
	draw.Draw(out, out.Rect, image.NewUniform(color.NRGBA{0x80, 0x80, 0xff, 0xff}), image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(0, 0, w, h), a, image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(w+4, 0, 2*w+4, h), b, image.Point{}, draw.Src)
	for y := range h {
		for x := range w {
			i := a.PixOffset(x, y)
			d := 0
			for c := range 3 {
				d = max(d, int(math.Abs(float64(a.Pix[i+c])-float64(b.Pix[i+c]))))
			}
			v := uint8(255 - min(255, d*4))
			out.SetNRGBA(2*w+8+x, y, color.NRGBA{255, v, v, 255})
		}
	}
	return out
}
