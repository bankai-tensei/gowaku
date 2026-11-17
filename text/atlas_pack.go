package text

import "image"

// noItem stands for no item: an image with nothing to draw, or the end of a list.
const noItem int32 = -1

// atlasEvictScan bounds how many of the least recently used items place looks
// through for one in a shelf of the right height, before it takes a whole
// shelf instead.
const atlasEvictScan = 1024

// Kinds of atlasItem.
const (
	itemFree uint8 = iota
	itemGlyph
	itemShape
	// itemOrphan is an image that no key maps to any more, kept until no
	// frame can be drawing it.
	itemOrphan
)

// atlasItem is an image on a page. The items of a pageSet form a list, the
// most recently used first.
type atlasItem struct {
	img   GlyphImage
	glyph GlyphKey
	shape shapeKey
	// shelf is the index of the shelf on the page of img.
	shelf int32
	// prev and next are the neighbors in the list of the page set.
	prev, next int32
	lastUsed   uint64
	kind       uint8
}

// freeRun is a free run of a shelf.
type freeRun struct {
	x, width int
}

// atlasShelf is a band of a page that holds images of about its height side by side.
// A shelf with no images is an empty band that a new shelf can take, and a dead
// one has been merged into a neighbor; its index waits to be reused.
type atlasShelf struct {
	y, height int
	// x is where the images end: the room from x to the right is free.
	x int
	// free are the runs left of x that evicted images gave back. They never
	// touch one another or x.
	free []freeRun
	// live counts the images.
	live     int
	lastUsed uint64
	dead     bool
}

// suits reports whether an image of height h belongs on s.
func (s *atlasShelf) suits(h int) bool { return h <= s.height && h*5 >= s.height*4 }

// take finds room for an image of width w, first in the runs given back and
// then right of the images.
func (s *atlasShelf) take(w, size int) (int, bool) {
	for j := range s.free {
		f := &s.free[j]
		if f.width < w {
			continue
		}
		x := f.x
		f.x += w
		f.width -= w
		if f.width == 0 {
			s.free[j] = s.free[len(s.free)-1]
			s.free = s.free[:len(s.free)-1]
		}
		return x, true
	}
	if s.x+w > size {
		return 0, false
	}
	x := s.x
	s.x += w
	return x, true
}

// release gives the run from x of width w back to s.
func (s *atlasShelf) release(x, w int) {
	lo, hi := x, x+w
	for j := 0; j < len(s.free); {
		f := s.free[j]
		if f.x+f.width == lo || f.x == hi {
			lo, hi = min(lo, f.x), max(hi, f.x+f.width)
			s.free[j] = s.free[len(s.free)-1]
			s.free = s.free[:len(s.free)-1]
			continue
		}
		j++
	}
	if hi == s.x {
		s.x = lo
		return
	}
	s.free = append(s.free, freeRun{x: lo, width: hi - lo})
}

type atlasPage struct {
	pix      []byte
	shelves  []atlasShelf
	nextY    int
	dirty    image.Rectangle
	lastUsed uint64
}

// pageSet is a list of pages of one pixel format.
type pageSet struct {
	pages []*atlasPage
	// bytes is the size of a texel, and max the number of pages to keep.
	bytes, max int
	color      bool
	// head is the item used most recently, and tail the one used least.
	head, tail int32
}

// setOf returns the page set of img.
func (a *Atlas) setOf(img *GlyphImage) *pageSet {
	if img.Color {
		return &a.color
	}
	return &a.gray
}

// newItem keeps img, placed on shelf, as an item used this frame.
func (a *Atlas) newItem(img GlyphImage, shelf int32) int32 {
	var i int32
	if n := len(a.freeItems); n > 0 {
		i = a.freeItems[n-1]
		a.freeItems = a.freeItems[:n-1]
	} else {
		i = int32(len(a.items))
		a.items = append(a.items, atlasItem{})
	}
	a.items[i] = atlasItem{img: img, shelf: shelf, lastUsed: a.frame}
	set := a.setOf(&img)
	a.pushFront(set, i)
	p := set.pages[img.Page]
	p.lastUsed = a.frame
	p.shelves[shelf].lastUsed = a.frame
	return i
}

