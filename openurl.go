package gowaku

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidURL means that OpenURL was given something other than an absolute
// URL: an empty string, a relative reference such as "example.com" or
// "/docs", a path such as `C:\docs`, or a string with spaces or control
// characters.
var ErrInvalidURL = errors.New("gowaku: not an absolute url")

// OpenURL opens u with the application that the system has for its scheme: a
// web page in the default browser, mailto: in the mail program, a file: URL
// in the application for the file. It returns once the system has the
// request, not when the application has opened, and may be called on any
// goroutine, with or without Run.
//
// u must be an absolute URL with a scheme of two or more letters; anything
// else is ErrInvalidURL. OpenURL does not restrict the scheme, and a scheme
// can do more than show a page: a file: URL of a program runs it on Windows,
// and other schemes start whatever registered them. An application that opens
// links it did not write, such as those of a document, should allow only the
// schemes it means to, as in:
//
//	if p, err := url.Parse(link); err == nil && (p.Scheme == "https" || p.Scheme == "http" || p.Scheme == "mailto") {
//		err = gowaku.OpenURL(link)
//	}
//
// It goes to ShellExecute on Windows, to NSWorkspace on macOS, and on Linux to
// the OpenURI portal of xdg-desktop-portal, then to xdg-open if the portal is
// missing or fails. No shell sees u. See docs/ADR/0116-keys-and-open-url.md.
func OpenURL(u string) error {
	if err := checkURL(u); err != nil {
		return err
	}
	return urlOpeners().open(u)
}

// checkURL accepts an absolute URL: a scheme and something after it, and for
// a scheme with an authority ("https://"), a host.
func checkURL(u string) error {
	if u == "" {
		return fmt.Errorf("%w: empty", ErrInvalidURL)
	}
	if i := strings.IndexFunc(u, func(r rune) bool { return r <= ' ' || r == 0x7F }); i >= 0 {
		return fmt.Errorf("%w: %q has a space or a control character", ErrInvalidURL, u)
	}
	p, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	switch {
	case p.Scheme == "":
		return fmt.Errorf("%w: %q has no scheme", ErrInvalidURL, u)
	case len(p.Scheme) == 1:
		// A drive letter, as in C:\docs or C:/docs, parses as a scheme.
		return fmt.Errorf("%w: %q is a path", ErrInvalidURL, u)
	case p.Opaque == "" && p.Host == "" && p.Path == "":
		return fmt.Errorf("%w: %q has nothing after the scheme", ErrInvalidURL, u)
	case strings.HasPrefix(u[len(p.Scheme)+1:], "//") && p.Host == "" && p.Scheme != "file":
		return fmt.Errorf("%w: %q has no host", ErrInvalidURL, u)
	}
	return nil
}

// openers are the ways to open a URL, in order of preference.
type openers []func(u string) error

// open tries each way until one takes u, and returns the errors of all of them
// if none does.
func (o openers) open(u string) error {
	if len(o) == 0 {
		return ErrUnsupported
	}
	var errs []error
	for _, open := range o {
		err := open(u)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return fmt.Errorf("gowaku: open url: %w", errors.Join(errs...))
}
