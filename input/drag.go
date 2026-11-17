package input

// DataKinds is a set of the kinds of data that a drag carries.
type DataKinds uint8

// The kinds of dragged data. See docs/ADR/0050-drag-and-drop.md.
const (
	DataText DataKinds = 1 << iota
	// DataFiles is a list of files, as a file manager drags them.
	DataFiles
)

// DragData is what a drag carries: text, a list of files, or both. A zero
// field is a kind that is not there.
type DragData struct {
	Text string
	// Files are absolute paths.
	Files []string
}

// Kinds returns the kinds that d holds.
func (d *DragData) Kinds() DataKinds {
	var k DataKinds
	if d.Text != "" {
		k |= DataText
	}
	if len(d.Files) > 0 {
		k |= DataFiles
	}
	return k
}

// DropEffect is what a drop does to the data: a set of them is what the source
// allows. The zero value is none: the drop is refused.
type DropEffect uint8

// The drop effects.
const (
	DropNone DropEffect = 0
	DropCopy DropEffect = 1 << (iota - 1)
	DropMove
	DropLink
)

// Pick returns the first of the effects in want, in its order of bits, that
// allowed has, or DropNone.
func (allowed DropEffect) Pick(want DropEffect) DropEffect {
	for _, e := range []DropEffect{DropCopy, DropMove, DropLink} {
		if want&e != 0 && allowed&e != 0 {
			return e
		}
	}
	return DropNone
}
