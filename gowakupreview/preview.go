// Package gowakupreview draws the previews of a package to PNG files, with a
// report of what each one shows and what may be wrong with it. The command
// gowaku-preview finds the previews and runs this package through go test;
// applications only write the previews.
//
// A preview is a function of a _test.go file whose name begins with Preview,
// with no parameters, returning a widget or an Item:
//
//	func PreviewAlbumTile() gowaku.Widget {
//		return basic.SizedBox{Width: 180, Child: albumTile(demoAlbum)}
//	}
//
//	func PreviewSongs() gowakupreview.Item {
//		return gowakupreview.Item{Widget: songs(demoList), Width: 900, Height: 600}
//	}
//
// Being in a test file of the package, a preview reaches its unexported
// functions and stays out of the application's binary. A function named
// PreviewTheme returning a theme.Set gives the theme of every preview of
// the package.
//
// Each preview is drawn light and dark, settled, and checked with
// gowakutest.Tester.Warnings. The files go to the directory the command
// chooses: one PNG per variant, a .diff.png where the picture changed since
// the last run, and report.json and report.txt for the whole package.
//
// A package with its own TestMain must call m.Run when Active reports true,
// and may skip its own setup then.
package gowakupreview

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// The environment through which the command talks to Run.
const (
	EnvOut    = "GOWAKU_PREVIEW_OUT"
	EnvFilter = "GOWAKU_PREVIEW_RUN"
	EnvAt     = "GOWAKU_PREVIEW_AT"
	EnvLive   = "GOWAKU_PREVIEW_LIVE"
)

// fitWidth and fitHeight are the window of a preview that leaves its size to
// its content, which is then cut out.
const fitWidth, fitHeight = 1600, 1200

// Themes picks the halves of the theme a preview is drawn in.
type Themes uint8

// The choices of Themes. The zero value draws both.
const (
	LightAndDark Themes = iota
	LightOnly
	DarkOnly
)

// Item is a preview with its settings. The zero value of each field is the
// default.
type Item struct {
	Widget core.Widget
	// Width and Height are the window in logical pixels. Zero leaves that
	// length to the content: the widget gets up to 1600 by 1200 and the
	// picture is cut to its size, which suits a widget that shrinks to its
	// content but not one that fills its parent.
	Width, Height float32
	// Widths draws the preview once for each width, for layouts that adapt.
	// It replaces Width.
	Widths []float32
	// Scale is the number of pixels per logical pixel. Zero selects 1.
	Scale float32
	// Themes picks light, dark or both.
	Themes Themes
	// Theme is the look. The zero value takes PreviewTheme of the package, or
	// the default theme.
	Theme theme.Set
	// At draws the frames at these times after the start as well as the
	// settled one, to see an animation part way. The command's -at flag adds
	// to them.
	At []time.Duration
	// Fonts are the fonts of the text. Nil uses those of the system.
	Fonts [][]byte
}

// Preview is a preview function found by the command.
type Preview struct {
	Name string
	Func func() any
}

// DirOf returns the directory below the command's output directory that holds
// the pictures and report of the package with import path pkg.
func DirOf(pkg string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_", ".", "_").Replace(pkg)
}

// Active reports whether the test binary runs for the command, for a TestMain
// that must step aside.
func Active() bool { return os.Getenv(EnvOut) != "" || os.Getenv(EnvLive) != "" }

// Image is one picture of a report.
type Image struct {
	Preview string `json:"preview"`
	// File is the name of the PNG in the directory of the report.
	File   string  `json:"file"`
	Theme  string  `json:"theme"`
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
	// At is the time of the frame, or empty for the settled one.
	At string `json:"at,omitempty"`
	// Changed says whether the picture differs from the last run, whose
	// difference is then in Diff.
	Changed  bool                 `json:"changed"`
	Diff     string               `json:"diff,omitempty"`
	Warnings []gowakutest.Warning `json:"warnings"`
	// Semantics is the accessibility tree, one node per line.
	Semantics string `json:"semantics"`
}

// Report is what Run found in a package.
type Report struct {
	Package string    `json:"package"`
	Time    time.Time `json:"time"`
	Images  []Image   `json:"images"`
	// Errors are previews that panicked or returned nothing.
	Errors []string `json:"errors,omitempty"`
}

