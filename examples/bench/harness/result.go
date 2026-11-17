//go:build windows || darwin

package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// warmup is the start of every scenario but resize that the numbers leave out.
const warmup = 2 * time.Second

// Result is one run.
type Result struct {
	Lib, Scenario string
	Seconds       float64
	CPUPercent    float64 // of one core
	GPUPercent    float64
	Watts         float64
	Adapters      []string
	PeakMB        float64
	Frames        int
	FPS           float64
	P50ms, P99ms  float64
	WorstMs       float64
	OverBudget    int // intervals over two refreshes
	Resizes       int
	MaxClientW    int32
	MaxClientH    int32
	StartupMs     float64
	Failed        string `json:",omitempty"`
	// OS is set by the harnesses other than the one for Windows, whose files
	// came first and leave it out.
	OS string `json:",omitempty"`
	// PeakFootprintMB is the largest physical footprint of the process, which
	// is what macOS calls its memory: private memory, compressed or not, and
	// the GPU memory that the process owns. macOS only.
	PeakFootprintMB float64 `json:",omitempty"`
	// ProcessWatts is the energy that the kernel bills the CPU time of the
	// process with, over the length of the scenario. It is an estimate of
	// the CPU only, and not of the machine as Watts is. macOS only.
	ProcessWatts float64 `json:",omitempty"`
}

func frameStats(r *Result, times []int64, from, to time.Time) {
	var in []int64
	for _, t := range times {
		if t >= from.UnixNano() && t < to.UnixNano() {
			in = append(in, t)
		}
	}
	r.Frames = len(in)
	r.FPS = float64(len(in)) / to.Sub(from).Seconds()
	if len(in) < 2 {
		return
	}
	gaps := make([]float64, 0, len(in)-1)
	for i := 1; i < len(in); i++ {
		gaps = append(gaps, float64(in[i]-in[i-1])/1e6)
	}
	slices.Sort(gaps)
	r.P50ms = gaps[len(gaps)/2]
	r.P99ms = gaps[len(gaps)*99/100]
	r.WorstMs = gaps[len(gaps)-1]
	for _, g := range gaps {
		if g > 2000.0/60 {
			r.OverBudget++
		}
	}
}

// startup returns the time from starting the process to its first frame.
func startup(exe, lib string) Result {
	r := Result{Lib: lib, Scenario: "startup"}
	cmd := exec.Command(exe, "-scenario", "startup", "-duration", "30s")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	begin := time.Now()
	if err := cmd.Start(); err != nil {
		r.Failed = err.Error()
		return r
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	in := bufio.NewScanner(stdout)
	for in.Scan() {
		if strings.Contains(in.Text(), "READY") {
			r.StartupMs = float64(time.Since(begin).Microseconds()) / 1000
			return r
		}
	}
	r.Failed = "exited without a first frame"
	return r
}

// readFrames reads the times of the frames that a program wrote, and computes
// their statistics between from and to.
func readFrames(r *Result, path string, from, to time.Time) {
	data, err := os.ReadFile(path)
	if err != nil {
		r.Failed = "no frames: " + err.Error()
		return
	}
	var times []int64
	if err := json.Unmarshal(data, &times); err != nil {
		r.Failed = err.Error()
		return
	}
	frameStats(r, times, from, to)
}

// save writes the results so far.
func save(path string, results []Result) {
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(results, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatal(err)
	}
}
