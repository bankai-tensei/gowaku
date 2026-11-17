package paint

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	transparentPx = color.RGBA{}
	redPx         = color.RGBA{R: 255, A: 255}
	greenPx       = color.RGBA{G: 255, A: 255}
	bluePx        = color.RGBA{B: 255, A: 255}
	// halfBluePx is blue at alpha 128, premultiplied.
	halfBluePx = color.RGBA{B: 128, A: 128}
	// halfBlueOnRedPx is halfBluePx drawn over redPx.
	halfBlueOnRedPx = color.RGBA{R: 127, B: 128, A: 255}
)

// wantPixels checks pixels of frame i of a, within tol per channel.
func wantPixels(t *testing.T, a *Animation, i int, tol int, want map[image.Point]color.RGBA) {
	t.Helper()
	for p, w := range want {
		got := a.Frames[i].At(p.X, p.Y)
		if !near(got, w, tol) {
			t.Errorf("frame %d at %v: got %v, want %v", i, p, got, w)
		}
	}
}

func near(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) bool { return int(x)-int(y) <= tol && int(y)-int(x) <= tol }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}

func decodeBytes(t *testing.T, data []byte) *Animation {
	t.Helper()
	a, err := DecodeAnimation(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	checkAnimation(t, a)
	return a
}

// checkAnimation checks what every decoded animation must satisfy.
func checkAnimation(t testing.TB, a *Animation) {
	t.Helper()
	if len(a.Frames) == 0 || len(a.Frames) != len(a.Delays) {
		t.Fatalf("%d frames, %d delays", len(a.Frames), len(a.Delays))
	}
	w, h := a.Size()
	for i, f := range a.Frames {
		pix, fw, fh := f.Pixels()
		if fw != w || fh != h || len(pix) != w*h*4 {
			t.Fatalf("frame %d is %dx%d with %d bytes, want %dx%d", i, fw, fh, len(pix), w, h)
		}
		for j := 0; j < len(pix); j += 4 {
			if pix[j] > pix[j+3] || pix[j+1] > pix[j+3] || pix[j+2] > pix[j+3] {
				t.Fatalf("frame %d is not premultiplied at byte %d", i, j)
			}
		}
		if a.Delays[i] <= minFrameDelay {
			t.Fatalf("delay %d is %v", i, a.Delays[i])
		}
	}
	if a.Loops < 0 {
		t.Fatalf("loops %d", a.Loops)
	}
}

func TestAnimation_FrameAt(t *testing.T) {
	ms := time.Millisecond
	a := &Animation{Frames: make([]*Image, 3), Delays: []time.Duration{100 * ms, 200 * ms, 300 * ms}, Loops: 2}
	cases := []struct {
		t     time.Duration
		frame int
		next  time.Duration
		ended bool
	}{
		{0, 0, 100 * ms, false},
		{99 * ms, 0, 100 * ms, false},
		{100 * ms, 1, 300 * ms, false},
		{599 * ms, 2, 600 * ms, false},
		{600 * ms, 0, 700 * ms, false},
		{1199 * ms, 2, 1200 * ms, false},
		{1200 * ms, 2, 0, true},
		{time.Hour, 2, 0, true},
		{-ms, 0, 100 * ms, false},
	}
	for _, c := range cases {
		frame, next, ended := a.FrameAt(c.t)
		if frame != c.frame || next != c.next || ended != c.ended {
			t.Errorf("FrameAt(%v) = %d, %v, %v; want %d, %v, %v", c.t, frame, next, ended, c.frame, c.next, c.ended)
		}
	}
	a.Loops = LoopForever
	if frame, next, ended := a.FrameAt(1250 * ms); frame != 0 || next != 1300*ms || ended {
		t.Errorf("forever: got %d, %v, %v", frame, next, ended)
	}
	one := &Animation{Frames: make([]*Image, 1), Delays: []time.Duration{time.Second}}
	if frame, _, ended := one.FrameAt(5 * time.Second); frame != 0 || !ended {
		t.Error("one frame is over from the start")
	}
	if allocs := testing.AllocsPerRun(100, func() { a.FrameAt(777 * ms) }); allocs != 0 {
		t.Errorf("FrameAt allocates %v", allocs)
	}
	defer func() {
		if r := recover(); r == nil || !strings.HasPrefix(r.(string), "gowaku: ") {
			t.Errorf("got %v, want a gowaku panic", r)
		}
	}()
	(&Animation{Frames: make([]*Image, 2), Delays: []time.Duration{ms}}).FrameAt(0)
}

// --- GIF ---

// gifDisposalCase builds three frames on a 4x2 canvas: opaque red over all of
// it, then blue and a transparent pixel at the top left with disposal d, then
// green at (3,1) with a palette of its own.
func gifDisposalCase(t *testing.T, d byte, loop int) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{}, redPx, bluePx}
	f0 := image.NewPaletted(image.Rect(0, 0, 4, 2), pal)
	for i := range f0.Pix {
		f0.Pix[i] = 1
	}
	f1 := image.NewPaletted(image.Rect(0, 0, 2, 1), pal)
	f1.Pix[0], f1.Pix[1] = 2, 0
	f2 := image.NewPaletted(image.Rect(3, 1, 4, 2), color.Palette{greenPx})
	g := &gif.GIF{
		Image:     []*image.Paletted{f0, f1, f2},
		Delay:     []int{0, 1, 5},
		Disposal:  []byte{gif.DisposalNone, d, 0},
		LoopCount: loop,
		Config:    image.Config{ColorModel: pal, Width: 4, Height: 2},
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDecodeAnimation_GIFDisposal(t *testing.T) {
	cases := []struct {
		name     string
		disposal byte
		topLeft  color.RGBA
		next     color.RGBA
	}{
		{"unspecified", 0, bluePx, redPx},
		{"none", gif.DisposalNone, bluePx, redPx},
		{"background", gif.DisposalBackground, transparentPx, transparentPx},
		{"previous", gif.DisposalPrevious, redPx, redPx},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := decodeBytes(t, gifDisposalCase(t, c.disposal, 0))
			if len(a.Frames) != 3 {
				t.Fatalf("%d frames", len(a.Frames))
			}
			wantPixels(t, a, 0, 0, map[image.Point]color.RGBA{{0, 0}: redPx, {3, 1}: redPx})
			// The transparent index leaves the red below.
			wantPixels(t, a, 1, 0, map[image.Point]color.RGBA{{0, 0}: bluePx, {1, 0}: redPx, {3, 1}: redPx})
			wantPixels(t, a, 2, 0, map[image.Point]color.RGBA{
				{0, 0}: c.topLeft, {1, 0}: c.next, {2, 0}: redPx, {0, 1}: redPx, {3, 1}: greenPx,
			})
		})
	}
}

