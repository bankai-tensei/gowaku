package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/internal/testutil"
)

func TestWindowHandle_CloseFromTheOnCloseOfAnother(t *testing.T) {
	p, b := newFakes(t)
	closed := 0
	var second, third *WindowHandle
	p.script = []func(){
		func() {
			var err error
			if third, err = OpenWindow(WindowOptions{OnClose: func() { closed++ }}, counter{}); err != nil {
				t.Fatal(err)
			}
			if second, err = OpenWindow(WindowOptions{OnClose: func() { third.Close() }}, counter{}); err != nil {
				t.Fatal(err)
			}
			second.Close()
			p.Wake()
		},
		func() {
			if alive(2) || alive(3) {
				t.Error("a window closed from the OnClose of another must go as well")
			}
			Quit()
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if closed != 1 || b.destroyed != 3 {
		t.Errorf("closed %d, destroyed %d; want 1, 3", closed, b.destroyed)
	}
}
