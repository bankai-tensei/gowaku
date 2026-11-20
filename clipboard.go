package gowaku

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/platform"
)

// The clipboard beyond the text that field.Input copies and pastes. See
// docs/ADR/0049-clipboard.md.
type (
	// ClipboardContent is what the clipboard holds: text, an HTML fragment,
	// an image and a list of files, each there or not.
	ClipboardContent = core.ClipboardContent
	// ClipboardKinds is a set of the kinds of clipboard data.
	ClipboardKinds = core.ClipboardKinds
)

// The kinds of clipboard data.
const (
	ClipboardText  = core.ClipboardText
	ClipboardHTML  = core.ClipboardHTML
	ClipboardImage = core.ClipboardImage
	// ClipboardFiles is a list of files, as a file manager copies them.
	ClipboardFiles = core.ClipboardFiles
)

// The content crosses to the platform by conversion, so that a field added to
// one struct and not the other fails to build; the kinds share their bits,
// which TestClipboardKinds_MatchThePlatform checks.

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
		return ClipboardKinds(c.ClipboardKinds())
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
		return ClipboardContent(c.ReadClipboard(platform.ClipboardKinds(kinds)))
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
		clip.WriteClipboard(platform.ClipboardContent(c))
		return
	}
	h.SetClipboardText(c.Text)
}
