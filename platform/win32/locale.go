package win32

import "strings"

// localeTag turns a Windows locale name into a BCP 47 tag. The names are BCP 47
// already ("ja-JP", "sr-Latn-RS") except for an alternate sort order after an
// underscore ("de-DE_phoneb"), and the invariant and private locales, which
// read as "".
func localeTag(name string) string {
	if i := strings.IndexByte(name, '_'); i >= 0 {
		name = name[:i]
	}
	if name == "" || strings.HasPrefix(name, "!") || strings.HasPrefix(name, "x-") {
		return ""
	}
	return name
}
