// The prelude that gowaku-shader puts before a custom shader. It declares
// ShaderInput, the textures and the helpers sample0, size0, sample1 and size1.
// Every other name starts with gowaku, blend or BLEND_. See
// docs/ADR/0045-shaders-svg-animated-images.md.

// The constants of the frame, cbuffer Frame of gpu/shaders/sdf.hlsli.
cbuffer GowakuFrame : register(b0) {
    float2 gowakuViewport; // physical pixels
    float gowakuScale;     // physical pixels per logical pixel
    float gowakuPadding;
};

// GowakuVaryings is Varyings of gpu/shaders/sdf.hlsli, what the vertex shader
// of gowaku writes. A test keeps the two the same. See gpu.SetShaderParams for
// what an InstanceShader keeps where.
struct GowakuVaryings {
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

// The textures, premultiplied. tex0 takes the slot of the image of the SDF
// shader, tex1 that of the glyph atlas, and the backdrop of blend modes that
// read the target keeps its own.
Texture2D<float4> gowakuTex1 : register(t0);
Texture2D<float4> gowakuTex0 : register(t1);
Texture2D<float4> gowakuBackdrop : register(t3);

#define BLEND_BACKDROP gowakuBackdrop
// The blend shift of the kind of an instance, BLEND_SHIFT of sdf.hlsli.
#define GOWAKU_BLEND_SHIFT 8

struct ShaderInput {
    float2 pos;       // the pixel center from the top left, in logical pixels
    float2 size;      // the size of the rectangle, in logical pixels
    float2 uv;        // pos / size
    float time;       // seconds
    float scale;      // physical pixels per logical pixel
    float4 params[4]; // the sixteen floats of paint.ShaderParams
};

// Where the textures lie, which main sets: the texel position at the top left
// and the bottom right of the rectangle, and the texels that hold the picture.
static float4 gowakuPlace0;
static float4 gowakuTexels0;
static float4 gowakuPlace1;
static float4 gowakuTexels1;

float4 gowakuTexel0(int2 t) {
    float2 p = float2(t);
    if (any(p < gowakuTexels0.xy) || any(p >= gowakuTexels0.zw)) {
        return float4(0.0, 0.0, 0.0, 0.0);
    }
    return gowakuTex0.Load(int3(t, 0));
}

float4 gowakuTexel1(int2 t) {
    float2 p = float2(t);
    if (any(p < gowakuTexels1.xy) || any(p >= gowakuTexels1.zw)) {
        return float4(0.0, 0.0, 0.0, 0.0);
    }
    return gowakuTex1.Load(int3(t, 0));
}

// sample0 blends the four texels of tex0 around uv, where 0 and 1 are the
// edges of the rectangle, with exact weights. Texels outside the picture read
// as transparent. It reads with Load, so that every GPU weighs the texels alike.
float4 sample0(float2 uv) {
    float2 at = lerp(gowakuPlace0.xy, gowakuPlace0.zw, uv) - 0.5;
    float2 base = floor(at);
    float2 f = at - base;
    int2 t = int2(base);
    float4 top = lerp(gowakuTexel0(t), gowakuTexel0(t + int2(1, 0)), f.x);
    float4 bottom = lerp(gowakuTexel0(t + int2(0, 1)), gowakuTexel0(t + int2(1, 1)), f.x);
    return lerp(top, bottom, f.y);
}

// sample1 is sample0 for tex1.
float4 sample1(float2 uv) {
    float2 at = lerp(gowakuPlace1.xy, gowakuPlace1.zw, uv) - 0.5;
    float2 base = floor(at);
    float2 f = at - base;
    int2 t = int2(base);
    float4 top = lerp(gowakuTexel1(t), gowakuTexel1(t + int2(1, 0)), f.x);
    float4 bottom = lerp(gowakuTexel1(t + int2(0, 1)), gowakuTexel1(t + int2(1, 1)), f.x);
    return lerp(top, bottom, f.y);
}

// size0 is how many texels of tex0 span the rectangle along each axis: one
// texel is 1 / size0() of uv.
float2 size0() {
    return abs(gowakuPlace0.zw - gowakuPlace0.xy);
}

// size1 is size0 for tex1.
float2 size1() {
    return abs(gowakuPlace1.zw - gowakuPlace1.xy);
}
