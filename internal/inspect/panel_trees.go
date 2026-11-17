//go:build gowakudebug

package inspect

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// TreeKind selects the tree a TreesPanel shows.
type TreeKind uint8

// The trees.
const (
	TreeWidgets TreeKind = iota
	TreeRenderObjects
	TreeSemantics
)

const (
	// openLevels is how many branching levels below the root a tree opens when
	// it first shows, and openBudget how many nodes it opens at most.
	openLevels = 2
	openBudget = 300
	// semanticsLife is how long the Semantics tree keeps a walk of the
	// accessibility tree before it walks it again, as the rebuilds of the
	// Inspector follow the frames of the window.
	semanticsLife = 250 * time.Millisecond
)

// TreesPanel shows one tree of Target, in step with the selection: a row that
// is clicked selects its node in the session, and a selection made elsewhere,
// by a pick or in another panel, opens the ancestors of its node and scrolls
// the node into view. A search field above the tree finds a node by its label.
type TreesPanel struct {
	core.StatefulBase
	Session *Session
	Target  Target
	Kind    TreeKind
}

// CreateState implements core.StatefulWidget.
func (TreesPanel) CreateState() core.State { return &treesState{} }

type treesState struct {
	core.StateOf[TreesPanel]
	cancel func()
	// target and kind are those the controllers were made for.
	target Target
	kind   TreeKind
	// Only the controller of the kind shown is set.
	elements *collection.TreeController[core.ElementView]
	objects  *collection.TreeController[core.RenderObject]
	nodes    *collection.TreeController[semKey]
	sem      *semIndex
	semAt    time.Duration
	// shown is the selection of the session the tree last followed, and
	// selecting says that the tree itself is changing it.
	shownT    Target
	shownEl   core.ElementView
	shownRO   core.RenderObject
	selecting bool
	query     string
	// missed says that nothing matches the query.
	missed bool
	// rows says whether the widget tree shows the inner workings of gowaku,
	// and view is the row it shows at the top, or the zero view for the top of
	// the tree. See trees_summary.go.
	rows summary
	view core.ElementView
	// objView is the render object the render tree shows at the top, or nil
	// for the root.
	objView core.RenderObject
}

// InitState implements core.State.
func (s *treesState) InitState() {
	s.reset()
	s.cancel = s.Widget().Session.Listen(s.follow)
}

// DidUpdateWidget implements core.State.
func (s *treesState) DidUpdateWidget(core.Widget) {
	if w := s.Widget(); w.Target != s.target || w.Kind != s.kind {
		s.reset()
	}
}

// Dispose implements core.State.
func (s *treesState) Dispose() { s.cancel() }

// reset makes the controllers anew for the target, opens the top of the tree,
// and follows the selection.
func (s *treesState) reset() {
	w := s.Widget()
	s.target, s.kind = w.Target, w.Kind
	s.elements, s.objects, s.nodes, s.sem = nil, nil, nil, nil
	s.shownT, s.shownEl, s.shownRO = nil, core.ElementView{}, nil
	s.view = core.ElementView{}
	s.objView = nil
	if w.Target == nil {
		return
	}
	tree := w.Target.Tree()
	budget := openBudget
	switch w.Kind {
	case TreeWidgets:
		c := &collection.TreeController[core.ElementView]{}
		s.elements = c
		for _, r := range s.rows.top(tree.RootElement()) {
			expandDefault(c.Expand, s.rows.children, r, openLevels, &budget)
		}
	case TreeRenderObjects:
		c := &collection.TreeController[core.RenderObject]{}
		s.objects = c
		if root := tree.RootRenderObject(); root != nil {
			expandDefault(c.Expand, objectChildren, root, openLevels, &budget)
		}
	case TreeSemantics:
		c := &collection.TreeController[semKey]{}
		s.nodes = c
		s.walkSemantics()
		expandDefault(c.Expand, s.semChildren, rootSem, openLevels+1, &budget)
	}
	s.follow()
}

// walkSemantics reads the accessibility tree of the target again.
func (s *treesState) walkSemantics() {
	s.sem = newSemIndex(s.target.Tree())
	s.semAt = s.Widget().Session.Now()
}

func (s *treesState) semChildren(k semKey) []semKey { return s.sem.children[k] }

