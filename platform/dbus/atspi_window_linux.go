package dbus

import (
	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

// A11yWindow is the frame of one window in the AT-SPI tree, and its
// platform.Accessibility (ADR 0043).
type A11yWindow struct {
	a  *A11y
	id platform.WindowID
	// geometry returns where the window is, on the UI goroutine.
	geometry func() WindowGeometry
	// number is the window in the paths of its nodes. It never changes.
	number uint16

	// The rest is guarded by a.mu.
	title  string
	active bool
	tree   *tree
	geo    WindowGeometry
}

var _ platform.Accessibility = (*A11yWindow)(nil)

// NewWindow adds the frame of a window to the application. geometry reports
// where the window is; it is called on the UI goroutine, at once and from
// Update and Moved. Nil leaves the extents at the top left of the screen.
func (a *A11y) NewWindow(id platform.WindowID, title string, geometry func() WindowGeometry) *A11yWindow {
	w := &A11yWindow{a: a, id: id, geometry: geometry, title: title, tree: newTree(nil, 0)}
	if geometry != nil {
		w.geo = geometry()
	}
	a.mu.Lock()
	a.lastWindow++
	if a.lastWindow == 0 {
		// 65535 windows later the numbers start again; the first are long gone.
		a.lastWindow = 1
	}
	w.number = a.lastWindow
	if !a.closed {
		a.windows = append(a.windows, w)
	}
	index := len(a.windows) - 1
	conn := a.conn
	frame := a.refLocked(w.path(0))
	a.mu.Unlock()
	if conn != nil {
		emit(conn, rootPath, eventName("ChildrenChanged"), "add", int32(index), 0, frame)
		emit(conn, w.path(0), windowEventName("Create"), "", 0, 0, nil)
	}
	return w
}

// path returns the object path of node id; zero is the frame.
func (w *A11yWindow) path(id semantics.ID) string { return nodePath(w.number, id) }

// Listening implements platform.Accessibility. On Linux it is true once the
// bridge is on the accessibility bus (ADR 0043 §2): readers subscribe to events
// before they read the tree, so waiting for their first question would miss
// what happens until then.
func (w *A11yWindow) Listening() bool { return w.a.listening.Load() }

// SetTitle changes the name of the frame.
func (w *A11yWindow) SetTitle(title string) {
	a := w.a
	a.mu.Lock()
	changed := w.title != title
	w.title = title
	conn := a.conn
	a.mu.Unlock()
	if conn != nil && changed {
		emit(conn, w.path(0), eventName("PropertyChange"), "accessible-name", 0, 0, title)
	}
}

// Moved reads the geometry again, after the window moved, resized or changed
// scale.
func (w *A11yWindow) Moved() {
	if w.geometry == nil {
		return
	}
	g := w.geometry()
	w.a.mu.Lock()
	w.geo = g
	w.a.mu.Unlock()
}

// SetActive says whether the window has the keyboard focus. Readers follow
// the focus only within the active window.
func (w *A11yWindow) SetActive(active bool) {
	a := w.a
	a.mu.Lock()
	changed := w.active != active
	w.active = active
	conn, t := a.conn, w.tree
	a.mu.Unlock()
	if conn == nil || !changed {
		return
	}
	frame := w.path(0)
	on := int32(0)
	member := "Deactivate"
	if active {
		on, member = 1, "Activate"
	}
	emit(conn, frame, windowEventName(member), "", 0, 0, nil)
	emit(conn, frame, eventName("StateChanged"), "active", on, 0, nil)
	if t.focus != 0 && t.has(t.focus) {
		emit(conn, w.path(t.focus), eventName("StateChanged"), "focused", on, 0, nil)
	}
}

// Destroy takes the frame out of the application, when the window is gone.
// Its objects stop answering.
func (w *A11yWindow) Destroy() {
	a := w.a
	a.mu.Lock()
	index := -1
	for i, x := range a.windows {
		if x == w {
			index = i
			a.windows = append(a.windows[:i], a.windows[i+1:]...)
			break
		}
	}
	conn := a.conn
	frame := a.refLocked(w.path(0))
	w.tree = newTree(nil, 0)
	a.mu.Unlock()
	if conn != nil && index >= 0 {
		emit(conn, rootPath, eventName("ChildrenChanged"), "remove", int32(index), 0, frame)
		emit(conn, string(frame.Path), windowEventName("Destroy"), "", 0, 0, nil)
	}
}

// Update implements platform.Accessibility.
func (w *A11yWindow) Update(root *semantics.Node, focus semantics.ID, d *semantics.Diff) {
	t := newTree(root, focus)
	var g WindowGeometry
	if w.geometry != nil {
		g = w.geometry()
	}
	a := w.a
	a.mu.Lock()
	gone := !w.attachedLocked()
	old := w.tree
	if !gone {
		w.tree = t
		if w.geometry != nil {
			w.geo = g
		}
	}
	conn := a.conn
	a.mu.Unlock()
	if gone || conn == nil || d == nil {
		return
	}
	w.emitChanges(conn, old, t, d)
}

// attachedLocked reports whether the window is still a child of the
// application. a.mu must be held.
func (w *A11yWindow) attachedLocked() bool {
	for _, x := range w.a.windows {
		if x == w {
			return true
		}
	}
	return false
}

// emitChanges tells the readers what turned old into t (ADR 0043 §4).
func (w *A11yWindow) emitChanges(conn *godbus.Conn, old, t *tree, d *semantics.Diff) {
	for _, c := range d.Changed {
		n, before := c.Node, old.node(c.Node.ID)
		if before == nil {
			continue
		}
		path := w.path(n.ID)
		if c.What&semantics.ChangedChildren != 0 {
			w.emitChildren(conn, path, before, n)
		}
		if n.ID == 0 {
			// The frame is named by the title.
			continue
		}
		if name := atspiName(n); name != atspiName(before) {
			emit(conn, path, eventName("PropertyChange"), "accessible-name", 0, 0, name)
		}
		if n.Hint != before.Hint {
			emit(conn, path, eventName("PropertyChange"), "accessible-description", 0, 0, n.Hint)
		}
		if hasRange(n) && (n.Range.Value != before.Range.Value || n.Value != before.Value) {
			emit(conn, path, eventName("PropertyChange"), "accessible-value", 0, 0, n.Range.Value)
		}
		if c.What&(semantics.ChangedState|semantics.ChangedRole|semantics.ChangedActions) != 0 {
			for _, s := range changedStates(nodeStates(before, false), nodeStates(n, false)) {
				on := int32(0)
				if s.on {
					on = 1
				}
				emit(conn, path, eventName("StateChanged"), s.name, on, 0, nil)
			}
		}
		if n.Text != nil && before.Text != nil {
			w.emitText(conn, path, before.Text, n.Text, c.What)
		}
		if n.Live != semantics.LiveOff && c.What&(semantics.ChangedLabel|semantics.ChangedValue|semantics.ChangedText) != 0 {
			emitAnnouncement(conn, path, n)
		}
	}
	// A live region speaks when it changes, not when it appears, as in ARIA:
	// otherwise every window would read its status lines on opening.
	if old.focus != t.focus {
		if old.focus != 0 && t.has(old.focus) {
			emit(conn, w.path(old.focus), eventName("StateChanged"), "focused", 0, 0, nil)
		}
		if t.focus != 0 && t.has(t.focus) {
			emit(conn, w.path(t.focus), eventName("StateChanged"), "focused", 1, 0, nil)
		}
	}
}

// emitChildren says which children of a node left and which came.
func (w *A11yWindow) emitChildren(conn *godbus.Conn, path string, before, after *semantics.Node) {
	is := make(map[semantics.ID]bool, len(after.Children))
	for _, c := range after.Children {
		is[c.ID] = true
	}
	was := make(map[semantics.ID]bool, len(before.Children))
	for _, c := range before.Children {
		was[c.ID] = true
	}
	w.a.mu.Lock()
	name := w.a.busName
	w.a.mu.Unlock()
	// Removed from the end first, so that each index is still right.
	for i := len(before.Children) - 1; i >= 0; i-- {
		if c := before.Children[i]; !is[c.ID] {
			emit(conn, path, eventName("ChildrenChanged"), "remove", int32(i), 0,
				ref{name, godbus.ObjectPath(w.path(c.ID))})
		}
	}
	for i, c := range after.Children {
		if !was[c.ID] {
			emit(conn, path, eventName("ChildrenChanged"), "add", int32(i), 0,
				ref{name, godbus.ObjectPath(w.path(c.ID))})
		}
	}
}

// emitText tells of a change of the text, the caret or the selection of a
// field, in characters.
func (w *A11yWindow) emitText(conn *godbus.Conn, path string, before, after *semantics.TextInfo, what semantics.Change) {
	if what&semantics.ChangedText != 0 && before.Text != after.Text {
		start, removed, inserted := textChange(before.Text, after.Text)
		if len(removed) > 0 {
			emit(conn, path, eventName("TextChanged"), "delete", int32(start), int32(len(removed)), string(removed))
		}
		if len(inserted) > 0 {
			emit(conn, path, eventName("TextChanged"), "insert", int32(start), int32(len(inserted)), string(inserted))
		}
	}
	if what&(semantics.ChangedSelection|semantics.ChangedText) == 0 {
		return
	}
	bt, at := newAtspiText(before), newAtspiText(after)
	if caret := at.caret(); caret != bt.caret() || what&semantics.ChangedText != 0 {
		emit(conn, path, eventName("TextCaretMoved"), "", int32(caret), 0, nil)
	}
	s0, e0, had := bt.selection()
	s1, e1, has := at.selection()
	if (had || has) && (s0 != s1 || e0 != e1 || had != has) {
		emit(conn, path, eventName("TextSelectionChanged"), "", 0, 0, nil)
	}
}

// emitAnnouncement has a live region read out. detail1 is its politeness, as
// AtspiLive counts it, which is the order of semantics.Live.
func emitAnnouncement(conn *godbus.Conn, path string, n *semantics.Node) {
	emit(conn, path, eventName("Announcement"), "", int32(n.Live), 0, announcement(n))
}
