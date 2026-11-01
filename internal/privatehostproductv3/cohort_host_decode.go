//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"unicode/utf8"
)

func DecodeCohortRoleReturns(raw []byte) ([2]CohortRoleReturn, error) {
	var r [2]CohortRoleReturn
	if len(raw) > 16384 || !utf8.Valid(raw) {
		return r, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if cohortSchema(d, reflect.TypeOf(r), 0) != nil || d.Decode(new(any)) != io.EOF || json.Unmarshal(raw, &r) != nil {
		return [2]CohortRoleReturn{}, ErrInvalid
	}
	return r, nil
}
