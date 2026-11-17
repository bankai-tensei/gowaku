// The triangles that mark a shape in the stencil buffer (pipeline P2). They
// have no color, so that no pixel shader goes with this one.

cbuffer Frame : register(b0) {
    float2 viewport; // physical pixels
    float scale;     // physical pixels per logical pixel
    float padding;
};

float4 main(float2 logical : POSITION) : SV_Position {
    float2 ndc = logical * scale / viewport * 2.0 - 1.0;
    return float4(ndc.x, -ndc.y, 0.0, 1.0);
}
