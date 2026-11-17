package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// bom is the byte order mark of UTF-8.
const bom = "\xef\xbb\xbf"

// ReadDoc reads a UTF-8 text file. It fails on text that is not UTF-8.
//
// It takes the byte order mark off the text and turns CRLF line ends into LF,
// and the Doc remembers both. A file is CRLF when most of its line ends are.
func ReadDoc(path string) (Doc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Doc{}, err
	}
	return decodeDoc(path, b)
}

// decodeDoc turns the bytes of the file at path into a Doc.
func decodeDoc(path string, b []byte) (Doc, error) {
	d := Doc{Path: path}
	if bytes.HasPrefix(b, []byte(bom)) {
		d.BOM = true
		b = b[len(bom):]
	} else if bytes.HasPrefix(b, []byte{0xff, 0xfe}) || bytes.HasPrefix(b, []byte{0xfe, 0xff}) {
		return Doc{}, fmt.Errorf("store: %s is UTF-16 text; only UTF-8 is read", path)
	}
	if !utf8.Valid(b) {
		return Doc{}, fmt.Errorf("store: %s is not UTF-8 text", path)
	}
	s := string(b)
	crlf := strings.Count(s, "\r\n")
	if crlf > 0 {
		lf := strings.Count(s, "\n") - crlf
		d.CRLF = crlf > lf
		s = strings.ReplaceAll(s, "\r\n", "\n")
	}
	d.Text = s
	return d, nil
}

// WriteDoc writes d to d.Path, with its line ends and byte order mark, through
// a temporary file renamed into place, so that a crash leaves the old file or
// the new one, never half of it.
func WriteDoc(d Doc) error {
	if d.Path == "" {
		return errors.New("store: WriteDoc of a Doc without a path")
	}
	return writeAtomic(d.Path, encodeDoc(d))
}

// encodeDoc returns the bytes that WriteDoc writes for d.
func encodeDoc(d Doc) []byte {
	s := d.Text
	if d.CRLF {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
	}
	n := len(s)
	if d.BOM {
		n += len(bom)
	}
	b := make([]byte, 0, n)
	if d.BOM {
		b = append(b, bom...)
	}
	return append(b, s...)
}

// writeAtomic writes data to path through a temporary file in the same
// folder that it renames over path. The file keeps the mode of the file it
// replaces. os.Rename replaces an existing file on Windows too, where it is
// MoveFileEx with MOVEFILE_REPLACE_EXISTING.
func writeAtomic(path string, data []byte) error {
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	// The leading dot keeps the temporary file out of the notebook's list.
	f, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := fill(f, data); err != nil {
		os.Remove(tmp) //nolint:errcheck // the write error is the one worth reporting
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		os.Remove(tmp) //nolint:errcheck // the chmod error is the one worth reporting
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) //nolint:errcheck // the rename error is the one worth reporting
		return err
	}
	return nil
}

// fill writes data to f, flushes it to the disk and closes f.
func fill(f *os.File, data []byte) error {
	_, err := f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
