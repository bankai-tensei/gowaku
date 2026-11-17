//go:build darwin && arm64

package metal

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/gpu"
)

// maxArgs is the most arguments, receiver and selector included, that a hot
// message takes: copyFromBuffer:...destinationOrigin: has eleven.
const maxArgs = 12

// libs holds what the frameworks export. Each is opened once (CODING_STYLE §8).
type libs struct {
	msgSend               uintptr
	autoreleasePoolPush   uintptr
	autoreleasePoolPop    uintptr
	createSystemDefaultMT uintptr
}

// loaded is the one dlopen of the frameworks for the process.
var loaded struct {
	once sync.Once
	libs libs
	err  error
}

func loadLibs() (*libs, error) {
	loaded.once.Do(func() {
		loaded.libs, loaded.err = openLibs()
	})
	return &loaded.libs, loaded.err
}

func openLibs() (libs, error) {
	var l libs
	objcLib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return l, fmt.Errorf("metal: open libobjc: %w", err)
	}
	mtl, err := purego.Dlopen("/System/Library/Frameworks/Metal.framework/Metal", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return l, fmt.Errorf("metal: open Metal.framework: %w (%w)", err, gpu.ErrNoGPU)
	}
	// QuartzCore defines CAMetalLayer, which the surface configures.
	if _, err := purego.Dlopen("/System/Library/Frameworks/QuartzCore.framework/QuartzCore", purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
		return l, fmt.Errorf("metal: open QuartzCore.framework: %w", err)
	}
	for _, s := range []struct {
		lib  uintptr
		name string
		fn   *uintptr
	}{
		{objcLib, "objc_msgSend", &l.msgSend},
		{objcLib, "objc_autoreleasePoolPush", &l.autoreleasePoolPush},
		{objcLib, "objc_autoreleasePoolPop", &l.autoreleasePoolPop},
		{mtl, "MTLCreateSystemDefaultDevice", &l.createSystemDefaultMT},
	} {
		fn, err := purego.Dlsym(s.lib, s.name)
		if err != nil {
			return l, fmt.Errorf("metal: missing %s: %w", s.name, err)
		}
		*s.fn = fn
	}
	return l, nil
}

// selectors are the messages of the hot path, registered once at New.
type selectors struct {
	retain, release                            uintptr
	commandBuffer, commit, waitUntilCompleted  uintptr
	status, presentDrawable                    uintptr
	renderCommandEncoderWithDescriptor         uintptr
	blitCommandEncoder, endEncoding            uintptr
	setRenderPipelineState                     uintptr
	setDepthStencilState                       uintptr
	setViewport, setScissorRect                uintptr
	setVertexBuffer, setVertexBufferOffset     uintptr
	setVertexBytes, setFragmentBytes           uintptr
	setFragmentTexture                         uintptr
	drawPrimitives, drawPrimitivesInstanced    uintptr
	setColorStoreAction                        uintptr
	setDepthStoreAction, setStencilStoreAction uintptr
	copyTexture, copyBufferToTexture           uintptr
	copyTextureToBuffer                        uintptr
	setTexture, setResolveTexture              uintptr
	setLoadAction, setStoreAction              uintptr
	nextDrawable, texture, contents            uintptr
	present                                    uintptr
}

