// Command harness measures the programs of the benchmark from the outside, the
// same way for each. On macOS it does without root and without the
// accessibility permission: it starts the programs, which size and drive
// themselves, and reads CPU, GPU time and memory from the kernel. See
// ../README.md.
//
//	harness -bin dir -out results.json
//	harness -bin dir -libs gowaku -scenarios scroll -runs 1
//	harness -report results.json
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ebitengine/purego"
)

// rusageInfoV6 is struct rusage_info_v6 of <sys/resource.h>, with room to
// spare in case a later kernel writes more.
type rusageInfoV6 struct {
	UUID                       [16]uint8
	UserTime                   uint64
	SystemTime                 uint64
	PkgIdleWkups               uint64
	InterruptWkups             uint64
	Pageins                    uint64
	WiredSize                  uint64
	ResidentSize               uint64
	PhysFootprint              uint64
	ProcStartAbstime           uint64
	ProcExitAbstime            uint64
	ChildUserTime              uint64
	ChildSystemTime            uint64
	ChildPkgIdleWkups          uint64
	ChildInterruptWkups        uint64
	ChildPageins               uint64
	ChildElapsedAbstime        uint64
	DiskioBytesread            uint64
	DiskioByteswritten         uint64
	CPUTimeQOS                 [7]uint64
	BilledSystemTime           uint64
	ServicedSystemTime         uint64
	LogicalWrites              uint64
	LifetimeMaxPhysFootprint   uint64
	Instructions               uint64
	Cycles                     uint64
	BilledEnergy               uint64
	ServicedEnergy             uint64
	IntervalMaxPhysFootprint   uint64
	RunnableTime               uint64
	Flags                      uint64
	UserPtime                  uint64
	SystemPtime                uint64
	Pinstructions              uint64
	Pcycles                    uint64
	EnergyNJ                   uint64
	PenergyNJ                  uint64
	SecureTimeInSystem         uint64
	SecurePtimeInSystem        uint64
	NeuralFootprint            uint64
	LifetimeMaxNeuralFootprint uint64
	IntervalMaxNeuralFootprint uint64
	ConclaveFootprint          uint64
	PageWaitTimeMach           uint64
	PageCacheHits              uint64
	Reserved                   [6]uint64
	spare                      [64]uint64
}

const rusageInfoV6Flavor = 6

type machTimebase struct{ numer, denom uint32 }

var (
	procPidRusage func(pid int32, flavor int32, buf *rusageInfoV6) int32
	timebase      machTimebase
)

func init() {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		log.Fatal(err)
	}
	purego.RegisterLibFunc(&procPidRusage, lib, "proc_pid_rusage")
	var machTimebaseInfo func(*machTimebase) int32
	purego.RegisterLibFunc(&machTimebaseInfo, lib, "mach_timebase_info")
	machTimebaseInfo(&timebase)
}

// usage is what the harness reads of a process at one moment.
type usage struct {
	cpu           time.Duration
	energyNJ      uint64
	peakFootprint uint64
	gpu           time.Duration
	ok            bool
}

func read(pid int) usage {
	var ri rusageInfoV6
	if procPidRusage(int32(pid), rusageInfoV6Flavor, &ri) != 0 {
		return usage{}
	}
	// The times are in units of the Mach clock, which is not the nanosecond on
	// Apple silicon.
	ticks := ri.UserTime + ri.SystemTime
	return usage{
		cpu:           time.Duration(ticks * uint64(timebase.numer) / uint64(timebase.denom)),
		energyNJ:      ri.EnergyNJ,
		peakFootprint: ri.LifetimeMaxPhysFootprint,
		gpu:           gpuTime(pid),
		ok:            true,
	}
}

var gpuUsage = regexp.MustCompile(`"accumulatedGPUTime"=(\d+)`)

// gpuTime returns the GPU time that a process has used so far, in all its
// command queues, as the GPU driver counts it for Activity Monitor. ioreg
// reads it without root.
func gpuTime(pid int) time.Duration {
	out, err := exec.Command("ioreg", "-r", "-d", "1", "-w", "0", "-c", "AGXDeviceUserClient").Output()
	if err != nil {
		return 0
	}
	creator := []byte(fmt.Sprintf(`"IOUserClientCreator" = "pid %d,`, pid))
	var total time.Duration
	// Each user client is a block that starts with "+-o".
	for block := range bytes.SplitSeq(out, []byte("+-o ")) {
		if !bytes.Contains(block, creator) {
			continue
		}
		for _, m := range gpuUsage.FindAllSubmatch(block, -1) {
			ns, _ := strconv.ParseInt(string(m[1]), 10, 64)
			total += time.Duration(ns)
		}
	}
	return total
}

