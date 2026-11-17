package paint

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"time"
)

// The limits of DecodeAnimation. A file comes from outside the program, so
// that one that declares more than these is refused before its pixels are
// decoded.
const (
	// MaxAnimationBytes is the largest file DecodeAnimation reads: 256 MiB.
	MaxAnimationBytes = 256 << 20
	// MaxAnimationSide is the largest width or height of the canvas.
	MaxAnimationSide = 16384
	// MaxAnimationFrames is the largest number of frames.
	MaxAnimationFrames = 4096
	// MaxAnimationPixels bounds the pixels of all frames together, each a
	// whole canvas: 2^25, which is 128 MiB of premultiplied RGBA. With their
	// mip levels, the frames of one animation fit in what the renderer keeps
	// on the GPU, so that a looping animation does not upload its frames
	// again on every loop.
	MaxAnimationPixels = 1 << 25
)

// LoopForever is the value of Animation.Loops for an animation that repeats
// without end.
const LoopForever = 0

// minFrameDelay and shortDelay give the convention of browsers for short
// delays: many files ask for 0 or 10 milliseconds and expect to be shown at
// 100, the rate of the browsers that made them.
const (
	minFrameDelay = 10 * time.Millisecond
	shortDelay    = 100 * time.Millisecond
)

// ErrAnimationTooLarge is wrapped by the error of DecodeAnimation for a file
// past one of its limits.
var ErrAnimationTooLarge = errors.New("paint: animation too large")

// errNoFrames is returned for an animation without frames.
var errNoFrames = errors.New("no frames")

// Animation is a sequence of pictures, each shown for its delay. Frames are
// whole pictures of the same size, with every disposal and blend of the file
// already applied, so that any frame can be drawn on its own.
type Animation struct {
	// Frames are the pictures in order.
	Frames []*Image
	// Delays holds how long each frame shows, one per frame.
	Delays []time.Duration
	// Loops is how many times the animation plays. LoopForever, the zero
	// value, repeats it without end.
	Loops int
}

// Size returns the size of the frames in pixels, and zero without frames.
func (a *Animation) Size() (width, height int) {
	if len(a.Frames) == 0 {
		return 0, 0
	}
	_, w, h := a.Frames[0].Pixels()
	return w, h
}

// Duration returns how long one play takes.
func (a *Animation) Duration() time.Duration {
	var total time.Duration
	for _, d := range a.Delays {
		total += max(d, 0)
	}
	return total
}

// FrameAt returns the frame that shows at time t after the start, the time
// after the start at which the next frame shows, and whether the animation is
// over at t. Once it is over, the last frame stays. An animation of one frame,
// or whose delays add up to zero, is over from the start. It panics if Delays
// does not hold one delay per frame. It does not allocate.
func (a *Animation) FrameAt(t time.Duration) (frame int, next time.Duration, ended bool) {
	if len(a.Delays) != len(a.Frames) {
		panic("gowaku: Animation needs one delay per frame")
	}
	total := a.Duration()
	last := len(a.Frames) - 1
	if last <= 0 || total <= 0 {
		return 0, 0, true
	}
	t = max(t, 0)
	play := t / total
	if a.Loops > 0 && play >= time.Duration(a.Loops) {
		return last, 0, true
	}
	start := play * total
	rest := t - start
	var end time.Duration
	for i, d := range a.Delays {
		end += max(d, 0)
		if rest < end {
			return i, start + end, false
		}
	}
	return last, start + total, false
}

// DecodeAnimation reads an animated GIF, APNG or animated WebP file and
// composites its frames. A still PNG, GIF, WebP or JPEG file becomes an
// animation of one frame.
//
// As browsers do, a frame delay of 10 milliseconds or less shows for 100
// milliseconds, and disposal to the background clears to transparent. A GIF
// file that loops n times plays n+1 times, which is what browsers do with it.
// The file is refused with an error wrapping ErrAnimationTooLarge past
// MaxAnimationBytes, MaxAnimationSide, MaxAnimationFrames or
// MaxAnimationPixels, and with another error if it is malformed.
func DecodeAnimation(r io.Reader) (*Animation, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxAnimationBytes+1))
	if err != nil {
		return nil, fmt.Errorf("paint: decode animation: %w", err)
	}
	if len(data) > MaxAnimationBytes {
		return nil, fmt.Errorf("paint: decode animation: file past %d bytes: %w", MaxAnimationBytes, ErrAnimationTooLarge)
	}
	a, err := decodeAnimation(data)
	if err != nil {
		return nil, fmt.Errorf("paint: decode animation: %w", err)
	}
	return a, nil
}

