package text

import (
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/shaping"
)

// maxVariations is how many axes a Style can set. The array keeps Style
// comparable.
const maxVariations = 4

// Variation sets an axis of a variable font, such as "wght" or "wdth", to a
// value in the units of the axis.
type Variation struct {
	// Axis is the four-letter tag of the axis. An axis the font does not have
	// is ignored.
	Axis  string
	Value float32
}

// variantKey identifies a variable font at one point of its axes.
type variantKey struct {
	font *font.Font
	vars [maxVariations + 2]font.Variation
}

// variant is a face set to a point of its axes, with a HarfBuzz shaper of its
// own: HarfBuzz caches one font per font file, whatever the face's axes.
type variant struct {
	face     *font.Face
	harfbuzz *shaping.HarfbuzzShaper
}

// variationsOf returns the axes st asks for. Weight drives "wght" and the size
// drives "opsz", as CSS does with font-optical-sizing: auto; Variations come
// last and win.
func variationsOf(st *Style) (vars [maxVariations + 2]font.Variation, n int) {
	vars[0] = font.Variation{Tag: ot.NewTag('w', 'g', 'h', 't'), Value: float32(st.Weight)}
	vars[1] = font.Variation{Tag: ot.NewTag('o', 'p', 's', 'z'), Value: st.Size}
	n = 2
	for _, v := range st.Variations {
		if len(v.Axis) != 4 {
			continue
		}
		vars[n] = font.Variation{Tag: ot.NewTag(v.Axis[0], v.Axis[1], v.Axis[2], v.Axis[3]), Value: v.Value}
		n++
	}
	return vars, n
}

// varied returns the face and shaper for face under the axes of st: face itself
// and the common shaper if the font is not variable.
func (s *Shaper) varied(face *font.Face, st *Style) (*font.Face, *shaping.HarfbuzzShaper) {
	if face == nil || s.static[face.Font] {
		return face, &s.harfbuzz
	}
	vars, n := variationsOf(st)
	key := variantKey{font: face.Font, vars: vars}
	if v, ok := s.variants[key]; ok {
		return v.face, v.harfbuzz
	}
	f := font.NewFace(face.Font)
	f.SetVariations(vars[:n])
	if len(f.Coords()) == 0 {
		// Not a variable font: remember, so that the next run costs a lookup.
		s.static[face.Font] = true
		return face, &s.harfbuzz
	}
	v := variant{face: f, harfbuzz: &shaping.HarfbuzzShaper{}}
	s.variants[key] = v
	return v.face, v.harfbuzz
}
