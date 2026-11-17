package text

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gobidi "github.com/go-text/typesetting/bidi"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/fontscan"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/segmenter"
	"github.com/go-text/typesetting/shaping"

	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text/jlreq"
)

// defaultFamilies are the Japanese families in order of preference: those of
// Windows, Linux and macOS. "Yu Gothic UI" is left out on purpose: its kana and
// punctuation are proportional, which leaves nothing for JLReq to work with.
// See docs/ADR/0007-text-architecture.md.
var defaultFamilies = [...]string{"Yu Gothic", "Noto Sans CJK JP", "Hiragino Sans", "Meiryo", fontscan.SansSerif}

// discard drops the diagnostics of fontscan, which would otherwise reach the
// log on the hot path.
type discard struct{}

// Printf implements fontscan.Logger.
func (discard) Printf(string, ...any) {}

// faceQuery is the part of a Style that selects fonts.
type faceQuery struct {
	family string
	weight uint16
	italic bool
	emoji  bool
}

// Shaper finds fonts and shapes text. Create one per process and share it
// among windows: the atlas and every Paragraph refer to it.
//
// gowaku.Run provides its Shaper above the root of every window. A widget of
// its own finds it with core.DependOn[*text.Shaper](ctx) in Build. A
// layout.Painter, as control.CustomPaint takes, has no BuildContext: the
// widget that makes the painter looks the Shaper up, lays the text out there
// (keep the Layout in its State while the text stays the same), and hands the
// painter the Layout, which it draws with paint.Canvas.DrawText. For labels
// the painter picks while it paints, it hands over a Labels.
type Shaper struct {
	fonts    *fontscan.FontMap
	language language.Language
	noKern   []shaping.FontFeature
	query    faceQuery
	queried  bool

	split    shaping.Segmenter
	harfbuzz shaping.HarfbuzzShaper
	breaks   segmenter.Segmenter
	breaker  jlreq.Breaker
	bidi     gobidi.Paragraph

	faces   []*font.Face
	faceIDs map[*font.Face]paint.FaceID
	// slanted marks, by FaceID, the faces that the atlas slants: synthetic
	// obliques of faces that have no italic. obliqueIDs maps a face to the ID
	// its italic text takes, which is its own ID if the face is italic itself.
	// See docs/ADR/0101-synthetic-italic.md.
	slanted    []bool
	obliqueIDs map[*font.Face]paint.FaceID
	// variants are variable fonts set to the axes of a style, and static the
	// fonts known to have no axes.
	variants map[variantKey]variant
	static   map[*font.Font]bool

	// startupPath is where SaveStartupFaces keeps its list of fonts, empty for
	// a Shaper of given fonts, and startupFaces the list as last read or
	// written.
	startupPath  string
	startupFaces []startupFace
	// adoptedFaces are the fonts that NewSystemShaper read ahead.
	adoptedFaces []*font.Face
}

// NewSystemShaper returns a Shaper that uses the fonts installed on the system.
// The index of the fonts is cached in the user cache directory.
func NewSystemShaper() (*Shaper, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("text: find cache directory: %w", err)
	}
	cache := filepath.Join(dir, "gowaku")
	return newSystemShaper(cache, filepath.Join(cache, startupFile()))
}

// newSystemShaper is NewSystemShaper with the index of the system fonts in
// cache and the list of SaveStartupFaces at startupPath.
func newSystemShaper(cache, startupPath string) (*Shaper, error) {
	s := newShaper()
	s.startupPath = startupPath
	s.startupFaces = readStartupFaces(s.startupPath)
	// Reading the index checks every font file, and reading a large Japanese
	// font takes as long, so the fonts that the last first frame used load
	// meanwhile.
	preloaded := preload(s.startupFaces)
	if err := s.fonts.UseSystemFonts(cache); err != nil {
		preloaded()
		return nil, fmt.Errorf("text: scan system fonts: %w", err)
	}
	s.adopt(preloaded(), cache)
	return s, nil
}

// NewShaper returns a Shaper that uses exactly the given font files, each the
// content of a TrueType or OpenType font or collection.
func NewShaper(fonts ...[]byte) (*Shaper, error) {
	if len(fonts) == 0 {
		return nil, errors.New("text: NewShaper needs at least one font")
	}
	s := newShaper()
	for i, data := range fonts {
		if err := s.fonts.AddFont(bytes.NewReader(data), fmt.Sprintf("font-%d", i), ""); err != nil {
			return nil, fmt.Errorf("text: load font %d: %w", i, err)
		}
	}
	return s, nil
}

