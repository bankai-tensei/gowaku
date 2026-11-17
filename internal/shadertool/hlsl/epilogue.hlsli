// The epilogue that gowaku-shader puts after a custom shader: the entry point,
// which builds ShaderInput, calls shade and blends the result as the SDF shader
// does. gpu/soft does the same in shader.go of the package.

// gowakuEdge is the coverage of the edge of a turned rectangle, as image in
// sdf.ps.hlsl measures it: the distance to the box over the distance one
// physical pixel spans, worked out from the transform.
float gowakuEdge(GowakuVaryings v) {
    float2 halfSize = v.rect.zw * 0.5;
    float2 p = v.logical - v.rect.xy - halfSize;
    float2 q = abs(p) - halfSize;
    float distance = min(max(q.x, q.y), 0.0) + length(max(q, 0.0));
    float2 n = q.x > q.y ? float2(1.0, 0.0) : float2(0.0, 1.0);
    if (q.x > 0.0 && q.y > 0.0) {
        n = normalize(q);
    }
    n = float2(p.x < 0.0 ? -n.x : n.x, p.y < 0.0 ? -n.y : n.y);
    float4 t = v.transform;
    float det = t.x * t.w - t.y * t.z;
    float2 perPixel = float2(dot(n, float2(t.w, -t.y)), dot(n, float2(-t.z, t.x)));
    float step = max(length(perPixel) / (abs(det) * gowakuScale), 1e-12);
    return saturate(0.5 - distance / step);
}

// gowakuExactLogical is the point of the rectangle under the pixel center, the
// inverse transform of the center, as exactLogical in sdf.ps.hlsl.
float2 gowakuExactLogical(GowakuVaryings v) {
    float4 t = v.transform;
    float2 s = v.position.xy / gowakuScale - v.translation;
    return float2(t.w * s.x - t.z * s.y, t.x * s.y - t.y * s.x) / (t.x * t.w - t.y * t.z);
}

float4 main(GowakuVaryings v) : SV_Target {
    float zoom = v.params.y > 0.0 ? v.params.y : 1.0;
    float coverage = 1.0;
    float2 local;
    if (any(v.transform != 0.0)) {
        local = clamp(gowakuExactLogical(v) - v.rect.xy, 0.0, v.rect.zw);
        coverage = gowakuEdge(v);
    } else {
        local = v.position.xy / gowakuScale - v.rect.xy;
    }
    gowakuPlace0 = v.endColor;
    gowakuTexels0 = v.borderColor;
    gowakuPlace1 = v.midColor0;
    gowakuTexels1 = v.midColor1;
    ShaderInput i;
    i.pos = local / zoom;
    i.size = v.rect.zw / zoom;
    i.uv = local / v.rect.zw;
    i.time = v.params.x;
    i.scale = gowakuScale * zoom;
    i.params[0] = v.radii;
    i.params[1] = v.stops;
    i.params[2] = v.gradientMatrix;
    i.params[3] = v.gradient;
    float4 c = shade(i) * (v.color.a * coverage);
    uint mode = v.mode.x >> GOWAKU_BLEND_SHIFT;
    if (mode == BLEND_SRC_OVER) {
        return c;
    }
    return blendOutput(c, coverage, mode, v.position);
}
