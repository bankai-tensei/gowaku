#include "sdf.hlsli"

// The smallest factor by which the transform stretches a length: the smaller
// singular value of its linear part.
float minScale(float4 t) {
    float sum = dot(t, t);
    float det = t.x * t.w - t.y * t.z;
    float largest = sqrt((sum + sqrt(max(sum * sum - 4.0 * det * det, 0.0))) * 0.5);
    return abs(det) / max(largest, 1e-12);
}

// Expands each instance to a quad. There is no vertex buffer for the corners:
// a four-vertex triangle strip derives them from the vertex ID.
Varyings main(Instance instance, uint vertex : SV_VertexID) {
    bool isTurned = turned(instance.transform);
    uint kind = instance.mode.x & KIND_MASK;
    // One physical pixel of room for the antialiased edge. Under a transform it
    // is measured on the surface, and the quad grows in its own coordinates.
    float margin = 1.0 / scale;
    if (isTurned) {
        margin /= max(minScale(instance.transform), 1e-6);
    }
    if (kind == KIND_SHADOW) {
        // Three standard deviations, where sigma is half the blur radius.
        margin += 1.5 * instance.params.y;
    }
    if (kind >= KIND_GLYPH && kind != KIND_BACKDROP) {
        // These have the exact size of their rectangle, with no edge of their
        // own to antialias. Turned, an image smooths its edge, and a glyph
        // needs a texel around it for the bilinear filter to fade out. A
        // backdrop has the edge of a rounded box.
        if (!isTurned || kind == KIND_COVER) {
            margin = 0.0;
        } else if (kind == KIND_GLYPH) {
            margin = 1.0 / scale;
        }
    }
    float2 corner = float2(vertex & 1, vertex >> 1);
    float2 logical = instance.rect.xy - margin + corner * (instance.rect.zw + 2.0 * margin);
    float2 surface = logical;
    if (isTurned) {
        float4 t = instance.transform;
        surface = float2(t.x * logical.x + t.z * logical.y, t.y * logical.x + t.w * logical.y) + instance.translation;
    }
    float2 ndc = surface * scale / viewport * 2.0 - 1.0;

    Varyings v;
    v.position = float4(ndc.x, -ndc.y, 0.0, 1.0);
    v.logical = logical;
    v.rect = instance.rect;
    v.radii = instance.radii;
    v.color = instance.color;
    v.endColor = instance.endColor;
    v.borderColor = instance.borderColor;
    v.gradient = instance.gradient;
    v.params = instance.params;
    v.mode = instance.mode;
    v.midColor0 = instance.midColor0;
    v.midColor1 = instance.midColor1;
    v.stops = instance.stops;
    v.transform = instance.transform;
    v.translation = instance.translation;
    v.gradientMatrix = instance.gradientMatrix;
    return v;
}
