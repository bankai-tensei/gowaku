// The SDF pipeline for Metal: a line-by-line port of gpu/shaders/sdf.vs.hlsl,
// path.vs.hlsl, sdf.ps.hlsl and blend.hlsli. Keep the four in step: a change
// there needs the same change here. Metal compiles this source when the device
// is created, so that building gowaku needs no Xcode (ADR 0082).
//
// HLSL reads cbuffer globals and textures from anywhere; MSL passes them in,
// so the functions here take the frame and the textures they read as
// arguments. Otherwise the names and the order follow the HLSL.

#include <metal_stdlib>
using namespace metal;

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

#define BLEND_SHIFT 8u
#define KIND_MASK 0xFFu

#define GRADIENT_NONE 0u
#define GRADIENT_LINEAR 1u
#define GRADIENT_RADIAL 2u
#define GRADIENT_SWEEP 3u
#define GRADIENT_FOCAL 4u
#define GRADIENT_SHAPE 15u
#define GRADIENT_RAMP 16u
#define GRADIENT_REFLECT 32u
#define GRADIENT_REPEAT 64u

#define FILTER_LINEAR 0u
#define FILTER_NEAREST 1u
#define FILTER_MIPMAP 2u

// Frame mirrors frameConstants in device.go and cbuffer Frame in sdf.hlsli.
struct Frame {
    float2 viewport; // physical pixels
    float scale;     // physical pixels per logical pixel
    float padding;
};

// Instance mirrors gpu.Instance byte for byte: 200 bytes, no padding. The
// packed types keep the alignment of the members at 4 bytes, as in Go.
struct Instance {
    packed_float4 rect;
    packed_float4 radii;
    packed_float4 color;
    packed_float4 endColor;
    packed_float4 borderColor;
    packed_float4 gradient;
    packed_float2 params;
    packed_uint2 mode;
    packed_float4 midColor0;
    packed_float4 midColor1;
    packed_float4 stops;
    packed_float4 transform;
    packed_float2 translation;
    packed_float4 gradientMatrix;
};

// Varyings carries the locations that spirv-cross gives the inputs of a custom
// shader, the order of the HLSL struct, so that the fragment function of
// gowaku-shader links to sdfVertex by them (ADR 0082).
struct Varyings {
    float4 position [[position]];
    float2 logical [[user(locn0)]];
    float4 rect [[user(locn1), flat]];
    float4 radii [[user(locn2), flat]];
    float4 color [[user(locn3), flat]];
    float4 endColor [[user(locn4), flat]];
    float4 borderColor [[user(locn5), flat]];
    float4 gradient [[user(locn6), flat]];
    float2 params [[user(locn7), flat]];
    uint2 mode [[user(locn8)]];
    float4 midColor0 [[user(locn9), flat]];
    float4 midColor1 [[user(locn10), flat]];
    float4 stops [[user(locn11), flat]];
    float4 transform [[user(locn12), flat]];
    float2 translation [[user(locn13), flat]];
    float4 gradientMatrix [[user(locn14), flat]];
};

bool turned(float4 transform) {
    return any(transform != 0.0);
}

// ---- sdf.vs.hlsl ----

float minScale(float4 t) {
    float sum = dot(t, t);
    float det = t.x * t.w - t.y * t.z;
    float largest = sqrt((sum + sqrt(max(sum * sum - 4.0 * det * det, 0.0))) * 0.5);
    return abs(det) / max(largest, 1e-12);
}

