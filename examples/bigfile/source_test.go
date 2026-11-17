package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openAll opens content as a file and waits for it to be indexed.
func openAll(t *testing.T, content string) *fileSource {
	t.Helper()
	p := filepath.Join(t.TempDir(), "log.txt")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	s, err := open(p, func(d bool, err error) {
		if d {
			done <- err
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.f.Close() }) //nolint:errcheck // only read
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	s.apply()
	return s
}

func TestFileSource_LinesOffsetsAndReader(t *testing.T) {
	var b strings.Builder
	for i := range 1000 {
		nl := "\n"
		if i%3 == 0 {
			nl = "\r\n"
		}
		fmt.Fprintf(&b, "line %d %s%s", i, strings.Repeat("x", i%7), nl)
	}
	b.WriteString("last")
	for _, content := range []string{b.String(), "", "one", "a\r\nb\n"} {
		s := openAll(t, content)
		want := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
		if s.Len() != len(want) {
			t.Fatalf("%d lines, want %d", s.Len(), len(want))
		}
		off := int64(0)
		for i, l := range want {
			if got := s.Line(i); got != l {
				t.Fatalf("line %d = %q, want %q", i, got, l)
			}
			if got := s.Offset(i); got != off {
				t.Fatalf("Offset(%d) = %d, want %d", i, got, off)
			}
			if got := s.LineAt(off); got != i {
				t.Fatalf("LineAt(%d) = %d, want %d", off, got, i)
			}
			off += int64(len(l)) + 1
		}
		if got := s.Offset(s.Len()); got != off {
			t.Errorf("Offset(Len) = %d, want %d", got, off)
		}
		r := s.Reader()
		var read []string
		if err := r.ReadLines(0, s.Len(), func(_ int, l string) bool {
			read = append(read, l)
			return true
		}); err != nil {
			t.Fatal(err)
		}
		if strings.Join(read, "\n") != strings.Join(want, "\n") {
			t.Errorf("the reader read %d lines differently", len(read))
		}
	}
}