// touch marks item i as used this frame.
func (a *Atlas) touch(i int32) {
	it := &a.items[i]
	if it.lastUsed == a.frame {
		return
	}
	it.lastUsed = a.frame
	set := a.setOf(&it.img)
	p := set.pages[it.img.Page]
	p.lastUsed = a.frame
	p.shelves[it.shelf].lastUsed = a.frame
	if set.head != i {
		a.unlink(set, i)
		a.pushFront(set, i)
	}
}

func (a *Atlas) pushFront(set *pageSet, i int32) {
	it := &a.items[i]
	it.prev, it.next = noItem, set.head
	if set.head != noItem {
		a.items[set.head].prev = i
	} else {
		set.tail = i
	}
	set.head = i
}

func (a *Atlas) unlink(set *pageSet, i int32) {
	it := &a.items[i]
	if it.prev != noItem {
		a.items[it.prev].next = it.next
	} else {
		set.head = it.next
	}
	if it.next != noItem {
		a.items[it.next].prev = it.prev
	} else {
		set.tail = it.prev
	}
	it.prev, it.next = noItem, noItem
}

// orphan lets go of item i, whose key now maps to another image. A frame may
// still be drawing it, in which case it waits for its turn to be evicted.
func (a *Atlas) orphan(i int32) {
	it := &a.items[i]
	if it.lastUsed != a.frame {
		a.evict(a.setOf(&it.img), i)
		return
	}
	it.kind = itemOrphan
}

// evict forgets item i of set and gives its room back. The pixels stay: an
// image written later replaces them, and nothing reads the rest, since a glyph
// reads only its own texels.
func (a *Atlas) evict(set *pageSet, i int32) {
	it := &a.items[i]
	switch it.kind {
	case itemGlyph:
		delete(a.entries, it.glyph)
	case itemShape:
		if e, ok := a.shapes[it.shape]; ok && e.item == i {
			delete(a.shapes, it.shape)
		}
	}
	p := set.pages[it.img.Page]
	p.release(int(it.shelf), it.img.X, it.img.Width)
	a.unlink(set, i)
	it.kind = itemFree
	a.freeItems = append(a.freeItems, i)
}

// place finds room for a width by height image in set: where there is some,
// on a new page while the set is below its limit, or else where the least
// recently used images were.
func (a *Atlas) place(set *pageSet, width, height int) (page int, shelf int32, x, y int) {
	for i, p := range set.pages {
		if s, x, y, ok := p.allocate(width, height, a.size); ok {
			return i, s, x, y
		}
	}
	if len(set.pages) >= set.max {
		if page, s, x, y, ok := a.reclaim(set, width, height); ok {
			return page, s, x, y
		}
		// Every image is in use by this frame, which only a text far larger
		// than the screen does. Such a frame gets a page beyond the limit.
	}
	p := &atlasPage{pix: make([]byte, a.size*a.size*set.bytes)}
	set.pages = append(set.pages, p)
	s, x, y, _ := p.allocate(width, height, a.size)
	return len(set.pages) - 1, s, x, y
}

// reclaim makes room for a width by height image in a full set by evicting
// images that the frame does not use. It tries, in turn:
//
//   - the least recently used images on shelves of about the right height, one
//     at a time, until one of them leaves room;
//   - the band of whole shelves unused for the longest that is tall enough,
//     up to a whole page.
func (a *Atlas) reclaim(set *pageSet, width, height int) (page int, shelf int32, x, y int, ok bool) {
	scanned := 0
	for i := set.tail; i != noItem && scanned < atlasEvictScan; scanned++ {
		it := &a.items[i]
		if it.lastUsed == a.frame {
			// The rest of the list is in use too.
			break
		}
		prev := it.prev
		page := it.img.Page
		p := set.pages[page]
		if p.shelves[it.shelf].suits(height) {
			a.evict(set, i)
			if s, x, y, ok := p.allocate(width, height, a.size); ok {
				return page, s, x, y, true
			}
		}
		i = prev
	}

	// The band of shelves unused for the longest that is tall enough. The
	// shelves of a page tile it from the top to nextY, and the room below is
	// free.
	bestPage, bestTop, bestBottom := -1, 0, 0
	var bestNewest uint64
	for pi, p := range set.pages {
		order := a.byY(p)
		for start := range order {
			var newest uint64
			for end := start; end < len(order); end++ {
				s := &p.shelves[order[end]]
				if s.live > 0 {
					if s.lastUsed == a.frame {
						break
					}
					newest = max(newest, s.lastUsed)
				}
				bottom := s.y + s.height
				if end == len(order)-1 {
					bottom = a.size
				}
				if top := p.shelves[order[start]].y; bottom-top >= height {
					if bestPage < 0 || newest < bestNewest {
						bestPage, bestTop, bestBottom, bestNewest = pi, top, s.y+s.height, newest
					}
					break
				}
			}
		}
	}
	if bestPage < 0 {
		return 0, 0, 0, 0, false
	}
	p := set.pages[bestPage]
	for i := set.tail; i != noItem; {
		it := &a.items[i]
		prev := it.prev
		if it.img.Page == bestPage {
			if y := p.shelves[it.shelf].y; y >= bestTop && y < bestBottom {
				a.evict(set, i)
			}
		}
		i = prev
	}
	s, x, y, ok := p.allocate(width, height, a.size)
	return bestPage, s, x, y, ok
}

