package render

import (
	"bytes"
	"image"
	"math"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// rampRows is the number of gradients of more than gpu.MaxStops stops that the
// ramp texture holds at once. Past it, rows that the current frame does not use
// make way, as glyphs do in the atlas.
const rampRows = 256

// hashStops returns an FNV-1a hash of the bits of stops. The bits hash as
// integers, where floats would go through the slower float hash of a map.
func hashStops(stops []paint.GradientStop) uint64 {
	h := uint64(14695981039346656037)
	mix := func(v float32) {
		h ^= uint64(math.Float32bits(v))
		h *= 1099511628211
	}
	for i := range stops {
		s := &stops[i]
		mix(s.Offset)
		mix(s.Color.R)
		mix(s.Color.G)
		mix(s.Color.B)
		mix(s.Color.A)
	}
	return h ^ uint64(len(stops))
}

func sameStops(a, b []paint.GradientStop) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// rampTable keeps the ramps of the gradients with many stops, one per row of a
// texture, and reuses a row for every gradient of the same stops.
type rampTable struct {
	texture gpu.Texture
	pix     []byte
	// rows finds a row by the hash of its stops. stops holds the stops of each
	// row, which settle a collision. A row that holds a table of
	// feComponentTransfer has no stops, and its texels in tables instead.
	rows     map[uint64]int
	hashes   []uint64
	stops    [][]paint.GradientStop
	tables   [][]byte
	lastUsed []uint64
	// lastRow is the row handed out last, which the next gradient is likely
	// to share.
	lastRow int
	// colors and offsets are scratch for FillRamp.
	colors  []geom.Color
	offsets []float32
	// dirty is the band of rows to upload.
	dirtyLow, dirtyHigh int
}

func newRampTable() *rampTable {
	return &rampTable{
		pix:    make([]byte, rampRows*gpu.RampWidth*4),
		rows:   make(map[uint64]int, rampRows),
		hashes: make([]uint64, 0, rampRows),
		stops:  make([][]paint.GradientStop, 0, rampRows),
		tables: make([][]byte, 0, rampRows),
		// lastUsed has a slot per row from the start, so that taking a row
		// never grows it.
		lastUsed: make([]uint64, rampRows),
		lastRow:  -1,
	}
}

// row returns the row that holds stops, filling one if none does. It reports
// false if every row is in use by the current frame.
func (r *rampTable) row(stops []paint.GradientStop, frame uint64) (int, bool) {
	if i := r.lastRow; i >= 0 && r.tables[i] == nil && sameStops(r.stops[i], stops) {
		r.lastUsed[i] = frame
		return i, true
	}
	hash := hashStops(stops)
	if i, ok := r.rows[hash]; ok && r.tables[i] == nil && sameStops(r.stops[i], stops) {
		r.lastUsed[i] = frame
		r.lastRow = i
		return i, true
	}
	i, ok := r.take(hash, frame)
	if !ok {
		return 0, false
	}
	r.stops[i] = append(r.stops[i][:0], stops...)
	r.colors, r.offsets = r.colors[:0], r.offsets[:0]
	for _, s := range stops {
		r.colors = append(r.colors, s.Color)
		r.offsets = append(r.offsets, s.Offset)
	}
	const stride = gpu.RampWidth * 4
	gpu.FillRamp(r.pix[i*stride:(i+1)*stride], r.colors, r.offsets)
	r.touch(i)
	return i, true
}

// table returns the row that holds texels, a table of feComponentTransfer of
// gpu.RampWidth RGBA texels, filling one if none does. It reports false if
// every row is in use by the current frame.
func (r *rampTable) table(texels *[gpu.RampWidth * 4]byte, frame uint64) (int, bool) {
	if i := r.lastRow; i >= 0 && bytes.Equal(r.tables[i], texels[:]) {
		r.lastUsed[i] = frame
		return i, true
	}
	hash := uint64(14695981039346656037)
	for _, v := range texels {
		hash ^= uint64(v)
		hash *= 1099511628211
	}
	if i, ok := r.rows[hash]; ok && bytes.Equal(r.tables[i], texels[:]) {
		r.lastUsed[i] = frame
		r.lastRow = i
		return i, true
	}
	i, ok := r.take(hash, frame)
	if !ok {
		return 0, false
	}
	r.stops[i] = r.stops[i][:0]
	// A new table is a cache miss, which may allocate.
	r.tables[i] = append([]byte(nil), texels[:]...)
	const stride = gpu.RampWidth * 4
	copy(r.pix[i*stride:(i+1)*stride], texels[:])
	r.touch(i)
	return i, true
}

// take hands out a row for a new hash: a new one while there is room, or else
// the one that went unused the longest, if not in this frame. The row keeps its
// old stops or table for the caller to replace.
func (r *rampTable) take(hash, frame uint64) (int, bool) {
	i := len(r.hashes)
	if i < rampRows {
		r.hashes = append(r.hashes, hash)
		r.stops = append(r.stops, nil)
		r.tables = append(r.tables, nil)
	} else {
		i = -1
		for k, used := range r.lastUsed {
			if used != frame && (i < 0 || used < r.lastUsed[i]) {
				i = k
			}
		}
		if i < 0 {
			return 0, false
		}
		if r.rows[r.hashes[i]] == i {
			delete(r.rows, r.hashes[i])
		}
		r.hashes[i] = hash
		// The row starts as one of a gradient, without a table.
		r.tables[i] = nil
	}
	// A collision leaves the other row out of the map; it still serves
	// through lastRow, and otherwise takes a new row when drawn again.
	r.rows[hash] = i
	r.lastUsed[i] = frame
	r.lastRow = i
	return i, true
}

// touch marks row i for upload.
func (r *rampTable) touch(i int) {
	if r.dirtyLow == r.dirtyHigh {
		r.dirtyLow, r.dirtyHigh = i, i+1
	} else {
		r.dirtyLow, r.dirtyHigh = min(r.dirtyLow, i), max(r.dirtyHigh, i+1)
	}
}

// upload creates the texture on first use and sends the rows that changed.
func (r *rampTable) upload(store TextureStore) error {
	if r.texture == nil {
		t, err := store.CreateTexture(gpu.RampWidth, rampRows, gpu.TextureRGBA8)
		if err != nil {
			return err
		}
		r.texture = t
	}
	if r.dirtyLow == r.dirtyHigh {
		return nil
	}
	region := image.Rect(0, r.dirtyLow, gpu.RampWidth, r.dirtyHigh)
	r.dirtyLow, r.dirtyHigh = 0, 0
	return store.UpdateTexture(r.texture, region, r.pix, gpu.RampWidth*4)
}

func (r *rampTable) destroy() {
	if r.texture != nil {
		r.texture.Destroy()
	}
}
