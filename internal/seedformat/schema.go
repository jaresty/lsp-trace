package seedformat

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema.json
var schemaJSON []byte

var (
	schemaOnce     sync.Once
	compiledSchema *jsonschema.Schema
	schemaErr      error
)

// SchemaJSON returns an isolated copy of the exact committed schema bytes.
func SchemaJSON() []byte { return append([]byte(nil), schemaJSON...) }

// ValidateSchema validates raw JSON against the structural seed-file contract.
// Decode additionally applies duplicate-member and workspace path policy checks.
func ValidateSchema(raw []byte) error {
	schemaOnce.Do(func() {
		resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
		if err != nil {
			schemaErr = err
			return
		}
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource("schema.json", resource); err != nil {
			schemaErr = err
			return
		}
		compiledSchema, schemaErr = compiler.Compile("schema.json")
	})
	if schemaErr != nil {
		return schemaErr
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON value required")
	}
	return compiledSchema.Validate(value)
}