// The instance buffer is bound at the first instance of the draw call, so the
// instance ID counts from zero.
vertex Varyings sdfVertex(uint vertexID [[vertex_id]], uint instanceID [[instance_id]],
                          constant Frame& frame [[buffer(0)]],
                          const device Instance* instances [[buffer(1)]]) {
    Instance instance = instances[instanceID];
    float scale = frame.scale;
    float4 transform = float4(instance.transform);
    bool isTurned = turned(transform);
    uint kind = instance.mode.x & KIND_MASK;
    float margin = 1.0 / scale;
    if (isTurned) {
        margin /= max(minScale(transform), 1e-6);
    }
    if (kind == KIND_SHADOW) {
        margin += 1.5 * instance.params.y;
    }
    if (kind >= KIND_GLYPH && kind != KIND_BACKDROP) {
        if (!isTurned || kind == KIND_COVER) {
            margin = 0.0;
        } else if (kind == KIND_GLYPH) {
            margin = 1.0 / scale;
        }
    }
    float4 rect = float4(instance.rect);
    float2 corner = float2(vertexID & 1u, vertexID >> 1u);
    float2 logical = rect.xy - margin + corner * (rect.zw + 2.0 * margin);
    float2 surface = logical;
    if (isTurned) {
        float4 t = transform;
        surface = float2(t.x * logical.x + t.z * logical.y, t.y * logical.x + t.w * logical.y) + float2(instance.translation);
    }
    float2 ndc = surface * scale / frame.viewport * 2.0 - 1.0;

    Varyings v;
    v.position = float4(ndc.x, -ndc.y, 0.0, 1.0);
    v.logical = logical;
    v.rect = rect;
    v.radii = float4(instance.radii);
    v.color = float4(instance.color);
    v.endColor = float4(instance.endColor);
    v.borderColor = float4(instance.borderColor);
    v.gradient = float4(instance.gradient);
    v.params = float2(instance.params);
    v.mode = uint2(instance.mode);
    v.midColor0 = float4(instance.midColor0);
    v.midColor1 = float4(instance.midColor1);
    v.stops = float4(instance.stops);
    v.transform = transform;
    v.translation = float2(instance.translation);
    v.gradientMatrix = float4(instance.gradientMatrix);
    return v;
}

// ---- path.vs.hlsl ----

struct PathVaryings {
    float4 position [[position]];
};

vertex PathVaryings pathVertex(uint vertexID [[vertex_id]], constant Frame& frame [[buffer(0)]],
                               const device packed_float2* vertices [[buffer(2)]]) {
    float2 logical = float2(vertices[vertexID]);
    float2 ndc = logical * frame.scale / frame.viewport * 2.0 - 1.0;
    PathVaryings v;
    v.position = float4(ndc.x, -ndc.y, 0.0, 1.0);
    return v;
}

// ---- sdf.ps.hlsl ----

float4 premultiply(float4 c) {
    return float4(c.rgb * c.a, c.a);
}

float roundedBoxDistance(float2 p, float2 halfSize, float4 radii) {
    float2 side = p.x > 0.0 ? radii.yz : radii.xw;
    float radius = p.y > 0.0 ? side.y : side.x;
    radius = min(radius, min(halfSize.x, halfSize.y));
    float2 q = abs(p) - halfSize + radius;
    return min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - radius;
}

float2 roundedBoxNormal(float2 p, float2 halfSize, float4 radii) {
    float2 side = p.x > 0.0 ? radii.yz : radii.xw;
    float radius = p.y > 0.0 ? side.y : side.x;
    radius = min(radius, min(halfSize.x, halfSize.y));
    float2 q = abs(p) - halfSize + radius;
    float2 n = q.x > q.y ? float2(1.0, 0.0) : float2(0.0, 1.0);
    if (q.x > 0.0 && q.y > 0.0) {
        n = normalize(q);
    }
    return float2(p.x < 0.0 ? -n.x : n.x, p.y < 0.0 ? -n.y : n.y);
}

float edgeStep(float2 n, float4 t, float scale) {
    float det = t.x * t.w - t.y * t.z;
    float2 perPixel = float2(dot(n, float2(t.w, -t.y)), dot(n, float2(-t.z, t.x)));
    return max(length(perPixel) / (abs(det) * scale), 1e-12);
}

float stopWeight(float t, float from, float to) {
    return saturate((t - from) / max(to - from, 1e-6));
}

float4 ramp(Varyings v, float t) {
    float4 c = premultiply(v.color);
    c = mix(c, premultiply(v.midColor0), stopWeight(t, v.stops.x, v.stops.y));
    c = mix(c, premultiply(v.midColor1), stopWeight(t, v.stops.y, v.stops.z));
    return mix(c, premultiply(v.endColor), stopWeight(t, v.stops.z, v.stops.w));
}

float4 rampTexels(texture2d<float> ramps, float t, int row) {
    float x = saturate(t) * 255.0;
    float i = min(floor(x), 254.0);
    float4 low = ramps.read(uint2(int(i), row), 0);
    float4 high = ramps.read(uint2(int(i) + 1, row), 0);
    return mix(low, high, x - i);
}

float sweep(float2 p, float2 axis) {
    float angle = atan2(axis.x * p.y - axis.y * p.x, dot(p, axis));
    float t = angle * 0.15915494;
    return t < 0.0 ? t + 1.0 : t;
}

