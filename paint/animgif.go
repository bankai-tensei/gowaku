package paint

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"time"
)

// decodeGIFAnimation reads a GIF file. The size and number of its frames are
// checked from its blocks before image/gif decodes them.
func decodeGIFAnimation(data []byte) (*Animation, error) {
	width, height, frames, err := scanGIF(data)
	if err != nil {
		return nil, err
	}
	if err := checkSize(width, height, frames); err != nil {
		return nil, err
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if g.Config.Width != width || g.Config.Height != height || len(g.Image) > frames {
		return nil, errors.New("gif: inconsistent file")
	}
	canvas := image.Rect(0, 0, width, height)
	c := newCompositor(width, height, len(g.Image))
	for i, m := range g.Image {
		r := m.Bounds().Intersect(canvas)
		if r.Empty() {
			// Nothing of the frame shows, but it still takes its time.
			r = image.Rectangle{}
		}
		var disp dispose
		if i < len(g.Disposal) {
			switch g.Disposal[i] {
			case gif.DisposalBackground:
				disp = disposeBackground
			case gif.DisposalPrevious:
				disp = disposePrevious
			}
		}
		var delay time.Duration
		if i < len(g.Delay) {
			delay = time.Duration(g.Delay[i]) * 10 * time.Millisecond
		}
		// The transparent index of a GIF frame is a palette entry of zero
		// alpha and the others are opaque, so drawing over leaves the canvas
		// where the frame is transparent, which is what GIF asks.
		c.add(m.SubImage(r), r, true, disp, delay)
	}
	return c.result(gifLoops(g.LoopCount))
}

// gifLoops converts the loop count of image/gif, where 0 repeats forever, -1
// plays once and n plays n+1 times, as browsers do.
func gifLoops(n int) int {
	switch {
	case n == 0:
		return LoopForever
	case n < 0:
		return 1
	}
	return n + 1
}

// scanGIF walks the blocks of a GIF file and returns the size of its canvas
// and the number of its frames.
func scanGIF(b []byte) (width, height, frames int, err error) {
	if len(b) < 13 {
		return 0, 0, 0, errors.New("gif: short header")
	}
	width, height = int(b[6])|int(b[7])<<8, int(b[8])|int(b[9])<<8
	p := 13
	if flags := b[10]; flags&0x80 != 0 {
		p += 3 << (flags&7 + 1)
	}
	for p < len(b) {
		switch b[p] {
		case 0x21: // extension: label, then data sub-blocks
			if p, err = skipSubBlocks(b, p+2); err != nil {
				return 0, 0, 0, err
			}
		case 0x2c: // image descriptor
			if p+10 > len(b) {
				return 0, 0, 0, errors.New("gif: short image descriptor")
			}
			flags := b[p+9]
			p += 10
			if flags&0x80 != 0 {
				p += 3 << (flags&7 + 1)
			}
			// The minimum code size of LZW, then the data.
			if p, err = skipSubBlocks(b, p+1); err != nil {
				return 0, 0, 0, err
			}
			frames++
			if frames > MaxAnimationFrames {
				return 0, 0, 0, fmt.Errorf("gif: more than %d frames: %w", MaxAnimationFrames, ErrAnimationTooLarge)
			}
		case 0x3b: // trailer
			return width, height, frames, nil
		default:
			return 0, 0, 0, fmt.Errorf("gif: unknown block %#x", b[p])
		}
	}
	return width, height, frames, nil
}

// skipSubBlocks returns the position after the sub-blocks that start at p.
func skipSubBlocks(b []byte, p int) (int, error) {
	for {
		if p >= len(b) {
			return 0, errors.New("gif: truncated block")
		}
		n := int(b[p])
		p += 1 + n
		if n == 0 {
			return p, nil
		}
	}
}
