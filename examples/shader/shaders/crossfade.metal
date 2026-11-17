#include <metal_stdlib>
#include <simd/simd.h>

using namespace metal;

struct type_GowakuFrame
{
    float2 gowakuViewport;
    float gowakuScale;
    float gowakuPadding;
};

struct main0_out
{
    float4 out_var_SV_Target [[color(0)]];
};

struct main0_in
{
    float2 in_var_LOGICAL [[user(locn0)]];
    float4 in_var_RECT [[user(locn1), flat]];
    float4 in_var_RADII [[user(locn2), flat]];
    float4 in_var_COLOR0 [[user(locn3), flat]];
    float4 in_var_COLOR1 [[user(locn4), flat]];
    float4 in_var_COLOR2 [[user(locn5), flat]];
    uint2 in_var_MODE [[user(locn8)]];
    float4 in_var_COLOR3 [[user(locn9), flat]];
    float4 in_var_COLOR4 [[user(locn10), flat]];
    float4 in_var_STOPS [[user(locn11), flat]];
    float4 in_var_TRANSFORM0 [[user(locn12), flat]];
    float2 in_var_TRANSFORM1 [[user(locn13), flat]];
};

fragment main0_out main0(main0_in in [[stage_in]], constant type_GowakuFrame& GowakuFrame [[buffer(0)]], texture2d<float> gowakuTex1 [[texture(0)]], texture2d<float> gowakuTex0 [[texture(1)]], texture2d<float> gowakuBackdrop [[texture(3)]], float4 gl_FragCoord [[position]])
{
    main0_out out = {};
    float4 _1135;
    do
    {
        float _206;
        float2 _207;
        if (any(in.in_var_TRANSFORM0 != float4(0.0)))
        {
            float2 _118 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_TRANSFORM1;
            float _120 = _118.x;
            float _123 = _118.y;
            float _134 = (in.in_var_TRANSFORM0.x * in.in_var_TRANSFORM0.w) - (in.in_var_TRANSFORM0.y * in.in_var_TRANSFORM0.z);
            float2 _141 = in.in_var_RECT.zw * 0.5;
            float2 _143 = (in.in_var_LOGICAL - in.in_var_RECT.xy) - _141;
            float2 _145 = abs(_143) - _141;
            float _146 = _145.x;
            float _147 = _145.y;
            bool _160;
            if (_146 > 0.0)
            {
                _160 = _147 > 0.0;
            }
            else
            {
                _160 = false;
            }
            float2 _164;
            if (_160)
            {
                _164 = fast::normalize(_145);
            }
            else
            {
                _164 = select(float2(0.0, 1.0), float2(1.0, 0.0), bool2(_146 > _147));
            }
            float _173;
            if (_143.x < 0.0)
            {
                _173 = -_164.x;
            }
            else
            {
                _173 = _164.x;
            }
            float _182;
            if (_143.y < 0.0)
            {
                _182 = -_164.y;
            }
            else
            {
                _182 = _164.y;
            }
            float2 _183 = float2(_173, _182);
            _206 = fast::clamp(0.5 - ((precise::min(precise::max(_146, _147), 0.0) + length(precise::max(_145, float2(0.0)))) / precise::max(length(float2(dot(_183, float2(in.in_var_TRANSFORM0.w, -in.in_var_TRANSFORM0.y)), dot(_183, float2(-in.in_var_TRANSFORM0.z, in.in_var_TRANSFORM0.x)))) / (abs(_134) * GowakuFrame.gowakuScale), 9.9999999600419720025001879548654e-13)), 0.0, 1.0);
            _207 = fast::clamp((float2((in.in_var_TRANSFORM0.w * _120) - (in.in_var_TRANSFORM0.z * _123), (in.in_var_TRANSFORM0.x * _123) - (in.in_var_TRANSFORM0.y * _120)) / float2(_134)) - in.in_var_RECT.xy, float2(0.0), in.in_var_RECT.zw);
        }
        else
        {
            _206 = 1.0;
            _207 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_RECT.xy;
        }
        float2 _220;
        float2 _209 = _207 / in.in_var_RECT.zw;
        float2 _213 = mix(in.in_var_COLOR1.xy, in.in_var_COLOR1.zw, _209) - float2(0.5);
        float2 _214 = floor(_213);
        float2 _215 = _213 - _214;
        int2 _216 = int2(_214);
        float4 _238;
        do
        {
            float2 _219 = float2(_216);
            _220 = in.in_var_COLOR2.xy;
            bool _229;
            if (!any(_219 < _220))
            {
                _229 = any(_219 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _229 = true;
            }
            if (_229)
            {
                _238 = float4(0.0);
                break;
            }
            _238 = gowakuTex0.read(uint2(int3(_216, 0).xy), 0);
            break;
        } while(false);
        int2 _239 = _216 + int2(1, 0);
        float4 _260;
        do
        {
            float2 _242 = float2(_239);
            bool _251;
            if (!any(_242 < _220))
            {
                _251 = any(_242 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _251 = true;
            }
            if (_251)
            {
                _260 = float4(0.0);
                break;
            }
            _260 = gowakuTex0.read(uint2(int3(_239, 0).xy), 0);
            break;
        } while(false);
        float4 _262 = float4(_215.x);
        int2 _264 = _216 + int2(0, 1);
        float4 _285;
        do
        {
            float2 _267 = float2(_264);
            bool _276;
            if (!any(_267 < _220))
            {
                _276 = any(_267 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _276 = true;
            }
            if (_276)
            {
                _285 = float4(0.0);
                break;
            }
            _285 = gowakuTex0.read(uint2(int3(_264, 0).xy), 0);
            break;
        } while(false);
        int2 _286 = _216 + int2(1);
        float4 _307;
        do
        {
            float2 _289 = float2(_286);
            bool _298;
            if (!any(_289 < _220))
            {
                _298 = any(_289 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _298 = true;
            }
            if (_298)
            {
                _307 = float4(0.0);
                break;
            }
            _307 = gowakuTex0.read(uint2(int3(_286, 0).xy), 0);
            break;
        } while(false);
        float2 _322;
        float2 _315 = mix(in.in_var_COLOR3.xy, in.in_var_COLOR3.zw, _209) - float2(0.5);
        float2 _316 = floor(_315);
        float2 _317 = _315 - _316;
        int2 _318 = int2(_316);
        float4 _340;
        do
        {
            float2 _321 = float2(_318);
            _322 = in.in_var_COLOR4.xy;
            bool _331;
            if (!any(_321 < _322))
            {
                _331 = any(_321 >= in.in_var_COLOR4.zw);
            }
            else
            {
                _331 = true;
            }
            if (_331)
            {
                _340 = float4(0.0);
                break;
            }
            _340 = gowakuTex1.read(uint2(int3(_318, 0).xy), 0);
            break;
        } while(false);
        int2 _341 = _318 + int2(1, 0);
        float4 _362;
        do
        {
            float2 _344 = float2(_341);
            bool _353;
            if (!any(_344 < _322))
            {
                _353 = any(_344 >= in.in_var_COLOR4.zw);
            }
            else
            {
                _353 = true;
            }
            if (_353)
            {
                _362 = float4(0.0);
                break;
            }
            _362 = gowakuTex1.read(uint2(int3(_341, 0).xy), 0);
            break;
        } while(false);
        float4 _364 = float4(_317.x);
        int2 _366 = _318 + int2(0, 1);
        float4 _387;
        do
        {
            float2 _369 = float2(_366);
            bool _378;
            if (!any(_369 < _322))
            {
                _378 = any(_369 >= in.in_var_COLOR4.zw);
            }
            else
            {
                _378 = true;
            }
            if (_378)
            {
                _387 = float4(0.0);
                break;
            }
            _387 = gowakuTex1.read(uint2(int3(_366, 0).xy), 0);
            break;
        } while(false);
        int2 _388 = _318 + int2(1);
        float4 _409;
        do
        {
            float2 _391 = float2(_388);
            bool _400;
            if (!any(_391 < _322))
            {
                _400 = any(_391 >= in.in_var_COLOR4.zw);
            }
            else
            {
                _400 = true;
            }
            if (_400)
            {
                _409 = float4(0.0);
                break;
            }
            _409 = gowakuTex1.read(uint2(int3(_388, 0).xy), 0);
            break;
        } while(false);
        float4 _416 = mix(mix(mix(_238, _260, _262), mix(_285, _307, _262), float4(_215.y)), mix(mix(_340, _362, _364), mix(_387, _409, _364), float4(_317.y)), float4(in.in_var_RADII.x));
        float _419 = _416.w;
        float4 _430 = float4(mix(_416.xyz, in.in_var_STOPS.xyz * _419, float3(in.in_var_STOPS.w)), _419) * (in.in_var_COLOR0.w * _206);
        uint _432 = in.in_var_MODE.x >> 8u;
        if (_432 == 0u)
        {
            _1135 = _430;
            break;
        }
        float4 _1134;
        do
        {
            if (_432 == 1u)
            {
                _1134 = float4(0.0, 0.0, 0.0, _206);
                break;
            }
            bool _447;
            if (!(_432 == 6u))
            {
                _447 = _432 == 14u;
            }
            else
            {
                _447 = true;
            }
            if (_447)
            {
                _1134 = float4(_206) - _430;
                break;
            }
            bool _452 = _432 == 2u;
            bool _457;
            if (!_452)
            {
                _457 = _432 == 5u;
            }
            else
            {
                _457 = true;
            }
            bool _462;
            if (!_457)
            {
                _462 = _432 == 7u;
            }
            else
            {
                _462 = true;
            }
            bool _467;
            if (!_462)
            {
                _467 = _432 == 10u;
            }
            else
            {
                _467 = true;
            }
            bool _472;
            if (!_467)
            {
                _472 = _432 >= 15u;
            }
            else
            {
                _472 = true;
            }
            if (_472)
            {
                float4 _1133;
                do
                {
                    float4 _485 = gowakuBackdrop.read(uint2(int3(int2(floor(gl_FragCoord.xy)), 0).xy), 0);
                    float4 _487 = _485 * (1.0 - _206);
                    if (_452)
                    {
                        _1133 = _430 + _487;
                        break;
                    }
                    if (_432 == 5u)
                    {
                        _1133 = (_430 * _485.w) + _487;
                        break;
                    }
                    if (_432 == 7u)
                    {
                        _1133 = (_430 * (1.0 - _485.w)) + _487;
                        break;
                    }
                    if (_432 == 10u)
                    {
                        _1133 = ((_430 * (1.0 - _485.w)) + (_485 * _430.w)) + _487;
                        break;
                    }
                    float _514 = _430.w;
                    if (_514 <= 0.0)
                    {
                        _1133 = float4(0.0);
                        break;
                    }
                    float3 _521 = fast::clamp(_430.xyz / float3(_514), float3(0.0), float3(1.0));
                    float _522 = _485.w;
                    float3 _531;
                    if (_522 > 0.0)
                    {
                        _531 = fast::clamp(_485.xyz / float3(_522), float3(0.0), float3(1.0));
                    }
                    else
                    {
                        _531 = float3(0.0);
                    }
                    float3 _1124;
                    do
                    {
                        if (_432 == 15u)
                        {
                            _1124 = _531 * _521;
                            break;
                        }
                        if (_432 == 16u)
                        {
                            float _543 = _521.x;
                            float _556;
                            if (_531.x <= 0.5)
                            {
                                _556 = (_543 * 2.0) * _531.x;
                            }
                            else
                            {
                                float _552 = (2.0 * _531.x) - 1.0;
                                _556 = (_543 + _552) - (_543 * _552);
                            }
                            float _557 = _521.y;
                            float _570;
                            if (_531.y <= 0.5)
                            {
                                _570 = (_557 * 2.0) * _531.y;
                            }
                            else
                            {
                                float _566 = (2.0 * _531.y) - 1.0;
                                _570 = (_557 + _566) - (_557 * _566);
                            }
                            float _571 = _521.z;
                            float _584;
                            if (_531.z <= 0.5)
                            {
                                _584 = (_571 * 2.0) * _531.z;
                            }
                            else
                            {
                                float _580 = (2.0 * _531.z) - 1.0;
                                _584 = (_571 + _580) - (_571 * _580);
                            }
                            _1124 = float3(_556, _570, _584);
                            break;
                        }
                        if (_432 == 17u)
                        {
                            _1124 = precise::min(_531, _521);
                            break;
                        }
                        if (_432 == 18u)
                        {
                            _1124 = precise::max(_531, _521);
                            break;
                        }
                        if (_432 == 19u)
                        {
                            float _598 = _521.x;
                            float _612;
                            do
                            {
                                if (_531.x <= 0.0)
                                {
                                    _612 = 0.0;
                                    break;
                                }
                                float _611;
                                if (_598 >= 1.0)
                                {
                                    _611 = 1.0;
                                }
                                else
                                {
                                    _611 = precise::min(1.0, _531.x / (1.0 - _598));
                                }
                                _612 = _611;
                                break;
                            } while(false);
                            float _614 = _521.y;
                            float _628;
                            do
                            {
                                if (_531.y <= 0.0)
                                {
                                    _628 = 0.0;
                                    break;
                                }
                                float _627;
                                if (_614 >= 1.0)
                                {
                                    _627 = 1.0;
                                }
                                else
                                {
                                    _627 = precise::min(1.0, _531.y / (1.0 - _614));
                                }
                                _628 = _627;
                                break;
                            } while(false);
                            float _630 = _521.z;
                            float _644;
                            do
                            {
                                if (_531.z <= 0.0)
                                {
                                    _644 = 0.0;
                                    break;
                                }
                                float _643;
                                if (_630 >= 1.0)
                                {
                                    _643 = 1.0;
                                }
                                else
                                {
                                    _643 = precise::min(1.0, _531.z / (1.0 - _630));
                                }
                                _644 = _643;
                                break;
                            } while(false);
                            _1124 = float3(_612, _628, _644);
                            break;
                        }
                        if (_432 == 20u)
                        {
                            float _650 = _521.x;
                            float _665;
                            do
                            {
                                if (_531.x >= 1.0)
                                {
                                    _665 = 1.0;
                                    break;
                                }
                                float _664;
                                if (_650 <= 0.0)
                                {
                                    _664 = 0.0;
                                }
                                else
                                {
                                    _664 = 1.0 - precise::min(1.0, (1.0 - _531.x) / _650);
                                }
                                _665 = _664;
                                break;
                            } while(false);
                            float _667 = _521.y;
                            float _682;
                            do
                            {
                                if (_531.y >= 1.0)
                                {
                                    _682 = 1.0;
                                    break;
                                }
                                float _681;
                                if (_667 <= 0.0)
                                {
                                    _681 = 0.0;
                                }
                                else
                                {
                                    _681 = 1.0 - precise::min(1.0, (1.0 - _531.y) / _667);
                                }
                                _682 = _681;
                                break;
                            } while(false);
                            float _684 = _521.z;
                            float _699;
                            do
                            {
                                if (_531.z >= 1.0)
                                {
                                    _699 = 1.0;
                                    break;
                                }
                                float _698;
                                if (_684 <= 0.0)
                                {
                                    _698 = 0.0;
                                }
                                else
                                {
                                    _698 = 1.0 - precise::min(1.0, (1.0 - _531.z) / _684);
                                }
                                _699 = _698;
                                break;
                            } while(false);
                            _1124 = float3(_665, _682, _699);
                            break;
                        }
                        if (_432 == 21u)
                        {
                            float _705 = _521.x;
                            float _717;
                            if (_705 <= 0.5)
                            {
                                _717 = (_531.x * 2.0) * _705;
                            }
                            else
                            {
                                float _713 = (2.0 * _705) - 1.0;
                                _717 = (_531.x + _713) - (_531.x * _713);
                            }
                            float _719 = _521.y;
                            float _731;
                            if (_719 <= 0.5)
                            {
                                _731 = (_531.y * 2.0) * _719;
                            }
                            else
                            {
                                float _727 = (2.0 * _719) - 1.0;
                                _731 = (_531.y + _727) - (_531.y * _727);
                            }
                            float _733 = _521.z;
                            float _745;
                            if (_733 <= 0.5)
                            {
                                _745 = (_531.z * 2.0) * _733;
                            }
                            else
                            {
                                float _741 = (2.0 * _733) - 1.0;
                                _745 = (_531.z + _741) - (_531.z * _741);
                            }
                            _1124 = float3(_717, _731, _745);
                            break;
                        }
                        if (_432 == 22u)
                        {
                            float _751 = _521.x;
                            float _779;
                            do
                            {
                                if (_751 <= 0.5)
                                {
                                    _779 = _531.x - (((1.0 - (2.0 * _751)) * _531.x) * (1.0 - _531.x));
                                    break;
                                }
                                float _773;
                                if (_531.x <= 0.25)
                                {
                                    _773 = ((((16.0 * _531.x) - 12.0) * _531.x) + 4.0) * _531.x;
                                }
                                else
                                {
                                    _773 = sqrt(_531.x);
                                }
                                _779 = _531.x + (((2.0 * _751) - 1.0) * (_773 - _531.x));
                                break;
                            } while(false);
                            float _781 = _521.y;
                            float _809;
                            do
                            {
                                if (_781 <= 0.5)
                                {
                                    _809 = _531.y - (((1.0 - (2.0 * _781)) * _531.y) * (1.0 - _531.y));
                                    break;
                                }
                                float _803;
                                if (_531.y <= 0.25)
                                {
                                    _803 = ((((16.0 * _531.y) - 12.0) * _531.y) + 4.0) * _531.y;
                                }
                                else
                                {
                                    _803 = sqrt(_531.y);
                                }
                                _809 = _531.y + (((2.0 * _781) - 1.0) * (_803 - _531.y));
                                break;
                            } while(false);
                            float _811 = _521.z;
                            float _839;
                            do
                            {
                                if (_811 <= 0.5)
                                {
                                    _839 = _531.z - (((1.0 - (2.0 * _811)) * _531.z) * (1.0 - _531.z));
                                    break;
                                }
                                float _833;
                                if (_531.z <= 0.25)
                                {
                                    _833 = ((((16.0 * _531.z) - 12.0) * _531.z) + 4.0) * _531.z;
                                }
                                else
                                {
                                    _833 = sqrt(_531.z);
                                }
                                _839 = _531.z + (((2.0 * _811) - 1.0) * (_833 - _531.z));
                                break;
                            } while(false);
                            _1124 = float3(_779, _809, _839);
                            break;
                        }
                        if (_432 == 23u)
                        {
                            _1124 = abs(_531 - _521);
                            break;
                        }
                        if (_432 == 24u)
                        {
                            _1124 = (_531 + _521) - ((_531 * 2.0) * _521);
                            break;
                        }
                        if (_432 == 25u)
                        {
                            float _864 = _521.x;
                            float _865 = _521.y;
                            float _867 = _521.z;
                            float _868 = precise::min(precise::min(_864, _865), _867);
                            float _870 = precise::max(precise::max(_864, _865), _867);
                            float3 _881;
                            if (_870 > _868)
                            {
                                _881 = ((_521 - float3(_868)) * (precise::max(precise::max(_531.x, _531.y), _531.z) - precise::min(precise::min(_531.x, _531.y), _531.z))) / float3(_870 - _868);
                            }
                            else
                            {
                                _881 = float3(0.0);
                            }
                            float3 _897 = _881 + float3((((0.300000011920928955078125 * _531.x) + (0.589999973773956298828125 * _531.y)) + (0.10999999940395355224609375 * _531.z)) - (((0.300000011920928955078125 * _881.x) + (0.589999973773956298828125 * _881.y)) + (0.10999999940395355224609375 * _881.z)));
                            float _898 = _897.x;
                            float _900 = _897.y;
                            float _903 = _897.z;
                            float _905 = ((0.300000011920928955078125 * _898) + (0.589999973773956298828125 * _900)) + (0.10999999940395355224609375 * _903);
                            float _907 = precise::min(precise::min(_898, _900), _903);
                            float _909 = precise::max(precise::max(_898, _900), _903);
                            float3 _920;
                            if (_907 < 0.0)
                            {
                                float3 _913 = float3(_905);
                                _920 = _913 + (((_897 - _913) * _905) / float3(_905 - _907));
                            }
                            else
                            {
                                _920 = _897;
                            }
                            float3 _932;
                            if (_909 > 1.0)
                            {
                                float3 _924 = float3(_905);
                                _932 = _924 + (((_920 - _924) * (1.0 - _905)) / float3(_909 - _905));
                            }
                            else
                            {
                                _932 = _920;
                            }
                            _1124 = _932;
                            break;
                        }
                        if (_432 == 26u)
                        {
                            float _936 = _521.x;
                            float _937 = _521.y;
                            float _939 = _521.z;
                            float _948 = precise::min(precise::min(_531.x, _531.y), _531.z);
                            float _950 = precise::max(precise::max(_531.x, _531.y), _531.z);
                            float3 _961;
                            if (_950 > _948)
                            {
                                _961 = ((_531 - float3(_948)) * (precise::max(precise::max(_936, _937), _939) - precise::min(precise::min(_936, _937), _939))) / float3(_950 - _948);
                            }
                            else
                            {
                                _961 = float3(0.0);
                            }
                            float3 _977 = _961 + float3((((0.300000011920928955078125 * _531.x) + (0.589999973773956298828125 * _531.y)) + (0.10999999940395355224609375 * _531.z)) - (((0.300000011920928955078125 * _961.x) + (0.589999973773956298828125 * _961.y)) + (0.10999999940395355224609375 * _961.z)));
                            float _978 = _977.x;
                            float _980 = _977.y;
                            float _983 = _977.z;
                            float _985 = ((0.300000011920928955078125 * _978) + (0.589999973773956298828125 * _980)) + (0.10999999940395355224609375 * _983);
                            float _987 = precise::min(precise::min(_978, _980), _983);
                            float _989 = precise::max(precise::max(_978, _980), _983);
                            float3 _1000;
                            if (_987 < 0.0)
                            {
                                float3 _993 = float3(_985);
                                _1000 = _993 + (((_977 - _993) * _985) / float3(_985 - _987));
                            }
                            else
                            {
                                _1000 = _977;
                            }
                            float3 _1012;
                            if (_989 > 1.0)
                            {
                                float3 _1004 = float3(_985);
                                _1012 = _1004 + (((_1000 - _1004) * (1.0 - _985)) / float3(_989 - _985));
                            }
                            else
                            {
                                _1012 = _1000;
                            }
                            _1124 = _1012;
                            break;
                        }
                        if (_432 == 27u)
                        {
                            float3 _1034 = _521 + float3((((0.300000011920928955078125 * _531.x) + (0.589999973773956298828125 * _531.y)) + (0.10999999940395355224609375 * _531.z)) - (((0.300000011920928955078125 * _521.x) + (0.589999973773956298828125 * _521.y)) + (0.10999999940395355224609375 * _521.z)));
                            float _1035 = _1034.x;
                            float _1037 = _1034.y;
                            float _1040 = _1034.z;
                            float _1042 = ((0.300000011920928955078125 * _1035) + (0.589999973773956298828125 * _1037)) + (0.10999999940395355224609375 * _1040);
                            float _1044 = precise::min(precise::min(_1035, _1037), _1040);
                            float _1046 = precise::max(precise::max(_1035, _1037), _1040);
                            float3 _1057;
                            if (_1044 < 0.0)
                            {
                                float3 _1050 = float3(_1042);
                                _1057 = _1050 + (((_1034 - _1050) * _1042) / float3(_1042 - _1044));
                            }
                            else
                            {
                                _1057 = _1034;
                            }
                            float3 _1069;
                            if (_1046 > 1.0)
                            {
                                float3 _1061 = float3(_1042);
                                _1069 = _1061 + (((_1057 - _1061) * (1.0 - _1042)) / float3(_1046 - _1042));
                            }
                            else
                            {
                                _1069 = _1057;
                            }
                            _1124 = _1069;
                            break;
                        }
                        float3 _1088 = _531 + float3((((0.300000011920928955078125 * _521.x) + (0.589999973773956298828125 * _521.y)) + (0.10999999940395355224609375 * _521.z)) - (((0.300000011920928955078125 * _531.x) + (0.589999973773956298828125 * _531.y)) + (0.10999999940395355224609375 * _531.z)));
                        float _1089 = _1088.x;
                        float _1091 = _1088.y;
                        float _1094 = _1088.z;
                        float _1096 = ((0.300000011920928955078125 * _1089) + (0.589999973773956298828125 * _1091)) + (0.10999999940395355224609375 * _1094);
                        float _1098 = precise::min(precise::min(_1089, _1091), _1094);
                        float _1100 = precise::max(precise::max(_1089, _1091), _1094);
                        float3 _1111;
                        if (_1098 < 0.0)
                        {
                            float3 _1104 = float3(_1096);
                            _1111 = _1104 + (((_1088 - _1104) * _1096) / float3(_1096 - _1098));
                        }
                        else
                        {
                            _1111 = _1088;
                        }
                        float3 _1123;
                        if (_1100 > 1.0)
                        {
                            float3 _1115 = float3(_1096);
                            _1123 = _1115 + (((_1111 - _1115) * (1.0 - _1096)) / float3(_1100 - _1096));
                        }
                        else
                        {
                            _1123 = _1111;
                        }
                        _1124 = _1123;
                        break;
                    } while(false);
                    _1133 = float4(((_521 * (1.0 - _522)) + (fast::clamp(_1124, float3(0.0), float3(1.0)) * _522)) * _514, _514);
                    break;
                } while(false);
                _1134 = _1133;
                break;
            }
            _1134 = _430;
            break;
        } while(false);
        _1135 = _1134;
        break;
    } while(false);
    out.out_var_SV_Target = _1135;
    return out;
}

