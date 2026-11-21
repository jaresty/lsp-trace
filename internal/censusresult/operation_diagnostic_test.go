package censusresult

import (
	"bytes"
	"strings"
	"testing"
)

func TestOperationDiagnosticStrict(t *testing.T) {
	d := OperationDiagnostic{SchemaVersion: OperationDiagnosticSchemaVersion, Operation: "submit", Reason: OperationUnconfirmed, Action: "OBSERVE_CURRENT_TASK", Recovery: &OperationRecovery{RunID: strings.Repeat("a", 64), StoreContext: "SAME_SELECTION"}}
	raw, e := MarshalOperationDiagnostic(d)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = DecodeOperationDiagnostic(raw); e != nil {
		t.Fatal(e)
	}
	for name, bad := range map[string][]byte{
		"retry":             bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":true`), 1),
		"missing_retry":     bytes.Replace(raw, []byte(`"retry":false,`), nil, 1),
		"null_retry":        bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":null`), 1),
		"duplicate":         bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":false,"retry":false`), 1),
		"unknown":           bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":false,"secret":"marker"`), 1),
		"trailing":          append(append([]byte(nil), raw...), []byte(`{}`)...),
		"unknown_operation": bytes.Replace(raw, []byte(`"submit"`), []byte(`"execute"`), 1),
		"unsafe_action":     bytes.Replace(raw, []byte(`"OBSERVE_CURRENT_TASK"`), []byte(`"RESUBMIT"`), 1),
		"unknown_reason":    bytes.Replace(raw, []byte(`"OPERATION_UNCONFIRMED"`), []byte(`"SPENT"`), 1),
		"invalid_run":       bytes.Replace(raw, []byte(strings.Repeat("a", 64)), []byte("secret/path"), 1),
		"wrong_context":     bytes.Replace(raw, []byte(`"SAME_SELECTION"`), []byte(`"DEFAULT"`), 1),
		"unknown_version":   bytes.Replace(raw, []byte(OperationDiagnosticSchemaVersion), []byte("v999"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := DecodeOperationDiagnostic(bad); e == nil {
				t.Fatal("invalid diagnostic accepted")
			}
		})
	}
	for _, r := range []OperationReason{StoreRootMissing, StorePathInvalid, StoreAliasRefused, StorePermissions, StoreSourceOverlap, StoreIO, ConfigUnclassified} {
		d := OperationDiagnostic{SchemaVersion: OperationDiagnosticSchemaVersion, Operation: "start", Reason: r, Action: "CORRECT_CONFIG"}
		if _, e := MarshalOperationDiagnostic(d); e != nil {
			t.Fatal(e)
		}
		d.Recovery = &OperationRecovery{RunID: strings.Repeat("a", 64), StoreContext: "SAME_SELECTION"}
		if _, e := MarshalOperationDiagnostic(d); e == nil {
			t.Fatal("config recovery accepted")
		}
	}
}
func TestOperationDiagnosticLegacyPin(t *testing.T) {
	d, e := NewDiagnostic(StageAssembly, nil)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := MarshalDiagnostic(d)
	if e != nil {
		t.Fatal(e)
	}
	const want = "{\"schema_version\":\"lsp-trace.census-diagnostic.v1\",\"status\":\"FAILED\",\"stage\":\"assembly\",\"code\":\"ASSEMBLY_FAILED\",\"retry\":true}\n"
	if string(raw) != want {
		t.Fatalf("legacy changed: %s", raw)
	}
	if _, e := DecodeOperationDiagnostic(raw); e == nil {
		t.Fatal("legacy accepted as operation diagnostic")
	}
}
