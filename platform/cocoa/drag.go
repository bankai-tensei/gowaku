package cocoa

import "github.com/bankai-tensei/gowaku/input"

// The bits of NSDragOperation, from NSDragging.h.
const (
	NSDragOperationNone    = 0
	NSDragOperationCopy    = 1
	NSDragOperationLink    = 2
	NSDragOperationGeneric = 4
	NSDragOperationPrivate = 8
	NSDragOperationMove    = 16
	NSDragOperationDelete  = 32
)

// The NSDraggingContext of a source: whether the drop target is in another
// application or in this one.
const (
	NSDraggingContextOutsideApplication = 0
	NSDraggingContextWithinApplication  = 1
)

// typeFileURL is the pasteboard type of a file URL, which Finder drags.
const typeFileURL = "public.file-url"

// kindOfType returns the kind of data that a pasteboard type holds, or none
// for a type that a drop does not read.
func kindOfType(t string) input.DataKinds {
	switch t {
	case typeFileURL:
		return input.DataFiles
	case "public.utf8-plain-text", "public.utf16-plain-text", "public.utf16-external-plain-text",
		"public.plain-text":
		return input.DataText
	}
	return 0
}

// allowedOf returns the effects that the operation mask of a drag source
// allows. Generic is the plain drag of Finder, which moves within a volume,
// so it counts as a move.
func allowedOf(mask uint64) input.DropEffect {
	var e input.DropEffect
	if mask&NSDragOperationCopy != 0 {
		e |= input.DropCopy
	}
	if mask&(NSDragOperationMove|NSDragOperationGeneric) != 0 {
		e |= input.DropMove
	}
	if mask&NSDragOperationLink != 0 {
		e |= input.DropLink
	}
	return e
}

// operationOf returns the operation that a destination answers for effect,
// within the mask of the source: Move if the source allows it, else Generic.
func operationOf(effect input.DropEffect, mask uint64) uint64 {
	switch {
	case effect&input.DropCopy != 0 && mask&NSDragOperationCopy != 0:
		return NSDragOperationCopy
	case effect&input.DropMove != 0 && mask&NSDragOperationMove != 0:
		return NSDragOperationMove
	case effect&input.DropMove != 0 && mask&NSDragOperationGeneric != 0:
		return NSDragOperationGeneric
	case effect&input.DropLink != 0 && mask&NSDragOperationLink != 0:
		return NSDragOperationLink
	}
	return NSDragOperationNone
}

// maskOf returns the operation mask of a drag source that allows the
// effects in allowed. A move offers Generic too, which Finder asks for.
func maskOf(allowed input.DropEffect) uint64 {
	var mask uint64
	if allowed&input.DropCopy != 0 {
		mask |= NSDragOperationCopy
	}
	if allowed&input.DropMove != 0 {
		mask |= NSDragOperationMove | NSDragOperationGeneric
	}
	if allowed&input.DropLink != 0 {
		mask |= NSDragOperationLink
	}
	return mask
}

// effectOf returns what the operation that ended a drag did, as the source
// sees it. A drop on the Trash, Delete, took the data away: a move.
func effectOf(op uint64) input.DropEffect {
	switch {
	case op&NSDragOperationCopy != 0:
		return input.DropCopy
	case op&(NSDragOperationMove|NSDragOperationGeneric|NSDragOperationDelete) != 0:
		return input.DropMove
	case op&NSDragOperationLink != 0:
		return input.DropLink
	}
	return input.DropNone
}