float focal(float2 p, float2 axis, float2 f) {
    float2 q = p - f;
    float2 d = -f;
    float a = dot(d, d) - dot(axis, axis);
    float b = dot(q, d);
    float c = dot(q, q);
    return (b - sqrt(max(b * b - a * c, 0.0))) / min(a, -1e-6);
}

float spread(float t, uint mode) {
    if ((mode & GRADIENT_REPEAT) != 0u) {
        return t - floor(t);
    }
    if ((mode & GRADIENT_REFLECT) != 0u) {
        float u = t * 0.5;
        u -= floor(u);
        return 1.0 - abs(u * 2.0 - 1.0);
    }
    return t;
}

float4 fill(Varyings v, texture2d<float> ramps) {
    uint shape = v.mode.y & GRADIENT_SHAPE;
    if (shape == GRADIENT_NONE) {
        return premultiply(v.color);
    }
    float2 start = v.gradient.xy;
    float2 axis = v.gradient.zw - start;
    float2 p = v.logical - start;
    if (any(v.gradientMatrix != 0.0)) {
        float4 m = v.gradientMatrix;
        p = float2(m.x * p.x + m.z * p.y, m.y * p.x + m.w * p.y);
    }
    float lengthSquared = max(dot(axis, axis), 1e-6);
    float t;
    if (shape == GRADIENT_SWEEP) {
        t = sweep(p, axis);
    } else {
        t = shape == GRADIENT_LINEAR
            ? dot(p, axis) / lengthSquared
            : shape == GRADIENT_FOCAL
            ? focal(p, axis, v.stops.yz)
            : sqrt(dot(p, p) / lengthSquared);
    }
    t = spread(t, v.mode.y);
    if ((v.mode.y & GRADIENT_RAMP) != 0u) {
        return rampTexels(ramps, t, int(v.stops.x));
    }
    return ramp(v, t);
}

float4 rrect(Varyings v, texture2d<float> ramps, float scale, thread float& coverage) {
    float2 halfSize = v.rect.zw * 0.5;
    float2 p = v.logical - v.rect.xy - halfSize;
    float distance = roundedBoxDistance(p, halfSize, v.radii);
    float border = v.params.x;
    float outer = saturate(0.5 - distance * scale);
    float inner = saturate(0.5 - (distance + border) * scale);
    if (turned(v.transform)) {
        float step = edgeStep(roundedBoxNormal(p, halfSize, v.radii), v.transform, scale);
        outer = saturate(0.5 - distance / step);
        inner = saturate(0.5 - (distance + border) / step);
    }
    coverage = outer;
    return fill(v, ramps) * inner + premultiply(v.borderColor) * (outer - inner);
}

float gaussian(float x, float sigma) {
    return exp(-(x * x) / (2.0 * sigma * sigma)) / (2.5066282746 * sigma);
}

// erf of the HLSL. Renamed so that it cannot meet a library function.
float2 erf2(float2 x) {
    float2 s = sign(x);
    float2 a = abs(x);
    float2 d = 1.0 + (0.278393 + (0.230389 + 0.078108 * (a * a)) * a) * a;
    d *= d;
    return s - s / (d * d);
}

float shadowSlice(float x, float y, float sigma, float radius, float2 halfSize) {
    float delta = min(halfSize.y - radius - abs(y), 0.0);
    float curved = halfSize.x - radius + sqrt(max(0.0, radius * radius - delta * delta));
    float2 integral = 0.5 + 0.5 * erf2((x + float2(-curved, curved)) * (0.70710678 / sigma));
    return integral.y - integral.x;
}

float shadowCoverage(float2 p, float2 halfSize, float radius, float sigma) {
    float low = p.y - halfSize.y;
    float high = p.y + halfSize.y;
    float start = clamp(-3.0 * sigma, low, high);
    float end = clamp(3.0 * sigma, low, high);
    float step = (end - start) / 6.0;
    float y = start + step * 0.5;
    float value = 0.0;
    for (int i = 0; i < 6; i++) {
        value += shadowSlice(p.x, p.y - y, sigma, radius, halfSize) * gaussian(y, sigma) * step;
        y += step;
    }
    return value;
}

float4 shadow(Varyings v, thread float& coverage) {
    float2 halfSize = v.rect.zw * 0.5;
    float2 p = v.logical - v.rect.xy - halfSize;
    float radius = max(max(v.radii.x, v.radii.y), max(v.radii.z, v.radii.w));
    radius = min(radius, min(halfSize.x, halfSize.y));
    float sigma = v.params.y * 0.5;
    coverage = shadowCoverage(p, halfSize, radius, sigma);
    return premultiply(v.color) * coverage;
}

