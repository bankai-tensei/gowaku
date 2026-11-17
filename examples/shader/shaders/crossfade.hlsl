// crossfade blends tex0 into tex1, and tints the result: a shader of two
// textures, which the parity tests of gpu draw.
//
//	params[0]  x: how far along, from 0 at tex0 to 1 at tex1
//	params[1]  the tint, not premultiplied; its alpha is the strength
float4 shade(ShaderInput i) {
    float4 c = lerp(sample0(i.uv), sample1(i.uv), i.params[0].x);
    return float4(lerp(c.rgb, i.params[1].rgb * c.a, i.params[1].a), c.a);
}
