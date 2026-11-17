// effect is the custom shader of the passes that filters and masks of groups
// draw, compiled by cmd/gowaku-shader. Each instance is one gpu.EffectOp: see
// gpu/effect.go for what it computes and where its parameters lie, and
// effect.go of gpu/soft for the same in Go, which must stay in step with this.
// See docs/ADR/0114-gpu-layers-masks-filters.md.
//
// The effects draw with a scale of 1 and Rect at the origin, so that pos is the
// texel under the pixel. The textures hold premultiplied sRGB.

#define EFFECT_COPY 0
#define EFFECT_FLOOD 1
#define EFFECT_MATRIX 2
#define EFFECT_TRANSFER 3
#define EFFECT_COMPOSITE 4
#define EFFECT_BLEND 5
#define EFFECT_MORPHOLOGY 6
#define EFFECT_GAUSSIAN 7
#define EFFECT_BOXES 8
#define EFFECT_MASK 9
#define EFFECT_SHADOW 10

#define EFFECT_LINEAR 256
#define EFFECT_ALPHA0 512
#define EFFECT_ALPHA1 1024
#define EFFECT_EMPTY0 2048
#define EFFECT_EMPTY1 4096
#define EFFECT_DILATE 8192
#define EFFECT_LUMINANCE 16384

#define COMPOSITE_OVER 0
#define COMPOSITE_IN 1
#define COMPOSITE_OUT 2
#define COMPOSITE_ATOP 3
#define COMPOSITE_XOR 4
#define COMPOSITE_ARITHMETIC 5
#define COMPOSITE_LIGHTER 6

static uint effectFlags;
static int2 effectSize;

float effectToLinear(float c) {
    return c <= 0.04045 ? c / 12.92 : pow((c + 0.055) / 1.055, 2.4);
}

float effectToSRGB(float c) {
    return c <= 0.0031308 ? c * 12.92 : 1.055 * pow(c, 1.0 / 2.4) - 0.055;
}

// effectDecode takes a stored color into the space of the effect.
float4 effectDecode(float4 c) {
    if ((effectFlags & EFFECT_LINEAR) == 0 || c.a <= 0.0) {
        return c;
    }
    float3 u = saturate(c.rgb / c.a);
    return float4(float3(effectToLinear(u.r), effectToLinear(u.g), effectToLinear(u.b)) * c.a, c.a);
}

// effectEncode takes a color of the space of the effect back to sRGB.
float4 effectEncode(float4 c) {
    if ((effectFlags & EFFECT_LINEAR) == 0 || c.a <= 0.0) {
        return c;
    }
    float3 u = saturate(c.rgb / c.a);
    return float4(float3(effectToSRGB(u.r), effectToSRGB(u.g), effectToSRGB(u.b)) * c.a, c.a);
}

bool effectOutside(int2 t) {
    return any(t < int2(0, 0)) || any(t >= effectSize);
}

// effectIn0 and effectIn1 read a texel of an input in the space of the effect,
// transparent outside the region.
float4 effectIn0(int2 t) {
    if ((effectFlags & EFFECT_EMPTY0) != 0 || effectOutside(t)) {
        return float4(0.0, 0.0, 0.0, 0.0);
    }
    float4 c = gowakuTex0.Load(int3(t, 0));
    if ((effectFlags & EFFECT_ALPHA0) != 0) {
        return float4(0.0, 0.0, 0.0, c.a);
    }
    return effectDecode(c);
}

float4 effectIn1(int2 t) {
    if ((effectFlags & EFFECT_EMPTY1) != 0 || effectOutside(t)) {
        return float4(0.0, 0.0, 0.0, 0.0);
    }
    float4 c = gowakuTex1.Load(int3(t, 0));
    if ((effectFlags & EFFECT_ALPHA1) != 0) {
        return float4(0.0, 0.0, 0.0, c.a);
    }
    return effectDecode(c);
}

// effectMoved reads In0 at t moved by d, blending the four texels around it as
// feOffset does between whole pixels.
float4 effectMoved(int2 t, float2 d) {
    float2 whole = floor(d);
    float2 f = d - whole;
    int2 s = t - int2(whole);
    float4 top = lerp(effectIn0(s), effectIn0(s - int2(1, 0)), f.x);
    float4 bottom = lerp(effectIn0(s - int2(0, 1)), effectIn0(s - int2(1, 1)), f.x);
    return lerp(top, bottom, f.y);
}

float4 effectUnpremultiply(float4 c) {
    return c.a > 0.0 ? float4(c.rgb / c.a, c.a) : float4(0.0, 0.0, 0.0, 0.0);
}

// effectLookup reads the transfer function of one channel, row of In1, at v.
float effectLookup(float v, int row, int channel) {
    float x = saturate(v) * 255.0;
    float i = min(floor(x), 254.0);
    float low = gowakuTex1.Load(int3(int(i), row, 0))[channel];
    float high = gowakuTex1.Load(int3(int(i) + 1, row, 0))[channel];
    return lerp(low, high, x - i);
}

float effectComposite(float a, float b, float aa, float ba, uint op, float4 k) {
    if (op == COMPOSITE_IN) {
        return a * ba;
    }
    if (op == COMPOSITE_OUT) {
        return a * (1.0 - ba);
    }
    if (op == COMPOSITE_ATOP) {
        return a * ba + b * (1.0 - aa);
    }
    if (op == COMPOSITE_XOR) {
        return a * (1.0 - ba) + b * (1.0 - aa);
    }
    if (op == COMPOSITE_LIGHTER) {
        return min(a + b, 1.0);
    }
    if (op == COMPOSITE_ARITHMETIC) {
        return saturate(k.x * a * b + k.y * a + k.z * b + k.w);
    }
    return a + b * (1.0 - aa);
}

