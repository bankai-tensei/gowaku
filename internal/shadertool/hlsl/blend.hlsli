// The blend modes of gpu.BlendMode, shared by sdf.ps.hlsl and the custom
// shaders of gowaku-shader, which carries a copy of this file
// (internal/shadertool/hlsl/blend.hlsli; a test keeps the two the same).
//
// The file that includes it defines BLEND_BACKDROP as the Texture2D<float4> of
// the backdrop, register t3, and calls blendOutput on the color of a fragment
// whose mode is not source-over. Every name here starts with blend or BLEND_.

#define BLEND_SRC_OVER 0
#define BLEND_CLEAR 1
#define BLEND_SRC 2
#define BLEND_DST 3
#define BLEND_DST_OVER 4
#define BLEND_SRC_IN 5
#define BLEND_DST_IN 6
#define BLEND_SRC_OUT 7
#define BLEND_DST_OUT 8
#define BLEND_SRC_ATOP 9
#define BLEND_DST_ATOP 10
#define BLEND_XOR 11
#define BLEND_PLUS 12
#define BLEND_SCREEN 13
#define BLEND_MODULATE 14
#define BLEND_MULTIPLY 15
#define BLEND_OVERLAY 16
#define BLEND_DARKEN 17
#define BLEND_LIGHTEN 18
#define BLEND_COLOR_DODGE 19
#define BLEND_COLOR_BURN 20
#define BLEND_HARD_LIGHT 21
#define BLEND_SOFT_LIGHT 22
#define BLEND_DIFFERENCE 23
#define BLEND_EXCLUSION 24
#define BLEND_HUE 25
#define BLEND_SATURATION 26
#define BLEND_COLOR 27
#define BLEND_LUMINOSITY 28

float blendMultiplyOrScreen(float b, float s) {
    return s <= 0.5 ? b * 2.0 * s : b + (2.0 * s - 1.0) - b * (2.0 * s - 1.0);
}

float blendColorDodge(float b, float s) {
    if (b <= 0.0) {
        return 0.0;
    }
    return s >= 1.0 ? 1.0 : min(1.0, b / (1.0 - s));
}

float blendColorBurn(float b, float s) {
    if (b >= 1.0) {
        return 1.0;
    }
    return s <= 0.0 ? 0.0 : 1.0 - min(1.0, (1.0 - b) / s);
}

float blendSoftLight(float b, float s) {
    if (s <= 0.5) {
        return b - (1.0 - 2.0 * s) * b * (1.0 - b);
    }
    float d = b <= 0.25 ? ((16.0 * b - 12.0) * b + 4.0) * b : sqrt(b);
    return b + (2.0 * s - 1.0) * (d - b);
}

float blendLuminosity(float3 c) {
    return 0.3 * c.r + 0.59 * c.g + 0.11 * c.b;
}

float3 blendClipColor(float3 c) {
    float l = blendLuminosity(c);
    float n = min(min(c.r, c.g), c.b);
    float x = max(max(c.r, c.g), c.b);
    if (n < 0.0) {
        c = l + (c - l) * l / (l - n);
    }
    if (x > 1.0) {
        c = l + (c - l) * (1.0 - l) / (x - l);
    }
    return c;
}

float3 blendSetLuminosity(float3 c, float l) {
    return blendClipColor(c + (l - blendLuminosity(c)));
}

float blendSaturation(float3 c) {
    return max(max(c.r, c.g), c.b) - min(min(c.r, c.g), c.b);
}

// setSaturation stretches c to saturation s, which moves its largest channel to
// s and its smallest to 0.
float3 blendSetSaturation(float3 c, float s) {
    float n = min(min(c.r, c.g), c.b);
    float x = max(max(c.r, c.g), c.b);
    return x > n ? (c - n) * s / (x - n) : float3(0.0, 0.0, 0.0);
}

