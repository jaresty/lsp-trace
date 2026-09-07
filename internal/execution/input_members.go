package execution

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// checkInputMembers uses the declared JSON names, not encoding/json's fallback
// case folding. ProductionInput and its nested input structs have no embedded
// fields or custom unmarshallers. Scalar/type validation remains decoder-owned.
func checkInputMembers(raw json.RawMessage, typ reflect.Type) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		var members map[string]json.RawMessage
		if err := json.Unmarshal(raw, &members); err != nil {
			return err
		}
		allowed := map[string]reflect.Type{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" || !f.IsExported() {
				continue
			}
			if name == "" {
				name = f.Name
			}
			allowed[name] = f.Type
		}
		for name, value := range members {
			field, ok := allowed[name]
			if !ok {
				return fmt.Errorf("unknown member %q", name)
			}
			if err := checkInputMembers(value, field); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			return nil
		} // base64 bytes, not JSON members
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return err
		}
		for _, value := range values {
			if err := checkInputMembers(value, typ.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}
