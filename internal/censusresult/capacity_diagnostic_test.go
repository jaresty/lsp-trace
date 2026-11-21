package censusresult_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	cr "lsp-trace/internal/censusresult"
	"lsp-trace/internal/schema"
	"testing"
)

func TestCapacityDiagnosticParity(t *testing.T) {
	for _, tc := range []struct {
		reason, unit            string
		spent, protected, limit uint64
	}{
		{"CAPACITY_BYTES", "bytes", 14825416, 54463091, 67108864},
		{"CAPACITY_WORK", "work_units", 4294967201, 96, 4294967296},
		{"CAPACITY_PACKETS", "packets", 0, 65, 64},
		{"CAPACITY_INVALID_QUOTE", "", 0, 0, 0},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			d := cr.CapacityDiagnostic{SchemaVersion: cr.CapacityDiagnosticSchemaVersion, Operation: "start", Reason: tc.reason, Action: "STOP"}
			if tc.unit != "" {
				d.Metrics = &cr.CapacityDiagnosticMetrics{Unit: tc.unit, Spent: tc.spent, Protected: tc.protected, Limit: tc.limit, Available: tc.limit - tc.spent, Deficit: tc.protected - (tc.limit - tc.spent)}
			}
			raw, e := cr.MarshalCapacityDiagnostic(d)
			if e != nil {
				t.Fatal(e)
			}
			check := func(name string, raw []byte, want bool) {
				t.Helper()
				_, ce := cr.DecodeCapacityDiagnostic(raw)
				_, se := schema.ValidateFor(raw, schema.FamilyCensusCapacityDiagnostic, "v3")
				if (ce == nil) != want || (se == nil) != want {
					t.Fatalf("%s parity codec=%v schema=%v want=%v: %s", name, ce, se, want, raw)
				}
			}
			check("valid", raw, true)
			if _, e := cr.DecodeOperationDiagnostic(raw); e == nil {
				t.Fatal("legacy accepted v3")
			}
			for name, bad := range map[string][]byte{
				"unknown":   bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":false,"path":"secret"`), 1),
				"duplicate": bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":false,"retry":false`), 1),
				"null":      bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":null`), 1),
				"missing":   bytes.Replace(raw, []byte(`"retry":false`), []byte(`"extra":false`), 1),
				"trailing":  append(append([]byte{}, raw...), []byte(`{}`)...),
				"retry":     bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":true`), 1),
				"action":    bytes.Replace(raw, []byte(`"STOP"`), []byte(`"RETRY"`), 1),
				"reason":    bytes.Replace(raw, []byte(tc.reason), []byte("OTHER"), 1),
				"operation": bytes.Replace(raw, []byte(`"start"`), []byte(`"task"`), 1),
				"recovery":  bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":false,"recovery":{}`), 1),
			} {
				check(name, bad, false)
			}
			if d.Metrics != nil {
				for _, key := range []string{"spent", "protected", "limit", "available", "deficit", "unit"} {
					var obj map[string]any
					_ = json.Unmarshal(raw, &obj)
					m := obj["metrics"].(map[string]any)
					delete(m, key)
					bad, _ := json.Marshal(obj)
					check("missing_"+key, bad, false)
				}
				for _, key := range []string{"spent", "protected", "limit", "available", "deficit"} {
					var obj map[string]any
					_ = json.Unmarshal(raw, &obj)
					m := obj["metrics"].(map[string]any)
					m[key] = m[key].(float64) + 1
					bad, _ := json.Marshal(obj)
					check("inconsistent_"+key, bad, false)
				}
				for _, value := range []string{"null", "-1", "1.5", "18446744073709551615", "18446744073709551616"} {
					old := fmt.Sprintf(`"deficit":%d`, d.Metrics.Deficit)
					check("badnumber", bytes.Replace(raw, []byte(old), []byte(`"deficit":`+value), 1), false)
				}
				check("nested_duplicate", bytes.Replace(raw, []byte(`"unit":`), []byte(`"spent":0,"unit":`), 1), false)
			}
		})
	}
}
