// Shared by sdf.vs.hlsl and sdf.ps.hlsl. Instance mirrors gpu.Instance.

#define KIND_RRECT 0
#define KIND_SHADOW 1
#define KIND_GLYPH 2
#define KIND_COVER 3
#define KIND_IMAGE 4
#define KIND_BACKDROP 5
#define KIND_FILTERED 6
#define KIND_BLUR_DOWN 7
#define KIND_BLUR_X 8
#define KIND_BLUR_Y 9

// The bits of the kind from here up hold the blend mode of the draw call.
#define BLEND_SHIFT 8
#define KIND_MASK 0xFF

#define GRADIENT_NONE 0
#define GRADIENT_LINEAR 1
#define GRADIENT_RADIAL 2
#define GRADIENT_SWEEP 3
#define GRADIENT_FOCAL 4
#define GRADIENT_SHAPE 15
#define GRADIENT_RAMP 16
#define GRADIENT_REFLECT 32
#define GRADIENT_REPEAT 64

// The filters of gpu.ImageFilter, which an image keeps in its gradient mode.
#define FILTER_LINEAR 0
#define FILTER_NEAREST 1
#define FILTER_MIPMAP 2

cbuffer Frame : register(b0) {
    float2 viewport; // physical pixels
    float scale;     // physical pixels per logical pixel
    float padding;
};

struct Instance {
    float4 rect : RECT;           // x, y, width, height
    float4 radii : RADII;         // clockwise from the top left
    float4 color : COLOR0;
    float4 endColor : COLOR1;
    float4 borderColor : COLOR2;
    float4 gradient : GRADIENT;   // start x, y, end x, y
    float2 params : PARAMS;       // border width, blur radius
    uint2 mode : MODE;            // kind, gradient mode
    float4 midColor0 : COLOR3;    // second gradient stop
    float4 midColor1 : COLOR4;    // third gradient stop
    float4 stops : STOPS;         // offsets of the four gradient stops
    float4 transform : TRANSFORM0;   // a, b, c, d of the transform to the surface
    float2 translation : TRANSFORM1; // e, f
    float4 gradientMatrix : GRADIENT1; // from the instance to the gradient
};

struct Varyings {
    float4 position : SV_Position;
    float2 logical : LOGICAL;
    nointerpolation float4 rect : RECT;
    nointerpolation float4 radii : RADII;
    nointerpolation float4 color : COLOR0;
    nointerpolation float4 endColor : COLOR1;
    nointerpolation float4 borderColor : COLOR2;
    nointerpolation float4 gradient : GRADIENT;
    nointerpolation float2 params : PARAMS;
    nointerpolation uint2 mode : MODE;
    nointerpolation float4 midColor0 : COLOR3;
    nointerpolation float4 midColor1 : COLOR4;
    nointerpolation float4 stops : STOPS;
    nointerpolation float4 transform : TRANSFORM0;
    nointerpolation float2 translation : TRANSFORM1;
    nointerpolation float4 gradientMatrix : GRADIENT1;
};

// turned reports whether an instance has a transform. All zeros means none,
// and then everything takes the exact path of upright quads.
bool turned(float4 transform) {
    return any(transform != 0.0);
}
