package text

// GraphemeBounds appends to dst the byte offsets of content at which its
// grapheme clusters start, followed by len(content), and returns the result.
// They are the offsets a caret can stop at before any shaping. An obscured
// field draws one mark for each cluster between two of them.
//
// An empty content gives just 0. It allocates, so it is meant for edits
// rather than for every frame.
func (s *Shaper) GraphemeBounds(content string, dst []int) []int {
	dst = append(dst, 0)
	if content == "" {
		return dst
	}
	runes, offsets := decode(content, 0)
	s.breaks.Init(runes)
	for it := s.breaks.GraphemeIterator(); it.Next(); {
		g := it.Grapheme()
		dst = append(dst, offsets[g.Offset+len(g.Text)])
	}
	return dst
}
