package d3d11

import (
	"golang.org/x/sys/windows"
)

// Constants and structures keep their header names so they can be searched for
// (CODING_STYLE §8).
const (
	D3D11_SDK_VERSION                = 7
	D3D11_CREATE_DEVICE_BGRA_SUPPORT = 0x20
	D3D_DRIVER_TYPE_UNKNOWN          = 0
	D3D_FEATURE_LEVEL_11_0           = 0xb000
	D3D_FEATURE_LEVEL_11_1           = 0xb100

	DXGI_FORMAT_UNKNOWN            = 0
	DXGI_FORMAT_R32G32B32A32_FLOAT = 2
	DXGI_FORMAT_R32G32_FLOAT       = 16
	DXGI_FORMAT_R32G32_UINT        = 17
	DXGI_FORMAT_R8G8B8A8_UNORM     = 28
	DXGI_FORMAT_R8_UNORM           = 61
	DXGI_FORMAT_D24_UNORM_S8_UINT  = 45
	DXGI_FORMAT_B8G8R8A8_UNORM     = 87

	DXGI_USAGE_RENDER_TARGET_OUTPUT = 0x20
	DXGI_SWAP_EFFECT_FLIP_DISCARD   = 4

	DXGI_SWAP_CHAIN_FLAG_ALLOW_TEARING = 2048
	DXGI_PRESENT_ALLOW_TEARING         = 0x200

	DXGI_MWA_NO_ALT_ENTER      = 2
	DXGI_ADAPTER_FLAG_SOFTWARE = 2

	DXGI_GPU_PREFERENCE_MINIMUM_POWER    = 1
	DXGI_GPU_PREFERENCE_HIGH_PERFORMANCE = 2

	DXGI_ERROR_NOT_FOUND      = 0x887A0002
	DXGI_ERROR_DEVICE_REMOVED = 0x887A0005
	DXGI_ERROR_DEVICE_HUNG    = 0x887A0006
	DXGI_ERROR_DEVICE_RESET   = 0x887A0007

	D3D11_USAGE_DEFAULT = 0
	D3D11_USAGE_DYNAMIC = 2
	D3D11_USAGE_STAGING = 3

	D3D11_BIND_VERTEX_BUFFER   = 0x1
	D3D11_BIND_CONSTANT_BUFFER = 0x4
	D3D11_BIND_SHADER_RESOURCE = 0x8
	D3D11_BIND_RENDER_TARGET   = 0x20
	D3D11_BIND_DEPTH_STENCIL   = 0x40

	D3D11_CLEAR_DEPTH   = 1
	D3D11_CLEAR_STENCIL = 2

	D3D11_DEPTH_WRITE_MASK_ZERO = 0
	D3D11_DEPTH_WRITE_MASK_ALL  = 1

	D3D11_COMPARISON_LESS      = 2
	D3D11_COMPARISON_EQUAL     = 3
	D3D11_COMPARISON_GREATER   = 5
	D3D11_COMPARISON_NOT_EQUAL = 6
	D3D11_COMPARISON_ALWAYS    = 8

	D3D11_STENCIL_OP_KEEP   = 1
	D3D11_STENCIL_OP_ZERO   = 2
	D3D11_STENCIL_OP_INVERT = 6
	D3D11_STENCIL_OP_INCR   = 7
	D3D11_STENCIL_OP_DECR   = 8

	D3D11_FILTER_MIN_MAG_MIP_LINEAR = 0x15
	D3D11_TEXTURE_ADDRESS_CLAMP     = 3
	D3D11_FLOAT32_MAX               = 3.402823466e+38

	D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST = 4
	D3D11_INPUT_PER_VERTEX_DATA           = 0

	D3D11_CPU_ACCESS_WRITE = 0x10000
	D3D11_CPU_ACCESS_READ  = 0x20000

	D3D11_MAP_READ          = 1
	D3D11_MAP_WRITE_DISCARD = 4

	D3D11_INPUT_PER_INSTANCE_DATA = 1

	D3D11_PRIMITIVE_TOPOLOGY_TRIANGLESTRIP = 5

	D3D11_BLEND_ZERO           = 1
	D3D11_BLEND_ONE            = 2
	D3D11_BLEND_SRC_COLOR      = 3
	D3D11_BLEND_INV_SRC_COLOR  = 4
	D3D11_BLEND_SRC_ALPHA      = 5
	D3D11_BLEND_INV_SRC_ALPHA  = 6
	D3D11_BLEND_DEST_ALPHA     = 7
	D3D11_BLEND_INV_DEST_ALPHA = 8
	D3D11_BLEND_OP_ADD         = 1

	D3D11_COLOR_WRITE_ENABLE_ALL = 0xF

	D3D11_FILL_SOLID = 3
	D3D11_CULL_NONE  = 1
)

