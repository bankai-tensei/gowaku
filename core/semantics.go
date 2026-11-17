package core

import (
	"slices"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// SemanticsArg is what comes with an action: the text or number of
// ActionSetValue, the range of ActionSetSelection.
type SemanticsArg struct {
	Text   string
	Number float64
	// SelectionStart and SelectionEnd count UTF-16 code units.
	SelectionStart, SelectionEnd int
}

// SemanticsActor is a RenderObject that does the actions of the node it opens.
// It reports whether it did the action.
type SemanticsActor interface {
	RenderObject
	PerformSemanticsAction(a semantics.Actions, arg SemanticsArg) bool
}

// SemanticsBlocker is a RenderObject that takes what is painted before it out
// of the semantics tree while BlocksSemantics reports true, as a modal dialog
// hides the page under it from a screen reader (aria-modal). The last one
// painted that is not hidden wins; what is painted after it, such as a toast
// or a popup opened from the dialog, stays in the tree. A popup anchored to a
// control, such as the list of a dropdown, does not block, so that the
// control stays in the tree and says that it is expanded.
//
// BlocksSemantics is asked at every build of the semantics tree, so it must
// be cheap and must not allocate.
type SemanticsBlocker interface {
	RenderObject
	BlocksSemantics() bool
}

// semanticsBlocker returns the SemanticsBlocker that blocks, painted last below
// ro, or nil.
func semanticsBlocker(ro RenderObject) RenderObject {
	if isHidden(ro) {
		return nil
	}
	for i := ro.ChildCount() - 1; i >= 0; i-- {
		if b := semanticsBlocker(ro.ChildAt(i)); b != nil {
			return b
		}
	}
	if b, ok := ro.(SemanticsBlocker); ok && b.BlocksSemantics() {
		return b
	}
	return nil
}

// SemanticsBuilder assembles the tree during a walk of the render objects.
type SemanticsBuilder struct {
	tree   *Tree
	root   *semantics.Node
	stack  []*semantics.Node
	origin geom.Offset
	// frame carries origin-relative positions to the window once a Transformer
	// turned or scaled them, and framed says that it does.
	frame  geom.Affine
	framed bool
	// current is the object whose Semantics runs.
	current RenderObject
	// focus is the node of the object in focus, or of its nearest ancestor that
	// opened one.
	focus semantics.ID
	// owners maps each node to the object that opened it, for the actions.
	owners map[semantics.ID]RenderObject
	// name is the label of the innermost open control that speaks for the text
	// inside it, or "".
	name string
	// names holds the name of each open node, to restore it on Close.
	names []string
	// kept records for each open node whether it is in the tree.
	kept []bool
	// describe is the hint for the next node that opens. See Describe.
	describe string
	// label is the label for the next node that opens. See Name.
	label string
	// captioning says that the next node that opens is a caption, and caption
	// is that node, under captionParent, until a Name takes it away or a
	// Name("") lets it be. See Caption.
	captioning             bool
	caption, captionParent *semantics.Node
	// blocked is the path from the root to the SemanticsBlocker that blocks,
	// root first, or empty when none does. blockedAt is the place in it of the
	// object whose Semantics runs, while that object is on the path.
	blocked   []RenderObject
	blockedAt int
}

func newSemanticsBuilder(t *Tree) *SemanticsBuilder {
	root := &semantics.Node{}
	return &SemanticsBuilder{tree: t, root: root, stack: []*semantics.Node{root}}
}

// Visit walks child, at its offset from the current object. A hidden Hider is
// skipped with its subtree, and so is what a SemanticsBlocker covers: on the
// way from the root to the blocker, a child painted before the next object on
// that way is left out, as the page under a dialog is. What is painted after
// it, such as a toast over the dialog, stays.
func (sb *SemanticsBuilder) Visit(child RenderObject) {
	if isHidden(child) {
		return
	}
	at := sb.blockedAt
	if at+1 < len(sb.blocked) && sb.current == sb.blocked[at] {
		if next := sb.blocked[at+1]; child == next {
			sb.blockedAt = at + 1
		} else if paintedBefore(sb.current, child, next) {
			return
		}
	}
	saved, savedRO := sb.origin, sb.current
	savedFrame, savedFramed := sb.frame, sb.framed
	if t, ok := sb.current.(Transformer); ok {
		if m := t.ChildTransform(); !m.IsIdentity() {
			// The children of a transform live in a frame of their own.
			base := geom.Translation(sb.origin)
			if sb.framed {
				base = sb.frame.Mul(base)
			}
			sb.frame, sb.framed, sb.origin = base.Mul(m), true, geom.Offset{}
		}
	}
	sb.origin = sb.origin.Add(child.Offset())
	sb.current = child
	top := sb.stack[len(sb.stack)-1]
	before := len(top.Children)
	child.Semantics(sb)
	if sb.tree != nil && sb.focus == 0 {
		if f := sb.tree.focus; f != nil && RenderObject(f) == child {
			// The object in focus opened no node of its own, as the focus of a
			// dialog route does: the first node inside it speaks for it, or else
			// the node around it.
			sb.focus = top.ID
			if len(top.Children) > before {
				sb.focus = top.Children[before].ID
			}
		}
	}
	sb.origin, sb.current = saved, savedRO
	sb.frame, sb.framed = savedFrame, savedFramed
	sb.blockedAt = at
}

// Open adds a node covering size at the current object. Everything visited
// until the matching Close becomes its children. Core sets the ID and the
// Focusable and Focused states.
func (sb *SemanticsBuilder) Open(node semantics.Node, size geom.Size) {
	node.Rect = geom.Rect{X: sb.origin.X, Y: sb.origin.Y, W: size.W, H: size.H}
	if sb.framed {
		// Assistive technology knows only upright boxes.
		node.Rect = sb.frame.ApplyRect(node.Rect)
	}
	sb.names = append(sb.names, sb.name)
	captioning := sb.captioning
	sb.captioning = false
	if node.Role == semantics.RoleText && node.Label != "" && node.Label == sb.name {
		// The caption of a labeled control, such as the text of a button, would
		// be read twice. It goes nowhere, and so does anything below it.
		sb.stack = append(sb.stack, &node)
		sb.kept = append(sb.kept, false)
		return
	}
	if sb.label != "" {
		if node.Label == "" {
			node.Label = sb.label
		}
		sb.dropCaption(node.Label)
		sb.label = ""
	}
	if sb.describe != "" {
		if node.Hint == "" {
			node.Hint = sb.describe
		}
		sb.describe = ""
	}
	if node.Label != "" && namesItself(node.Role) {
		sb.name = node.Label
	}
	if ro := sb.current; ro != nil {
		b := ro.base()
		if b.semanticsID == 0 && b.tree != nil {
			b.tree.lastSemanticsID++
			b.semanticsID = semantics.ID(b.tree.lastSemanticsID)
		}
		node.ID = b.semanticsID
		if sb.owners != nil {
			sb.owners[node.ID] = ro
		}
		if f := focusTargetWithin(ro); f != nil {
			node.State |= semantics.Focusable
			node.Actions |= semantics.ActionFocus
			if sb.tree != nil && sb.tree.focus == f {
				node.State |= semantics.Focused
				sb.focus = node.ID
			}
		}
	}
	top := sb.stack[len(sb.stack)-1]
	top.Children = append(top.Children, &node)
	if captioning {
		sb.caption, sb.captionParent = &node, top
	}
	sb.stack = append(sb.stack, &node)
	sb.kept = append(sb.kept, true)
}

// dropCaption takes the caption out of the tree when it says label, the name
// of the control beside it, whether the control took it from Name or had it.
func (sb *SemanticsBuilder) dropCaption(label string) {
	c, parent := sb.caption, sb.captionParent
	sb.caption, sb.captionParent = nil, nil
	if c == nil || c.Label != label {
		return
	}
	for i, n := range parent.Children {
		if n == c {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return
		}
	}
}

// Describe gives the next node that opens the hint, unless it has one, as a
// tooltip describes the control it wraps. Describe("") cancels it; call that
// after visiting the child, so that the hint goes no further than intended.
func (sb *SemanticsBuilder) Describe(hint string) { sb.describe = hint }

// Name gives the next node that opens the label, unless it has one, as the name
// of a row names the field beside it. Name("") cancels it; call that after
// visiting the child, as for Describe. When the node takes the label, or has
// the same one, a Caption that says it leaves the tree.
func (sb *SemanticsBuilder) Name(label string) {
	sb.label = label
	if label == "" {
		sb.caption, sb.captionParent = nil, nil
	}
}

// Caption marks the node that opens next as the visible name of a control that
// a later Name names, as the text at the left of a settings row names the
// switch at its right. If the control takes the name, or already says the
// same, the caption leaves the tree, so that a screen reader does not say the
// name twice; if the control has another label, the caption stays as text.
func (sb *SemanticsBuilder) Caption() { sb.captioning = true }

// Close ends the node of the matching Open.
func (sb *SemanticsBuilder) Close() {
	if top := sb.stack[len(sb.stack)-1]; sb.kept[len(sb.kept)-1] && top.Label == "" && namesFromContent(top.Role) {
		nameFromContent(top)
	}
	sb.name = sb.names[len(sb.names)-1]
	sb.names = sb.names[:len(sb.names)-1]
	sb.stack = sb.stack[:len(sb.stack)-1]
	sb.kept = sb.kept[:len(sb.kept)-1]
}

// namesItself reports whether a node of role r with a label speaks for a text
// inside it that says the same, as a button does for its caption and a group for
// its title.
func namesItself(r semantics.Role) bool {
	switch r {
	case semantics.RoleButton, semantics.RoleCheckbox, semantics.RoleSwitch, semantics.RoleRadio,
		semantics.RoleTab, semantics.RoleListItem, semantics.RoleLink, semantics.RoleHeading,
		semantics.RoleText, semantics.RoleGroup, semantics.RoleList, semantics.RoleDialog,
		semantics.RoleRadioGroup, semantics.RoleTabList, semantics.RoleMenuItem, semantics.RoleMenuItemCheckbox,
		semantics.RoleMenu, semantics.RoleMenuBar:
		return true
	}
	return false
}

// namesFromContent reports whether a node of role r without a label takes its
// name from the text inside it, as ARIA has buttons and links do. A pressable
// Card is the usual case: its title is the name of the button.
func namesFromContent(r semantics.Role) bool {
	return r == semantics.RoleButton || r == semantics.RoleLink
}

// nameFromContent names n with the texts below it, joined by spaces, and drops
// those texts so that they are not read a second time. It leaves n alone when
// there is no text.
func nameFromContent(n *semantics.Node) {
	var name string
	n.Walk(func(m *semantics.Node) bool {
		if m != n && m.Role == semantics.RoleText && len(m.Children) == 0 && m.Label != "" {
			if name != "" {
				name += " "
			}
			name += m.Label
		}
		return true
	})
	if name == "" {
		return
	}
	n.Label = name
	n.Children = dropTexts(n.Children)
}

// dropTexts removes the text leaves of a subtree, in place.
func dropTexts(children []*semantics.Node) []*semantics.Node {
	kept := children[:0]
	for _, c := range children {
		if c.Role == semantics.RoleText && len(c.Children) == 0 {
			continue
		}
		c.Children = dropTexts(c.Children)
		kept = append(kept, c)
	}
	return kept
}

// focusTargetWithin returns the object that takes the focus for a node opened by
// ro: ro itself, or the first one below it through single children, which is
// how widgets stack Semantics, Focus and the control. It looks no further, so
// that a group does not claim the focus of a control inside it.
func focusTargetWithin(ro RenderObject) FocusTarget {
	for depth := 0; ro != nil && depth < 4; depth++ {
		if f, ok := ro.(FocusTarget); ok {
			if f.CanFocus() {
				return f
			}
			return nil
		}
		if ro.ChildCount() != 1 {
			return nil
		}
		ro = ro.ChildAt(0)
	}
	return nil
}

// paintedBefore reports whether child is a child of parent painted before
// next, another child of it.
func paintedBefore(parent, child, next RenderObject) bool {
	for i := range parent.ChildCount() {
		switch parent.ChildAt(i) {
		case next:
			return false
		case child:
			return true
		}
	}
	return false
}

// build walks the tree from its root, leaving out what a SemanticsBlocker
// covers.
func (sb *SemanticsBuilder) build() {
	t := sb.tree
	if isHidden(t.rootRO) {
		return
	}
	t.blockedPath = t.blockedPath[:0]
	if b := semanticsBlocker(t.rootRO); b != nil {
		for ro := b; ro != nil; ro = ro.Parent() {
			t.blockedPath = append(t.blockedPath, ro)
		}
		slices.Reverse(t.blockedPath)
	}
	sb.blocked, sb.blockedAt = t.blockedPath, 0
	sb.current = t.rootRO
	t.rootRO.Semantics(sb)
	sb.focusSelected()
}

// SemanticsTree returns the accessibility tree with the ID of the node in focus,
// zero for none. Unlike Semantics, it remembers which object opened each node,
// for PerformSemanticsAction. The root node has the ID zero. Like Semantics,
// it leaves out what a SemanticsBlocker covers, so that a screen reader stays
// in a dialog while it is open and cannot act on the page under it.
func (t *Tree) SemanticsTree() (root *semantics.Node, focus semantics.ID) {
	sb := newSemanticsBuilder(t)
	if t.owners == nil {
		t.owners = make(map[semantics.ID]RenderObject)
	}
	clear(t.owners)
	sb.owners = t.owners
	sb.build()
	return sb.root, sb.focus
}

// SemanticsOwner returns the object that opened the node id of the last
// SemanticsTree, or nil. A tool that relates the semantics tree to what is
// painted, as gowakutest.Tester.Warnings does to leave out the text of a
// disabled control, looks the objects up with it.
func (t *Tree) SemanticsOwner(id semantics.ID) RenderObject { return t.owners[id] }

// focusSelected moves the focus from a group of choices that has it to the
// chosen one inside, as the arrow keys move the focus between native radio
// buttons, so that a reader says each choice as it is picked.
func (sb *SemanticsBuilder) focusSelected() {
	if sb.focus == 0 {
		return
	}
	group := sb.root.Find(sb.focus)
	if group == nil || group.Role != semantics.RoleRadioGroup && group.Role != semantics.RoleTabList {
		return
	}
	for _, c := range group.Children {
		if c.State&semantics.Selected != 0 {
			group.State &^= semantics.Focused
			c.State |= semantics.Focused | semantics.Focusable
			sb.focus = c.ID
			return
		}
	}
}

// PerformSemanticsAction does action a to the node id of the last SemanticsTree,
// as a screen reader asks. It reports whether anything did it. ActionFocus
// gives the keyboard to the node, and shows the focus as Tab does.
func (t *Tree) PerformSemanticsAction(id semantics.ID, a semantics.Actions, arg SemanticsArg) bool {
	ro := t.owners[id]
	if ro == nil || ro.base().tree != t || insideHidden(ro) {
		return false
	}
	if a == semantics.ActionFocus {
		f := focusTargetWithin(ro)
		if f == nil {
			return false
		}
		t.setFocusVisible(true)
		t.RequestFocus(f)
		return true
	}
	if actor, ok := ro.(SemanticsActor); ok {
		return actor.PerformSemanticsAction(a, arg)
	}
	return false
}