// Run draws previews, or opens one in a window, as the command asks through
// the environment. The generated test of the command calls it; outside the
// command it skips.
func Run(t *testing.T, pkg string, themeOf func() theme.Set, previews []Preview) {
	if name := os.Getenv(EnvLive); name != "" {
		live(t, name, themeOf, previews)
		return
	}
	base := os.Getenv(EnvOut)
	if base == "" {
		t.Skip("gowakupreview: run through the gowaku-preview command")
	}
	out := filepath.Join(base, DirOf(pkg))
	var filter *regexp.Regexp
	if f := os.Getenv(EnvFilter); f != "" {
		var err error
		if filter, err = regexp.Compile(f); err != nil {
			t.Fatalf("gowakupreview: %s: %v", EnvFilter, err)
		}
	}
	extra, err := parseTimes(os.Getenv(EnvAt))
	if err != nil {
		t.Fatalf("gowakupreview: %s: %v", EnvAt, err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	report := Report{Package: pkg, Time: time.Now(), Images: []Image{}}
	for _, p := range previews {
		if filter != nil && !filter.MatchString(p.Name) {
			continue
		}
		item, err := itemOf(p)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", p.Name, err))
			continue
		}
		if item.Theme == (theme.Set{}) && themeOf != nil {
			item.Theme = themeOf()
		}
		item.At = append(item.At, extra...)
		images, err := draw(t, out, p.Name, item)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", p.Name, err))
		}
		report.Images = append(report.Images, images...)
		// Written after each preview, so that the report keeps those before one
		// that panics. The panic itself fails the test with its stack.
		if err := writeReport(out, report); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeReport(out, report); err != nil {
		t.Fatal(err)
	}
}

// itemOf calls a preview.
func itemOf(p Preview) (item Item, err error) {
	switch v := p.Func().(type) {
	case Item:
		item = v
	case core.Widget:
		item = Item{Widget: v}
	default:
		return Item{}, fmt.Errorf("returned %T, want a widget or a gowakupreview.Item", v)
	}
	if item.Widget == nil {
		return Item{}, fmt.Errorf("returned no widget")
	}
	return item, nil
}

// draw renders every variant of one preview.
func draw(t *testing.T, out, name string, item Item) ([]Image, error) {
	var images []Image
	widths := item.Widths
	if len(widths) == 0 {
		widths = []float32{item.Width}
	}
	var darks []bool
	switch item.Themes {
	case LightOnly:
		darks = []bool{false}
	case DarkOnly:
		darks = []bool{true}
	default:
		darks = []bool{false, true}
	}
	times := append([]time.Duration{-1}, item.At...)
	for _, w := range widths {
		for _, dark := range darks {
			for _, at := range times {
				v := variant{width: w, dark: dark, at: at, manyWidths: len(widths) > 1}
				img, err := drawOne(t, out, name, item, v)
				if err != nil {
					return images, err
				}
				images = append(images, img)
			}
		}
	}
	return images, nil
}

// variant is one picture of a preview. A negative at is the settled frame.
type variant struct {
	width      float32
	dark       bool
	at         time.Duration
	manyWidths bool
}

func (v variant) file(name string) string {
	if v.manyWidths {
		name += fmt.Sprintf("@w%g", v.width)
	}
	name += "@" + v.theme()
	if v.at >= 0 {
		name += "@" + v.at.String()
	}
	return name
}

func (v variant) theme() string {
	if v.dark {
		return "dark"
	}
	return "light"
}

// drawOne renders one variant.
func drawOne(t *testing.T, out, name string, item Item, v variant) (Image, error) {
	scale := item.Scale
	if scale <= 0 {
		scale = 1
	}
	fitW, fitH := v.width <= 0, item.Height <= 0
	winW, winH := v.width, item.Height
	if fitW {
		winW = fitWidth
	}
	if fitH {
		winH = fitHeight
	}
	var size geom.Size
	root := core.Widget(measure{out: &size, child: item.Widget})
	if fitW || fitH {
		root = basic.Align{Child: root}
	}
	tt := gowakutest.New(t, root, gowakutest.Options{Width: winW, Height: winH, Scale: scale, Theme: item.Theme, Dark: v.dark, Fonts: item.Fonts})
	if v.at >= 0 {
		tt.Advance(v.at)
	} else {
		tt.Settle()
	}
	shot := tt.Screenshot()
	cutW, cutH := winW, winH
	if fitW {
		cutW = max(size.W, 1)
	}
	if fitH {
		cutH = max(size.H, 1)
	}
	if fitW || fitH {
		r := image.Rect(0, 0, int(math.Ceil(float64(cutW*scale))), int(math.Ceil(float64(cutH*scale)))).Intersect(shot.Rect)
		shot = shot.SubImage(r).(*image.NRGBA)
	}
	file := v.file(name)
	img := Image{Preview: name, File: file + ".png", Theme: v.theme(), Width: cutW, Height: cutH, Semantics: tt.Dump(), Warnings: tt.Warnings()}
	if v.at >= 0 {
		img.At = v.at.String()
	}
	if img.Warnings == nil {
		img.Warnings = []gowakutest.Warning{}
	}
	changed, diff, err := compare(filepath.Join(out, img.File), shot)
	if err != nil {
		return img, err
	}
	img.Changed = changed
	diffPath := filepath.Join(out, file+".diff.png")
	if diff != nil {
		img.Diff = file + ".diff.png"
		if err := writePNG(diffPath, diff); err != nil {
			return img, err
		}
	} else if err := os.Remove(diffPath); err != nil && !os.IsNotExist(err) {
		return img, err
	}
	return img, writePNG(filepath.Join(out, img.File), shot)
}

// compare reads the picture of the last run at path and reports whether shot
// differs from it, with a picture of the difference: the new picture faded,
// with the changed pixels in red. A first run counts as unchanged.
func compare(path string, shot *image.NRGBA) (bool, *image.NRGBA, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	old, err := png.Decode(f)
	_ = f.Close() //nolint:errcheck // read only; a decoding error below counts as a change
	if err != nil {
		return true, nil, nil
	}
	b, ob := shot.Bounds(), old.Bounds()
	diff := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	changed := ob.Dx() != b.Dx() || ob.Dy() != b.Dy()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := shot.NRGBAAt(b.Min.X+x, b.Min.Y+y)
			if x < ob.Dx() && y < ob.Dy() {
				o := color.NRGBAModel.Convert(old.At(ob.Min.X+x, ob.Min.Y+y)).(color.NRGBA)
				if c == o {
					g := uint8((int(c.R) + int(c.G) + int(c.B)) / 3)
					g = 255 - (255-g)/4
					diff.SetNRGBA(x, y, color.NRGBA{g, g, g, 255})
					continue
				}
			}
			changed = true
			diff.SetNRGBA(x, y, color.NRGBA{230, 40, 60, 255})
		}
	}
	if !changed {
		return false, nil, nil
	}
	return true, diff, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close() //nolint:errcheck // the encoding error is the one to report
		return err
	}
	return f.Close()
}

