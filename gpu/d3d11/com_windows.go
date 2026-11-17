package d3d11

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// vtable indices. They were checked against the C vtable structs of the
// mingw-w64 headers with offsetof.
const (
	iUnknown_QueryInterface = 0 // IUnknown::QueryInterface
	iUnknown_Release        = 2 // IUnknown::Release

	iDXGIFactory_MakeWindowAssociation       = 8  // IDXGIFactory::MakeWindowAssociation
	iDXGIFactory1_EnumAdapters1              = 12 // IDXGIFactory1::EnumAdapters1
	iDXGIFactory2_CreateSwapChainForHwnd     = 15 // IDXGIFactory2::CreateSwapChainForHwnd
	iDXGIFactory6_EnumAdapterByGpuPreference = 29 // IDXGIFactory6::EnumAdapterByGpuPreference

	iDXGIAdapter1_GetDesc1 = 10 // IDXGIAdapter1::GetDesc1

	iDXGISwapChain_Present       = 8  // IDXGISwapChain::Present
	iDXGISwapChain_GetBuffer     = 9  // IDXGISwapChain::GetBuffer
	iDXGISwapChain_ResizeBuffers = 13 // IDXGISwapChain::ResizeBuffers

	iD3D11Device_CreateBuffer             = 3  // ID3D11Device::CreateBuffer
	iD3D11Device_CreateTexture2D          = 5  // ID3D11Device::CreateTexture2D
	iD3D11Device_CreateShaderResourceView = 7  // ID3D11Device::CreateShaderResourceView
	iD3D11Device_CreateRenderTargetView   = 9  // ID3D11Device::CreateRenderTargetView
	iD3D11Device_CreateDepthStencilView   = 10 // ID3D11Device::CreateDepthStencilView
	iD3D11Device_CreateDepthStencilState  = 21 // ID3D11Device::CreateDepthStencilState
	iD3D11Device_CreateSamplerState       = 23 // ID3D11Device::CreateSamplerState
	iD3D11Device_CreateInputLayout        = 11 // ID3D11Device::CreateInputLayout
	iD3D11Device_CreateVertexShader       = 12 // ID3D11Device::CreateVertexShader
	iD3D11Device_CreatePixelShader        = 15 // ID3D11Device::CreatePixelShader
	iD3D11Device_CreateBlendState         = 20 // ID3D11Device::CreateBlendState
	iD3D11Device_CreateRasterizerState    = 22 // ID3D11Device::CreateRasterizerState
	iD3D11Device_CreateQuery              = 24 // ID3D11Device::CreateQuery
	iD3D11Device_GetDeviceRemovedReason   = 39 // ID3D11Device::GetDeviceRemovedReason

	iD3D11DeviceContext1_ClearView             = 132 // ID3D11DeviceContext1::ClearView
	iD3D11DeviceContext_VSSetConstantBuffers   = 7   // ID3D11DeviceContext::VSSetConstantBuffers
	iD3D11DeviceContext_PSSetShaderResources   = 8   // ID3D11DeviceContext::PSSetShaderResources
	iD3D11DeviceContext_PSSetShader            = 9   // ID3D11DeviceContext::PSSetShader
	iD3D11DeviceContext_PSSetSamplers          = 10  // ID3D11DeviceContext::PSSetSamplers
	iD3D11DeviceContext_VSSetShader            = 11  // ID3D11DeviceContext::VSSetShader
	iD3D11DeviceContext_Draw                   = 13  // ID3D11DeviceContext::Draw
	iD3D11DeviceContext_Map                    = 14  // ID3D11DeviceContext::Map
	iD3D11DeviceContext_Unmap                  = 15  // ID3D11DeviceContext::Unmap
	iD3D11DeviceContext_PSSetConstantBuffers   = 16  // ID3D11DeviceContext::PSSetConstantBuffers
	iD3D11DeviceContext_IASetInputLayout       = 17  // ID3D11DeviceContext::IASetInputLayout
	iD3D11DeviceContext_IASetVertexBuffers     = 18  // ID3D11DeviceContext::IASetVertexBuffers
	iD3D11DeviceContext_DrawInstanced          = 21  // ID3D11DeviceContext::DrawInstanced
	iD3D11DeviceContext_IASetPrimitiveTopology = 24  // ID3D11DeviceContext::IASetPrimitiveTopology
	iD3D11DeviceContext_Begin                  = 27  // ID3D11DeviceContext::Begin
	iD3D11DeviceContext_End                    = 28  // ID3D11DeviceContext::End
	iD3D11DeviceContext_GetData                = 29  // ID3D11DeviceContext::GetData
	iD3D11DeviceContext_OMSetRenderTargets     = 33  // ID3D11DeviceContext::OMSetRenderTargets
	iD3D11DeviceContext_OMSetBlendState        = 35  // ID3D11DeviceContext::OMSetBlendState
	iD3D11DeviceContext_OMSetDepthStencilState = 36  // ID3D11DeviceContext::OMSetDepthStencilState
	iD3D11DeviceContext_RSSetState             = 43  // ID3D11DeviceContext::RSSetState
	iD3D11DeviceContext_RSSetViewports         = 44  // ID3D11DeviceContext::RSSetViewports
	iD3D11DeviceContext_RSSetScissorRects      = 45  // ID3D11DeviceContext::RSSetScissorRects
	iD3D11DeviceContext_CopySubresourceRegion  = 46  // ID3D11DeviceContext::CopySubresourceRegion
	iD3D11DeviceContext_CopyResource           = 47  // ID3D11DeviceContext::CopyResource
	iD3D11DeviceContext_UpdateSubresource      = 48  // ID3D11DeviceContext::UpdateSubresource
	iD3D11DeviceContext_ClearRenderTargetView  = 50  // ID3D11DeviceContext::ClearRenderTargetView
	iD3D11DeviceContext_ClearDepthStencilView  = 53  // ID3D11DeviceContext::ClearDepthStencilView
	iD3D11DeviceContext_ResolveSubresource     = 57  // ID3D11DeviceContext::ResolveSubresource
	iD3D11DeviceContext_ClearState             = 110 // ID3D11DeviceContext::ClearState
	iD3D11DeviceContext_Flush                  = 111 // ID3D11DeviceContext::Flush
)

