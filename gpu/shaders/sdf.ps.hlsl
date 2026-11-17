#include "sdf.hlsli"

// The glyph atlas. Glyphs map one texel to one physical pixel, so it is read
// with Load. A turned glyph filters its own texels, bilinearly, in glyph().
Texture2D<float> atlas : register(t0);

// The image of the draw call. Its colors are premultiplied. FilterLinear reads
// it through the sampler, at level 0; FilterNearest and FilterMipmap read it
// with Load, so that every backend weighs the texels alike.
Texture2D<float4> picture : register(t1);
SamplerState pictureSampler : register(s0);

// The gradients of more than four stops, one per row. Their colors are
// premultiplied.
Texture2D<float4> ramps : register(t2);

// The backdrop: a copy of the target under a draw call whose blend mode reads
// it, at the same pixel coordinates. Its colors are premultiplied.
Texture2D<float4> backdrop : register(t3);

// The scratch target that a blur pass reads, or the blurred picture that
// InstanceBackdrop and InstanceFiltered draw: the source shrunk by the factor
// of the blur, a texel of it under factor by factor pixels. Its colors are
// premultiplied. See gpu.Blur.
Texture2D<float4> filtered : register(t4);

// Colors arrive non-premultiplied and leave premultiplied: mixing a fill and a
// border by coverage only stays correct on premultiplied values.
float4 premultiply(float4 c) {
    return float4(c.rgb * c.a, c.a);
}

// Signed distance to a rounded box centered on the origin, negative inside.
// After Inigo Quilez, "2D distance functions".
float roundedBoxDistance(float2 p, float2 halfSize, float4 radii) {
    float2 side = p.x > 0.0 ? radii.yz : radii.xw; // top, bottom
    float radius = p.y > 0.0 ? side.y : side.x;
    radius = min(radius, min(halfSize.x, halfSize.y));
    float2 q = abs(p) - halfSize + radius;
    return min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - radius;
}

// The outward normal of the rounded box at p, the direction in which
// roundedBoxDistance grows.
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

// How much a distance along n, in the coordinates of the instance, changes over
// one physical pixel of the surface: the length of its screen gradient. It is
// what fwidth would measure, worked out from the transform so that every
// backend gets the same value. Upright, it is 1 / scale.
float edgeStep(float2 n, float4 t) {
    float det = t.x * t.w - t.y * t.z;
    float2 perPixel = float2(dot(n, float2(t.w, -t.y)), dot(n, float2(-t.z, t.x)));
    return max(length(perPixel) / (abs(det) * scale), 1e-12);
}

// How far t has come from one gradient stop to the next.
float stopWeight(float t, float from, float to) {
    return saturate((t - from) / max(to - from, 1e-6));
}

// Blends the four stops in turn. Each lerp holds until t reaches the stop
// before it, so the stops of one segment decide its color. A gradient of fewer
// stops repeats its first one, which makes the extra lerps no-ops.
float4 ramp(Varyings v, float t) {
    float4 c = premultiply(v.color);
    c = lerp(c, premultiply(v.midColor0), stopWeight(t, v.stops.x, v.stops.y));
    c = lerp(c, premultiply(v.midColor1), stopWeight(t, v.stops.y, v.stops.z));
    return lerp(c, premultiply(v.endColor), stopWeight(t, v.stops.z, v.stops.w));
}

// Reads t from row of the ramps. Texel i holds the color at i / 255, and the
// two texels around t blend linearly, past the ends of the row its end colors
// continue. The texels are read with Load, so that every backend weighs them
// alike.
float4 rampTexels(float t, int row) {
    float x = saturate(t) * 255.0;
    float i = min(floor(x), 254.0);
    float4 low = ramps.Load(int3(int(i), row, 0));
    float4 high = ramps.Load(int3(int(i) + 1, row, 0));
    return lerp(low, high, x - i);
}

// The turn of p clockwise from axis, from 0 to 1.
float sweep(float2 p, float2 axis) {
    float angle = atan2(axis.x * p.y - axis.y * p.x, dot(p, axis));
    float t = angle * 0.15915494;
    return t < 0.0 ? t + 1.0 : t;
}