// DXGI_SAMPLE_DESC is the header's structure of the same name.
type DXGI_SAMPLE_DESC struct {
	Count   uint32
	Quality uint32
}

// DXGI_SWAP_CHAIN_DESC1 is the header's structure of the same name.
type DXGI_SWAP_CHAIN_DESC1 struct {
	Width       uint32
	Height      uint32
	Format      uint32
	Stereo      int32
	SampleDesc  DXGI_SAMPLE_DESC
	BufferUsage uint32
	BufferCount uint32
	Scaling     uint32
	SwapEffect  uint32
	AlphaMode   uint32
	Flags       uint32
}

// DXGI_ADAPTER_DESC1 is the header's structure of the same name.
type DXGI_ADAPTER_DESC1 struct {
	Description           [128]uint16
	VendorID              uint32
	DeviceID              uint32
	SubSysID              uint32
	Revision              uint32
	DedicatedVideoMemory  uintptr
	DedicatedSystemMemory uintptr
	SharedSystemMemory    uintptr
	AdapterLuid           uint64
	Flags                 uint32
}

// D3D11_BUFFER_DESC is the header's structure of the same name.
type D3D11_BUFFER_DESC struct {
	ByteWidth           uint32
	Usage               uint32
	BindFlags           uint32
	CPUAccessFlags      uint32
	MiscFlags           uint32
	StructureByteStride uint32
}

// D3D11_MAPPED_SUBRESOURCE is the header's structure of the same name.
type D3D11_MAPPED_SUBRESOURCE struct {
	pData      *byte
	RowPitch   uint32
	DepthPitch uint32
}

// D3D11_TEXTURE2D_DESC is the header's structure of the same name.
type D3D11_TEXTURE2D_DESC struct {
	Width          uint32
	Height         uint32
	MipLevels      uint32
	ArraySize      uint32
	Format         uint32
	SampleDesc     DXGI_SAMPLE_DESC
	Usage          uint32
	BindFlags      uint32
	CPUAccessFlags uint32
	MiscFlags      uint32
}

// D3D11_DEPTH_STENCILOP_DESC is the header's structure of the same name.
type D3D11_DEPTH_STENCILOP_DESC struct {
	StencilFailOp, StencilDepthFailOp, StencilPassOp, StencilFunc uint32
}

// D3D11_DEPTH_STENCIL_DESC is the header's structure of the same name.
type D3D11_DEPTH_STENCIL_DESC struct {
	DepthEnable      int32
	DepthWriteMask   uint32
	DepthFunc        uint32
	StencilEnable    int32
	StencilReadMask  uint8
	StencilWriteMask uint8
	FrontFace        D3D11_DEPTH_STENCILOP_DESC
	BackFace         D3D11_DEPTH_STENCILOP_DESC
}

// D3D11_SAMPLER_DESC is the header's structure of the same name.
type D3D11_SAMPLER_DESC struct {
	Filter         uint32
	AddressU       uint32
	AddressV       uint32
	AddressW       uint32
	MipLODBias     float32
	MaxAnisotropy  uint32
	ComparisonFunc uint32
	BorderColor    [4]float32
	MinLOD         float32
	MaxLOD         float32
}

// D3D11_QUERY_DESC is the header's structure of the same name.
type D3D11_QUERY_DESC struct {
	Query     uint32
	MiscFlags uint32
}

// D3D11_QUERY_DATA_TIMESTAMP_DISJOINT is the header's structure of the same name.
type D3D11_QUERY_DATA_TIMESTAMP_DISJOINT struct {
	Frequency uint64
	Disjoint  int32
}

