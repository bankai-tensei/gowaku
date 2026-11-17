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
    float2 in_var_PARAMS [[user(locn7), flat]];
    uint2 in_var_MODE [[user(locn8)]];
    float4 in_var_STOPS [[user(locn11), flat]];
    float4 in_var_TRANSFORM0 [[user(locn12), flat]];
    float2 in_var_TRANSFORM1 [[user(locn13), flat]];
};

fragment main0_out main0(main0_in in [[stage_in]], constant type_GowakuFrame& GowakuFrame [[buffer(0)]], texture2d<float> gowakuTex0 [[texture(1)]], texture2d<float> gowakuBackdrop [[texture(3)]], float4 gl_FragCoord [[position]])
{
    main0_out out = {};
    float4 _1051;
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
        float2 _208 = float2((in.in_var_PARAMS.y > 0.0) ? in.in_var_PARAMS.y : 1.0);
        float2 _211 = in.in_var_RECT.zw / _208;
        float2 _216 = (_207 / _208) - (in.in_var_RADII.xy * _211);
        float _217 = length(_216);
        float2 _238;
        if (_217 > 0.0)
        {
            _238 = _216 / float2(_217);
        }
        else
        {
            _238 = float2(0.0);
        }
        float2 _252;
        float2 _245 = mix(in.in_var_COLOR1.xy, in.in_var_COLOR1.zw, (_207 / in.in_var_RECT.zw) + ((_238 * ((in.in_var_RADII.z * sin((_217 * in.in_var_RADII.w) - (in.in_var_PARAMS.x * in.in_var_STOPS.x))) * fast::clamp(1.0 - (_217 / (0.5 * length(_211))), 0.0, 1.0))) / _211)) - float2(0.5);
        float2 _246 = floor(_245);
        float2 _247 = _245 - _246;
        int2 _248 = int2(_246);
        float4 _270;
        do
        {
            float2 _251 = float2(_248);
            _252 = in.in_var_COLOR2.xy;
            bool _261;
            if (!any(_251 < _252))
            {
                _261 = any(_251 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _261 = true;
            }
            if (_261)
            {
                _270 = float4(0.0);
                break;
            }
            _270 = gowakuTex0.read(uint2(int3(_248, 0).xy), 0);
            break;
        } while(false);
        int2 _271 = _248 + int2(1, 0);
        float4 _292;
        do
        {
            float2 _274 = float2(_271);
            bool _283;
            if (!any(_274 < _252))
            {
                _283 = any(_274 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _283 = true;
            }
            if (_283)
            {
                _292 = float4(0.0);
                break;
            }
            _292 = gowakuTex0.read(uint2(int3(_271, 0).xy), 0);
            break;
        } while(false);
        float4 _294 = float4(_247.x);
        int2 _296 = _248 + int2(0, 1);
        float4 _317;
        do
        {
            float2 _299 = float2(_296);
            bool _308;
            if (!any(_299 < _252))
            {
                _308 = any(_299 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _308 = true;
            }
            if (_308)
            {
                _317 = float4(0.0);
                break;
            }
            _317 = gowakuTex0.read(uint2(int3(_296, 0).xy), 0);
            break;
        } while(false);
        int2 _318 = _248 + int2(1);
        float4 _339;
        do
        {
            float2 _321 = float2(_318);
            bool _330;
            if (!any(_321 < _252))
            {
                _330 = any(_321 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _330 = true;
            }
            if (_330)
            {
                _339 = float4(0.0);
                break;
            }
            _339 = gowakuTex0.read(uint2(int3(_318, 0).xy), 0);
            break;
        } while(false);
        float4 _346 = mix(mix(_270, _292, _294), mix(_317, _339, _294), float4(_247.y)) * (in.in_var_COLOR0.w * _206);
        uint _348 = in.in_var_MODE.x >> 8u;
        if (_348 == 0u)
        {
            _1051 = _346;
            break;
        }
        float4 _1050;
        do
        {
            if (_348 == 1u)
            {
                _1050 = float4(0.0, 0.0, 0.0, _206);
                break;
            }
            bool _363;
            if (!(_348 == 6u))
            {
                _363 = _348 == 14u;
            }
            else
            {
                _363 = true;
            }
            if (_363)
            {
                _1050 = float4(_206) - _346;
                break;
            }
            bool _368 = _348 == 2u;
            bool _373;
            if (!_368)
            {
                _373 = _348 == 5u;
            }
            else
            {
                _373 = true;
            }
            bool _378;
            if (!_373)
            {
                _378 = _348 == 7u;
            }
            else
            {
                _378 = true;
            }
            bool _383;
            if (!_378)
            {
                _383 = _348 == 10u;
            }
            else
            {
                _383 = true;
            }
            bool _388;
            if (!_383)
            {
                _388 = _348 >= 15u;
            }
            else
            {
                _388 = true;
            }
            if (_388)
            {
                float4 _1049;
                do
                {
                    float4 _401 = gowakuBackdrop.read(uint2(int3(int2(floor(gl_FragCoord.xy)), 0).xy), 0);
                    float4 _403 = _401 * (1.0 - _206);
                    if (_368)
                    {
                        _1049 = _346 + _403;
                        break;
                    }
                    if (_348 == 5u)
                    {
                        _1049 = (_346 * _401.w) + _403;
                        break;
                    }
                    if (_348 == 7u)
                    {
                        _1049 = (_346 * (1.0 - _401.w)) + _403;
                        break;
                    }
                    if (_348 == 10u)
                    {
                        _1049 = ((_346 * (1.0 - _401.w)) + (_401 * _346.w)) + _403;
                        break;
                    }
                    float _430 = _346.w;
                    if (_430 <= 0.0)
                    {
                        _1049 = float4(0.0);
                        break;
                    }
                    float3 _437 = fast::clamp(_346.xyz / float3(_430), float3(0.0), float3(1.0));
                    float _438 = _401.w;
                    float3 _447;
                    if (_438 > 0.0)
                    {
                        _447 = fast::clamp(_401.xyz / float3(_438), float3(0.0), float3(1.0));
                    }
                    else
                    {
                        _447 = float3(0.0);
                    }
                    float3 _1040;
                    do
                    {
                        if (_348 == 15u)
                        {
                            _1040 = _447 * _437;
                            break;
                        }
                        if (_348 == 16u)
                        {
                            float _459 = _437.x;
                            float _472;
                            if (_447.x <= 0.5)
                            {
                                _472 = (_459 * 2.0) * _447.x;
                            }
                            else
                            {
                                float _468 = (2.0 * _447.x) - 1.0;
                                _472 = (_459 + _468) - (_459 * _468);
                            }
                            float _473 = _437.y;
                            float _486;
                            if (_447.y <= 0.5)
                            {
                                _486 = (_473 * 2.0) * _447.y;
                            }
                            else
                            {
                                float _482 = (2.0 * _447.y) - 1.0;
                                _486 = (_473 + _482) - (_473 * _482);
                            }
                            float _487 = _437.z;
                            float _500;
                            if (_447.z <= 0.5)
                            {
                                _500 = (_487 * 2.0) * _447.z;
                            }
                            else
                            {
                                float _496 = (2.0 * _447.z) - 1.0;
                                _500 = (_487 + _496) - (_487 * _496);
                            }
                            _1040 = float3(_472, _486, _500);
                            break;
                        }
                        if (_348 == 17u)
                        {
                            _1040 = precise::min(_447, _437);
                            break;
                        }
                        if (_348 == 18u)
                        {
                            _1040 = precise::max(_447, _437);
                            break;
                        }
                        if (_348 == 19u)
                        {
                            float _514 = _437.x;
                            float _528;
                            do
                            {
                                if (_447.x <= 0.0)
                                {
                                    _528 = 0.0;
                                    break;
                                }
                                float _527;
                                if (_514 >= 1.0)
                                {
                                    _527 = 1.0;
                                }
                                else
                                {
                                    _527 = precise::min(1.0, _447.x / (1.0 - _514));
                                }
                                _528 = _527;
                                break;
                            } while(false);
                            float _530 = _437.y;
                            float _544;
                            do
                            {
                                if (_447.y <= 0.0)
                                {
                                    _544 = 0.0;
                                    break;
                                }
                                float _543;
                                if (_530 >= 1.0)
                                {
                                    _543 = 1.0;
                                }
                                else
                                {
                                    _543 = precise::min(1.0, _447.y / (1.0 - _530));
                                }
                                _544 = _543;
                                break;
                            } while(false);
                            float _546 = _437.z;
                            float _560;
                            do
                            {
                                if (_447.z <= 0.0)
                                {
                                    _560 = 0.0;
                                    break;
                                }
                                float _559;
                                if (_546 >= 1.0)
                                {
                                    _559 = 1.0;
                                }
                                else
                                {
                                    _559 = precise::min(1.0, _447.z / (1.0 - _546));
                                }
                                _560 = _559;
                                break;
                            } while(false);
                            _1040 = float3(_528, _544, _560);
                            break;
                        }
                        if (_348 == 20u)
                        {
                            float _566 = _437.x;
                            float _581;
                            do
                            {
                                if (_447.x >= 1.0)
                                {
                                    _581 = 1.0;
                                    break;
                                }
                                float _580;
                                if (_566 <= 0.0)
                                {
                                    _580 = 0.0;
                                }
                                else
                                {
                                    _580 = 1.0 - precise::min(1.0, (1.0 - _447.x) / _566);
                                }
                                _581 = _580;
                                break;
                            } while(false);
                            float _583 = _437.y;
                            float _598;
                            do
                            {
                                if (_447.y >= 1.0)
                                {
                                    _598 = 1.0;
                                    break;
                                }
                                float _597;
                                if (_583 <= 0.0)
                                {
                                    _597 = 0.0;
                                }
                                else
                                {
                                    _597 = 1.0 - precise::min(1.0, (1.0 - _447.y) / _583);
                                }
                                _598 = _597;
                                break;
                            } while(false);
                            float _600 = _437.z;
                            float _615;
                            do
                            {
                                if (_447.z >= 1.0)
                                {
                                    _615 = 1.0;
                                    break;
                                }
                                float _614;
                                if (_600 <= 0.0)
                                {
                                    _614 = 0.0;
                                }
                                else
                                {
                                    _614 = 1.0 - precise::min(1.0, (1.0 - _447.z) / _600);
                                }
                                _615 = _614;
                                break;
                            } while(false);
                            _1040 = float3(_581, _598, _615);
                            break;
                        }
                        if (_348 == 21u)
                        {
                            float _621 = _437.x;
                            float _633;
                            if (_621 <= 0.5)
                            {
                                _633 = (_447.x * 2.0) * _621;
                            }
                            else
                            {
                                float _629 = (2.0 * _621) - 1.0;
                                _633 = (_447.x + _629) - (_447.x * _629);
                            }
                            float _635 = _437.y;
                            float _647;
                            if (_635 <= 0.5)
                            {
                                _647 = (_447.y * 2.0) * _635;
                            }
                            else
                            {
                                float _643 = (2.0 * _635) - 1.0;
                                _647 = (_447.y + _643) - (_447.y * _643);
                            }
                            float _649 = _437.z;
                            float _661;
                            if (_649 <= 0.5)
                            {
                                _661 = (_447.z * 2.0) * _649;
                            }
                            else
                            {
                                float _657 = (2.0 * _649) - 1.0;
                                _661 = (_447.z + _657) - (_447.z * _657);
                            }
                            _1040 = float3(_633, _647, _661);
                            break;
                        }
                        if (_348 == 22u)
                        {
                            float _667 = _437.x;
                            float _695;
                            do
                            {
                                if (_667 <= 0.5)
                                {
                                    _695 = _447.x - (((1.0 - (2.0 * _667)) * _447.x) * (1.0 - _447.x));
                                    break;
                                }
                                float _689;
                                if (_447.x <= 0.25)
                                {
                                    _689 = ((((16.0 * _447.x) - 12.0) * _447.x) + 4.0) * _447.x;
                                }
                                else
                                {
                                    _689 = sqrt(_447.x);
                                }
                                _695 = _447.x + (((2.0 * _667) - 1.0) * (_689 - _447.x));
                                break;
                            } while(false);
                            float _697 = _437.y;
                            float _725;
                            do
                            {
                                if (_697 <= 0.5)
                                {
                                    _725 = _447.y - (((1.0 - (2.0 * _697)) * _447.y) * (1.0 - _447.y));
                                    break;
                                }
                                float _719;
                                if (_447.y <= 0.25)
                                {
                                    _719 = ((((16.0 * _447.y) - 12.0) * _447.y) + 4.0) * _447.y;
                                }
                                else
                                {
                                    _719 = sqrt(_447.y);
                                }
                                _725 = _447.y + (((2.0 * _697) - 1.0) * (_719 - _447.y));
                                break;
                            } while(false);
                            float _727 = _437.z;
                            float _755;
                            do
                            {
                                if (_727 <= 0.5)
                                {
                                    _755 = _447.z - (((1.0 - (2.0 * _727)) * _447.z) * (1.0 - _447.z));
                                    break;
                                }
                                float _749;
                                if (_447.z <= 0.25)
                                {
                                    _749 = ((((16.0 * _447.z) - 12.0) * _447.z) + 4.0) * _447.z;
                                }
                                else
                                {
                                    _749 = sqrt(_447.z);
                                }
                                _755 = _447.z + (((2.0 * _727) - 1.0) * (_749 - _447.z));
                                break;
                            } while(false);
                            _1040 = float3(_695, _725, _755);
                            break;
                        }
                        if (_348 == 23u)
                        {
                            _1040 = abs(_447 - _437);
                            break;
                        }
                        if (_348 == 24u)
                        {
                            _1040 = (_447 + _437) - ((_447 * 2.0) * _437);
                            break;
                        }
                        if (_348 == 25u)
                        {
                            float _780 = _437.x;
                            float _781 = _437.y;
                            float _783 = _437.z;
                            float _784 = precise::min(precise::min(_780, _781), _783);
                            float _786 = precise::max(precise::max(_780, _781), _783);
                            float3 _797;
                            if (_786 > _784)
                            {
                                _797 = ((_437 - float3(_784)) * (precise::max(precise::max(_447.x, _447.y), _447.z) - precise::min(precise::min(_447.x, _447.y), _447.z))) / float3(_786 - _784);
                            }
                            else
                            {
                                _797 = float3(0.0);
                            }
                            float3 _813 = _797 + float3((((0.300000011920928955078125 * _447.x) + (0.589999973773956298828125 * _447.y)) + (0.10999999940395355224609375 * _447.z)) - (((0.300000011920928955078125 * _797.x) + (0.589999973773956298828125 * _797.y)) + (0.10999999940395355224609375 * _797.z)));
                            float _814 = _813.x;
                            float _816 = _813.y;
                            float _819 = _813.z;
                            float _821 = ((0.300000011920928955078125 * _814) + (0.589999973773956298828125 * _816)) + (0.10999999940395355224609375 * _819);
                            float _823 = precise::min(precise::min(_814, _816), _819);
                            float _825 = precise::max(precise::max(_814, _816), _819);
                            float3 _836;
                            if (_823 < 0.0)
                            {
                                float3 _829 = float3(_821);
                                _836 = _829 + (((_813 - _829) * _821) / float3(_821 - _823));
                            }
                            else
                            {
                                _836 = _813;
                            }
                            float3 _848;
                            if (_825 > 1.0)
                            {
                                float3 _840 = float3(_821);
                                _848 = _840 + (((_836 - _840) * (1.0 - _821)) / float3(_825 - _821));
                            }
                            else
                            {
                                _848 = _836;
                            }
                            _1040 = _848;
                            break;
                        }
                        if (_348 == 26u)
                        {
                            float _852 = _437.x;
                            float _853 = _437.y;
                            float _855 = _437.z;
                            float _864 = precise::min(precise::min(_447.x, _447.y), _447.z);
                            float _866 = precise::max(precise::max(_447.x, _447.y), _447.z);
                            float3 _877;
                            if (_866 > _864)
                            {
                                _877 = ((_447 - float3(_864)) * (precise::max(precise::max(_852, _853), _855) - precise::min(precise::min(_852, _853), _855))) / float3(_866 - _864);
                            }
                            else
                            {
                                _877 = float3(0.0);
                            }
                            float3 _893 = _877 + float3((((0.300000011920928955078125 * _447.x) + (0.589999973773956298828125 * _447.y)) + (0.10999999940395355224609375 * _447.z)) - (((0.300000011920928955078125 * _877.x) + (0.589999973773956298828125 * _877.y)) + (0.10999999940395355224609375 * _877.z)));
                            float _894 = _893.x;
                            float _896 = _893.y;
                            float _899 = _893.z;
                            float _901 = ((0.300000011920928955078125 * _894) + (0.589999973773956298828125 * _896)) + (0.10999999940395355224609375 * _899);
                            float _903 = precise::min(precise::min(_894, _896), _899);
                            float _905 = precise::max(precise::max(_894, _896), _899);
                            float3 _916;
                            if (_903 < 0.0)
                            {
                                float3 _909 = float3(_901);
                                _916 = _909 + (((_893 - _909) * _901) / float3(_901 - _903));
                            }
                            else
                            {
                                _916 = _893;
                            }
                            float3 _928;
                            if (_905 > 1.0)
                            {
                                float3 _920 = float3(_901);
                                _928 = _920 + (((_916 - _920) * (1.0 - _901)) / float3(_905 - _901));
                            }
                            else
                            {
                                _928 = _916;
                            }
                            _1040 = _928;
                            break;
                        }
                        if (_348 == 27u)
                        {
                            float3 _950 = _437 + float3((((0.300000011920928955078125 * _447.x) + (0.589999973773956298828125 * _447.y)) + (0.10999999940395355224609375 * _447.z)) - (((0.300000011920928955078125 * _437.x) + (0.589999973773956298828125 * _437.y)) + (0.10999999940395355224609375 * _437.z)));
                            float _951 = _950.x;
                            float _953 = _950.y;
                            float _956 = _950.z;
                            float _958 = ((0.300000011920928955078125 * _951) + (0.589999973773956298828125 * _953)) + (0.10999999940395355224609375 * _956);
                            float _960 = precise::min(precise::min(_951, _953), _956);
                            float _962 = precise::max(precise::max(_951, _953), _956);
                            float3 _973;
                            if (_960 < 0.0)
                            {
                                float3 _966 = float3(_958);
                                _973 = _966 + (((_950 - _966) * _958) / float3(_958 - _960));
                            }
                            else
                            {
                                _973 = _950;
                            }
                            float3 _985;
                            if (_962 > 1.0)
                            {
                                float3 _977 = float3(_958);
                                _985 = _977 + (((_973 - _977) * (1.0 - _958)) / float3(_962 - _958));
                            }
                            else
                            {
                                _985 = _973;
                            }
                            _1040 = _985;
                            break;
                        }
                        float3 _1004 = _447 + float3((((0.300000011920928955078125 * _437.x) + (0.589999973773956298828125 * _437.y)) + (0.10999999940395355224609375 * _437.z)) - (((0.300000011920928955078125 * _447.x) + (0.589999973773956298828125 * _447.y)) + (0.10999999940395355224609375 * _447.z)));
                        float _1005 = _1004.x;
                        float _1007 = _1004.y;
                        float _1010 = _1004.z;
                        float _1012 = ((0.300000011920928955078125 * _1005) + (0.589999973773956298828125 * _1007)) + (0.10999999940395355224609375 * _1010);
                        float _1014 = precise::min(precise::min(_1005, _1007), _1010);
                        float _1016 = precise::max(precise::max(_1005, _1007), _1010);
                        float3 _1027;
                        if (_1014 < 0.0)
                        {
                            float3 _1020 = float3(_1012);
                            _1027 = _1020 + (((_1004 - _1020) * _1012) / float3(_1012 - _1014));
                        }
                        else
                        {
                            _1027 = _1004;
                        }
                        float3 _1039;
                        if (_1016 > 1.0)
                        {
                            float3 _1031 = float3(_1012);
                            _1039 = _1031 + (((_1027 - _1031) * (1.0 - _1012)) / float3(_1016 - _1012));
                        }
                        else
                        {
                            _1039 = _1027;
                        }
                        _1040 = _1039;
                        break;
                    } while(false);
                    _1049 = float4(((_437 * (1.0 - _438)) + (fast::clamp(_1040, float3(0.0), float3(1.0)) * _438)) * _430, _430);
                    break;
                } while(false);
                _1050 = _1049;
                break;
            }
            _1050 = _346;
            break;
        } while(false);
        _1051 = _1050;
        break;
    } while(false);
    out.out_var_SV_Target = _1051;
    return out;
}

