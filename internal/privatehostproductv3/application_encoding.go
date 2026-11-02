//go:build darwin || linux

package privatehostproductv3

import (
	"encoding/json"
	"reflect"
)

// Closed acyclic application DTO graph admission BEFORE encoding/json allocates
// an encoder buffer. This is a per-record bound, never an application-total cap.
func applicationAdmit(v any, limit int) bool {
	b := &encodedBudget{left: limit, work: 10000}
	var walk func(reflect.Value, int) bool
	walk = func(v reflect.Value, depth int) bool {
		if !v.IsValid() || depth > 20 || !b.fixed(32) {
			return false
		}
		switch v.Kind() {
		case reflect.String:
			return b.strings(v.String())
		case reflect.Struct:
			for n := 0; n < v.NumField(); n++ {
				if !b.strings(v.Type().Field(n).Name) || !walk(v.Field(n), depth+1) {
					return false
				}
			}
		case reflect.Array, reflect.Slice:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				return v.Len() <= limit/2 && b.fixed(((v.Len()+2)/3)*4+2)
			}
			if v.Len() > 256 {
				return false
			}
			for n := 0; n < v.Len(); n++ {
				if !walk(v.Index(n), depth+1) {
					return false
				}
			}
		case reflect.Bool, reflect.Int, reflect.Int64, reflect.Uint8, reflect.Uint64:
			return true
		default:
			return false
		}
		return true
	}
	return walk(reflect.ValueOf(v), 0)
}
func applicationMarshal(v any, limit int) ([]byte, error) {
	if !applicationAdmit(v, limit) {
		return nil, ErrInvalid
	}
	raw, e := json.Marshal(v)
	if e != nil || len(raw) > limit {
		return nil, ErrInvalid
	}
	return raw, nil
}
