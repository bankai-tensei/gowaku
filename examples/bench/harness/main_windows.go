// Command harness measures the programs of the benchmark from the outside, the
// same way for each: it starts them, drives their windows with posted messages,
// and reads CPU, GPU and memory from the OS. See ../README.md.
//
//	harness -bin dir -out results.json
//	harness -bin dir -libs gowaku -scenarios scroll -runs 1
//	harness -report results.json
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// clientWidth and clientHeight are the size of the window in physical pixels,
	// which is 800x700 at the 150% of the test machine.
	clientWidth, clientHeight = 1200, 1050

	wmMouseMove  = 0x0200
	wmMouseWheel = 0x020A

	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpNoMove     = 0x0002
	hwndTopmost   = ^uintptr(0)

	smCXScreen = 0
	smCYScreen = 1
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	psapi                    = windows.NewLazySystemDLL("psapi.dll")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procSetWindowPos         = user32.NewProc("SetWindowPos")
	procPostMessage          = user32.NewProc("PostMessageW")
	procGetClientRect        = user32.NewProc("GetClientRect")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procClientToScreen       = user32.NewProc("ClientToScreen")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
	procSetDPIAwareness      = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
)

type rect struct{ left, top, right, bottom int32 }

type point struct{ x, y int32 }

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// search is the state of enumWindow. Windows callbacks are a limited resource of
// the Go runtime, so there is one for the whole process.
var search struct {
	pid   uint32
	found uintptr
}

var enumWindow = windows.NewCallback(func(hwnd, _ uintptr) uintptr {
	var owner uint32
	// owner lives on this stack for the length of the call.
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
	if owner == search.pid {
		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible != 0 {
			search.found = hwnd
			return 0
		}
	}
	return 1
})

// findWindow returns the first visible top-level window of a process.
func findWindow(pid uint32) uintptr {
	search.pid, search.found = pid, 0
	procEnumWindows.Call(enumWindow, 0)
	return search.found
}

// awayFromCursor moves a window to the corner of the desktop that is farthest from
// the mouse, whose real position would otherwise cause hover effects in the idle
// scenario.
func awayFromCursor(hwnd uintptr) {
	var at point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&at)))
	screenW, _, _ := procGetSystemMetrics.Call(smCXScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCYScreen)
	var outer rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&outer)))
	x, y := int32(0), int32(0)
	if at.x < int32(screenW)/2 {
		x = int32(screenW) - (outer.right - outer.left)
	}
	if at.y < int32(screenH)/2 {
		y = int32(screenH) - (outer.bottom - outer.top)
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
}

func clientSize(hwnd uintptr) (w, h int32) {
	var r rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r.right - r.left, r.bottom - r.top
}

// setClient gives a window the client size w x h. flags go to SetWindowPos.
func setClient(hwnd uintptr, w, h int32, insertAfter, flags uintptr) {
	var outer rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&outer)))
	cw, ch := clientSize(hwnd)
	frameW := outer.right - outer.left - cw
	frameH := outer.bottom - outer.top - ch
	procSetWindowPos.Call(hwnd, insertAfter, 0, 0, uintptr(w+frameW), uintptr(h+frameH), flags)
}

func cpuTime(p windows.Handle) time.Duration {
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(p, &created, &exited, &kernel, &user); err != nil {
		return 0
	}
	return time.Duration(kernel.Nanoseconds() + user.Nanoseconds())
}

func peakMemory(p windows.Handle) uint64 {
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	procGetProcessMemoryInfo.Call(uintptr(p), uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
	return uint64(c.peakWorkingSetSize)
}

// gpuSampler reads the 3D engines of a process, and the package power of the
// machine, once a second with typeperf.
type gpuSampler struct {
	cmd  *exec.Cmd
	path string
}

func startSampler(pid int, seconds int, dir string) (*gpuSampler, error) {
	path := filepath.Join(dir, fmt.Sprintf("gpu-%d.csv", pid))
	engines := fmt.Sprintf(`\GPU Engine(pid_%d_*engtype_3D)\Utilization Percentage`, pid)
	cmd := exec.Command("typeperf", engines, `\Energy Meter(rapl_package0_pkg)\Power`,
		"-si", "1", "-sc", strconv.Itoa(seconds), "-f", "CSV", "-o", path, "-y")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &gpuSampler{cmd: cmd, path: path}, nil
}

// finish returns the mean use of the 3D engines in percent, the mean package
// power in watts, and the adapters that the process used.
func (g *gpuSampler) finish() (gpuPercent, watts float64, adapters []string) {
	_ = g.cmd.Wait() // a process that exited first ends typeperf with an error
	f, err := os.Open(g.path)
	if err != nil {
		return math.NaN(), math.NaN(), nil
	}
	defer f.Close()
	rows, err := csv.NewReader(bufio.NewReader(f)).ReadAll()
	if err != nil || len(rows) < 2 {
		return math.NaN(), math.NaN(), nil
	}
	seen := map[string]float64{}
	var gpuSum, powerSum float64
	var gpuRows, powerRows int
	header := rows[0]
	for _, row := range rows[1:] {
		rowGPU, any := 0.0, false
		for i := 1; i < len(row) && i < len(header); i++ {
			v, err := strconv.ParseFloat(strings.TrimSpace(row[i]), 64)
			if err != nil {
				continue
			}
			name := strings.ToLower(header[i])
			if strings.Contains(name, "energy meter") {
				powerSum += v / 1000
				powerRows++
				continue
			}
			rowGPU += v
			any = true
			if at := strings.Index(name, "luid_"); at >= 0 {
				seen[name[at:at+27]] += v
			}
		}
		if any {
			gpuSum += rowGPU
			gpuRows++
		}
	}
	for k, v := range seen {
		if v > 0 {
			adapters = append(adapters, k)
		}
	}
	slices.Sort(adapters)
	if gpuRows > 0 {
		gpuPercent = gpuSum / float64(gpuRows)
	}
	if powerRows > 0 {
		watts = powerSum / float64(powerRows)
	}
	return gpuPercent, watts, adapters
}

// wheel scrolls a window down, 30 notches a second, until stop closes.
func wheel(hwnd uintptr, stop <-chan struct{}) {
	var at point
	at.x, at.y = clientWidth/2, clientHeight/2
	client := uintptr(at.y)<<16 | uintptr(at.x)
	procPostMessage.Call(hwnd, wmMouseMove, 0, client)
	procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&at)))
	screen := uintptr(uint32(at.y))<<16 | uintptr(uint32(at.x))
	down := uintptr(0xff88) << 16 // -120 in the high word: one notch toward the user
	tick := time.NewTicker(time.Second / 30)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			procPostMessage.Call(hwnd, wmMouseWheel, down, screen)
		}
	}
}