// mix is B(Cb, Cs) of the W3C Compositing and Blending Level 1 formulas, on
// colors that are not premultiplied.
float3 blendMix(float3 b, float3 s, uint mode) {
    if (mode == BLEND_MULTIPLY) {
        return b * s;
    }
    if (mode == BLEND_OVERLAY) {
        return float3(blendMultiplyOrScreen(s.r, b.r), blendMultiplyOrScreen(s.g, b.g), blendMultiplyOrScreen(s.b, b.b));
    }
    if (mode == BLEND_DARKEN) {
        return min(b, s);
    }
    if (mode == BLEND_LIGHTEN) {
        return max(b, s);
    }
    if (mode == BLEND_COLOR_DODGE) {
        return float3(blendColorDodge(b.r, s.r), blendColorDodge(b.g, s.g), blendColorDodge(b.b, s.b));
    }
    if (mode == BLEND_COLOR_BURN) {
        return float3(blendColorBurn(b.r, s.r), blendColorBurn(b.g, s.g), blendColorBurn(b.b, s.b));
    }
    if (mode == BLEND_HARD_LIGHT) {
        return float3(blendMultiplyOrScreen(b.r, s.r), blendMultiplyOrScreen(b.g, s.g), blendMultiplyOrScreen(b.b, s.b));
    }
    if (mode == BLEND_SOFT_LIGHT) {
        return float3(blendSoftLight(b.r, s.r), blendSoftLight(b.g, s.g), blendSoftLight(b.b, s.b));
    }
    if (mode == BLEND_DIFFERENCE) {
        return abs(b - s);
    }
    if (mode == BLEND_EXCLUSION) {
        return b + s - 2.0 * b * s;
    }
    if (mode == BLEND_HUE) {
        return blendSetLuminosity(blendSetSaturation(s, blendSaturation(b)), blendLuminosity(b));
    }
    if (mode == BLEND_SATURATION) {
        return blendSetLuminosity(blendSetSaturation(b, blendSaturation(s)), blendLuminosity(b));
    }
    if (mode == BLEND_COLOR) {
        return blendSetLuminosity(s, blendLuminosity(b));
    }
    return blendSetLuminosity(b, blendLuminosity(s)); // BLEND_LUMINOSITY
}

// readBackdrop composes c, which has coverage applied, with the backdrop under
// the fragment. The Porter-Duff modes here replace the target, so the result
// fades to the backdrop by coverage itself. The others are the W3C formulas:
// the mixed color takes the place of the source where the backdrop is opaque,
// and the result goes source-over onto the target.
float4 blendReadBackdrop(float4 c, float coverage, uint mode, float4 position) {
    float4 d = BLEND_BACKDROP.Load(int3(int2(floor(position.xy)), 0));
    float4 rest = d * (1.0 - coverage);
    if (mode == BLEND_SRC) {
        return c + rest;
    }
    if (mode == BLEND_SRC_IN) {
        return c * d.a + rest;
    }
    if (mode == BLEND_SRC_OUT) {
        return c * (1.0 - d.a) + rest;
    }
    if (mode == BLEND_DST_ATOP) {
        return c * (1.0 - d.a) + d * c.a + rest;
    }
    if (c.a <= 0.0) {
        return float4(0.0, 0.0, 0.0, 0.0);
    }
    float3 s = saturate(c.rgb / c.a);
    float3 b = d.a > 0.0 ? saturate(d.rgb / d.a) : float3(0.0, 0.0, 0.0);
    float3 mixed = (1.0 - d.a) * s + d.a * saturate(blendMix(b, s, mode));
    return float4(mixed * c.a, c.a);
}

// blendOutput shapes what the fixed-function blend of mode takes in. See
// gpu.BlendMode.Factors.
float4 blendOutput(float4 c, float coverage, uint mode, float4 position) {
    if (mode == BLEND_CLEAR) {
        // ZERO, INV_SRC_ALPHA: the target fades by coverage.
        return float4(0.0, 0.0, 0.0, coverage);
    }
    if (mode == BLEND_DST_IN || mode == BLEND_MODULATE) {
        // ZERO, INV_SRC_ALPHA or INV_SRC_COLOR: the target keeps
        // 1 - coverage + c, the alpha or color of the source where it covers
        // and 1 outside.
        return coverage - c;
    }
    if (mode == BLEND_SRC || mode == BLEND_SRC_IN || mode == BLEND_SRC_OUT || mode == BLEND_DST_ATOP ||
        mode >= BLEND_MULTIPLY) {
        return blendReadBackdrop(c, coverage, mode, position);
    }
    return c;
}
