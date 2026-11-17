package kit

import "github.com/bankai-tensei/gowaku/core"

// FieldKind is the kind of value a control asks for, which decides the words
// of a form field left empty: a text is entered, a choice made, a box
// checked.
type FieldKind uint8

const (
	// FieldText is a control that takes typed text, and what a form field
	// assumes when no control reports.
	FieldText FieldKind = iota
	// FieldChoice is a control that chooses from given values: a dropdown, a
	// date, a group of radio buttons or chips.
	FieldChoice
	// FieldCheck is a box to check, such as a consent.
	FieldCheck
)

// String implements fmt.Stringer.
func (k FieldKind) String() string {
	switch k {
	case FieldChoice:
		return "FieldChoice"
	case FieldCheck:
		return "FieldCheck"
	}
	return "FieldText"
}

// FieldReport is what a form field (field.FormItem) hears from the control
// inside it. The field puts one in a core.Inherited above its child, and the
// controls, some of which live in packages below field, report into it with
// ReportField.
type FieldReport struct {
	// Kind is what a control reported, FieldText while none has.
	Kind FieldKind
}

// ReportField tells the form field around ctx, if any, the kind of the control
// at ctx. A control calls it as it builds.
func ReportField(ctx core.BuildContext, kind FieldKind) {
	if r, ok := core.Lookup[*FieldReport](ctx); ok && r != nil {
		r.Kind = kind
	}
}
