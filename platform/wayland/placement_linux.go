package wayland

// Wayland keeps the place of windows to the compositor: a client neither
// learns where its windows are nor puts them anywhere, and has no point of the
// screen to compare. A tab torn off a TabBar therefore opens its window where
// the compositor likes, and cannot be dropped on another window; the
// xdg-toplevel-drag protocol that would allow it is not used.

// ClientOrigin implements platform.Placement. Wayland does not tell.
func (w *Window) ClientOrigin() (physicalX, physicalY int, ok bool) { return 0, 0, false }

// MoveClientTo implements platform.Placement. Wayland ignores it.
func (w *Window) MoveClientTo(_, _ int) {}

// ShowsScreenPoint implements platform.Placement. Wayland does not tell.
func (w *Window) ShowsScreenPoint(_, _ int) bool { return false }