func newShaper() *Shaper {
	return &Shaper{
		fonts:    fontscan.NewFontMap(discard{}),
		language: language.NewLanguage("ja"),
		// Japanese fonts kern kana against each other, which would pull glyphs
		// off the em box that the composition rules are based on.
		noKern:   []shaping.FontFeature{{Tag: ot.NewTag('k', 'e', 'r', 'n'), Value: 0}},
		faceIDs:  make(map[*font.Face]paint.FaceID),
		variants: make(map[variantKey]variant),
		static:   make(map[*font.Font]bool),
	}
}

// selectFaces makes the font map answer for st. Changing the query makes the
// font map rebuild its candidates, so it only happens when st differs.
func (s *Shaper) selectFaces(st *Style) { s.selectFacesFor(st, false) }

// selectFacesFor is selectFaces, with the color emoji fonts first if emoji is
// set.
func (s *Shaper) selectFacesFor(st *Style, emoji bool) {
	q := faceQuery{st.Family, st.Weight, st.Italic, emoji}
	if s.queried && q == s.query {
		return
	}
	s.query, s.queried = q, true
	families := make([]string, 0, len(defaultFamilies)+len(emojiFamilies)+1)
	if emoji {
		families = append(families, emojiFamilies[:]...)
	}
	for f := range strings.SplitSeq(st.Family, ",") {
		if f = strings.TrimSpace(f); f != "" {
			families = append(families, f)
		}
	}
	families = append(families, defaultFamilies[:]...)
	aspect := font.Aspect{Weight: font.Weight(st.Weight), Style: font.StyleNormal}
	if st.Italic {
		aspect.Style = font.StyleItalic
	}
	s.fonts.SetQuery(fontscan.Query{Families: families, Aspect: aspect})
}

// faceID returns the ID of f, handing out a new one on first sight.
func (s *Shaper) faceID(f *font.Face) paint.FaceID {
	id, ok := s.faceIDs[f]
	if !ok {
		id = paint.FaceID(len(s.faces))
		s.faces = append(s.faces, f)
		s.slanted = append(s.slanted, false)
		s.faceIDs[f] = id
	}
	return id
}

// italicFaceID returns the ID that italic text in f takes: that of f if it is
// an italic or oblique face, or else that of a synthetic oblique of f, which
// the atlas slants by obliqueShear. Fonts without an italic, as Japanese fonts
// are, then still show emphasis.
func (s *Shaper) italicFaceID(f *font.Face) paint.FaceID {
	id, ok := s.obliqueIDs[f]
	if ok {
		return id
	}
	if f.Describe().Aspect.Style == font.StyleItalic {
		// fontscan reports oblique faces as italic too.
		id = s.faceID(f)
	} else {
		id = paint.FaceID(len(s.faces))
		s.faces = append(s.faces, f)
		s.slanted = append(s.slanted, true)
	}
	if s.obliqueIDs == nil {
		s.obliqueIDs = make(map[*font.Face]paint.FaceID)
	}
	s.obliqueIDs[f] = id
	return id
}

// lineMetrics returns the ascent and descent of the primary font of st in
// logical pixels, and how far the center of its em box lies above the
// baseline, in ems. Lines use these whatever the fallback fonts say, so that the
// baselines stay on a fixed grid.
func (s *Shaper) lineMetrics(st *Style) (ascent, descent, emCenter float32) {
	face := s.fonts.ResolveFace('あ')
	emCenter = defaultEmCenter
	if gid, ok := face.NominalGlyph('あ'); ok && face.HasVerticalMetrics() {
		// The vertical origin is the top of the em box.
		_, top := face.GlyphVOrigin(gid)
		emCenter = top/float32(face.Upem()) - 0.5
	}
	if extents, ok := face.FontHExtents(); ok {
		unit := st.Size / float32(face.Upem())
		return extents.Ascender * unit, -extents.Descender * unit, emCenter
	}
	return st.Size * 0.88, st.Size * 0.12, emCenter
}
