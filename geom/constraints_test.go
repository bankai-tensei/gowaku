package geom

import "testing"

func TestConstraints_Constrain(t *testing.T) {
	c := Constraints{MinW: 10, MaxW: 20, MinH: 5, MaxH: Inf}
	tests := []struct {
		name     string
		in, want Size
	}{
		{"範囲内", Size{15, 7}, Size{15, 7}},
		{"小さすぎる", Size{1, 1}, Size{10, 5}},
		{"大きすぎる", Size{100, 1000}, Size{20, 1000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.Constrain(tt.in); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstraints_Deflate(t *testing.T) {
	c := Constraints{MinW: 10, MaxW: 100, MinH: 0, MaxH: Inf}
	got := c.Deflate(Insets{Left: 5, Right: 10, Top: 1, Bottom: 1})
	if want := (Constraints{MinW: 0, MaxW: 85, MinH: 0, MaxH: Inf}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestConstraints_DeflateNeverGoesNegative(t *testing.T) {
	got := Tight(Size{4, 4}).Deflate(All(10))
	if want := (Constraints{}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestConstraints_IsTight(t *testing.T) {
	if !Tight(Size{1, 2}).IsTight() {
		t.Error("Tight must be tight")
	}
	if (Constraints{MaxW: 1, MinH: 1, MaxH: 1}).IsTight() {
		t.Error("a free width is not tight")
	}
}

func TestConstraints_EnforceLetsTheOuterWin(t *testing.T) {
	outer := Constraints{MinW: 10, MaxW: 20, MinH: 0, MaxH: 100}
	got := outer.Enforce(Tight(Size{50, 50}))
	if want := (Constraints{MinW: 20, MaxW: 20, MinH: 50, MaxH: 50}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
