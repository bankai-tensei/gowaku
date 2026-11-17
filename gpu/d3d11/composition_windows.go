package d3d11

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// A transparent window has no redirection surface, and a flip-model swap chain
// of an HWND cannot carry alpha to DWM. Its swap chain is made for
// composition, with premultiplied alpha, and DirectComposition puts it in the
// window as the content of one visual. See
// docs/ADR/0111-window-decorations.md.

// vtable indices of DirectComposition and of the composition swap chain. Before
// SetContent, IDCompositionVisual has SetOffsetX, SetOffsetY, SetTransform and
// SetClip in two overloads each, SetTransformParent, SetEffect,
// SetBitmapInterpolationMode and SetBorderMode: 12 slots after IUnknown. A
// probe on Windows 11 confirmed it (16, AddVisual, refuses a nil visual).
const (
	iDXGIFactory2_CreateSwapChainForComposition = 24 // IDXGIFactory2::CreateSwapChainForComposition

	iDCompositionDevice_Commit              = 3 // IDCompositionDevice::Commit
	iDCompositionDevice_CreateTargetForHwnd = 6 // IDCompositionDevice::CreateTargetForHwnd
	iDCompositionDevice_CreateVisual        = 7 // IDCompositionDevice::CreateVisual

	iDCompositionTarget_SetRoot = 3 // IDCompositionTarget::SetRoot

	iDCompositionVisual_SetContent = 15 // IDCompositionVisual::SetContent
)

// The values of a composition swap chain.
const (
	DXGI_ALPHA_MODE_PREMULTIPLIED = 1
	DXGI_SCALING_STRETCH          = 0
)

var (
	dcompDLL                     = windows.NewLazySystemDLL("dcomp.dll")
	procDCompositionCreateDevice = dcompDLL.NewProc("DCompositionCreateDevice")
)

// The interface IDs of DirectComposition.
var (
	IID_IDXGIDevice         = windows.GUID{Data1: 0x54ec77fa, Data2: 0x1377, Data3: 0x44e6, Data4: [8]byte{0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c}}
	IID_IDCompositionDevice = windows.GUID{Data1: 0xc37ea93a, Data2: 0xe7aa, Data3: 0x450d, Data4: [8]byte{0xb1, 0x6f, 0x97, 0x46, 0xcb, 0x04, 0x07, 0xf3}}
)

// composition holds the DirectComposition objects that show the swap chain of
// a transparent window. Each holds one reference.
type composition struct {
	device *unknown
	target *unknown
	visual *unknown
}

// release drops the objects, the visual first.
func (c *composition) release() {
	for _, u := range []*unknown{c.visual, c.target, c.device} {
		if u != nil {
			u.Release()
		}
	}
	*c = composition{}
}

// call runs method i of the COM object u with args after the object itself.
func (u *unknown) call(i int, args ...uintptr) hresult {
	r, _, _ := syscall.SyscallN(u.vtbl[i], append([]uintptr{uintptr(unsafe.Pointer(u))}, args...)...)
	return hr(r)
}

// createCompositionSurface makes the swap chain of a transparent window and
// puts it in the window.
func (d *Device) createCompositionSurface(win platform.Win32Handle, desc *DXGI_SWAP_CHAIN_DESC1) (*IDXGISwapChain1, composition, error) {
	var c composition
	if err := procDCompositionCreateDevice.Find(); err != nil {
		return nil, c, fmt.Errorf("load DCompositionCreateDevice: %w", err)
	}
	// Composition swap chains stretch, carry alpha, and do not tear.
	desc.Scaling, desc.AlphaMode, desc.Flags = DXGI_SCALING_STRETCH, DXGI_ALPHA_MODE_PREMULTIPLIED, 0
	var chain *IDXGISwapChain1
	if err := d.factory.call(iDXGIFactory2_CreateSwapChainForComposition, uintptr(unsafe.Pointer(d.device)),
		uintptr(unsafe.Pointer(desc)), 0, uintptr(unsafe.Pointer(&chain))).err("create composition swap chain"); err != nil {
		return nil, c, err
	}
	var dxgiDevice *unknown
	if err := d.device.call(iUnknown_QueryInterface, uintptr(unsafe.Pointer(&IID_IDXGIDevice)),
		uintptr(unsafe.Pointer(&dxgiDevice))).err("query dxgi device"); err != nil {
		chain.Release()
		return nil, c, err
	}
	r, _, _ := procDCompositionCreateDevice.Call(uintptr(unsafe.Pointer(dxgiDevice)),
		uintptr(unsafe.Pointer(&IID_IDCompositionDevice)), uintptr(unsafe.Pointer(&c.device)))
	dxgiDevice.Release()
	err := hr(r).err("create composition device")
	if err == nil {
		// topmost TRUE puts the visual over the rest of the window.
		err = c.device.call(iDCompositionDevice_CreateTargetForHwnd, uintptr(win.HWND), 1,
			uintptr(unsafe.Pointer(&c.target))).err("create composition target")
	}
	if err == nil {
		err = c.device.call(iDCompositionDevice_CreateVisual, uintptr(unsafe.Pointer(&c.visual))).err("create composition visual")
	}
	if err == nil {
		err = c.visual.call(iDCompositionVisual_SetContent, uintptr(unsafe.Pointer(chain))).err("set composition content")
	}
	if err == nil {
		err = c.target.call(iDCompositionTarget_SetRoot, uintptr(unsafe.Pointer(c.visual))).err("set composition root")
	}
	if err == nil {
		err = c.device.call(iDCompositionDevice_Commit).err("commit composition")
	}
	if err != nil {
		c.release()
		chain.Release()
		return nil, composition{}, err
	}
	return chain, c, nil
}
