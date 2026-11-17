//go:build gowakudebug

package inspect

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/editor"
)

// The pages of the details.
const (
	detailsPage = iota
	picturePage
)

// DetailsPanel shows the selection: its widget, element, render object,
// semantics and display list. It only reads them: the Inspector changes
// nothing in the window it inspects.
type DetailsPanel struct {
	core.StatefulBase
	Session *Session
	Target  Target
}

// CreateState implements core.StatefulWidget.
func (DetailsPanel) CreateState() core.State { return &detailsState{} }

type detailsState struct {
	core.StateOf[DetailsPanel]
	cancel func()
	page   int
	// picture holds the display list of dumped, recorded when the page shows
	// and the selection changes, or on Refresh.
	picture    editor.Controller
	dumpTarget Target
	dumped     core.RenderObject
	recorded   bool
	str        *inspectStrings
	// sem is the last walk of the accessibility tree of semTarget.
	sem       *semIndex
	semTarget Target
	semAt     time.Duration
}

// InitState implements core.State.
func (s *detailsState) InitState() {
	s.str = &stringsEN
	s.cancel = s.Widget().Session.Listen(s.changed)
}

// Dispose implements core.State.
func (s *detailsState) Dispose() { s.cancel() }

// changed follows the session: the display list of a new selection is
// recorded if it shows.
func (s *detailsState) changed() {
	if s.page == picturePage {
		s.record(false)
	}
}

func (s *detailsState) setPage(i int) {
	s.SetState(func() { s.page = i })
	if i == picturePage {
		s.record(false)
	}
}

// record dumps the display list of the selected render object, unless it is
// the one dumped already and force is false.
func (s *detailsState) record(force bool) {
	t, _, ro := s.Widget().Session.Selection()
	if !force && s.recorded && t == s.dumpTarget && ro == s.dumped {
		return
	}
	s.dumpTarget, s.dumped, s.recorded = t, ro, true
	s.picture.SetText(dumpOf(t, ro, s.str))
}

// dumpOf returns the text of the picture of ro in t.
func dumpOf(t Target, ro core.RenderObject, str *inspectStrings) string {
	switch {
	case t == nil || ro == nil:
		return ""
	case !t.Tree().HostedBy(ro):
		return str.detached
	case core.NeedsLayoutOf(ro):
		return str.notLaidOut
	}
	head := describe(ro)
	pic := pictureOf(ro)
	if pic == "" {
		return head + "\n\n" + str.drawsNothing
	}
	return head + "\n\n" + pic
}

// Build implements core.State.
func (s *detailsState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	str := stringsOf(ctx)
	s.str = str
	t, el, ro := w.Session.Selection()
	if t == nil || t != w.Target || el.IsZero() && ro == nil {
		return basic.Center{Child: basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Text{
			Content: str.noSelection, Style: th.Text.Body.WithColor(th.Color.Ink2),
		}}}
	}
	grid := collection.PropertyGrid{Categories: s.categories(t, el, ro, str), Filter: true, Label: str.details}
	picture := basic.Column{
		Cross:   layout.CrossStretch,
		Spacing: theme.Space8,
		Children: []core.Widget{
			basic.Row{Main: layout.MainEnd, Children: []core.Widget{
				control.Button{Label: str.refresh, OnPress: func() { s.record(true) }},
			}},
			basic.Expanded{Child: editor.Editor{
				Controller: &s.picture, Label: str.displayList, ReadOnly: true, LineNumbers: true,
				Style: text.Style{Size: 12},
			}},
		},
	}
	return basic.Padding{
		Insets: geom.All(theme.Space8),
		Child: control.Tabs{
			Label:    str.details,
			Selected: s.page,
			OnChange: s.setPage,
			Pages: []control.TabPage{
				{Label: str.details, Child: basic.Padding{Insets: geom.Insets{Top: theme.Space8}, Child: grid}},
				{Label: str.displayList, Child: basic.Padding{Insets: geom.Insets{Top: theme.Space8}, Child: picture}},
			},
		},
	}
}

// categories returns the sections of the details of the selection.
func (s *detailsState) categories(t Target, el core.ElementView, ro core.RenderObject, str *inspectStrings) []collection.PropertyCategory {
	var sections []detailSection
	if !el.IsZero() {
		sections = append(sections,
			detailSection{name: "Widget", rows: widgetRows(el.Widget())},
			detailSection{name: "Element", rows: elementRows(el)},
		)
	}
	if ro != nil {
		hosted := t.Tree().HostedBy(ro)
		sections = append(sections, detailSection{name: "RenderObject", rows: objectRows(ro, hosted, str)})
		if hosted {
			sections = append(sections, detailSection{name: "Semantics", rows: s.semanticsOf(t, ro, str)})
		}
	}
	out := make([]collection.PropertyCategory, len(sections))
	for i, sec := range sections {
		props := make([]collection.Property, len(sec.rows))
		for j, r := range sec.rows {
			props[j] = collection.Property{Name: r.name, Editor: collection.PropertyText{Value: r.value}}
			if r.isColor {
				props[j].Editor = collection.PropertyColor{Value: r.color}
			}
		}
		out[i] = collection.PropertyCategory{Name: sec.name, Properties: props}
	}
	return out
}

// semanticsOf returns the rows of the semantics node of ro, walking the
// accessibility tree again when the last walk is old.
func (s *detailsState) semanticsOf(t Target, ro core.RenderObject, str *inspectStrings) []detailRow {
	now := s.Widget().Session.Now()
	if s.sem == nil || s.semTarget != t || now-s.semAt >= semanticsLife {
		s.sem, s.semTarget, s.semAt = newSemIndex(t.Tree()), t, now
	}
	k, owner, ok := s.sem.of(ro)
	if !ok && s.semAt != now {
		// The object may be newer than the last walk.
		s.sem, s.semAt = newSemIndex(t.Tree()), now
		k, owner, ok = s.sem.of(ro)
	}
	if !ok || k == rootSem {
		return []detailRow{{name: "Node", value: str.noNode}}
	}
	return semanticsRows(s.sem.nodes[k], owner == ro, owner, str)
}
