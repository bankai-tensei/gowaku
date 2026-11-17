// Package wtest holds the test helpers that the widget packages share: trees
// with a shaper, frame stepping, display list inspection, semantics queries,
// synthetic input and a fake platform host.
//
// It depends only on the core level packages, so the tests of every widget
// tier can import it without a cycle.
package wtest