// effectBoxTriangle and effectBoxWeight are gpu.BoxWeight.
float effectBoxTriangle(float n) {
    return n < 0.0 ? 0.0 : (n + 1.0) * (n + 2.0) * 0.5;
}

float effectBoxWeight(float m, float3 d) {
    float n = effectBoxTriangle(m) - effectBoxTriangle(m - d.x) - effectBoxTriangle(m - d.y) - effectBoxTriangle(m - d.z) +
        effectBoxTriangle(m - d.x - d.y) + effectBoxTriangle(m - d.x - d.z) + effectBoxTriangle(m - d.y - d.z) -
        effectBoxTriangle(m - d.x - d.y - d.z);
    return n / (d.x * d.y * d.z);
}

float4 effect(int2 t, uint op, float4 p0, float4 p1, float4 p4, float4 p5, float4 p6, float4 p7) {
    if (op == EFFECT_COPY) {
        return effectMoved(t, p0.xy);
    }
    if (op == EFFECT_FLOOD) {
        return effectDecode(p0);
    }
    if (op == EFFECT_MATRIX) {
        float4 u = effectUnpremultiply(effectIn0(t));
        float4 v = saturate(float4(dot(p4, u), dot(p5, u), dot(p6, u), dot(p7, u)) + p0);
        return float4(v.rgb * v.a, v.a);
    }
    if (op == EFFECT_TRANSFER) {
        float4 u = effectUnpremultiply(effectIn0(t));
        int row = int(p0.x);
        float4 v = saturate(float4(
            effectLookup(u.r, row, 0), effectLookup(u.g, row, 1), effectLookup(u.b, row, 2), effectLookup(u.a, row, 3)));
        return float4(v.rgb * v.a, v.a);
    }
    if (op == EFFECT_COMPOSITE) {
        float4 a = effectIn0(t);
        float4 b = effectIn1(t);
        uint oper = uint(p0.x);
        float4 c = float4(
            effectComposite(a.r, b.r, a.a, b.a, oper, p1), effectComposite(a.g, b.g, a.a, b.a, oper, p1),
            effectComposite(a.b, b.b, a.a, b.a, oper, p1), effectComposite(a.a, b.a, a.a, b.a, oper, p1));
        if (oper == COMPOSITE_ARITHMETIC) {
            c.rgb = min(c.rgb, c.a);
        }
        return c;
    }
    if (op == EFFECT_BLEND) {
        float4 a = effectIn0(t);
        float4 b = effectIn1(t);
        uint mode = uint(p0.x);
        float3 s = a.a > 0.0 ? a.rgb / a.a : float3(0.0, 0.0, 0.0);
        float3 d = b.a > 0.0 ? b.rgb / b.a : float3(0.0, 0.0, 0.0);
        float3 mixed = s;
        if (mode == BLEND_SCREEN) {
            mixed = s + d - s * d;
        } else if (mode >= BLEND_MULTIPLY) {
            mixed = blendMix(saturate(d), saturate(s), mode);
        }
        return float4((1.0 - b.a) * a.rgb + (1.0 - a.a) * b.rgb + a.a * b.a * mixed, a.a + b.a - a.a * b.a);
    }
    if (op == EFFECT_MORPHOLOGY) {
        int2 axis = int2(p0.xy);
        int radius = int(p0.z);
        bool dilate = (effectFlags & EFFECT_DILATE) != 0;
        float4 c = effectIn0(t);
        [loop]
        for (int k = -radius; k <= radius; k++) {
            int2 s = t + axis * k;
            if (effectOutside(s)) {
                continue;
            }
            float4 v = effectIn0(s);
            c = dilate ? max(c, v) : min(c, v);
        }
        return c;
    }
    if (op == EFFECT_GAUSSIAN) {
        int2 axis = int2(p0.xy);
        float falloff = -0.5 / (p0.z * p0.z);
        int radius = int(p0.w);
        float4 sum = float4(0.0, 0.0, 0.0, 0.0);
        float total = 0.0;
        [loop]
        for (int k = -radius; k <= radius; k++) {
            float w = exp(float(k * k) * falloff);
            sum += effectIn0(t + axis * k) * w;
            total += w;
        }
        return sum / total;
    }
    if (op == EFFECT_BOXES) {
        int2 axis = int2(p0.xy);
        int first = int(p1.w);
        int span = int(p1.x + p1.y + p1.z) - 2;
        float4 sum = float4(0.0, 0.0, 0.0, 0.0);
        [loop]
        for (int m = 0; m < span; m++) {
            sum += effectIn0(t + axis * (m - first)) * effectBoxWeight(float(m), p1.xyz);
        }
        return sum;
    }
    if (op == EFFECT_MASK) {
        float4 c = gowakuTex0.Load(int3(t, 0));
        float4 m = gowakuTex1.Load(int3(t, 0));
        if (effectOutside(t)) {
            return float4(0.0, 0.0, 0.0, 0.0);
        }
        float v = (effectFlags & EFFECT_LUMINANCE) != 0 ? dot(m.rgb, float3(0.2125, 0.7154, 0.0721)) : m.a;
        return c * v;
    }
    // EFFECT_SHADOW
    float a = effectMoved(t, p0.xy).a;
    float4 top = effectIn1(t);
    return top + effectDecode(p1) * (a * (1.0 - top.a));
}

float4 shade(ShaderInput i) {
    uint word = uint(i.time);
    effectFlags = word;
    effectSize = int2(i.params[3].xy);
    int2 t = int2(floor(i.pos));
    float4 c = effect(t, word & 255u, i.params[0], i.params[1], gowakuPlace0, gowakuTexels0, gowakuPlace1, gowakuTexels1);
    return effectEncode(c);
}
