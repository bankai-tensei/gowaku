package desktop

import "testing"

func TestLocale_Environment(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{"何も無い", nil, "en"},
		{"LANG だけ", map[string]string{"LANG": "ja_JP.UTF-8"}, "ja-JP"},
		{"LC_ALL が勝つ", map[string]string{"LC_ALL": "de_DE.UTF-8", "LC_MESSAGES": "fr_FR", "LANG": "ja_JP.UTF-8"}, "de-DE"},
		{"LC_MESSAGES が LANG に勝つ", map[string]string{"LC_MESSAGES": "fr_FR.UTF-8", "LANG": "ja_JP.UTF-8"}, "fr-FR"},
		{"C", map[string]string{"LANG": "C"}, "en"},
		{"C.UTF-8", map[string]string{"LANG": "C.UTF-8"}, "en"},
		{"POSIX", map[string]string{"LC_ALL": "POSIX"}, "en"},
		{"LANGUAGE が先", map[string]string{"LANG": "en_US.UTF-8", "LANGUAGE": "ja:en"}, "ja"},
		{"LANGUAGE の空の項目は飛ばす", map[string]string{"LANG": "en_US.UTF-8", "LANGUAGE": ":pt_BR"}, "pt-BR"},
		{"C のとき LANGUAGE は見ない", map[string]string{"LANG": "C", "LANGUAGE": "ja"}, "en"},
		{"地域なし", map[string]string{"LANG": "ja"}, "ja"},
		{"修飾子", map[string]string{"LANG": "sr_RS.UTF-8@latin"}, "sr-Latn-RS"},
		{"euro", map[string]string{"LANG": "de_DE@euro"}, "de-DE"},
		{"読めない名前", map[string]string{"LANG": "/usr/lib/locale/x"}, "en"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Locale(func(k string) string { return tc.env[k] })
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
