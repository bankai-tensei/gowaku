package theme

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// readVectors returns the fields of each data line of a file in testdata. Every
// file there names the upstream source of its values in its header.
func readVectors(t *testing.T, name string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		rows = append(rows, strings.Fields(line))
	}
	if len(rows) == 0 {
		t.Fatalf("%s has no vectors", name)
	}
	return rows
}

func parseFloat(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func parseRGB(t *testing.T, s string) uint32 {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(v)
}