// vtable is a COM method table. The length only has to exceed the largest
// index in use; the real table lives in the DLL and is never copied.
type vtable [160]uintptr

// unknown is the head of every COM object: a pointer to its vtable. COM
// interface types embed it, so a *ID3D11Device is the COM pointer itself. It
// points outside the Go heap, so the garbage collector ignores it.
type unknown struct {
	vtbl *vtable
}

// Release drops one reference and returns the number left.
func (u *unknown) Release() uint32 {
	r, _, _ := syscall.SyscallN(u.vtbl[iUnknown_Release], uintptr(unsafe.Pointer(u)))
	return uint32(r)
}

// hresult is a COM status code. Negative values are failures.
type hresult int32

func (s hresult) failed() bool { return s < 0 }

// err returns nil for a success code.
func (s hresult) err(op string) error {
	if !s.failed() {
		return nil
	}
	return fmt.Errorf("%s: HRESULT %#08x", op, uint32(s))
}

func hr(r uintptr) hresult { return hresult(int32(uint32(r))) }

// The COM interfaces gowaku uses. Pointers passed to their methods below are
// converted inside the SyscallN argument list, which keeps the pointee alive
// for the duration of the call. D3D copies everything it reads from them.
type (
	// IDXGIFactory2 is the interface of the same name.
	IDXGIFactory2 struct{ unknown }
	// IDXGIFactory6 is the interface of the same name.
	IDXGIFactory6 struct{ unknown }
	// IDXGIAdapter1 is the interface of the same name.
	IDXGIAdapter1 struct{ unknown }
	// ID3D11DeviceContext1 is the interface of the same name.
	ID3D11DeviceContext1 struct{ unknown }
	// IDXGISwapChain1 is the interface of the same name.
	IDXGISwapChain1 struct{ unknown }
	// ID3D11Device is the interface of the same name.
	ID3D11Device struct{ unknown }
	// ID3D11DeviceContext is the interface of the same name.
	ID3D11DeviceContext struct{ unknown }
	// ID3D11Buffer is the interface of the same name.
	ID3D11Buffer struct{ unknown }
	// ID3D11Texture2D is the interface of the same name.
	ID3D11Texture2D struct{ unknown }
	// ID3D11ShaderResourceView is the interface of the same name.
	ID3D11ShaderResourceView struct{ unknown }
	// ID3D11DepthStencilView is the interface of the same name.
	ID3D11DepthStencilView struct{ unknown }
	// ID3D11DepthStencilState is the interface of the same name.
	ID3D11DepthStencilState struct{ unknown }
	// ID3D11SamplerState is the interface of the same name.
	ID3D11SamplerState struct{ unknown }
	// ID3D11RenderTargetView is the interface of the same name.
	ID3D11RenderTargetView struct{ unknown }
	// ID3D11InputLayout is the interface of the same name.
	ID3D11InputLayout struct{ unknown }
	// ID3D11VertexShader is the interface of the same name.
	ID3D11VertexShader struct{ unknown }
	// ID3D11PixelShader is the interface of the same name.
	ID3D11PixelShader struct{ unknown }
	// ID3D11BlendState is the interface of the same name.
	ID3D11BlendState struct{ unknown }
	// ID3D11RasterizerState is the interface of the same name.
	ID3D11RasterizerState struct{ unknown }
	// ID3D11Query is the interface of the same name.
	ID3D11Query struct{ unknown }
)