func decodeAnimation(data []byte) (*Animation, error) {
	switch {
	case bytes.HasPrefix(data, []byte(pngSignature)):
		return decodePNGAnimation(data)
	case bytes.HasPrefix(data, []byte("GIF8")):
		return decodeGIFAnimation(data)
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return decodeWebPAnimation(data)
	case bytes.HasPrefix(data, []byte{0xff, 0xd8}):
		return decodeStill(data, jpeg.DecodeConfig, jpeg.Decode)
	}
	return nil, errors.New("unknown format")
}

// checkSize refuses an animation of frames whole canvases of width by height
// past the limits.
func checkSize(width, height, frames int) error {
	switch {
	case width <= 0 || height <= 0:
		return fmt.Errorf("empty canvas %dx%d", width, height)
	case frames <= 0:
		return errNoFrames
	case width > MaxAnimationSide || height > MaxAnimationSide:
		return fmt.Errorf("canvas %dx%d past %d: %w", width, height, MaxAnimationSide, ErrAnimationTooLarge)
	case frames > MaxAnimationFrames:
		return fmt.Errorf("%d frames past %d: %w", frames, MaxAnimationFrames, ErrAnimationTooLarge)
	case int64(width)*int64(height)*int64(frames) > MaxAnimationPixels:
		return fmt.Errorf("%d frames of %dx%d past %d pixels: %w", frames, width, height, MaxAnimationPixels, ErrAnimationTooLarge)
	}
	return nil
}

// decodeStill reads a picture of one frame, checking its size first.
func decodeStill(data []byte, config func(io.Reader) (image.Config, error), decode func(io.Reader) (image.Image, error)) (*Animation, error) {
	cfg, err := config(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := checkSize(cfg.Width, cfg.Height, 1); err != nil {
		return nil, err
	}
	m, err := decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if b := m.Bounds(); b.Dx() != cfg.Width || b.Dy() != cfg.Height {
		return nil, fmt.Errorf("picture of %dx%d in a file of %dx%d", b.Dx(), b.Dy(), cfg.Width, cfg.Height)
	}
	return &Animation{Frames: []*Image{NewImage(m)}, Delays: []time.Duration{shortDelay}, Loops: 1}, nil
}

// browserDelay applies the convention of browsers to a delay of a file.
func browserDelay(d time.Duration) time.Duration {
	if d <= minFrameDelay {
		return shortDelay
	}
	return d
}

// dispose says what happens to the area of a frame before the next one.
type dispose uint8

const (
	// disposeNone leaves the frame on the canvas.
	disposeNone dispose = iota
	// disposeBackground clears the area of the frame to transparent.
	disposeBackground
	// disposePrevious puts back what the area held before the frame.
	disposePrevious
)

// compositor draws frames on a canvas and keeps a copy of each result.
type compositor struct {
	canvas *image.RGBA
	// saved holds the area of a frame that disposes to the previous picture,
	// as it was before the frame.
	saved *image.RGBA
	anim  Animation
}

func newCompositor(width, height, frames int) *compositor {
	c := &compositor{canvas: image.NewRGBA(image.Rect(0, 0, width, height))}
	c.anim.Frames = make([]*Image, 0, frames)
	c.anim.Delays = make([]time.Duration, 0, frames)
	return c
}

// add draws m into the area r of the canvas, blended over what is there or
// replacing it, records the canvas as a frame shown for delay, and then
// disposes of the area. m has the size of r. r lies in the canvas.
func (c *compositor) add(m image.Image, r image.Rectangle, over bool, disp dispose, delay time.Duration) {
	if disp == disposePrevious {
		if c.saved == nil {
			c.saved = image.NewRGBA(c.canvas.Bounds())
		}
		draw.Draw(c.saved, r, c.canvas, r.Min, draw.Src)
	}
	op := draw.Src
	if over {
		op = draw.Over
	}
	// image.RGBA is premultiplied, and draw converts straight alpha sources
	// such as image.NRGBA as it blends.
	draw.Draw(c.canvas, r, m, m.Bounds().Min, op)
	pix := make([]byte, len(c.canvas.Pix))
	copy(pix, c.canvas.Pix)
	b := c.canvas.Bounds()
	c.anim.Frames = append(c.anim.Frames, &Image{pix: pix, width: b.Dx(), height: b.Dy()})
	c.anim.Delays = append(c.anim.Delays, browserDelay(delay))
	switch disp {
	case disposeBackground:
		draw.Draw(c.canvas, r, image.Transparent, image.Point{}, draw.Src)
	case disposePrevious:
		draw.Draw(c.canvas, r, c.saved, r.Min, draw.Src)
	}
}

// result returns the animation, which loops loops times.
func (c *compositor) result(loops int) (*Animation, error) {
	if len(c.anim.Frames) == 0 {
		return nil, errNoFrames
	}
	c.anim.Loops = loops
	return &c.anim, nil
}
