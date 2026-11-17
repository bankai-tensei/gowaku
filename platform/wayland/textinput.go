package wayland

import (
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/input"
)

// textInputUpdate is what one done event of zwp_text_input_v3 changes, in the
// order the protocol applies it: the old composition goes, the commit goes in,
// and the new composition comes.
type textInputUpdate struct {
	// endComposition means that the composition shown so far ends first.
	endComposition bool
	commit         string
	// composition is the new composition, or nil if there is none.
	composition *input.Composition
}

// textInputDone works out a done event. composing says whether a composition
// is shown. preedit, begin and end are the last preedit_string, and commit the
// last commit_string, since the done before.
func textInputDone(composing bool, preedit string, begin, end int32, commit string) textInputUpdate {
	u := textInputUpdate{commit: commit}
	if preedit != "" {
		u.composition = compositionOf(preedit, begin, end)
	}
	// A composition that is replaced need not end first, unless text is
	// committed in between.
	u.endComposition = composing && (commit != "" || u.composition == nil)
	return u
}

// compositionOf turns a preedit string into a composition. The protocol has no
// clauses, only the cursor, which becomes a range when the input method selects
// a part, as Mozc does with the clause it converts. That part is the target.
func compositionOf(text string, begin, end int32) *input.Composition {
	c := &input.Composition{Text: text, Cursor: len(text)}
	b, e := boundary(text, begin), boundary(text, end)
	if begin < 0 || end < 0 {
		// Both -1 hide the cursor.
		c.Clauses = []input.Clause{{Start: 0, End: len(text), Kind: input.ClauseInput}}
		return c
	}
	if e < b {
		b, e = e, b
	}
	c.Cursor = b
	if b == e {
		c.Clauses = []input.Clause{{Start: 0, End: len(text), Kind: input.ClauseInput}}
		return c
	}
	if b > 0 {
		c.Clauses = append(c.Clauses, input.Clause{Start: 0, End: b, Kind: input.ClauseConverted})
	}
	c.Clauses = append(c.Clauses, input.Clause{Start: b, End: e, Kind: input.ClauseTarget})
	if e < len(text) {
		c.Clauses = append(c.Clauses, input.Clause{Start: e, End: len(text), Kind: input.ClauseConverted})
	}
	return c
}

// boundary clamps a byte offset from the input method into text, back to the
// start of a character.
func boundary(text string, offset int32) int {
	i := min(max(int(offset), 0), len(text))
	for i > 0 && i < len(text) && !utf8.RuneStart(text[i]) {
		i--
	}
	return i
}
