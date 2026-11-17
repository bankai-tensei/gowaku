package text

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/fontscan"
)

// startupFile names the list of the fonts that the last SaveStartupFaces found
// loaded, in the cache directory of NewSystemShaper. Each program has its own,
// by the name of its executable, since each draws a first frame of its own.
func startupFile() string {
	name := "app"
	if exe, err := os.Executable(); err == nil {
		name = strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	}
	clean := strings.Map(func(r rune) rune {
		if r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return r
		}
		return '_'
	}, name)
	return "startup_faces_v1_" + clean + ".txt"
}

// startupFace is a font of the list: where it is, and the size and time of
// change of its file, which tell whether it is still the same.
type startupFace struct {
	location fontscan.Location
	size     int64
	modTime  int64
}

// preloaded is a font of the list that NewSystemShaper read in parallel with
// the index of the system fonts.
type preloaded struct {
	location fontscan.Location
	face     *font.Face
}

// readStartupFaces returns the list in path, without the fonts whose files
// changed since.
func readStartupFaces(path string) []startupFace {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // it was only read
	var out []startupFace
	in := bufio.NewScanner(f)
	for in.Scan() {
		fields := strings.Split(in.Text(), "\t")
		if len(fields) != 5 {
			return nil
		}
		index, err1 := strconv.ParseUint(fields[1], 10, 16)
		instance, err2 := strconv.ParseUint(fields[2], 10, 16)
		size, err3 := strconv.ParseInt(fields[3], 10, 64)
		modTime, err4 := strconv.ParseInt(fields[4], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			return nil
		}
		out = append(out, startupFace{
			location: fontscan.Location{File: fields[0], Index: uint16(index), Instance: uint16(instance)},
			size:     size, modTime: modTime,
		})
	}
	return out
}

// writeStartupFaces replaces the list in path.
func writeStartupFaces(path string, faces []startupFace) error {
	var b strings.Builder
	for _, f := range faces {
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\n", f.location.File, f.location.Index, f.location.Instance, f.size, f.modTime)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// statFace returns the entry of the list for the font at location, or false if
// its file cannot be read.
func statFace(location fontscan.Location) (startupFace, bool) {
	info, err := os.Stat(location.File)
	if err != nil {
		return startupFace{}, false
	}
	return startupFace{location: location, size: info.Size(), modTime: info.ModTime().UnixNano()}, true
}

// preload starts reading the fonts of list, each on a goroutine of its own, and
// returns a function that waits for them, in the order of list. A font whose
// file changed, or that fails to load, is left out.
func preload(list []startupFace) func() []preloaded {
	faces := make([]preloaded, len(list))
	var wg sync.WaitGroup
	for i, want := range list {
		if have, ok := statFace(want.location); !ok || have != want {
			continue
		}
		wg.Go(func() { faces[i] = preloaded{want.location, loadFace(want.location)} })
	}
	return func() []preloaded {
		wg.Wait()
		// The first face that the font map takes in is its last resort, so the
		// order must not depend on which file was read first.
		return slices.DeleteFunc(faces, func(p preloaded) bool { return p.face == nil })
	}
}

// loadFace reads the font at location the way fontscan does, or returns nil.
func loadFace(location fontscan.Location) *font.Face {
	f, err := os.Open(location.File)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // it was only read
	loaders, err := ot.NewLoaders(f)
	if err != nil || int(location.Index) >= len(loaders) {
		return nil
	}
	ft, err := font.NewFont(loaders[location.Index])
	if err != nil {
		return nil
	}
	return font.NewFace(ft)
}

// adopt hands the font map the fonts that NewSystemShaper read ahead, so that
// it finds them loaded where it would have read them itself. Each goes in as a
// face of the same location, family and aspect as the system font, which the
// font map then resolves to the same face.
func (s *Shaper) adopt(faces []preloaded, cacheDir string) {
	if len(faces) == 0 {
		return
	}
	system, err := fontscan.SystemFonts(discard{}, cacheDir)
	if err != nil {
		return
	}
	for _, p := range faces {
		i := slices.IndexFunc(system, func(fp fontscan.Footprint) bool { return fp.Location == p.location })
		if i < 0 {
			// Not a system font any more: adding it would change what the
			// font map picks.
			continue
		}
		s.fonts.AddFace(p.face, p.location, font.Description{Family: system[i].Family, Aspect: system[i].Aspect})
		s.adoptedFaces = append(s.adoptedFaces, p.face)
	}
}

// SaveStartupFaces remembers the fonts loaded so far, so that the next
// NewSystemShaper reads them in parallel with the index of the system fonts
// instead of the first frame reading them one after the other. Call it once the
// first frame is on the screen. A Shaper of given fonts ignores it. See
// docs/ADR/0080-startup.md.
func (s *Shaper) SaveStartupFaces() {
	if s.startupPath == "" {
		return
	}
	var list []startupFace
	for _, f := range s.faces {
		location := s.fonts.FontLocation(f.Font)
		if location.File == "" || slices.ContainsFunc(list, func(e startupFace) bool { return e.location == location }) {
			continue
		}
		if e, ok := statFace(location); ok {
			list = append(list, e)
		}
	}
	if !slices.Equal(list, s.startupFaces) {
		// The list only spares time; a cache directory that cannot be written
		// costs the time it would have spared.
		_ = writeStartupFaces(s.startupPath, list) //nolint:errcheck // see above
		s.startupFaces = list
	}
}