func (f *IDXGIFactory2) queryFactory6() *IDXGIFactory6 {
	var out *IDXGIFactory6
	r, _, _ := syscall.SyscallN(f.vtbl[iUnknown_QueryInterface], uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(&IID_IDXGIFactory6)), uintptr(unsafe.Pointer(&out)))
	if hr(r).failed() {
		return nil
	}
	return out
}

func (f *IDXGIFactory2) enumAdapters1(index uint32) (*IDXGIAdapter1, hresult) {
	var out *IDXGIAdapter1
	r, _, _ := syscall.SyscallN(f.vtbl[iDXGIFactory1_EnumAdapters1], uintptr(unsafe.Pointer(f)),
		uintptr(index), uintptr(unsafe.Pointer(&out)))
	return out, hr(r)
}

func (f *IDXGIFactory2) makeWindowAssociation(hwnd platform.HWND, flags uint32) error {
	r, _, _ := syscall.SyscallN(f.vtbl[iDXGIFactory_MakeWindowAssociation], uintptr(unsafe.Pointer(f)),
		uintptr(hwnd), uintptr(flags))
	return hr(r).err("make window association")
}

func (f *IDXGIFactory2) createSwapChainForHwnd(dev *ID3D11Device, hwnd platform.HWND, desc *DXGI_SWAP_CHAIN_DESC1) (*IDXGISwapChain1, error) {
	var out *IDXGISwapChain1
	r, _, _ := syscall.SyscallN(f.vtbl[iDXGIFactory2_CreateSwapChainForHwnd], uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(dev)), uintptr(hwnd), uintptr(unsafe.Pointer(desc)), 0, 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create swap chain")
}

func (f *IDXGIFactory6) enumAdapterByGpuPreference(index, preference uint32) (*IDXGIAdapter1, hresult) {
	var out *IDXGIAdapter1
	r, _, _ := syscall.SyscallN(f.vtbl[iDXGIFactory6_EnumAdapterByGpuPreference], uintptr(unsafe.Pointer(f)),
		uintptr(index), uintptr(preference), uintptr(unsafe.Pointer(&IID_IDXGIAdapter1)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r)
}

func (a *IDXGIAdapter1) desc1() (DXGI_ADAPTER_DESC1, error) {
	var d DXGI_ADAPTER_DESC1
	r, _, _ := syscall.SyscallN(a.vtbl[iDXGIAdapter1_GetDesc1], uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&d)))
	return d, hr(r).err("get adapter desc")
}

func (s *IDXGISwapChain1) present(syncInterval, flags uint32) hresult {
	r, _, _ := syscall.SyscallN(s.vtbl[iDXGISwapChain_Present], uintptr(unsafe.Pointer(s)),
		uintptr(syncInterval), uintptr(flags))
	return hr(r)
}

