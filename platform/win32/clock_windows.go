package win32

import (
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	procQueryPerformanceCounter   = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency = kernel32.NewProc("QueryPerformanceFrequency")
)

// qpcFrequency is the frequency of the performance counter, which does not
// change while the system runs, or 0 until PreciseTime reads it.
var qpcFrequency atomic.Int64

// PreciseTime returns a monotonic time from QueryPerformanceCounter, whose steps
// are well under a microsecond where those of time.Now on Windows are about half a
// millisecond. Only differences between two readings mean anything. It exists for
// benchmarks that time single frames, and for the Inspector of -tags
// gowakudebug (ADR 0073). It does not allocate, and is safe for any goroutine.
func PreciseTime() time.Duration {
	frequency := qpcFrequency.Load()
	if frequency == 0 {
		var f int64
		// It writes one int64 through the pointer during the call.
		_, _, _ = procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&f)))
		if f <= 0 {
			return 0
		}
		qpcFrequency.Store(f)
		frequency = f
	}
	count := queryCounter()
	seconds := count / frequency
	rest := count % frequency
	return time.Duration(seconds)*time.Second + time.Duration(rest*int64(time.Second)/frequency)
}

// queryCounter reads the performance counter. The int64 it writes to lives on
// the stack: syscall.SyscallN, unlike LazyProc.Call, does not force what its
// arguments point to onto the heap.
func queryCounter() int64 {
	var count int64
	_, _, _ = syscall.SyscallN(procQueryPerformanceCounter.Addr(), uintptr(unsafe.Pointer(&count)))
	return count
}
