package layout

import (
	"math/rand/v2"
	"testing"
)

// slowMeasured is MeasuredExtents the obvious way, to check it against.
type slowMeasured struct {
	h   []float32 // -1 for unmeasured
	est float32
	avg bool
}

func (s *slowMeasured) estimate() float32 {
	if !s.avg {
		return s.est
	}
	var sum float64
	n := 0
	for _, h := range s.h {
		if h >= 0 {
			sum += float64(h)
			n++
		}
	}
	if n == 0 {
		return s.est
	}
	return float32(sum / float64(n))
}

func (s *slowMeasured) extent(i int) float32 {
	if s.h[i] >= 0 {
		return s.h[i]
	}
	return s.estimate()
}

func (s *slowMeasured) offset(i int) float32 {
	var top float64
	for k := range i {
		top += float64(s.extent(k))
	}
	return float32(top)
}

func checkMeasured(t *testing.T, step int, m *MeasuredExtents, s *slowMeasured) {
	t.Helper()
	if m.Len() != len(s.h) {
		t.Fatalf("step %d: Len = %d, want %d", step, m.Len(), len(s.h))
	}
	if got, want := m.Estimate(), s.estimate(); !near(got, want) {
		t.Fatalf("step %d: Estimate = %v, want %v", step, got, want)
	}
	for i := 0; i <= len(s.h); i++ {
		if got, want := m.Offset(i), s.offset(i); !near(float32(got), want) {
			t.Fatalf("step %d: Offset(%d) = %v, want %v", step, i, got, want)
		}
	}
	if got, want := m.Total(), s.offset(len(s.h)); !near(float32(got), want) {
		t.Fatalf("step %d: Total = %v, want %v", step, got, want)
	}
	for i := range s.h {
		if s.extent(i) == 0 {
			continue
		}
		top := m.Offset(i)
		for _, y := range []float64{top, top + float64(m.Extent(i))/2} {
			if got := m.IndexAt(y); got != i {
				t.Fatalf("step %d: IndexAt(%v) = %d, want %d", step, y, got, i)
			}
		}
	}
	if len(s.h) > 0 {
		if got := m.IndexAt(-5); got != 0 {
			t.Fatalf("step %d: IndexAt(-5) = %d", step, got)
		}
		if got := m.IndexAt(m.Total() + 100); got != len(s.h)-1 {
			t.Fatalf("step %d: IndexAt past the end = %d", step, got)
		}
	}
}

func near(a, b float32) bool {
	d := a - b
	return d > -0.01 && d < 0.01
}

func TestMeasuredExtents_MatchesTheSlowWay(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	var m MeasuredExtents
	s := &slowMeasured{}
	m.SetEstimate(40, true)
	s.est, s.avg = 40, true
	checkMeasured(t, -1, &m, s)
	for step := range 400 {
		switch op := r.IntN(10); {
		case op < 5 && len(s.h) > 0:
			i := r.IntN(len(s.h))
			h := float32(r.IntN(8)) * 12 // zero hides the row
			m.Set(i, h)
			s.h[i] = h
		case op < 6 && len(s.h) > 0:
			i := r.IntN(len(s.h))
			m.Forget(i)
			s.h[i] = -1
		case op < 8:
			n := r.IntN(70)
			m.Resize(n)
			for len(s.h) < n {
				s.h = append(s.h, -1)
			}
			s.h = s.h[:n]
		case op < 9:
			e, avg := float32(r.IntN(60)), r.IntN(2) == 0
			m.SetEstimate(e, avg)
			s.est, s.avg = e, avg
		default:
			m.ForgetAll()
			for i := range s.h {
				s.h[i] = -1
			}
		}
		checkMeasured(t, step, &m, s)
	}
}

func TestMeasuredExtents_CountsMeasuredRows(t *testing.T) {
	var m MeasuredExtents
	m.Resize(10)
	m.SetEstimate(50, false)
	if m.Total() != 500 || m.MeasuredCount() != 0 || m.Measured(3) {
		t.Fatalf("Total = %v, measured = %d", m.Total(), m.MeasuredCount())
	}
	m.Set(3, 20)
	m.Set(3, 30)
	if m.Total() != 480 || m.MeasuredCount() != 1 || !m.Measured(3) || m.Extent(3) != 30 {
		t.Fatalf("Total = %v, measured = %d", m.Total(), m.MeasuredCount())
	}
	// Growing keeps what was measured.
	m.Resize(1000)
	if !m.Measured(3) || m.Extent(3) != 30 || m.Total() != 30+999*50 {
		t.Fatalf("after growing Total = %v", m.Total())
	}
	m.Resize(2)
	if m.MeasuredCount() != 0 || m.Total() != 100 {
		t.Fatalf("after shrinking Total = %v, measured = %d", m.Total(), m.MeasuredCount())
	}
}

func TestMeasuredExtents_NegativeHeightPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	var m MeasuredExtents
	m.Resize(1)
	m.Set(0, -1)
}

func TestMeasuredExtents_DoesNotAllocate(t *testing.T) {
	var m MeasuredExtents
	m.Resize(100000)
	m.SetEstimate(48, true)
	i := 0
	allocs := testing.AllocsPerRun(100, func() {
		i = (i + 7919) % 100000
		m.Set(i, float32(i%90))
		_ = m.IndexAt(m.Offset(i) + 1)
		_ = m.Total()
		m.Forget((i + 1) % 100000)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func BenchmarkMeasuredExtents_IndexAt_100k(b *testing.B) {
	var m MeasuredExtents
	m.Resize(100000)
	m.SetEstimate(48, true)
	for i := 0; i < 100000; i += 3 {
		m.Set(i, float32(20+i%60))
	}
	total := m.Total()
	y := float64(0)
	b.ReportAllocs()
	for b.Loop() {
		y += 7777
		if y > total {
			y -= total
		}
		_ = m.Offset(m.IndexAt(y))
	}
}

func BenchmarkMeasuredExtents_Set_100k(b *testing.B) {
	var m MeasuredExtents
	m.Resize(100000)
	m.SetEstimate(48, true)
	i, k := 0, 0
	b.ReportAllocs()
	for b.Loop() {
		i = (i + 7919) % 100000
		k++
		m.Set(i, float32(20+k%61))
	}
}
