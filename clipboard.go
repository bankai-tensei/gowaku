package gowaku

import "github.com/bankai-tensei/gowaku/platform"

// The clipboard beyond the text that field.Input copies and pastes. See
// docs/ADR/0049-clipboard.md.
type (
	// ClipboardContent is platform.ClipboardContent: text, an HTML fragment,
	// an image and a list of files, each there or not.
	ClipboardContent = platform.ClipboardContent
	// ClipboardKinds is platform.ClipboardKinds, a set of kinds.
	ClipboardKinds = platform.ClipboardKinds
)

// The kinds of clipboard data.
const (
	ClipboardText  = platform.ClipboardText
	ClipboardHTML  = platform.ClipboardHTML
	ClipboardImage = platform.ClipboardImage
	ClipboardFiles = platform.ClipboardFiles
)

// ClipboardHas reports what kinds the clipboard holds, without reading them:
// whether a Paste can be enabled. It must be called on the UI goroutine, outside
// a Build.
//
// The clipboard functions go through the core.Host of the tree of ctx, the
// one that RenderBase.Host and BuildContext.Host return, as the copy and paste
// of field.Input do. In a window of Run that is the clipboard of the OS; a
// host that implements platform.Clipboard, as that of a gowakutest.Tester,
// takes every kind, and any other host only the text.
func ClipboardHas(ctx BuildContext) ClipboardKinds {
	h := ctx.Host()
	if c, ok := h.(platform.Clipboard); ok {
		return c.ClipboardKinds()
	}
	if h.ClipboardText() != "" {
		return ClipboardText
	}
	return 0
}

// ReadClipboard reads the kinds asked for, of those that the clipboard holds.
// Another program owns the data, so this waits for it, up to a fraction of a
// second on Linux. It must be called on the UI goroutine, outside a Build.
//
//	c := gowaku.ReadClipboard(ctx, gowaku.ClipboardImage|gowaku.ClipboardFiles)
//	if c.Image != nil { ... }
func ReadClipboard(ctx BuildContext, kinds ClipboardKinds) ClipboardContent {
	h := ctx.Host()
	if c, ok := h.(platform.Clipboard); ok {
		return c.ReadClipboard(kinds)
	}
	if kinds&ClipboardText != 0 {
		return ClipboardContent{Text: h.ClipboardText()}
	}
	return ClipboardContent{}
}

// WriteClipboard replaces the clipboard with every kind that c holds, so that
// each program takes the form it understands: an image goes as PNG and as a
// bitmap, files as a list that Explorer, Nautilus and Dolphin paste. It must be
// called on the UI goroutine, outside a Build.
func WriteClipboard(ctx BuildContext, c ClipboardContent) {
	h := ctx.Host()
	if clip, ok := h.(platform.Clipboard); ok {
		clip.WriteClipboard(c)
		return
	}
	h.SetClipboardText(c.Text)
}
