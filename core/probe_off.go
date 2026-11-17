//go:build !gowakudebug

package core

// probing is false outside -tags gowakudebug, so that the compiler drops every
// call to a Probe.
const probing = false
