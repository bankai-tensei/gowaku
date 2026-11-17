// Package text turns strings into positioned glyphs: font lookup with fallback,
// shaping, Japanese line breaking through text/jlreq, cursor and selection
// geometry, and the glyph atlas.
//
// The third-party typesetting library stays behind this package. The
// result of a Layout is a list of paint.GlyphRun, so paint needs no fonts.
//
// text imports paint for its types, and geom. It must not import gpu, platform,
// core or anything above them. Nothing here is safe for concurrent use: a
// Shaper and everything made from it belong to the UI goroutine.
package text
