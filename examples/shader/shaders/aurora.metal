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
    float2 in_var_PARAMS [[user(locn7), flat]];
    uint2 in_var_MODE [[user(locn8)]];
    float4 in_var_STOPS [[user(locn11), flat]];
    float4 in_var_TRANSFORM0 [[user(locn12), flat]];
    float2 in_var_TRANSFORM1 [[user(locn13), flat]];
    float4 in_var_GRADIENT1 [[user(locn14), flat]];
};

fragment main0_out main0(main0_in in [[stage_in]], constant type_GowakuFrame& GowakuFrame [[buffer(0)]], texture2d<float> gowakuBackdrop [[texture(3)]], float4 gl_FragCoord [[position]])
{
    main0_out out = {};
    float4 _938;
    do
    {
        float _199;
        float2 _200;
        if (any(in.in_var_TRANSFORM0 != float4(0.0)))
        {
            float2 _111 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_TRANSFORM1;
            float _113 = _111.x;
            float _116 = _111.y;
            float _127 = (in.in_var_TRANSFORM0.x * in.in_var_TRANSFORM0.w) - (in.in_var_TRANSFORM0.y * in.in_var_TRANSFORM0.z);
            float2 _134 = in.in_var_RECT.zw * 0.5;
            float2 _136 = (in.in_var_LOGICAL - in.in_var_RECT.xy) - _134;
            float2 _138 = abs(_136) - _134;
            float _139 = _138.x;
            float _140 = _138.y;
            bool _153;
            if (_139 > 0.0)
            {
                _153 = _140 > 0.0;
            }
            else
            {
                _153 = false;
            }
            float2 _157;
            if (_153)
            {
                _157 = fast::normalize(_138);
            }
            else
            {
                _157 = select(float2(0.0, 1.0), float2(1.0, 0.0), bool2(_139 > _140));
            }
            float _166;
            if (_136.x < 0.0)
            {
                _166 = -_157.x;
            }
            else
            {
                _166 = _157.x;
            }
            float _175;
            if (_136.y < 0.0)
            {
                _175 = -_157.y;
            }
            else
            {
                _175 = _157.y;
            }
            float2 _176 = float2(_166, _175);
            _199 = fast::clamp(0.5 - ((precise::min(precise::max(_139, _140), 0.0) + length(precise::max(_138, float2(0.0)))) / precise::max(length(float2(dot(_176, float2(in.in_var_TRANSFORM0.w, -in.in_var_TRANSFORM0.y)), dot(_176, float2(-in.in_var_TRANSFORM0.z, in.in_var_TRANSFORM0.x)))) / (abs(_127) * GowakuFrame.gowakuScale), 9.9999999600419720025001879548654e-13)), 0.0, 1.0);
            _200 = fast::clamp((float2((in.in_var_TRANSFORM0.w * _113) - (in.in_var_TRANSFORM0.z * _116), (in.in_var_TRANSFORM0.x * _116) - (in.in_var_TRANSFORM0.y * _113)) / float2(_127)) - in.in_var_RECT.xy, float2(0.0), in.in_var_RECT.zw);
        }
        else
        {
            _199 = 1.0;
            _200 = (gl_FragCoord.xy / float2(GowakuFrame.gowakuScale)) - in.in_var_RECT.xy;
        }
        float2 _201 = float2((in.in_var_PARAMS.y > 0.0) ? in.in_var_PARAMS.y : 1.0);
        float4 _223 = mix(in.in_var_RADII, in.in_var_STOPS, float4(fast::clamp((_200 / in.in_var_RECT.zw).x + ((0.5 * sin((length((_200 / _201) - ((in.in_var_RECT.zw / _201) * 0.5)) * in.in_var_GRADIENT1.x) - (in.in_var_PARAMS.x * in.in_var_GRADIENT1.y))) * in.in_var_GRADIENT1.z), 0.0, 1.0)));
        float _225 = _223.w;
        float4 _233 = float4(_223.xyz * _225, _225) * (in.in_var_COLOR0.w * _199);
        uint _235 = in.in_var_MODE.x >> 8u;
        if (_235 == 0u)
        {
            _938 = _233;
            break;
        }
        float4 _937;
        do
        {
            if (_235 == 1u)
            {
                _937 = float4(0.0, 0.0, 0.0, _199);
                break;
            }
            bool _250;
            if (!(_235 == 6u))
            {
                _250 = _235 == 14u;
            }
            else
            {
                _250 = true;
            }
            if (_250)
            {
                _937 = float4(_199) - _233;
                break;
            }
            bool _255 = _235 == 2u;
            bool _260;
            if (!_255)
            {
                _260 = _235 == 5u;
            }
            else
            {
                _260 = true;
            }
            bool _265;
            if (!_260)
            {
                _265 = _235 == 7u;
            }
            else
            {
                _265 = true;
            }
            bool _270;
            if (!_265)
            {
                _270 = _235 == 10u;
            }
            else
            {
                _270 = true;
            }
            bool _275;
            if (!_270)
            {
                _275 = _235 >= 15u;
            }
            else
            {
                _275 = true;
            }
            if (_275)
            {
                float4 _936;
                do
                {
                    float4 _288 = gowakuBackdrop.read(uint2(int3(int2(floor(gl_FragCoord.xy)), 0).xy), 0);
                    float4 _290 = _288 * (1.0 - _199);
                    if (_255)
                    {
                        _936 = _233 + _290;
                        break;
                    }
                    if (_235 == 5u)
                    {
                        _936 = (_233 * _288.w) + _290;
                        break;
                    }
                    if (_235 == 7u)
                    {
                        _936 = (_233 * (1.0 - _288.w)) + _290;
                        break;
                    }
                    if (_235 == 10u)
                    {
                        _936 = ((_233 * (1.0 - _288.w)) + (_288 * _233.w)) + _290;
                        break;
                    }
                    float _317 = _233.w;
                    if (_317 <= 0.0)
                    {
                        _936 = float4(0.0);
                        break;
                    }
                    float3 _324 = fast::clamp(_233.xyz / float3(_317), float3(0.0), float3(1.0));
                    float _325 = _288.w;
                    float3 _334;
                    if (_325 > 0.0)
                    {
                        _334 = fast::clamp(_288.xyz / float3(_325), float3(0.0), float3(1.0));
                    }
                    else
                    {
                        _334 = float3(0.0);
                    }
                    float3 _927;
                    do
                    {
                        if (_235 == 15u)
                        {
                            _927 = _334 * _324;
                            break;
                        }
                        if (_235 == 16u)
                        {
                            float _346 = _324.x;
                            float _359;
                            if (_334.x <= 0.5)
                            {
                                _359 = (_346 * 2.0) * _334.x;
                            }
                            else
                            {
                                float _355 = (2.0 * _334.x) - 1.0;
                                _359 = (_346 + _355) - (_346 * _355);
                            }
                            float _360 = _324.y;
                            float _373;
                            if (_334.y <= 0.5)
                            {
                                _373 = (_360 * 2.0) * _334.y;
                            }
                            else
                            {
                                float _369 = (2.0 * _334.y) - 1.0;
                                _373 = (_360 + _369) - (_360 * _369);
                            }
                            float _374 = _324.z;
                            float _387;
                            if (_334.z <= 0.5)
                            {
                                _387 = (_374 * 2.0) * _334.z;
                            }
                            else
                            {
                                float _383 = (2.0 * _334.z) - 1.0;
                                _387 = (_374 + _383) - (_374 * _383);
                            }
                            _927 = float3(_359, _373, _387);
                            break;
                        }
                        if (_235 == 17u)
                        {
                            _927 = precise::min(_334, _324);
                            break;
                        }
                        if (_235 == 18u)
                        {
                            _927 = precise::max(_334, _324);
                            break;
                        }
                        if (_235 == 19u)
                        {
                            float _401 = _324.x;
                            float _415;
                            do
                            {
                                if (_334.x <= 0.0)
                                {
                                    _415 = 0.0;
                                    break;
                                }
                                float _414;
                                if (_401 >= 1.0)
                                {
                                    _414 = 1.0;
                                }
                                else
                                {
                                    _414 = precise::min(1.0, _334.x / (1.0 - _401));
                                }
                                _415 = _414;
                                break;
                            } while(false);
                            float _417 = _324.y;
                            float _431;
                            do
                            {
                                if (_334.y <= 0.0)
                                {
                                    _431 = 0.0;
                                    break;
                                }
                                float _430;
                                if (_417 >= 1.0)
                                {
                                    _430 = 1.0;
                                }
                                else
                                {
                                    _430 = precise::min(1.0, _334.y / (1.0 - _417));
                                }
                                _431 = _430;
                                break;
                            } while(false);
                            float _433 = _324.z;
                            float _447;
                            do
                            {
                                if (_334.z <= 0.0)
                                {
                                    _447 = 0.0;
                                    break;
                                }
                                float _446;
                                if (_433 >= 1.0)
                                {
                                    _446 = 1.0;
                                }
                                else
                                {
                                    _446 = precise::min(1.0, _334.z / (1.0 - _433));
                                }
                                _447 = _446;
                                break;
                            } while(false);
                            _927 = float3(_415, _431, _447);
                            break;
                        }
                        if (_235 == 20u)
                        {
                            float _453 = _324.x;
                            float _468;
                            do
                            {
                                if (_334.x >= 1.0)
                                {
                                    _468 = 1.0;
                                    break;
                                }
                                float _467;
                                if (_453 <= 0.0)
                                {
                                    _467 = 0.0;
                                }
                                else
                                {
                                    _467 = 1.0 - precise::min(1.0, (1.0 - _334.x) / _453);
                                }
                                _468 = _467;
                                break;
                            } while(false);
                            float _470 = _324.y;
                            float _485;
                            do
                            {
                                if (_334.y >= 1.0)
                                {
                                    _485 = 1.0;
                                    break;
                                }
                                float _484;
                                if (_470 <= 0.0)
                                {
                                    _484 = 0.0;
                                }
                                else
                                {
                                    _484 = 1.0 - precise::min(1.0, (1.0 - _334.y) / _470);
                                }
                                _485 = _484;
                                break;
                            } while(false);
                            float _487 = _324.z;
                            float _502;
                            do
                            {
                                if (_334.z >= 1.0)
                                {
                                    _502 = 1.0;
                                    break;
                                }
                                float _501;
                                if (_487 <= 0.0)
                                {
                                    _501 = 0.0;
                                }
                                else
                                {
                                    _501 = 1.0 - precise::min(1.0, (1.0 - _334.z) / _487);
                                }
                                _502 = _501;
                                break;
                            } while(false);
                            _927 = float3(_468, _485, _502);
                            break;
                        }
                        if (_235 == 21u)
                        {
                            float _508 = _324.x;
                            float _520;
                            if (_508 <= 0.5)
                            {
                                _520 = (_334.x * 2.0) * _508;
                            }
                            else
                            {
                                float _516 = (2.0 * _508) - 1.0;
                                _520 = (_334.x + _516) - (_334.x * _516);
                            }
                            float _522 = _324.y;
                            float _534;
                            if (_522 <= 0.5)
                            {
                                _534 = (_334.y * 2.0) * _522;
                            }
                            else
                            {
                                float _530 = (2.0 * _522) - 1.0;
                                _534 = (_334.y + _530) - (_334.y * _530);
                            }
                            float _536 = _324.z;
                            float _548;
                            if (_536 <= 0.5)
                            {
                                _548 = (_334.z * 2.0) * _536;
                            }
                            else
                            {
                                float _544 = (2.0 * _536) - 1.0;
                                _548 = (_334.z + _544) - (_334.z * _544);
                            }
                            _927 = float3(_520, _534, _548);
                            break;
                        }
                        if (_235 == 22u)
                        {
                            float _554 = _324.x;
                            float _582;
                            do
                            {
                                if (_554 <= 0.5)
                                {
                                    _582 = _334.x - (((1.0 - (2.0 * _554)) * _334.x) * (1.0 - _334.x));
                                    break;
                                }
                                float _576;
                                if (_334.x <= 0.25)
                                {
                                    _576 = ((((16.0 * _334.x) - 12.0) * _334.x) + 4.0) * _334.x;
                                }
                                else
                                {
                                    _576 = sqrt(_334.x);
                                }
                                _582 = _334.x + (((2.0 * _554) - 1.0) * (_576 - _334.x));
                                break;
                            } while(false);
                            float _584 = _324.y;
                            float _612;
                            do
                            {
                                if (_584 <= 0.5)
                                {
                                    _612 = _334.y - (((1.0 - (2.0 * _584)) * _334.y) * (1.0 - _334.y));
                                    break;
                                }
                                float _606;
                                if (_334.y <= 0.25)
                                {
                                    _606 = ((((16.0 * _334.y) - 12.0) * _334.y) + 4.0) * _334.y;
                                }
                                else
                                {
                                    _606 = sqrt(_334.y);
                                }
                                _612 = _334.y + (((2.0 * _584) - 1.0) * (_606 - _334.y));
                                break;
                            } while(false);
                            float _614 = _324.z;
                            float _642;
                            do
                            {
                                if (_614 <= 0.5)
                                {
                                    _642 = _334.z - (((1.0 - (2.0 * _614)) * _334.z) * (1.0 - _334.z));
                                    break;
                                }
                                float _636;
                                if (_334.z <= 0.25)
                                {
                                    _636 = ((((16.0 * _334.z) - 12.0) * _334.z) + 4.0) * _334.z;
                                }
                                else
                                {
                                    _636 = sqrt(_334.z);
                                }
                                _642 = _334.z + (((2.0 * _614) - 1.0) * (_636 - _334.z));
                                break;
                            } while(false);
                            _927 = float3(_582, _612, _642);
                            break;
                        }
                        if (_235 == 23u)
                        {
                            _927 = abs(_334 - _324);
                            break;
                        }
                        if (_235 == 24u)
                        {
                            _927 = (_334 + _324) - ((_334 * 2.0) * _324);
                            break;
                        }
                        if (_235 == 25u)
                        {
                            float _667 = _324.x;
                            float _668 = _324.y;
                            float _670 = _324.z;
                            float _671 = precise::min(precise::min(_667, _668), _670);
                            float _673 = precise::max(precise::max(_667, _668), _670);
                            float3 _684;
                            if (_673 > _671)
                            {
                                _684 = ((_324 - float3(_671)) * (precise::max(precise::max(_334.x, _334.y), _334.z) - precise::min(precise::min(_334.x, _334.y), _334.z))) / float3(_673 - _671);
                            }
                            else
                            {
                                _684 = float3(0.0);
                            }
                            float3 _700 = _684 + float3((((0.300000011920928955078125 * _334.x) + (0.589999973773956298828125 * _334.y)) + (0.10999999940395355224609375 * _334.z)) - (((0.300000011920928955078125 * _684.x) + (0.589999973773956298828125 * _684.y)) + (0.10999999940395355224609375 * _684.z)));
                            float _701 = _700.x;
                            float _703 = _700.y;
                            float _706 = _700.z;
                            float _708 = ((0.300000011920928955078125 * _701) + (0.589999973773956298828125 * _703)) + (0.10999999940395355224609375 * _706);
                            float _710 = precise::min(precise::min(_701, _703), _706);
                            float _712 = precise::max(precise::max(_701, _703), _706);
                            float3 _723;
                            if (_710 < 0.0)
                            {
                                float3 _716 = float3(_708);
                                _723 = _716 + (((_700 - _716) * _708) / float3(_708 - _710));
                            }
                            else
                            {
                                _723 = _700;
                            }
                            float3 _735;
                            if (_712 > 1.0)
                            {
                                float3 _727 = float3(_708);
                                _735 = _727 + (((_723 - _727) * (1.0 - _708)) / float3(_712 - _708));
                            }
                            else
                            {
                                _735 = _723;
                            }
                            _927 = _735;
                            break;
                        }
                        if (_235 == 26u)
                        {
                            float _739 = _324.x;
                            float _740 = _324.y;
                            float _742 = _324.z;
                            float _751 = precise::min(precise::min(_334.x, _334.y), _334.z);
                            float _753 = precise::max(precise::max(_334.x, _334.y), _334.z);
                            float3 _764;
                            if (_753 > _751)
                            {
                                _764 = ((_334 - float3(_751)) * (precise::max(precise::max(_739, _740), _742) - precise::min(precise::min(_739, _740), _742))) / float3(_753 - _751);
                            }
                            else
                            {
                                _764 = float3(0.0);
                            }
                            float3 _780 = _764 + float3((((0.300000011920928955078125 * _334.x) + (0.589999973773956298828125 * _334.y)) + (0.10999999940395355224609375 * _334.z)) - (((0.300000011920928955078125 * _764.x) + (0.589999973773956298828125 * _764.y)) + (0.10999999940395355224609375 * _764.z)));
                            float _781 = _780.x;
                            float _783 = _780.y;
                            float _786 = _780.z;
                            float _788 = ((0.300000011920928955078125 * _781) + (0.589999973773956298828125 * _783)) + (0.10999999940395355224609375 * _786);
                            float _790 = precise::min(precise::min(_781, _783), _786);
                            float _792 = precise::max(precise::max(_781, _783), _786);
                            float3 _803;
                            if (_790 < 0.0)
                            {
                                float3 _796 = float3(_788);
                                _803 = _796 + (((_780 - _796) * _788) / float3(_788 - _790));
                            }
                            else
                            {
                                _803 = _780;
                            }
                            float3 _815;
                            if (_792 > 1.0)
                            {
                                float3 _807 = float3(_788);
                                _815 = _807 + (((_803 - _807) * (1.0 - _788)) / float3(_792 - _788));
                            }
                            else
                            {
                                _815 = _803;
                            }
                            _927 = _815;
                            break;
                        }
                        if (_235 == 27u)
                        {
                            float3 _837 = _324 + float3((((0.300000011920928955078125 * _334.x) + (0.589999973773956298828125 * _334.y)) + (0.10999999940395355224609375 * _334.z)) - (((0.300000011920928955078125 * _324.x) + (0.589999973773956298828125 * _324.y)) + (0.10999999940395355224609375 * _324.z)));
                            float _838 = _837.x;
                            float _840 = _837.y;
                            float _843 = _837.z;
                            float _845 = ((0.300000011920928955078125 * _838) + (0.589999973773956298828125 * _840)) + (0.10999999940395355224609375 * _843);
                            float _847 = precise::min(precise::min(_838, _840), _843);
                            float _849 = precise::max(precise::max(_838, _840), _843);
                            float3 _860;
                            if (_847 < 0.0)
                            {
                                float3 _853 = float3(_845);
                                _860 = _853 + (((_837 - _853) * _845) / float3(_845 - _847));
                            }
                            else
                            {
                                _860 = _837;
                            }
                            float3 _872;
                            if (_849 > 1.0)
                            {
                                float3 _864 = float3(_845);
                                _872 = _864 + (((_860 - _864) * (1.0 - _845)) / float3(_849 - _845));
                            }
                            else
                            {
                                _872 = _860;
                            }
                            _927 = _872;
                            break;
                        }
                        float3 _891 = _334 + float3((((0.300000011920928955078125 * _324.x) + (0.589999973773956298828125 * _324.y)) + (0.10999999940395355224609375 * _324.z)) - (((0.300000011920928955078125 * _334.x) + (0.589999973773956298828125 * _334.y)) + (0.10999999940395355224609375 * _334.z)));
                        float _892 = _891.x;
                        float _894 = _891.y;
                        float _897 = _891.z;
                        float _899 = ((0.300000011920928955078125 * _892) + (0.589999973773956298828125 * _894)) + (0.10999999940395355224609375 * _897);
                        float _901 = precise::min(precise::min(_892, _894), _897);
                        float _903 = precise::max(precise::max(_892, _894), _897);
                        float3 _914;
                        if (_901 < 0.0)
                        {
                            float3 _907 = float3(_899);
                            _914 = _907 + (((_891 - _907) * _899) / float3(_899 - _901));
                        }
                        else
                        {
                            _914 = _891;
                        }
                        float3 _926;
                        if (_903 > 1.0)
                        {
                            float3 _918 = float3(_899);
                            _926 = _918 + (((_914 - _918) * (1.0 - _899)) / float3(_903 - _899));
                        }
                        else
                        {
                            _926 = _914;
                        }
                        _927 = _926;
                        break;
                    } while(false);
                    _936 = float4(((_324 * (1.0 - _325)) + (fast::clamp(_927, float3(0.0), float3(1.0)) * _325)) * _317, _317);
                    break;
                } while(false);
                _937 = _936;
                break;
            }
            _937 = _233;
            break;
        } while(false);
        _938 = _937;
        break;
    } while(false);
    out.out_var_SV_Target = _938;
    return out;
}

