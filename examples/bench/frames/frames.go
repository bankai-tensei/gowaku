// Package frames is what the three programs of the benchmark share: the clock
// that records frames, and the protocol with the harness.
package frames

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
)

// Rows is the length of the list of the scroll scenario.
const Rows = 10000

// ScrollSpeed is how fast the list of the scroll scenario moves, in logical pixels
// a second: 30 notches of the wheel of 100 pixels each.
const ScrollSpeed = 3000

// Boxes is the number of boxes of the animate scenario, in a square.
const (
	Boxes     = 100
	BoxesSide = 10
)

// Flags are the arguments that the harness passes to every program.
type Flags struct {
	// Scenario is scroll, animate, resize, idle or startup.
	Scenario string
	// Out receives the times of the frames.
	Out string
	// Duration is how long the program stays up.
	Duration time.Duration
	// AutoScroll makes gowaku move the list of the scroll scenario by itself at
	// ScrollSpeed, as Gio and Cogent Core always do, for a harness that cannot
	// send wheel input (macOS without the accessibility permission).
	AutoScroll bool
}

// Parse reads the arguments of the process.
func Parse() Flags {
	var f Flags
	flag.StringVar(&f.Scenario, "scenario", "scroll", "scroll, animate, resize, idle or startup")
	flag.StringVar(&f.Out, "out", "", "file for the times of the frames")
	flag.DurationVar(&f.Duration, "duration", 10*time.Second, "how long to run")
	flag.BoolVar(&f.AutoScroll, "autoscroll", false, "move the list by itself, without the wheel (gowaku)")
	flag.Parse()
	return f
}

// Recorder remembers when frames happened. Mark may run on any goroutine.
type Recorder struct {
	flags Flags
	times []int64
	next  atomic.Int64
	ready sync.Once
	// profile is the prefix of the profile files, if any.
	profile string
}

// New returns a recorder for a run. It ends the process when the run is over.
func New(f Flags) *Recorder {
	r := &Recorder{flags: f, times: make([]int64, 1<<20)}
	// BENCH_PROFILE=prefix writes prefix.cpu and prefix.heap, through the harness
	// as well, which passes its environment on.
	r.profile = os.Getenv("BENCH_PROFILE")
	if r.profile != "" {
		out, err := os.Create(r.profile + ".cpu")
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(out); err != nil {
			log.Fatal(err)
		}
	}
	go func() {
		time.Sleep(f.Duration)
		r.finish()
	}()
	return r
}

// Mark records a frame.
func (r *Recorder) Mark() {
	if i := r.next.Add(1) - 1; int(i) < len(r.times) {
		r.times[i] = time.Now().UnixNano()
	}
}

// Ready tells the harness that the first frame is on the screen. In the startup
// scenario that ends the run.
func (r *Recorder) Ready() {
	r.ready.Do(func() {
		fmt.Println("READY")
		if r.flags.Scenario == "startup" {
			os.Exit(0)
		}
	})
}

func (r *Recorder) finish() {
	if r.profile != "" {
		pprof.StopCPUProfile()
		out, err := os.Create(r.profile + ".heap")
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.Lookup("allocs").WriteTo(out, 0); err != nil {
			log.Fatal(err)
		}
	}
	n := min(int(r.next.Load()), len(r.times))
	if r.flags.Out != "" {
		data, err := json.Marshal(r.times[:n])
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(r.flags.Out, data, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	os.Exit(0)
}

// Label returns the text of row i, the same in every program.
func Label(i int) string { return fmt.Sprintf("%d 行目の項目 Item number %d", i+1, i+1) }

// Wave returns the phase of the animate scenario at a time since its start: it
// goes from 0 to 1 and back in two seconds.
func Wave(t time.Duration) float32 {
	const period = 2 * time.Second
	x := float32(t%period) / float32(period)
	if x > 0.5 {
		return 2 - 2*x
	}
	return 2 * x
}
