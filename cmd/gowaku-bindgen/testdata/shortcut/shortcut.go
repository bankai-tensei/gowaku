// Package shortcut is the example of cmd/gowaku-bindgen/README.md: the
// bindings of ../example-shortcut-v1.xml, made by gowaku-bindgen, and the code
// that uses them on a window of platform/wayland.
package shortcut

//go:generate go run ../.. -out shortcut_linux.go ../example-shortcut-v1.xml
