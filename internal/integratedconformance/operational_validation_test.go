package integratedconformance

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/custodyevidence"
	executionruntime "lsp-trace/internal/execution"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/verification"
)

func cloneOperationalEvidence(t *testing.T, e *custodyevidence.Evidence) custodyevidence.Evidence {
	t.Helper()
	b, _ := json.Marshal(e)
	var copy custodyevidence.Evidence
	if err := json.Unmarshal(b, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func TestOperationalSemanticValidationParity(t *testing.T) {
	h := newOperationalHarness(t)
	request := operationalRequest(t)
	a := operationalSuccess(t, h.run("direct", request, "", nil))
	base := a.Operational
	checks := map[string]func(*custodyevidence.Evidence){
		"valid":           func(*custodyevidence.Evidence) {},
		"source-id":       func(e *custodyevidence.Evidence) { e.Identity.SourceID = e.Identity.SnapshotID },
		"snapshot-id":     func(e *custodyevidence.Evidence) { e.Identity.SnapshotID = e.Identity.SourceID },
		"collection-id":   func(e *custodyevidence.Evidence) { e.Identity.CollectionID = e.Identity.SourceID },
		"identity-policy": func(e *custodyevidence.Evidence) { e.Identity.Policy = source.IdentityPolicyV1 },
		"manifest":        func(e *custodyevidence.Evidence) { e.Identity.Manifest = append(e.Identity.Manifest, ' ') },
		"content":         func(e *custodyevidence.Evidence) { e.InputEvidence.Inputs[0].Content = []byte("forged bytes") },
		"canonical-receipt": func(e *custodyevidence.Evidence) {
			e.InputEvidence.Inputs[0].CanonicalReceipt = append(e.InputEvidence.Inputs[0].CanonicalReceipt, ' ')
		},
		"receipt-id":     func(e *custodyevidence.Evidence) { e.InputEvidence.Inputs[0].ID = e.Identity.SourceID },
		"missing-input":  func(e *custodyevidence.Evidence) { e.InputEvidence.Inputs = e.InputEvidence.Inputs[1:] },
		"missing-output": func(e *custodyevidence.Evidence) { e.Outputs = e.Outputs[1:] },
		"output-content": func(e *custodyevidence.Evidence) { e.Outputs[0].Content = []byte("wrong consumed bytes") },
		"output-path":    func(e *custodyevidence.Evidence) { e.Outputs[0].Path = "another" },
		"contribution": func(e *custodyevidence.Evidence) {
			e.InputEvidence.Contributions[0].ReceiptIDs = []string{e.Identity.SourceID}
		},
		"incompleteness":        func(e *custodyevidence.Evidence) { e.InputEvidence.IncompleteReasons = nil },
		"admission-policy":      func(e *custodyevidence.Evidence) { e.Admission.Policy = source.IdentityPolicyV1 },
		"admission-snapshot":    func(e *custodyevidence.Evidence) { e.Admission.SnapshotID = e.Identity.SourceID },
		"forged-authentication": func(e *custodyevidence.Evidence) { e.Admission.Status = schema.AuthenticationAuthenticated },
		"permission":            func(e *custodyevidence.Evidence) { e.RequireAuthenticated = true },
		"acquisition-status":    func(e *custodyevidence.Evidence) { e.AcquisitionStatus = "PARTIAL" },
	}
	for name, mutate := range checks {
		t.Run(name, func(t *testing.T) {
			e := cloneOperationalEvidence(t, base)
			mutate(&e)
			raw, _ := json.Marshal(e)
			want := name == "valid"
			if _, err := custodyevidence.ValidateFor(raw, schema.FamilyOperationalCustody, "v1"); (err == nil) != want {
				t.Fatalf("ASSERT_OPERATIONAL_SEMANTIC_%s: direct err=%v", name, err)
			}
			params, _ := json.Marshal(map[string]any{"input": json.RawMessage(raw), "schema": map[string]any{"family": schema.FamilyOperationalCustody, "version": "v1"}})
			if _, failure := operation.ValidateHandler(context.Background(), operation.Request{Name: operation.Validate, Input: params}); (failure == nil) != want {
				t.Fatalf("ASSERT_OPERATIONAL_SEMANTIC_%s: shared operation disagrees", name)
			}
			cmd := exec.Command(h.cli, "validate", "--family", schema.FamilyOperationalCustody, "--version", "v1", "-")
			cmd.Stdin = bytes.NewReader(raw)
			if out, err := cmd.CombinedOutput(); (err == nil) != want {
				t.Fatalf("ASSERT_OPERATIONAL_SEMANTIC_%s: CLI err=%v %s", name, err, out)
			}
			line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "lsp_trace_v1_validate", "arguments": json.RawMessage(params)}})
			cmd = exec.Command(h.mcp)
			cmd.Stdin = bytes.NewReader(append(line, '\n'))
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Result struct {
					IsError bool `json:"isError"`
				} `json:"result"`
				Error any `json:"error"`
			}
			if err := json.Unmarshal(out, &envelope); err != nil {
				t.Fatal(err)
			}
			if (!envelope.Result.IsError && envelope.Error == nil) != want {
				t.Fatalf("ASSERT_OPERATIONAL_SEMANTIC_%s: MCP disagrees %s", name, out)
			}
		})
	}
	raw, _ := json.Marshal(base)
	if _, err := schema.ValidateFor(raw, schema.FamilyOperationalCustody, "v1"); err == nil {
		t.Fatal("ASSERT_OPERATIONAL_SCHEMA_FAIL_CLOSED: low-level shape validation silently claimed semantic admission")
	}
	// Every top-level field is structurally required, including retained bytes.
	var object map[string]json.RawMessage
	_ = json.Unmarshal(raw, &object)
	for key := range object {
		t.Run("omit-"+key, func(t *testing.T) {
			var copy map[string]json.RawMessage
			_ = json.Unmarshal(raw, &copy)
			delete(copy, key)
			missing, _ := json.Marshal(copy)
			if _, err := custodyevidence.ValidateFor(missing, schema.FamilyOperationalCustody, "v1"); err == nil {
				t.Fatal("ASSERT_OPERATIONAL_OMISSION: missing required field accepted")
			}
		})
	}
}

