package capacitydiagnostic

import (
	"encoding/json"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const descriptorValidInput = `{"schema_version":"lsp-trace.census-operation-diagnostic.v3","operation":"start","reason":"CAPACITY_INVALID_QUOTE","action":"STOP","retry":false}`

// descriptorSchemaAccepts uses the independent schema engine, not the wire
// interpreter, to check that supported forms agree on parsed JSON instances.
func descriptorSchemaAccepts(t *testing.T, s map[string]any, raw string) bool {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var parsed any
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("urn:capacity-descriptor-test", parsed); err != nil {
		t.Fatal(err)
	}
	compiled, err := c.Compile("urn:capacity-descriptor-test")
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}
	return compiled.Validate(value) == nil
}

func TestDescriptorSupportedForms(t *testing.T) {
	shared := wireRule{kind: "object", object: []wireField{{name: "x", rule: constant(true)}}}
	for _, tc := range []struct {
		name      string
		rule      wireRule
		good, bad string
	}{
		{"string", constant("x"), `"x"`, `"y"`},
		{"bool", constant(false), `false`, `null`},
		{"uint64", constant(uint64(1)), `1`, `2`},
		{"enum", wireRule{values: []any{"x", "y"}}, `"y"`, `"z"`},
		{"bool_enum", wireRule{values: []any{true, false}}, `true`, `null`},
		{"uint64_enum", wireRule{values: []any{uint64(0), uint64(2)}}, `2`, `1`},
		{"zero_range", integer(0, 0), `0`, `1`},
		{"range", integer(1, 3), `3`, `4`},
		{"multiple", wireRule{kind: "integer", minimum: 1, maximum: 10, multiple: 2}, `2`, `3`},
		{"optional", wireRule{kind: "object", object: []wireField{{name: "x", optional: true, rule: constant(true)}}}, `{}`, `{"x":false}`},
		{"shared_acyclic", wireRule{kind: "object", object: []wireField{{name: "a", rule: shared}, {name: "b", rule: shared}}}, `{"a":{"x":true},"b":{"x":true}}`, `{"a":{"x":true}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.rule.schema()
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range []struct {
				raw  string
				want bool
			}{{tc.good, true}, {tc.bad, false}} {
				if got := tc.rule.valid(json.RawMessage(v.raw)); got != v.want {
					t.Errorf("wire acceptance=%v want=%v", got, v.want)
				}
				if got := descriptorSchemaAccepts(t, s, v.raw); got != v.want {
					t.Errorf("schema acceptance=%v want=%v", got, v.want)
				}
			}
		})
	}
}

func TestDescriptorCounterexamples(t *testing.T) {
	for _, tc := range []struct {
		name string
		rule wireRule
		raw  string
	}{
		{"unsupported_string", wireRule{kind: "string"}, `"x"`},
		{"empty", wireRule{}, `true`},
		{"enum_bypasses_range", wireRule{kind: "integer", values: []any{uint64(1)}, maximum: 0}, `1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.rule.valid(json.RawMessage(tc.raw)) {
				t.Error("invalid descriptor admitted input")
			}
			if s, err := tc.rule.schema(); err == nil || s != nil {
				t.Errorf("invalid descriptor published schema: schema=%v error=%v", s, err)
			}
		})
	}
}

func TestDescriptorMalformedEntrypoints(t *testing.T) {
	cycleFields := []wireField{{name: "loop"}}
	cycle := wireRule{kind: "object", object: cycleFields}
	cycleFields[0].rule = cycle
	cases := []struct {
		name string
		rule wireRule
	}{
		{"empty", wireRule{}}, {"unknown_kind", wireRule{kind: "unknown"}},
		{"unsupported_string", wireRule{kind: "string"}},
		{"empty_values", wireRule{values: []any{}}},
		{"nil_constant", constant(nil)}, {"int_constant", constant(1)},
		{"float_constant", constant(float64(1))}, {"slice_constant", constant([]any{true})},
		{"map_constant", constant(map[string]any{"x": true})},
		{"invalid_utf8_constant", constant(string([]byte{0xff}))},
		{"pointer_constant", constant(new(bool))},
		{"invalid_utf8_field", wireRule{kind: "object", object: []wireField{{name: string([]byte{0xff}), rule: constant(true)}}}},
		{"mixed_enum", wireRule{values: []any{"x", uint64(1)}}},
		{"duplicate_enum", wireRule{values: []any{"x", "x"}}},
		{"kind_and_values", wireRule{kind: "integer", values: []any{uint64(1)}}},
		{"values_and_minimum", wireRule{values: []any{uint64(1)}, minimum: 1}},
		{"values_and_maximum", wireRule{values: []any{uint64(1)}, maximum: 2}},
		{"values_and_multiple", wireRule{values: []any{uint64(1)}, multiple: 1}},
		{"values_and_object", wireRule{values: []any{"x"}, object: []wireField{}}},
		{"inverted_range", integer(2, 1)},
		{"integer_object", wireRule{kind: "integer", object: []wireField{}}},
		{"integer_empty_enum", wireRule{kind: "integer", values: []any{}}},
		{"nil_object", wireRule{kind: "object"}},
		{"empty_object", wireRule{kind: "object", object: []wireField{}}},
		{"object_range", wireRule{kind: "object", minimum: 1, object: []wireField{{name: "x", rule: constant(true)}}}},
		{"empty_field_name", wireRule{kind: "object", object: []wireField{{rule: constant(true)}}}},
		{"duplicate_field", wireRule{kind: "object", object: []wireField{{name: "x", rule: constant(true)}, {name: "x", rule: constant(false)}}}},
		{"nested_invalid", wireRule{kind: "object", object: []wireField{{name: "x", optional: true}}}},
		{"cycle", cycle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := capacityShape
			// Unused optional malformed descriptors must still block the root gate.
			capacityShape.object = append(append([]wireField(nil), old.object...), wireField{name: "unused", optional: true, rule: tc.rule})
			defer func() { capacityShape = old }()
			assertDescriptorEntrypointsReject(t)
			if tc.rule.valid(json.RawMessage(`true`)) {
				t.Error("malformed rule admitted input")
			}
			if s, err := tc.rule.schema(); err == nil || s != nil {
				t.Error("malformed rule published schema")
			}
		})
	}
}

