package censusresult_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/capacitydiagnostic"
	cr "lsp-trace/internal/censusresult"
	"lsp-trace/internal/schema"
)

func TestCapacityShapePublication(t *testing.T) {
	published, err := schema.BytesFor(schema.FamilyCensusCapacityDiagnostic, "v3")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := capacitydiagnostic.CapacityDiagnosticSchema()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(published, generated) {
		t.Fatal("generated V3 schema drift: run go generate ./internal/capacitydiagnostic")
	}
}

// A single deterministic corpus supplies both full-contract entry points and
// explicitly distinguishes structural rejection from semantic/wire limitations.
func TestCapacityShapeMutationCorpus(t *testing.T) {
	type candidate struct {
		name, raw string
		valid     bool
		// The parsed JSON instance is structurally valid, but raw-wire limits
		// or sibling arithmetic make the full contract reject it.
		structuralOnly bool
	}
	var corpus []candidate
	add := func(name, raw string) { corpus = append(corpus, candidate{name: name, raw: raw}) }
	limited := func(name, raw string) {
		corpus = append(corpus, candidate{name: name, raw: raw, structuralOnly: true})
	}
	fixtures := []cr.CapacityDiagnostic{
		{Reason: "CAPACITY_INVALID_QUOTE"},
		{Reason: "CAPACITY_BYTES", Metrics: &cr.CapacityDiagnosticMetrics{Unit: "bytes", Spent: 7, Protected: 4, Limit: 10, Available: 3, Deficit: 1}},
		{Reason: "CAPACITY_WORK", Metrics: &cr.CapacityDiagnosticMetrics{Unit: "work_units", Spent: 1<<32 - 31, Protected: 32, Limit: 1 << 32, Available: 31, Deficit: 1}},
		{Reason: "CAPACITY_PACKETS", Metrics: &cr.CapacityDiagnosticMetrics{Unit: "packets", Protected: 65, Limit: 64, Available: 64, Deficit: 1}},
		{Reason: "CAPACITY_BYTES", Metrics: &cr.CapacityDiagnosticMetrics{Unit: "bytes", Protected: 1, Limit: 0, Available: 0, Deficit: 1}},
		{Reason: "CAPACITY_BYTES", Metrics: &cr.CapacityDiagnosticMetrics{Unit: "bytes", Spent: 64 << 20, Protected: 64 << 20, Limit: 64 << 20, Deficit: 64 << 20}},
		{Reason: "CAPACITY_WORK", Metrics: &cr.CapacityDiagnosticMetrics{Unit: "work_units", Spent: 1 << 32, Protected: 32 * (64 << 20), Limit: 1 << 32, Deficit: 32 * (64 << 20)}},
		{Reason: "CAPACITY_PACKETS", Metrics: &cr.CapacityDiagnosticMetrics{Unit: "packets", Protected: 423, Limit: 64, Available: 64, Deficit: 359}},
	}
	for _, fixture := range fixtures {
		fixture.SchemaVersion, fixture.Operation, fixture.Action = cr.CapacityDiagnosticSchemaVersion, "start", "STOP"
		encoded, err := cr.MarshalCapacityDiagnostic(fixture)
		if err != nil {
			t.Fatal(err)
		}
		raw := strings.TrimSpace(string(encoded))
		prefix := fixture.Reason + "/"
		corpus = append(corpus, candidate{name: prefix + "canonical", raw: raw, valid: true})
		corpus = append(corpus, candidate{name: prefix + "whitespace", raw: " \n" + raw + "\t", valid: true})
		add(prefix+"trailing_object", raw+"{}")
		add(prefix+"trailing_junk", raw+"x")
		limited(prefix+"oversize", raw+strings.Repeat(" ", 4096))
		var top map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &top); err != nil {
			t.Fatal(err)
		}
		for _, scope := range []string{"top", "metrics"} {
			fields := top
			if scope == "metrics" {
				if fixture.Metrics == nil {
					continue
				}
				fields = make(map[string]json.RawMessage)
				if err := json.Unmarshal(top["metrics"], &fields); err != nil {
					t.Fatal(err)
				}
			}
			keys := make([]string, 0, len(fields))
			for key := range fields {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			wrap := func(object string) string {
				if scope == "top" {
					return object
				}
				return strings.Replace(raw, string(top["metrics"]), object, 1)
			}
			// Keep raw pairs rather than round-tripping mutated objects through
			// a map: duplicate names and their order are the observation.
			mutate := func(key, replacement string) string {
				pairs := []string{}
				for _, k := range keys {
					pair := fmt.Sprintf("%q:%s", k, fields[k])
					if k == key {
						pair = replacement
					}
					if pair != "" {
						pairs = append(pairs, pair)
					}
				}
				return wrap("{" + strings.Join(pairs, ",") + "}")
			}
			for _, key := range keys {
				value := string(fields[key])
				pair := fmt.Sprintf("%q:%s", key, value)
				name := prefix + scope + "/" + key + "/"
				for _, variant := range []string{strings.ToUpper(key[:1]) + key[1:], strings.ToUpper(key)} {
					alias := fmt.Sprintf("%q:%s", variant, value)
					add(name+variant+"_only", mutate(key, alias))
					add(name+variant+"_extra", mutate(key, pair+","+alias))
					bad := fmt.Sprintf("%q:null", key)
					add(name+variant+"_overwrite_after", mutate(key, bad+","+alias))
					add(name+variant+"_overwrite_before", mutate(key, alias+","+bad))
				}
				limited(name+"duplicate", mutate(key, pair+","+pair))
				escaped := fmt.Sprintf(`"\u%04x%s":%s`, key[0], key[1:], value)
				limited(name+"escaped_duplicate", mutate(key, pair+","+escaped))
				limited(name+"escaped_duplicate_reverse", mutate(key, escaped+","+pair))
				corpus = append(corpus, candidate{name: name + "escaped_canonical", raw: mutate(key, escaped), valid: true})
				if key != "metrics" || fixture.Metrics != nil {
					add(name+"missing", mutate(key, ""))
				}
				for i, wrong := range []string{"null", "[]", "{}", `"INVALID"`, "true", "-1", "1.5", "18446744073709551615", "18446744073709551616"} {
					add(fmt.Sprintf("%stype_%d", name, i), mutate(key, fmt.Sprintf("%q:%s", key, wrong)))
				}
			}
			first := keys[0]
			for _, unknown := range []string{"unknown", "path", "identity", "head", "run_locator"} {
				add(prefix+scope+"/extra_"+unknown, mutate(first, fmt.Sprintf("%q:%s,%q:0", first, fields[first], unknown)))
			}
		}
		if fixture.Metrics == nil {
			add(prefix+"null_metrics", strings.TrimSuffix(raw, "}")+`,"metrics":null}`)
			add(prefix+"forbidden_metrics", strings.TrimSuffix(raw, "}")+`,"metrics":{"unit":"packets","spent":0,"protected":65,"limit":64,"available":64,"deficit":1}}`)
		} else {
			for _, unit := range []string{"bytes", "work_units", "packets"} {
				if unit != fixture.Metrics.Unit {
					add(prefix+"conflicting_unit_"+unit, strings.Replace(raw, `"unit":"`+fixture.Metrics.Unit+`"`, `"unit":"`+unit+`"`, 1))
				}
			}
			for _, reason := range []string{"CAPACITY_INVALID_QUOTE", "CAPACITY_BYTES", "CAPACITY_WORK", "CAPACITY_PACKETS"} {
				if reason != fixture.Reason {
					add(prefix+"conflicting_reason_"+reason, strings.Replace(raw, fixture.Reason, reason, 1))
				}
			}
		}
	}
	// Arithmetic violations intentionally admitted by standalone structure.
	for _, tc := range []struct{ name, metrics string }{
		{"subtraction_underflow", `"spent":11,"protected":4,"limit":10,"available":0,"deficit":4`},
		{"equality_not_refusal", `"spent":7,"protected":3,"limit":10,"available":3,"deficit":1`},
		{"wrong_available", `"spent":7,"protected":4,"limit":10,"available":2,"deficit":2`},
		{"wrong_deficit", `"spent":7,"protected":4,"limit":10,"available":3,"deficit":2`},
	} {
		raw := `{"schema_version":"lsp-trace.census-operation-diagnostic.v3","operation":"start","reason":"CAPACITY_BYTES","action":"STOP","retry":false,"metrics":{"unit":"bytes",` + tc.metrics + `}}`
		corpus = append(corpus, candidate{name: "arithmetic/" + tc.name, raw: raw, structuralOnly: true})
	}
	for _, root := range []string{"null", "[]", "true", "0", `"string"`, ""} {
		add("invalid_root/"+root, root)
	}
	for _, tc := range corpus {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, ce := cr.DecodeCapacityDiagnostic([]byte(tc.raw))
			_, se := schema.ValidateFor([]byte(tc.raw), schema.FamilyCensusCapacityDiagnostic, "v3")
			if (ce == nil) != tc.valid || (se == nil) != tc.valid {
				t.Fatalf("full-contract acceptance want=%v codec=%v composed=%v raw=%s", tc.valid, ce, se, tc.raw)
			}
			// JSON Schema consumes parsed instances, not raw duplicate keys or
			// byte counts. Explicitly model that boundary instead of claiming
			// the standalone schema can enforce the full wire contract.
			var parsed any
			if err := json.Unmarshal([]byte(tc.raw), &parsed); err == nil {
				collapsed, err := json.Marshal(parsed)
				if err != nil {
					t.Fatal(err)
				}
				_, err = schema.ValidateStructure(collapsed, schema.FamilyCensusCapacityDiagnostic, "v3")
				if (err == nil) != (tc.valid || tc.structuralOnly) {
					t.Fatalf("standalone structural classification want=%v: %v raw=%s", tc.valid || tc.structuralOnly, err, tc.raw)
				}
			} else if tc.valid || tc.structuralOnly {
				t.Fatalf("classified structural instance cannot be parsed: %v", err)
			}
		})
	}
	t.Logf("deterministic corpus: %d cases", len(corpus))
}

func TestCapacityShapeDuplicateStructuralLimitation(t *testing.T) {
	raw := []byte(`{"schema_version":"lsp-trace.census-operation-diagnostic.v3","operation":"start","reason":"CAPACITY_INVALID_QUOTE","action":"STOP","retry":false,"\u0072etry":false}`)
	// Once a JSON parser has collapsed duplicate decoded names, JSON Schema
	// sees only the last value. Never claim raw-wire parity for that boundary.
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	collapsed, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schema.ValidateStructure(collapsed, schema.FamilyCensusCapacityDiagnostic, "v3"); err != nil {
		t.Fatal(err)
	}
	if _, err := cr.DecodeCapacityDiagnostic(raw); err == nil {
		t.Fatal("raw escaped duplicate accepted by codec")
	}
	if _, err := schema.ValidateFor(raw, schema.FamilyCensusCapacityDiagnostic, "v3"); err == nil {
		t.Fatal("raw escaped duplicate accepted by composed schema")
	}
}
