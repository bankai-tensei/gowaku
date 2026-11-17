package wtest

import (
	"strings"
	"testing"
)

// ExpectPanic fails the test unless f panics with a message containing want.
func ExpectPanic(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		got, _ := recover().(string)
		if !strings.Contains(got, want) {
			t.Errorf("got panic %q, want one containing %q", got, want)
		}
	}()
	f()
}

// CityNames is the list the autocomplete tests offer.
func CityNames() []string {
	return []string{"Kyoto", "Tokyo", "Toyama", "Kobe", "Otaru"}
}
