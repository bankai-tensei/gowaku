package win32

import (
	"github.com/bankai-tensei/gowaku/platform/win32/com"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The Selection, Grid, GridItem, Table and TableItem patterns, for the
// containers of choices and for grids such as DataTable. A grid may give nodes
// only to the rows in view; RowCount and ColumnCount of its node tell the
// whole size then.

// elements returns a SAFEARRAY of the element objects of ids, which the
// caller owns. b.mu must not be held.
func (b *bridge) elements(ids []semantics.ID) *com.SAFEARRAY {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]*com.Unknown, 0, len(ids))
	for _, id := range ids {
		if p := b.simpleLocked(id); p != nil {
			out = append(out, p)
		}
	}
	return com.NewSafeArrayUnknown(out)
}

// withSnapshot runs fn on the tree of the node behind this, under the lock of
// its bridge.
func withSnapshot(this *com.Unknown, fn func(p *provider, s *snapshot)) uintptr {
	p, _, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	p.b.mu.Lock()
	defer p.b.mu.Unlock()
	fn(p, &p.b.snap)
	return com.S_OK
}

// ISelectionProvider

func selectionGetSelection(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	var ids []semantics.ID
	var b *bridge
	hr := withSnapshot(this, func(p *provider, s *snapshot) { ids, b = s.selection(p.id), p.b })
	if hr != com.S_OK {
		return hr
	}
	*ret = b.elements(ids)
	return com.S_OK
}

func selectionCanSelectMultiple(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	_, n, hr := lookupNode(this)
	if hr != com.S_OK {
		return hr
	}
	setBool(ret, n.State&semantics.Multiselectable != 0)
	return com.S_OK
}

func selectionIsSelectionRequired(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 0
	_, _, hr := lookupNode(this)
	return hr
}

// IGridProvider

// gridGetItem returns the cell at row and column. A cell of a virtual grid
// has a node only while it is in view: one out of view fails with
// E_INVALIDARG, as one outside the grid does.
func gridGetItem(this *com.Unknown, row, column uintptr, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	hr := withSnapshot(this, func(p *provider, s *snapshot) {
		if id, ok := s.gridItem(p.id, int(int32(row)), int(int32(column))); ok {
			*ret = p.b.simpleLocked(id)
		}
	})
	if hr == com.S_OK && *ret == nil {
		return com.E_INVALIDARG
	}
	return hr
}

func gridCount(this *com.Unknown, ret *int32, rows bool) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 0
	return withSnapshot(this, func(p *provider, s *snapshot) {
		r, c := s.gridSize(p.id)
		if rows {
			*ret = int32(r)
		} else {
			*ret = int32(c)
		}
	})
}

func gridRowCount(this *com.Unknown, ret *int32) uintptr { return gridCount(this, ret, true) }

func gridColumnCount(this *com.Unknown, ret *int32) uintptr { return gridCount(this, ret, false) }

// IGridItemProvider

func gridItemPlace(this *com.Unknown, ret *int32, row bool) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 0
	return withSnapshot(this, func(p *provider, s *snapshot) {
		r, c, _ := s.gridPosition(p.id)
		if row {
			*ret = int32(r)
		} else {
			*ret = int32(c)
		}
	})
}

func gridItemRow(this *com.Unknown, ret *int32) uintptr { return gridItemPlace(this, ret, true) }

func gridItemColumn(this *com.Unknown, ret *int32) uintptr { return gridItemPlace(this, ret, false) }

// gridItemSpan answers RowSpan and ColumnSpan: a cell covers one of each.
func gridItemSpan(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = 1
	_, _, hr := lookupNode(this)
	return hr
}

func gridItemContainingGrid(this *com.Unknown, ret **com.Unknown) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	return withSnapshot(this, func(p *provider, s *snapshot) {
		if id, ok := s.gridOf(p.id); ok {
			*ret = p.b.simpleLocked(id)
		}
	})
}

// ITableProvider

// tableGetRowHeaders returns no element: the rows of a grid have no titles.
func tableGetRowHeaders(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	if _, _, hr := lookupNode(this); hr != com.S_OK {
		return hr
	}
	*ret = com.NewSafeArrayUnknown(nil)
	return com.S_OK
}

func tableGetColumnHeaders(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	var ids []semantics.ID
	var b *bridge
	hr := withSnapshot(this, func(p *provider, s *snapshot) { ids, b = s.columnHeaders(p.id, -1), p.b })
	if hr != com.S_OK {
		return hr
	}
	*ret = b.elements(ids)
	return com.S_OK
}

func tableRowOrColumnMajor(this *com.Unknown, ret *int32) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = RowOrColumnMajor_RowMajor
	_, _, hr := lookupNode(this)
	return hr
}

// ITableItemProvider

func tableItemGetRowHeaderItems(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	return tableGetRowHeaders(this, ret)
}

func tableItemGetColumnHeaderItems(this *com.Unknown, ret **com.SAFEARRAY) uintptr {
	if ret == nil {
		return com.E_POINTER
	}
	*ret = nil
	var ids []semantics.ID
	var b *bridge
	hr := withSnapshot(this, func(p *provider, s *snapshot) {
		b = p.b
		_, col, ok := s.gridPosition(p.id)
		if grid, found := s.gridOf(p.id); ok && found {
			ids = s.columnHeaders(grid, col)
		}
	})
	if hr != com.S_OK {
		return hr
	}
	*ret = b.elements(ids)
	return com.S_OK
}
