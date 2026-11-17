package markdown

import "strings"

// splitRow splits a row of a GFM table into its cells. The pipes at the
// start and the end are optional, and an escaped pipe \| belongs to the cell
// as a literal |, even inside code.
func splitRow(s string) []string {
	s = strings.Trim(s, " \t")
	var cells []string
	var cell strings.Builder
	leading := strings.HasPrefix(s, "|")
	trailing := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && s[i+1] == '|':
			cell.WriteByte('|')
			i++
			trailing = false
		case c == '\\' && i+1 < len(s):
			cell.WriteByte(c)
			cell.WriteByte(s[i+1])
			i++
			trailing = false
		case c == '|':
			cells = append(cells, strings.Trim(cell.String(), " \t"))
			cell.Reset()
			trailing = true
		default:
			cell.WriteByte(c)
			trailing = false
		}
	}
	if !trailing {
		cells = append(cells, strings.Trim(cell.String(), " \t"))
	}
	if leading && len(cells) > 0 {
		cells = cells[1:]
	}
	return cells
}

// hasPipe reports whether s has a pipe that is not escaped.
func hasPipe(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '|':
			return true
		}
	}
	return false
}

// parseDelimiterRow reads the row under the header of a table, such as
// "| :--- | :---: | ---: |", into the alignments of the columns.
func parseDelimiterRow(s string) ([]align, bool) {
	if strings.TrimSpace(s) == "" {
		return nil, false
	}
	cells := splitRow(s)
	if len(cells) == 0 {
		return nil, false
	}
	aligns := make([]align, len(cells))
	for i, c := range cells {
		left := strings.HasPrefix(c, ":")
		right := strings.HasSuffix(c, ":") && len(c) > 1
		dashes := strings.TrimSuffix(strings.TrimPrefix(c, ":"), ":")
		if dashes == "" || strings.Trim(dashes, "-") != "" {
			return nil, false
		}
		switch {
		case left && right:
			aligns[i] = alignCenter
		case left:
			aligns[i] = alignLeft
		case right:
			aligns[i] = alignRight
		}
	}
	return aligns, true
}