// The offset of a focal gradient at p: the circle through axis around 0 holds
// offset 1, and the focal point f offset 0. As gpu.FocalOffset.
float focal(float2 p, float2 axis, float2 f) {
    float2 q = p - f;
    float2 d = -f;
    float a = dot(d, d) - dot(axis, axis);
    float b = dot(q, d);
    float c = dot(q, q);
    return (b - sqrt(max(b * b - a * c, 0.0))) / min(a, -1e-6);
}

// Maps t into 0 to 1 by the spread of mode, as gpu.SpreadOffset.
float spread(float t, uint mode) {
    if ((mode & GRADIENT_REPEAT) != 0) {
        return t - floor(t);
    }
    if ((mode & GRADIENT_REFLECT) != 0) {
        float u = t * 0.5;
        u -= floor(u);
        return 1.0 - abs(u * 2.0 - 1.0);
    }
    return t;
}

float4 fill(Varyings v) {
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
    if ((v.mode.y & GRADIENT_RAMP) != 0) {
        return rampTexels(t, int(v.stops.x));
    }
    return ramp(v, t);
}

// Each shape returns its premultiplied color with the coverage of its edge
// applied, and the coverage on its own, which the blend modes need apart.

float4 rrect(Varyings v, out float coverage) {
    float2 halfSize = v.rect.zw * 0.5;
    float2 p = v.logical - v.rect.xy - halfSize;
    float distance = roundedBoxDistance(p, halfSize, v.radii);
    float border = v.params.x;
    // Distances are in logical pixels and the edge is one physical pixel wide.
    float outer = saturate(0.5 - distance * scale);
    float inner = saturate(0.5 - (distance + border) * scale);
    if (turned(v.transform)) {
        float step = edgeStep(roundedBoxNormal(p, halfSize, v.radii), v.transform);
        outer = saturate(0.5 - distance / step);
        inner = saturate(0.5 - (distance + border) / step);
    }
    coverage = outer;
    return fill(v) * inner + premultiply(v.borderColor) * (outer - inner);
}

float gaussian(float x, float sigma) {
    return exp(-(x * x) / (2.0 * sigma * sigma)) / (2.5066282746 * sigma);
}

// Abramowitz and Stegun 7.1.27.
float2 erf(float2 x) {
    float2 s = sign(x);
    float2 a = abs(x);
    float2 d = 1.0 + (0.278393 + (0.230389 + 0.078108 * (a * a)) * a) * a;
    d *= d;
    return s - s / (d * d);
}

// The blur of one horizontal slice of the rounded box, at height y.
float shadowSlice(float x, float y, float sigma, float radius, float2 halfSize) {
    float delta = min(halfSize.y - radius - abs(y), 0.0);
    float curved = halfSize.x - radius + sqrt(max(0.0, radius * radius - delta * delta));
    float2 integral = 0.5 + 0.5 * erf((x + float2(-curved, curved)) * (0.70710678 / sigma));
    return integral.y - integral.x;
}

// The Gaussian blur of a rounded box. A blur is separable, so one axis has an
// exact solution and only the other needs sampling. After Evan Wallace, "Fast
// Rounded Rectangle Shadows".
float shadowCoverage(float2 p, float2 halfSize, float radius, float sigma) {
    float low = p.y - halfSize.y;
    float high = p.y + halfSize.y;
    float start = clamp(-3.0 * sigma, low, high);
    float end = clamp(3.0 * sigma, low, high);
    float step = (end - start) / 6.0;
    float y = start + step * 0.5;
    float value = 0.0;
    [unroll]
    for (int i = 0; i < 6; i++) {
        value += shadowSlice(p.x, p.y - y, sigma, radius, halfSize) * gaussian(y, sigma) * step;
        y += step;
    }
    return value;
}

float4 shadow(Varyings v, out float coverage) {
    float2 halfSize = v.rect.zw * 0.5;
    float2 p = v.logical - v.rect.xy - halfSize;
    float radius = max(max(v.radii.x, v.radii.y), max(v.radii.z, v.radii.w));
    radius = min(radius, min(halfSize.x, halfSize.y));
    float sigma = v.params.y * 0.5;
    coverage = shadowCoverage(p, halfSize, radius, sigma);
    return premultiply(v.color) * coverage;
}

// glyphTexel reads the atlas at t, and nothing outside the glyph, whose neighbors on
// the page are other glyphs.
float glyphTexel(int2 t, int2 first, int2 size) {
    int2 inside = t - first;
    if (any(inside < 0) || any(inside >= size)) {
        return 0.0;
    }
    return atlas.Load(int3(t, 0));
}