func TestDecodeAnimation_GIFDelaysAndLoops(t *testing.T) {
	a := decodeBytes(t, gifDisposalCase(t, 0, 0))
	want := []time.Duration{shortDelay, shortDelay, 50 * time.Millisecond}
	for i, d := range want {
		if a.Delays[i] != d {
			t.Errorf("delay %d: got %v, want %v", i, a.Delays[i], d)
		}
	}
	for loop, plays := range map[int]int{0: LoopForever, -1: 1, 2: 3} {
		if a := decodeBytes(t, gifDisposalCase(t, 0, loop)); a.Loops != plays {
			t.Errorf("loop count %d: got %d plays, want %d", loop, a.Loops, plays)
		}
	}
}

// --- APNG ---

// apngTestFrame is a frame for buildAPNG. pix holds its samples in rows.
type apngTestFrame struct {
	x, y, w, h     int
	pix            []byte
	num, den       uint16
	dispose, blend byte
}

// pngSpec is the format of the pictures of buildAPNG.
type pngSpec struct {
	colorType byte
	// bpp is the number of bytes of a pixel.
	bpp    int
	shared map[string][]byte
}

var rgbaSpec = pngSpec{colorType: 6, bpp: 4}

// buildAPNG writes an APNG file. A hidden default image, if any, is not a
// frame; without one the first frame is the default image.
func buildAPNG(w, h int, plays uint32, spec pngSpec, hidden []byte, frames []apngTestFrame) []byte {
	b := []byte(pngSignature)
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr, uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9] = 8, spec.colorType
	b = appendChunk(b, "IHDR", ihdr)
	for _, name := range []string{"PLTE", "tRNS"} {
		if body, ok := spec.shared[name]; ok {
			b = appendChunk(b, name, body)
		}
	}
	b = appendChunk(b, "acTL", binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, uint32(len(frames))), plays))
	var seq uint32
	next := func() []byte {
		s := binary.BigEndian.AppendUint32(nil, seq)
		seq++
		return s
	}
	if hidden != nil {
		b = appendChunk(b, "IDAT", zrows(hidden, w, spec.bpp))
	}
	for i, f := range frames {
		fc := next()
		for _, v := range []int{f.w, f.h, f.x, f.y} {
			fc = binary.BigEndian.AppendUint32(fc, uint32(v))
		}
		fc = binary.BigEndian.AppendUint16(fc, f.num)
		fc = binary.BigEndian.AppendUint16(fc, f.den)
		fc = append(fc, f.dispose, f.blend)
		b = appendChunk(b, "fcTL", fc)
		data := zrows(f.pix, f.w, spec.bpp)
		if i == 0 && hidden == nil {
			b = appendChunk(b, "IDAT", data)
		} else {
			b = appendChunk(b, "fdAT", append(next(), data...))
		}
	}
	return appendChunk(b, "IEND", nil)
}

