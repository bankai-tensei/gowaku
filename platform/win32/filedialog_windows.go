package win32

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// The COM classes and interfaces of the common item dialog (shobjidl_core.h).
var (
	CLSID_FileOpenDialog = mustGUID("{DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7}")
	CLSID_FileSaveDialog = mustGUID("{C0B4E2F3-BA21-4773-8DBA-335EC946EB8B}")
	IID_IFileOpenDialog  = mustGUID("{D57C7288-D4AD-4768-BE02-9D969532D960}")
	IID_IFileSaveDialog  = mustGUID("{84BCCD23-5FDE-4CDB-AEA4-AF64B83D78AB}")
	IID_IShellItem       = mustGUID("{43826D1E-E718-42EE-BC55-A1E261C37BFE}")

	procCoCreateInstance            = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = windows.NewLazySystemDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
)

// Values of the headers, and the vtable slots of the interfaces, each counted
// from the start of IUnknown.
const (
	CLSCTX_INPROC_SERVER = 0x1
	SIGDN_FILESYSPATH    = 0x80058000
	FOS_OVERWRITEPROMPT  = 0x2
	FOS_PICKFOLDERS      = 0x20
	FOS_FORCEFILESYSTEM  = 0x40
	FOS_ALLOWMULTISELECT = 0x200
	// errorCancelled is what Show returns when the user closes the dialog:
	// the HRESULT of the Win32 error 1223.
	errorCancelled = 0x800704C7

	iUnknown_Release                = 2
	iModalWindow_Show               = 3
	iFileDialog_SetFileTypes        = 4
	iFileDialog_SetOptions          = 9
	iFileDialog_GetOptions          = 10
	iFileDialog_SetFolder           = 12
	iFileDialog_SetFileName         = 15
	iFileDialog_SetTitle            = 17
	iFileDialog_GetResult           = 20
	iFileDialog_SetDefaultExtension = 22
	iFileOpenDialog_GetResults      = 27
	iShellItem_GetDisplayName       = 5
	iShellItemArray_GetCount        = 7
	iShellItemArray_GetItemAt       = 8
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString(s)
	if err != nil {
		panic(err)
	}
	return g
}

// comObject is any COM interface: a pointer to its vtable.
type comObject struct{ vtbl *[32]uintptr }

// call calls method slot of o. Pointers converted to uintptr in args stay
// valid during the call, as for syscall.SyscallN: the directive below puts
// what they point to on the heap and keeps it alive, though the stack of the
// dialog's own message loop grows and moves.
//
//go:uintptrescapes
func (o *comObject) call(slot int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObject) release() {
	if o != nil {
		o.call(iUnknown_Release)
	}
}