// follow shows the selection of the session, if it changed: it selects the
// node, opens its ancestors and scrolls it into view.
func (s *treesState) follow() {
	t, el, ro := s.Widget().Session.Selection()
	if t == s.shownT && el == s.shownEl && ro == s.shownRO {
		return
	}
	s.shownT, s.shownEl, s.shownRO = t, el, ro
	if s.selecting || s.target == nil {
		return
	}
	mine := t == s.target
	switch s.kind {
	case TreeWidgets:
		if mine && el.IsZero() && ro != nil {
			el = t.Tree().ElementOf(ro)
		}
		if mine {
			el = s.rows.row(el)
		}
		if !mine || el.IsZero() {
			s.elements.ClearSelection()
			return
		}
		s.reroot(el)
		for p := s.rows.parent(el); !p.IsZero(); p = s.rows.parent(p) {
			s.elements.Expand(p)
		}
		s.elements.Select(el)
	case TreeRenderObjects:
		if !mine || ro == nil {
			s.objects.ClearSelection()
			return
		}
		s.rerootObject(ro)
		for p := ro.Parent(); p != nil; p = p.Parent() {
			s.objects.Expand(p)
		}
		s.objects.Select(ro)
	case TreeSemantics:
		if mine && ro != nil {
			// The object may be newer than the last walk.
			s.walkSemantics()
			if k, _, ok := s.sem.of(ro); ok && k != rootSem {
				for p, ok := s.sem.parent[k]; ok; p, ok = s.sem.parent[p] {
					s.nodes.Expand(p)
				}
				s.nodes.Select(k)
				return
			}
		}
		s.nodes.ClearSelection()
	}
}

// selectElement selects el in the session, without following the change back.
func (s *treesState) selectElement(el core.ElementView) {
	s.selecting = true
	s.Widget().Session.Select(s.target, el)
	s.selecting = false
}

// selectObject selects ro in the session, without following the change back.
func (s *treesState) selectObject(ro core.RenderObject) {
	s.selecting = true
	s.Widget().Session.SelectRenderObject(s.target, ro)
	s.selecting = false
}

// search finds the first node whose label holds q.
func (s *treesState) search(q string) {
	s.SetState(func() { s.query = q })
	if q != "" {
		s.find(q, false)
	} else {
		s.SetState(func() { s.missed = false })
	}
}

// searchNext finds the next node after the selected one whose label holds q.
func (s *treesState) searchNext(q string) {
	s.SetState(func() { s.query = q })
	if q != "" {
		s.find(q, true)
	}
}

// find selects the node whose label holds q, after the selection with next,
// or else from the top. The selection then shows by follow.
func (s *treesState) find(q string, next bool) {
	if s.target == nil {
		return
	}
	tree := s.target.Tree()
	_, el, ro := s.Widget().Session.Selection()
	ok := false
	switch s.kind {
	case TreeWidgets:
		var from core.ElementView
		if next {
			from = el
		}
		var found core.ElementView
		found, ok = findNext(s.rows.top(tree.RootElement()), s.rows.children, elementLabel, from, q)
		if ok {
			s.Widget().Session.Select(s.target, found)
		}
	case TreeRenderObjects:
		root := tree.RootRenderObject()
		if root == nil {
			break
		}
		var from core.RenderObject
		if next {
			from = ro
		}
		var found core.RenderObject
		found, ok = findNext([]core.RenderObject{root}, objectChildren, describe, from, q)
		if ok {
			s.Widget().Session.SelectRenderObject(s.target, found)
		}
	case TreeSemantics:
		s.walkSemantics()
		var from semKey
		if sel := s.nodes.Selection(); next && len(sel) == 1 {
			from = sel[0]
		}
		var found semKey
		found, ok = findNext(s.sem.children[rootSem], s.semChildren, s.semLabel, from, q)
		if ok {
			s.selectNode(found)
			s.nodes.Select(found)
			for p, has := s.sem.parent[found]; has; p, has = s.sem.parent[p] {
				s.nodes.Expand(p)
			}
		}
	}
	s.SetState(func() { s.missed = !ok })
}

func (s *treesState) semLabel(k semKey) string { return semLabel(s.sem.nodes[k]) }

// selectNode selects the object that opened the semantics node k.
func (s *treesState) selectNode(k semKey) {
	if ro := ownerOf(s.target.Tree().RootRenderObject(), k.id); ro != nil {
		s.selectObject(ro)
	}
}