func (s *IDXGISwapChain1) backBuffer() (*ID3D11Texture2D, error) {
	var out *ID3D11Texture2D
	r, _, _ := syscall.SyscallN(s.vtbl[iDXGISwapChain_GetBuffer], uintptr(unsafe.Pointer(s)),
		0, uintptr(unsafe.Pointer(&IID_ID3D11Texture2D)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("get back buffer")
}

func (s *IDXGISwapChain1) resizeBuffers(width, height, flags uint32) hresult {
	// Zero keeps the buffer count, and DXGI_FORMAT_UNKNOWN the format.
	r, _, _ := syscall.SyscallN(s.vtbl[iDXGISwapChain_ResizeBuffers], uintptr(unsafe.Pointer(s)),
		0, uintptr(width), uintptr(height), DXGI_FORMAT_UNKNOWN, uintptr(flags))
	return hr(r)
}

func (d *ID3D11Device) createBuffer(desc *D3D11_BUFFER_DESC) (*ID3D11Buffer, error) {
	var out *ID3D11Buffer
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateBuffer], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)), 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create buffer")
}

func (d *ID3D11Device) createTexture2D(desc *D3D11_TEXTURE2D_DESC) (*ID3D11Texture2D, error) {
	var out *ID3D11Texture2D
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateTexture2D], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)), 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create texture")
}

func (d *ID3D11Device) createShaderResourceView(t *ID3D11Texture2D) (*ID3D11ShaderResourceView, error) {
	var out *ID3D11ShaderResourceView
	// A null description views the whole texture in its own format.
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateShaderResourceView], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(t)), 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create shader resource view")
}

func (d *ID3D11Device) createDepthStencilView(t *ID3D11Texture2D) (*ID3D11DepthStencilView, error) {
	var out *ID3D11DepthStencilView
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateDepthStencilView], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(t)), 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create depth stencil view")
}

func (d *ID3D11Device) createDepthStencilState(desc *D3D11_DEPTH_STENCIL_DESC) (*ID3D11DepthStencilState, error) {
	var out *ID3D11DepthStencilState
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateDepthStencilState], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create depth stencil state")
}

func (d *ID3D11Device) createSamplerState(desc *D3D11_SAMPLER_DESC) (*ID3D11SamplerState, error) {
	var out *ID3D11SamplerState
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateSamplerState], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create sampler state")
}

func (d *ID3D11Device) createRenderTargetView(t *ID3D11Texture2D) (*ID3D11RenderTargetView, error) {
	var out *ID3D11RenderTargetView
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateRenderTargetView], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(t)), 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create render target view")
}

func (d *ID3D11Device) createInputLayout(elems []D3D11_INPUT_ELEMENT_DESC, vs []byte) (*ID3D11InputLayout, error) {
	var out *ID3D11InputLayout
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateInputLayout], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(unsafe.SliceData(elems))), uintptr(len(elems)),
		uintptr(unsafe.Pointer(unsafe.SliceData(vs))), uintptr(len(vs)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create input layout")
}

func (d *ID3D11Device) createVertexShader(code []byte) (*ID3D11VertexShader, error) {
	var out *ID3D11VertexShader
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateVertexShader], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(unsafe.SliceData(code))), uintptr(len(code)), 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create vertex shader")
}

func (d *ID3D11Device) createPixelShader(code []byte) (*ID3D11PixelShader, error) {
	var out *ID3D11PixelShader
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreatePixelShader], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(unsafe.SliceData(code))), uintptr(len(code)), 0, uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create pixel shader")
}

func (d *ID3D11Device) createBlendState(desc *D3D11_BLEND_DESC) (*ID3D11BlendState, error) {
	var out *ID3D11BlendState
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateBlendState], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create blend state")
}

func (d *ID3D11Device) createRasterizerState(desc *D3D11_RASTERIZER_DESC) (*ID3D11RasterizerState, error) {
	var out *ID3D11RasterizerState
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateRasterizerState], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("create rasterizer state")
}

func (d *ID3D11Device) createQuery(desc *D3D11_QUERY_DESC) (*ID3D11Query, error) {
	var q *ID3D11Query
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_CreateQuery], uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)), uintptr(unsafe.Pointer(&q)))
	return q, hr(r).err("CreateQuery")
}

func (d *ID3D11Device) deviceRemovedReason() hresult {
	r, _, _ := syscall.SyscallN(d.vtbl[iD3D11Device_GetDeviceRemovedReason], uintptr(unsafe.Pointer(d)))
	return hr(r)
}

