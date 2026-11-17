package paint

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"time"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

// apngFrame is a frame of an APNG file, as its fcTL chunk describes it.
type apngFrame struct {
	rect     image.Rectangle
	delay    time.Duration
	dispose  dispose
	over     bool
	payloads [][]byte
}

// apngFile is what decodePNGAnimation finds in the chunks of a PNG file.
type apngFile struct {
	ihdr []byte
	// shared are the chunks before the image data that a frame needs to be
	// decoded on its own: the palette and its transparency.
	shared         [][]byte
	animated       bool
	frameCount     int
	plays          int
	frames         []apngFrame
	defaultIsFrame bool
	seq            uint32
}

// decodePNGAnimation reads an APNG file, or a still PNG file as one frame.
// Each frame is rebuilt as a PNG stream of its own and decoded by image/png.
func decodePNGAnimation(data []byte) (*Animation, error) {
	f, err := parseAPNG(data)
	if err != nil {
		return nil, err
	}
	if !f.animated {
		return decodeStill(data, png.DecodeConfig, png.Decode)
	}
	width, height := int(binary.BigEndian.Uint32(f.ihdr)), int(binary.BigEndian.Uint32(f.ihdr[4:]))
	if len(f.frames) != f.frameCount {
		return nil, fmt.Errorf("apng: %d frames, acTL says %d", len(f.frames), f.frameCount)
	}
	if err := checkSize(width, height, len(f.frames)); err != nil {
		return nil, err
	}
	canvas := image.Rect(0, 0, width, height)
	c := newCompositor(width, height, len(f.frames))
	var stream []byte
	for i := range f.frames {
		fr := &f.frames[i]
		if !fr.rect.In(canvas) {
			return nil, fmt.Errorf("apng: frame %d at %v outside the canvas", i, fr.rect)
		}
		if i == 0 && f.defaultIsFrame && fr.rect != canvas {
			return nil, errors.New("apng: the first frame is the default image but not the canvas")
		}
		if len(fr.payloads) == 0 {
			return nil, fmt.Errorf("apng: frame %d has no data", i)
		}
		stream = f.frameStream(stream[:0], fr)
		m, err := png.Decode(bytes.NewReader(stream))
		if err != nil {
			return nil, fmt.Errorf("apng: frame %d: %w", i, err)
		}
		if m.Bounds().Size() != fr.rect.Size() {
			return nil, fmt.Errorf("apng: frame %d decodes to %v", i, m.Bounds())
		}
		disp := fr.dispose
		if i == 0 && disp == disposePrevious {
			// There is no previous picture: the specification treats it as
			// disposal to the background.
			disp = disposeBackground
		}
		c.add(m, fr.rect, fr.over, disp, fr.delay)
	}
	return c.result(f.plays)
}

