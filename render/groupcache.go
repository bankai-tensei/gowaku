package render

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// A cached group (paint.GroupStyle.Cache) keeps its picture, the target its last
// pass wrote, out of the pool of groups, and later frames draw that picture
// instead of running the jobs again, for as long as the version of its
// paint.GroupCache, the scale of the surface and that of the canvas stay. Moving
// the group moves the picture by whole physical pixels. See
// docs/ADR/0115-cached-groups.md.

// groupCacheBudget bounds the bytes of the pictures that cached groups keep:
// eight pictures of 2048 by 2048.
const groupCacheBudget = 128 << 20

// cachedGroup is the picture of a cached group.
type cachedGroup struct {
	key *paint.GroupCache
	// tex is the picture, nil until a draw keeps one, and r where it lay on
	// the surface when origin was the origin of the group.
	tex    *groupTexture
	r      groupRect
	origin geom.Offset
	// What the picture was drawn for.
	version        uint64
	scale, opScale float32
	lastUsed       uint64
}

// fits reports whether e was drawn for the group of op at version, on a surface
// of scale.
func (e *cachedGroup) fits(version uint64, op *paint.Op, scale float32) bool {
	return e.version == version && e.scale == scale && e.opScale == op.Scale
}

// moved returns where the picture of e lies for the group of op: moved by
// whole physical pixels as the group moved, rounded as a layer is.
func (e *cachedGroup) moved(op *paint.Op, scale float32) groupRect {
	r := e.r
	r.x += int(geom.Floor32((op.Origin.X-e.origin.X)*scale + 0.5))
	r.y += int(geom.Floor32((op.Origin.Y-e.origin.Y)*scale + 0.5))
	return r
}

// bytes returns what the picture of e holds on the GPU.
func (e *cachedGroup) bytes() int {
	if e.tex == nil {
		return 0
	}
	return e.tex.width * e.tex.height * 4
}

// rememberGroup records that the group of op at version was drawn without
// keeping a picture, so that the next draw of the same keeps one. It returns
// the entry of key, made if e is nil.
func (g *Resources) rememberGroup(key *paint.GroupCache, e *cachedGroup, version uint64, op *paint.Op, scale float32) *cachedGroup {
	if e == nil {
		e = &cachedGroup{key: key}
		if g.cachedGroups == nil {
			g.cachedGroups = make(map[*paint.GroupCache]*cachedGroup)
		}
		g.cachedGroups[key] = e
		g.cachedList = append(g.cachedList, e)
	}
	g.retire(e)
	e.version, e.scale, e.opScale, e.origin = version, scale, op.Scale, op.Origin
	e.lastUsed = g.frame
	return e
}

// keepGroup takes picture, which the group of op drew at r, out of the pool as
// the picture of key, unless the budget has no room for it even after the
// pictures not drawn this frame make way.
func (g *Resources) keepGroup(key *paint.GroupCache, e *cachedGroup, picture *groupTexture, r groupRect, version uint64, op *paint.Op, scale float32) {
	e = g.rememberGroup(key, e, version, op, scale)
	need := picture.width * picture.height * 4
	for g.groupCacheBytes+need > groupCacheBudget {
		var oldest *cachedGroup
		for _, o := range g.cachedList {
			if o.tex != nil && o.lastUsed != g.frame && (oldest == nil || o.lastUsed < oldest.lastUsed) {
				oldest = o
			}
		}
		if oldest == nil {
			return
		}
		g.retire(oldest)
	}
	for i, t := range g.groupTextures {
		if t != picture {
			continue
		}
		last := len(g.groupTextures) - 1
		g.groupTextures[i] = g.groupTextures[last]
		g.groupTextures[last] = nil
		g.groupTextures = g.groupTextures[:last]
		break
	}
	picture.inUse = true
	e.tex, e.r = picture, r
	g.groupCacheBytes += need
}

// retire lets go of the picture of e. It goes back to the pool when the next
// frame begins: the frame in progress may still draw it.
func (g *Resources) retire(e *cachedGroup) {
	if e.tex == nil {
		return
	}
	g.groupCacheBytes -= e.bytes()
	g.retired = append(g.retired, e.tex)
	e.tex = nil
}

// beginGroupCache gives the retired pictures back to the pool and forgets the
// cached groups not drawn for groupIdle frames.
func (g *Resources) beginGroupCache() {
	for _, t := range g.retired {
		t.inUse, t.lastUsed = false, g.frame
		g.groupTextures = append(g.groupTextures, t)
	}
	clear(g.retired)
	g.retired = g.retired[:0]
	kept := g.cachedList[:0]
	for _, e := range g.cachedList {
		if e.lastUsed+groupIdle < g.frame {
			if e.tex != nil {
				g.groupCacheBytes -= e.bytes()
				e.tex.tex.Destroy()
				e.tex = nil
			}
			delete(g.cachedGroups, e.key)
			continue
		}
		kept = append(kept, e)
	}
	clear(g.cachedList[len(kept):])
	g.cachedList = kept
}

// destroyGroupCache releases the pictures of cached groups.
func (g *Resources) destroyGroupCache() {
	for _, e := range g.cachedList {
		if e.tex != nil {
			e.tex.tex.Destroy()
		}
	}
	for _, t := range g.retired {
		t.tex.Destroy()
	}
	g.cachedList, g.retired, g.groupCacheBytes = nil, nil, 0
	clear(g.cachedGroups)
}
