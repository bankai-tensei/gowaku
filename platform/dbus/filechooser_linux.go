package dbus

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/platform"
)

const (
	fileChooserInterface = "org.freedesktop.portal.FileChooser"
	requestInterface     = "org.freedesktop.portal.Request"
	// responseCancelled is what the Response signal carries when the user
	// closed the dialog.
	responseOK        = 0
	responseCancelled = 1
)

var lastToken atomic.Uint64

// ShowFileDialog asks the portal for files and blocks until the user is done:
// call it from a goroutine of its own. parent identifies the window to stay
// in front of, as the portal wants it ("x11:XID" or "wayland:HANDLE"), or "".
// It returns nil paths if the user canceled, and platform.ErrNoFileDialog
// without a bus or a portal.
func ShowFileDialog(parent string, opts platform.FileDialogOptions) ([]string, error) {
	paths, err := showFileDialog(parent, opts)
	if errors.Is(err, ErrUnavailable) {
		err = platform.ErrNoFileDialog
	}
	return paths, err
}

func showFileDialog(parent string, opts platform.FileDialogOptions) ([]string, error) {
	c := session()
	if c == nil {
		return nil, ErrUnavailable
	}
	token := fmt.Sprintf("gowaku%d", lastToken.Add(1))
	// The request object has a path that the call returns, but the response
	// can come before the return. The path is predictable, so the match goes
	// first (the Request documentation of the portal).
	sender := strings.ReplaceAll(strings.TrimPrefix(c.Names()[0], ":"), ".", "_")
	path := godbus.ObjectPath(portalPath + "/request/" + sender + "/" + token)
	match := []godbus.MatchOption{
		godbus.WithMatchObjectPath(path),
		godbus.WithMatchInterface(requestInterface),
		godbus.WithMatchMember("Response"),
	}
	if err := c.AddMatchSignal(match...); err != nil {
		return nil, fmt.Errorf("dbus: watch file chooser: %w", err)
	}
	defer c.RemoveMatchSignal(match...) //nolint:errcheck // the bus forgets it with the connection anyway
	signals := make(chan *godbus.Signal, 4)
	c.Signal(signals)
	defer c.RemoveSignal(signals)

	method, options := fileChooserRequest(token, opts)
	r, err := call(c, fileChooserInterface+"."+method, parent, opts.Title, options)
	if err != nil {
		return nil, err
	}
	var handle godbus.ObjectPath
	if err := r.Store(&handle); err != nil {
		return nil, fmt.Errorf("dbus: file chooser: %w", err)
	}
	for s := range signals {
		if (s.Path != path && s.Path != handle) || s.Name != requestInterface+".Response" || len(s.Body) != 2 {
			continue
		}
		code, _ := s.Body[0].(uint32)
		results, _ := s.Body[1].(map[string]godbus.Variant)
		switch code {
		case responseOK:
			uris, _ := results["uris"].Value().([]string)
			return pathsOf(uris), nil
		case responseCancelled:
			return nil, nil
		default:
			return nil, fmt.Errorf("dbus: file chooser ended with %d", code)
		}
	}
	return nil, fmt.Errorf("dbus: connection closed during the file chooser")
}

// fileFilter is the (sa(us)) of a filter: its name, and patterns as (0, glob).
type fileFilter struct {
	Name     string
	Patterns []filterPattern
}

type filterPattern struct {
	Kind    uint32
	Pattern string
}

// fileChooserRequest returns the method and options of a request.
func fileChooserRequest(token string, opts platform.FileDialogOptions) (string, map[string]godbus.Variant) {
	options := map[string]godbus.Variant{
		"handle_token": godbus.MakeVariant(token),
		"modal":        godbus.MakeVariant(true),
	}
	if len(opts.Filters) > 0 {
		filters := make([]fileFilter, 0, len(opts.Filters))
		for _, f := range opts.Filters {
			ff := fileFilter{Name: f.Name}
			for _, p := range f.Patterns {
				ff.Patterns = append(ff.Patterns, filterPattern{0, p})
			}
			filters = append(filters, ff)
		}
		options["filters"] = godbus.MakeVariant(filters)
	}
	if opts.Folder != "" {
		// A NUL-terminated byte string, as paths need not be UTF-8.
		options["current_folder"] = godbus.MakeVariant(append([]byte(opts.Folder), 0))
	}
	if opts.Save {
		if opts.Name != "" {
			options["current_name"] = godbus.MakeVariant(opts.Name)
		}
		return "SaveFile", options
	}
	options["multiple"] = godbus.MakeVariant(opts.Multiple)
	options["directory"] = godbus.MakeVariant(opts.Directory)
	return "OpenFile", options
}

// pathsOf turns file URIs into paths.
func pathsOf(uris []string) []string {
	paths := make([]string, 0, len(uris))
	for _, u := range uris {
		if p, ok := pathOfURI(u); ok {
			paths = append(paths, p)
		}
	}
	return paths
}
