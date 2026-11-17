package platform

import "image"

// ClipboardKinds is a set of the kinds of data on the clipboard.
type ClipboardKinds uint8

// The kinds of clipboard data.
const (
	ClipboardText ClipboardKinds = 1 << iota
	// ClipboardHTML is a fragment of HTML, as a browser copies it.
	ClipboardHTML
	ClipboardImage
	// ClipboardFiles is a list of files, as a file manager copies them.
	ClipboardFiles
)

// ClipboardContent is what the clipboard holds, in the forms gowaku knows. A
// zero field is a kind that is not there. See docs/ADR/0049-clipboard.md.
type ClipboardContent struct {
	Text string
	// HTML is a fragment, without the html and body elements.
	HTML  string
	Image *image.NRGBA
	// Files are absolute paths.
	Files []string
}

// Kinds returns the kinds that c holds.
func (c *ClipboardContent) Kinds() ClipboardKinds {
	var k ClipboardKinds
	if c.Text != "" {
		k |= ClipboardText
	}
	if c.HTML != "" {
		k |= ClipboardHTML
	}
	if c.Image != nil {
		k |= ClipboardImage
	}
	if len(c.Files) > 0 {
		k |= ClipboardFiles
	}
	return k
}

// Clipboard is the clipboard of the OS beyond text. A Window implements it.
type Clipboard interface {
	// ClipboardKinds reports what kinds the clipboard holds now, without
	// reading them.
	ClipboardKinds() ClipboardKinds
	// ReadClipboard reads the kinds asked for, of those that are there. A kind
	// whose owner does not answer in time comes back empty.
	ReadClipboard(kinds ClipboardKinds) ClipboardContent
	// WriteClipboard replaces the clipboard with every kind that c holds, so
	// that each program takes the form it understands.
	WriteClipboard(c ClipboardContent)
}
