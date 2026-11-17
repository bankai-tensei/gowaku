package win32

import (
	"testing"
	"unicode/utf16"

	"github.com/bankai-tensei/gowaku/input"
)

func TestBuildComposition_Clauses(t *testing.T) {
	units := utf16.Encode([]rune("日本語を変換"))
	attrs := []byte{
		ATTR_TARGET_CONVERTED, ATTR_TARGET_CONVERTED, ATTR_TARGET_CONVERTED, ATTR_TARGET_CONVERTED,
		ATTR_CONVERTED, ATTR_CONVERTED,
	}
	c := buildComposition(units, attrs, []uint32{0, 4, 6}, 4)
	if c.Text != "日本語を変換" {
		t.Errorf("got %q", c.Text)
	}
	if c.Cursor != 12 {
		t.Errorf("cursor %d, want the byte offset 12", c.Cursor)
	}
	want := []input.Clause{{Start: 0, End: 12, Kind: input.ClauseTarget}, {Start: 12, End: 18, Kind: input.ClauseConverted}}
	if len(c.Clauses) != 2 || c.Clauses[0] != want[0] || c.Clauses[1] != want[1] {
		t.Errorf("got %+v", c.Clauses)
	}
}

func TestBuildComposition_NoClausesYet(t *testing.T) {
	units := utf16.Encode([]rune("にほん"))
	c := buildComposition(units, []byte{0, 0, 0}, nil, 3)
	if len(c.Clauses) != 1 || c.Clauses[0] != (input.Clause{End: 9, Kind: input.ClauseInput}) {
		t.Errorf("got %+v", c.Clauses)
	}
	if c.Cursor != 9 {
		t.Errorf("cursor %d", c.Cursor)
	}
}

func TestBuildComposition_SurrogatePairs(t *testing.T) {
	// 𠮷 is two UTF-16 units and four UTF-8 bytes.
	units := utf16.Encode([]rune("𠮷野家"))
	if len(units) != 4 {
		t.Fatalf("got %d units", len(units))
	}
	c := buildComposition(units, []byte{1, 1, 2, 2}, []uint32{0, 2, 4}, 2)
	if c.Cursor != 4 {
		t.Errorf("cursor %d, want 4", c.Cursor)
	}
	if c.Clauses[0].End != 4 || c.Clauses[1].End != 10 {
		t.Errorf("got %+v", c.Clauses)
	}
}

func TestBuildComposition_OutOfRangeValues(t *testing.T) {
	units := utf16.Encode([]rune("あ"))
	c := buildComposition(units, nil, []uint32{0, 9}, -1)
	if c.Cursor != 3 || len(c.Clauses) != 0 {
		t.Errorf("got %+v", c)
	}
}

func TestClauseKind_Attributes(t *testing.T) {
	tests := []struct {
		attr byte
		want input.ClauseKind
	}{
		{ATTR_INPUT, input.ClauseInput},
		{ATTR_INPUT_ERROR, input.ClauseInput},
		{ATTR_TARGET_CONVERTED, input.ClauseTarget},
		{ATTR_TARGET_NOTCONVERTED, input.ClauseTarget},
		{ATTR_CONVERTED, input.ClauseConverted},
		{ATTR_FIXEDCONVERTED, input.ClauseConverted},
	}
	for _, tt := range tests {
		if got := clauseKind([]byte{tt.attr}, 0); got != tt.want {
			t.Errorf("attr %d: got %d, want %d", tt.attr, got, tt.want)
		}
	}
}