// The context methods run on every frame. Their pointer arguments refer to
// fields of long-lived structs so that nothing escapes to the heap.

func (c *ID3D11DeviceContext) vsSetConstantBuffer(slot uint32, b **ID3D11Buffer) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_VSSetConstantBuffers], uintptr(unsafe.Pointer(c)),
		uintptr(slot), 1, uintptr(unsafe.Pointer(b)))
}

func (c *ID3D11DeviceContext) psSetConstantBuffer(slot uint32, b **ID3D11Buffer) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_PSSetConstantBuffers], uintptr(unsafe.Pointer(c)),
		uintptr(slot), 1, uintptr(unsafe.Pointer(b)))
}

func (c *ID3D11DeviceContext) vsSetShader(s *ID3D11VertexShader) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_VSSetShader], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(s)), 0, 0)
}

func (c *ID3D11DeviceContext) psSetShader(s *ID3D11PixelShader) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_PSSetShader], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(s)), 0, 0)
}

func (c *ID3D11DeviceContext) drawInstanced(vertexCount, instanceCount, startVertex, startInstance uint32) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_DrawInstanced], uintptr(unsafe.Pointer(c)),
		uintptr(vertexCount), uintptr(instanceCount), uintptr(startVertex), uintptr(startInstance))
}

func (c *ID3D11DeviceContext) mapResource(resource uintptr, mapType uint32, out *D3D11_MAPPED_SUBRESOURCE) error {
	r, _, _ := syscall.SyscallN(c.vtbl[iD3D11DeviceContext_Map], uintptr(unsafe.Pointer(c)),
		resource, 0, uintptr(mapType), 0, uintptr(unsafe.Pointer(out)))
	return hr(r).err("map")
}

func (c *ID3D11DeviceContext) unmapResource(resource uintptr) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_Unmap], uintptr(unsafe.Pointer(c)), resource, 0)
}

func (c *ID3D11DeviceContext) iaSetInputLayout(l *ID3D11InputLayout) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_IASetInputLayout], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(l)))
}

func (c *ID3D11DeviceContext) iaSetVertexBuffer(b **ID3D11Buffer, stride, offset *uint32) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_IASetVertexBuffers], uintptr(unsafe.Pointer(c)),
		0, 1, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(stride)), uintptr(unsafe.Pointer(offset)))
}

func (c *ID3D11DeviceContext) iaSetPrimitiveTopology(topology uint32) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_IASetPrimitiveTopology], uintptr(unsafe.Pointer(c)),
		uintptr(topology))
}

func (c *ID3D11DeviceContext) omSetRenderTarget(rtv **ID3D11RenderTargetView, dsv *ID3D11DepthStencilView) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_OMSetRenderTargets], uintptr(unsafe.Pointer(c)),
		1, uintptr(unsafe.Pointer(rtv)), uintptr(unsafe.Pointer(dsv)))
}

func (c *ID3D11DeviceContext) omSetDepthStencilState(s *ID3D11DepthStencilState) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_OMSetDepthStencilState], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(s)), 0)
}

func (c *ID3D11DeviceContext) clearDepthStencilView(v *ID3D11DepthStencilView) {
	// Depth is a float argument. The Go runtime copies every argument to the
	// matching XMM register too, and the bits of 0 are also 0.0.
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_ClearDepthStencilView], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(v)), D3D11_CLEAR_DEPTH|D3D11_CLEAR_STENCIL, 0, 0)
}

func (c *ID3D11DeviceContext) resolveSubresource(dst, src *ID3D11Texture2D, format uint32) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_ResolveSubresource], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(dst)), 0, uintptr(unsafe.Pointer(src)), 0, uintptr(format))
}

func (c *ID3D11DeviceContext) draw(vertexCount, startVertex uint32) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_Draw], uintptr(unsafe.Pointer(c)),
		uintptr(vertexCount), uintptr(startVertex))
}

func (c *ID3D11DeviceContext) psSetSampler(s **ID3D11SamplerState) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_PSSetSamplers], uintptr(unsafe.Pointer(c)),
		0, 1, uintptr(unsafe.Pointer(s)))
}

