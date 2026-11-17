package testutil

import "reflect"

// EmptyFields returns the names of the fields of the struct *ptr that are left
// out: empty strings, arrays of strings with an empty element, and nil
// functions when funcs is true. Tests use it to check that a table is
// complete; the library itself never walks a struct (CODING_STYLE §5.2).
func EmptyFields(ptr any, funcs bool) []string {
	var out []string
	v := reflect.ValueOf(ptr).Elem()
	for i := range v.NumField() {
		f, name := v.Field(i), v.Type().Field(i).Name
		switch f.Kind() {
		case reflect.String:
			if f.String() == "" {
				out = append(out, name)
			}
		case reflect.Array:
			if f.Type().Elem().Kind() != reflect.String {
				continue
			}
			for j := range f.Len() {
				if f.Index(j).String() == "" {
					out = append(out, name)
					break
				}
			}
		case reflect.Func:
			if funcs && f.IsNil() {
				out = append(out, name)
			}
		}
	}
	return out
}