// measure runs one program through one scenario.
func measure(exe, lib, scenario string, length time.Duration, dir string) Result {
	r := Result{Lib: lib, Scenario: scenario, Seconds: length.Seconds(), OS: "darwin"}
	out := filepath.Join(dir, fmt.Sprintf("%s-%s.json", lib, scenario))
	_ = os.Remove(out) // left over from an earlier run, if at all
	total := warmup + length + 1500*time.Millisecond
	// Nothing outside the program can send it the wheel without the
	// accessibility permission, so every program moves its list by itself.
	cmd := exec.Command(exe, "-scenario", scenario, "-out", out, "-duration", total.String(), "-autoscroll")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		r.Failed = err.Error()
		return r
	}
	defer func() { _ = cmd.Process.Kill() }()
	ready := make(chan struct{})
	go func() {
		in := bufio.NewScanner(stdout)
		var once sync.Once
		for in.Scan() {
			if strings.Contains(in.Text(), "READY") {
				once.Do(func() { close(ready) })
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(15 * time.Second):
		r.Failed = "no first frame"
		return r
	}
	time.Sleep(warmup)

	pid := cmd.Process.Pid
	from := time.Now()
	u0 := read(pid)
	time.Sleep(length)
	to := time.Now()
	u1 := read(pid)
	if !u0.ok || !u1.ok {
		r.Failed = "proc_pid_rusage failed"
		return r
	}
	seconds := to.Sub(from).Seconds()
	r.CPUPercent = 100 * (u1.cpu - u0.cpu).Seconds() / seconds
	r.GPUPercent = 100 * (u1.gpu - u0.gpu).Seconds() / seconds
	r.ProcessWatts = float64(u1.energyNJ-u0.energyNJ) / 1e9 / seconds
	r.PeakFootprintMB = float64(u1.peakFootprint) / (1 << 20)

	if err := cmd.Wait(); err != nil {
		r.Failed = "exit: " + err.Error()
	}
	// ru_maxrss is in bytes on macOS: the peak resident size, as the peak
	// working set of Windows.
	if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		r.PeakMB = float64(ru.Maxrss) / (1 << 20)
	}
	readFrames(&r, out, from, to)
	return r
}

func main() {
	bin := flag.String("bin", "", "directory of gowaku, gio and cogent")
	outPath := flag.String("out", "", "file for the results")
	libs := flag.String("libs", "gowaku,gio,cogent", "programs to run")
	scenarios := flag.String("scenarios", "startup,scroll,animate,idle", "scenarios to run; resize needs a harness that can size other windows, which macOS allows only with the accessibility permission")
	runs := flag.Int("runs", 3, "how often to repeat each")
	stretch := flag.Int("stretch", 1, "multiply the length of every scenario, to look for leaks")
	reportPath := flag.String("report", "", "print this results file as a Markdown table and exit")
	flag.Parse()
	if *reportPath != "" {
		if err := report(*reportPath, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	lengths := map[string]time.Duration{
		"scroll": 10 * time.Second, "animate": 10 * time.Second,
		"idle":  60 * time.Second,
		"dense": 10 * time.Second, "dense-flat": 10 * time.Second,
	}
	dir, err := os.MkdirTemp("", "gowaku-bench")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	var results []Result
	for run := range *runs {
		for scenario := range strings.SplitSeq(*scenarios, ",") {
			if scenario != "startup" && lengths[scenario] == 0 {
				log.Printf("skipping %s: not measured on macOS", scenario)
				continue
			}
			for lib := range strings.SplitSeq(*libs, ",") {
				exe := filepath.Join(*bin, lib)
				var r Result
				if scenario == "startup" {
					for range 5 {
						r = startup(exe, lib)
						r.OS = "darwin"
						results = append(results, r)
					}
				} else {
					r = measure(exe, lib, scenario, lengths[scenario]*time.Duration(*stretch), dir)
					results = append(results, r)
				}
				line, _ := json.Marshal(r) // a Result always marshals
				fmt.Printf("run %d: %s\n", run+1, line)
				time.Sleep(time.Second)
			}
		}
		save(*outPath, results)
	}
}
