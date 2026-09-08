package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/manageddiagnostic"
)

const (
	assertPrivateDiagnosticValid  = "ASSERT_FR23_PRIVATE_REQUEST_DIAGNOSTIC_VALID"
	assertPrivateDiagnosticClosed = "ASSERT_FR23_PRIVATE_REQUEST_DIAGNOSTIC_CLOSED"
	assertPrivateTimeoutAuthority = "ASSERT_FR23_PRIVATE_TIMEOUT_AUTHORITY"
	assertPrivatePublishSafety    = "ASSERT_FR23_PRIVATE_PUBLISH_SAFETY"
)

func TestFR23PrivateRequestDiagnosticValidationCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request-diagnostic.json")
	raw := []byte(`{"schema_version":"lsp-trace.private-request-diagnostic.v1","manager":{"attempt_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","session_id":"session-1","generation":1},"selected_operation":{"method":"textDocument/prepareCallHierarchy","dispatch_attempted":true,"write":{"state":"complete","count":1,"bytes":128},"read":{"state":"failed","count":0,"bytes":0},"classification":"FAILED_AT_LIMIT_TIMING_CONSISTENT","deadline_observations":[{"source":"configured_limit","milliseconds":30000},{"source":"observed_terminal","kind":"unknown"}],"elapsed":{"milliseconds":30372,"bounded":true},"process":{"state":"exited","exit_code":0},"sequence":2},"retention":{"retained_bytes":17710,"fallback":false,"omitted_records":0,"truncated":false,"max_records":64,"max_bytes":65536},"artifact":{"length":1,"sha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"validate-private-request-diagnostics", path}); code != 0 {
		t.Fatal(assertPrivateDiagnosticValid)
	}
}

func TestFR23PrivateRequestDiagnosticMutationsAndPublication(t *testing.T) {
	record := manageddiagnostic.Record{
		SessionID: "session-1", Generation: 1, Sequence: 2,
		Phase: manageddiagnostic.PhaseRequestDispatch, Terminal: manageddiagnostic.TerminalUnknown,
		Reason:  manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: "request-failed"},
		Timing:  manageddiagnostic.Timing{Scope: "round-trip", Start: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: 0}, End: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: 30372000000}, Elapsed: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: 30372000000}},
		Limits:  manageddiagnostic.Limits{EffectiveDeadlineNS: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: 30000000000}},
		Request: manageddiagnostic.RequestFacts{Method: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: "textDocument/prepareCallHierarchy"}, OwnerSequence: manageddiagnostic.Fact[uint64]{Status: manageddiagnostic.Observed, Value: 2}, ProtocolID: manageddiagnostic.Fact[uint64]{Status: manageddiagnostic.Observed, Value: 1}},
		Write:   manageddiagnostic.IOFacts{State: manageddiagnostic.IOComplete, Messages: 1, Bytes: 128}, Read: manageddiagnostic.IOFacts{State: manageddiagnostic.IOFailed},
		CallHierarchy: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Unavailable}, DocumentSupplyCompleted: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Unavailable}, ProcessExit: manageddiagnostic.ProcessExit{Status: manageddiagnostic.Unavailable}, Stderr: manageddiagnostic.Stderr{Status: manageddiagnostic.Withheld}, NumericRPCCode: manageddiagnostic.Fact[int]{Status: manageddiagnostic.Unavailable},
	}
	raw, err := projectPrivateRequestDiagnostic(strings.Repeat("a", 64), "session-1", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{record}}, []byte("public-v3"), 64, 65536)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	operation := object["selected_operation"].(map[string]any)
	if operation["classification"] != "FAILED_AT_LIMIT_TIMING_CONSISTENT" {
		t.Fatal(assertPrivateTimeoutAuthority)
	}
	mutate := func(name string, change func(map[string]any)) {
		t.Helper()
		var candidate map[string]any
		_ = json.Unmarshal(raw, &candidate)
		change(candidate)
		altered, _ := json.Marshal(candidate)
		if _, err := decodePrivateRequestDiagnostic(altered); err == nil {
			t.Fatalf("%s/%s", assertPrivateDiagnosticClosed, name)
		}
	}
	mutate("unknown-field", func(d map[string]any) { d["stderr"] = "SECRET" })
	mutate("known-field-method-smuggling", func(d map[string]any) { d["selected_operation"].(map[string]any)["method"] = "/private/path" })
	mutate("known-field-identity-smuggling", func(d map[string]any) { d["manager"].(map[string]any)["session_id"] = "/private/path" })
	mutate("inferred-timeout", func(d map[string]any) {
		d["selected_operation"].(map[string]any)["classification"] = "TIMEOUT_OBSERVED"
	})
	root := t.TempDir()
	path := filepath.Join(root, "private.json")
	if err := publishPrivateRequestDiagnostic(root, "private.json", raw); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("%s/mode: %o", assertPrivatePublishSafety, info.Mode().Perm())
	}
	if err := publishPrivateRequestDiagnostic(root, "private.json", raw); err == nil {
		t.Fatalf("%s/no-replace", assertPrivatePublishSafety)
	}
	for _, selector := range []string{"../escape.json", "/absolute.json", "a/../../escape.json"} {
		if err := publishPrivateRequestDiagnostic(root, selector, raw); err == nil {
			t.Fatalf("%s/root-escape/%q", assertPrivatePublishSafety, selector)
		}
	}
	stored, _ := os.ReadFile(path)
	if !bytes.Equal(stored, raw) {
		t.Fatalf("%s/immutable", assertPrivatePublishSafety)
	}
}