// parseAPNG checks the chunks of a PNG file and collects those of an APNG.
func parseAPNG(data []byte) (*apngFile, error) {
	f := &apngFile{}
	p := len(pngSignature)
	sawData := false
	for {
		if len(data)-p < 12 {
			return nil, errors.New("apng: truncated chunk")
		}
		n := binary.BigEndian.Uint32(data[p:])
		if n > 1<<31-1 || uint64(n) > uint64(len(data)-p-12) {
			return nil, errors.New("apng: chunk past the end")
		}
		typ := string(data[p+4 : p+8])
		body := data[p+8 : p+8+int(n)]
		if crc32.ChecksumIEEE(data[p+4:p+8+int(n)]) != binary.BigEndian.Uint32(data[p+8+int(n):]) {
			return nil, fmt.Errorf("apng: bad checksum of %q", typ)
		}
		p += 12 + int(n)
		if f.ihdr == nil && typ != "IHDR" {
			return nil, errors.New("apng: IHDR is not first")
		}
		var err error
		switch typ {
		case "IHDR":
			if f.ihdr != nil || len(body) != 13 {
				return nil, errors.New("apng: bad IHDR")
			}
			f.ihdr = body
		case "PLTE", "tRNS":
			if !sawData {
				f.shared = append(f.shared, data[p-12-int(n):p])
			}
		case "acTL":
			err = f.actl(body, sawData)
		case "fcTL":
			err = f.fctl(body, sawData)
		case "IDAT":
			sawData = true
			if len(f.frames) == 1 && f.defaultIsFrame {
				f.frames[0].payloads = append(f.frames[0].payloads, body)
			}
		case "fdAT":
			err = f.fdat(body)
		case "IEND":
			if !sawData {
				return nil, errors.New("apng: no IDAT")
			}
			return f, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func (f *apngFile) actl(body []byte, sawData bool) error {
	if f.animated || sawData || len(body) != 8 {
		return errors.New("apng: bad acTL")
	}
	frames := binary.BigEndian.Uint32(body)
	plays := binary.BigEndian.Uint32(body[4:])
	if frames == 0 {
		return errors.New("apng: acTL without frames")
	}
	if frames > MaxAnimationFrames {
		return fmt.Errorf("apng: %d frames past %d: %w", frames, MaxAnimationFrames, ErrAnimationTooLarge)
	}
	if plays > 1<<31-1 {
		return errors.New("apng: bad play count")
	}
	f.animated, f.frameCount, f.plays = true, int(frames), int(plays)
	return nil
}

// sequence checks the sequence number that starts body, which counts fcTL and
// fdAT chunks together from zero.
func (f *apngFile) sequence(body []byte) error {
	if binary.BigEndian.Uint32(body) != f.seq {
		return fmt.Errorf("apng: sequence number %d, want %d", binary.BigEndian.Uint32(body), f.seq)
	}
	f.seq++
	return nil
}

func (f *apngFile) fctl(body []byte, sawData bool) error {
	if !f.animated || len(body) != 26 {
		return errors.New("apng: bad fcTL")
	}
	if err := f.sequence(body); err != nil {
		return err
	}
	if len(f.frames) >= f.frameCount {
		return errors.New("apng: more fcTL than acTL frames")
	}
	if len(f.frames) > 0 && len(f.frames[len(f.frames)-1].payloads) == 0 {
		return errors.New("apng: frame without data")
	}
	be := binary.BigEndian
	w, h, x, y := be.Uint32(body[4:]), be.Uint32(body[8:]), be.Uint32(body[12:]), be.Uint32(body[16:])
	if w == 0 || h == 0 || w > MaxAnimationSide || h > MaxAnimationSide || x > MaxAnimationSide || y > MaxAnimationSide {
		return fmt.Errorf("apng: frame %dx%d at %d,%d", w, h, x, y)
	}
	num, den := time.Duration(be.Uint16(body[20:])), time.Duration(be.Uint16(body[22:]))
	if den == 0 {
		den = 100
	}
	if body[24] > 2 || body[25] > 1 {
		return errors.New("apng: bad dispose or blend op")
	}
	if !sawData && len(f.frames) == 0 {
		f.defaultIsFrame = true
	}
	f.frames = append(f.frames, apngFrame{
		rect:    image.Rect(int(x), int(y), int(x+w), int(y+h)),
		delay:   num * time.Second / den,
		dispose: dispose(body[24]),
		over:    body[25] == 1,
	})
	return nil
}

func (f *apngFile) fdat(body []byte) error {
	if !f.animated || len(body) < 4 {
		return errors.New("apng: bad fdAT")
	}
	if err := f.sequence(body); err != nil {
		return err
	}
	if len(f.frames) == 0 || (len(f.frames) == 1 && f.defaultIsFrame) {
		return errors.New("apng: fdAT before the fcTL of its frame")
	}
	fr := &f.frames[len(f.frames)-1]
	fr.payloads = append(fr.payloads, body[4:])
	return nil
}

// frameStream appends to b a PNG file that holds the frame alone: the header
// with the size of the frame, the shared chunks and its data as IDAT.
func (f *apngFile) frameStream(b []byte, fr *apngFrame) []byte {
	b = append(b, pngSignature...)
	var ihdr [13]byte
	copy(ihdr[:], f.ihdr)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(fr.rect.Dx()))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(fr.rect.Dy()))
	b = appendChunk(b, "IHDR", ihdr[:])
	for _, c := range f.shared {
		b = append(b, c...)
	}
	for _, d := range fr.payloads {
		b = appendChunk(b, "IDAT", d)
	}
	return appendChunk(b, "IEND", nil)
}

// appendChunk appends a PNG chunk with its length and checksum.
func appendChunk(b []byte, typ string, body []byte) []byte {
	b = binary.BigEndian.AppendUint32(b, uint32(len(body)))
	start := len(b)
	b = append(b, typ...)
	b = append(b, body...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b[start:]))
}