float glyphTexel(texture2d<float> atlas, int2 t, int2 first, int2 size) {
    int2 inside = t - first;
    if (any(inside < 0) || any(inside >= size)) {
        return 0.0;
    }
    return atlas.read(uint2(t), 0).r;
}

float turnedGlyph(Varyings v, texture2d<float> atlas, float scale) {
    int2 first = int2(v.radii.xy);
    int2 size = int2(round(v.rect.zw * scale));
    float2 at = v.radii.xy + (v.logical - v.rect.xy) * scale - 0.5;
    float2 base = floor(at);
    float2 f = at - base;
    int2 t = int2(base);
    float top = mix(glyphTexel(atlas, t, first, size), glyphTexel(atlas, t + int2(1, 0), first, size), f.x);
    float bottom = mix(glyphTexel(atlas, t + int2(0, 1), first, size), glyphTexel(atlas, t + int2(1, 1), first, size), f.x);
    return mix(top, bottom, f.y);
}

float4 glyph(Varyings v, texture2d<float> atlas, float scale, thread float& coverage) {
    if (turned(v.transform)) {
        coverage = turnedGlyph(v, atlas, scale);
    } else {
        int2 origin = int2(round(v.rect.xy * scale));
        int2 texel = int2(v.radii.xy) + int2(floor(v.position.xy)) - origin;
        coverage = atlas.read(uint2(texel), 0).r;
    }
    return premultiply(v.color) * coverage;
}

float2 exactLogical(Varyings v, float scale) {
    float4 t = v.transform;
    float2 s = v.position.xy / scale - v.translation;
    return float2(t.w * s.x - t.z * s.y, t.x * s.y - t.y * s.x) / (t.x * t.w - t.y * t.z);
}

#define NEAREST_BIAS (1.0 / 256.0)

float4 nearestTexel(texture2d<float> picture, float2 uv) {
    int2 size = int2(picture.get_width(0), picture.get_height(0));
    int2 t = int2(floor(uv * float2(size) + NEAREST_BIAS));
    return picture.read(uint2(clamp(t, int2(0, 0), size - 1)), 0);
}

float4 levelTexel(texture2d<float> picture, int2 t, int2 size, uint level) {
    return picture.read(uint2(clamp(t, int2(0, 0), size - 1)), level);
}

float4 bilinearLevel(texture2d<float> picture, float2 uv, uint level) {
    int2 size = int2(picture.get_width(level), picture.get_height(level));
    float2 at = uv * float2(size) - 0.5;
    float2 base = floor(at);
    float2 f = at - base;
    int2 t = int2(base);
    float4 top = mix(levelTexel(picture, t, size, level), levelTexel(picture, t + int2(1, 0), size, level), f.x);
    float4 bottom = mix(levelTexel(picture, t + int2(0, 1), size, level), levelTexel(picture, t + int2(1, 1), size, level), f.x);
    return mix(top, bottom, f.y);
}

#define MIN_LOD (1.0 / 256.0)

float levelOfDetail(Varyings v, float2 size, float scale) {
    float2 texels = (v.gradient.zw - v.gradient.xy) * size / v.rect.zw;
    float stretch;
    if (turned(v.transform)) {
        float4 t = v.transform;
        float det = max(abs(t.x * t.w - t.y * t.z), 1e-12);
        float4 m = float4(texels.x * t.w, -texels.x * t.z, -texels.y * t.y, texels.y * t.x) / det;
        float sum = dot(m, m);
        float mdet = m.x * m.w - m.y * m.z;
        stretch = sqrt((sum + sqrt(max(sum * sum - 4.0 * mdet * mdet, 0.0))) * 0.5);
    } else {
        stretch = max(abs(texels.x), abs(texels.y));
    }
    return log2(stretch / scale);
}

constexpr sampler pictureSampler(coord::normalized, address::clamp_to_edge, filter::linear, mip_filter::none);

float4 mipmapped(Varyings v, texture2d<float> picture, float2 uv, float scale) {
    uint levels = picture.get_num_mip_levels();
    float lod = levelOfDetail(v, float2(picture.get_width(0), picture.get_height(0)), scale);
    float4 c;
    if (lod > MIN_LOD) {
        lod = min(lod, float(levels - 1));
        float low = floor(lod);
        float f = lod - low;
        c = bilinearLevel(picture, uv, uint(low));
        if (f > 0.0) {
            c = mix(c, bilinearLevel(picture, uv, uint(low) + 1), f);
        }
    } else {
        c = picture.sample(pictureSampler, uv, level(0.0));
    }
    return c;
}