// sweep resizes a window as fast as it answers, between 800x600 and the largest
// that the machine allows. SetWindowPos waits for the window procedure, so the
// count says how many sizes the program took.
func sweep(hwnd uintptr, stop <-chan struct{}, count *atomic.Int64, largest *[2]atomic.Int32) {
	const steps = 60
	// Windows caps a window at about the size of the desktop. Going on past that
	// would only pause the test.
	setClient(hwnd, 3840, 2160, 0, swpNoMove|swpNoZOrder|swpNoActivate)
	maxW, maxH := clientSize(hwnd)
	largest[0].Store(maxW)
	largest[1].Store(maxH)
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		default:
		}
		x := float64(i%(2*steps)) / steps
		if x > 1 {
			x = 2 - x
		}
		w := 800 + int32(x*float64(maxW-800))
		h := 600 + int32(x*float64(maxH-600))
		setClient(hwnd, w, h, 0, swpNoMove|swpNoZOrder|swpNoActivate)
		count.Add(1)
		time.Sleep(time.Second / 120)
	}
}

// measure runs one program through one scenario.
func measure(exe, lib, scenario string, length time.Duration, dir string) Result {
	r := Result{Lib: lib, Scenario: scenario, Seconds: length.Seconds()}
	out := filepath.Join(dir, fmt.Sprintf("%s-%s.json", lib, scenario))
	_ = os.Remove(out) // left over from an earlier run, if at all
	total := warmup + length + 1500*time.Millisecond
	cmd := exec.Command(exe, "-scenario", scenario, "-out", out, "-duration", total.String())
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
	var hwnd uintptr
	for deadline := time.Now().Add(15 * time.Second); hwnd == 0 && time.Now().Before(deadline); {
		hwnd = findWindow(uint32(cmd.Process.Pid))
		time.Sleep(10 * time.Millisecond)
	}
	if hwnd == 0 {
		r.Failed = "no window"
		return r
	}
	// On top without taking the focus: an occluded window may stop presenting.
	setClient(hwnd, clientWidth, clientHeight, hwndTopmost, swpNoMove|swpNoActivate)
	awayFromCursor(hwnd)
	select {
	case <-ready:
	case <-time.After(15 * time.Second):
		r.Failed = "no first frame"
		return r
	}
	proc, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(cmd.Process.Pid))
	if err != nil {
		r.Failed = err.Error()
		return r
	}
	defer windows.CloseHandle(proc)

	stop := make(chan struct{})
	var resizes atomic.Int64
	var largest [2]atomic.Int32
	var drivers sync.WaitGroup
	switch scenario {
	case "scroll":
		drivers.Go(func() { wheel(hwnd, stop) })
	case "resize":
		drivers.Go(func() { sweep(hwnd, stop, &resizes, &largest) })
	}
	time.Sleep(warmup)

	sampler, err := startSampler(cmd.Process.Pid, int(length.Seconds()), dir)
	if err != nil {
		r.Failed = err.Error()
		return r
	}
	from := time.Now()
	cpu0 := cpuTime(proc)
	resizes0 := resizes.Load()
	time.Sleep(length)
	to := time.Now()
	cpu1 := cpuTime(proc)
	r.Resizes = int(resizes.Load() - resizes0)
	r.PeakMB = float64(peakMemory(proc)) / (1 << 20)
	close(stop)
	drivers.Wait()
	r.CPUPercent = 100 * float64(cpu1-cpu0) / float64(to.Sub(from))
	r.GPUPercent, r.Watts, r.Adapters = sampler.finish()
	r.MaxClientW, r.MaxClientH = largest[0].Load(), largest[1].Load()

	if err := cmd.Wait(); err != nil {
		r.Failed = "exit: " + err.Error()
	}
	readFrames(&r, out, from, to)
	return r
}

func main() {
	// The harness must see physical pixels, like the programs it measures.
	const perMonitorAwareV2 = ^uintptr(3)
	procSetDPIAwareness.Call(perMonitorAwareV2)
	bin := flag.String("bin", "", "directory of gowaku.exe, gio.exe and cogent.exe")
	outPath := flag.String("out", "", "file for the results")
	libs := flag.String("libs", "gowaku,gio,cogent", "programs to run")
	scenarios := flag.String("scenarios", "startup,scroll,animate,resize,idle", "scenarios to run")
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
		"resize": 5 * time.Second, "idle": 60 * time.Second,
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
			for lib := range strings.SplitSeq(*libs, ",") {
				exe := filepath.Join(*bin, lib+".exe")
				var r Result
				if scenario == "startup" {
					for range 5 {
						r = startup(exe, lib)
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
