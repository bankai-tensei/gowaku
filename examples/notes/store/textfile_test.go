package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteDoc(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		text      string
		crlf, bom bool
		// back is what WriteDoc writes again; empty means file.
		back string
	}{
		{name: "empty", file: "", text: ""},
		{name: "lf", file: "a\nb\n", text: "a\nb\n"},
		{name: "no final newline", file: "a\nb", text: "a\nb"},
		{name: "crlf", file: "a\r\nb\r\n", text: "a\nb\n", crlf: true},
		{name: "bom", file: bom + "あ\n", text: "あ\n", bom: true},
		{name: "bom crlf", file: bom + "あ\r\nい\r\n", text: "あ\nい\n", crlf: true, bom: true},
		{name: "mostly crlf", file: "a\r\nb\r\nc\nd\r\n", text: "a\nb\nc\nd\n", crlf: true, back: "a\r\nb\r\nc\r\nd\r\n"},
		{name: "mostly lf", file: "a\nb\nc\r\nd\n", text: "a\nb\nc\nd\n", back: "a\nb\nc\nd\n"},
		{name: "tie is lf", file: "a\r\nb\n", text: "a\nb\n", back: "a\nb\n"},
		{name: "lone cr kept", file: "a\rb\n", text: "a\rb\n"},
		{name: "japanese", file: "｜漢字《かんじ》の\n文章", text: "｜漢字《かんじ》の\n文章"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.txt")
			if err := os.WriteFile(path, []byte(tt.file), 0o644); err != nil {
				t.Fatal(err)
			}
			d, err := ReadDoc(path)
			if err != nil {
				t.Fatal(err)
			}
			want := Doc{Path: path, Text: tt.text, CRLF: tt.crlf, BOM: tt.bom}
			if d != want {
				t.Fatalf("ReadDoc = %+v, want %+v", d, want)
			}
			if err := WriteDoc(d); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			back := tt.back
			if back == "" {
				back = tt.file
			}
			if string(b) != back {
				t.Fatalf("WriteDoc wrote %q, want %q", b, back)
			}
		})
	}
}

func TestReadDocRejects(t *testing.T) {
	tests := []struct {
		name, file, want string
	}{
		{"invalid utf8", "abc\xff\xfe", "not UTF-8"},
		{"shift_jis", "\x82\xa0\x82\xa2", "not UTF-8"},
		{"utf16le", "\xff\xfea\x00", "UTF-16"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.txt")
			if err := os.WriteFile(path, []byte(tt.file), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := ReadDoc(path)
			if err == nil {
				t.Fatal("ReadDoc succeeded")
			}
			if !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), path) {
				t.Fatalf("error %q does not name the file and %q", err, tt.want)
			}
		})
	}
	if _, err := ReadDoc(filepath.Join(t.TempDir(), "missing.txt")); !os.IsNotExist(err) {
		t.Fatalf("ReadDoc of a missing file: %v", err)
	}
}

func TestWriteDocNormalizesText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.txt")
	// A Text that already has a CRLF must not come out as CR CR LF.
	if err := WriteDoc(Doc{Path: path, Text: "a\r\nb\n", CRLF: true}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "a\r\nb\r\n" {
		t.Fatalf("wrote %q", b)
	}
	if err := WriteDoc(Doc{Text: "x"}); err == nil {
		t.Fatal("WriteDoc without a path succeeded")
	}
}

func TestWriteDocAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	for i, s := range []string{"first", "second, longer", "3"} {
		if err := WriteDoc(Doc{Path: path, Text: s}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		b, err := os.ReadFile(path)
		if err != nil || string(b) != s {
			t.Fatalf("write %d: read %q, %v", i, b, err)
		}
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("folder holds %v, want only a.txt", names)
	}
}

func TestWriteDocFailureLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	// The target is a folder, so the rename fails after the temporary file
	// is written.
	path := filepath.Join(dir, "a.txt")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteDoc(Doc{Path: path, Text: "x"}); err == nil {
		t.Fatal("WriteDoc over a folder succeeded")
	}
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("folder holds %d entries, want 1", len(ents))
	}
}