// The queries and flags of GPU timing.
const (
	D3D11_QUERY_TIMESTAMP          = 2
	D3D11_QUERY_TIMESTAMP_DISJOINT = 3
	D3D11_ASYNC_GETDATA_DONOTFLUSH = 1
)

// D3D11_BOX is the header's structure of the same name.
type D3D11_BOX struct {
	left, top, front, right, bottom, back uint32
}

// D3D11_INPUT_ELEMENT_DESC is the header's structure of the same name.
type D3D11_INPUT_ELEMENT_DESC struct {
	SemanticName         *byte
	SemanticIndex        uint32
	Format               uint32
	InputSlot            uint32
	AlignedByteOffset    uint32
	InputSlotClass       uint32
	InstanceDataStepRate uint32
}

// D3D11_VIEWPORT is the header's structure of the same name.
type D3D11_VIEWPORT struct {
	TopLeftX float32
	TopLeftY float32
	Width    float32
	Height   float32
	MinDepth float32
	MaxDepth float32
}

// D3D11_RECT is the header's structure of the same name.
type D3D11_RECT struct {
	left, top, right, bottom int32
}

// D3D11_RENDER_TARGET_BLEND_DESC is the header's structure of the same name.
type D3D11_RENDER_TARGET_BLEND_DESC struct {
	BlendEnable           int32
	SrcBlend              uint32
	DestBlend             uint32
	BlendOp               uint32
	SrcBlendAlpha         uint32
	DestBlendAlpha        uint32
	BlendOpAlpha          uint32
	RenderTargetWriteMask uint8
}

// D3D11_BLEND_DESC is the header's structure of the same name.
type D3D11_BLEND_DESC struct {
	AlphaToCoverageEnable  int32
	IndependentBlendEnable int32
	RenderTarget           [8]D3D11_RENDER_TARGET_BLEND_DESC
}

// D3D11_RASTERIZER_DESC is the header's structure of the same name.
type D3D11_RASTERIZER_DESC struct {
	FillMode              uint32
	CullMode              uint32
	FrontCounterClockwise int32
	DepthBias             int32
	DepthBiasClamp        float32
	SlopeScaledDepthBias  float32
	DepthClipEnable       int32
	ScissorEnable         int32
	MultisampleEnable     int32
	AntialiasedLineEnable int32
}

var (
	d3d11DLL = windows.NewLazySystemDLL("d3d11.dll")
	dxgiDLL  = windows.NewLazySystemDLL("dxgi.dll")

	procD3D11CreateDevice  = d3d11DLL.NewProc("D3D11CreateDevice")
	procCreateDXGIFactory1 = dxgiDLL.NewProc("CreateDXGIFactory1")
)

// Interface IDs, checked against the mingw-w64 headers.
var (
	IID_IDXGIFactory2        = windows.GUID{Data1: 0x50c83a1c, Data2: 0xe072, Data3: 0x4c48, Data4: [8]byte{0x87, 0xb0, 0x36, 0x30, 0xfa, 0x36, 0xa6, 0xd0}}
	IID_IDXGIFactory6        = windows.GUID{Data1: 0xc1b6694f, Data2: 0xff09, Data3: 0x44a9, Data4: [8]byte{0xb0, 0x3c, 0x77, 0x90, 0x0a, 0x0a, 0x1d, 0x17}}
	IID_IDXGIAdapter1        = windows.GUID{Data1: 0x29038f61, Data2: 0x3839, Data3: 0x4626, Data4: [8]byte{0x91, 0xfd, 0x08, 0x68, 0x79, 0x01, 0x1a, 0x05}}
	IID_ID3D11DeviceContext1 = windows.GUID{Data1: 0xbb2c6faa, Data2: 0xb5fb, Data3: 0x4082, Data4: [8]byte{0x8e, 0x6b, 0x38, 0x8b, 0x8c, 0xfa, 0x90, 0xe1}}
	IID_ID3D11Texture2D      = windows.GUID{Data1: 0x6f15aaf2, Data2: 0xd208, Data3: 0x4e89, Data4: [8]byte{0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c}}
)
