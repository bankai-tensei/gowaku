package wayland

import (
	"testing"
	"unsafe"
)

func TestInterfaceSet_MatchesTheLayoutOfLibwayland(t *testing.T) {
	var tables [2]Interface
	manager, item := &tables[0], &tables[1]
	manager.Set("example_manager", 2, []Message{
		{Name: "destroy"},
		{Name: "make", Signature: "n?s", Types: []*Interface{item, nil}},
	}, nil)
	item.Set("example_item", 1, nil, []Message{{Name: "done", Signature: "u", Types: []*Interface{nil}}})

	in := &manager.in
	if got := goString(uintptr(unsafe.Pointer(in.name))); got != "example_manager" {
		t.Errorf("name %q", got)
	}
	if in.version != 2 || in.methodCount != 2 || in.eventCount != 0 || in.events != nil {
		t.Errorf("header %+v", *in)
	}
	methods := unsafe.Slice(in.methods, in.methodCount)
	if methods[0].types != nil {
		t.Error("a message without arguments has types")
	}
	if got := goString(uintptr(unsafe.Pointer(methods[1].signature))); got != "n?s" {
		t.Errorf("signature %q", got)
	}
	types := unsafe.Slice(methods[1].types, 2)
	if types[0] != &item.in || types[1] != nil {
		t.Errorf("types %v, want the table of example_item and nil", types)
	}
}

func TestInterfaceSet_PanicsOnTypesThatMissTheSignature(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	var in Interface
	in.Set("example", 1, []Message{{Name: "make", Signature: "2uu", Types: []*Interface{nil}}}, nil)
}

func TestSignatureLetters(t *testing.T) {
	for s, want := range map[string]int{"": 0, "u": 1, "2n?sf": 3, "usun": 4, "10?o": 1} {
		if got := signatureLetters(s); got != want {
			t.Errorf("signatureLetters(%q) = %d, want %d", s, got, want)
		}
	}
}