// turnedGlyph filters the four texels around the fragment. The atlas has no
// padding, so it cannot go through a sampler.
float turnedGlyph(Varyings v) {
    int2 first = int2(v.radii.xy);
    int2 size = int2(round(v.rect.zw * scale));
    float2 at = v.radii.xy + (v.logical - v.rect.xy) * scale - 0.5;
    float2 base = floor(at);
    float2 f = at - base;
    int2 t = int2(base);
    float top = lerp(glyphTexel(t, first, size), glyphTexel(t + int2(1, 0), first, size), f.x);
    float bottom = lerp(glyphTexel(t + int2(0, 1), first, size), glyphTexel(t + int2(1, 1), first, size), f.x);
    return lerp(top, bottom, f.y);
}

// Radii carries the first texel of the glyph. Rect sits on physical pixels, so
// the pixel's distance from it is the distance into the image.
float4 glyph(Varyings v, out float coverage) {
    if (turned(v.transform)) {
        coverage = turnedGlyph(v);
    } else {
        int2 origin = int2(round(v.rect.xy * scale));
        int2 texel = int2(v.radii.xy) + int2(floor(v.position.xy)) - origin;
        coverage = atlas.Load(int3(texel, 0));
    }
    return premultiply(v.color) * coverage;
}

// exactLogical is the point of Rect under the center of the pixel: the inverse
// transform of the center. The interpolated varying is affine in the corners
// that the rasterizer snapped to its subpixel grid, which puts it off by up to
// a hundredth of a texel on a shrunk image: enough to change the texel that
// FilterNearest picks, and for backends to disagree.
float2 exactLogical(Varyings v) {
    float4 t = v.transform;
    float2 s = v.position.xy / scale - v.translation;
    return float2(t.w * s.x - t.z * s.y, t.x * s.y - t.y * s.x) / (t.x * t.w - t.y * t.z);
}

// NEAREST_BIAS moves a position that lands on the edge between two texels, as
// every pixel does when an image shrinks to a half or a quarter, clearly onto
// the texel after it, so that rounding cannot make backends disagree.
#define NEAREST_BIAS (1.0 / 256.0)

// nearestTexel reads the texel of level 0 under uv.
float4 nearestTexel(float2 uv) {
    uint width, height, levels;
    picture.GetDimensions(0, width, height, levels);
    int2 size = int2(width, height);
    int2 t = int2(floor(uv * float2(size) + NEAREST_BIAS));
    return picture.Load(int3(clamp(t, int2(0, 0), size - 1), 0));
}

// levelTexel reads texel t of a level of the given size, repeating its edge.
float4 levelTexel(int2 t, int2 size, uint level) {
    return picture.Load(int3(clamp(t, int2(0, 0), size - 1), level));
}

// bilinearLevel blends the four texels of a level around uv, as a bilinear
// sampler that clamps does, but with exact weights.
float4 bilinearLevel(float2 uv, uint level) {
    uint width, height, levels;
    picture.GetDimensions(level, width, height, levels);
    int2 size = int2(width, height);
    float2 at = uv * float2(size) - 0.5;
    float2 base = floor(at);
    float2 f = at - base;
    int2 t = int2(base);
    float4 top = lerp(levelTexel(t, size, level), levelTexel(t + int2(1, 0), size, level), f.x);
    float4 bottom = lerp(levelTexel(t + int2(0, 1), size, level), levelTexel(t + int2(1, 1), size, level), f.x);
    return lerp(top, bottom, f.y);
}

// MIN_LOD is the level of detail below which a mipmapped image counts as not
// shrunk, so that drawing at its own size does not depend on rounding.
#define MIN_LOD (1.0 / 256.0)

