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
    float4 _1047;
    do
    {
        float _204;
        float2 _205;
        if (any(in.in_var_TRANSFORM0 != float4(0.0)))
        {
            float2 _116 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_TRANSFORM1;
            float _118 = _116.x;
            float _121 = _116.y;
            float _132 = (in.in_var_TRANSFORM0.x * in.in_var_TRANSFORM0.w) - (in.in_var_TRANSFORM0.y * in.in_var_TRANSFORM0.z);
            float2 _139 = in.in_var_RECT.zw * 0.5;
            float2 _141 = (in.in_var_LOGICAL - in.in_var_RECT.xy) - _139;
            float2 _143 = abs(_141) - _139;
            float _144 = _143.x;
            float _145 = _143.y;
            bool _158;
            if (_144 > 0.0)
            {
                _158 = _145 > 0.0;
            }
            else
            {
                _158 = false;
            }
            float2 _162;
            if (_158)
            {
                _162 = fast::normalize(_143);
            }
            else
            {
                _162 = select(float2(0.0, 1.0), float2(1.0, 0.0), bool2(_144 > _145));
            }
            float _171;
            if (_141.x < 0.0)
            {
                _171 = -_162.x;
            }
            else
            {
                _171 = _162.x;
            }
            float _180;
            if (_141.y < 0.0)
            {
                _180 = -_162.y;
            }
            else
            {
                _180 = _162.y;
            }
            float2 _181 = float2(_171, _180);
            _204 = fast::clamp(0.5 - ((precise::min(precise::max(_144, _145), 0.0) + length(precise::max(_143, float2(0.0)))) / precise::max(length(float2(dot(_181, float2(in.in_var_TRANSFORM0.w, -in.in_var_TRANSFORM0.y)), dot(_181, float2(-in.in_var_TRANSFORM0.z, in.in_var_TRANSFORM0.x)))) / (abs(_132) * GowakuFrame.gowakuScale), 9.9999999600419720025001879548654e-13)), 0.0, 1.0);
            _205 = fast::clamp((float2((in.in_var_TRANSFORM0.w * _118) - (in.in_var_TRANSFORM0.z * _121), (in.in_var_TRANSFORM0.x * _121) - (in.in_var_TRANSFORM0.y * _118)) / float2(_132)) - in.in_var_RECT.xy, float2(0.0), in.in_var_RECT.zw);
        }
        else
        {
            _204 = 1.0;
            _205 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_RECT.xy;
        }
        float2 _219;
        float2 _207 = _205 / in.in_var_RECT.zw;
        float2 _212 = mix(in.in_var_COLOR1.xy, in.in_var_COLOR1.zw, _207) - float2(0.5);
        float2 _213 = floor(_212);
        float2 _214 = _212 - _213;
        int2 _215 = int2(_213);
        float4 _237;
        do
        {
            float2 _218 = float2(_215);
            _219 = in.in_var_COLOR2.xy;
            bool _228;
            if (!any(_218 < _219))
            {
                _228 = any(_218 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _228 = true;
            }
            if (_228)
            {
                _237 = float4(0.0);
                break;
            }
            _237 = gowakuTex0.read(uint2(int3(_215, 0).xy), 0);
            break;
        } while(false);
        int2 _238 = _215 + int2(1, 0);
        float4 _259;
        do
        {
            float2 _241 = float2(_238);
            bool _250;
            if (!any(_241 < _219))
            {
                _250 = any(_241 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _250 = true;
            }
            if (_250)
            {
                _259 = float4(0.0);
                break;
            }
            _259 = gowakuTex0.read(uint2(int3(_238, 0).xy), 0);
            break;
        } while(false);
        float4 _261 = float4(_214.x);
        int2 _263 = _215 + int2(0, 1);
        float4 _284;
        do
        {
            float2 _266 = float2(_263);
            bool _275;
            if (!any(_266 < _219))
            {
                _275 = any(_266 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _275 = true;
            }
            if (_275)
            {
                _284 = float4(0.0);
                break;
            }
            _284 = gowakuTex0.read(uint2(int3(_263, 0).xy), 0);
            break;
        } while(false);
        int2 _285 = _215 + int2(1);
        float4 _306;
        do
        {
            float2 _288 = float2(_285);
            bool _297;
            if (!any(_288 < _219))
            {
                _297 = any(_288 >= in.in_var_COLOR2.zw);
            }
            else
            {
                _297 = true;
            }
            if (_297)
            {
                _306 = float4(0.0);
                break;
            }
            _306 = gowakuTex0.read(uint2(int3(_285, 0).xy), 0);
            break;
        } while(false);
        float4 _310 = mix(mix(_237, _259, _261), mix(_284, _306, _261), float4(_214.y));
        float _329 = _310.w;
        float4 _342 = float4(mix(_310.xyz, in.in_var_STOPS.xyz * _329, float3(fast::clamp(1.0 - (abs((_207.x + ((_207.y - 0.5) * in.in_var_RADII.z)) - ((fract(in.in_var_PARAMS.x * in.in_var_RADII.x) * 1.60000002384185791015625) - 0.300000011920928955078125)) / in.in_var_RADII.y), 0.0, 1.0) * in.in_var_STOPS.w)), _329) * (in.in_var_COLOR0.w * _204);
        uint _344 = in.in_var_MODE.x >> 8u;
        if (_344 == 0u)
        {
            _1047 = _342;
            break;
        }
        float4 _1046;
        do
        {
            if (_344 == 1u)
            {
                _1046 = float4(0.0, 0.0, 0.0, _204);
                break;
            }
            bool _359;
            if (!(_344 == 6u))
            {
                _359 = _344 == 14u;
            }
            else
            {
                _359 = true;
            }
            if (_359)
            {
                _1046 = float4(_204) - _342;
                break;
            }
            bool _364 = _344 == 2u;
            bool _369;
            if (!_364)
            {
                _369 = _344 == 5u;
            }
            else
            {
                _369 = true;
            }
            bool _374;
            if (!_369)
            {
                _374 = _344 == 7u;
            }
            else
            {
                _374 = true;
            }
            bool _379;
            if (!_374)
            {
                _379 = _344 == 10u;
            }
            else
            {
                _379 = true;
            }
            bool _384;
            if (!_379)
            {
                _384 = _344 >= 15u;
            }
            else
            {
                _384 = true;
            }
            if (_384)
            {
                float4 _1045;
                do
                {
                    float4 _397 = gowakuBackdrop.read(uint2(int3(int2(floor(gl_FragCoord.xy)), 0).xy), 0);
                    float4 _399 = _397 * (1.0 - _204);
                    if (_364)
                    {
                        _1045 = _342 + _399;
                        break;
                    }
                    if (_344 == 5u)
                    {
                        _1045 = (_342 * _397.w) + _399;
                        break;
                    }
                    if (_344 == 7u)
                    {
                        _1045 = (_342 * (1.0 - _397.w)) + _399;
                        break;
                    }
                    if (_344 == 10u)
                    {
                        _1045 = ((_342 * (1.0 - _397.w)) + (_397 * _342.w)) + _399;
                        break;
                    }
                    float _426 = _342.w;
                    if (_426 <= 0.0)
                    {
                        _1045 = float4(0.0);
                        break;
                    }
                    float3 _433 = fast::clamp(_342.xyz / float3(_426), float3(0.0), float3(1.0));
                    float _434 = _397.w;
                    float3 _443;
                    if (_434 > 0.0)
                    {
                        _443 = fast::clamp(_397.xyz / float3(_434), float3(0.0), float3(1.0));
                    }
                    else
                    {
                        _443 = float3(0.0);
                    }
                    float3 _1036;
                    do
                    {
                        if (_344 == 15u)
                        {
                            _1036 = _443 * _433;
                            break;
                        }
                        if (_344 == 16u)
                        {
                            float _455 = _433.x;
                            float _468;
                            if (_443.x <= 0.5)
                            {
                                _468 = (_455 * 2.0) * _443.x;
                            }
                            else
                            {
                                float _464 = (2.0 * _443.x) - 1.0;
                                _468 = (_455 + _464) - (_455 * _464);
                            }
                            float _469 = _433.y;
                            float _482;
                            if (_443.y <= 0.5)
                            {
                                _482 = (_469 * 2.0) * _443.y;
                            }
                            else
                            {
                                float _478 = (2.0 * _443.y) - 1.0;
                                _482 = (_469 + _478) - (_469 * _478);
                            }
                            float _483 = _433.z;
                            float _496;
                            if (_443.z <= 0.5)
                            {
                                _496 = (_483 * 2.0) * _443.z;
                            }
                            else
                            {
                                float _492 = (2.0 * _443.z) - 1.0;
                                _496 = (_483 + _492) - (_483 * _492);
                            }
                            _1036 = float3(_468, _482, _496);
                            break;
                        }
                        if (_344 == 17u)
                        {
                            _1036 = precise::min(_443, _433);
                            break;
                        }
                        if (_344 == 18u)
                        {
                            _1036 = precise::max(_443, _433);
                            break;
                        }
                        if (_344 == 19u)
                        {
                            float _510 = _433.x;
                            float _524;
                            do
                            {
                                if (_443.x <= 0.0)
                                {
                                    _524 = 0.0;
                                    break;
                                }
                                float _523;
                                if (_510 >= 1.0)
                                {
                                    _523 = 1.0;
                                }
                                else
                                {
                                    _523 = precise::min(1.0, _443.x / (1.0 - _510));
                                }
                                _524 = _523;
                                break;
                            } while(false);
                            float _526 = _433.y;
                            float _540;
                            do
                            {
                                if (_443.y <= 0.0)
                                {
                                    _540 = 0.0;
                                    break;
                                }
                                float _539;
                                if (_526 >= 1.0)
                                {
                                    _539 = 1.0;
                                }
                                else
                                {
                                    _539 = precise::min(1.0, _443.y / (1.0 - _526));
                                }
                                _540 = _539;
                                break;
                            } while(false);
                            float _542 = _433.z;
                            float _556;
                            do
                            {
                                if (_443.z <= 0.0)
                                {
                                    _556 = 0.0;
                                    break;
                                }
                                float _555;
                                if (_542 >= 1.0)
                                {
                                    _555 = 1.0;
                                }
                                else
                                {
                                    _555 = precise::min(1.0, _443.z / (1.0 - _542));
                                }
                                _556 = _555;
                                break;
                            } while(false);
                            _1036 = float3(_524, _540, _556);
                            break;
                        }
                        if (_344 == 20u)
                        {
                            float _562 = _433.x;
                            float _577;
                            do
                            {
                                if (_443.x >= 1.0)
                                {
                                    _577 = 1.0;
                                    break;
                                }
                                float _576;
                                if (_562 <= 0.0)
                                {
                                    _576 = 0.0;
                                }
                                else
                                {
                                    _576 = 1.0 - precise::min(1.0, (1.0 - _443.x) / _562);
                                }
                                _577 = _576;
                                break;
                            } while(false);
                            float _579 = _433.y;
                            float _594;
                            do
                            {
                                if (_443.y >= 1.0)
                                {
                                    _594 = 1.0;
                                    break;
                                }
                                float _593;
                                if (_579 <= 0.0)
                                {
                                    _593 = 0.0;
                                }
                                else
                                {
                                    _593 = 1.0 - precise::min(1.0, (1.0 - _443.y) / _579);
                                }
                                _594 = _593;
                                break;
                            } while(false);
                            float _596 = _433.z;
                            float _611;
                            do
                            {
                                if (_443.z >= 1.0)
                                {
                                    _611 = 1.0;
                                    break;
                                }
                                float _610;
                                if (_596 <= 0.0)
                                {
                                    _610 = 0.0;
                                }
                                else
                                {
                                    _610 = 1.0 - precise::min(1.0, (1.0 - _443.z) / _596);
                                }
                                _611 = _610;
                                break;
                            } while(false);
                            _1036 = float3(_577, _594, _611);
                            break;
                        }
                        if (_344 == 21u)
                        {
                            float _617 = _433.x;
                            float _629;
                            if (_617 <= 0.5)
                            {
                                _629 = (_443.x * 2.0) * _617;
                            }
                            else
                            {
                                float _625 = (2.0 * _617) - 1.0;
                                _629 = (_443.x + _625) - (_443.x * _625);
                            }
                            float _631 = _433.y;
                            float _643;
                            if (_631 <= 0.5)
                            {
                                _643 = (_443.y * 2.0) * _631;
                            }
                            else
                            {
                                float _639 = (2.0 * _631) - 1.0;
                                _643 = (_443.y + _639) - (_443.y * _639);
                            }
                            float _645 = _433.z;
                            float _657;
                            if (_645 <= 0.5)
                            {
                                _657 = (_443.z * 2.0) * _645;
                            }
                            else
                            {
                                float _653 = (2.0 * _645) - 1.0;
                                _657 = (_443.z + _653) - (_443.z * _653);
                            }
                            _1036 = float3(_629, _643, _657);
                            break;
                        }
                        if (_344 == 22u)
                        {
                            float _663 = _433.x;
                            float _691;
                            do
                            {
                                if (_663 <= 0.5)
                                {
                                    _691 = _443.x - (((1.0 - (2.0 * _663)) * _443.x) * (1.0 - _443.x));
                                    break;
                                }
                                float _685;
                                if (_443.x <= 0.25)
                                {
                                    _685 = ((((16.0 * _443.x) - 12.0) * _443.x) + 4.0) * _443.x;
                                }
                                else
                                {
                                    _685 = sqrt(_443.x);
                                }
                                _691 = _443.x + (((2.0 * _663) - 1.0) * (_685 - _443.x));
                                break;
                            } while(false);
                            float _693 = _433.y;
                            float _721;
                            do
                            {
                                if (_693 <= 0.5)
                                {
                                    _721 = _443.y - (((1.0 - (2.0 * _693)) * _443.y) * (1.0 - _443.y));
                                    break;
                                }
                                float _715;
                                if (_443.y <= 0.25)
                                {
                                    _715 = ((((16.0 * _443.y) - 12.0) * _443.y) + 4.0) * _443.y;
                                }
                                else
                                {
                                    _715 = sqrt(_443.y);
                                }
                                _721 = _443.y + (((2.0 * _693) - 1.0) * (_715 - _443.y));
                                break;
                            } while(false);
                            float _723 = _433.z;
                            float _751;
                            do
                            {
                                if (_723 <= 0.5)
                                {
                                    _751 = _443.z - (((1.0 - (2.0 * _723)) * _443.z) * (1.0 - _443.z));
                                    break;
                                }
                                float _745;
                                if (_443.z <= 0.25)
                                {
                                    _745 = ((((16.0 * _443.z) - 12.0) * _443.z) + 4.0) * _443.z;
                                }
                                else
                                {
                                    _745 = sqrt(_443.z);
                                }
                                _751 = _443.z + (((2.0 * _723) - 1.0) * (_745 - _443.z));
                                break;
                            } while(false);
                            _1036 = float3(_691, _721, _751);
                            break;
                        }
                        if (_344 == 23u)
                        {
                            _1036 = abs(_443 - _433);
                            break;
                        }
                        if (_344 == 24u)
                        {
                            _1036 = (_443 + _433) - ((_443 * 2.0) * _433);
                            break;
                        }
                        if (_344 == 25u)
                        {
                            float _776 = _433.x;
                            float _777 = _433.y;
                            float _779 = _433.z;
                            float _780 = precise::min(precise::min(_776, _777), _779);
                            float _782 = precise::max(precise::max(_776, _777), _779);
                            float3 _793;
                            if (_782 > _780)
                            {
                                _793 = ((_433 - float3(_780)) * (precise::max(precise::max(_443.x, _443.y), _443.z) - precise::min(precise::min(_443.x, _443.y), _443.z))) / float3(_782 - _780);
                            }
                            else
                            {
                                _793 = float3(0.0);
                            }
                            float3 _809 = _793 + float3((((0.300000011920928955078125 * _443.x) + (0.589999973773956298828125 * _443.y)) + (0.10999999940395355224609375 * _443.z)) - (((0.300000011920928955078125 * _793.x) + (0.589999973773956298828125 * _793.y)) + (0.10999999940395355224609375 * _793.z)));
                            float _810 = _809.x;
                            float _812 = _809.y;
                            float _815 = _809.z;
                            float _817 = ((0.300000011920928955078125 * _810) + (0.589999973773956298828125 * _812)) + (0.10999999940395355224609375 * _815);
                            float _819 = precise::min(precise::min(_810, _812), _815);
                            float _821 = precise::max(precise::max(_810, _812), _815);
                            float3 _832;
                            if (_819 < 0.0)
                            {
                                float3 _825 = float3(_817);
                                _832 = _825 + (((_809 - _825) * _817) / float3(_817 - _819));
                            }
                            else
                            {
                                _832 = _809;
                            }
                            float3 _844;
                            if (_821 > 1.0)
                            {
                                float3 _836 = float3(_817);
                                _844 = _836 + (((_832 - _836) * (1.0 - _817)) / float3(_821 - _817));
                            }
                            else
                            {
                                _844 = _832;
                            }
                            _1036 = _844;
                            break;
                        }
                        if (_344 == 26u)
                        {
                            float _848 = _433.x;
                            float _849 = _433.y;
                            float _851 = _433.z;
                            float _860 = precise::min(precise::min(_443.x, _443.y), _443.z);
                            float _862 = precise::max(precise::max(_443.x, _443.y), _443.z);
                            float3 _873;
                            if (_862 > _860)
                            {
                                _873 = ((_443 - float3(_860)) * (precise::max(precise::max(_848, _849), _851) - precise::min(precise::min(_848, _849), _851))) / float3(_862 - _860);
                            }
                            else
                            {
                                _873 = float3(0.0);
                            }
                            float3 _889 = _873 + float3((((0.300000011920928955078125 * _443.x) + (0.589999973773956298828125 * _443.y)) + (0.10999999940395355224609375 * _443.z)) - (((0.300000011920928955078125 * _873.x) + (0.589999973773956298828125 * _873.y)) + (0.10999999940395355224609375 * _873.z)));
                            float _890 = _889.x;
                            float _892 = _889.y;
                            float _895 = _889.z;
                            float _897 = ((0.300000011920928955078125 * _890) + (0.589999973773956298828125 * _892)) + (0.10999999940395355224609375 * _895);
                            float _899 = precise::min(precise::min(_890, _892), _895);
                            float _901 = precise::max(precise::max(_890, _892), _895);
                            float3 _912;
                            if (_899 < 0.0)
                            {
                                float3 _905 = float3(_897);
                                _912 = _905 + (((_889 - _905) * _897) / float3(_897 - _899));
                            }
                            else
                            {
                                _912 = _889;
                            }
                            float3 _924;
                            if (_901 > 1.0)
                            {
                                float3 _916 = float3(_897);
                                _924 = _916 + (((_912 - _916) * (1.0 - _897)) / float3(_901 - _897));
                            }
                            else
                            {
                                _924 = _912;
                            }
                            _1036 = _924;
                            break;
                        }
                        if (_344 == 27u)
                        {
                            float3 _946 = _433 + float3((((0.300000011920928955078125 * _443.x) + (0.589999973773956298828125 * _443.y)) + (0.10999999940395355224609375 * _443.z)) - (((0.300000011920928955078125 * _433.x) + (0.589999973773956298828125 * _433.y)) + (0.10999999940395355224609375 * _433.z)));
                            float _947 = _946.x;
                            float _949 = _946.y;
                            float _952 = _946.z;
                            float _954 = ((0.300000011920928955078125 * _947) + (0.589999973773956298828125 * _949)) + (0.10999999940395355224609375 * _952);
                            float _956 = precise::min(precise::min(_947, _949), _952);
                            float _958 = precise::max(precise::max(_947, _949), _952);
                            float3 _969;
                            if (_956 < 0.0)
                            {
                                float3 _962 = float3(_954);
                                _969 = _962 + (((_946 - _962) * _954) / float3(_954 - _956));
                            }
                            else
                            {
                                _969 = _946;
                            }
                            float3 _981;
                            if (_958 > 1.0)
                            {
                                float3 _973 = float3(_954);
                                _981 = _973 + (((_969 - _973) * (1.0 - _954)) / float3(_958 - _954));
                            }
                            else
                            {
                                _981 = _969;
                            }
                            _1036 = _981;
                            break;
                        }
                        float3 _1000 = _443 + float3((((0.300000011920928955078125 * _433.x) + (0.589999973773956298828125 * _433.y)) + (0.10999999940395355224609375 * _433.z)) - (((0.300000011920928955078125 * _443.x) + (0.589999973773956298828125 * _443.y)) + (0.10999999940395355224609375 * _443.z)));
                        float _1001 = _1000.x;
                        float _1003 = _1000.y;
                        float _1006 = _1000.z;
                        float _1008 = ((0.300000011920928955078125 * _1001) + (0.589999973773956298828125 * _1003)) + (0.10999999940395355224609375 * _1006);
                        float _1010 = precise::min(precise::min(_1001, _1003), _1006);
                        float _1012 = precise::max(precise::max(_1001, _1003), _1006);
                        float3 _1023;
                        if (_1010 < 0.0)
                        {
                            float3 _1016 = float3(_1008);
                            _1023 = _1016 + (((_1000 - _1016) * _1008) / float3(_1008 - _1010));
                        }
                        else
                        {
                            _1023 = _1000;
                        }
                        float3 _1035;
                        if (_1012 > 1.0)
                        {
                            float3 _1027 = float3(_1008);
                            _1035 = _1027 + (((_1023 - _1027) * (1.0 - _1008)) / float3(_1012 - _1008));
                        }
                        else
                        {
                            _1035 = _1023;
                        }
                        _1036 = _1035;
                        break;
                    } while(false);
                    _1045 = float4(((_433 * (1.0 - _434)) + (fast::clamp(_1036, float3(0.0), float3(1.0)) * _434)) * _426, _426);
                    break;
                } while(false);
                _1046 = _1045;
                break;
            }
            _1046 = _342;
            break;
        } while(false);
        _1047 = _1046;
        break;
    } while(false);
    out.out_var_SV_Target = _1047;
    return out;
}

