// shimmer runs a band of light across what it masks, which it reads as tex0:
// ShaderMask hands it the drawing of its child. Where the child draws nothing
// it stays transparent.
//
//	params[0]  x: passes per second; y: the width of the band, in uv; z: its slant
//	params[1]  the color of the light, not premultiplied; its alpha is the strength
float4 shade(ShaderInput i) {
    float4 c = sample0(i.uv);
    float at = frac(i.time * i.params[0].x) * 1.6 - 0.3;
    float x = i.uv.x + (i.uv.y - 0.5) * i.params[0].z;
    float band = saturate(1.0 - abs(x - at) / i.params[0].y);
    float3 light = i.params[1].rgb * c.a;
    return float4(lerp(c.rgb, light, band * i.params[1].a), c.a);
}
