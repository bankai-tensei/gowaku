#include <metal_stdlib>
#include <simd/simd.h>

using namespace metal;

struct type_GowakuFrame
{
    float2 gowakuViewport;
    float gowakuScale;
    float gowakuPadding;
};

constant float _124 = {};

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
    float4 in_var_GRADIENT [[user(locn6), flat]];
    float2 in_var_PARAMS [[user(locn7), flat]];
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
    float4 _3815;
    do
    {
        float _244;
        float2 _245;
        if (any(in.in_var_TRANSFORM0 != float4(0.0)))
        {
            float2 _156 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_TRANSFORM1;
            float _158 = _156.x;
            float _161 = _156.y;
            float _172 = (in.in_var_TRANSFORM0.x * in.in_var_TRANSFORM0.w) - (in.in_var_TRANSFORM0.y * in.in_var_TRANSFORM0.z);
            float2 _179 = in.in_var_RECT.zw * 0.5;
            float2 _181 = (in.in_var_LOGICAL - in.in_var_RECT.xy) - _179;
            float2 _183 = abs(_181) - _179;
            float _184 = _183.x;
            float _185 = _183.y;
            bool _198;
            if (_184 > 0.0)
            {
                _198 = _185 > 0.0;
            }
            else
            {
                _198 = false;
            }
            float2 _202;
            if (_198)
            {
                _202 = fast::normalize(_183);
            }
            else
            {
                _202 = select(float2(0.0, 1.0), float2(1.0, 0.0), bool2(_184 > _185));
            }
            float _211;
            if (_181.x < 0.0)
            {
                _211 = -_202.x;
            }
            else
            {
                _211 = _202.x;
            }
            float _220;
            if (_181.y < 0.0)
            {
                _220 = -_202.y;
            }
            else
            {
                _220 = _202.y;
            }
            float2 _221 = float2(_211, _220);
            _244 = fast::clamp(0.5 - ((precise::min(precise::max(_184, _185), 0.0) + length(precise::max(_183, float2(0.0)))) / precise::max(length(float2(dot(_221, float2(in.in_var_TRANSFORM0.w, -in.in_var_TRANSFORM0.y)), dot(_221, float2(-in.in_var_TRANSFORM0.z, in.in_var_TRANSFORM0.x)))) / (abs(_172) * GowakuFrame.gowakuScale), 9.9999999600419720025001879548654e-13)), 0.0, 1.0);
            _245 = fast::clamp((float2((in.in_var_TRANSFORM0.w * _158) - (in.in_var_TRANSFORM0.z * _161), (in.in_var_TRANSFORM0.x * _161) - (in.in_var_TRANSFORM0.y * _158)) / float2(_172)) - in.in_var_RECT.xy, float2(0.0), in.in_var_RECT.zw);
        }
        else
        {
            _244 = 1.0;
            _245 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_RECT.xy;
        }
        uint _249 = uint(in.in_var_PARAMS.x);
        int2 _251 = int2(in.in_var_GRADIENT.xy);
        int2 _253 = int2(floor(_245 / float2((in.in_var_PARAMS.y > 0.0) ? in.in_var_PARAMS.y : 1.0)));
        uint _254 = _249 & 255u;
        float4 _3053;
        do
        {
            if (_254 == 0u)
            {
                bool _269;
                float2 _261 = floor(in.in_var_RADII.xy);
                float2 _262 = in.in_var_RADII.xy - _261;
                int2 _264 = _253 - int2(_261);
                float4 _349;
                do
                {
                    _269 = !((_249 & 2048u) != 0u);
                    bool _280;
                    if (_269)
                    {
                        bool _279;
                        if (!any(_264 < int2(0)))
                        {
                            _279 = any(_264 >= _251);
                        }
                        else
                        {
                            _279 = true;
                        }
                        _280 = _279;
                    }
                    else
                    {
                        _280 = true;
                    }
                    if (_280)
                    {
                        _349 = float4(0.0);
                        break;
                    }
                    float4 _288 = gowakuTex0.read(uint2(int3(_264, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _349 = float4(0.0, 0.0, 0.0, _288.w);
                        break;
                    }
                    float4 _348;
                    do
                    {
                        bool _304;
                        if (!((_249 & 256u) == 0u))
                        {
                            _304 = _288.w <= 0.0;
                        }
                        else
                        {
                            _304 = true;
                        }
                        if (_304)
                        {
                            _348 = _288;
                            break;
                        }
                        float _308 = _288.w;
                        float3 _311 = fast::clamp(_288.xyz / float3(_308), float3(0.0), float3(1.0));
                        float _312 = _311.x;
                        float _321;
                        if (_312 <= 0.040449999272823333740234375)
                        {
                            _321 = _312 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _321 = powr((_312 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _322 = _311.y;
                        float _331;
                        if (_322 <= 0.040449999272823333740234375)
                        {
                            _331 = _322 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _331 = powr((_322 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _332 = _311.z;
                        float _341;
                        if (_332 <= 0.040449999272823333740234375)
                        {
                            _341 = _332 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _341 = powr((_332 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _348 = float4(float3(_321, _331, _341) * _308, _308);
                        break;
                    } while(false);
                    _349 = _348;
                    break;
                } while(false);
                int2 _350 = _264 - int2(1, 0);
                float4 _432;
                do
                {
                    bool _363;
                    if (_269)
                    {
                        bool _362;
                        if (!any(_350 < int2(0)))
                        {
                            _362 = any(_350 >= _251);
                        }
                        else
                        {
                            _362 = true;
                        }
                        _363 = _362;
                    }
                    else
                    {
                        _363 = true;
                    }
                    if (_363)
                    {
                        _432 = float4(0.0);
                        break;
                    }
                    float4 _371 = gowakuTex0.read(uint2(int3(_350, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _432 = float4(0.0, 0.0, 0.0, _371.w);
                        break;
                    }
                    float4 _431;
                    do
                    {
                        bool _387;
                        if (!((_249 & 256u) == 0u))
                        {
                            _387 = _371.w <= 0.0;
                        }
                        else
                        {
                            _387 = true;
                        }
                        if (_387)
                        {
                            _431 = _371;
                            break;
                        }
                        float _391 = _371.w;
                        float3 _394 = fast::clamp(_371.xyz / float3(_391), float3(0.0), float3(1.0));
                        float _395 = _394.x;
                        float _404;
                        if (_395 <= 0.040449999272823333740234375)
                        {
                            _404 = _395 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _404 = powr((_395 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _405 = _394.y;
                        float _414;
                        if (_405 <= 0.040449999272823333740234375)
                        {
                            _414 = _405 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _414 = powr((_405 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _415 = _394.z;
                        float _424;
                        if (_415 <= 0.040449999272823333740234375)
                        {
                            _424 = _415 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _424 = powr((_415 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _431 = float4(float3(_404, _414, _424) * _391, _391);
                        break;
                    } while(false);
                    _432 = _431;
                    break;
                } while(false);
                float4 _434 = float4(_262.x);
                int2 _436 = _264 - int2(0, 1);
                float4 _518;
                do
                {
                    bool _449;
                    if (_269)
                    {
                        bool _448;
                        if (!any(_436 < int2(0)))
                        {
                            _448 = any(_436 >= _251);
                        }
                        else
                        {
                            _448 = true;
                        }
                        _449 = _448;
                    }
                    else
                    {
                        _449 = true;
                    }
                    if (_449)
                    {
                        _518 = float4(0.0);
                        break;
                    }
                    float4 _457 = gowakuTex0.read(uint2(int3(_436, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _518 = float4(0.0, 0.0, 0.0, _457.w);
                        break;
                    }
                    float4 _517;
                    do
                    {
                        bool _473;
                        if (!((_249 & 256u) == 0u))
                        {
                            _473 = _457.w <= 0.0;
                        }
                        else
                        {
                            _473 = true;
                        }
                        if (_473)
                        {
                            _517 = _457;
                            break;
                        }
                        float _477 = _457.w;
                        float3 _480 = fast::clamp(_457.xyz / float3(_477), float3(0.0), float3(1.0));
                        float _481 = _480.x;
                        float _490;
                        if (_481 <= 0.040449999272823333740234375)
                        {
                            _490 = _481 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _490 = powr((_481 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _491 = _480.y;
                        float _500;
                        if (_491 <= 0.040449999272823333740234375)
                        {
                            _500 = _491 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _500 = powr((_491 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _501 = _480.z;
                        float _510;
                        if (_501 <= 0.040449999272823333740234375)
                        {
                            _510 = _501 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _510 = powr((_501 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _517 = float4(float3(_490, _500, _510) * _477, _477);
                        break;
                    } while(false);
                    _518 = _517;
                    break;
                } while(false);
                int2 _519 = _264 - int2(1);
                float4 _601;
                do
                {
                    bool _532;
                    if (_269)
                    {
                        bool _531;
                        if (!any(_519 < int2(0)))
                        {
                            _531 = any(_519 >= _251);
                        }
                        else
                        {
                            _531 = true;
                        }
                        _532 = _531;
                    }
                    else
                    {
                        _532 = true;
                    }
                    if (_532)
                    {
                        _601 = float4(0.0);
                        break;
                    }
                    float4 _540 = gowakuTex0.read(uint2(int3(_519, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _601 = float4(0.0, 0.0, 0.0, _540.w);
                        break;
                    }
                    float4 _600;
                    do
                    {
                        bool _556;
                        if (!((_249 & 256u) == 0u))
                        {
                            _556 = _540.w <= 0.0;
                        }
                        else
                        {
                            _556 = true;
                        }
                        if (_556)
                        {
                            _600 = _540;
                            break;
                        }
                        float _560 = _540.w;
                        float3 _563 = fast::clamp(_540.xyz / float3(_560), float3(0.0), float3(1.0));
                        float _564 = _563.x;
                        float _573;
                        if (_564 <= 0.040449999272823333740234375)
                        {
                            _573 = _564 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _573 = powr((_564 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _574 = _563.y;
                        float _583;
                        if (_574 <= 0.040449999272823333740234375)
                        {
                            _583 = _574 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _583 = powr((_574 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _584 = _563.z;
                        float _593;
                        if (_584 <= 0.040449999272823333740234375)
                        {
                            _593 = _584 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _593 = powr((_584 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _600 = float4(float3(_573, _583, _593) * _560, _560);
                        break;
                    } while(false);
                    _601 = _600;
                    break;
                } while(false);
                _3053 = mix(mix(_349, _432, _434), mix(_518, _601, _434), float4(_262.y));
                break;
            }
            if (_254 == 1u)
            {
                float4 _662;
                do
                {
                    bool _618;
                    if (!((_249 & 256u) == 0u))
                    {
                        _618 = in.in_var_RADII.w <= 0.0;
                    }
                    else
                    {
                        _618 = true;
                    }
                    if (_618)
                    {
                        _662 = in.in_var_RADII;
                        break;
                    }
                    float3 _625 = fast::clamp(in.in_var_RADII.xyz / float3(in.in_var_RADII.w), float3(0.0), float3(1.0));
                    float _626 = _625.x;
                    float _635;
                    if (_626 <= 0.040449999272823333740234375)
                    {
                        _635 = _626 * 0.077399380505084991455078125;
                    }
                    else
                    {
                        _635 = powr((_626 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                    }
                    float _636 = _625.y;
                    float _645;
                    if (_636 <= 0.040449999272823333740234375)
                    {
                        _645 = _636 * 0.077399380505084991455078125;
                    }
                    else
                    {
                        _645 = powr((_636 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                    }
                    float _646 = _625.z;
                    float _655;
                    if (_646 <= 0.040449999272823333740234375)
                    {
                        _655 = _646 * 0.077399380505084991455078125;
                    }
                    else
                    {
                        _655 = powr((_646 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                    }
                    _662 = float4(float3(_635, _645, _655) * in.in_var_RADII.w, in.in_var_RADII.w);
                    break;
                } while(false);
                _3053 = _662;
                break;
            }
            if (_254 == 2u)
            {
                float4 _750;
                do
                {
                    bool _681;
                    if (!((_249 & 2048u) != 0u))
                    {
                        bool _680;
                        if (!any(_253 < int2(0)))
                        {
                            _680 = any(_253 >= _251);
                        }
                        else
                        {
                            _680 = true;
                        }
                        _681 = _680;
                    }
                    else
                    {
                        _681 = true;
                    }
                    if (_681)
                    {
                        _750 = float4(0.0);
                        break;
                    }
                    float4 _689 = gowakuTex0.read(uint2(int3(_253, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _750 = float4(0.0, 0.0, 0.0, _689.w);
                        break;
                    }
                    float4 _749;
                    do
                    {
                        bool _705;
                        if (!((_249 & 256u) == 0u))
                        {
                            _705 = _689.w <= 0.0;
                        }
                        else
                        {
                            _705 = true;
                        }
                        if (_705)
                        {
                            _749 = _689;
                            break;
                        }
                        float _709 = _689.w;
                        float3 _712 = fast::clamp(_689.xyz / float3(_709), float3(0.0), float3(1.0));
                        float _713 = _712.x;
                        float _722;
                        if (_713 <= 0.040449999272823333740234375)
                        {
                            _722 = _713 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _722 = powr((_713 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _723 = _712.y;
                        float _732;
                        if (_723 <= 0.040449999272823333740234375)
                        {
                            _732 = _723 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _732 = powr((_723 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _733 = _712.z;
                        float _742;
                        if (_733 <= 0.040449999272823333740234375)
                        {
                            _742 = _733 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _742 = powr((_733 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _749 = float4(float3(_722, _732, _742) * _709, _709);
                        break;
                    } while(false);
                    _750 = _749;
                    break;
                } while(false);
                float4 _763;
                if (_750.w > 0.0)
                {
                    _763 = float4(_750.xyz / float3(_750.w), _750.w);
                }
                else
                {
                    _763 = float4(0.0);
                }
                float4 _770 = fast::clamp(float4(dot(in.in_var_COLOR1, _763), dot(in.in_var_COLOR2, _763), dot(in.in_var_COLOR3, _763), dot(in.in_var_COLOR4, _763)) + in.in_var_RADII, float4(0.0), float4(1.0));
                float _772 = _770.w;
                _3053 = float4(_770.xyz * _772, _772);
                break;
            }
            if (_254 == 3u)
            {
                float4 _865;
                do
                {
                    bool _796;
                    if (!((_249 & 2048u) != 0u))
                    {
                        bool _795;
                        if (!any(_253 < int2(0)))
                        {
                            _795 = any(_253 >= _251);
                        }
                        else
                        {
                            _795 = true;
                        }
                        _796 = _795;
                    }
                    else
                    {
                        _796 = true;
                    }
                    if (_796)
                    {
                        _865 = float4(0.0);
                        break;
                    }
                    float4 _804 = gowakuTex0.read(uint2(int3(_253, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _865 = float4(0.0, 0.0, 0.0, _804.w);
                        break;
                    }
                    float4 _864;
                    do
                    {
                        bool _820;
                        if (!((_249 & 256u) == 0u))
                        {
                            _820 = _804.w <= 0.0;
                        }
                        else
                        {
                            _820 = true;
                        }
                        if (_820)
                        {
                            _864 = _804;
                            break;
                        }
                        float _824 = _804.w;
                        float3 _827 = fast::clamp(_804.xyz / float3(_824), float3(0.0), float3(1.0));
                        float _828 = _827.x;
                        float _837;
                        if (_828 <= 0.040449999272823333740234375)
                        {
                            _837 = _828 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _837 = powr((_828 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _838 = _827.y;
                        float _847;
                        if (_838 <= 0.040449999272823333740234375)
                        {
                            _847 = _838 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _847 = powr((_838 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _848 = _827.z;
                        float _857;
                        if (_848 <= 0.040449999272823333740234375)
                        {
                            _857 = _848 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _857 = powr((_848 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _864 = float4(float3(_837, _847, _857) * _824, _824);
                        break;
                    } while(false);
                    _865 = _864;
                    break;
                } while(false);
                float4 _878;
                if (_865.w > 0.0)
                {
                    _878 = float4(_865.xyz / float3(_865.w), _865.w);
                }
                else
                {
                    _878 = float4(0.0);
                }
                int _880 = int(in.in_var_RADII.x);
                float _883 = fast::clamp(_878.x, 0.0, 1.0) * 255.0;
                float _885 = precise::min(floor(_883), 254.0);
                int _886 = int(_885);
                float _902 = fast::clamp(_878.y, 0.0, 1.0) * 255.0;
                float _904 = precise::min(floor(_902), 254.0);
                int _905 = int(_904);
                float _921 = fast::clamp(_878.z, 0.0, 1.0) * 255.0;
                float _923 = precise::min(floor(_921), 254.0);
                int _924 = int(_923);
                float _940 = fast::clamp(_878.w, 0.0, 1.0) * 255.0;
                float _942 = precise::min(floor(_940), 254.0);
                int _943 = int(_942);
                float4 _958 = fast::clamp(float4(mix(gowakuTex1.read(uint2(int3(_886, _880, 0).xy), 0).x, gowakuTex1.read(uint2(int3(_886 + 1, _880, 0).xy), 0).x, _883 - _885), mix(gowakuTex1.read(uint2(int3(_905, _880, 0).xy), 0).y, gowakuTex1.read(uint2(int3(_905 + 1, _880, 0).xy), 0).y, _902 - _904), mix(gowakuTex1.read(uint2(int3(_924, _880, 0).xy), 0).z, gowakuTex1.read(uint2(int3(_924 + 1, _880, 0).xy), 0).z, _921 - _923), mix(gowakuTex1.read(uint2(int3(_943, _880, 0).xy), 0).w, gowakuTex1.read(uint2(int3(_943 + 1, _880, 0).xy), 0).w, _940 - _942)), float4(0.0), float4(1.0));
                float _960 = _958.w;
                _3053 = float4(_958.xyz * _960, _960);
                break;
            }
            if (_254 == 4u)
            {
                float4 _1053;
                do
                {
                    bool _984;
                    if (!((_249 & 2048u) != 0u))
                    {
                        bool _983;
                        if (!any(_253 < int2(0)))
                        {
                            _983 = any(_253 >= _251);
                        }
                        else
                        {
                            _983 = true;
                        }
                        _984 = _983;
                    }
                    else
                    {
                        _984 = true;
                    }
                    if (_984)
                    {
                        _1053 = float4(0.0);
                        break;
                    }
                    float4 _992 = gowakuTex0.read(uint2(int3(_253, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _1053 = float4(0.0, 0.0, 0.0, _992.w);
                        break;
                    }
                    float4 _1052;
                    do
                    {
                        bool _1008;
                        if (!((_249 & 256u) == 0u))
                        {
                            _1008 = _992.w <= 0.0;
                        }
                        else
                        {
                            _1008 = true;
                        }
                        if (_1008)
                        {
                            _1052 = _992;
                            break;
                        }
                        float _1012 = _992.w;
                        float3 _1015 = fast::clamp(_992.xyz / float3(_1012), float3(0.0), float3(1.0));
                        float _1016 = _1015.x;
                        float _1025;
                        if (_1016 <= 0.040449999272823333740234375)
                        {
                            _1025 = _1016 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1025 = powr((_1016 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1026 = _1015.y;
                        float _1035;
                        if (_1026 <= 0.040449999272823333740234375)
                        {
                            _1035 = _1026 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1035 = powr((_1026 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1036 = _1015.z;
                        float _1045;
                        if (_1036 <= 0.040449999272823333740234375)
                        {
                            _1045 = _1036 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1045 = powr((_1036 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _1052 = float4(float3(_1025, _1035, _1045) * _1012, _1012);
                        break;
                    } while(false);
                    _1053 = _1052;
                    break;
                } while(false);
                float4 _1138;
                do
                {
                    bool _1069;
                    if (!((_249 & 4096u) != 0u))
                    {
                        bool _1068;
                        if (!any(_253 < int2(0)))
                        {
                            _1068 = any(_253 >= _251);
                        }
                        else
                        {
                            _1068 = true;
                        }
                        _1069 = _1068;
                    }
                    else
                    {
                        _1069 = true;
                    }
                    if (_1069)
                    {
                        _1138 = float4(0.0);
                        break;
                    }
                    float4 _1077 = gowakuTex1.read(uint2(int3(_253, 0).xy), 0);
                    if ((_249 & 1024u) != 0u)
                    {
                        _1138 = float4(0.0, 0.0, 0.0, _1077.w);
                        break;
                    }
                    float4 _1137;
                    do
                    {
                        bool _1093;
                        if (!((_249 & 256u) == 0u))
                        {
                            _1093 = _1077.w <= 0.0;
                        }
                        else
                        {
                            _1093 = true;
                        }
                        if (_1093)
                        {
                            _1137 = _1077;
                            break;
                        }
                        float _1097 = _1077.w;
                        float3 _1100 = fast::clamp(_1077.xyz / float3(_1097), float3(0.0), float3(1.0));
                        float _1101 = _1100.x;
                        float _1110;
                        if (_1101 <= 0.040449999272823333740234375)
                        {
                            _1110 = _1101 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1110 = powr((_1101 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1111 = _1100.y;
                        float _1120;
                        if (_1111 <= 0.040449999272823333740234375)
                        {
                            _1120 = _1111 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1120 = powr((_1111 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1121 = _1100.z;
                        float _1130;
                        if (_1121 <= 0.040449999272823333740234375)
                        {
                            _1130 = _1121 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1130 = powr((_1121 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _1137 = float4(float3(_1110, _1120, _1130) * _1097, _1097);
                        break;
                    } while(false);
                    _1138 = _1137;
                    break;
                } while(false);
                bool _1147;
                uint _1140 = uint(in.in_var_RADII.x);
                float _1194;
                do
                {
                    _1147 = _1140 == 1u;
                    if (_1147)
                    {
                        _1194 = _1053.x * _1138.w;
                        break;
                    }
                    if (_1140 == 2u)
                    {
                        _1194 = _1053.x * (1.0 - _1138.w);
                        break;
                    }
                    if (_1140 == 3u)
                    {
                        _1194 = (_1053.x * _1138.w) + (_1138.x * (1.0 - _1053.w));
                        break;
                    }
                    if (_1140 == 4u)
                    {
                        _1194 = (_1053.x * (1.0 - _1138.w)) + (_1138.x * (1.0 - _1053.w));
                        break;
                    }
                    if (_1140 == 6u)
                    {
                        _1194 = precise::min(_1053.x + _1138.x, 1.0);
                        break;
                    }
                    if (_1140 == 5u)
                    {
                        _1194 = fast::clamp(((((in.in_var_STOPS.x * _1053.x) * _1138.x) + (in.in_var_STOPS.y * _1053.x)) + (in.in_var_STOPS.z * _1138.x)) + in.in_var_STOPS.w, 0.0, 1.0);
                        break;
                    }
                    _1194 = _1053.x + (_1138.x * (1.0 - _1053.w));
                    break;
                } while(false);
                float _1245;
                do
                {
                    if (_1147)
                    {
                        _1245 = _1053.y * _1138.w;
                        break;
                    }
                    if (_1140 == 2u)
                    {
                        _1245 = _1053.y * (1.0 - _1138.w);
                        break;
                    }
                    if (_1140 == 3u)
                    {
                        _1245 = (_1053.y * _1138.w) + (_1138.y * (1.0 - _1053.w));
                        break;
                    }
                    if (_1140 == 4u)
                    {
                        _1245 = (_1053.y * (1.0 - _1138.w)) + (_1138.y * (1.0 - _1053.w));
                        break;
                    }
                    if (_1140 == 6u)
                    {
                        _1245 = precise::min(_1053.y + _1138.y, 1.0);
                        break;
                    }
                    if (_1140 == 5u)
                    {
                        _1245 = fast::clamp(((((in.in_var_STOPS.x * _1053.y) * _1138.y) + (in.in_var_STOPS.y * _1053.y)) + (in.in_var_STOPS.z * _1138.y)) + in.in_var_STOPS.w, 0.0, 1.0);
                        break;
                    }
                    _1245 = _1053.y + (_1138.y * (1.0 - _1053.w));
                    break;
                } while(false);
                float _1296;
                do
                {
                    if (_1147)
                    {
                        _1296 = _1053.z * _1138.w;
                        break;
                    }
                    if (_1140 == 2u)
                    {
                        _1296 = _1053.z * (1.0 - _1138.w);
                        break;
                    }
                    if (_1140 == 3u)
                    {
                        _1296 = (_1053.z * _1138.w) + (_1138.z * (1.0 - _1053.w));
                        break;
                    }
                    if (_1140 == 4u)
                    {
                        _1296 = (_1053.z * (1.0 - _1138.w)) + (_1138.z * (1.0 - _1053.w));
                        break;
                    }
                    if (_1140 == 6u)
                    {
                        _1296 = precise::min(_1053.z + _1138.z, 1.0);
                        break;
                    }
                    if (_1140 == 5u)
                    {
                        _1296 = fast::clamp(((((in.in_var_STOPS.x * _1053.z) * _1138.z) + (in.in_var_STOPS.y * _1053.z)) + (in.in_var_STOPS.z * _1138.z)) + in.in_var_STOPS.w, 0.0, 1.0);
                        break;
                    }
                    _1296 = _1053.z + (_1138.z * (1.0 - _1053.w));
                    break;
                } while(false);
                float _1341;
                do
                {
                    if (_1147)
                    {
                        _1341 = _1053.w * _1138.w;
                        break;
                    }
                    if (_1140 == 2u)
                    {
                        _1341 = _1053.w * (1.0 - _1138.w);
                        break;
                    }
                    if (_1140 == 3u)
                    {
                        _1341 = _1138.w;
                        break;
                    }
                    if (_1140 == 4u)
                    {
                        _1341 = (_1053.w * (1.0 - _1138.w)) + (_1138.w * (1.0 - _1053.w));
                        break;
                    }
                    if (_1140 == 6u)
                    {
                        _1341 = precise::min(_1053.w + _1138.w, 1.0);
                        break;
                    }
                    if (_1140 == 5u)
                    {
                        _1341 = fast::clamp(((((in.in_var_STOPS.x * _1053.w) * _1138.w) + (in.in_var_STOPS.y * _1053.w)) + (in.in_var_STOPS.z * _1138.w)) + in.in_var_STOPS.w, 0.0, 1.0);
                        break;
                    }
                    _1341 = _1053.w + (_1138.w * (1.0 - _1053.w));
                    break;
                } while(false);
                float4 _1342 = float4(_1194, _1245, _1296, _1341);
                float4 _1350;
                if (_1140 == 5u)
                {
                    float3 _1348 = precise::min(_1342.xyz, float3(_1341));
                    _1350 = float4(_1348.x, _1348.y, _1348.z, _1342.w);
                }
                else
                {
                    _1350 = _1342;
                }
                _3053 = _1350;
                break;
            }
            if (_254 == 5u)
            {
                float4 _1438;
                do
                {
                    bool _1369;
                    if (!((_249 & 2048u) != 0u))
                    {
                        bool _1368;
                        if (!any(_253 < int2(0)))
                        {
                            _1368 = any(_253 >= _251);
                        }
                        else
                        {
                            _1368 = true;
                        }
                        _1369 = _1368;
                    }
                    else
                    {
                        _1369 = true;
                    }
                    if (_1369)
                    {
                        _1438 = float4(0.0);
                        break;
                    }
                    float4 _1377 = gowakuTex0.read(uint2(int3(_253, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _1438 = float4(0.0, 0.0, 0.0, _1377.w);
                        break;
                    }
                    float4 _1437;
                    do
                    {
                        bool _1393;
                        if (!((_249 & 256u) == 0u))
                        {
                            _1393 = _1377.w <= 0.0;
                        }
                        else
                        {
                            _1393 = true;
                        }
                        if (_1393)
                        {
                            _1437 = _1377;
                            break;
                        }
                        float _1397 = _1377.w;
                        float3 _1400 = fast::clamp(_1377.xyz / float3(_1397), float3(0.0), float3(1.0));
                        float _1401 = _1400.x;
                        float _1410;
                        if (_1401 <= 0.040449999272823333740234375)
                        {
                            _1410 = _1401 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1410 = powr((_1401 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1411 = _1400.y;
                        float _1420;
                        if (_1411 <= 0.040449999272823333740234375)
                        {
                            _1420 = _1411 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1420 = powr((_1411 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1421 = _1400.z;
                        float _1430;
                        if (_1421 <= 0.040449999272823333740234375)
                        {
                            _1430 = _1421 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1430 = powr((_1421 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _1437 = float4(float3(_1410, _1420, _1430) * _1397, _1397);
                        break;
                    } while(false);
                    _1438 = _1437;
                    break;
                } while(false);
                float4 _1523;
                do
                {
                    bool _1454;
                    if (!((_249 & 4096u) != 0u))
                    {
                        bool _1453;
                        if (!any(_253 < int2(0)))
                        {
                            _1453 = any(_253 >= _251);
                        }
                        else
                        {
                            _1453 = true;
                        }
                        _1454 = _1453;
                    }
                    else
                    {
                        _1454 = true;
                    }
                    if (_1454)
                    {
                        _1523 = float4(0.0);
                        break;
                    }
                    float4 _1462 = gowakuTex1.read(uint2(int3(_253, 0).xy), 0);
                    if ((_249 & 1024u) != 0u)
                    {
                        _1523 = float4(0.0, 0.0, 0.0, _1462.w);
                        break;
                    }
                    float4 _1522;
                    do
                    {
                        bool _1478;
                        if (!((_249 & 256u) == 0u))
                        {
                            _1478 = _1462.w <= 0.0;
                        }
                        else
                        {
                            _1478 = true;
                        }
                        if (_1478)
                        {
                            _1522 = _1462;
                            break;
                        }
                        float _1482 = _1462.w;
                        float3 _1485 = fast::clamp(_1462.xyz / float3(_1482), float3(0.0), float3(1.0));
                        float _1486 = _1485.x;
                        float _1495;
                        if (_1486 <= 0.040449999272823333740234375)
                        {
                            _1495 = _1486 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1495 = powr((_1486 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1496 = _1485.y;
                        float _1505;
                        if (_1496 <= 0.040449999272823333740234375)
                        {
                            _1505 = _1496 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1505 = powr((_1496 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _1506 = _1485.z;
                        float _1515;
                        if (_1506 <= 0.040449999272823333740234375)
                        {
                            _1515 = _1506 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _1515 = powr((_1506 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _1522 = float4(float3(_1495, _1505, _1515) * _1482, _1482);
                        break;
                    } while(false);
                    _1523 = _1522;
                    break;
                } while(false);
                uint _1525 = uint(in.in_var_RADII.x);
                float3 _1534;
                if (_1438.w > 0.0)
                {
                    _1534 = _1438.xyz / float3(_1438.w);
                }
                else
                {
                    _1534 = float3(0.0);
                }
                float3 _1543;
                if (_1523.w > 0.0)
                {
                    _1543 = _1523.xyz / float3(_1523.w);
                }
                else
                {
                    _1543 = float3(0.0);
                }
                float3 _2148;
                if (_1525 == 13u)
                {
                    _2148 = (_1534 + _1543) - (_1534 * _1543);
                }
                else
                {
                    float3 _2147;
                    if (_1525 >= 15u)
                    {
                        float3 _1554 = fast::clamp(_1543, float3(0.0), float3(1.0));
                        float3 _1555 = fast::clamp(_1534, float3(0.0), float3(1.0));
                        float3 _2146;
                        do
                        {
                            if (_1525 == 15u)
                            {
                                _2146 = _1554 * _1555;
                                break;
                            }
                            if (_1525 == 16u)
                            {
                                float _1565 = _1555.x;
                                float _1566 = _1554.x;
                                float _1578;
                                if (_1566 <= 0.5)
                                {
                                    _1578 = (_1565 * 2.0) * _1566;
                                }
                                else
                                {
                                    float _1574 = (2.0 * _1566) - 1.0;
                                    _1578 = (_1565 + _1574) - (_1565 * _1574);
                                }
                                float _1579 = _1555.y;
                                float _1580 = _1554.y;
                                float _1592;
                                if (_1580 <= 0.5)
                                {
                                    _1592 = (_1579 * 2.0) * _1580;
                                }
                                else
                                {
                                    float _1588 = (2.0 * _1580) - 1.0;
                                    _1592 = (_1579 + _1588) - (_1579 * _1588);
                                }
                                float _1593 = _1555.z;
                                float _1594 = _1554.z;
                                float _1606;
                                if (_1594 <= 0.5)
                                {
                                    _1606 = (_1593 * 2.0) * _1594;
                                }
                                else
                                {
                                    float _1602 = (2.0 * _1594) - 1.0;
                                    _1606 = (_1593 + _1602) - (_1593 * _1602);
                                }
                                _2146 = float3(_1578, _1592, _1606);
                                break;
                            }
                            if (_1525 == 17u)
                            {
                                _2146 = precise::min(_1554, _1555);
                                break;
                            }
                            if (_1525 == 18u)
                            {
                                _2146 = precise::max(_1554, _1555);
                                break;
                            }
                            if (_1525 == 19u)
                            {
                                float _1619 = _1554.x;
                                float _1620 = _1555.x;
                                float _1634;
                                do
                                {
                                    if (_1619 <= 0.0)
                                    {
                                        _1634 = 0.0;
                                        break;
                                    }
                                    float _1633;
                                    if (_1620 >= 1.0)
                                    {
                                        _1633 = 1.0;
                                    }
                                    else
                                    {
                                        _1633 = precise::min(1.0, _1619 / (1.0 - _1620));
                                    }
                                    _1634 = _1633;
                                    break;
                                } while(false);
                                float _1635 = _1554.y;
                                float _1636 = _1555.y;
                                float _1650;
                                do
                                {
                                    if (_1635 <= 0.0)
                                    {
                                        _1650 = 0.0;
                                        break;
                                    }
                                    float _1649;
                                    if (_1636 >= 1.0)
                                    {
                                        _1649 = 1.0;
                                    }
                                    else
                                    {
                                        _1649 = precise::min(1.0, _1635 / (1.0 - _1636));
                                    }
                                    _1650 = _1649;
                                    break;
                                } while(false);
                                float _1651 = _1554.z;
                                float _1652 = _1555.z;
                                float _1666;
                                do
                                {
                                    if (_1651 <= 0.0)
                                    {
                                        _1666 = 0.0;
                                        break;
                                    }
                                    float _1665;
                                    if (_1652 >= 1.0)
                                    {
                                        _1665 = 1.0;
                                    }
                                    else
                                    {
                                        _1665 = precise::min(1.0, _1651 / (1.0 - _1652));
                                    }
                                    _1666 = _1665;
                                    break;
                                } while(false);
                                _2146 = float3(_1634, _1650, _1666);
                                break;
                            }
                            if (_1525 == 20u)
                            {
                                float _1671 = _1554.x;
                                float _1672 = _1555.x;
                                float _1687;
                                do
                                {
                                    if (_1671 >= 1.0)
                                    {
                                        _1687 = 1.0;
                                        break;
                                    }
                                    float _1686;
                                    if (_1672 <= 0.0)
                                    {
                                        _1686 = 0.0;
                                    }
                                    else
                                    {
                                        _1686 = 1.0 - precise::min(1.0, (1.0 - _1671) / _1672);
                                    }
                                    _1687 = _1686;
                                    break;
                                } while(false);
                                float _1688 = _1554.y;
                                float _1689 = _1555.y;
                                float _1704;
                                do
                                {
                                    if (_1688 >= 1.0)
                                    {
                                        _1704 = 1.0;
                                        break;
                                    }
                                    float _1703;
                                    if (_1689 <= 0.0)
                                    {
                                        _1703 = 0.0;
                                    }
                                    else
                                    {
                                        _1703 = 1.0 - precise::min(1.0, (1.0 - _1688) / _1689);
                                    }
                                    _1704 = _1703;
                                    break;
                                } while(false);
                                float _1705 = _1554.z;
                                float _1706 = _1555.z;
                                float _1721;
                                do
                                {
                                    if (_1705 >= 1.0)
                                    {
                                        _1721 = 1.0;
                                        break;
                                    }
                                    float _1720;
                                    if (_1706 <= 0.0)
                                    {
                                        _1720 = 0.0;
                                    }
                                    else
                                    {
                                        _1720 = 1.0 - precise::min(1.0, (1.0 - _1705) / _1706);
                                    }
                                    _1721 = _1720;
                                    break;
                                } while(false);
                                _2146 = float3(_1687, _1704, _1721);
                                break;
                            }
                            if (_1525 == 21u)
                            {
                                float _1726 = _1554.x;
                                float _1727 = _1555.x;
                                float _1739;
                                if (_1727 <= 0.5)
                                {
                                    _1739 = (_1726 * 2.0) * _1727;
                                }
                                else
                                {
                                    float _1735 = (2.0 * _1727) - 1.0;
                                    _1739 = (_1726 + _1735) - (_1726 * _1735);
                                }
                                float _1740 = _1554.y;
                                float _1741 = _1555.y;
                                float _1753;
                                if (_1741 <= 0.5)
                                {
                                    _1753 = (_1740 * 2.0) * _1741;
                                }
                                else
                                {
                                    float _1749 = (2.0 * _1741) - 1.0;
                                    _1753 = (_1740 + _1749) - (_1740 * _1749);
                                }
                                float _1754 = _1554.z;
                                float _1755 = _1555.z;
                                float _1767;
                                if (_1755 <= 0.5)
                                {
                                    _1767 = (_1754 * 2.0) * _1755;
                                }
                                else
                                {
                                    float _1763 = (2.0 * _1755) - 1.0;
                                    _1767 = (_1754 + _1763) - (_1754 * _1763);
                                }
                                _2146 = float3(_1739, _1753, _1767);
                                break;
                            }
                            if (_1525 == 22u)
                            {
                                float _1772 = _1554.x;
                                float _1773 = _1555.x;
                                float _1801;
                                do
                                {
                                    if (_1773 <= 0.5)
                                    {
                                        _1801 = _1772 - (((1.0 - (2.0 * _1773)) * _1772) * (1.0 - _1772));
                                        break;
                                    }
                                    float _1795;
                                    if (_1772 <= 0.25)
                                    {
                                        _1795 = ((((16.0 * _1772) - 12.0) * _1772) + 4.0) * _1772;
                                    }
                                    else
                                    {
                                        _1795 = sqrt(_1772);
                                    }
                                    _1801 = _1772 + (((2.0 * _1773) - 1.0) * (_1795 - _1772));
                                    break;
                                } while(false);
                                float _1802 = _1554.y;
                                float _1803 = _1555.y;
                                float _1831;
                                do
                                {
                                    if (_1803 <= 0.5)
                                    {
                                        _1831 = _1802 - (((1.0 - (2.0 * _1803)) * _1802) * (1.0 - _1802));
                                        break;
                                    }
                                    float _1825;
                                    if (_1802 <= 0.25)
                                    {
                                        _1825 = ((((16.0 * _1802) - 12.0) * _1802) + 4.0) * _1802;
                                    }
                                    else
                                    {
                                        _1825 = sqrt(_1802);
                                    }
                                    _1831 = _1802 + (((2.0 * _1803) - 1.0) * (_1825 - _1802));
                                    break;
                                } while(false);
                                float _1832 = _1554.z;
                                float _1833 = _1555.z;
                                float _1861;
                                do
                                {
                                    if (_1833 <= 0.5)
                                    {
                                        _1861 = _1832 - (((1.0 - (2.0 * _1833)) * _1832) * (1.0 - _1832));
                                        break;
                                    }
                                    float _1855;
                                    if (_1832 <= 0.25)
                                    {
                                        _1855 = ((((16.0 * _1832) - 12.0) * _1832) + 4.0) * _1832;
                                    }
                                    else
                                    {
                                        _1855 = sqrt(_1832);
                                    }
                                    _1861 = _1832 + (((2.0 * _1833) - 1.0) * (_1855 - _1832));
                                    break;
                                } while(false);
                                _2146 = float3(_1801, _1831, _1861);
                                break;
                            }
                            if (_1525 == 23u)
                            {
                                _2146 = abs(_1554 - _1555);
                                break;
                            }
                            if (_1525 == 24u)
                            {
                                _2146 = (_1554 + _1555) - ((_1554 * 2.0) * _1555);
                                break;
                            }
                            if (_1525 == 25u)
                            {
                                float _1878 = _1554.x;
                                float _1879 = _1554.y;
                                float _1881 = _1554.z;
                                float _1886 = _1555.x;
                                float _1887 = _1555.y;
                                float _1889 = _1555.z;
                                float _1890 = precise::min(precise::min(_1886, _1887), _1889);
                                float _1892 = precise::max(precise::max(_1886, _1887), _1889);
                                float3 _1903;
                                if (_1892 > _1890)
                                {
                                    _1903 = ((_1555 - float3(_1890)) * (precise::max(precise::max(_1878, _1879), _1881) - precise::min(precise::min(_1878, _1879), _1881))) / float3(_1892 - _1890);
                                }
                                else
                                {
                                    _1903 = float3(0.0);
                                }
                                float3 _1919 = _1903 + float3((((0.300000011920928955078125 * _1878) + (0.589999973773956298828125 * _1879)) + (0.10999999940395355224609375 * _1881)) - (((0.300000011920928955078125 * _1903.x) + (0.589999973773956298828125 * _1903.y)) + (0.10999999940395355224609375 * _1903.z)));
                                float _1920 = _1919.x;
                                float _1922 = _1919.y;
                                float _1925 = _1919.z;
                                float _1927 = ((0.300000011920928955078125 * _1920) + (0.589999973773956298828125 * _1922)) + (0.10999999940395355224609375 * _1925);
                                float _1929 = precise::min(precise::min(_1920, _1922), _1925);
                                float _1931 = precise::max(precise::max(_1920, _1922), _1925);
                                float3 _1942;
                                if (_1929 < 0.0)
                                {
                                    float3 _1935 = float3(_1927);
                                    _1942 = _1935 + (((_1919 - _1935) * _1927) / float3(_1927 - _1929));
                                }
                                else
                                {
                                    _1942 = _1919;
                                }
                                float3 _1954;
                                if (_1931 > 1.0)
                                {
                                    float3 _1946 = float3(_1927);
                                    _1954 = _1946 + (((_1942 - _1946) * (1.0 - _1927)) / float3(_1931 - _1927));
                                }
                                else
                                {
                                    _1954 = _1942;
                                }
                                _2146 = _1954;
                                break;
                            }
                            if (_1525 == 26u)
                            {
                                float _1958 = _1555.x;
                                float _1959 = _1555.y;
                                float _1961 = _1555.z;
                                float _1966 = _1554.x;
                                float _1967 = _1554.y;
                                float _1969 = _1554.z;
                                float _1970 = precise::min(precise::min(_1966, _1967), _1969);
                                float _1972 = precise::max(precise::max(_1966, _1967), _1969);
                                float3 _1983;
                                if (_1972 > _1970)
                                {
                                    _1983 = ((_1554 - float3(_1970)) * (precise::max(precise::max(_1958, _1959), _1961) - precise::min(precise::min(_1958, _1959), _1961))) / float3(_1972 - _1970);
                                }
                                else
                                {
                                    _1983 = float3(0.0);
                                }
                                float3 _1999 = _1983 + float3((((0.300000011920928955078125 * _1966) + (0.589999973773956298828125 * _1967)) + (0.10999999940395355224609375 * _1969)) - (((0.300000011920928955078125 * _1983.x) + (0.589999973773956298828125 * _1983.y)) + (0.10999999940395355224609375 * _1983.z)));
                                float _2000 = _1999.x;
                                float _2002 = _1999.y;
                                float _2005 = _1999.z;
                                float _2007 = ((0.300000011920928955078125 * _2000) + (0.589999973773956298828125 * _2002)) + (0.10999999940395355224609375 * _2005);
                                float _2009 = precise::min(precise::min(_2000, _2002), _2005);
                                float _2011 = precise::max(precise::max(_2000, _2002), _2005);
                                float3 _2022;
                                if (_2009 < 0.0)
                                {
                                    float3 _2015 = float3(_2007);
                                    _2022 = _2015 + (((_1999 - _2015) * _2007) / float3(_2007 - _2009));
                                }
                                else
                                {
                                    _2022 = _1999;
                                }
                                float3 _2034;
                                if (_2011 > 1.0)
                                {
                                    float3 _2026 = float3(_2007);
                                    _2034 = _2026 + (((_2022 - _2026) * (1.0 - _2007)) / float3(_2011 - _2007));
                                }
                                else
                                {
                                    _2034 = _2022;
                                }
                                _2146 = _2034;
                                break;
                            }
                            if (_1525 == 27u)
                            {
                                float3 _2056 = _1555 + float3((((0.300000011920928955078125 * _1554.x) + (0.589999973773956298828125 * _1554.y)) + (0.10999999940395355224609375 * _1554.z)) - (((0.300000011920928955078125 * _1555.x) + (0.589999973773956298828125 * _1555.y)) + (0.10999999940395355224609375 * _1555.z)));
                                float _2057 = _2056.x;
                                float _2059 = _2056.y;
                                float _2062 = _2056.z;
                                float _2064 = ((0.300000011920928955078125 * _2057) + (0.589999973773956298828125 * _2059)) + (0.10999999940395355224609375 * _2062);
                                float _2066 = precise::min(precise::min(_2057, _2059), _2062);
                                float _2068 = precise::max(precise::max(_2057, _2059), _2062);
                                float3 _2079;
                                if (_2066 < 0.0)
                                {
                                    float3 _2072 = float3(_2064);
                                    _2079 = _2072 + (((_2056 - _2072) * _2064) / float3(_2064 - _2066));
                                }
                                else
                                {
                                    _2079 = _2056;
                                }
                                float3 _2091;
                                if (_2068 > 1.0)
                                {
                                    float3 _2083 = float3(_2064);
                                    _2091 = _2083 + (((_2079 - _2083) * (1.0 - _2064)) / float3(_2068 - _2064));
                                }
                                else
                                {
                                    _2091 = _2079;
                                }
                                _2146 = _2091;
                                break;
                            }
                            float3 _2110 = _1554 + float3((((0.300000011920928955078125 * _1555.x) + (0.589999973773956298828125 * _1555.y)) + (0.10999999940395355224609375 * _1555.z)) - (((0.300000011920928955078125 * _1554.x) + (0.589999973773956298828125 * _1554.y)) + (0.10999999940395355224609375 * _1554.z)));
                            float _2111 = _2110.x;
                            float _2113 = _2110.y;
                            float _2116 = _2110.z;
                            float _2118 = ((0.300000011920928955078125 * _2111) + (0.589999973773956298828125 * _2113)) + (0.10999999940395355224609375 * _2116);
                            float _2120 = precise::min(precise::min(_2111, _2113), _2116);
                            float _2122 = precise::max(precise::max(_2111, _2113), _2116);
                            float3 _2133;
                            if (_2120 < 0.0)
                            {
                                float3 _2126 = float3(_2118);
                                _2133 = _2126 + (((_2110 - _2126) * _2118) / float3(_2118 - _2120));
                            }
                            else
                            {
                                _2133 = _2110;
                            }
                            float3 _2145;
                            if (_2122 > 1.0)
                            {
                                float3 _2137 = float3(_2118);
                                _2145 = _2137 + (((_2133 - _2137) * (1.0 - _2118)) / float3(_2122 - _2118));
                            }
                            else
                            {
                                _2145 = _2133;
                            }
                            _2146 = _2145;
                            break;
                        } while(false);
                        _2147 = _2146;
                    }
                    else
                    {
                        _2147 = _1534;
                    }
                    _2148 = _2147;
                }
                float _2156 = _1438.w * _1523.w;
                _3053 = float4(((_1438.xyz * (1.0 - _1523.w)) + (_1523.xyz * (1.0 - _1438.w))) + (_2148 * _2156), (_1438.w + _1523.w) - _2156);
                break;
            }
            if (_254 == 6u)
            {
                bool _2178;
                int2 _2169 = int2(in.in_var_RADII.xy);
                int _2171 = int(in.in_var_RADII.z);
                bool _2173 = (_249 & 8192u) != 0u;
                float4 _2258;
                do
                {
                    _2178 = !((_249 & 2048u) != 0u);
                    bool _2189;
                    if (_2178)
                    {
                        bool _2188;
                        if (!any(_253 < int2(0)))
                        {
                            _2188 = any(_253 >= _251);
                        }
                        else
                        {
                            _2188 = true;
                        }
                        _2189 = _2188;
                    }
                    else
                    {
                        _2189 = true;
                    }
                    if (_2189)
                    {
                        _2258 = float4(0.0);
                        break;
                    }
                    float4 _2197 = gowakuTex0.read(uint2(int3(_253, 0).xy), 0);
                    if ((_249 & 512u) != 0u)
                    {
                        _2258 = float4(0.0, 0.0, 0.0, _2197.w);
                        break;
                    }
                    float4 _2257;
                    do
                    {
                        bool _2213;
                        if (!((_249 & 256u) == 0u))
                        {
                            _2213 = _2197.w <= 0.0;
                        }
                        else
                        {
                            _2213 = true;
                        }
                        if (_2213)
                        {
                            _2257 = _2197;
                            break;
                        }
                        float _2217 = _2197.w;
                        float3 _2220 = fast::clamp(_2197.xyz / float3(_2217), float3(0.0), float3(1.0));
                        float _2221 = _2220.x;
                        float _2230;
                        if (_2221 <= 0.040449999272823333740234375)
                        {
                            _2230 = _2221 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _2230 = powr((_2221 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _2231 = _2220.y;
                        float _2240;
                        if (_2231 <= 0.040449999272823333740234375)
                        {
                            _2240 = _2231 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _2240 = powr((_2231 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        float _2241 = _2220.z;
                        float _2250;
                        if (_2241 <= 0.040449999272823333740234375)
                        {
                            _2250 = _2241 * 0.077399380505084991455078125;
                        }
                        else
                        {
                            _2250 = powr((_2241 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                        }
                        _2257 = float4(float3(_2230, _2240, _2250) * _2217, _2217);
                        break;
                    } while(false);
                    _2258 = _2257;
                    break;
                } while(false);
                int _2259 = -_2171;
                float4 _2261;
                _2261 = _2258;
                float4 _2262;
                for (int _2264 = _2259; _2264 <= _2171; _2261 = _2262, _2264++)
                {
                    int2 _2271 = _253 + (_2169 * int2(_2264));
                    bool _2274 = !any(_2271 < int2(0));
                    bool _2279;
                    if (_2274)
                    {
                        _2279 = any(_2271 >= _251);
                    }
                    else
                    {
                        _2279 = true;
                    }
                    if (_2279)
                    {
                        _2262 = _2261;
                        continue;
                    }
                    float4 _2360;
                    do
                    {
                        bool _2291;
                        if (_2178)
                        {
                            bool _2290;
                            if (_2274)
                            {
                                _2290 = any(_2271 >= _251);
                            }
                            else
                            {
                                _2290 = true;
                            }
                            _2291 = _2290;
                        }
                        else
                        {
                            _2291 = true;
                        }
                        if (_2291)
                        {
                            _2360 = float4(0.0);
                            break;
                        }
                        float4 _2299 = gowakuTex0.read(uint2(int3(_2271, 0).xy), 0);
                        if ((_249 & 512u) != 0u)
                        {
                            _2360 = float4(0.0, 0.0, 0.0, _2299.w);
                            break;
                        }
                        float4 _2359;
                        do
                        {
                            bool _2315;
                            if (!((_249 & 256u) == 0u))
                            {
                                _2315 = _2299.w <= 0.0;
                            }
                            else
                            {
                                _2315 = true;
                            }
                            if (_2315)
                            {
                                _2359 = _2299;
                                break;
                            }
                            float _2319 = _2299.w;
                            float3 _2322 = fast::clamp(_2299.xyz / float3(_2319), float3(0.0), float3(1.0));
                            float _2323 = _2322.x;
                            float _2332;
                            if (_2323 <= 0.040449999272823333740234375)
                            {
                                _2332 = _2323 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2332 = powr((_2323 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            float _2333 = _2322.y;
                            float _2342;
                            if (_2333 <= 0.040449999272823333740234375)
                            {
                                _2342 = _2333 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2342 = powr((_2333 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            float _2343 = _2322.z;
                            float _2352;
                            if (_2343 <= 0.040449999272823333740234375)
                            {
                                _2352 = _2343 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2352 = powr((_2343 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            _2359 = float4(float3(_2332, _2342, _2352) * _2319, _2319);
                            break;
                        } while(false);
                        _2360 = _2359;
                        break;
                    } while(false);
                    float4 _2366;
                    if (_2173)
                    {
                        _2366 = precise::max(_2261, _2360);
                    }
                    else
                    {
                        _2366 = precise::min(_2261, _2360);
                    }
                    _2262 = _2366;
                }
                _3053 = _2261;
                break;
            }
            if (_254 == 7u)
            {
                int2 _2371 = int2(in.in_var_RADII.xy);
                float _2374 = (-0.5) / (in.in_var_RADII.z * in.in_var_RADII.z);
                int _2376 = int(in.in_var_RADII.w);
                int _2377 = -_2376;
                float _2379;
                float4 _2382;
                _2379 = 0.0;
                _2382 = float4(0.0);
                float _2380;
                float4 _2383;
                for (int _2384 = _2377; _2384 <= _2376; _2379 = _2380, _2382 = _2383, _2384++)
                {
                    float _2392 = exp(float(_2384 * _2384) * _2374);
                    int2 _2395 = _253 + (_2371 * int2(_2384));
                    float4 _2480;
                    do
                    {
                        bool _2411;
                        if (!((_249 & 2048u) != 0u))
                        {
                            bool _2410;
                            if (!any(_2395 < int2(0)))
                            {
                                _2410 = any(_2395 >= _251);
                            }
                            else
                            {
                                _2410 = true;
                            }
                            _2411 = _2410;
                        }
                        else
                        {
                            _2411 = true;
                        }
                        if (_2411)
                        {
                            _2480 = float4(0.0);
                            break;
                        }
                        float4 _2419 = gowakuTex0.read(uint2(int3(_2395, 0).xy), 0);
                        if ((_249 & 512u) != 0u)
                        {
                            _2480 = float4(0.0, 0.0, 0.0, _2419.w);
                            break;
                        }
                        float4 _2479;
                        do
                        {
                            bool _2435;
                            if (!((_249 & 256u) == 0u))
                            {
                                _2435 = _2419.w <= 0.0;
                            }
                            else
                            {
                                _2435 = true;
                            }
                            if (_2435)
                            {
                                _2479 = _2419;
                                break;
                            }
                            float _2439 = _2419.w;
                            float3 _2442 = fast::clamp(_2419.xyz / float3(_2439), float3(0.0), float3(1.0));
                            float _2443 = _2442.x;
                            float _2452;
                            if (_2443 <= 0.040449999272823333740234375)
                            {
                                _2452 = _2443 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2452 = powr((_2443 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            float _2453 = _2442.y;
                            float _2462;
                            if (_2453 <= 0.040449999272823333740234375)
                            {
                                _2462 = _2453 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2462 = powr((_2453 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            float _2463 = _2442.z;
                            float _2472;
                            if (_2463 <= 0.040449999272823333740234375)
                            {
                                _2472 = _2463 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2472 = powr((_2463 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            _2479 = float4(float3(_2452, _2462, _2472) * _2439, _2439);
                            break;
                        } while(false);
                        _2480 = _2479;
                        break;
                    } while(false);
                    _2383 = _2382 + (_2480 * _2392);
                    _2380 = _2379 + _2392;
                }
                _3053 = _2382 / float4(_2379);
                break;
            }
            if (_254 == 8u)
            {
                int2 _2488 = int2(in.in_var_RADII.xy);
                int _2490 = int(in.in_var_STOPS.w);
                int _2497 = int((in.in_var_STOPS.x + in.in_var_STOPS.y) + in.in_var_STOPS.z) - 2;
                float4 _2499;
                _2499 = float4(0.0);
                float4 _2500;
                for (int _2502 = 0; _2502 < _2497; _2499 = _2500, _2502++)
                {
                    int2 _2510 = _253 + (_2488 * int2(_2502 - _2490));
                    float4 _2595;
                    do
                    {
                        bool _2526;
                        if (!((_249 & 2048u) != 0u))
                        {
                            bool _2525;
                            if (!any(_2510 < int2(0)))
                            {
                                _2525 = any(_2510 >= _251);
                            }
                            else
                            {
                                _2525 = true;
                            }
                            _2526 = _2525;
                        }
                        else
                        {
                            _2526 = true;
                        }
                        if (_2526)
                        {
                            _2595 = float4(0.0);
                            break;
                        }
                        float4 _2534 = gowakuTex0.read(uint2(int3(_2510, 0).xy), 0);
                        if ((_249 & 512u) != 0u)
                        {
                            _2595 = float4(0.0, 0.0, 0.0, _2534.w);
                            break;
                        }
                        float4 _2594;
                        do
                        {
                            bool _2550;
                            if (!((_249 & 256u) == 0u))
                            {
                                _2550 = _2534.w <= 0.0;
                            }
                            else
                            {
                                _2550 = true;
                            }
                            if (_2550)
                            {
                                _2594 = _2534;
                                break;
                            }
                            float _2554 = _2534.w;
                            float3 _2557 = fast::clamp(_2534.xyz / float3(_2554), float3(0.0), float3(1.0));
                            float _2558 = _2557.x;
                            float _2567;
                            if (_2558 <= 0.040449999272823333740234375)
                            {
                                _2567 = _2558 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2567 = powr((_2558 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            float _2568 = _2557.y;
                            float _2577;
                            if (_2568 <= 0.040449999272823333740234375)
                            {
                                _2577 = _2568 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2577 = powr((_2568 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            float _2578 = _2557.z;
                            float _2587;
                            if (_2578 <= 0.040449999272823333740234375)
                            {
                                _2587 = _2578 * 0.077399380505084991455078125;
                            }
                            else
                            {
                                _2587 = powr((_2578 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                            }
                            _2594 = float4(float3(_2567, _2577, _2587) * _2554, _2554);
                            break;
                        } while(false);
                        _2595 = _2594;
                        break;
                    } while(false);
                    float _2596 = float(_2502);
                    float _2605;
                    if (_2596 < 0.0)
                    {
                        _2605 = 0.0;
                    }
                    else
                    {
                        _2605 = ((_2596 + 1.0) * (_2596 + 2.0)) * 0.5;
                    }
                    float _2606 = _2596 - in.in_var_STOPS.x;
                    float _2615;
                    if (_2606 < 0.0)
                    {
                        _2615 = 0.0;
                    }
                    else
                    {
                        _2615 = ((_2606 + 1.0) * (_2606 + 2.0)) * 0.5;
                    }
                    float _2617 = _2596 - in.in_var_STOPS.y;
                    float _2626;
                    if (_2617 < 0.0)
                    {
                        _2626 = 0.0;
                    }
                    else
                    {
                        _2626 = ((_2617 + 1.0) * (_2617 + 2.0)) * 0.5;
                    }
                    float _2628 = _2596 - in.in_var_STOPS.z;
                    float _2637;
                    if (_2628 < 0.0)
                    {
                        _2637 = 0.0;
                    }
                    else
                    {
                        _2637 = ((_2628 + 1.0) * (_2628 + 2.0)) * 0.5;
                    }
                    float _2639 = _2606 - in.in_var_STOPS.y;
                    float _2648;
                    if (_2639 < 0.0)
                    {
                        _2648 = 0.0;
                    }
                    else
                    {
                        _2648 = ((_2639 + 1.0) * (_2639 + 2.0)) * 0.5;
                    }
                    float _2650 = _2606 - in.in_var_STOPS.z;
                    float _2659;
                    if (_2650 < 0.0)
                    {
                        _2659 = 0.0;
                    }
                    else
                    {
                        _2659 = ((_2650 + 1.0) * (_2650 + 2.0)) * 0.5;
                    }
                    float _2661 = _2617 - in.in_var_STOPS.z;
                    float _2670;
                    if (_2661 < 0.0)
                    {
                        _2670 = 0.0;
                    }
                    else
                    {
                        _2670 = ((_2661 + 1.0) * (_2661 + 2.0)) * 0.5;
                    }
                    float _2672 = _2639 - in.in_var_STOPS.z;
                    float _2681;
                    if (_2672 < 0.0)
                    {
                        _2681 = 0.0;
                    }
                    else
                    {
                        _2681 = ((_2672 + 1.0) * (_2672 + 2.0)) * 0.5;
                    }
                    _2500 = _2499 + (_2595 * ((((((((_2605 - _2615) - _2626) - _2637) + _2648) + _2659) + _2670) - _2681) / ((in.in_var_STOPS.x * in.in_var_STOPS.y) * in.in_var_STOPS.z)));
                }
                _3053 = _2499;
                break;
            }
            if (_254 == 9u)
            {
                int2 _2693 = int3(_253, 0).xy;
                float4 _2697 = gowakuTex1.read(uint2(_2693), 0);
                bool _2705;
                if (!any(_253 < int2(0)))
                {
                    _2705 = any(_253 >= _251);
                }
                else
                {
                    _2705 = true;
                }
                if (_2705)
                {
                    _3053 = float4(0.0);
                    break;
                }
                float _2716;
                if ((_249 & 16384u) != 0u)
                {
                    _2716 = dot(_2697.xyz, float3(0.2125000059604644775390625, 0.7153999805450439453125, 0.07209999859333038330078125));
                }
                else
                {
                    _2716 = _2697.w;
                }
                _3053 = gowakuTex0.read(uint2(_2693), 0) * _2716;
                break;
            }
            bool _2727;
            float2 _2719 = floor(in.in_var_RADII.xy);
            float2 _2720 = in.in_var_RADII.xy - _2719;
            int2 _2722 = _253 - int2(_2719);
            float4 _2768;
            do
            {
                _2727 = !((_249 & 2048u) != 0u);
                bool _2738;
                if (_2727)
                {
                    bool _2737;
                    if (!any(_2722 < int2(0)))
                    {
                        _2737 = any(_2722 >= _251);
                    }
                    else
                    {
                        _2737 = true;
                    }
                    _2738 = _2737;
                }
                else
                {
                    _2738 = true;
                }
                if (_2738)
                {
                    _2768 = float4(0.0);
                    break;
                }
                float4 _2746 = gowakuTex0.read(uint2(int3(_2722, 0).xy), 0);
                if ((_249 & 512u) != 0u)
                {
                    _2768 = float4(0.0, 0.0, 0.0, _2746.w);
                    break;
                }
                float4 _2767;
                do
                {
                    bool _2762;
                    if (!((_249 & 256u) == 0u))
                    {
                        _2762 = _2746.w <= 0.0;
                    }
                    else
                    {
                        _2762 = true;
                    }
                    if (_2762)
                    {
                        _2767 = _2746;
                        break;
                    }
                    _2767 = float4(_124, _124, _124, _2746.w);
                    break;
                } while(false);
                _2768 = _2767;
                break;
            } while(false);
            int2 _2769 = _2722 - int2(1, 0);
            float4 _2812;
            do
            {
                bool _2782;
                if (_2727)
                {
                    bool _2781;
                    if (!any(_2769 < int2(0)))
                    {
                        _2781 = any(_2769 >= _251);
                    }
                    else
                    {
                        _2781 = true;
                    }
                    _2782 = _2781;
                }
                else
                {
                    _2782 = true;
                }
                if (_2782)
                {
                    _2812 = float4(0.0);
                    break;
                }
                float4 _2790 = gowakuTex0.read(uint2(int3(_2769, 0).xy), 0);
                if ((_249 & 512u) != 0u)
                {
                    _2812 = float4(0.0, 0.0, 0.0, _2790.w);
                    break;
                }
                float4 _2811;
                do
                {
                    bool _2806;
                    if (!((_249 & 256u) == 0u))
                    {
                        _2806 = _2790.w <= 0.0;
                    }
                    else
                    {
                        _2806 = true;
                    }
                    if (_2806)
                    {
                        _2811 = _2790;
                        break;
                    }
                    _2811 = float4(_124, _124, _124, _2790.w);
                    break;
                } while(false);
                _2812 = _2811;
                break;
            } while(false);
            float4 _2814 = float4(_2720.x);
            int2 _2816 = _2722 - int2(0, 1);
            float4 _2859;
            do
            {
                bool _2829;
                if (_2727)
                {
                    bool _2828;
                    if (!any(_2816 < int2(0)))
                    {
                        _2828 = any(_2816 >= _251);
                    }
                    else
                    {
                        _2828 = true;
                    }
                    _2829 = _2828;
                }
                else
                {
                    _2829 = true;
                }
                if (_2829)
                {
                    _2859 = float4(0.0);
                    break;
                }
                float4 _2837 = gowakuTex0.read(uint2(int3(_2816, 0).xy), 0);
                if ((_249 & 512u) != 0u)
                {
                    _2859 = float4(0.0, 0.0, 0.0, _2837.w);
                    break;
                }
                float4 _2858;
                do
                {
                    bool _2853;
                    if (!((_249 & 256u) == 0u))
                    {
                        _2853 = _2837.w <= 0.0;
                    }
                    else
                    {
                        _2853 = true;
                    }
                    if (_2853)
                    {
                        _2858 = _2837;
                        break;
                    }
                    _2858 = float4(_124, _124, _124, _2837.w);
                    break;
                } while(false);
                _2859 = _2858;
                break;
            } while(false);
            int2 _2860 = _2722 - int2(1);
            float4 _2903;
            do
            {
                bool _2873;
                if (_2727)
                {
                    bool _2872;
                    if (!any(_2860 < int2(0)))
                    {
                        _2872 = any(_2860 >= _251);
                    }
                    else
                    {
                        _2872 = true;
                    }
                    _2873 = _2872;
                }
                else
                {
                    _2873 = true;
                }
                if (_2873)
                {
                    _2903 = float4(0.0);
                    break;
                }
                float4 _2881 = gowakuTex0.read(uint2(int3(_2860, 0).xy), 0);
                if ((_249 & 512u) != 0u)
                {
                    _2903 = float4(0.0, 0.0, 0.0, _2881.w);
                    break;
                }
                float4 _2902;
                do
                {
                    bool _2897;
                    if (!((_249 & 256u) == 0u))
                    {
                        _2897 = _2881.w <= 0.0;
                    }
                    else
                    {
                        _2897 = true;
                    }
                    if (_2897)
                    {
                        _2902 = _2881;
                        break;
                    }
                    _2902 = float4(_124, _124, _124, _2881.w);
                    break;
                } while(false);
                _2903 = _2902;
                break;
            } while(false);
            float4 _2993;
            do
            {
                bool _2924;
                if (!((_249 & 4096u) != 0u))
                {
                    bool _2923;
                    if (!any(_253 < int2(0)))
                    {
                        _2923 = any(_253 >= _251);
                    }
                    else
                    {
                        _2923 = true;
                    }
                    _2924 = _2923;
                }
                else
                {
                    _2924 = true;
                }
                if (_2924)
                {
                    _2993 = float4(0.0);
                    break;
                }
                float4 _2932 = gowakuTex1.read(uint2(int3(_253, 0).xy), 0);
                if ((_249 & 1024u) != 0u)
                {
                    _2993 = float4(0.0, 0.0, 0.0, _2932.w);
                    break;
                }
                float4 _2992;
                do
                {
                    bool _2948;
                    if (!((_249 & 256u) == 0u))
                    {
                        _2948 = _2932.w <= 0.0;
                    }
                    else
                    {
                        _2948 = true;
                    }
                    if (_2948)
                    {
                        _2992 = _2932;
                        break;
                    }
                    float _2952 = _2932.w;
                    float3 _2955 = fast::clamp(_2932.xyz / float3(_2952), float3(0.0), float3(1.0));
                    float _2956 = _2955.x;
                    float _2965;
                    if (_2956 <= 0.040449999272823333740234375)
                    {
                        _2965 = _2956 * 0.077399380505084991455078125;
                    }
                    else
                    {
                        _2965 = powr((_2956 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                    }
                    float _2966 = _2955.y;
                    float _2975;
                    if (_2966 <= 0.040449999272823333740234375)
                    {
                        _2975 = _2966 * 0.077399380505084991455078125;
                    }
                    else
                    {
                        _2975 = powr((_2966 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                    }
                    float _2976 = _2955.z;
                    float _2985;
                    if (_2976 <= 0.040449999272823333740234375)
                    {
                        _2985 = _2976 * 0.077399380505084991455078125;
                    }
                    else
                    {
                        _2985 = powr((_2976 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                    }
                    _2992 = float4(float3(_2965, _2975, _2985) * _2952, _2952);
                    break;
                } while(false);
                _2993 = _2992;
                break;
            } while(false);
            float4 _3047;
            do
            {
                bool _3003;
                if (!((_249 & 256u) == 0u))
                {
                    _3003 = in.in_var_STOPS.w <= 0.0;
                }
                else
                {
                    _3003 = true;
                }
                if (_3003)
                {
                    _3047 = in.in_var_STOPS;
                    break;
                }
                float3 _3010 = fast::clamp(in.in_var_STOPS.xyz / float3(in.in_var_STOPS.w), float3(0.0), float3(1.0));
                float _3011 = _3010.x;
                float _3020;
                if (_3011 <= 0.040449999272823333740234375)
                {
                    _3020 = _3011 * 0.077399380505084991455078125;
                }
                else
                {
                    _3020 = powr((_3011 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                }
                float _3021 = _3010.y;
                float _3030;
                if (_3021 <= 0.040449999272823333740234375)
                {
                    _3030 = _3021 * 0.077399380505084991455078125;
                }
                else
                {
                    _3030 = powr((_3021 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                }
                float _3031 = _3010.z;
                float _3040;
                if (_3031 <= 0.040449999272823333740234375)
                {
                    _3040 = _3031 * 0.077399380505084991455078125;
                }
                else
                {
                    _3040 = powr((_3031 + 0.054999999701976776123046875) * 0.947867333889007568359375, 2.400000095367431640625);
                }
                _3047 = float4(float3(_3020, _3030, _3040) * in.in_var_STOPS.w, in.in_var_STOPS.w);
                break;
            } while(false);
            _3053 = _2993 + (_3047 * (mix(mix(_2768, _2812, _2814), mix(_2859, _2903, _2814), float4(_2720.y)).w * (1.0 - _2993.w)));
            break;
        } while(false);
        float4 _3107;
        do
        {
            bool _3063;
            if (!((_249 & 256u) == 0u))
            {
                _3063 = _3053.w <= 0.0;
            }
            else
            {
                _3063 = true;
            }
            if (_3063)
            {
                _3107 = _3053;
                break;
            }
            float3 _3070 = fast::clamp(_3053.xyz / float3(_3053.w), float3(0.0), float3(1.0));
            float _3071 = _3070.x;
            float _3080;
            if (_3071 <= 0.003130800090730190277099609375)
            {
                _3080 = _3071 * 12.9200000762939453125;
            }
            else
            {
                _3080 = (1.05499994754791259765625 * powr(_3071, 0.4166666567325592041015625)) - 0.054999999701976776123046875;
            }
            float _3081 = _3070.y;
            float _3090;
            if (_3081 <= 0.003130800090730190277099609375)
            {
                _3090 = _3081 * 12.9200000762939453125;
            }
            else
            {
                _3090 = (1.05499994754791259765625 * powr(_3081, 0.4166666567325592041015625)) - 0.054999999701976776123046875;
            }
            float _3091 = _3070.z;
            float _3100;
            if (_3091 <= 0.003130800090730190277099609375)
            {
                _3100 = _3091 * 12.9200000762939453125;
            }
            else
            {
                _3100 = (1.05499994754791259765625 * powr(_3091, 0.4166666567325592041015625)) - 0.054999999701976776123046875;
            }
            _3107 = float4(float3(_3080, _3090, _3100) * _3053.w, _3053.w);
            break;
        } while(false);
        float4 _3110 = _3107 * (in.in_var_COLOR0.w * _244);
        uint _3112 = in.in_var_MODE.x >> 8u;
        if (_3112 == 0u)
        {
            _3815 = _3110;
            break;
        }
        float4 _3814;
        do
        {
            if (_3112 == 1u)
            {
                _3814 = float4(0.0, 0.0, 0.0, _244);
                break;
            }
            bool _3127;
            if (!(_3112 == 6u))
            {
                _3127 = _3112 == 14u;
            }
            else
            {
                _3127 = true;
            }
            if (_3127)
            {
                _3814 = float4(_244) - _3110;
                break;
            }
            bool _3132 = _3112 == 2u;
            bool _3137;
            if (!_3132)
            {
                _3137 = _3112 == 5u;
            }
            else
            {
                _3137 = true;
            }
            bool _3142;
            if (!_3137)
            {
                _3142 = _3112 == 7u;
            }
            else
            {
                _3142 = true;
            }
            bool _3147;
            if (!_3142)
            {
                _3147 = _3112 == 10u;
            }
            else
            {
                _3147 = true;
            }
            bool _3152;
            if (!_3147)
            {
                _3152 = _3112 >= 15u;
            }
            else
            {
                _3152 = true;
            }
            if (_3152)
            {
                float4 _3813;
                do
                {
                    float4 _3165 = gowakuBackdrop.read(uint2(int3(int2(floor(gl_FragCoord.xy)), 0).xy), 0);
                    float4 _3167 = _3165 * (1.0 - _244);
                    if (_3132)
                    {
                        _3813 = _3110 + _3167;
                        break;
                    }
                    if (_3112 == 5u)
                    {
                        _3813 = (_3110 * _3165.w) + _3167;
                        break;
                    }
                    if (_3112 == 7u)
                    {
                        _3813 = (_3110 * (1.0 - _3165.w)) + _3167;
                        break;
                    }
                    if (_3112 == 10u)
                    {
                        _3813 = ((_3110 * (1.0 - _3165.w)) + (_3165 * _3110.w)) + _3167;
                        break;
                    }
                    float _3194 = _3110.w;
                    if (_3194 <= 0.0)
                    {
                        _3813 = float4(0.0);
                        break;
                    }
                    float3 _3201 = fast::clamp(_3110.xyz / float3(_3194), float3(0.0), float3(1.0));
                    float _3202 = _3165.w;
                    float3 _3211;
                    if (_3202 > 0.0)
                    {
                        _3211 = fast::clamp(_3165.xyz / float3(_3202), float3(0.0), float3(1.0));
                    }
                    else
                    {
                        _3211 = float3(0.0);
                    }
                    float3 _3804;
                    do
                    {
                        if (_3112 == 15u)
                        {
                            _3804 = _3211 * _3201;
                            break;
                        }
                        if (_3112 == 16u)
                        {
                            float _3223 = _3201.x;
                            float _3236;
                            if (_3211.x <= 0.5)
                            {
                                _3236 = (_3223 * 2.0) * _3211.x;
                            }
                            else
                            {
                                float _3232 = (2.0 * _3211.x) - 1.0;
                                _3236 = (_3223 + _3232) - (_3223 * _3232);
                            }
                            float _3237 = _3201.y;
                            float _3250;
                            if (_3211.y <= 0.5)
                            {
                                _3250 = (_3237 * 2.0) * _3211.y;
                            }
                            else
                            {
                                float _3246 = (2.0 * _3211.y) - 1.0;
                                _3250 = (_3237 + _3246) - (_3237 * _3246);
                            }
                            float _3251 = _3201.z;
                            float _3264;
                            if (_3211.z <= 0.5)
                            {
                                _3264 = (_3251 * 2.0) * _3211.z;
                            }
                            else
                            {
                                float _3260 = (2.0 * _3211.z) - 1.0;
                                _3264 = (_3251 + _3260) - (_3251 * _3260);
                            }
                            _3804 = float3(_3236, _3250, _3264);
                            break;
                        }
                        if (_3112 == 17u)
                        {
                            _3804 = precise::min(_3211, _3201);
                            break;
                        }
                        if (_3112 == 18u)
                        {
                            _3804 = precise::max(_3211, _3201);
                            break;
                        }
                        if (_3112 == 19u)
                        {
                            float _3278 = _3201.x;
                            float _3292;
                            do
                            {
                                if (_3211.x <= 0.0)
                                {
                                    _3292 = 0.0;
                                    break;
                                }
                                float _3291;
                                if (_3278 >= 1.0)
                                {
                                    _3291 = 1.0;
                                }
                                else
                                {
                                    _3291 = precise::min(1.0, _3211.x / (1.0 - _3278));
                                }
                                _3292 = _3291;
                                break;
                            } while(false);
                            float _3294 = _3201.y;
                            float _3308;
                            do
                            {
                                if (_3211.y <= 0.0)
                                {
                                    _3308 = 0.0;
                                    break;
                                }
                                float _3307;
                                if (_3294 >= 1.0)
                                {
                                    _3307 = 1.0;
                                }
                                else
                                {
                                    _3307 = precise::min(1.0, _3211.y / (1.0 - _3294));
                                }
                                _3308 = _3307;
                                break;
                            } while(false);
                            float _3310 = _3201.z;
                            float _3324;
                            do
                            {
                                if (_3211.z <= 0.0)
                                {
                                    _3324 = 0.0;
                                    break;
                                }
                                float _3323;
                                if (_3310 >= 1.0)
                                {
                                    _3323 = 1.0;
                                }
                                else
                                {
                                    _3323 = precise::min(1.0, _3211.z / (1.0 - _3310));
                                }
                                _3324 = _3323;
                                break;
                            } while(false);
                            _3804 = float3(_3292, _3308, _3324);
                            break;
                        }
                        if (_3112 == 20u)
                        {
                            float _3330 = _3201.x;
                            float _3345;
                            do
                            {
                                if (_3211.x >= 1.0)
                                {
                                    _3345 = 1.0;
                                    break;
                                }
                                float _3344;
                                if (_3330 <= 0.0)
                                {
                                    _3344 = 0.0;
                                }
                                else
                                {
                                    _3344 = 1.0 - precise::min(1.0, (1.0 - _3211.x) / _3330);
                                }
                                _3345 = _3344;
                                break;
                            } while(false);
                            float _3347 = _3201.y;
                            float _3362;
                            do
                            {
                                if (_3211.y >= 1.0)
                                {
                                    _3362 = 1.0;
                                    break;
                                }
                                float _3361;
                                if (_3347 <= 0.0)
                                {
                                    _3361 = 0.0;
                                }
                                else
                                {
                                    _3361 = 1.0 - precise::min(1.0, (1.0 - _3211.y) / _3347);
                                }
                                _3362 = _3361;
                                break;
                            } while(false);
                            float _3364 = _3201.z;
                            float _3379;
                            do
                            {
                                if (_3211.z >= 1.0)
                                {
                                    _3379 = 1.0;
                                    break;
                                }
                                float _3378;
                                if (_3364 <= 0.0)
                                {
                                    _3378 = 0.0;
                                }
                                else
                                {
                                    _3378 = 1.0 - precise::min(1.0, (1.0 - _3211.z) / _3364);
                                }
                                _3379 = _3378;
                                break;
                            } while(false);
                            _3804 = float3(_3345, _3362, _3379);
                            break;
                        }
                        if (_3112 == 21u)
                        {
                            float _3385 = _3201.x;
                            float _3397;
                            if (_3385 <= 0.5)
                            {
                                _3397 = (_3211.x * 2.0) * _3385;
                            }
                            else
                            {
                                float _3393 = (2.0 * _3385) - 1.0;
                                _3397 = (_3211.x + _3393) - (_3211.x * _3393);
                            }
                            float _3399 = _3201.y;
                            float _3411;
                            if (_3399 <= 0.5)
                            {
                                _3411 = (_3211.y * 2.0) * _3399;
                            }
                            else
                            {
                                float _3407 = (2.0 * _3399) - 1.0;
                                _3411 = (_3211.y + _3407) - (_3211.y * _3407);
                            }
                            float _3413 = _3201.z;
                            float _3425;
                            if (_3413 <= 0.5)
                            {
                                _3425 = (_3211.z * 2.0) * _3413;
                            }
                            else
                            {
                                float _3421 = (2.0 * _3413) - 1.0;
                                _3425 = (_3211.z + _3421) - (_3211.z * _3421);
                            }
                            _3804 = float3(_3397, _3411, _3425);
                            break;
                        }
                        if (_3112 == 22u)
                        {
                            float _3431 = _3201.x;
                            float _3459;
                            do
                            {
                                if (_3431 <= 0.5)
                                {
                                    _3459 = _3211.x - (((1.0 - (2.0 * _3431)) * _3211.x) * (1.0 - _3211.x));
                                    break;
                                }
                                float _3453;
                                if (_3211.x <= 0.25)
                                {
                                    _3453 = ((((16.0 * _3211.x) - 12.0) * _3211.x) + 4.0) * _3211.x;
                                }
                                else
                                {
                                    _3453 = sqrt(_3211.x);
                                }
                                _3459 = _3211.x + (((2.0 * _3431) - 1.0) * (_3453 - _3211.x));
                                break;
                            } while(false);
                            float _3461 = _3201.y;
                            float _3489;
                            do
                            {
                                if (_3461 <= 0.5)
                                {
                                    _3489 = _3211.y - (((1.0 - (2.0 * _3461)) * _3211.y) * (1.0 - _3211.y));
                                    break;
                                }
                                float _3483;
                                if (_3211.y <= 0.25)
                                {
                                    _3483 = ((((16.0 * _3211.y) - 12.0) * _3211.y) + 4.0) * _3211.y;
                                }
                                else
                                {
                                    _3483 = sqrt(_3211.y);
                                }
                                _3489 = _3211.y + (((2.0 * _3461) - 1.0) * (_3483 - _3211.y));
                                break;
                            } while(false);
                            float _3491 = _3201.z;
                            float _3519;
                            do
                            {
                                if (_3491 <= 0.5)
                                {
                                    _3519 = _3211.z - (((1.0 - (2.0 * _3491)) * _3211.z) * (1.0 - _3211.z));
                                    break;
                                }
                                float _3513;
                                if (_3211.z <= 0.25)
                                {
                                    _3513 = ((((16.0 * _3211.z) - 12.0) * _3211.z) + 4.0) * _3211.z;
                                }
                                else
                                {
                                    _3513 = sqrt(_3211.z);
                                }
                                _3519 = _3211.z + (((2.0 * _3491) - 1.0) * (_3513 - _3211.z));
                                break;
                            } while(false);
                            _3804 = float3(_3459, _3489, _3519);
                            break;
                        }
                        if (_3112 == 23u)
                        {
                            _3804 = abs(_3211 - _3201);
                            break;
                        }
                        if (_3112 == 24u)
                        {
                            _3804 = (_3211 + _3201) - ((_3211 * 2.0) * _3201);
                            break;
                        }
                        if (_3112 == 25u)
                        {
                            float _3544 = _3201.x;
                            float _3545 = _3201.y;
                            float _3547 = _3201.z;
                            float _3548 = precise::min(precise::min(_3544, _3545), _3547);
                            float _3550 = precise::max(precise::max(_3544, _3545), _3547);
                            float3 _3561;
                            if (_3550 > _3548)
                            {
                                _3561 = ((_3201 - float3(_3548)) * (precise::max(precise::max(_3211.x, _3211.y), _3211.z) - precise::min(precise::min(_3211.x, _3211.y), _3211.z))) / float3(_3550 - _3548);
                            }
                            else
                            {
                                _3561 = float3(0.0);
                            }
                            float3 _3577 = _3561 + float3((((0.300000011920928955078125 * _3211.x) + (0.589999973773956298828125 * _3211.y)) + (0.10999999940395355224609375 * _3211.z)) - (((0.300000011920928955078125 * _3561.x) + (0.589999973773956298828125 * _3561.y)) + (0.10999999940395355224609375 * _3561.z)));
                            float _3578 = _3577.x;
                            float _3580 = _3577.y;
                            float _3583 = _3577.z;
                            float _3585 = ((0.300000011920928955078125 * _3578) + (0.589999973773956298828125 * _3580)) + (0.10999999940395355224609375 * _3583);
                            float _3587 = precise::min(precise::min(_3578, _3580), _3583);
                            float _3589 = precise::max(precise::max(_3578, _3580), _3583);
                            float3 _3600;
                            if (_3587 < 0.0)
                            {
                                float3 _3593 = float3(_3585);
                                _3600 = _3593 + (((_3577 - _3593) * _3585) / float3(_3585 - _3587));
                            }
                            else
                            {
                                _3600 = _3577;
                            }
                            float3 _3612;
                            if (_3589 > 1.0)
                            {
                                float3 _3604 = float3(_3585);
                                _3612 = _3604 + (((_3600 - _3604) * (1.0 - _3585)) / float3(_3589 - _3585));
                            }
                            else
                            {
                                _3612 = _3600;
                            }
                            _3804 = _3612;
                            break;
                        }
                        if (_3112 == 26u)
                        {
                            float _3616 = _3201.x;
                            float _3617 = _3201.y;
                            float _3619 = _3201.z;
                            float _3628 = precise::min(precise::min(_3211.x, _3211.y), _3211.z);
                            float _3630 = precise::max(precise::max(_3211.x, _3211.y), _3211.z);
                            float3 _3641;
                            if (_3630 > _3628)
                            {
                                _3641 = ((_3211 - float3(_3628)) * (precise::max(precise::max(_3616, _3617), _3619) - precise::min(precise::min(_3616, _3617), _3619))) / float3(_3630 - _3628);
                            }
                            else
                            {
                                _3641 = float3(0.0);
                            }
                            float3 _3657 = _3641 + float3((((0.300000011920928955078125 * _3211.x) + (0.589999973773956298828125 * _3211.y)) + (0.10999999940395355224609375 * _3211.z)) - (((0.300000011920928955078125 * _3641.x) + (0.589999973773956298828125 * _3641.y)) + (0.10999999940395355224609375 * _3641.z)));
                            float _3658 = _3657.x;
                            float _3660 = _3657.y;
                            float _3663 = _3657.z;
                            float _3665 = ((0.300000011920928955078125 * _3658) + (0.589999973773956298828125 * _3660)) + (0.10999999940395355224609375 * _3663);
                            float _3667 = precise::min(precise::min(_3658, _3660), _3663);
                            float _3669 = precise::max(precise::max(_3658, _3660), _3663);
                            float3 _3680;
                            if (_3667 < 0.0)
                            {
                                float3 _3673 = float3(_3665);
                                _3680 = _3673 + (((_3657 - _3673) * _3665) / float3(_3665 - _3667));
                            }
                            else
                            {
                                _3680 = _3657;
                            }
                            float3 _3692;
                            if (_3669 > 1.0)
                            {
                                float3 _3684 = float3(_3665);
                                _3692 = _3684 + (((_3680 - _3684) * (1.0 - _3665)) / float3(_3669 - _3665));
                            }
                            else
                            {
                                _3692 = _3680;
                            }
                            _3804 = _3692;
                            break;
                        }
                        if (_3112 == 27u)
                        {
                            float3 _3714 = _3201 + float3((((0.300000011920928955078125 * _3211.x) + (0.589999973773956298828125 * _3211.y)) + (0.10999999940395355224609375 * _3211.z)) - (((0.300000011920928955078125 * _3201.x) + (0.589999973773956298828125 * _3201.y)) + (0.10999999940395355224609375 * _3201.z)));
                            float _3715 = _3714.x;
                            float _3717 = _3714.y;
                            float _3720 = _3714.z;
                            float _3722 = ((0.300000011920928955078125 * _3715) + (0.589999973773956298828125 * _3717)) + (0.10999999940395355224609375 * _3720);
                            float _3724 = precise::min(precise::min(_3715, _3717), _3720);
                            float _3726 = precise::max(precise::max(_3715, _3717), _3720);
                            float3 _3737;
                            if (_3724 < 0.0)
                            {
                                float3 _3730 = float3(_3722);
                                _3737 = _3730 + (((_3714 - _3730) * _3722) / float3(_3722 - _3724));
                            }
                            else
                            {
                                _3737 = _3714;
                            }
                            float3 _3749;
                            if (_3726 > 1.0)
                            {
                                float3 _3741 = float3(_3722);
                                _3749 = _3741 + (((_3737 - _3741) * (1.0 - _3722)) / float3(_3726 - _3722));
                            }
                            else
                            {
                                _3749 = _3737;
                            }
                            _3804 = _3749;
                            break;
                        }
                        float3 _3768 = _3211 + float3((((0.300000011920928955078125 * _3201.x) + (0.589999973773956298828125 * _3201.y)) + (0.10999999940395355224609375 * _3201.z)) - (((0.300000011920928955078125 * _3211.x) + (0.589999973773956298828125 * _3211.y)) + (0.10999999940395355224609375 * _3211.z)));
                        float _3769 = _3768.x;
                        float _3771 = _3768.y;
                        float _3774 = _3768.z;
                        float _3776 = ((0.300000011920928955078125 * _3769) + (0.589999973773956298828125 * _3771)) + (0.10999999940395355224609375 * _3774);
                        float _3778 = precise::min(precise::min(_3769, _3771), _3774);
                        float _3780 = precise::max(precise::max(_3769, _3771), _3774);
                        float3 _3791;
                        if (_3778 < 0.0)
                        {
                            float3 _3784 = float3(_3776);
                            _3791 = _3784 + (((_3768 - _3784) * _3776) / float3(_3776 - _3778));
                        }
                        else
                        {
                            _3791 = _3768;
                        }
                        float3 _3803;
                        if (_3780 > 1.0)
                        {
                            float3 _3795 = float3(_3776);
                            _3803 = _3795 + (((_3791 - _3795) * (1.0 - _3776)) / float3(_3780 - _3776));
                        }
                        else
                        {
                            _3803 = _3791;
                        }
                        _3804 = _3803;
                        break;
                    } while(false);
                    _3813 = float4(((_3201 * (1.0 - _3202)) + (fast::clamp(_3804, float3(0.0), float3(1.0)) * _3202)) * _3194, _3194);
                    break;
                } while(false);
                _3814 = _3813;
                break;
            }
            _3814 = _3110;
            break;
        } while(false);
        _3815 = _3814;
        break;
    } while(false);
    out.out_var_SV_Target = _3815;
    return out;
}

