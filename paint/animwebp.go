package paint

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"time"

	"golang.org/x/image/webp"
)

// The flags of the VP8X chunk of a WebP file.
const (
	webpFlagAnimation = 0x02
	webpFlagAlpha     = 0x10
)

// The flags of an ANMF chunk.
const (
	anmfDisposeBackground = 0x01
	anmfNoBlend           = 0x02
)

// riffChunk is a chunk of a RIFF file.
type riffChunk struct {
	fourCC string
	body   []byte
}

// webpFrame is the bitstream of a frame of a WebP file: VP8L, or VP8 with an
// optional ALPH chunk.
type webpFrame struct {
	alpha, bitstream riffChunk
}

// decodeWebPAnimation reads an animated WebP file, or a still one as one
// frame. Each frame is wrapped as a WebP file of its own and decoded by
// x/image/webp, after the size in its bitstream is checked against the frame.
func decodeWebPAnimation(data []byte) (*Animation, error) {
	size := binary.LittleEndian.Uint32(data[4:])
	if uint64(size) < 4 || uint64(size) > uint64(len(data)-8) {
		return nil, errors.New("webp: bad RIFF size")
	}
	chunks, err := riffChunks(data[12 : 8+size])
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, errors.New("webp: no chunks")
	}
	first := chunks[0]
	switch first.fourCC {
	case "VP8 ", "VP8L":
		return decodeWebPStill(webpFrame{bitstream: first}, 0, 0)
	case "VP8X":
	default:
		return nil, fmt.Errorf("webp: unknown first chunk %q", first.fourCC)
	}
	if len(first.body) < 10 {
		return nil, errors.New("webp: short VP8X")
	}
	flags := first.body[0]
	width, height := 1+int(uint24(first.body[4:])), 1+int(uint24(first.body[7:]))
	if flags&webpFlagAnimation == 0 {
		fr, err := frameChunks(chunks[1:])
		if err != nil {
			return nil, err
		}
		return decodeWebPStill(fr, width, height)
	}
	return decodeWebPFrames(chunks[1:], width, height)
}

// decodeWebPStill decodes a still picture. A width and height of zero take
// the size of the bitstream.
func decodeWebPStill(fr webpFrame, width, height int) (*Animation, error) {
	w, h, err := fr.size()
	if err != nil {
		return nil, err
	}
	if width == 0 && height == 0 {
		width, height = w, h
	}
	if w != width || h != height {
		return nil, fmt.Errorf("webp: bitstream of %dx%d in a canvas of %dx%d", w, h, width, height)
	}
	if err := checkSize(width, height, 1); err != nil {
		return nil, err
	}
	_, m, err := fr.decodeInto(nil, width, height)
	if err != nil {
		return nil, err
	}
	c := newCompositor(width, height, 1)
	c.add(m, image.Rect(0, 0, width, height), false, disposeNone, shortDelay)
	return c.result(1)
}

func decodeWebPFrames(chunks []riffChunk, width, height int) (*Animation, error) {
	loops := -1
	frames := 0
	for _, c := range chunks {
		switch c.fourCC {
		case "ANIM":
			if len(c.body) < 6 {
				return nil, errors.New("webp: short ANIM")
			}
			// The background color in the first four bytes is a hint, which
			// browsers ignore: the canvas starts, and disposes, transparent.
			loops = int(binary.LittleEndian.Uint16(c.body[4:]))
		case "ANMF":
			frames++
		}
	}
	if loops < 0 {
		return nil, errors.New("webp: animation without ANIM")
	}
	if err := checkSize(width, height, frames); err != nil {
		return nil, err
	}
	canvas := image.Rect(0, 0, width, height)
	comp := newCompositor(width, height, frames)
	var scratch []byte
	for _, c := range chunks {
		if c.fourCC != "ANMF" {
			continue
		}
		if len(c.body) < 16 {
			return nil, errors.New("webp: short ANMF")
		}
		b := c.body
		x, y := 2*int(uint24(b)), 2*int(uint24(b[3:]))
		w, h := 1+int(uint24(b[6:])), 1+int(uint24(b[9:]))
		delay := time.Duration(uint24(b[12:])) * time.Millisecond
		flags := b[15]
		r := image.Rect(x, y, x+w, y+h)
		if !r.In(canvas) {
			return nil, fmt.Errorf("webp: frame at %v outside the canvas", r)
		}
		sub, err := riffChunks(b[16:])
		if err != nil {
			return nil, err
		}
		fr, err := frameChunks(sub)
		if err != nil {
			return nil, err
		}
		fw, fh, err := fr.size()
		if err != nil {
			return nil, err
		}
		if fw != w || fh != h {
			return nil, fmt.Errorf("webp: bitstream of %dx%d in a frame of %dx%d", fw, fh, w, h)
		}
		var m image.Image
		if scratch, m, err = fr.decodeInto(scratch, w, h); err != nil {
			return nil, err
		}
		disp := disposeNone
		if flags&anmfDisposeBackground != 0 {
			disp = disposeBackground
		}
		comp.add(m, r, flags&anmfNoBlend == 0, disp, delay)
	}
	return comp.result(loops)
}

