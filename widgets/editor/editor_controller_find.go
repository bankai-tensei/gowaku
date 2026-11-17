package editor

// refind searches the whole text for the query anew: at once for a short
// text, on a goroutine of its own for a long one or a file.
func (c *Controller) refind() {
	c.stopFind()
	if !c.find.active() || !async(&c.buf) {
		c.find.all(&c.buf)
		return
	}
	f := &c.find
	f.matches = f.matches[:0]
	j := &findJob{
		q:    finder{query: f.query, opts: f.opts, re: f.re, lower: f.lower},
		wake: c.wakeFind,
	}
	f.job, f.log = j, nil
	j.queue(c.buf.segments(0)...)
}

// stopFind ends the search goroutine, whose reports are then dropped.
func (c *Controller) stopFind() {
	if j := c.find.job; j != nil {
		j.stop.Store(true)
		c.find.job, c.find.log = nil, nil
	}
}

// wakeFind asks the UI goroutine to take the matches the search goroutine
// found. It runs on that goroutine.
func (c *Controller) wakeFind() {
	if p := c.post.Load(); p != nil {
		(*p)(c.applyFind)
	}
}

// applyFind takes the matches the search goroutine found, on the UI
// goroutine.
func (c *Controller) applyFind() {
	f := &c.find
	j := f.job
	if j == nil {
		return
	}
	reports := j.take()
	if len(reports) == 0 {
		return
	}
	for _, r := range reports {
		if r.done {
			continue
		}
		f.matches = merge(f.matches, moveThrough(r.matches, f.log[r.logAt:]))
	}
	if len(f.matches) >= maxMatches {
		f.matches = f.matches[:maxMatches]
		j.stop.Store(true)
	}
	if !j.busy() {
		f.job, f.log = nil, nil
	}
	c.current = f.index(c.Selection())
	c.findChanged()
	for _, v := range c.views {
		if v.r != nil && v.r.state != nil {
			v.r.state.findProgress()
		}
	}
}

// findGrown searches the lines the source grew by, from line from to line to of
// the source, which the change ch added at the end of the text.
func (c *Controller) findGrown(ch lineChange, from, to int) {
	f := &c.find
	if !f.active() {
		return
	}
	if !async(&c.buf) {
		f.splice(&c.buf, ch)
		return
	}
	n := to - from
	first := ch.first + ch.added - n
	// The lines searched again, as the last line an append reopened: their
	// matches go, and so do those the search goroutine still holds for them.
	if old := ch.first + ch.removed; first < old {
		f.matches = f.matches[:f.firstInLine(first)]
		if f.job != nil {
			f.log = append(f.log, lineChange{first: first, removed: old - first, added: old - first})
		}
	}
	seg := c.buf.sourceSeg(from, n, first, len(f.log))
	if f.job == nil {
		f.job = &findJob{q: finder{query: f.query, opts: f.opts, re: f.re, lower: f.lower}, wake: c.wakeFind}
		f.log = nil
		seg.logAt = 0
	}
	f.job.queue(seg)
}

// Searching reports whether a search of a long text or a file still runs on a
// goroutine of its own, so that Matches does not hold all the matches yet.
func (c *Controller) Searching() bool { return c.find.job != nil }
