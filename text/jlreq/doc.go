// Package jlreq breaks Japanese text into lines by the rules of "Requirements
// for Japanese Text Layout" (W3C JLReq): line start and line end prohibitions,
// unbreakable sequences, squeezing of adjacent punctuation, hanging punctuation
// and the space between Japanese and Western text.
//
// The package works on Units, which carry a rune and a measured advance, and
// knows nothing about fonts. That keeps every rule testable with made-up
// metrics.
//
// jlreq imports the standard library only.
package jlreq
