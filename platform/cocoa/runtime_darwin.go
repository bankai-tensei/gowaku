//go:build darwin && arm64

package cocoa

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// init locks the main goroutine to the main thread, the only one on which
// AppKit works. This is the exception to "init has no side effects"
// (CODING_STYLE §3.2): the lock must be taken before main runs, while the main
// goroutine is still on the main thread, and nothing later can take it back.
func init() {
	runtime.LockOSThread()
}

// NSPoint is a CGPoint. It, NSSize and NSRect hold doubles, which objc.Send
// passes in floating-point registers.
type NSPoint struct{ X, Y float64 }

// NSSize is a width and height in points.
type NSSize struct{ Width, Height float64 }

// NSRect is an origin and a size in points.
type NSRect struct {
	Origin NSPoint
	Size   NSSize
}

// NSRange is a range of UTF-16 code units of an NSString.
type NSRange struct{ Location, Length uint64 }

// NSNotFound is the location of a range that is not there.
const NSNotFound = 1<<63 - 1

// The constants of AppKit that the package uses, with the names of the headers.
const (
	NSApplicationActivationPolicyRegular = 0
	NSBackingStoreBuffered               = 2

	NSWindowStyleMaskTitled              = 1 << 0
	NSWindowStyleMaskClosable            = 1 << 1
	NSWindowStyleMaskMiniaturizable      = 1 << 2
	NSWindowStyleMaskResizable           = 1 << 3
	NSWindowStyleMaskFullSizeContentView = 1 << 15

	NSEventTypeApplicationDefined = 15

	NSTrackingMouseEnteredAndExited = 0x01
	NSTrackingMouseMoved            = 0x02
	NSTrackingCursorUpdate          = 0x04
	NSTrackingActiveAlways          = 0x80
	NSTrackingInVisibleRect         = 0x200

	NSTerminateCancel = 0

	NSUnderlineStyleThick = 0x02

	NSModalResponseOK = 1

	NSWindowAbove = 1

	NSUTF8StringEncoding = 4
)

// wakeSubtype tells the event that Wake posts from others of its type.
const wakeSubtype = 0x6777 // "gw"

// libs holds what the frameworks export. Each is opened once (CODING_STYLE §8).
type libs struct {
	defaultRunLoopMode objc.ID
	autoreleasePush    uintptr
	autoreleasePop     uintptr
	// enableSecureInput and disableSecureInput are EnableSecureEventInput
	// and DisableSecureEventInput of HIToolbox, which count their calls.
	enableSecureInput  uintptr
	disableSecureInput uintptr
}

var loaded struct {
	once sync.Once
	libs libs
	err  error
}

func loadLibs() (*libs, error) {
	loaded.once.Do(func() { loaded.libs, loaded.err = openLibs() })
	return &loaded.libs, loaded.err
}

func openLibs() (libs, error) {
	var l libs
	objcLib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return l, fmt.Errorf("cocoa: open libobjc: %w", err)
	}
	foundation, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return l, fmt.Errorf("cocoa: open Foundation: %w", err)
	}
	for _, path := range []string{
		"/System/Library/Frameworks/AppKit.framework/AppKit",
		"/System/Library/Frameworks/QuartzCore.framework/QuartzCore",
		// UTType, for the types of a file dialog.
		"/System/Library/Frameworks/UniformTypeIdentifiers.framework/UniformTypeIdentifiers",
	} {
		if _, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
			return l, fmt.Errorf("cocoa: open %s: %w", path, err)
		}
	}
	mode, err := purego.Dlsym(foundation, "NSDefaultRunLoopMode")
	if err != nil {
		return l, fmt.Errorf("cocoa: missing NSDefaultRunLoopMode: %w", err)
	}
	// The symbol is an NSString *const, which the framework keeps alive.
	l.defaultRunLoopMode = **(**objc.ID)(unsafe.Pointer(&mode))
	if l.autoreleasePush, err = purego.Dlsym(objcLib, "objc_autoreleasePoolPush"); err != nil {
		return l, err
	}
	if l.autoreleasePop, err = purego.Dlsym(objcLib, "objc_autoreleasePoolPop"); err != nil {
		return l, err
	}
	carbon, err := purego.Dlopen("/System/Library/Frameworks/Carbon.framework/Carbon", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return l, fmt.Errorf("cocoa: open Carbon: %w", err)
	}
	if l.enableSecureInput, err = purego.Dlsym(carbon, "EnableSecureEventInput"); err != nil {
		return l, fmt.Errorf("cocoa: missing EnableSecureEventInput: %w", err)
	}
	if l.disableSecureInput, err = purego.Dlsym(carbon, "DisableSecureEventInput"); err != nil {
		return l, fmt.Errorf("cocoa: missing DisableSecureEventInput: %w", err)
	}
	return l, nil
}

// responds reports whether obj answers the message name, which tells a newer
// macOS, with the replacement of a deprecated message, from an older one.
func responds(obj objc.ID, name string) bool {
	return objc.Send[bool](obj, sel("respondsToSelector:"), sel(name))
}

// pool runs fn inside an autorelease pool on the thread of the caller.
func (l *libs) pool(fn func()) {
	token := l.push()
	defer l.pop(token)
	fn()
}

// push and pop bracket an autorelease pool. Pools belong to a thread, so the
// goroutine stays on its thread in between; LockOSThread nests.
func (l *libs) push() uintptr {
	runtime.LockOSThread()
	token, _, _ := purego.SyscallN(l.autoreleasePush)
	return token
}

func (l *libs) pop(token uintptr) {
	purego.SyscallN(l.autoreleasePop, token)
	runtime.UnlockOSThread()
}

// sel returns the selector of a message. The runtime keeps a table, so this
// is a lookup after the first time.
func sel(name string) objc.SEL { return objc.RegisterName(name) }

// class returns an Objective-C class as an object, to send it class messages.
func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

// nsString returns an autoreleased NSString.
func nsString(s string) objc.ID {
	str := class("NSString").Send(sel("alloc"))
	if s == "" {
		str = str.Send(sel("init"))
	} else {
		// The bytes are copied before initWithBytes returns.
		str = str.Send(sel("initWithBytes:length:encoding:"),
			unsafe.Pointer(unsafe.StringData(s)), uint64(len(s)), uint64(NSUTF8StringEncoding))
	}
	return str.Send(sel("autorelease"))
}

// goString copies an NSString.
func goString(str objc.ID) string {
	if str == 0 {
		return ""
	}
	p := objc.Send[*byte](str, sel("UTF8String"))
	if p == nil {
		return ""
	}
	// UTF8String points to a NUL-terminated buffer that the autorelease pool
	// of the caller keeps.
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return validUTF8(string(unsafe.Slice(p, n)))
}

// nsArray returns an autoreleased NSArray of objects.
func nsArray(objects ...objc.ID) objc.ID {
	if len(objects) == 0 {
		return class("NSArray").Send(sel("array"))
	}
	// The array retains the objects before the call returns.
	return class("NSArray").Send(sel("arrayWithObjects:count:"), unsafe.Pointer(&objects[0]), uint64(len(objects)))
}

// count returns the length of an NSArray.
func count(array objc.ID) int {
	if array == 0 {
		return 0
	}
	return int(objc.Send[uint64](array, sel("count")))
}

// at returns an element of an NSArray.
func at(array objc.ID, i int) objc.ID {
	return array.Send(sel("objectAtIndex:"), uint64(i))
}