// riffChunks splits b into chunks, each padded to an even length.
func riffChunks(b []byte) ([]riffChunk, error) {
	var out []riffChunk
	for len(b) > 0 {
		if len(b) < 8 {
			return nil, errors.New("webp: truncated chunk")
		}
		n := binary.LittleEndian.Uint32(b[4:])
		if uint64(n) > uint64(len(b)-8) {
			return nil, errors.New("webp: chunk past the end")
		}
		out = append(out, riffChunk{fourCC: string(b[:4]), body: b[8 : 8+n]})
		b = b[8+n:]
		if n%2 == 1 && len(b) > 0 {
			b = b[1:]
		}
	}
	return out, nil
}

// frameChunks finds the bitstream of a frame among chunks, with the ALPH
// chunk before it. Other chunks are left alone.
func frameChunks(chunks []riffChunk) (webpFrame, error) {
	var fr webpFrame
	for _, c := range chunks {
		switch c.fourCC {
		case "ALPH":
			if fr.alpha.fourCC == "" {
				fr.alpha = c
			}
		case "VP8 ", "VP8L":
			fr.bitstream = c
			if c.fourCC == "VP8L" {
				// A lossless bitstream carries its own alpha.
				fr.alpha = riffChunk{}
			}
			return fr, nil
		}
	}
	return fr, errors.New("webp: frame without a bitstream")
}

// size returns the size that the bitstream declares.
func (fr webpFrame) size() (width, height int, err error) {
	cfg, err := webp.DecodeConfig(bytes.NewReader(appendRIFF(nil, riffChunk{}, fr.bitstream)))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// decodeInto decodes the frame, of the given size, wrapped as a file of its
// own. It builds the file in scratch, which it returns for the next frame.
func (fr webpFrame) decodeInto(scratch []byte, width, height int) ([]byte, image.Image, error) {
	var vp8x riffChunk
	if fr.alpha.fourCC != "" {
		var body [10]byte
		body[0] = webpFlagAlpha
		putUint24(body[4:], uint32(width-1))
		putUint24(body[7:], uint32(height-1))
		vp8x = riffChunk{fourCC: "VP8X", body: body[:]}
	}
	scratch = appendRIFF(scratch[:0], vp8x, fr.alpha, fr.bitstream)
	m, err := webp.Decode(bytes.NewReader(scratch))
	if err != nil {
		return scratch, nil, err
	}
	if m.Bounds().Dx() != width || m.Bounds().Dy() != height {
		return scratch, nil, fmt.Errorf("webp: frame decodes to %v", m.Bounds())
	}
	return scratch, lossyToRGB(m), nil
}

// lossyToRGB converts the YCbCr picture of a lossy frame to RGB as libwebp
// does, with the BT.601 matrix of limited range. x/image/webp returns the
// samples, and Go converts them as JPEG does, with full range, which turns
// white into 235. Chroma is taken from the nearest sample, without the
// smoothing of libwebp. Other pictures are returned as they are.
func lossyToRGB(m image.Image) image.Image {
	var yc *image.YCbCr
	var alpha []byte
	alphaStride := 0
	switch p := m.(type) {
	case *image.YCbCr:
		yc = p
	case *image.NYCbCrA:
		yc, alpha, alphaStride = &p.YCbCr, p.A, p.AStride
	default:
		return m
	}
	b := yc.Rect
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			yi, ci := yc.YOffset(x, y), yc.COffset(x, y)
			luma := int(yc.Y[yi])
			u, v := int(yc.Cb[ci]), int(yc.Cr[ci])
			o := out.PixOffset(x, y)
			out.Pix[o] = webpClip(mulHi(luma, 19077) + mulHi(v, 26149) - 14234)
			out.Pix[o+1] = webpClip(mulHi(luma, 19077) - mulHi(u, 6419) - mulHi(v, 13320) + 8708)
			out.Pix[o+2] = webpClip(mulHi(luma, 19077) + mulHi(u, 33050) - 17685)
			out.Pix[o+3] = 255
			if alpha != nil {
				out.Pix[o+3] = alpha[(y-b.Min.Y)*alphaStride+(x-b.Min.X)]
			}
		}
	}
	return out
}

// mulHi and webpClip are MultHi and VP8Clip8 of libwebp (src/dsp/yuv.h), in
// fixed point with 6 fraction bits.
func mulHi(v, coeff int) int { return v * coeff >> 8 }

func webpClip(v int) byte {
	const mask = 256<<6 - 1
	switch {
	case v&^mask == 0:
		return byte(v >> 6)
	case v < 0:
		return 0
	}
	return 255
}

// appendRIFF appends a WebP file made of the chunks that have a fourCC.
func appendRIFF(b []byte, chunks ...riffChunk) []byte {
	start := len(b)
	b = append(b, "RIFF\x00\x00\x00\x00WEBP"...)
	for _, c := range chunks {
		if c.fourCC == "" {
			continue
		}
		b = append(b, c.fourCC...)
		b = binary.LittleEndian.AppendUint32(b, uint32(len(c.body)))
		b = append(b, c.body...)
		if len(c.body)%2 == 1 {
			b = append(b, 0)
		}
	}
	binary.LittleEndian.PutUint32(b[start+4:], uint32(len(b)-start-8))
	return b
}

func uint24(b []byte) uint32 { return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 }

func putUint24(b []byte, v uint32) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }
