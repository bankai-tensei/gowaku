package geom

// Affine is a 2D affine transform. A point (x, y) maps to
// (A*x + C*y + E, B*x + D*y + F).
//
// The zero value counts as the identity wherever a transform is optional:
// IsIdentity reports true for it, and paint.Canvas.Transform and the Transform
// widget leave the drawing as it is. Mul, Apply and the other methods use plain
// arithmetic, so start from Identity when composing transforms.
type Affine struct {
	A, B, C, D, E, F float32
}

// Identity returns the transform that leaves points unchanged.
func Identity() Affine { return Affine{A: 1, D: 1} }

// Translation returns a transform that moves points by d.
func Translation(d Offset) Affine { return Affine{A: 1, D: 1, E: d.X, F: d.Y} }

// Scaling returns a transform that scales about the origin.
func Scaling(sx, sy float32) Affine { return Affine{A: sx, D: sy} }

// Rotation returns a transform that rotates about the origin by radians.
// Positive angles turn clockwise on screen, where Y points down.
func Rotation(radians float32) Affine {
	s, c := Sin32(radians), Cos32(radians)
	return Affine{A: c, B: s, C: -s, D: c}
}

// Skewing returns a transform that slants about the origin: x moves by
// tan(sx) times y, and y by tan(sy) times x. The angles are in radians.
func Skewing(sx, sy float32) Affine {
	return Affine{A: 1, B: Tan32(sy), C: Tan32(sx), D: 1}
}

// IsIdentity reports whether m leaves points unchanged. The zero value counts
// as the identity.
func (m Affine) IsIdentity() bool {
	return m == Affine{} || m == Identity()
}

// IsScaleTranslate reports whether m only scales, by the same factor on both
// axes, and moves. Such a transform keeps rectangles upright and circles round.
// The zero value counts, as the identity.
func (m Affine) IsScaleTranslate() bool {
	return m == Affine{} || (m.B == 0 && m.C == 0 && m.A == m.D)
}

// Mul returns the transform that applies o first and then m.
func (m Affine) Mul(o Affine) Affine {
	return Affine{
		A: m.A*o.A + m.C*o.B,
		B: m.B*o.A + m.D*o.B,
		C: m.A*o.C + m.C*o.D,
		D: m.B*o.C + m.D*o.D,
		E: m.A*o.E + m.C*o.F + m.E,
		F: m.B*o.E + m.D*o.F + m.F,
	}
}

// Det returns the determinant of the linear part of m: the factor by which it
// changes areas. Zero means that m flattens everything onto a line or a point.
func (m Affine) Det() float32 { return m.A*m.D - m.B*m.C }

// Invert returns the transform that undoes m. It reports false if m flattens
// the plane, which no transform can undo.
func (m Affine) Invert() (Affine, bool) {
	det := m.Det()
	if det == 0 {
		return Affine{}, false
	}
	inv := 1 / det
	a, b, c, d := m.D*inv, -m.B*inv, -m.C*inv, m.A*inv
	return Affine{A: a, B: b, C: c, D: d, E: -(a*m.E + c*m.F), F: -(b*m.E + d*m.F)}, true
}

// Apply returns p transformed by m.
func (m Affine) Apply(p Offset) Offset {
	return Offset{m.A*p.X + m.C*p.Y + m.E, m.B*p.X + m.D*p.Y + m.F}
}

// ApplyVector returns v transformed by the linear part of m, without the move:
// what a difference of two points becomes.
func (m Affine) ApplyVector(v Offset) Offset {
	return Offset{m.A*v.X + m.C*v.Y, m.B*v.X + m.D*v.Y}
}

// ApplyRect returns the smallest upright rectangle that holds r transformed by
// m.
func (m Affine) ApplyRect(r Rect) Rect {
	p0 := m.Apply(Offset{r.X, r.Y})
	p1 := m.Apply(Offset{r.X + r.W, r.Y})
	p2 := m.Apply(Offset{r.X, r.Y + r.H})
	p3 := m.Apply(Offset{r.X + r.W, r.Y + r.H})
	left, top := min(p0.X, p1.X, p2.X, p3.X), min(p0.Y, p1.Y, p2.Y, p3.Y)
	right, bottom := max(p0.X, p1.X, p2.X, p3.X), max(p0.Y, p1.Y, p2.Y, p3.Y)
	return Rect{X: left, Y: top, W: right - left, H: bottom - top}
}

// MinScale returns the smallest factor by which m stretches a length, whatever
// its direction: the smaller singular value of the linear part.
func (m Affine) MinScale() float32 {
	sum := m.A*m.A + m.B*m.B + m.C*m.C + m.D*m.D
	det := m.Det()
	largest := Sqrt32((sum + Sqrt32(max(sum*sum-4*det*det, 0))) * 0.5)
	if largest == 0 {
		return 0
	}
	// Dividing the area factor by the larger value avoids the cancellation of
	// the textbook formula for the smaller one.
	return Abs32(det) / largest
}
