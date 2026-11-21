package censusresult

import (
	"bytes"
	"testing"
)

func TestStoredVersionDiagnosticClosed(t *testing.T) {
	d := OperationDiagnostic{SchemaVersion: StoredVersionDiagnosticSchemaVersion, Operation: "task", Reason: UnsupportedStoredVersion, Action: "STOP"}
	raw, e := MarshalOperationDiagnostic(d)
	if e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []func(*OperationDiagnostic){
		func(d *OperationDiagnostic) { d.SchemaVersion = OperationDiagnosticSchemaVersion },
		func(d *OperationDiagnostic) { d.SchemaVersion = "unknown" },
		func(d *OperationDiagnostic) { d.Operation = "start" },
		func(d *OperationDiagnostic) { d.Operation = "unknown" },
		func(d *OperationDiagnostic) { d.Reason = RunUnavailable },
		func(d *OperationDiagnostic) { d.Action = "OBSERVE_CURRENT_TASK" },
		func(d *OperationDiagnostic) { d.Retry = true },
		func(d *OperationDiagnostic) { d.Recovery = &OperationRecovery{} },
	} {
		bad := d
		mutation(&bad)
		if _, e := MarshalOperationDiagnostic(bad); e == nil {
			t.Fatalf("open v2 contract: %+v", bad)
		}
	}
	for _, bad := range [][]byte{
		bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":null`), 1),
		bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":false,"retry":false`), 1),
		bytes.Replace(raw, []byte(`"retry":false`), []byte(`"retry":false,"path":"secret"`), 1),
		append(append([]byte(nil), raw...), []byte(`{}`)...),
	} {
		if _, e := DecodeOperationDiagnostic(bad); e == nil {
			t.Fatal("invalid wire accepted")
		}
	}
	for _, op := range []string{"task", "read", "submit", "result"} {
		d.Operation = op
		raw, e = MarshalOperationDiagnostic(d)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = DecodeOperationDiagnostic(raw); e != nil {
			t.Fatal(e)
		}
	}
}
