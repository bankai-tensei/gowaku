package geom

// Offset is a point or a displacement in logical pixels.
type Offset struct {
	X, Y float32
}

// Add returns o translated by d.
func (o Offset) Add(d Offset) Offset { return Offset{o.X + d.X, o.Y + d.Y} }

// Sub returns the displacement from d to o.
func (o Offset) Sub(d Offset) Offset { return Offset{o.X - d.X, o.Y - d.Y} }

// Scale returns o multiplied by s on both axes.
func (o Offset) Scale(s float32) Offset { return Offset{o.X * s, o.Y * s} }

// Size is a width and height in logical pixels.
type Size struct {
	W, H float32
}

// Add returns the component-wise sum of s and d.
func (s Size) Add(d Size) Size { return Size{s.W + d.W, s.H + d.H} }

// Empty reports whether s encloses no area.
func (s Size) Empty() bool { return s.W <= 0 || s.H <= 0 }

// Length returns the distance of o from the origin.
func (o Offset) Length() float32 { return Sqrt32(o.X*o.X + o.Y*o.Y) }
