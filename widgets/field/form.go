package field

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// formErrorGap is the room between a field and its error.
const formErrorGap = theme.Space4

// Form groups the FormItems below it, so that they are checked and sent
// together. Submit it with a FormController, from a button for instance, or
// press Enter in a single-line text field of one of its FormItems (Input,
// NumberField, Search, Autocomplete, the empty text of a TokenField) that has
// no OnSubmit of its own:
//
//	var form field.FormController // kept in the State
//	field.Form{Controller: &form, OnSubmit: s.save, Child: basic.Column{Children: []core.Widget{
//		field.FormItem[string]{Value: s.name, Required: true, Child: field.Input{
//			Label: "名前", Value: s.name, OnChange: s.setName,
//		}},
//		control.Button{Label: "保存", Style: control.ButtonPrimary, OnPress: func() { form.Submit() }},
//	}}}
//
// A submit checks every field and shows the errors. If one is wrong, the focus
// goes to the first wrong field on screen and OnSubmit does not run.
type Form struct {
	core.StatefulBase
	// Controller, if set, is how code outside the form submits it.
	Controller *FormController
	// OnSubmit runs when a submit finds every field valid.
	OnSubmit func()
	Child    core.Widget
}

// FormController submits or resets the Form it is given to. The zero value is
// ready to use; keep it where it outlives a build, such as in a State.
type FormController struct {
	form *formState
}

// Submit checks every field of the form and shows their errors. It runs the
// OnSubmit of the form and returns true if all are valid; otherwise it gives the
// focus to the first wrong field and returns false. It panics if no Form uses
// the controller.
func (c *FormController) Submit() bool {
	if c.form == nil {
		panic("gowaku: FormController.Submit called while no Form uses the controller")
	}
	return c.form.submit()
}

// Reset hides the errors of every field of the form, which then wait for the
// next blur or submit again. The values stay; they belong to the caller. It
// does nothing if no Form uses the controller.
func (c *FormController) Reset() {
	if c.form == nil {
		return
	}
	for _, f := range c.form.fields {
		f.reset()
	}
}

type formState struct {
	core.StateOf[Form]
	fields []formEntry
}

// CreateState implements core.StatefulWidget.
func (Form) CreateState() core.State { return &formState{} }

// InitState implements core.State.
func (s *formState) InitState() {
	if c := s.Widget().Controller; c != nil {
		c.form = s
	}
}

// DidUpdateWidget implements core.State.
func (s *formState) DidUpdateWidget(old core.Widget) {
	if c := old.(Form).Controller; c != nil && c.form == s {
		c.form = nil
	}
	if c := s.Widget().Controller; c != nil {
		c.form = s
	}
}

// Dispose implements core.State.
func (s *formState) Dispose() {
	if c := s.Widget().Controller; c != nil && c.form == s {
		c.form = nil
	}
}

// Build implements core.State.
func (s *formState) Build(core.BuildContext) core.Widget {
	return core.Inherited[*formState]{Value: s, Child: s.Widget().Child}
}

func (s *formState) submit() bool {
	var first formEntry
	var firstAt geom.Rect
	for _, f := range s.fields {
		if f.check() {
			continue
		}
		if at := f.rect(); first == nil || before(at, firstAt) {
			first, firstAt = f, at
		}
	}
	if first != nil {
		first.focus()
		return false
	}
	if on := s.Widget().OnSubmit; on != nil {
		on()
	}
	return true
}

// before reports whether a comes before b in reading order.
func before(a, b geom.Rect) bool {
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.X < b.X
}

// formEntry is a FormItem as its Form and its control see it.
type formEntry interface {
	// check validates the value now, shows the error and reports validity.
	check() bool
	reset()
	focus()
	rect() geom.Rect
	// focusChanged hears from the control inside about the keyboard.
	focusChanged(focused bool)
}

// fieldScope is what a FormItem tells the control inside it: where to report
// the keyboard, the Form that Enter submits, and the error to show.
type fieldScope struct {
	field formEntry
	// form is the Form that Enter in a text field submits, or nil.
	form *formState
	err  string
	// required says the field must be filled, for a screen reader.
	required bool
}

