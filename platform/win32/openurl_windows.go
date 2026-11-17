package win32

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

// OpenURL hands u to ShellExecute, which opens it with the handler of its
// scheme: a web page in the default browser, mailto: in the mail program. It
// needs no window and may be called on any goroutine. u is not checked here;
// gowaku.OpenURL checks it.
//
// The shell may run COM handlers for a scheme, so the call runs on a thread of
// its own with an apartment, as the documentation of ShellExecute asks, and
// not on a thread that some other code initialized differently.
func OpenURL(u string) error {
	file, err := windows.UTF16PtrFromString(u)
	if err != nil {
		return fmt.Errorf("win32: open url: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("win32: open url: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		// S_FALSE, already initialized, also needs its CoUninitialize. A
		// thread in another mode (RPC_E_CHANGED_MODE) can still try.
		err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
		if err == nil || errors.Is(err, syscall.Errno(1)) {
			defer windows.CoUninitialize()
		}
		done <- windows.ShellExecute(0, verb, file, nil, nil, SW_SHOWNORMAL)
	}()
	if err := <-done; err != nil {
		return fmt.Errorf("win32: ShellExecute: %w", err)
	}
	return nil
}