// byY returns the indices of the shelves of p that are not dead, from the top.
// It reuses the storage of the last call.
func (a *Atlas) byY(p *atlasPage) []int32 {
	order := a.order[:0]
	for i := range p.shelves {
		if p.shelves[i].dead {
			continue
		}
		order = append(order, int32(i))
		for j := len(order) - 1; j > 0 && p.shelves[order[j]].y < p.shelves[order[j-1]].y; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	a.order = order
	return order
}

// allocate finds room on the page: in a shelf of about the right height, in an
// empty band, or in a new shelf below the others.
func (p *atlasPage) allocate(width, height, size int) (shelf int32, x, y int, ok bool) {
	band := -1
	for i := range p.shelves {
		s := &p.shelves[i]
		if s.dead {
			continue
		}
		if s.live == 0 {
			if height <= s.height && (band < 0 || s.height < p.shelves[band].height) {
				band = i
			}
			continue
		}
		if s.suits(height) {
			if x, ok := s.take(width, size); ok {
				s.live++
				return int32(i), x, s.y, true
			}
		}
	}
	if band >= 0 {
		if s := &p.shelves[band]; !s.suits(height) {
			// The rest of a tall band stays free for another shelf.
			rest := atlasShelf{y: s.y + height, height: s.height - height}
			s.height = height
			p.addShelf(rest)
		}
		s := &p.shelves[band]
		s.x, s.live = width, 1
		return int32(band), 0, s.y, true
	}
	if p.nextY+height > size || width > size {
		return 0, 0, 0, false
	}
	y = p.nextY
	p.nextY += height
	i := p.addShelf(atlasShelf{y: y, height: height, x: width, live: 1})
	return i, 0, y, true
}

// addShelf adds s in the place of a dead shelf, or at the end.
func (p *atlasPage) addShelf(s atlasShelf) int32 {
	for i := range p.shelves {
		if p.shelves[i].dead {
			s.free = p.shelves[i].free[:0]
			p.shelves[i] = s
			return int32(i)
		}
	}
	p.shelves = append(p.shelves, s)
	return int32(len(p.shelves) - 1)
}

// release gives back the run from x of width w on shelf i. A shelf left with no
// images becomes an empty band, merged with the empty bands next to it, and
// given back to the page if it is the last.
func (p *atlasPage) release(i, x, w int) {
	s := &p.shelves[i]
	s.live--
	if s.live > 0 {
		s.release(x, w)
		return
	}
	s.x = 0
	s.free = s.free[:0]
	for merged := true; merged; {
		merged = false
		for j := range p.shelves {
			o := &p.shelves[j]
			if j == i || o.dead || o.live > 0 {
				continue
			}
			switch {
			case o.y+o.height == s.y:
				s.y = o.y
			case s.y+s.height == o.y:
			default:
				continue
			}
			s.height += o.height
			s.lastUsed = max(s.lastUsed, o.lastUsed)
			o.dead = true
			merged = true
		}
	}
	if s.y+s.height == p.nextY {
		p.nextY = s.y
		s.dead = true
	}
}
