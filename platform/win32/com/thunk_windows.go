package com

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procFlushInstructionCache = kernel32.NewProc("FlushInstructionCache")

// ErrNoThunk means that this architecture has no float thunk.
var ErrNoThunk = errors.New("com: no float thunk for this architecture")

// NewFloatThunk returns code to put in a vtable for a method that takes
// floats double arguments right after this, and any number of integer or
// pointer arguments after them (at most three in all on amd64).
//
// syscall.NewCallback cannot receive floats. The thunk moves the doubles from
// their floating-point registers into the integer registers where a callback
// expects its next arguments, and jumps to target, a syscall.NewCallback
// function whose arguments are this, the bits of each double as a uintptr
// (math.Float64frombits turns them back), then the rest.
//
// The code is never freed, so make a thunk once per method and process.
func NewFloatThunk(target uintptr, floats int) (uintptr, error) {
	code, err := floatThunkCode(target, floats)
	if err != nil {
		return 0, err
	}
	return makeExecutable(code)
}

// NewFloatCaller is the reverse of NewFloatThunk, for calling a COM method
// that takes doubles: syscall.SyscallN cannot load floating-point registers.
// Call the returned code with SyscallN and the same arguments as target,
// each double given as its bits; it moves them into floating-point registers
// and jumps to target.
//
// The code is never freed, so make a caller once per method and process.
func NewFloatCaller(target uintptr, floats int) (uintptr, error) {
	code, err := floatCallerCode(target, floats)
	if err != nil {
		return 0, err
	}
	return makeExecutable(code)
}

// makeExecutable copies code into memory of its own that can run but not be
// written.
func makeExecutable(code []byte) (uintptr, error) {
	size := uintptr(len(code))
	addr, err := windows.VirtualAlloc(0, size, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		return 0, fmt.Errorf("com: VirtualAlloc: %w", err)
	}
	// The pages are ours from VirtualAlloc and outside the Go heap; reading
	// through &addr avoids a uintptr-to-pointer conversion.
	mem := *(*unsafe.Pointer)(unsafe.Pointer(&addr))
	copy(unsafe.Slice((*byte)(mem), size), code)
	var old uint32
	if err := windows.VirtualProtect(addr, size, windows.PAGE_EXECUTE_READ, &old); err != nil {
		return 0, fmt.Errorf("com: VirtualProtect: %w", err)
	}
	if r, _, err := procFlushInstructionCache.Call(uintptr(windows.CurrentProcess()), addr, size); r == 0 {
		return 0, fmt.Errorf("com: FlushInstructionCache: %w", err)
	}
	return addr, nil
}