func TestOperationalPublicationIntegrity(t *testing.T) {
	h := newOperationalHarness(t)
	request := operationalRequest(t)
	a := operationalSuccess(t, h.run("direct", request, "", nil))
	raw, err := os.ReadFile(a.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := json.Marshal(a.Operational)
	if !bytes.Equal(raw, expected) {
		t.Fatal("ASSERT_OPERATIONAL_PUBLICATION_BYTES: output and published evidence differ")
	}
	// The production exact-byte verifier is applicable to the flat pair; the
	// historical CLI custody selector and graph semantic verifier are not.
	receipt, err := os.ReadFile(a.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := verification.VerifyReceipt(raw, receipt); err != nil {
		t.Fatalf("ASSERT_OPERATIONAL_CUSTODY: original failed %v", err)
	}
	if err := os.WriteFile(a.Artifact, append(raw, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := os.ReadFile(a.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := verification.VerifyReceipt(changed, receipt); err == nil {
		t.Fatal("ASSERT_OPERATIONAL_CUSTODY_MUTATION: changed publication accepted")
	}
	// A publication failure retains the full operational artifact in error
	// diagnostics; it does not pretend receipt completion succeeded.
	request = operationalRequest(t)
	if err := os.Mkdir(filepath.Join(request["root"].(string), "artifact.json"), 0700); err != nil {
		t.Fatal(err)
	}
	got := h.run("direct", request, "", nil)
	if !got.Failed {
		t.Fatal("ASSERT_OPERATIONAL_PUBLICATION_FAILURE: occupied destination accepted")
	}
	if e := operationalFailureEvidence(t, got); len(e.InputEvidence.Inputs) != 4 {
		t.Fatal("ASSERT_OPERATIONAL_PUBLICATION_FAILURE: custody lost on publication failure")
	}
}

type interruptedOperationalContext struct {
	context.Context
	calls int
}

func (c *interruptedOperationalContext) Err() error {
	c.calls++
	if c.calls >= 4 {
		return context.Canceled
	}
	return nil
}

func TestOperationalInterruptedReadRetention(t *testing.T) {
	request := operationalRequest(t)
	raw, _ := json.Marshal(request)
	ctx := &interruptedOperationalContext{Context: context.Background()}
	_, failure := executionruntime.NewProductionExecutor().Execute(ctx, operation.Request{Name: operation.CustodyExecute, RequestID: "offline-1", Input: raw})
	if failure == nil {
		t.Fatal("ASSERT_OPERATIONAL_INTERRUPTED_READ: cancellation not propagated")
	}
	diagnostics, _ := json.Marshal(failure.Diagnostics)
	e := operationalFailureEvidence(t, operationalObservation{Failed: true, Output: string(diagnostics)})
	if len(e.InputEvidence.Inputs) != 1 || len(e.Outputs) != 1 || !strings.Contains(string(e.Outputs[0].Content), "actual bytes") {
		t.Fatal("ASSERT_OPERATIONAL_INTERRUPTED_READ: already acquired receipt/content lost before snapshot")
	}
	assertOperationalUnpublished(t, request)
}
