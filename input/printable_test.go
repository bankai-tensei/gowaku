package input

import "testing"

func TestKeyCode_Printable(t *testing.T) {
	for _, k := range []KeyCode{KeySpace, KeyA, KeyZ, Key0, KeyMinus, KeyBackquote, KeyAt, KeyHash} {
		if !k.Printable() {
			t.Errorf("%v types a character, Printable says no", k)
		}
	}
	for _, k := range []KeyCode{KeyNone, KeyTab, KeyEnter, KeyEscape, KeyBackspace, KeyLeft, KeyDelete, KeyF1, KeyAlt} {
		if k.Printable() {
			t.Errorf("%v types nothing, Printable says yes", k)
		}
	}
}
