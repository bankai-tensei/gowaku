//go:build gowakudebug

package inspect

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The functions here turn the selection into the rows of the details, as text.

const (
	// maxValueDepth is how deep a value of a field shows the structs inside it.
	maxValueDepth = 2
	// maxValueLen bounds the text of one value, and maxStringLen a string in it.
	maxValueLen  = 160
	maxStringLen = 80
)

// detailRow is a name and a value. A row of a color shows a swatch.
type detailRow struct {
	name, value string
	color       geom.Color
	isColor     bool
}

// detailSection is a group of rows under a title.
type detailSection struct {
	name string
	rows []detailRow
}

var (
	colorType  = reflect.TypeFor[geom.Color]()
	widgetType = reflect.TypeFor[core.Widget]()
)

// widgetRows describes w: its type, its key and its exported fields.
func widgetRows(w core.Widget) []detailRow {
	rows := []detailRow{{name: "Type", value: TypeName(w)}}
	if w == nil {
		return rows
	}
	key := "—"
	if k := w.Key(); k != nil {
		key = keyLabel(k)
	}
	rows = append(rows, detailRow{name: "Key", value: key})
	v := reflect.ValueOf(w)
	for v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return append(rows, detailRow{name: "Value", value: formatValue(v, 0)})
	}
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() || f.Anonymous && !hasExportedFields(f.Type) {
			continue
		}
		rows = append(rows, fieldRow(f.Name, v.Field(i)))
	}
	return rows
}

// fieldRow is the row of a field: a swatch for a color, text for the rest.
func fieldRow(name string, v reflect.Value) detailRow {
	if v.Type() == colorType {
		c := v.Interface().(geom.Color)
		return detailRow{name: name, value: hexColor(c), color: c, isColor: true}
	}
	return detailRow{name: name, value: formatValue(v, 0)}
}

// hasExportedFields reports whether t is a struct with an exported field, as
// an embedded struct must be to say anything.
func hasExportedFields(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return true
	}
	for i := range t.NumField() {
		if t.Field(i).IsExported() {
			return true
		}
	}
	return false
}

// formatValue spells v compactly: a function as "func", a widget as its type,
// a slice as its type and length, a color in hex, a struct as its fields that
// are not zero, down to maxValueDepth.
func formatValue(v reflect.Value, depth int) string {
	return clip(formatInner(v, depth), maxValueLen)
}

func formatInner(v reflect.Value, depth int) string {
	if !v.IsValid() {
		return "nil"
	}
	t := v.Type()
	if t == colorType {
		return hexColor(v.Interface().(geom.Color))
	}
	switch v.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if s, ok := stringer(v); ok {
			return s
		}
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if s, ok := stringer(v); ok {
			return s
		}
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'g', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64)
	case reflect.String:
		if s, ok := stringer(v); ok {
			return s
		}
		return strconv.Quote(clip(v.String(), maxStringLen))
	case reflect.Func:
		if v.IsNil() {
			return "nil"
		}
		return "func"
	case reflect.Chan:
		if v.IsNil() {
			return "nil"
		}
		return "chan"
	case reflect.Slice:
		if v.IsNil() {
			return "nil"
		}
		return fmt.Sprintf("%s len %d", typeString(t), v.Len())
	case reflect.Array:
		return typeString(t)
	case reflect.Map:
		if v.IsNil() {
			return "nil"
		}
		return fmt.Sprintf("%s len %d", typeString(t), v.Len())
	case reflect.Pointer:
		if v.IsNil() {
			return "nil"
		}
		return "*" + typeString(t.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return "nil"
		}
		e := v.Elem()
		if e.Type().Implements(widgetType) {
			return typeString(e.Type())
		}
		return formatInner(e, depth)
	case reflect.Struct:
		return formatStruct(v, depth)
	}
	return typeString(t)
}

// stringer returns the String of an enumeration, such as layout.Axis.
func stringer(v reflect.Value) (string, bool) {
	if !v.CanInterface() {
		return "", false
	}
	if s, ok := v.Interface().(fmt.Stringer); ok {
		return s.String(), true
	}
	return "", false
}

// formatStruct spells the fields of v that are not zero: {Left: 8, Top: 4}.
func formatStruct(v reflect.Value, depth int) string {
	t := v.Type()
	if t.Implements(widgetType) {
		return typeString(t)
	}
	if depth >= maxValueDepth {
		return typeString(t) + "{…}"
	}
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() || v.Field(i).IsZero() {
			continue
		}
		if n > 0 {
			b.WriteString(", ")
		}
		n++
		b.WriteString(f.Name + ": " + formatInner(v.Field(i), depth+1))
		if b.Len() > maxValueLen {
			break
		}
	}
	b.WriteByte('}')
	return b.String()
}

