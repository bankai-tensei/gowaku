package gowaku

import (
	"errors"
	"testing"
)

func TestCheckURL(t *testing.T) {
	for _, c := range []struct {
		name string
		u    string
		ok   bool
	}{
		{"https", "https://example.com", true},
		{"パスと問い合わせ", "https://example.com/a/b?q=1#x", true},
		{"大文字のスキーム", "HTTPS://example.com", true},
		{"mailto", "mailto:someone@example.com", true},
		{"file", "file:///C:/docs/readme.md", true},
		{"独自のスキーム", "vscode://file/c:/x", true},
		{"空", "", false},
		{"スキームが無い", "example.com", false},
		{"絶対パス", "/docs/readme.md", false},
		{"スキーム相対", "//example.com/x", false},
		{"ドライブ文字", `C:\docs\readme.md`, false},
		{"ドライブ文字とスラッシュ", "C:/docs", false},
		{"スキームだけ", "https:", false},
		{"ホストが無い", "https:///x", false},
		{"空白", "https://example.com/a b", false},
		{"改行", "https://example.com/\nx", false},
		{"先頭のダッシュ", "-https://example.com", false},
		{"壊れたエスケープ", "https://example.com/%zz", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := checkURL(c.u)
			if (err == nil) != c.ok {
				t.Fatalf("checkURL(%q) = %v, want ok %v", c.u, err, c.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidURL) {
				t.Errorf("checkURL(%q) = %v, not ErrInvalidURL", c.u, err)
			}
		})
	}
}

func TestOpeners_FallBack(t *testing.T) {
	errPortal := errors.New("no portal")
	var got []string
	portal := func(u string) error { got = append(got, "portal "+u); return errPortal }
	xdg := func(u string) error { got = append(got, "xdg-open "+u); return nil }

	if err := (openers{portal, xdg}).open("https://example.com"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(got) != 2 || got[0] != "portal https://example.com" || got[1] != "xdg-open https://example.com" {
		t.Errorf("tried %q", got)
	}

	got = nil
	if err := (openers{xdg, portal}).open("https://example.com"); err != nil || len(got) != 1 {
		t.Errorf("open: %v after %q, want the first to take it", err, got)
	}

	errExec := errors.New("no xdg-open")
	fail := func(string) error { return errExec }
	err := (openers{portal, fail}).open("https://example.com")
	if !errors.Is(err, errPortal) || !errors.Is(err, errExec) {
		t.Errorf("got %v, want both errors", err)
	}

	if err := (openers{}).open("https://example.com"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("got %v, want ErrUnsupported", err)
	}
}

func TestOpenURL_RejectsBeforeOpening(t *testing.T) {
	if err := OpenURL("example.com"); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("got %v, want ErrInvalidURL", err)
	}
}