// Build implements core.State.
func (s *treesState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	if w.Target == nil {
		return basic.SizedBox{}
	}
	th := basic.ThemeOf(ctx)
	str := stringsOf(ctx)
	var tree core.Widget
	switch s.kind {
	case TreeWidgets:
		tree = s.elementTree(th, str)
	case TreeRenderObjects:
		tree = s.objectTree(th, str)
	default:
		if w.Session.Now()-s.semAt >= semanticsLife {
			s.walkSemantics()
		}
		tree = s.semanticsTree(str)
	}
	var search core.Widget = field.Search{
		Value: s.query, Placeholder: str.findInTree,
		OnChange: s.search, OnSubmit: s.searchNext,
	}
	if s.kind == TreeWidgets {
		search = basic.Row{Spacing: theme.Space12, Cross: layout.CrossCenter, Children: []core.Widget{
			basic.Expanded{Child: search},
			control.Checkbox{Value: s.rows.all, Label: str.showInternal, OnChange: s.setAll},
		}}
	}
	children := []core.Widget{search}
	if s.kind == TreeWidgets && !s.view.IsZero() && s.view.Mounted() {
		children = append(children, s.trail(str))
	}
	if s.kind == TreeRenderObjects && s.objView != nil && s.target.Tree().HostedBy(s.objView) {
		children = append(children, s.objectTrail(str))
	}
	if s.missed && s.query != "" {
		children = append(children, basic.Padding{
			Insets: geom.Insets{Left: theme.Space8},
			Child:  basic.Text{Content: str.noMatch, Style: th.Text.Caption.WithColor(th.Color.Ink2)},
		})
	}
	children = append(children, basic.Expanded{Child: tree})
	return basic.Padding{
		Insets: geom.Insets{Top: theme.Space8},
		Child:  basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space8, Children: children},
	}
}

// reroot moves the top of the view so that row shows within maxViewDepth
// levels of it, as the indentation of a deep row leaves no room for its label.
func (s *treesState) reroot(row core.ElementView) {
	if !s.view.IsZero() && !s.view.Mounted() {
		s.view = core.ElementView{}
	}
	top := s.view
	if top.IsZero() {
		if tops := s.rows.top(s.target.Tree().RootElement()); len(tops) == 1 {
			top = tops[0]
		}
	}
	if d := s.rows.below(top, row); d >= 0 && d <= maxViewDepth {
		return
	}
	v := row
	for range viewContext {
		if p := s.rows.parent(v); !p.IsZero() {
			v = p
		}
	}
	s.view = v
}

// setView shows the tree from row down, or from the top with the zero view.
func (s *treesState) setView(row core.ElementView) {
	s.SetState(func() { s.view = row })
	if !row.IsZero() {
		s.elements.Expand(row)
	}
}

// setAll shows or hides the inner workings of gowaku in the widget tree.
func (s *treesState) setAll(all bool) {
	s.SetState(func() {
		s.rows.all = all
		if !s.view.IsZero() {
			s.view = s.rows.row(s.view)
		}
	})
	// Follow the selection again in the other tree.
	s.shownT, s.shownEl, s.shownRO = nil, core.ElementView{}, nil
	s.follow()
}

// trail is the breadcrumb above a view that starts below the top: the rows
// from the top down to the top of the view.
func (s *treesState) trail(str *inspectStrings) core.Widget {
	path := s.rows.path(s.view)
	items := make([]nav.BreadcrumbItem, len(path))
	for i, r := range path {
		items[i] = nav.BreadcrumbItem{Label: TypeName(r.Widget())}
	}
	return nav.Breadcrumb{
		Items: items, Label: str.viewTrail,
		OnSelect: func(i int) {
			if i == 0 {
				s.setView(core.ElementView{})
				return
			}
			s.setView(path[i])
		},
	}
}

func (s *treesState) elementTree(th *theme.Theme, str *inspectStrings) core.Widget {
	roots := s.rows.top(s.target.Tree().RootElement())
	if !s.view.IsZero() && s.view.Mounted() {
		roots = []core.ElementView{s.view}
	}
	faint := th.Color.Ink3
	return collection.Tree[core.ElementView]{
		Roots:       roots,
		ChildrenOf:  s.rows.children,
		HasChildren: s.rows.hasChildren,
		LabelOf:     elementLabel,
		IconOf:      func(v core.ElementView) core.Widget { return letterMark{letter: kindLetter(v.Kind()), color: faint} },
		Label:       str.widgetTree,
		Controller:  s.elements,
		OnSelectionChange: func(sel []core.ElementView) {
			if len(sel) == 1 {
				s.selectElement(sel[0])
			}
		},
	}
}

