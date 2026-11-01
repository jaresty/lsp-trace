//go:build darwin || linux

package privatehostproductv3

import "reflect"

func AdmitCohortReport(r CohortReport) bool { return admitCohortGraph(r) }

// Preserve the existing conservative 4 MiB application field-graph bound BEFORE
// marshal. This is not streaming, practical amplification accounting, or heap
// measurement. Only the concrete acyclic cohort DTOs reach this private helper.
func admitCohortGraph(value any) bool {
	b := newEncodedBudget()
	var walk func(reflect.Value, int) bool
	walk = func(v reflect.Value, depth int) bool {
		if depth > 20 || !b.fixed(64) {
			return false
		}
		switch v.Kind() {
		case reflect.String:
			return b.strings(v.String())
		case reflect.Struct:
			for n := 0; n < v.NumField(); n++ {
				if !walk(v.Field(n), depth+1) {
					return false
				}
			}
		case reflect.Array, reflect.Slice:
			if v.Len() > 256 {
				return false
			}
			for n := 0; n < v.Len(); n++ {
				if !walk(v.Index(n), depth+1) {
					return false
				}
			}
		case reflect.Pointer:
			if !v.IsNil() {
				return walk(v.Elem(), depth+1)
			}
		case reflect.Bool, reflect.Int, reflect.Uint8, reflect.Uint64:
			return true
		default:
			return false
		}
		return true
	}
	return walk(reflect.ValueOf(value), 0)
}
