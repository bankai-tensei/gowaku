package dbus

import "testing"

func TestPathOfURI(t *testing.T) {
	for _, tt := range []struct {
		uri  string
		want string
		ok   bool
	}{
		{"file:///home/user/a.txt", "/home/user/a.txt", true},
		{"file:///home/user/%E6%97%A5%E6%9C%AC%E8%AA%9E%20%E3%83%A1%E3%83%A2.txt", "/home/user/日本語 メモ.txt", true},
		{"file://localhost/tmp/x", "/tmp/x", true},
		{"file://server/share/x", "", false},
		{"sftp://host/x", "", false},
		{"not a uri%", "", false},
	} {
		got, ok := pathOfURI(tt.uri)
		if got != tt.want || ok != tt.ok {
			t.Errorf("pathOfURI(%q) = %q, %v; want %q, %v", tt.uri, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSchemeOfName(t *testing.T) {
	for name, want := range map[string]ColorScheme{
		"prefer-dark": PreferDark, "'prefer-light'": PreferLight, "default": NoPreference, "": NoPreference,
	} {
		if got := schemeOfName(name); got != want {
			t.Errorf("schemeOfName(%q) = %v, want %v", name, got, want)
		}
	}
}
