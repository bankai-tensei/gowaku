package field

import (
	"sort"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text"
)

// An obscured Input shows one bullet for each grapheme cluster of its text.
// The layout is of the bullets; offsets of the display, the text with the
// composition, map to offsets of the bullets and back, as for ruby. Moves by
// word go to the ends, so that the bullets do not tell where the spaces are.
// See Input.Obscure and docs/ADR/0088-input-obscure.md.

// obscureBullet is what an obscured field draws for each character.
const obscureBullet = "•"

// obscureView is the projection of an obscured field onto its bullets.
type obscureView struct {
	// bounds are the offsets of the display where its clusters start, then
	// its length: bullet k stands for the display from bounds[k] to
	// bounds[k+1].
	bounds []int
	// bullets is a run of bullets at least as long as any shown, so that the
	// text shaped is a part of it rather than a new string per edit.
	bullets string
}

// update follows display and returns the bullets to shape for it.
func (v *obscureView) update(shaper *text.Shaper, display string) string {
	v.bounds = shaper.GraphemeBounds(display, v.bounds[:0])
	return v.marks(len(v.bounds) - 1)
}

// marks returns n bullets.
func (v *obscureView) marks(n int) string {
	if size := n * len(obscureBullet); len(v.bullets) < size {
		v.bullets = strings.Repeat(obscureBullet, max(n, 2*len(v.bullets)/len(obscureBullet), 16))
	}
	return v.bullets[:n*len(obscureBullet)]
}

// toShown returns the offset in the bullets of display offset d. An offset
// inside a cluster goes to its start.
func (v *obscureView) toShown(d int) int {
	return clusterAt(v.bounds, d) * len(obscureBullet)
}

// fromShown returns the display offset of bullet offset s.
func (v *obscureView) fromShown(s int) int {
	if len(v.bounds) == 0 {
		return 0
	}
	return v.bounds[min(max(s/len(obscureBullet), 0), len(v.bounds)-1)]
}

// end returns the length of the display.
func (v *obscureView) end() int {
	if len(v.bounds) == 0 {
		return 0
	}
	return v.bounds[len(v.bounds)-1]
}

// clusterAt returns the cluster of bounds that offset is in, or the number
// of clusters at the end.
func clusterAt(bounds []int, offset int) int {
	return max(sort.SearchInts(bounds, offset+1)-1, 0)
}

// secureHost returns the Host as a core.SecureInputHost, or nil.
func (r *RenderEditable) secureHost() core.SecureInputHost {
	h, _ := r.Host().(core.SecureInputHost)
	return h
}

// focusIME tells the host about a field that gains or loses the keyboard. An
// obscured field keeps the IME off, and asks for secure input while it has
// the keyboard, so that a Japanese IME never composes a password.
func (r *RenderEditable) focusIME(focused bool) {
	secure := r.secureHost()
	if focused && r.field.Obscure && secure != nil {
		secure.SetSecureInput(true)
	}
	r.Host().SetIMEEnabled(focused && !r.field.Obscure && r.editable())
	if !focused && r.field.Obscure && secure != nil {
		secure.SetSecureInput(false)
	}
}

// obscureChanged follows Obscure turning on or off, as a button that shows
// the password does. It runs during Build, so it reports nothing.
func (r *RenderEditable) obscureChanged() {
	// What was typed before may not come back through undo in the other
	// mode, nor stay in memory longer than needed.
	r.history.Clear()
	if r.Composition != nil {
		r.Composition = nil
		r.Host().CancelComposition()
	}
	r.ruby.fresh = false
	r.invalidate()
	if !r.focused {
		return
	}
	r.imeRect = geom.Rect{}
	secure := r.secureHost()
	if r.field.Obscure {
		if secure != nil {
			secure.SetSecureInput(true)
		}
		r.Host().SetIMEEnabled(false)
		return
	}
	r.Host().SetIMEEnabled(r.editable())
	if secure != nil {
		secure.SetSecureInput(false)
	}
}

// obscuredText returns what a screen reader gets of an obscured field: a
// bullet for each cluster of the text, and the clusters that the anchor and
// the caret are at, which are UTF-16 offsets of the bullets too.
func (r *RenderEditable) obscuredText() (bullets string, anchor, caret int) {
	bounds := r.shaper.GraphemeBounds(r.Content, nil)
	return r.obscure.marks(len(bounds) - 1), clusterAt(bounds, r.Anchor), clusterAt(bounds, r.Caret)
}

// obscuredOffset returns the byte offset of the text at cluster k, from a
// screen reader that counts the bullets.
func (r *RenderEditable) obscuredOffset(k int) int {
	bounds := r.shaper.GraphemeBounds(r.Content, nil)
	return bounds[min(max(k, 0), len(bounds)-1)]
}
