package d3d11

import (
	"testing"
	"unsafe"

	"github.com/bankai-tensei/gowaku/gpu"
)

// The expected values were printed by a C program built against the
// mingw-w64 headers, not derived from the Go definitions.
func TestABI_StructLayout(t *testing.T) {
	tests := []struct {
		name      string
		got, want uintptr
	}{
		{"sizeof DXGI_SAMPLE_DESC", unsafe.Sizeof(DXGI_SAMPLE_DESC{}), 8},
		{"sizeof DXGI_SWAP_CHAIN_DESC1", unsafe.Sizeof(DXGI_SWAP_CHAIN_DESC1{}), 48},
		{"offsetof DXGI_SWAP_CHAIN_DESC1.SampleDesc", unsafe.Offsetof(DXGI_SWAP_CHAIN_DESC1{}.SampleDesc), 16},
		{"offsetof DXGI_SWAP_CHAIN_DESC1.Flags", unsafe.Offsetof(DXGI_SWAP_CHAIN_DESC1{}.Flags), 44},
		{"sizeof DXGI_ADAPTER_DESC1", unsafe.Sizeof(DXGI_ADAPTER_DESC1{}), 312},
		{"offsetof DXGI_ADAPTER_DESC1.DedicatedVideoMemory", unsafe.Offsetof(DXGI_ADAPTER_DESC1{}.DedicatedVideoMemory), 272},
		{"offsetof DXGI_ADAPTER_DESC1.AdapterLuid", unsafe.Offsetof(DXGI_ADAPTER_DESC1{}.AdapterLuid), 296},
		{"offsetof DXGI_ADAPTER_DESC1.Flags", unsafe.Offsetof(DXGI_ADAPTER_DESC1{}.Flags), 304},
		{"sizeof D3D11_BUFFER_DESC", unsafe.Sizeof(D3D11_BUFFER_DESC{}), 24},
		{"sizeof D3D11_MAPPED_SUBRESOURCE", unsafe.Sizeof(D3D11_MAPPED_SUBRESOURCE{}), 16},
		{"sizeof D3D11_TEXTURE2D_DESC", unsafe.Sizeof(D3D11_TEXTURE2D_DESC{}), 44},
		{"offsetof D3D11_TEXTURE2D_DESC.Usage", unsafe.Offsetof(D3D11_TEXTURE2D_DESC{}.Usage), 28},
		{"sizeof D3D11_INPUT_ELEMENT_DESC", unsafe.Sizeof(D3D11_INPUT_ELEMENT_DESC{}), 32},
		{"offsetof D3D11_INPUT_ELEMENT_DESC.Format", unsafe.Offsetof(D3D11_INPUT_ELEMENT_DESC{}.Format), 12},
		{"offsetof D3D11_INPUT_ELEMENT_DESC.InstanceDataStepRate", unsafe.Offsetof(D3D11_INPUT_ELEMENT_DESC{}.InstanceDataStepRate), 28},
		{"sizeof D3D11_VIEWPORT", unsafe.Sizeof(D3D11_VIEWPORT{}), 24},
		{"sizeof D3D11_RENDER_TARGET_BLEND_DESC", unsafe.Sizeof(D3D11_RENDER_TARGET_BLEND_DESC{}), 32},
		{"sizeof D3D11_BLEND_DESC", unsafe.Sizeof(D3D11_BLEND_DESC{}), 264},
		{"offsetof D3D11_BLEND_DESC.RenderTarget", unsafe.Offsetof(D3D11_BLEND_DESC{}.RenderTarget), 8},
		{"sizeof D3D11_RASTERIZER_DESC", unsafe.Sizeof(D3D11_RASTERIZER_DESC{}), 40},
		{"sizeof D3D11_RECT", unsafe.Sizeof(D3D11_RECT{}), 16},
		{"sizeof D3D11_BOX", unsafe.Sizeof(D3D11_BOX{}), 24},
		{"sizeof D3D11_QUERY_DESC", unsafe.Sizeof(D3D11_QUERY_DESC{}), 8},
		{"sizeof D3D11_QUERY_DATA_TIMESTAMP_DISJOINT", unsafe.Sizeof(D3D11_QUERY_DATA_TIMESTAMP_DISJOINT{}), 16},
		{"sizeof D3D11_DEPTH_STENCIL_DESC", unsafe.Sizeof(D3D11_DEPTH_STENCIL_DESC{}), 52},
		{"offsetof D3D11_DEPTH_STENCIL_DESC.FrontFace", unsafe.Offsetof(D3D11_DEPTH_STENCIL_DESC{}.FrontFace), 20},
		{"sizeof D3D11_SAMPLER_DESC", unsafe.Sizeof(D3D11_SAMPLER_DESC{}), 52},
		{"offsetof D3D11_SAMPLER_DESC.BorderColor", unsafe.Offsetof(D3D11_SAMPLER_DESC{}.BorderColor), 28},
		// The input layout and cbuffer in the shaders rely on these.
		{"sizeof gpu.Instance", unsafe.Sizeof(gpu.Instance{}), 200},
		{"offsetof gpu.Instance.Radii", unsafe.Offsetof(gpu.Instance{}.Radii), 16},
		{"offsetof gpu.Instance.Color", unsafe.Offsetof(gpu.Instance{}.Color), 32},
		{"offsetof gpu.Instance.EndColor", unsafe.Offsetof(gpu.Instance{}.EndColor), 48},
		{"offsetof gpu.Instance.BorderColor", unsafe.Offsetof(gpu.Instance{}.BorderColor), 64},
		{"offsetof gpu.Instance.GradientStart", unsafe.Offsetof(gpu.Instance{}.GradientStart), 80},
		{"offsetof gpu.Instance.GradientEnd", unsafe.Offsetof(gpu.Instance{}.GradientEnd), 88},
		{"offsetof gpu.Instance.BorderWidth", unsafe.Offsetof(gpu.Instance{}.BorderWidth), 96},
		{"offsetof gpu.Instance.Kind", unsafe.Offsetof(gpu.Instance{}.Kind), 104},
		{"offsetof gpu.Instance.MidColors", unsafe.Offsetof(gpu.Instance{}.MidColors), 112},
		{"offsetof gpu.Instance.StopOffsets", unsafe.Offsetof(gpu.Instance{}.StopOffsets), 144},
		{"offsetof gpu.Instance.Transform", unsafe.Offsetof(gpu.Instance{}.Transform), 160},
		{"offsetof gpu.Instance.GradientMatrix", unsafe.Offsetof(gpu.Instance{}.GradientMatrix), 184},
		{"sizeof frameConstants", unsafe.Sizeof(frameConstants{}), 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %d, want %d", tt.got, tt.want)
			}
		})
	}
}

func TestHresult_Err(t *testing.T) {
	if err := hr(0).err("op"); err != nil {
		t.Errorf("S_OK: got %v, want nil", err)
	}
	// DXGI_STATUS_OCCLUDED is a success code that Present returns routinely.
	if err := hr(0x087A0001).err("op"); err != nil {
		t.Errorf("DXGI_STATUS_OCCLUDED: got %v, want nil", err)
	}
	if err := hr(DXGI_ERROR_DEVICE_REMOVED).err("op"); err == nil {
		t.Error("DXGI_ERROR_DEVICE_REMOVED: got nil, want an error")
	}
}