func assertDescriptorEntrypointsReject(t *testing.T) {
	t.Helper()
	if _, err := DecodeCapacityDiagnostic([]byte(descriptorValidInput)); err == nil {
		t.Error("malformed descriptor admitted at decode entrypoint")
	}
	if raw, err := CapacityDiagnosticSchema(); err == nil || raw != nil {
		t.Errorf("malformed descriptor published at exported entrypoint: bytes=%d error=%v", len(raw), err)
	}
}

func TestDescriptorMalformedBranches(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"root_not_object", func() { capacityShape = constant(true) }},
		{"nil_branches", func() { capacityReasons = nil }},
		{"missing_reason", func() { capacityShape.object[2].name = "other" }},
		{"missing_metrics", func() { capacityShape.object[5].name = "other" }},
		{"wide_refinement", func() { capacityReasons[1].metrics[1].rule = integer(0, 1<<33) }},
		{"range_for_enum", func() { capacityReasons[1].metrics[0].rule = integer(0, 1) }},
		{"nested_refinement", func() {
			capacityReasons[1].metrics[0].rule = wireRule{kind: "object", object: []wireField{{name: "x", rule: constant(true)}}}
		}},
		{"branch_cycle", func() {
			f := []wireField{{name: "unit"}}
			f[0].rule = wireRule{kind: "object", object: f}
			capacityReasons[1].metrics = f
		}},
		{"incompatible_multiple", func() { capacityShape.object[5].rule.object[2].rule.multiple = 3 }},
		{"missing_multiple", func() { capacityShape.object[5].rule.object[3].rule.multiple = 2 }},

		{"empty_reason", func() { capacityReasons[1].reason = "" }},
		{"duplicate_reason", func() { capacityReasons[1].reason = capacityReasons[0].reason }},
		{"unknown_reason", func() { capacityReasons[1].reason = "UNKNOWN" }},
		{"missing_branch", func() { capacityReasons = capacityReasons[:3] }},
		{"empty_metrics", func() { capacityReasons[1].metrics = []wireField{} }},
		{"optional_refinement", func() { capacityReasons[1].metrics[0].optional = true }},
		{"unknown_refinement", func() { capacityReasons[1].metrics[0].name = "unknown" }},
		{"duplicate_refinement", func() { capacityReasons[1].metrics = append(capacityReasons[1].metrics, capacityReasons[1].metrics[0]) }},
		{"invalid_refinement", func() { capacityReasons[1].metrics[0].rule = wireRule{kind: "string"} }},
		{"incompatible_refinement", func() { capacityReasons[1].metrics[0].rule = constant(uint64(1)) }},
		{"optional_reason", func() { capacityShape.object[2].optional = true }},
		{"required_metrics", func() { capacityShape.object[5].optional = false }},
		{"optional_metric_field", func() { capacityShape.object[5].rule.object[0].optional = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldShape, oldReasons := capacityShape, capacityReasons
			capacityShape.object = append([]wireField(nil), oldShape.object...)
			capacityShape.object[5].rule.object = append([]wireField(nil), oldShape.object[5].rule.object...)
			capacityReasons = append([]reasonShape(nil), oldReasons...)
			for i := range capacityReasons {
				if capacityReasons[i].metrics != nil {
					capacityReasons[i].metrics = append([]wireField{}, capacityReasons[i].metrics...)
				}
			}
			defer func() { capacityShape, capacityReasons = oldShape, oldReasons }()
			tc.change()
			assertDescriptorEntrypointsReject(t)
		})
	}
}
