// aurora fills its rectangle with rings that run out from the middle over a
// gradient: the background of examples/shader, drawn by CustomPaint.
//
//	params[0]  the color at the left, not premultiplied
//	params[1]  the color at the right
//	params[2]  x: rings per logical pixel, in radians; y: their speed, in
//	           radians per second; z: how much they move the gradient
float4 shade(ShaderInput i) {
    float2 c = i.pos - i.size * 0.5;
    float d = length(c);
    float wave = 0.5 + 0.5 * sin(d * i.params[2].x - i.time * i.params[2].y);
    float t = saturate(i.uv.x + (wave - 0.5) * i.params[2].z);
    float4 color = lerp(i.params[0], i.params[1], t);
    return float4(color.rgb * color.a, color.a);
}