// levelOfDetail is log2 of how many texels of level 0, whose size is given,
// one physical pixel spans in the direction where it spans the most: the
// larger singular value of the map from the surface to the texels. Rotation
// does not change it, and a shape squeezed along one axis takes the level of
// that axis.
float levelOfDetail(Varyings v, float2 size) {
    // Texels per logical pixel of Rect, along x and y.
    float2 texels = (v.gradient.zw - v.gradient.xy) * size / v.rect.zw;
    float stretch;
    if (turned(v.transform)) {
        // M = diag(texels)·T⁻¹, where T = [[a, c], [b, d]] takes Rect to the
        // surface. The sign of the determinant changes neither singular value.
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

// mipmapped blends the two levels around the level of detail, each read
// bilinearly. Where the image does not shrink, it reads as FilterLinear does.
float4 mipmapped(Varyings v, float2 uv) {
    uint width, height, levels;
    picture.GetDimensions(0, width, height, levels);
    float lod = levelOfDetail(v, float2(width, height));
    float4 c;
    if (lod > MIN_LOD) {
        lod = min(lod, float(levels - 1));
        float low = floor(lod);
        float f = lod - low;
        c = bilinearLevel(uv, uint(low));
        if (f > 0.0) {
            c = lerp(c, bilinearLevel(uv, uint(low) + 1), f);
        }
    } else {
        // Not shrunk, or a level of detail that is not a number.
        c = picture.SampleLevel(pictureSampler, uv, 0.0);
    }
    return c;
}

// Gradient carries the texture coordinates of the top left and bottom right.
// Turned, the quad has a margin for the antialiased edge, which repeats the
// outermost texels.
float4 image(Varyings v, out float coverage) {
    float2 t = (v.logical - v.rect.xy) / v.rect.zw;
    coverage = 1.0;
    if (turned(v.transform)) {
        t = saturate((exactLogical(v) - v.rect.xy) / v.rect.zw);
        float2 halfSize = v.rect.zw * 0.5;
        float2 p = v.logical - v.rect.xy - halfSize;
        float4 square = float4(0.0, 0.0, 0.0, 0.0);
        float distance = roundedBoxDistance(p, halfSize, square);
        coverage = saturate(0.5 - distance / edgeStep(roundedBoxNormal(p, halfSize, square), v.transform));
    }
    float2 uv = lerp(v.gradient.xy, v.gradient.zw, t);
    float4 c;
    if (v.mode.y == FILTER_NEAREST) {
        c = nearestTexel(uv);
    } else if (v.mode.y == FILTER_MIPMAP) {
        c = mipmapped(v, uv);
    } else {
        c = picture.SampleLevel(pictureSampler, uv, 0.0);
    }
    return c * (v.color.a * coverage);
}

// blurFactor is gpu.BlurFactor: how much a blur of standard deviation sigma,
// in physical pixels, shrinks the picture.
float blurFactor(float sigma) {
    return sigma < 8.0 ? 2.0 : (sigma < 32.0 ? 4.0 : 8.0);
}

// clampTexel keeps t inside r, whose corners are both inside.
int2 clampTexel(int2 t, float4 r) {
    return clamp(t, int2(r.xy), int2(r.zw));
}

// blurDown is the first pass of a blur: a texel of the shrunk picture is the
// mean of the factor by factor texels of the source under it. Radii hold the
// texels of the source, past which it repeats its edge; BorderWidth is the
// factor, and Blur says whether the source is the image instead of the
// backdrop.
float4 blurDown(Varyings v) {
    int f = int(v.params.x);
    int2 base = int2(floor(v.position.xy)) * f;
    float4 sum = float4(0.0, 0.0, 0.0, 0.0);
    [loop]
    for (int y = 0; y < f; y++) {
        [loop]
        for (int x = 0; x < f; x++) {
            int3 t = int3(clampTexel(base + int2(x, y), v.radii), 0);
            if (v.params.y > 0.0) {
                sum += picture.Load(t);
            } else {
                sum += backdrop.Load(t);
            }
        }
    }
    return sum / float(f * f);
}

// blurLine is a pass of the separable Gaussian along direction, over the shrunk
// picture in filtered. BorderWidth is the standard deviation in texels, and
// Radii hold the texels written, past which the picture repeats its edge. The
// weights are those of gpu.BlurRadius texels on each side, normalized.
float4 blurLine(Varyings v, int2 direction) {
    float sigma = v.params.x;
    int radius = min(int(ceil(3.0 * sigma)), 48);
    float falloff = -0.5 / (sigma * sigma);
    int2 center = int2(floor(v.position.xy));
    float4 sum = float4(0.0, 0.0, 0.0, 0.0);
    float total = 0.0;
    [loop]
    for (int k = -radius; k <= radius; k++) {
        float w = exp(float(k * k) * falloff);
        sum += filtered.Load(int3(clampTexel(center + direction * k, v.radii), 0)) * w;
        total += w;
    }
    return sum / total;
}

// blurredTexels reads the blurred picture at p, in shrunk texels whose centers
// are whole numbers, by blending the four texels around it. The passes wrote a
// texel more than the source covers on every side, so only the top left can
// run out, at the edge of the target.
float4 blurredTexels(float2 p) {
    float2 base = floor(p);
    float2 t = p - base;
    int2 low = max(int2(base), 0);
    int2 high = max(int2(base) + 1, 0);
    float4 top = lerp(filtered.Load(int3(low.x, low.y, 0)), filtered.Load(int3(high.x, low.y, 0)), t.x);
    float4 bottom = lerp(filtered.Load(int3(low.x, high.y, 0)), filtered.Load(int3(high.x, high.y, 0)), t.x);
    return lerp(top, bottom, t.y);
}

// backdropBlur draws the blurred backdrop into the rounded box. The blurred
// picture lies at the place of the target, shrunk.
float4 backdropBlur(Varyings v, out float coverage) {
    rrect(v, coverage);
    float f = blurFactor(v.params.y * scale);
    return blurredTexels(v.position.xy / f - 0.5) * (v.color.a * coverage);
}

// filteredImage draws the blurred image as image draws the image: the texture
// coordinates of Gradient lead to the place in the image, which the blurred
// picture holds shrunk.
float4 filteredImage(Varyings v, out float coverage) {
    float2 t = (v.logical - v.rect.xy) / v.rect.zw;
    coverage = 1.0;
    if (turned(v.transform)) {
        t = saturate(t);
        float2 halfSize = v.rect.zw * 0.5;
        float2 p = v.logical - v.rect.xy - halfSize;
        float4 square = float4(0.0, 0.0, 0.0, 0.0);
        float distance = roundedBoxDistance(p, halfSize, square);
        coverage = saturate(0.5 - distance / edgeStep(roundedBoxNormal(p, halfSize, square), v.transform));
    }
    uint width, height;
    picture.GetDimensions(width, height);
    float2 at = lerp(v.gradient.xy, v.gradient.zw, t) * float2(width, height);
    float f = blurFactor(v.params.y * scale);
    return blurredTexels(at / f - 0.5) * (v.color.a * coverage);
}

float4 shade(Varyings v, uint kind, out float coverage) {
    coverage = 1.0;
    float4 c;
    if (kind == KIND_SHADOW) {
        c = shadow(v, coverage);
    } else if (kind == KIND_GLYPH) {
        c = glyph(v, coverage);
    } else if (kind == KIND_COVER) {
        c = fill(v);
    } else if (kind == KIND_IMAGE) {
        c = image(v, coverage);
    } else if (kind == KIND_BACKDROP) {
        c = backdropBlur(v, coverage);
    } else if (kind == KIND_FILTERED) {
        c = filteredImage(v, coverage);
    } else if (kind == KIND_BLUR_DOWN) {
        c = blurDown(v);
    } else if (kind == KIND_BLUR_X) {
        c = blurLine(v, int2(1, 0));
    } else if (kind == KIND_BLUR_Y) {
        c = blurLine(v, int2(0, 1));
    } else {
        c = rrect(v, coverage);
    }
    return c;
}

#define BLEND_BACKDROP backdrop
#include "blend.hlsli"

#ifdef __spirv__
// SCISSOR_IN_SHADER repeats the scissor test here, on the pixel, for drivers
// that get it wrong: Mesa's lavapipe, in a multisampled pass, lets samples of
// the column or row just outside the scissor rectangle through. Vulkan sets it
// only there, so that elsewhere the test and its discard compile away.
[[vk::constant_id(0)]] const bool SCISSOR_IN_SHADER = false;

// Scissor holds the scissor rectangle of the draw call, in physical pixels:
// left, top, right and bottom, the last two past the edge.
struct Scissor {
    int4 bounds;
};
[[vk::push_constant]] Scissor scissor;
#endif

float4 main(Varyings v) : SV_Target {
#ifdef __spirv__
    if (SCISSOR_IN_SHADER) {
        int2 p = int2(floor(v.position.xy));
        if (any(p < scissor.bounds.xy) || any(p >= scissor.bounds.zw)) {
            discard;
        }
    }
#endif
    float coverage;
    float4 c = shade(v, v.mode.x & KIND_MASK, coverage);
    uint mode = v.mode.x >> BLEND_SHIFT;
    if (mode == BLEND_SRC_OVER) {
        return c;
    }
    return blendOutput(c, coverage, mode, v.position);
}
