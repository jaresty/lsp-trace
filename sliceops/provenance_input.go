package sliceops

import (
	"encoding/json"
	"fmt"
	"lsp-trace/internal/strictjson"
	"reflect"
	"strings"
)

func provenanceInput(raw []byte) error {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return err
	}
	selected := false
	for k := range keys {
		if strings.EqualFold(k, "graph_provenance") {
			selected = true
		}
	}
	if !selected {
		return nil
	} // Historical omitted-mode case decoding is unchanged.
	allowed := map[string]bool{}
	typ := reflect.TypeOf(request{})
	for i := 0; i < typ.NumField(); i++ {
		allowed[strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	for k := range keys {
		if !allowed[k] {
			return fmt.Errorf("noncanonical provenance input field %q", k)
		}
	}
	if string(keys["graph_provenance"]) != "true" && string(keys["graph_provenance"]) != "false" {
		return fmt.Errorf("graph_provenance must be a boolean")
	}
	return nil
}