// writeReport writes report.json, and report.txt for reading at a glance.
func writeReport(out string, r Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "report.json"), data, 0o644); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d pictures\n", r.Package, len(r.Images))
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "ERROR %s\n", e)
	}
	for i := range r.Images {
		img := &r.Images[i]
		mark := " "
		if img.Changed {
			mark = "*"
		}
		fmt.Fprintf(&b, "%s %s  %gx%g\n", mark, img.File, img.Width, img.Height)
		for _, w := range img.Warnings {
			fmt.Fprintf(&b, "    %s\n", w)
		}
	}
	return os.WriteFile(filepath.Join(out, "report.txt"), []byte(b.String()), 0o644)
}

// parseTimes reads a comma-separated list of durations.
func parseTimes(s string) ([]time.Duration, error) {
	var out []time.Duration
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		d, err := time.ParseDuration(f)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// live opens the preview called name in a real window, until it is closed.
func live(t *testing.T, name string, themeOf func() theme.Set, previews []Preview) {
	for _, p := range previews {
		if p.Name != name {
			continue
		}
		item, err := itemOf(p)
		if err != nil {
			t.Fatal(err)
		}
		if item.Theme == (theme.Set{}) && themeOf != nil {
			item.Theme = themeOf()
		}
		w, h := item.Width, item.Height
		if len(item.Widths) > 0 {
			w = item.Widths[0]
		}
		home := item.Widget
		if w <= 0 || h <= 0 {
			home = basic.Center{Child: home}
		}
		if w <= 0 {
			w = 800
		}
		if h <= 0 {
			h = 600
		}
		if err := gowaku.Run(gowaku.App{Title: "Preview: " + name, Width: w, Height: h, Theme: item.Theme, Home: home}); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("gowakupreview: no preview named %q", name)
}

// measure records the size its child gets at each layout.
type measure struct {
	out   *geom.Size
	child core.Widget
}

// Key implements core.Widget.
func (measure) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (m measure) CreateRenderObject() core.RenderObject { return &measureObject{out: m.out} }

// UpdateRenderObject implements core.RenderWidget.
func (m measure) UpdateRenderObject(ro core.RenderObject) { ro.(*measureObject).out = m.out }

// ChildWidget implements core.SingleChildWidget.
func (m measure) ChildWidget() core.Widget { return m.child }

type measureObject struct {
	layout.RenderProxy
	out *geom.Size
}

// Layout implements core.RenderObject.
func (r *measureObject) Layout(c geom.Constraints) geom.Size {
	sz := r.RenderProxy.Layout(c)
	*r.out = sz
	return sz
}
