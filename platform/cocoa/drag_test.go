package cocoa

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestKindOfType_FilesAndText(t *testing.T) {
	for _, c := range []struct {
		typ  string
		want input.DataKinds
	}{
		{"public.file-url", input.DataFiles},
		{"public.utf8-plain-text", input.DataText},
		{"public.utf16-external-plain-text", input.DataText},
		{"public.png", 0},
		{"public.url", 0},
	} {
		if got := kindOfType(c.typ); got != c.want {
			t.Errorf("%s: got %v, want %v", c.typ, got, c.want)
		}
	}
}

func TestAllowedOf_GenericIsMove(t *testing.T) {
	for _, c := range []struct {
		name string
		mask uint64
		want input.DropEffect
	}{
		{"none", NSDragOperationNone, input.DropNone},
		{"Finder: copy, link, generic, move, delete", NSDragOperationCopy | NSDragOperationLink | NSDragOperationGeneric |
			NSDragOperationMove | NSDragOperationDelete, input.DropCopy | input.DropMove | input.DropLink},
		{"Option held: copy only", NSDragOperationCopy, input.DropCopy},
		{"Command held: generic only", NSDragOperationGeneric, input.DropMove},
		{"every", ^uint64(0), input.DropCopy | input.DropMove | input.DropLink},
	} {
		if got := allowedOf(c.mask); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOperationOf_StaysInMask(t *testing.T) {
	for _, c := range []struct {
		name   string
		effect input.DropEffect
		mask   uint64
		want   uint64
	}{
		{"copy", input.DropCopy, NSDragOperationCopy | NSDragOperationMove, NSDragOperationCopy},
		{"move", input.DropMove, NSDragOperationCopy | NSDragOperationMove, NSDragOperationMove},
		{"move as generic", input.DropMove, NSDragOperationCopy | NSDragOperationGeneric, NSDragOperationGeneric},
		{"link", input.DropLink, NSDragOperationLink, NSDragOperationLink},
		{"not allowed", input.DropMove, NSDragOperationCopy, NSDragOperationNone},
		{"refused", input.DropNone, ^uint64(0), NSDragOperationNone},
	} {
		if got := operationOf(c.effect, c.mask); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestMaskOf_EffectOf_RoundTrip(t *testing.T) {
	for _, e := range []input.DropEffect{input.DropCopy, input.DropMove, input.DropLink} {
		mask := maskOf(e)
		if got := allowedOf(mask); got != e {
			t.Errorf("allowedOf(maskOf(%v)) = %v", e, got)
		}
		if got := effectOf(operationOf(e, mask)); got != e {
			t.Errorf("effectOf(operationOf(%v)) = %v", e, got)
		}
	}
	if got := maskOf(input.DropMove); got&NSDragOperationGeneric == 0 {
		t.Errorf("a move does not offer Generic: %d", got)
	}
	if got := effectOf(NSDragOperationDelete); got != input.DropMove {
		t.Errorf("Trash: got %v, want move", got)
	}
	if got := effectOf(NSDragOperationNone); got != input.DropNone {
		t.Errorf("canceled: got %v", got)
	}
}