// submit submits the Form around the field, as Enter in a text field without
// an OnSubmit of its own does. It does nothing outside a Form.
func (f fieldScope) submit() {
	if f.form != nil {
		f.form.submit()
	}
}

// errorOnly returns the scope that the fields a control holds inside see,
// such as the hex field of a ColorPicker: they show the error and nothing
// else, since the control speaks for the field.
func (f fieldScope) errorOnly() fieldScope {
	return fieldScope{err: f.err}
}

// state returns the Invalid and Required states of the control.
func (f fieldScope) state() semantics.State {
	var s semantics.State
	if f.err != "" {
		s |= semantics.Invalid
	}
	if f.required {
		s |= semantics.Required
	}
	return s
}

// fieldScopeOf returns the scope of the FormItem around ctx, or the zero
// scope outside one.
func fieldScopeOf(ctx core.BuildContext) fieldScope {
	s, _ := core.DependOn[fieldScope](ctx)
	return s
}

// focusChanged reports the keyboard to the field, if there is one.
func (f fieldScope) focusChanged(focused bool) {
	if f.field != nil {
		f.field.focusChanged(focused)
	}
}

// FormItem checks the value of the control in Child and shows the error below
// it. The field controls of this package inside (Input, Search, NumberField,
// Autocomplete, TokenField, DropdownMenu, DatePicker, TimePicker, ColorPicker)
// also turn their border red and say the error to a screen reader.
//
// Value is the value the control shows, which the caller keeps, as for the
// control itself. Go cannot infer T from a literal, so write it:
// FormItem[string]{...}.
//
// The field waits before it judges: it checks when the control loses the
// keyboard and when the Form is submitted, never while someone types into it
// for the first time. Once it has shown an error, it checks on every change, so
// that the error goes as soon as the value is right. Controls that do not
// report the keyboard, such as Checkbox, are checked on submit.
type FormItem[T comparable] struct {
	core.StatefulBase
	Value T
	// Required makes the zero value an error: an empty or blank string, false,
	// the zero time, or no choice in a DropdownMenu. The error says what the
	// control asks for: Strings.Required (入力してください, Required) for a
	// text field; Strings.RequiredChoice (選択してください, Choose one) for a
	// DropdownMenu, DatePicker, TimePicker, ColorPicker, RadioGroup,
	// SegmentedControl or a Chip that toggles; Strings.RequiredCheck
	// (チェックしてください, Check this box) for a Checkbox and any other
	// control of a bool.
	Required bool
	// RequiredMessage, if set, is the error of Required in place of the
	// words of the control.
	RequiredMessage string
	// Validators each return an error for a wrong value, or "" for a right
	// one. The first error wins, after Required.
	Validators []func(T) string
	Child      core.Widget
}

type formFieldState[T comparable] struct {
	core.StateOf[FormItem[T]]
	form *formState
	// report is the kind of the control inside, which it tells as it builds.
	report kit.FieldReport
	err    string
	// live means the field has shown an error and checks on every change.
	live bool
	// due asks the next build to check, after a blur.
	due bool
}

// CreateState implements core.StatefulWidget.
func (FormItem[T]) CreateState() core.State { return &formFieldState[T]{} }

// InitState implements core.State.
func (s *formFieldState[T]) InitState() {
	if f, ok := core.Lookup[*formState](s.Context()); ok {
		s.form = f
		f.fields = append(f.fields, s)
	}
}

// Dispose implements core.State.
func (s *formFieldState[T]) Dispose() {
	if s.form != nil {
		if i := slices.Index(s.form.fields, formEntry(s)); i >= 0 {
			s.form.fields = slices.Delete(s.form.fields, i, i+1)
		}
	}
}