// comdlgFilterSpec is COMDLG_FILTERSPEC.
type comdlgFilterSpec struct {
	name, spec *uint16
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

// ShowFileDialog implements platform.Window with the common item dialog. It is
// modal, and returns once the user is done: the dialog runs a message loop of
// its own on the UI thread.
func (w *Window) ShowFileDialog(opts platform.FileDialogOptions, done func([]string, error)) {
	done(w.showFileDialog(opts))
}

func (w *Window) showFileDialog(opts platform.FileDialogOptions) ([]string, error) {
	// The dialog needs a single-threaded apartment. A second initialization
	// on the same thread is harmless: it returns S_FALSE, which x/sys reports
	// as an error. It stays for the life of the thread.
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil && !errors.Is(err, syscall.Errno(1)) {
		return nil, fmt.Errorf("win32: CoInitializeEx: %w", err)
	}
	clsid, iid := &CLSID_FileOpenDialog, &IID_IFileOpenDialog
	if opts.Save {
		clsid, iid = &CLSID_FileSaveDialog, &IID_IFileSaveDialog
	}
	var dialog *comObject
	hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&dialog)))
	if failed(hr) {
		return nil, fmt.Errorf("win32: create file dialog: HRESULT %#x", uint32(hr))
	}
	defer dialog.release()

	var flags uint32
	dialog.call(iFileDialog_GetOptions, uintptr(unsafe.Pointer(&flags)))
	flags |= FOS_FORCEFILESYSTEM
	switch {
	case opts.Save:
		flags |= FOS_OVERWRITEPROMPT
	case opts.Directory:
		flags |= FOS_PICKFOLDERS
	}
	if opts.Multiple && !opts.Save {
		flags |= FOS_ALLOWMULTISELECT
	}
	dialog.call(iFileDialog_SetOptions, uintptr(flags))

	// The strings and the filters must outlive the calls that take them,
	// until the dialog is done; keep and specs hold them.
	var keep [][]uint16
	var specs []comdlgFilterSpec
	str := func(s string) *uint16 {
		u, err := windows.UTF16FromString(s)
		if err != nil {
			// A NUL inside: drop the rest, as C would read it.
			u, _ = windows.UTF16FromString(s[:strings.IndexByte(s, 0)]) //nolint:errcheck // no NUL left
		}
		keep = append(keep, u)
		return &u[0]
	}
	if opts.Title != "" {
		dialog.call(iFileDialog_SetTitle, uintptr(unsafe.Pointer(str(opts.Title))))
	}
	if len(opts.Filters) > 0 && !opts.Directory {
		specs = make([]comdlgFilterSpec, len(opts.Filters))
		for i, f := range opts.Filters {
			specs[i] = comdlgFilterSpec{name: str(f.Name), spec: str(strings.Join(f.Patterns, ";"))}
		}
		dialog.call(iFileDialog_SetFileTypes, uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0])))
		if ext := defaultExtension(opts.Filters); ext != "" && opts.Save {
			dialog.call(iFileDialog_SetDefaultExtension, uintptr(unsafe.Pointer(str(ext))))
		}
	}
	if opts.Name != "" {
		dialog.call(iFileDialog_SetFileName, uintptr(unsafe.Pointer(str(opts.Name))))
	}
	if opts.Folder != "" {
		var folder *comObject
		hr, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(str(filepath.Clean(opts.Folder)))), 0,
			uintptr(unsafe.Pointer(&IID_IShellItem)), uintptr(unsafe.Pointer(&folder)))
		if !failed(hr) {
			dialog.call(iFileDialog_SetFolder, uintptr(unsafe.Pointer(folder)))
			folder.release()
		}
	}

	hr = dialog.call(iModalWindow_Show, uintptr(w.hwnd))
	runtime.KeepAlive(keep)
	runtime.KeepAlive(specs)
	if uint32(hr) == errorCancelled {
		return nil, nil
	}
	if failed(hr) {
		return nil, fmt.Errorf("win32: file dialog: HRESULT %#x", uint32(hr))
	}
	if opts.Save || !opts.Multiple {
		var item *comObject
		if hr := dialog.call(iFileDialog_GetResult, uintptr(unsafe.Pointer(&item))); failed(hr) {
			return nil, fmt.Errorf("win32: file dialog result: HRESULT %#x", uint32(hr))
		}
		defer item.release()
		p, err := itemPath(item)
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	}
	var items *comObject
	if hr := dialog.call(iFileOpenDialog_GetResults, uintptr(unsafe.Pointer(&items))); failed(hr) {
		return nil, fmt.Errorf("win32: file dialog results: HRESULT %#x", uint32(hr))
	}
	defer items.release()
	var count uint32
	items.call(iShellItemArray_GetCount, uintptr(unsafe.Pointer(&count)))
	paths := make([]string, 0, count)
	for i := range count {
		var item *comObject
		if hr := items.call(iShellItemArray_GetItemAt, uintptr(i), uintptr(unsafe.Pointer(&item))); failed(hr) {
			continue
		}
		p, err := itemPath(item)
		item.release()
		if err == nil {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// itemPath returns the file system path of an IShellItem.
func itemPath(item *comObject) (string, error) {
	var name *uint16
	if hr := item.call(iShellItem_GetDisplayName, SIGDN_FILESYSPATH, uintptr(unsafe.Pointer(&name))); failed(hr) {
		return "", fmt.Errorf("win32: path of a chosen item: HRESULT %#x", uint32(hr))
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(name))
	return windows.UTF16PtrToString(name), nil
}

// defaultExtension returns the extension a save dialog adds to a name without
// one: that of the first pattern of the first filter, as "txt" of "*.txt".
func defaultExtension(filters []platform.FileFilter) string {
	for _, p := range filters[0].Patterns {
		if ext, ok := strings.CutPrefix(p, "*."); ok && !strings.ContainsAny(ext, "*?") {
			return ext
		}
	}
	return ""
}