// typeString names t without the paths of its packages, as TypeName does.
func typeString(t reflect.Type) string {
	name := t.String()
	if i := strings.IndexByte(name, '['); i > 0 && t.Kind() != reflect.Slice && t.Kind() != reflect.Array && t.Kind() != reflect.Map {
		name = name[:i] + shortArgs(name[i:])
	}
	return name
}

// hexColor spells c as #RRGGBB, with the alpha after it when it is not opaque.
func hexColor(c geom.Color) string {
	s := c.Std()
	if s.A == 255 {
		return fmt.Sprintf("#%02X%02X%02X", s.R, s.G, s.B)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", s.R, s.G, s.B, s.A)
}

// elementRows describes the element el.
func elementRows(el core.ElementView) []detailRow {
	state := "—"
	if st := el.State(); st != nil {
		state = TypeName(st)
	}
	return []detailRow{
		{name: "Kind", value: el.Kind().String()},
		{name: "Depth", value: strconv.Itoa(el.Depth())},
		{name: "Mounted", value: strconv.FormatBool(el.Mounted())},
		{name: "Dirty", value: strconv.FormatBool(el.Dirty())},
		{name: "Dependencies", value: strconv.Itoa(el.Dependencies())},
		{name: "State", value: state},
		{name: "Children", value: strconv.Itoa(el.ChildCount())},
	}
}

// objectRows describes ro, which hosted says is still in the tree.
func objectRows(ro core.RenderObject, hosted bool, str *inspectStrings) []detailRow {
	rows := []detailRow{{name: "Type", value: TypeName(ro)}}
	if !hosted {
		return append(rows, detailRow{name: "Status", value: str.detached})
	}
	sz, off := ro.Size(), ro.Offset()
	return append(rows,
		detailRow{name: "Offset", value: point(off.X, off.Y)},
		detailRow{name: "Size", value: size(sz.W, sz.H)},
		detailRow{name: "Global rect", value: rect(globalRect(ro))},
		detailRow{name: "Constraints", value: constraints(core.ConstraintsOf(ro))},
		detailRow{name: "Needs layout", value: strconv.FormatBool(core.NeedsLayoutOf(ro))},
		detailRow{name: "Relayout boundary", value: strconv.FormatBool(core.IsRelayoutBoundary(ro))},
		detailRow{name: "Layer", value: strconv.FormatBool(core.HasLayer(ro))},
		detailRow{name: "Children", value: strconv.Itoa(ro.ChildCount())},
	)
}

// semanticsRows describes n, the node of ro or, unless exact, of the ancestor
// owner of ro.
func semanticsRows(n *semantics.Node, exact bool, owner core.RenderObject, str *inspectStrings) []detailRow {
	var rows []detailRow
	if !exact {
		rows = append(rows, detailRow{name: "Node", value: fmt.Sprintf(str.ofAncestor, TypeName(owner))})
	}
	rows = append(rows, detailRow{name: "Role", value: n.Role.String()})
	add := func(name, value string) {
		if value != "" {
			rows = append(rows, detailRow{name: name, value: value})
		}
	}
	add("Label", quoteOrEmpty(n.Label))
	add("Value", quoteOrEmpty(n.Value))
	add("Hint", quoteOrEmpty(n.Hint))
	add("State", flags(uint64(n.State), stateNames[:]))
	add("Actions", flags(uint64(n.Actions), actionNames[:]))
	if n.Range.Max > n.Range.Min {
		r := n.Range
		add("Range", fmt.Sprintf("%g in %g…%g", r.Value, r.Min, r.Max))
	}
	if n.Level > 0 {
		add("Level", strconv.Itoa(n.Level))
	}
	if n.SetSize > 0 {
		add("Position", fmt.Sprintf("%d of %d", n.PosInSet, n.SetSize))
	}
	add("Rect", rect(n.Rect))
	add("ID", strconv.FormatUint(uint64(n.ID), 10))
	add("Children", strconv.Itoa(len(n.Children)))
	return rows
}

func quoteOrEmpty(s string) string {
	if s == "" {
		return ""
	}
	return strconv.Quote(clip(s, maxStringLen))
}

// The names of the bits of semantics.State and semantics.Actions, from the
// lowest.
var (
	stateNames = [...]string{
		"Checked", "Mixed", "Selected", "Expanded", "Collapsed", "Disabled", "Focusable",
		"Focused", "ReadOnly", "Multiline", "Multiselectable", "Invalid", "Required",
	}
	actionNames = [...]string{
		"Tap", "Focus", "Increase", "Decrease", "SetValue", "Expand", "Collapse",
		"SetSelection", "ScrollIntoView",
	}
)

// flags spells the set bits of v by names, or "" for none.
func flags(v uint64, names []string) string {
	var parts []string
	for i, n := range names {
		if v&(1<<i) != 0 {
			parts = append(parts, n)
		}
	}
	if rest := v >> len(names); rest != 0 {
		parts = append(parts, fmt.Sprintf("0x%x", rest<<len(names)))
	}
	return strings.Join(parts, " ")
}

func point(x, y float32) string { return trim(x) + ", " + trim(y) }

func size(w, h float32) string { return trim(w) + "×" + trim(h) }

func rect(r geom.Rect) string { return point(r.X, r.Y) + "  " + size(r.W, r.H) }

// constraints spells c as the ranges of the width and the height:
// "w 0…460  h 32".
func constraints(c geom.Constraints) string {
	return "w " + span(c.MinW, c.MaxW) + "  h " + span(c.MinH, c.MaxH)
}

func span(lo, hi float32) string {
	if lo == hi {
		return bound(lo)
	}
	return bound(lo) + "…" + bound(hi)
}

func bound(v float32) string {
	if v >= geom.Inf || math.IsInf(float64(v), 1) {
		return "∞"
	}
	return trim(v)
}

// pictureOf records what ro draws, at its own origin, and returns it as text,
// or "" when it draws nothing. It is for the Inspector only, outside a frame:
// a repaint boundary records its layer if the layer is out of date, as the next
// frame would.
func pictureOf(ro core.RenderObject) string {
	var cv paint.Canvas
	cv.Reset()
	ro.Paint(&cv, geom.Offset{})
	dl := cv.DisplayList()
	if len(dl.Ops) == 0 {
		return ""
	}
	var b strings.Builder
	writeDisplayList(&b, dl, "")
	return strings.TrimSuffix(b.String(), "\n")
}

// writeDisplayList writes dl as text, one command a line, with the commands
// of a layer indented below it.
func writeDisplayList(b *strings.Builder, dl *paint.DisplayList, indent string) {
	for i := range dl.Ops {
		op := &dl.Ops[i]
		r := op.RRect.Rect
		fmt.Fprintf(b, "%s%s x=%g y=%g w=%g h=%g", indent, op.Kind, r.X, r.Y, r.W, r.H)
		if rr := op.RRect; rr.TopLeft != 0 || rr.TopRight != 0 || rr.BottomRight != 0 || rr.BottomLeft != 0 {
			fmt.Fprintf(b, " radii=%g,%g,%g,%g", rr.TopLeft, rr.TopRight, rr.BottomRight, rr.BottomLeft)
		}
		switch {
		case op.HasGradient:
			g := &op.Gradient
			fmt.Fprintf(b, " gradient=%d from=%g,%g %s to=%g,%g %s", g.Kind, g.Start.X, g.Start.Y, hexColor(g.StartColor), g.End.X, g.End.Y, hexColor(g.EndColor))
			for i := range g.StopCount {
				fmt.Fprintf(b, " stop=%g %s", g.Stops[i].Offset, hexColor(g.Stops[i].Color))
			}
		case op.Kind == paint.OpLayer:
			// The alpha of a layer is its opacity, with zero for opaque.
			if a := op.Color.A; a != 0 && a != 1 {
				fmt.Fprintf(b, " opacity=%g", a)
			}
		case op.Kind != paint.OpClipPush && op.Kind != paint.OpClipPop:
			fmt.Fprintf(b, " color=%s", hexColor(op.Color))
		}
		if op.BorderWidth > 0 {
			fmt.Fprintf(b, " border=%g %s", op.BorderWidth, hexColor(op.BorderColor))
		}
		if op.Stroke.Width > 0 {
			fmt.Fprintf(b, " stroke=%g %s", op.Stroke.Width, hexColor(op.Stroke.Color))
		}
		if op.Glyphs != nil {
			fmt.Fprintf(b, " origin=%g,%g size=%g glyphs=%d", op.Origin.X, op.Origin.Y, op.Glyphs.Size, len(op.Glyphs.Glyphs))
		}
		if op.Blur > 0 {
			fmt.Fprintf(b, " blur=%g", op.Blur)
		}
		if op.Blend != paint.BlendSrcOver {
			fmt.Fprintf(b, " blend=%d", op.Blend)
		}
		if m := op.Transform; !m.IsIdentity() {
			fmt.Fprintf(b, " transform=%.4g,%.4g,%.4g,%.4g,%.4g,%.4g", m.A, m.B, m.C, m.D, m.E, m.F)
		}
		if op.Clipped {
			fmt.Fprintf(b, " clip=%g,%g,%g,%g", op.Clip.X, op.Clip.Y, op.Clip.W, op.Clip.H)
		}
		b.WriteByte('\n')
		if op.Kind == paint.OpLayer && op.Layer != nil {
			writeDisplayList(b, op.Layer.DisplayList(), indent+"  ")
		}
	}
}