// Build implements core.State.
func (s *formFieldState[T]) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	words := basic.StringsOf(ctx)
	switch {
	case s.due:
		s.due = false
		s.err = s.validate(words)
		s.live = s.live || s.err != ""
	case s.live:
		s.err = s.validate(words)
	}
	// The child keeps its place whether or not an error follows it, so that
	// it keeps its State.
	children := make([]core.Widget, 1, 2)
	children[0] = w.Child
	if s.err != "" {
		th := basic.ThemeOf(ctx)
		children = append(children, interact.Semantics{
			Role: semantics.RoleText, Label: s.err, Live: semantics.LivePolite,
			Child: basic.Text{Content: s.err, Style: kit.Colored(th.Text.Caption, th.Color.Danger)},
		})
	}
	scope := fieldScope{field: s, form: s.form, err: s.err, required: w.Required}
	return core.Inherited[*kit.FieldReport]{Value: &s.report, Child: core.Inherited[fieldScope]{Value: scope, Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: formErrorGap, Children: children,
	}}}
}

// validate returns the error of the current value, or "".
func (s *formFieldState[T]) validate(words *basic.Strings) string {
	w := s.Widget()
	if w.Required && blank(w.Value) {
		return s.requiredMessage(words)
	}
	for _, v := range w.Validators {
		if err := v(w.Value); err != "" {
			return err
		}
	}
	return ""
}

// requiredMessage returns the error of a value left empty, in the words of
// the control inside.
func (s *formFieldState[T]) requiredMessage(words *basic.Strings) string {
	w := s.Widget()
	if w.RequiredMessage != "" {
		return w.RequiredMessage
	}
	kind := s.report.Kind
	if _, ok := any(w.Value).(bool); ok && kind == kit.FieldText {
		// A bool is never typed: it is a Switch or a control of the
		// application.
		kind = kit.FieldCheck
	}
	switch kind {
	case kit.FieldChoice:
		return words.RequiredChoice
	case kit.FieldCheck:
		return words.RequiredCheck
	}
	return words.Required
}

// blank reports whether v counts as left empty.
func blank[T comparable](v T) bool {
	if s, ok := any(v).(string); ok {
		return strings.TrimSpace(s) == ""
	}
	var zero T
	return v == zero
}

func (s *formFieldState[T]) check() bool {
	s.due = false
	s.err = s.validate(basic.LookupStrings(s.Context()))
	s.live = s.live || s.err != ""
	kit.Rebuild(&s.StateBase)
	return s.err == ""
}

func (s *formFieldState[T]) reset() {
	s.err, s.live, s.due = "", false, false
	kit.Rebuild(&s.StateBase)
}

// focusChanged hears the keyboard leave or reach the control. A control of
// several parts, such as a TimePicker, tells both as the keyboard moves from
// one part to the next; the arrival then takes back the blur before the build
// that would check.
func (s *formFieldState[T]) focusChanged(focused bool) {
	switch {
	case !s.Mounted():
	case focused:
		s.due = false
	default:
		s.due = true
		kit.Rebuild(&s.StateBase)
	}
}

func (s *formFieldState[T]) focus() {
	if t := kit.FirstFocusable(s.Context().RenderObject()); t != nil {
		core.RestoreFocus(t)
	}
}

func (s *formFieldState[T]) rect() geom.Rect {
	if ro := s.Context().RenderObject(); ro != nil {
		return kit.GlobalRect(ro)
	}
	return geom.Rect{}
}

// MinLength returns a validator for FormItem that wants at least n
// characters, and otherwise says message. An empty string passes, so that an
// optional field may stay empty; add Required for one that may not.
func MinLength(n int, message string) func(string) string {
	if message == "" {
		panic("gowaku: MinLength needs a message to show")
	}
	return func(s string) string {
		if s != "" && utf8.RuneCountInString(s) < n {
			return message
		}
		return ""
	}
}

// Pattern returns a validator for FormItem that wants the whole string to
// match re, and otherwise says message. An empty string passes, as for
// MinLength.
func Pattern(re *regexp.Regexp, message string) func(string) string {
	if message == "" {
		panic("gowaku: Pattern needs a message to show")
	}
	return func(s string) string {
		if s == "" {
			return ""
		}
		if loc := re.FindStringIndex(s); len(loc) < 2 || loc[0] != 0 || loc[1] != len(s) {
			return message
		}
		return ""
	}
}
