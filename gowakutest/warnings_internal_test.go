package gowakutest

import (
	"strings"
	"testing"
)

func TestContrastDetail_NeverPrintsTheNeed(t *testing.T) {
	for _, c := range []struct {
		ratio, need float64
		want        string
	}{
		{4.4999, 4.5, "4.49:1"},
		{4.4951, 4.5, "4.49:1"},
		{2.999, 3, "2.99:1"},
		{1.5, 4.5, "1.50:1"},
	} {
		if got := contrastDetail(c.ratio, c.need); !strings.Contains(got, c.want) {
			t.Errorf("contrastDetail(%v, %v) = %q, want it to say %s", c.ratio, c.need, got, c.want)
		}
	}
}
