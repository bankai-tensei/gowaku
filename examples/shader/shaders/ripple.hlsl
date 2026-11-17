// ripple bends what it masks, which it reads as tex0, with rings that run
// out from a center and fade toward the edge.
//
//	params[0]  xy: the center, in uv; z: how far the rings move a pixel, in
//	           logical pixels; w: rings per logical pixel, in radians
//	params[1]  x: the speed of the rings, in radians per second
float4 shade(ShaderInput i) {
    float2 c = i.pos - i.params[0].xy * i.size;
    float d = length(c);
    float reach = 0.5 * length(i.size);
    float push = i.params[0].z * sin(d * i.params[0].w - i.time * i.params[1].x) * saturate(1.0 - d / reach);
    float2 direction = d > 0.0 ? c / d : float2(0.0, 0.0);
    return sample0(i.uv + direction * push / i.size);
}
