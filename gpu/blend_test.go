package gpu

import "testing"

func TestBlendMode_Factors(t *testing.T) {
	tests := []struct {
		mode     BlendMode
		src, dst BlendFactor
		reads    bool
	}{
		{BlendSrcOver, FactorOne, FactorInvSrcAlpha, false},
		{BlendClear, FactorZero, FactorInvSrcAlpha, false},
		{BlendSrc, FactorOne, FactorZero, true},
		{BlendDst, FactorZero, FactorOne, false},
		{BlendDstOver, FactorInvDstAlpha, FactorOne, false},
		{BlendSrcIn, FactorOne, FactorZero, true},
		{BlendDstIn, FactorZero, FactorInvSrcAlpha, false},
		{BlendSrcOut, FactorOne, FactorZero, true},
		{BlendDstOut, FactorZero, FactorInvSrcAlpha, false},
		{BlendSrcAtop, FactorDstAlpha, FactorInvSrcAlpha, false},
		{BlendDstAtop, FactorOne, FactorZero, true},
		{BlendXor, FactorInvDstAlpha, FactorInvSrcAlpha, false},
		{BlendPlus, FactorOne, FactorOne, false},
		{BlendScreen, FactorOne, FactorInvSrcColor, false},
		{BlendModulate, FactorZero, FactorInvSrcColor, false},
		{BlendMultiply, FactorOne, FactorInvSrcAlpha, true},
		{BlendLuminosity, FactorOne, FactorInvSrcAlpha, true},
	}
	for _, tt := range tests {
		src, dst := tt.mode.Factors()
		if src != tt.src || dst != tt.dst {
			t.Errorf("%d: factors %d, %d, want %d, %d", tt.mode, src, dst, tt.src, tt.dst)
		}
		if got := tt.mode.ReadsBackdrop(); got != tt.reads {
			t.Errorf("%d: ReadsBackdrop %v, want %v", tt.mode, got, tt.reads)
		}
	}
	for m := BlendMultiply; int(m) < BlendModeCount; m++ {
		if !m.ReadsBackdrop() {
			t.Errorf("%d must read the backdrop", m)
		}
	}
	if BlendMode(BlendModeCount).ReadsBackdrop() {
		t.Error("an unknown mode must not read the backdrop")
	}
}

func TestInstanceKind_Blend(t *testing.T) {
	for m := range BlendMode(BlendModeCount) {
		for _, k := range []InstanceKind{InstanceRRect, InstanceShadow, InstanceGlyph, InstanceCover, InstanceImage} {
			got := k.WithBlend(m)
			if got.Base() != k || got.Blend() != m {
				t.Errorf("%d with %d: got base %d and blend %d", k, m, got.Base(), got.Blend())
			}
			if got.WithBlend(BlendSrcOver) != k {
				t.Errorf("%d with %d and back: got %d", k, m, got.WithBlend(BlendSrcOver))
			}
		}
	}
}

func TestBlendMode_State(t *testing.T) {
	tests := map[BlendMode]BlendMode{
		BlendSrcOver:    BlendSrcOver,
		BlendMultiply:   BlendSrcOver,
		BlendLuminosity: BlendSrcOver,
		BlendSrc:        BlendSrc,
		BlendSrcIn:      BlendSrc,
		BlendDstAtop:    BlendSrc,
		BlendClear:      BlendClear,
		BlendDstIn:      BlendClear,
		BlendDstOut:     BlendClear,
		BlendScreen:     BlendScreen,
	}
	for m, want := range tests {
		if got := m.State(); got != want {
			t.Errorf("%d: got %d, want %d", m, got, want)
		}
	}
}