func (c *ID3D11DeviceContext) omSetBlendState(s *ID3D11BlendState) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_OMSetBlendState], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(s)), 0, 0xFFFFFFFF)
}

func (c *ID3D11DeviceContext) rsSetState(s *ID3D11RasterizerState) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_RSSetState], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(s)))
}

func (c *ID3D11DeviceContext) rsSetViewport(v *D3D11_VIEWPORT) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_RSSetViewports], uintptr(unsafe.Pointer(c)),
		1, uintptr(unsafe.Pointer(v)))
}

func (c *ID3D11DeviceContext) rsSetScissorRect(r *D3D11_RECT) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_RSSetScissorRects], uintptr(unsafe.Pointer(c)),
		1, uintptr(unsafe.Pointer(r)))
}

// queryContext1 returns the newer interface of c, which comes with Windows 8.
func (c *ID3D11DeviceContext) queryContext1() (*ID3D11DeviceContext1, error) {
	var out *ID3D11DeviceContext1
	r, _, _ := syscall.SyscallN(c.vtbl[iUnknown_QueryInterface], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(&IID_ID3D11DeviceContext1)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r).err("query ID3D11DeviceContext1")
}

// clearView fills r of v with color.
func (c *ID3D11DeviceContext1) clearView(v *ID3D11RenderTargetView, color *[4]float32, r *D3D11_RECT) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext1_ClearView], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(color)), uintptr(unsafe.Pointer(r)), 1)
}

func (c *ID3D11DeviceContext) psSetShaderResource(slot uint32, v **ID3D11ShaderResourceView) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_PSSetShaderResources], uintptr(unsafe.Pointer(c)),
		uintptr(slot), 1, uintptr(unsafe.Pointer(v)))
}

// updateSubresource copies pix into box of t. The driver finishes reading pix
// before the call returns.
func (c *ID3D11DeviceContext) updateSubresource(t *ID3D11Texture2D, subresource uint32, box *D3D11_BOX, pix *byte, rowPitch uint32) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_UpdateSubresource], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(t)), uintptr(subresource), uintptr(unsafe.Pointer(box)), uintptr(unsafe.Pointer(pix)), uintptr(rowPitch), 0)
}

// copySubresourceRegion copies box of src to dst, with its top left at x, y.
func (c *ID3D11DeviceContext) copySubresourceRegion(dst *ID3D11Texture2D, x, y uint32, src *ID3D11Texture2D, box *D3D11_BOX) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_CopySubresourceRegion], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(dst)), 0, uintptr(x), uintptr(y), 0, uintptr(unsafe.Pointer(src)), 0, uintptr(unsafe.Pointer(box)))
}

func (c *ID3D11DeviceContext) copyResource(dst, src *ID3D11Texture2D) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_CopyResource], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(dst)), uintptr(unsafe.Pointer(src)))
}

func (c *ID3D11DeviceContext) clearRenderTargetView(rtv *ID3D11RenderTargetView, rgba *[4]float32) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_ClearRenderTargetView], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(rtv)), uintptr(unsafe.Pointer(rgba)))
}

func (c *ID3D11DeviceContext) begin(q *ID3D11Query) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_Begin], uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(q)))
}

func (c *ID3D11DeviceContext) end(q *ID3D11Query) {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_End], uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(q)))
}

// getData reads the result of q into the size bytes at data. It reports S_FALSE,
// which is 1, while the GPU has not got that far.
func (c *ID3D11DeviceContext) getData(q *ID3D11Query, data unsafe.Pointer, size, flags uint32) hresult {
	r, _, _ := syscall.SyscallN(c.vtbl[iD3D11DeviceContext_GetData], uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(q)), uintptr(data), uintptr(size), uintptr(flags))
	return hr(r)
}

func (c *ID3D11DeviceContext) clearState() {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_ClearState], uintptr(unsafe.Pointer(c)))
}

func (c *ID3D11DeviceContext) flush() {
	_, _, _ = syscall.SyscallN(c.vtbl[iD3D11DeviceContext_Flush], uintptr(unsafe.Pointer(c)))
}

// adapterName returns the description of d as a Go string.
func adapterName(d *DXGI_ADAPTER_DESC1) string { return windows.UTF16ToString(d.Description[:]) }
