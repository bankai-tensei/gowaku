//go:build darwin && arm64

package cocoa

import (
	"strings"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
)

// ShowFileDialog implements platform.Window with NSOpenPanel and NSSavePanel,
// as a sheet on the window. done runs from the event loop when the sheet
// closes.
func (w *Window) ShowFileDialog(opts platform.FileDialogOptions, done func(paths []string, err error)) {
	w.p.libs.pool(func() {
		var panel objc.ID
		if opts.Save {
			panel = class("NSSavePanel").Send(sel("savePanel"))
			if opts.Name != "" {
				panel.Send(sel("setNameFieldStringValue:"), nsString(opts.Name))
			}
		} else {
			panel = class("NSOpenPanel").Send(sel("openPanel"))
			panel.Send(sel("setCanChooseFiles:"), !opts.Directory)
			panel.Send(sel("setCanChooseDirectories:"), opts.Directory)
			panel.Send(sel("setAllowsMultipleSelection:"), opts.Multiple)
		}
		if opts.Title != "" {
			panel.Send(sel("setMessage:"), nsString(opts.Title))
		}
		if opts.Folder != "" {
			url := class("NSURL").Send(sel("fileURLWithPath:isDirectory:"), nsString(opts.Folder), true)
			panel.Send(sel("setDirectoryURL:"), url)
		}
		if types := fileTypes(opts.Filters); len(types) > 0 {
			// The filters hold globs; a UTType stands for each extension, also
			// for one that the system does not know.
			allowed := make([]objc.ID, 0, len(types))
			for _, t := range types {
				if ut := class("UTType").Send(sel("typeWithFilenameExtension:"), nsString(t)); ut != 0 {
					allowed = append(allowed, ut)
				}
			}
			if len(allowed) > 0 {
				panel.Send(sel("setAllowedContentTypes:"), nsArray(allowed...))
			}
		}
		panel.Send(sel("retain"))
		block := objc.NewBlock(func(_ objc.Block, response int64) {
			var paths []string
			if response == NSModalResponseOK {
				if opts.Save {
					paths = append(paths, goString(panel.Send(sel("URL")).Send(sel("path"))))
				} else {
					urls := panel.Send(sel("URLs"))
					for i := range count(urls) {
						paths = append(paths, goString(at(urls, i).Send(sel("path"))))
					}
				}
			}
			panel.Send(sel("release"))
			done(paths, nil)
			w.p.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
		})
		panel.Send(sel("beginSheetModalForWindow:completionHandler:"), w.window, block)
		// The panel copied the block.
		block.Release()
	})
}

// fileTypes returns the extensions of the patterns of filters, such as "png"
// for "*.png". A pattern for every file, such as "*", lifts the limit.
func fileTypes(filters []platform.FileFilter) []string {
	var types []string
	for _, f := range filters {
		for _, pattern := range f.Patterns {
			ext, ok := strings.CutPrefix(pattern, "*.")
			if !ok || ext == "*" || ext == "" {
				return nil
			}
			types = append(types, ext)
		}
	}
	return types
}
