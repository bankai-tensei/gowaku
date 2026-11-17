package hyphen

import "testing"

func TestEnglish(t *testing.T) {
	en := English()
	tests := []struct{ word, want string }{
		{"hyphenation", "hy-phen-ation"},
		{"typesetting", "type-set-ting"},
		{"computer", "com-puter"}, // "er" is shorter than three letters
		{"table", "ta-ble"},       // an exception
		{"project", "project"},    // an exception without points
		{"Associate", "As-so-ciate"},
		{"cat", "cat"},             // too short
		{"abc123def", "abc123def"}, // not only letters
	}
	for _, tt := range tests {
		if got := en.Hyphenate(tt.word); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.word, got, tt.want)
		}
	}
}

func TestParse_LeftAndRightMinimums(t *testing.T) {
	// One pattern allows a break between every pair of letters.
	p, err := Parse("a1a", "", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Hyphenate("aaaaaaa"); got != "aa-a-a-aaa" {
		t.Errorf("got %s", got)
	}
	if got := p.Points("aaaaaaa", nil); got[0] != 2 || got[len(got)-1] != 4 {
		t.Errorf("points %v, want from 2 to 4", got)
	}
}

func TestParse_Errors(t *testing.T) {
	if _, err := Parse("12", "", 2, 2); err == nil {
		t.Error("a pattern of digits only must fail")
	}
}
