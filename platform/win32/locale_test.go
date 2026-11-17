package win32

import "testing"

func TestLocaleTag_WindowsNames(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"ja-JP", "ja-JP"},
		{"en-US", "en-US"},
		{"sr-Latn-RS", "sr-Latn-RS"},
		{"de-DE_phoneb", "de-DE"},
		{"zh-CN_stroke", "zh-CN"},
		{"", ""},
		{"!x-sys-default-locale", ""},
		{"x-IV_mathan", ""},
	} {
		if got := localeTag(tc.name); got != tc.want {
			t.Errorf("localeTag(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
