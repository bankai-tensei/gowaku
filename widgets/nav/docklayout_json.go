package nav

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bankai-tensei/gowaku/layout"
)

// DockVersion is the version of the JSON form of a DockArrangement that this
// package writes. Unmarshaling takes this version and no other.
//
// The hidden panels came later within version 1: a reader from before them
// ignores the key, and takes the hidden panels for new ones. See
// docs/ADR/0102-dock-autohide-nesting.md.
const DockVersion = 1

// ErrDockVersion is returned, wrapped, by unmarshaling a DockArrangement whose
// version is missing or one this package does not know, as one saved by a
// newer program.
var ErrDockVersion = errors.New("gowaku: unsupported dock arrangement version")

// dockWire is the JSON form of a DockArrangement. Fields that a later version
// adds, such as windows of torn off panels, are ignored by this one.
type dockWire struct {
	Version   int           `json:"version"`
	Root      *dockNodeWire `json:"root,omitempty"`
	Maximized string        `json:"maximized,omitempty"`
	Closed    []string      `json:"closed,omitempty"`
	// Hidden are the hidden panels by side. Sides this version does not know
	// are ignored.
	Hidden map[string][]dockHiddenWire `json:"hidden,omitempty"`
}

// dockHiddenWire is the JSON form of a DockHidden, whose side is its key.
type dockHiddenWire struct {
	ID     string   `json:"id"`
	Size   float32  `json:"size,omitempty"`
	Origin []string `json:"origin,omitempty"`
	Share  float32  `json:"share,omitempty"`
}

// dockNodeWire is the JSON form of a DockNode: a split names its axis.
type dockNodeWire struct {
	Split    string         `json:"split,omitempty"`
	Weights  []float32      `json:"weights,omitempty"`
	Children []dockNodeWire `json:"children,omitempty"`
	Panels   []string       `json:"panels,omitempty"`
	Active   string         `json:"active,omitempty"`
}

// The names of the axes of a split.
const (
	dockSplitHorizontal = "horizontal"
	dockSplitVertical   = "vertical"
)

// MarshalJSON implements json.Marshaler.
func (a DockArrangement) MarshalJSON() ([]byte, error) {
	w := dockWire{Version: DockVersion, Maximized: a.Maximized, Closed: a.Closed}
	if !a.empty() {
		root := toWire(&a.Root)
		w.Root = &root
	}
	for _, h := range a.Hidden {
		if w.Hidden == nil {
			w.Hidden = make(map[string][]dockHiddenWire)
		}
		side := h.Side.side().String()
		w.Hidden[side] = append(w.Hidden[side], dockHiddenWire{ID: h.ID, Size: h.Size, Origin: h.Origin, Share: h.Share})
	}
	return json.Marshal(w)
}

// UnmarshalJSON implements json.Unmarshaler. It fails with ErrDockVersion for
// a version other than DockVersion, and with another error for a split whose
// axis it does not know. What it reads is checked against the panels of the
// layout only when the layout shows it.
func (a *DockArrangement) UnmarshalJSON(data []byte) error {
	var w dockWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	if w.Version != DockVersion {
		return fmt.Errorf("nav: dock arrangement version %d: %w", w.Version, ErrDockVersion)
	}
	out := DockArrangement{Maximized: w.Maximized, Closed: w.Closed}
	if w.Root != nil {
		root, err := fromWire(w.Root)
		if err != nil {
			return err
		}
		out.Root = root
	}
	// The sides in a fixed order, since the map has none.
	for side := DockLeft; side <= DockBottom; side++ {
		for _, h := range w.Hidden[side.String()] {
			out.Hidden = append(out.Hidden, DockHidden{ID: h.ID, Side: side, Size: h.Size, Origin: h.Origin, Share: h.Share})
		}
	}
	*a = out
	return nil
}

func toWire(n *DockNode) dockNodeWire {
	if len(n.Children) == 0 {
		return dockNodeWire{Panels: n.Panels, Active: n.Active}
	}
	w := dockNodeWire{Split: dockSplitHorizontal, Weights: n.Weights, Children: make([]dockNodeWire, len(n.Children))}
	if n.Axis == layout.Vertical {
		w.Split = dockSplitVertical
	}
	for i := range n.Children {
		w.Children[i] = toWire(&n.Children[i])
	}
	return w
}

func fromWire(w *dockNodeWire) (DockNode, error) {
	if len(w.Children) == 0 {
		return DockNode{Panels: w.Panels, Active: w.Active}, nil
	}
	n := DockNode{Weights: w.Weights, Children: make([]DockNode, len(w.Children))}
	switch w.Split {
	case dockSplitHorizontal, "":
	case dockSplitVertical:
		n.Axis = layout.Vertical
	default:
		return DockNode{}, fmt.Errorf("nav: dock arrangement: unknown split %q", w.Split)
	}
	for i := range w.Children {
		c, err := fromWire(&w.Children[i])
		if err != nil {
			return DockNode{}, err
		}
		n.Children[i] = c
	}
	return n, nil
}
