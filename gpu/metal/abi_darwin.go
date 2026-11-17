//go:build darwin && arm64

package metal

// The structs and enumerations of Metal.h that gowaku uses, with the names of
// the headers. NSUInteger is 64 bits on arm64.

// MTLPixelFormat
const (
	MTLPixelFormatInvalid                = 0
	MTLPixelFormatR8Unorm                = 10
	MTLPixelFormatRGBA8Unorm             = 70
	MTLPixelFormatBGRA8Unorm             = 80
	MTLPixelFormatDepth32Float_Stencil8  = 260
	MTLTextureType2D                     = 2
	MTLTextureType2DMultisample          = 4
	MTLTextureUsageShaderRead            = 0x1
	MTLTextureUsageRenderTarget          = 0x4
	MTLStorageModeShared                 = 0
	MTLStorageModePrivate                = 2
	MTLResourceStorageModeShared         = MTLStorageModeShared << 4
	MTLResourceCPUCacheModeWriteCombined = 1
)

// MTLLoadAction and MTLStoreAction
const (
	MTLLoadActionDontCare = 0
	MTLLoadActionLoad     = 1
	MTLLoadActionClear    = 2

	MTLStoreActionDontCare                   = 0
	MTLStoreActionStore                      = 1
	MTLStoreActionMultisampleResolve         = 2
	MTLStoreActionStoreAndMultisampleResolve = 3
	MTLStoreActionUnknown                    = 4
)

// MTLPrimitiveType
const (
	MTLPrimitiveTypeTriangle      = 3
	MTLPrimitiveTypeTriangleStrip = 4
)

// MTLCompareFunction
const (
	MTLCompareFunctionLess     = 1
	MTLCompareFunctionEqual    = 2
	MTLCompareFunctionGreater  = 4
	MTLCompareFunctionNotEqual = 5
	MTLCompareFunctionAlways   = 7
)

// MTLStencilOperation
const (
	MTLStencilOperationKeep          = 0
	MTLStencilOperationZero          = 1
	MTLStencilOperationInvert        = 5
	MTLStencilOperationIncrementWrap = 6
	MTLStencilOperationDecrementWrap = 7
)

// MTLBlendFactor, MTLBlendOperation and MTLColorWriteMask
const (
	MTLBlendFactorZero                     = 0
	MTLBlendFactorOne                      = 1
	MTLBlendFactorSourceColor              = 2
	MTLBlendFactorOneMinusSourceColor      = 3
	MTLBlendFactorSourceAlpha              = 4
	MTLBlendFactorOneMinusSourceAlpha      = 5
	MTLBlendFactorDestinationAlpha         = 8
	MTLBlendFactorOneMinusDestinationAlpha = 9
	MTLBlendOperationAdd                   = 0
	MTLColorWriteMaskNone                  = 0
	MTLColorWriteMaskAll                   = 0xF
)

// MTLCommandBufferStatus
const (
	MTLCommandBufferStatusCompleted = 4
	MTLCommandBufferStatusError     = 5
)

// MTLMathMode and MTLMathFloatingPointFunctions
const (
	MTLMathModeSafe                      = 0
	MTLMathFloatingPointFunctionsPrecise = 1
)

// MTLViewport is passed by pointer: it is a composite of more than 16 bytes
// and more than four members, which the AAPCS64 hands over in memory.
type MTLViewport struct {
	originX, originY, width, height, znear, zfar float64
}

// MTLScissorRect is passed by pointer, as MTLViewport.
type MTLScissorRect struct {
	x, y, width, height uint64
}

// MTLOrigin and MTLSize are passed by pointer, as MTLViewport.
type MTLOrigin struct{ x, y, z uint64 }

// MTLSize is the size of a region of a texture.
type MTLSize struct{ width, height, depth uint64 }

// MTLRegion is the region of replaceRegion and getBytes.
type MTLRegion struct {
	origin MTLOrigin
	size   MTLSize
}

// MTLClearColor is a homogeneous aggregate of four doubles, which goes in
// floating-point registers: it takes objc.Send, not the integer path.
type MTLClearColor struct {
	red, green, blue, alpha float64
}

// CGSize is the drawableSize of a CAMetalLayer, two doubles.
type CGSize struct {
	width, height float64
}

// NSUTF8StringEncoding is the encoding of Go strings.
const NSUTF8StringEncoding = 4