func (s *selectors) register() {
	for _, e := range []struct {
		sel  *uintptr
		name string
	}{
		{&s.retain, "retain"}, {&s.release, "release"},
		{&s.commandBuffer, "commandBuffer"}, {&s.commit, "commit"},
		{&s.waitUntilCompleted, "waitUntilCompleted"}, {&s.status, "status"},
		{&s.presentDrawable, "presentDrawable:"},
		{&s.renderCommandEncoderWithDescriptor, "renderCommandEncoderWithDescriptor:"},
		{&s.blitCommandEncoder, "blitCommandEncoder"}, {&s.endEncoding, "endEncoding"},
		{&s.setRenderPipelineState, "setRenderPipelineState:"},
		{&s.setDepthStencilState, "setDepthStencilState:"},
		{&s.setViewport, "setViewport:"}, {&s.setScissorRect, "setScissorRect:"},
		{&s.setVertexBuffer, "setVertexBuffer:offset:atIndex:"},
		{&s.setVertexBufferOffset, "setVertexBufferOffset:atIndex:"},
		{&s.setVertexBytes, "setVertexBytes:length:atIndex:"},
		{&s.setFragmentBytes, "setFragmentBytes:length:atIndex:"},
		{&s.setFragmentTexture, "setFragmentTexture:atIndex:"},
		{&s.drawPrimitives, "drawPrimitives:vertexStart:vertexCount:"},
		{&s.drawPrimitivesInstanced, "drawPrimitives:vertexStart:vertexCount:instanceCount:"},
		{&s.setColorStoreAction, "setColorStoreAction:atIndex:"},
		{&s.setDepthStoreAction, "setDepthStoreAction:"},
		{&s.setStencilStoreAction, "setStencilStoreAction:"},
		{&s.copyTexture, "copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:toTexture:destinationSlice:destinationLevel:destinationOrigin:"},
		{&s.copyBufferToTexture, "copyFromBuffer:sourceOffset:sourceBytesPerRow:sourceBytesPerImage:sourceSize:toTexture:destinationSlice:destinationLevel:destinationOrigin:"},
		{&s.copyTextureToBuffer, "copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:toBuffer:destinationOffset:destinationBytesPerRow:destinationBytesPerImage:"},
		{&s.setTexture, "setTexture:"}, {&s.setResolveTexture, "setResolveTexture:"},
		{&s.setLoadAction, "setLoadAction:"}, {&s.setStoreAction, "setStoreAction:"},
		{&s.nextDrawable, "nextDrawable"}, {&s.texture, "texture"}, {&s.contents, "contents"},
		{&s.present, "present"},
	} {
		*e.sel = uintptr(objc.RegisterName(e.name))
	}
}

// send sends a message whose arguments are all integers or pointers. Pointers
// among args must lead to fields of the Device or a Surface, which outlive the
// call, because a uintptr does not keep its object alive or in place (see
// ptr). It copies the arguments to a field so that they stay on the stack of
// the caller and nothing allocates.
func (d *Device) send(obj, sel uintptr, args ...uintptr) uintptr {
	d.args[0], d.args[1] = obj, sel
	n := 2 + copy(d.args[2:], args)
	r, _, _ := purego.SyscallN(d.libs.msgSend, d.args[:n]...)
	return r
}

// ptr converts a pointer to a field for send.
func ptr[T any](p *T) uintptr {
	return uintptr(unsafe.Pointer(p))
}

// cPointer converts an address that Metal returned. Going through memory
// keeps go vet from taking it for a Go pointer hidden in a uintptr.
func cPointer(address uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&address))
}

// pushPool and popPool bracket every entry of the package, so that what Metal
// hands back autoreleased goes before the call returns. Objects kept across
// calls are retained. A pool must not outlive the call: the event loop of the
// platform has pools of its own, and they must pop in order.
//
// Pools belong to a thread, so the goroutine stays on its thread in between.
// The UI goroutine is locked already, and LockOSThread nests; New runs on
// another goroutine (ADR 0018).
func (d *Device) pushPool() uintptr {
	runtime.LockOSThread()
	r, _, _ := purego.SyscallN(d.libs.autoreleasePoolPush)
	return r
}

func (d *Device) popPool(token uintptr) {
	purego.SyscallN(d.libs.autoreleasePoolPop, token)
	runtime.UnlockOSThread()
}

// nsString returns a new NSString, which the caller releases.
func nsString(s string) objc.ID {
	str := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("alloc"))
	if s == "" {
		return str.Send(objc.RegisterName("init"))
	}
	// The bytes are copied before initWithBytes returns.
	return str.Send(objc.RegisterName("initWithBytes:length:encoding:"),
		unsafe.Pointer(unsafe.StringData(s)), uint64(len(s)), uint64(NSUTF8StringEncoding))
}

// goString copies an NSString.
func goString(str objc.ID) string {
	if str == 0 {
		return ""
	}
	p := objc.Send[*byte](str, objc.RegisterName("UTF8String"))
	if p == nil {
		return ""
	}
	// UTF8String points to a NUL-terminated buffer that the autorelease pool
	// of the caller keeps.
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// describe returns the localizedDescription of an NSError, for messages.
func describe(err objc.ID) error {
	if err == 0 {
		return errors.New("unknown error")
	}
	return errors.New(goString(err.Send(objc.RegisterName("localizedDescription"))))
}
