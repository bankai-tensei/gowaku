package desktop

import "strings"

// Locale returns the language of the messages of the user as a BCP 47 tag,
// read from the environment the way gettext does: the first entry of LANGUAGE
// unless the locale is C, else the first of LC_ALL, LC_MESSAGES and LANG that is
// set. The C and POSIX locales, and an empty environment, read as "en".
// getenv is os.Getenv, or a fake in tests.
func Locale(getenv func(string) string) string {
	locale := ""
	for _, name := range [...]string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(name); v != "" {
			locale = v
			break
		}
	}
	tag := posixToBCP47(locale)
	if tag == "" {
		// gettext ignores LANGUAGE in the C locale too.
		return "en"
	}
	for _, entry := range strings.Split(getenv("LANGUAGE"), ":") {
		if t := posixToBCP47(entry); t != "" {
			return t
		}
	}
	return tag
}

// posixToBCP47 turns a POSIX locale name, language[_territory][.codeset][@modifier],
// into a BCP 47 tag: "ja_JP.UTF-8" is "ja-JP" and "sr_RS@latin" is
// "sr-Latn-RS". It returns "" for C, POSIX and names it cannot read.
func posixToBCP47(name string) string {
	modifier := ""
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name, modifier = name[:i], name[i+1:]
	}
	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}
	lang, region, _ := strings.Cut(name, "_")
	if lang == "" || lang == "C" || lang == "POSIX" || !letters(lang) || !letters(region) {
		return ""
	}
	tag := strings.ToLower(lang)
	switch modifier {
	case "latin":
		tag += "-Latn"
	case "cyrillic":
		tag += "-Cyrl"
	}
	if region != "" {
		tag += "-" + strings.ToUpper(region)
	}
	return tag
}

// letters reports whether s is made of ASCII letters only.
func letters(s string) bool {
	for i := range len(s) {
		c := s[i] | 0x20
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}