// zrows compresses rows of samples with the filter None.
func zrows(pix []byte, w, bpp int) []byte {
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	for row := 0; row < len(pix); row += w * bpp {
		if _, err := zw.Write([]byte{0}); err != nil {
			panic(err)
		}
		if _, err := zw.Write(pix[row : row+w*bpp]); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return z.Bytes()
}

func nrgbaPix(w, h int, c ...color.NRGBA) []byte {
	pix := make([]byte, 0, w*h*4)
	for i := range w * h {
		p := c[i%len(c)]
		pix = append(pix, p.R, p.G, p.B, p.A)
	}
	return pix
}

var (
	nRed      = color.NRGBA{R: 255, A: 255}
	nGreen    = color.NRGBA{G: 255, A: 255}
	nHalfBlue = color.NRGBA{B: 255, A: 128}
	nMagenta  = color.NRGBA{R: 255, B: 255, A: 255}
	nClear    = color.NRGBA{}
)

// apngModesCase builds three frames on a 4x1 canvas: opaque red, then half
// blue and a transparent pixel at the left with the given blend and dispose
// ops, then green at the right.
func apngModesCase(dispose, blend byte) []byte {
	return buildAPNG(4, 1, 0, rgbaSpec, nil, []apngTestFrame{
		{w: 4, h: 1, pix: nrgbaPix(4, 1, nRed), num: 1, den: 10},
		{w: 2, h: 1, pix: nrgbaPix(2, 1, nHalfBlue, nClear), num: 1, den: 10, dispose: dispose, blend: blend},
		{x: 3, w: 1, h: 1, pix: nrgbaPix(1, 1, nGreen), num: 1, den: 10},
	})
}

func TestDecodeAnimation_APNGDisposeAndBlend(t *testing.T) {
	for _, blend := range []struct {
		name        string
		op          byte
		left, right color.RGBA
	}{
		{"source", 0, halfBluePx, transparentPx},
		{"over", 1, halfBlueOnRedPx, redPx},
	} {
		for _, disp := range []struct {
			name string
			op   byte
			// after is what the left pixels hold under the green frame;
			// nil means as in the frame before.
			after *color.RGBA
		}{
			{"none", 0, nil},
			{"background", 1, &transparentPx},
			{"previous", 2, &redPx},
		} {
			t.Run(blend.name+"/"+disp.name, func(t *testing.T) {
				a := decodeBytes(t, apngModesCase(disp.op, blend.op))
				if len(a.Frames) != 3 || a.Loops != LoopForever {
					t.Fatalf("%d frames, %d loops", len(a.Frames), a.Loops)
				}
				wantPixels(t, a, 0, 0, map[image.Point]color.RGBA{{0, 0}: redPx, {3, 0}: redPx})
				wantPixels(t, a, 1, 1, map[image.Point]color.RGBA{{0, 0}: blend.left, {1, 0}: blend.right, {2, 0}: redPx})
				left, right := blend.left, blend.right
				if disp.after != nil {
					left, right = *disp.after, *disp.after
				}
				wantPixels(t, a, 2, 1, map[image.Point]color.RGBA{{0, 0}: left, {1, 0}: right, {2, 0}: redPx, {3, 0}: greenPx})
			})
		}
	}
}

func TestDecodeAnimation_APNGFirstFrameDisposePrevious(t *testing.T) {
	// The first frame has no previous picture, so it clears to transparent.
	a := decodeBytes(t, buildAPNG(2, 1, 1, rgbaSpec, nil, []apngTestFrame{
		{w: 2, h: 1, pix: nrgbaPix(2, 1, nRed), dispose: 2},
		{x: 1, w: 1, h: 1, pix: nrgbaPix(1, 1, nGreen), blend: 1},
	}))
	wantPixels(t, a, 1, 0, map[image.Point]color.RGBA{{0, 0}: transparentPx, {1, 0}: greenPx})
	if a.Loops != 1 {
		t.Errorf("loops %d", a.Loops)
	}
}

func TestDecodeAnimation_APNGHiddenDefaultImage(t *testing.T) {
	a := decodeBytes(t, buildAPNG(2, 2, 3, rgbaSpec, nrgbaPix(2, 2, nMagenta), []apngTestFrame{
		{x: 1, y: 1, w: 1, h: 1, pix: nrgbaPix(1, 1, nRed), num: 25},
		{w: 1, h: 1, pix: nrgbaPix(1, 1, nGreen), num: 3, den: 100},
	}))
	if len(a.Frames) != 2 || a.Loops != 3 {
		t.Fatalf("%d frames, %d loops", len(a.Frames), a.Loops)
	}
	// The default image is not part of the animation.
	wantPixels(t, a, 0, 0, map[image.Point]color.RGBA{{0, 0}: transparentPx, {1, 1}: redPx})
	wantPixels(t, a, 1, 0, map[image.Point]color.RGBA{{0, 0}: greenPx, {1, 1}: redPx, {1, 0}: transparentPx})
	// 25/0 is 25/100 of a second, and 3/100 is 30 milliseconds.
	if a.Delays[0] != 250*time.Millisecond || a.Delays[1] != 30*time.Millisecond {
		t.Errorf("delays %v", a.Delays)
	}
}

func TestDecodeAnimation_APNGPalette(t *testing.T) {
	spec := pngSpec{colorType: 3, bpp: 1, shared: map[string][]byte{
		"PLTE": {0, 0, 0, 255, 0, 0, 0, 0, 255},
		"tRNS": {0, 255, 128},
	}}
	a := decodeBytes(t, buildAPNG(2, 1, 0, spec, nil, []apngTestFrame{
		{w: 2, h: 1, pix: []byte{1, 0}},
		{x: 1, w: 1, h: 1, pix: []byte{2}, blend: 1},
	}))
	wantPixels(t, a, 0, 0, map[image.Point]color.RGBA{{0, 0}: redPx, {1, 0}: transparentPx})
	wantPixels(t, a, 1, 1, map[image.Point]color.RGBA{{0, 0}: redPx, {1, 0}: halfBluePx})
	// Short delays follow the convention of browsers.
	if a.Delays[0] != shortDelay {
		t.Errorf("delay %v", a.Delays[0])
	}
}

func TestDecodeAnimation_APNGErrors(t *testing.T) {
	good := apngModesCase(0, 0)
	corrupt := func(f func(b []byte) []byte) []byte { return f(bytes.Clone(good)) }
	// chunkAt returns the offset of the n-th chunk of type typ.
	chunkAt := func(b []byte, typ string, n int) int {
		for p := 8; p+8 <= len(b); p += 12 + int(binary.BigEndian.Uint32(b[p:])) {
			if string(b[p+4:p+8]) == typ {
				if n == 0 {
					return p
				}
				n--
			}
		}
		t.Fatalf("no %s", typ)
		return 0
	}
	// rechunk rewrites the checksum of the chunk at p.
	rechunk := func(b []byte, p int) []byte {
		n := int(binary.BigEndian.Uint32(b[p:]))
		binary.BigEndian.PutUint32(b[p+8+n:], crc32.ChecksumIEEE(b[p+4:p+8+n]))
		return b
	}
	cases := map[string][]byte{
		"bad checksum": corrupt(func(b []byte) []byte { b[chunkAt(b, "fdAT", 0)+12] ^= 1; return b }),
		"bad sequence": corrupt(func(b []byte) []byte {
			p := chunkAt(b, "fdAT", 0)
			binary.BigEndian.PutUint32(b[p+8:], 7)
			return rechunk(b, p)
		}),
		"frame outside the canvas": corrupt(func(b []byte) []byte {
			p := chunkAt(b, "fcTL", 2)
			// x of a frame 1 wide on a canvas 4 wide.
			binary.BigEndian.PutUint32(b[p+8+16:], 4)
			return rechunk(b, p)
		}),
		"more frames than acTL": corrupt(func(b []byte) []byte {
			p := chunkAt(b, "acTL", 0)
			binary.BigEndian.PutUint32(b[p+8:], 2)
			return rechunk(b, p)
		}),
		"fewer frames than acTL": corrupt(func(b []byte) []byte {
			p := chunkAt(b, "acTL", 0)
			binary.BigEndian.PutUint32(b[p+8:], 4)
			return rechunk(b, p)
		}),
		"truncated": good[:len(good)-20],
		"bad dispose op": corrupt(func(b []byte) []byte {
			p := chunkAt(b, "fcTL", 1)
			b[p+8+24] = 3
			return rechunk(b, p)
		}),
	}
	for name, data := range cases {
		if _, err := DecodeAnimation(bytes.NewReader(data)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	tooMany := buildAPNG(1, 1, 0, rgbaSpec, nil, []apngTestFrame{{w: 1, h: 1, pix: nrgbaPix(1, 1, nRed)}})
	p := chunkAt(tooMany, "acTL", 0)
	binary.BigEndian.PutUint32(tooMany[p+8:], MaxAnimationFrames+1)
	rechunk(tooMany, p)
	if _, err := DecodeAnimation(bytes.NewReader(tooMany)); !errors.Is(err, ErrAnimationTooLarge) {
		t.Errorf("too many frames: %v", err)
	}
}

// --- WebP ---

// webpBitstream returns the chunks after the header of a still WebP fixture:
// its bitstream, with ALPH before it if any.
func webpBitstream(t *testing.T, name string) webpFrame {
	t.Helper()
	data := readFixture(t, name+".webp")
	chunks, err := riffChunks(data[12:])
	if err != nil {
		t.Fatal(err)
	}
	fr, err := frameChunks(chunks)
	if err != nil {
		t.Fatal(err)
	}
	return fr
}

func readFixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "anim", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// webpTestFrame is a frame for buildWebP.
type webpTestFrame struct {
	x, y, w, h int
	ms         int
	flags      byte
	fr         webpFrame
}

func buildWebP(w, h int, loops uint16, frames []webpTestFrame) []byte {
	var vp8x [10]byte
	vp8x[0] = webpFlagAnimation | webpFlagAlpha
	putUint24(vp8x[4:], uint32(w-1))
	putUint24(vp8x[7:], uint32(h-1))
	anim := binary.LittleEndian.AppendUint16([]byte{0xff, 0xff, 0xff, 0xff}, loops)
	chunks := []riffChunk{{"VP8X", vp8x[:]}, {"ANIM", anim}}
	for _, f := range frames {
		var head [16]byte
		putUint24(head[0:], uint32(f.x/2))
		putUint24(head[3:], uint32(f.y/2))
		putUint24(head[6:], uint32(f.w-1))
		putUint24(head[9:], uint32(f.h-1))
		putUint24(head[12:], uint32(f.ms))
		head[15] = f.flags
		body := appendRIFF(nil, f.fr.alpha, f.fr.bitstream)[12:]
		chunks = append(chunks, riffChunk{"ANMF", append(head[:], body...)})
	}
	return appendRIFF(nil, chunks...)
}

func TestDecodeAnimation_WebPDisposeAndBlend(t *testing.T) {
	red, blue, green := webpBitstream(t, "red4"), webpBitstream(t, "blue4half"), webpBitstream(t, "green2")
	for _, blend := range []struct {
		name  string
		flags byte
		over  color.RGBA
	}{
		{"blend", 0, halfBlueOnRedPx},
		{"no blend", anmfNoBlend, halfBluePx},
	} {
		for _, disp := range []struct {
			name    string
			flags   byte
			cleared bool
		}{
			{"none", 0, false},
			{"background", anmfDisposeBackground, true},
		} {
			t.Run(blend.name+"/"+disp.name, func(t *testing.T) {
				a := decodeBytes(t, buildWebP(8, 4, 2, []webpTestFrame{
					{w: 4, h: 4, ms: 0, flags: anmfNoBlend, fr: red},
					{x: 2, w: 4, h: 4, ms: 50, flags: blend.flags | disp.flags, fr: blue},
					{x: 6, y: 2, w: 2, h: 2, ms: 70, flags: anmfNoBlend, fr: green},
				}))
				if len(a.Frames) != 3 || a.Loops != 2 {
					t.Fatalf("%d frames, %d loops", len(a.Frames), a.Loops)
				}
				if a.Delays[0] != shortDelay || a.Delays[1] != 50*time.Millisecond {
					t.Errorf("delays %v", a.Delays)
				}
				wantPixels(t, a, 0, 0, map[image.Point]color.RGBA{{0, 0}: redPx, {3, 3}: redPx, {4, 0}: transparentPx})
				wantPixels(t, a, 1, 1, map[image.Point]color.RGBA{
					{0, 0}: redPx, {2, 0}: blend.over, {3, 3}: blend.over, {4, 0}: halfBluePx, {6, 0}: transparentPx,
				})
				mid, side := blend.over, halfBluePx
				if disp.cleared {
					mid, side = transparentPx, transparentPx
				}
				wantPixels(t, a, 2, 1, map[image.Point]color.RGBA{
					{0, 0}: redPx, {2, 0}: mid, {4, 1}: side, {6, 2}: greenPx, {7, 3}: greenPx, {6, 0}: transparentPx,
				})
			})
		}
	}
}

func TestDecodeAnimation_WebPAlphaWithLossy(t *testing.T) {
	// ALPH and VP8 make a file with a VP8X chunk of their own.
	a := decodeBytes(t, buildWebP(6, 4, 0, []webpTestFrame{
		{x: 2, w: 4, h: 4, ms: 100, fr: webpBitstream(t, "whitehalf4")},
	}))
	wantPixels(t, a, 0, 6, map[image.Point]color.RGBA{
		{0, 0}: transparentPx, {2, 1}: transparentPx, {3, 2}: transparentPx,
		{4, 0}: {255, 255, 255, 255}, {5, 3}: {255, 255, 255, 255},
	})
}

func TestDecodeAnimation_WebPErrors(t *testing.T) {
	red := webpBitstream(t, "red4")
	cases := map[string][]byte{
		"outside the canvas":  buildWebP(4, 4, 0, []webpTestFrame{{x: 2, w: 4, h: 4, fr: red}}),
		"bitstream size":      buildWebP(4, 4, 0, []webpTestFrame{{w: 2, h: 2, fr: red}}),
		"no frames":           buildWebP(4, 4, 0, nil),
		"truncated":           buildWebP(4, 4, 0, []webpTestFrame{{w: 4, h: 4, fr: red}})[:40],
		"canvas past the max": buildWebP(MaxAnimationSide+2, 4, 0, []webpTestFrame{{w: 4, h: 4, fr: red}}),
	}
	for name, data := range cases {
		if _, err := DecodeAnimation(bytes.NewReader(data)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	noAnim := buildWebP(4, 4, 0, []webpTestFrame{{w: 4, h: 4, fr: red}})
	noAnim = bytes.Replace(noAnim, []byte("ANIM"), []byte("XNIM"), 1)
	if _, err := DecodeAnimation(bytes.NewReader(noAnim)); err == nil {
		t.Error("no ANIM: no error")
	}
	big := buildWebP(MaxAnimationSide, MaxAnimationSide, 0, []webpTestFrame{{w: 4, h: 4, fr: red}})
	if _, err := DecodeAnimation(bytes.NewReader(big)); !errors.Is(err, ErrAnimationTooLarge) {
		t.Errorf("too many pixels: %v", err)
	}
}

// TestDecodeAnimation_EncoderFixtures decodes animations that real encoders
// made from fixtureMovie: libwebp's WebPAnimEncoder and ffmpeg's APNG encoder,
// which pick partial frames, blends and disposals on their own. Both are
// lossless, so each frame must come back exactly.
func TestDecodeAnimation_EncoderFixtures(t *testing.T) {
	want := fixtureMovie()
	for _, name := range []string{"movie.webp", "movie.png"} {
		t.Run(name, func(t *testing.T) {
			a := decodeBytes(t, readFixture(t, name))
			if len(a.Frames) != len(want) || a.Loops != LoopForever {
				t.Fatalf("%d frames, %d loops", len(a.Frames), a.Loops)
			}
			for i, m := range want {
				if a.Delays[i] != 100*time.Millisecond {
					t.Errorf("delay %d: %v", i, a.Delays[i])
				}
				if got, exp := a.Frames[i], NewImage(m); !bytes.Equal(got.pix, exp.pix) {
					t.Errorf("frame %d differs from its source", i)
				}
			}
		})
	}
}

// --- Still pictures and limits ---

func TestDecodeAnimation_StillPictures(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := range m.Pix {
		m.Pix[i] = 255
	}
	var pngData, jpegData, gifData bytes.Buffer
	if err := png.Encode(&pngData, m); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, m, nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&gifData, m, nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"png":         pngData.Bytes(),
		"jpeg":        jpegData.Bytes(),
		"gif":         gifData.Bytes(),
		"webp vp8l":   readFixture(t, "red4.webp"),
		"webp alpha":  readFixture(t, "whitehalf4.webp"),
		"webp simple": readFixture(t, "green2.webp"),
	} {
		a, err := DecodeAnimation(bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		checkAnimation(t, a)
		if len(a.Frames) != 1 {
			t.Errorf("%s: %d frames", name, len(a.Frames))
		}
	}
	if _, err := DecodeAnimation(strings.NewReader("hello, world")); err == nil {
		t.Error("an unknown format decodes")
	}
}

func TestDecodeAnimation_GIFLimits(t *testing.T) {
	// A logical screen past the side limit, with one tiny frame.
	pal := color.Palette{redPx}
	g := &gif.GIF{
		Image:  []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 1, 1), pal)},
		Delay:  []int{0},
		Config: image.Config{ColorModel: pal, Width: MaxAnimationSide + 1, Height: 1},
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAnimation(&b); !errors.Is(err, ErrAnimationTooLarge) {
		t.Errorf("wide screen: %v", err)
	}
	// Many frames of a canvas that is large together.
	g.Config.Width, g.Config.Height = 4096, 1024
	for range 8 {
		g.Image = append(g.Image, g.Image[0])
		g.Delay = append(g.Delay, 0)
	}
	b.Reset()
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAnimation(&b); !errors.Is(err, ErrAnimationTooLarge) {
		t.Errorf("many pixels: %v", err)
	}
}

// --- Fuzzing ---

func animationSeeds(t testing.TB) map[string][]byte {
	seeds := map[string][]byte{
		"apng modes":  apngModesCase(2, 1),
		"apng hidden": buildAPNG(2, 2, 3, rgbaSpec, nrgbaPix(2, 2, nMagenta), []apngTestFrame{{x: 1, y: 1, w: 1, h: 1, pix: nrgbaPix(1, 1, nRed)}}),
		"movie.webp":  readFixture(t, "movie.webp"),
		"movie.png":   readFixture(t, "movie.png"),
		"red4.webp":   readFixture(t, "red4.webp"),
		"white.webp":  readFixture(t, "whitehalf4.webp"),
	}
	if tt, ok := t.(*testing.T); ok {
		seeds["gif"] = gifDisposalCase(tt, gif.DisposalPrevious, 2)
	}
	return seeds
}

func fuzzDecode(t *testing.T, data []byte) {
	a, err := decodeAnimation(data)
	if err != nil {
		if a != nil {
			t.Fatal("an animation with an error")
		}
		return
	}
	checkAnimation(t, a)
	w, h := a.Size()
	if checkSize(w, h, len(a.Frames)) != nil {
		t.Fatalf("%d frames of %dx%d pass the limits", len(a.Frames), w, h)
	}
}

func FuzzDecodeAnimation(f *testing.F) {
	for _, s := range animationSeeds(f) {
		f.Add(s)
	}
	var b bytes.Buffer
	pal := color.Palette{color.RGBA{}, redPx}
	err := gif.EncodeAll(&b, &gif.GIF{
		Image:    []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 2, 2), pal), image.NewPaletted(image.Rect(1, 1, 2, 2), pal)},
		Delay:    []int{1, 2},
		Disposal: []byte{gif.DisposalBackground, gif.DisposalPrevious},
		Config:   image.Config{ColorModel: pal, Width: 2, Height: 2},
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(b.Bytes())
	f.Fuzz(fuzzDecode)
}

// FuzzDecodeAPNG mutates APNG files only, and mends the checksums of the
// chunks, which would refuse nearly every mutation, to reach deeper into them.
func FuzzDecodeAPNG(f *testing.F) {
	f.Add(apngModesCase(0, 0))
	f.Add(apngModesCase(1, 1))
	f.Add(apngModesCase(2, 0))
	f.Add(buildAPNG(2, 2, 3, rgbaSpec, nrgbaPix(2, 2, nMagenta), []apngTestFrame{{x: 1, y: 1, w: 1, h: 1, pix: nrgbaPix(1, 1, nRed)}}))
	f.Add(readFixture(f, "movie.png"))
	f.Fuzz(func(t *testing.T, data []byte) { fuzzDecode(t, mendPNGChecksums(data)) })
}

// mendPNGChecksums returns data with the checksum of each whole chunk
// rewritten.
func mendPNGChecksums(data []byte) []byte {
	data = bytes.Clone(data)
	for p := len(pngSignature); p+12 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[p:]))
		if n < 0 || n > len(data)-p-12 {
			break
		}
		binary.BigEndian.PutUint32(data[p+8+n:], crc32.ChecksumIEEE(data[p+4:p+8+n]))
		p += 12 + n
	}
	return data
}

// FuzzDecodeAnimatedWebP mutates WebP files only, to reach deeper into the
// chunks.
func FuzzDecodeAnimatedWebP(f *testing.F) {
	f.Add(readFixture(f, "movie.webp"))
	f.Add(readFixture(f, "whitehalf4.webp"))
	f.Add(readFixture(f, "blue4half.webp"))
	f.Fuzz(fuzzDecode)
}
