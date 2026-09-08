package retainedcalls

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Walk only typed acquisition slices, never []byte/RawMessage contents. Paths
// are logical JSON pointers. This preserves exact-number and opaque payloads.
func acquisitionSlicesV2(v reflect.Value, path string, visit func(reflect.Value, string) error) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return acquisitionSlicesV2(v.Elem(), path, visit)
	}
	switch v.Kind() {
	case reflect.Struct:
		typ := v.Type()
		for i := 0; i < v.NumField(); i++ {
			tag := typ.Field(i).Tag.Get("json")
			if strings.Contains(tag, ",omitempty") && v.Field(i).Kind() == reflect.Slice && v.Field(i).Len() == 0 {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = typ.Field(i).Name
			}
			if err := acquisitionSlicesV2(v.Field(i), path+"/"+name, visit); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return nil
		}
		if err := visit(v, path); err != nil {
			return err
		}
		for i := 0; i < v.Len(); i++ {
			if err := acquisitionSlicesV2(v.Index(i), fmt.Sprintf("%s/%d", path, i), visit); err != nil {
				return err
			}
		}
	}
	return nil
}
func normalizeArraysV2(a AcquisitionV2) (AcquisitionV2, []string, error) {
	// Clone before changing nested slices: never mutate the admitted coordinator.
	raw, err := json.Marshal(a)
	if err != nil {
		return a, nil, err
	}
	var out AcquisitionV2
	if err = json.Unmarshal(raw, &out); err != nil {
		return a, nil, err
	}
	markers := []string{}
	err = acquisitionSlicesV2(reflect.ValueOf(&out), "", func(v reflect.Value, path string) error {
		if v.IsNil() {
			markers = append(markers, path)
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}
		return nil
	})
	sort.Strings(markers)
	return out, markers, err
}
func restoreArraysV2(a AcquisitionV2, markers []string) (AcquisitionV2, error) {
	if markers == nil {
		return a, errors.New("V2 missing acquisition null markers")
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return a, err
	}
	var out AcquisitionV2
	if err = json.Unmarshal(raw, &out); err != nil {
		return a, err
	}
	wanted := map[string]bool{}
	prior := ""
	for _, path := range markers {
		if path <= prior {
			return a, errors.New("V2 duplicate/unsorted null marker")
		}
		wanted[path] = true
		prior = path
	}
	err = acquisitionSlicesV2(reflect.ValueOf(&out), "", func(v reflect.Value, path string) error {
		if v.IsNil() {
			return fmt.Errorf("V2 typed acquisition arrays must not be null: %s", path)
		}
		if wanted[path] {
			if v.Len() != 0 {
				return errors.New("V2 nonempty null-marked array")
			}
			v.Set(reflect.Zero(v.Type()))
			delete(wanted, path)
		}
		return nil
	})
	if err == nil && len(wanted) > 0 {
		err = errors.New("V2 dangling acquisition null marker")
	}
	return out, err
}