// rerootObject moves the top of the render tree so that ro shows within
// maxViewDepth levels of it, as reroot does for the widget tree.
func (s *treesState) rerootObject(ro core.RenderObject) {
	tree := s.target.Tree()
	if s.objView != nil && !tree.HostedBy(s.objView) {
		s.objView = nil
	}
	top := s.objView
	if top == nil {
		top = tree.RootRenderObject()
	}
	depth := 0
	for p := ro; p != nil && p != top; p = p.Parent() {
		depth++
	}
	if depth <= maxViewDepth && isAncestorOrSelf(top, ro) {
		return
	}
	v := ro
	for range viewContext {
		if p := v.Parent(); p != nil {
			v = p
		}
	}
	s.objView = v
}

// isAncestorOrSelf reports whether a is ro or lies above it.
func isAncestorOrSelf(a, ro core.RenderObject) bool {
	for p := ro; p != nil; p = p.Parent() {
		if p == a {
			return true
		}
	}
	return false
}

// objectTrail is the breadcrumb above a render tree shown from below its root.
func (s *treesState) objectTrail(str *inspectStrings) core.Widget {
	var path []core.RenderObject
	for p := s.objView; p != nil; p = p.Parent() {
		path = append(path, p)
	}
	items := make([]nav.BreadcrumbItem, len(path))
	for i := range path {
		items[len(path)-1-i] = nav.BreadcrumbItem{Label: TypeName(path[i])}
	}
	return nav.Breadcrumb{
		Items: items, Label: str.viewTrail,
		OnSelect: func(i int) {
			ro := path[len(path)-1-i]
			s.SetState(func() {
				if ro.Parent() == nil {
					s.objView = nil
				} else {
					s.objView = ro
				}
			})
			s.objects.Expand(ro)
		},
	}
}

func (s *treesState) objectTree(th *theme.Theme, str *inspectStrings) core.Widget {
	root := s.target.Tree().RootRenderObject()
	if root == nil {
		return basic.SizedBox{}
	}
	if s.objView != nil && s.target.Tree().HostedBy(s.objView) {
		root = s.objView
	}
	layer, boundary := th.Color.AccentInk, th.Color.Ink3
	return collection.Tree[core.RenderObject]{
		Roots:       []core.RenderObject{root},
		ChildrenOf:  objectChildren,
		HasChildren: objectHasChildren,
		LabelOf:     describe,
		IconOf: func(ro core.RenderObject) core.Widget {
			switch {
			case core.HasLayer(ro):
				return squareMark{color: layer, filled: true}
			case core.IsRelayoutBoundary(ro):
				return squareMark{color: boundary}
			}
			return nil
		},
		Label:      str.renderTree,
		Controller: s.objects,
		OnSelectionChange: func(sel []core.RenderObject) {
			if len(sel) == 1 {
				s.selectObject(sel[0])
			}
		},
	}
}

func (s *treesState) semanticsTree(str *inspectStrings) core.Widget {
	return collection.Tree[semKey]{
		Roots:       s.sem.children[rootSem],
		ChildrenOf:  s.semChildren,
		HasChildren: func(k semKey) bool { return len(s.sem.children[k]) > 0 },
		LabelOf:     s.semLabel,
		Label:       str.semanticsTree,
		Controller:  s.nodes,
		OnSelectionChange: func(sel []semKey) {
			if len(sel) == 1 {
				s.selectNode(sel[0])
			}
		},
	}
}

// letterMark is the faint letter before a row of the element tree, which
// tells the kind of the element: S, F, I or R.
type letterMark struct {
	core.StatelessBase
	letter string
	color  geom.Color
}

// Build implements core.StatelessWidget.
func (m letterMark) Build(core.BuildContext) core.Widget {
	return basic.Center{Child: basic.Text{
		Content: m.letter,
		Style:   text.Style{Size: 10, Weight: 600, LineHeight: 1, Color: m.color},
	}}
}

// squareMark is the small square before a row of the render tree: filled for
// a repaint boundary that keeps a layer, hollow for a relayout boundary.
type squareMark struct {
	core.StatelessBase
	color  geom.Color
	filled bool
}

// Build implements core.StatelessWidget.
func (m squareMark) Build(core.BuildContext) core.Widget {
	c := basic.Container{Width: 8, Height: 8, Radius: 2, Border: paint.Stroke{Width: 1.25, Color: m.color}}
	if m.filled {
		c.Color = m.color
	}
	return basic.Center{Child: c}
}