float4 image(Varyings v, texture2d<float> picture, float scale, thread float& coverage) {
    float2 t = (v.logical - v.rect.xy) / v.rect.zw;
    coverage = 1.0;
    if (turned(v.transform)) {
        t = saturate((exactLogical(v, scale) - v.rect.xy) / v.rect.zw);
        float2 halfSize = v.rect.zw * 0.5;
        float2 p = v.logical - v.rect.xy - halfSize;
        float4 square = float4(0.0, 0.0, 0.0, 0.0);
        float distance = roundedBoxDistance(p, halfSize, square);
        coverage = saturate(0.5 - distance / edgeStep(roundedBoxNormal(p, halfSize, square), v.transform, scale));
    }
    float2 uv = mix(v.gradient.xy, v.gradient.zw, t);
    float4 c;
    if (v.mode.y == FILTER_NEAREST) {
        c = nearestTexel(picture, uv);
    } else if (v.mode.y == FILTER_MIPMAP) {
        c = mipmapped(v, picture, uv, scale);
    } else {
        c = picture.sample(pictureSampler, uv, level(0.0));
    }
    return c * (v.color.a * coverage);
}

float blurFactor(float sigma) {
    return sigma < 8.0 ? 2.0 : (sigma < 32.0 ? 4.0 : 8.0);
}

int2 clampTexel(int2 t, float4 r) {
    return clamp(t, int2(r.xy), int2(r.zw));
}

float4 blurDown(Varyings v, texture2d<float> picture, texture2d<float> backdrop) {
    int f = int(v.params.x);
    int2 base = int2(floor(v.position.xy)) * f;
    float4 sum = float4(0.0, 0.0, 0.0, 0.0);
    for (int y = 0; y < f; y++) {
        for (int x = 0; x < f; x++) {
            uint2 t = uint2(clampTexel(base + int2(x, y), v.radii));
            if (v.params.y > 0.0) {
                sum += picture.read(t, 0);
            } else {
                sum += backdrop.read(t, 0);
            }
        }
    }
    return sum / float(f * f);
}

float4 blurLine(Varyings v, texture2d<float> filtered, int2 direction) {
    float sigma = v.params.x;
    int radius = min(int(ceil(3.0 * sigma)), 48);
    float falloff = -0.5 / (sigma * sigma);
    int2 center = int2(floor(v.position.xy));
    float4 sum = float4(0.0, 0.0, 0.0, 0.0);
    float total = 0.0;
    for (int k = -radius; k <= radius; k++) {
        float w = exp(float(k * k) * falloff);
        sum += filtered.read(uint2(clampTexel(center + direction * k, v.radii)), 0) * w;
        total += w;
    }
    return sum / total;
}

float4 blurredTexels(texture2d<float> filtered, float2 p) {
    float2 base = floor(p);
    float2 t = p - base;
    int2 low = max(int2(base), 0);
    int2 high = max(int2(base) + 1, 0);
    float4 top = mix(filtered.read(uint2(low.x, low.y), 0), filtered.read(uint2(high.x, low.y), 0), t.x);
    float4 bottom = mix(filtered.read(uint2(low.x, high.y), 0), filtered.read(uint2(high.x, high.y), 0), t.x);
    return mix(top, bottom, t.y);
}

float4 backdropBlur(Varyings v, texture2d<float> ramps, texture2d<float> filtered, float scale, thread float& coverage) {
    rrect(v, ramps, scale, coverage);
    float f = blurFactor(v.params.y * scale);
    return blurredTexels(filtered, v.position.xy / f - 0.5) * (v.color.a * coverage);
}

float4 filteredImage(Varyings v, texture2d<float> picture, texture2d<float> filtered, float scale, thread float& coverage) {
    float2 t = (v.logical - v.rect.xy) / v.rect.zw;
    coverage = 1.0;
    if (turned(v.transform)) {
        t = saturate(t);
        float2 halfSize = v.rect.zw * 0.5;
        float2 p = v.logical - v.rect.xy - halfSize;
        float4 square = float4(0.0, 0.0, 0.0, 0.0);
        float distance = roundedBoxDistance(p, halfSize, square);
        coverage = saturate(0.5 - distance / edgeStep(roundedBoxNormal(p, halfSize, square), v.transform, scale));
    }
    float2 at = mix(v.gradient.xy, v.gradient.zw, t) * float2(picture.get_width(), picture.get_height());
    float f = blurFactor(v.params.y * scale);
    return blurredTexels(filtered, at / f - 0.5) * (v.color.a * coverage);
}

