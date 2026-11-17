// Command bigfile opens a file of any size at once: the editor reads only the
// lines it shows, from a LineSource that indexes the file on a goroutine of
// its own, and holds in memory only the lines that are edited. Ctrl+F
// searches the whole file without stopping the window.
//
//	bigfile path              opens the file
//	bigfile -gen 1024 path    first writes a log of 1024 MB to path
//	bigfile -report path      prints how long the file took and exits
//
// The source here only reads. Saving, following a log that grows and noticing
// a file changed on disk belong to the application; see
// docs/ADR/0157-editor-line-source.md.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/editor"
)

var (
	gen    = flag.Int("gen", 0, "write a log of this many MB to the path first")
	report = flag.Bool("report", false, "print how long the file took and exit")
	start  = time.Now()
)

type viewer struct {
	gowaku.StatefulBase
	path string
}

type viewerState struct {
	gowaku.StateBase
	ctl    editor.Controller
	src    *fileSource
	status string
	first  time.Duration
}

func (viewer) CreateState() gowaku.State { return &viewerState{} }

func (s *viewerState) InitState() {
	path := s.Widget().(viewer).path
	post := s.Poster()
	src, err := open(path, func(done bool, err error) {
		post(func() { s.found(done, err) })
	})
	if err != nil {
		log.Fatal(err)
	}
	s.src = src
	s.ctl.SetSource(src)
	s.status = "reading " + filepath.Base(path) + "…"
	// The first frame is drawn before the tick that runs this.
	s.After(0, func() { s.first = time.Since(start) })
}

// found takes what the indexing goroutine found, on the UI goroutine.
func (s *viewerState) found(done bool, err error) {
	from := s.src.apply()
	s.ctl.SourceGrew(from)
	s.SetState(func() {
		s.status = fmt.Sprintf("%d lines so far", s.src.n)
		if done {
			s.status = fmt.Sprintf("%d lines, read in %d ms; the first lines showed after %d ms",
				s.src.n, time.Since(start).Milliseconds(), s.first.Milliseconds())
		}
		if err != nil {
			s.status = "could not read the file: " + err.Error()
		}
	})
	if done && *report {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Printf("%s\nheap %d MB\n", s.status, m.HeapAlloc>>20)
		gowaku.Quit()
	}
}

func (s *viewerState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Column{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		basic.Expanded{Child: editor.Editor{Controller: &s.ctl, Label: s.Widget().(viewer).path, LineNumbers: true}},
		basic.Padding{Insets: layout.Insets{Left: 12, Right: 12, Top: 6, Bottom: 6}, Child: basic.Text{Content: s.status, Style: th.Text.Caption}},
	}}
}

// writeLog writes a log of about mb megabytes to path.
func writeLog(path string, mb int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	for i, size := 0, 0; size < mb<<20; i++ {
		level, status := "INFO ", 200
		if i%1000 == 0 {
			level, status = "ERROR", 503
		}
		n, err := fmt.Fprintf(w, "2026-11-18T10:%02d:%02d.%03dZ %s request %d handled path=/api/v1/items status=%d\n",
			i/60%60, i%60, i%1000, level, i, status)
		if err != nil {
			_ = f.Close() //nolint:errcheck // the error of the write is the one to report
			return err
		}
		size += n
	}
	if err := w.Flush(); err != nil {
		_ = f.Close() //nolint:errcheck // the error of the write is the one to report
		return err
	}
	return f.Close()
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: bigfile [-gen MB] [-report] path")
		os.Exit(2)
	}
	path := flag.Arg(0)
	if *gen > 0 {
		if err := writeLog(path, *gen); err != nil {
			log.Fatal(err)
		}
	}
	start = time.Now()
	err := gowaku.Run(gowaku.App{Title: "bigfile", Width: 1000, Height: 700, Home: viewer{path: path}})
	if err != nil {
		log.Fatal(err)
	}
}
