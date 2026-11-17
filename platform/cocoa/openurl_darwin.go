//go:build darwin && arm64

package cocoa

import (
	"errors"

	"github.com/ebitengine/purego/objc"
)

// OpenURL hands u to NSWorkspace, which opens it with the handler of its
// scheme: a web page in the default browser, mailto: in Mail. It needs no
// window and may be called on any goroutine, as NSWorkspace may be used on any
// thread. u is not checked here; gowaku.OpenURL checks it.
func OpenURL(u string) error {
	l, err := loadLibs()
	if err != nil {
		return err
	}
	l.pool(func() {
		url := class("NSURL").Send(sel("URLWithString:"), nsString(u))
		if url == 0 {
			err = errors.New("cocoa: NSURL does not take the url")
			return
		}
		workspace := class("NSWorkspace").Send(sel("sharedWorkspace"))
		if !objc.Send[bool](workspace, sel("openURL:"), url) {
			err = errors.New("cocoa: NSWorkspace could not open the url")
		}
	})
	return err
}