// ---- blend.hlsli ----

#define BLEND_SRC_OVER 0u
#define BLEND_CLEAR 1u
#define BLEND_SRC 2u
#define BLEND_DST 3u
#define BLEND_DST_OVER 4u
#define BLEND_SRC_IN 5u
#define BLEND_DST_IN 6u
#define BLEND_SRC_OUT 7u
#define BLEND_DST_OUT 8u
#define BLEND_SRC_ATOP 9u
#define BLEND_DST_ATOP 10u
#define BLEND_XOR 11u
#define BLEND_PLUS 12u
#define BLEND_SCREEN 13u
#define BLEND_MODULATE 14u
#define BLEND_MULTIPLY 15u
#define BLEND_OVERLAY 16u
#define BLEND_DARKEN 17u
#define BLEND_LIGHTEN 18u
#define BLEND_COLOR_DODGE 19u
#define BLEND_COLOR_BURN 20u
#define BLEND_HARD_LIGHT 21u
#define BLEND_SOFT_LIGHT 22u
#define BLEND_DIFFERENCE 23u
#define BLEND_EXCLUSION 24u
#define BLEND_HUE 25u
#define BLEND_SATURATION 26u
#define BLEND_COLOR 27u
#define BLEND_LUMINOSITY 28u

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

float3 blendSetSaturation(float3 c, float s) {
    float n = min(min(c.r, c.g), c.b);
    float x = max(max(c.r, c.g), c.b);
    return x > n ? (c - n) * s / (x - n) : float3(0.0, 0.0, 0.0);
}

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
    return blendSetLuminosity(b, blendLuminosity(s));
}

float4 blendReadBackdrop(texture2d<float> backdrop, float4 c, float coverage, uint mode, float4 position) {
    float4 d = backdrop.read(uint2(int2(floor(position.xy))), 0);
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

float4 blendOutput(texture2d<float> backdrop, float4 c, float coverage, uint mode, float4 position) {
    if (mode == BLEND_CLEAR) {
        return float4(0.0, 0.0, 0.0, coverage);
    }
    if (mode == BLEND_DST_IN || mode == BLEND_MODULATE) {
        return coverage - c;
    }
    if (mode == BLEND_SRC || mode == BLEND_SRC_IN || mode == BLEND_SRC_OUT || mode == BLEND_DST_ATOP ||
        mode >= BLEND_MULTIPLY) {
        return blendReadBackdrop(backdrop, c, coverage, mode, position);
    }
    return c;
}

// ---- main of sdf.ps.hlsl ----

fragment float4 sdfFragment(Varyings v [[stage_in]],
                            constant Frame& frame [[buffer(0)]],
                            texture2d<float> atlas [[texture(0)]],
                            texture2d<float> picture [[texture(1)]],
                            texture2d<float> ramps [[texture(2)]],
                            texture2d<float> backdrop [[texture(3)]],
                            texture2d<float> filtered [[texture(4)]]) {
    float scale = frame.scale;
    uint kind = v.mode.x & KIND_MASK;
    float coverage = 1.0;
    float4 c;
    if (kind == KIND_SHADOW) {
        c = shadow(v, coverage);
    } else if (kind == KIND_GLYPH) {
        c = glyph(v, atlas, scale, coverage);
    } else if (kind == KIND_COVER) {
        c = fill(v, ramps);
    } else if (kind == KIND_IMAGE) {
        c = image(v, picture, scale, coverage);
    } else if (kind == KIND_BACKDROP) {
        c = backdropBlur(v, ramps, filtered, scale, coverage);
    } else if (kind == KIND_FILTERED) {
        c = filteredImage(v, picture, filtered, scale, coverage);
    } else if (kind == KIND_BLUR_DOWN) {
        c = blurDown(v, picture, backdrop);
    } else if (kind == KIND_BLUR_X) {
        c = blurLine(v, filtered, int2(1, 0));
    } else if (kind == KIND_BLUR_Y) {
        c = blurLine(v, filtered, int2(0, 1));
    } else {
        c = rrect(v, ramps, scale, coverage);
    }
    uint mode = v.mode.x >> BLEND_SHIFT;
    if (mode == BLEND_SRC_OVER) {
        return c;
    }
    return blendOutput(backdrop, c, coverage, mode, v.position);
}
