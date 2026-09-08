package graphprovenance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"lsp-trace/internal/manageddiagnostic"
)

func TestV3EmbedsExactAdmittedV2AndUnavailableDiagnostics(t *testing.T) {
	result, root := coordinatorV2Fixture(t, nil)
	v2, err := CaptureV2(context.Background(), result, root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CaptureV3(v2, result.Request.Context.SessionID, result.Request.Context.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	var got EvidenceV3
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	embedded, err := base64.StdEncoding.DecodeString(got.GraphProvenanceV2)
	if err != nil || !bytes.Equal(embedded, v2) {
		t.Fatal("ASSERT_FR23_V3_EXACT_V2_BYTES")
	}
	sum := sha256.Sum256(v2)
	if got.GraphProvenanceV2SHA256 != fmt.Sprintf("sha256:%x", sum) {
		t.Fatal("ASSERT_FR23_V3_V2_DIGEST")
	}
	if got.SessionID != result.Request.Context.SessionID || got.Generation != result.Request.Context.Generation {
		t.Fatal("ASSERT_FR23_V3_EXACT_GENERATION_BINDING")
	}
	if got.Diagnostics.Status != manageddiagnostic.QueryUnavailable || got.Diagnostics.Records == nil || len(got.Diagnostics.Records) != 0 {
		t.Fatalf("ASSERT_FR23_V3_UNAVAILABLE_ACCOUNTING: %+v", got.Diagnostics)
	}
	if _, err := ValidateFor(raw, Family, "v3"); err != nil {
		t.Fatalf("ASSERT_FR23_V3_OFFLINE_ADMISSION: %v", err)
	}
}

func TestV3OfflineAdmissionRejectsSemanticMutations(t *testing.T) {
	result, root := coordinatorV2Fixture(t, nil)
	v2, err := CaptureV2(context.Background(), result, root)
	if err != nil {
		t.Fatal(err)
	}
	sessionID, generation := result.Request.Context.SessionID, result.Request.Context.Generation
	record := manageddiagnostic.RecordCapabilityCheck(manageddiagnostic.CapabilityObservation{SessionID: sessionID, Generation: generation, Sequence: 1, Supported: true})
	record.Read.State, record.Write.State = manageddiagnostic.IOUnavailable, manageddiagnostic.IOUnavailable
	raw, err := CaptureV3(v2, sessionID, generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{record}})
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(t *testing.T, assertion string, change func(map[string]any)) {
		t.Helper()
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		change(doc)
		bad, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateFor(bad, Family, "v3"); err == nil {
			t.Fatal(assertion)
		}
	}
	mutate(t, "ASSERT_FR23_V3_REPLAY_BINDS_AUTHORITY/session", func(d map[string]any) { d["session_id"] = "wrong" })
	mutate(t, "ASSERT_FR23_V3_REPLAY_BINDS_AUTHORITY/generation", func(d map[string]any) { d["generation"] = float64(generation + 1) })
	mutate(t, "ASSERT_FR23_V3_REPLAY_DIAGNOSTIC_SEMANTICS/status", func(d map[string]any) { d["diagnostics"].(map[string]any)["status"] = "future" })
	mutate(t, "ASSERT_FR23_V3_REPLAY_ACCOUNTING/bytes", func(d map[string]any) { d["diagnostics"].(map[string]any)["retained_bytes"] = float64(0) })
	mutate(t, "ASSERT_FR23_V3_REPLAY_ACCOUNTING/omitted", func(d map[string]any) { d["diagnostics"].(map[string]any)["omitted_records"] = float64(1) })
	mutate(t, "ASSERT_FR23_V3_REPLAY_ORDERING/duplicate", func(d map[string]any) {
		diag := d["diagnostics"].(map[string]any)
		records := diag["records"].([]any)
		diag["records"] = append(records, records[0])
	})
	mutate(t, "ASSERT_FR23_V3_REPLAY_ORDERING/reorder", func(d map[string]any) {
		diag := d["diagnostics"].(map[string]any)
		r := diag["records"].([]any)[0].(map[string]any)
		copy := make(map[string]any, len(r))
		for k, v := range r {
			copy[k] = v
		}
		copy["sequence"] = float64(2)
		diag["records"] = []any{copy, r}
	})
	mutate(t, "ASSERT_FR23_V3_REPLAY_HIDDEN_VALUE", func(d map[string]any) {
		r := d["diagnostics"].(map[string]any)["records"].([]any)[0].(map[string]any)
		r["call_hierarchy"] = map[string]any{"status": "unavailable", "value": true}
	})
	mutate(t, "ASSERT_FR23_V3_STRUCTURAL_BEFORE_SEMANTIC", func(d map[string]any) { delete(d, "diagnostics") })
}

func TestV3EnvelopeUsesInheritedV2Limit(t *testing.T) {
	legacyWrongBoundary := MaxEnvelopeBytes + MaxDiagnosticBytes + 1
	if _, err := validateForV3(bytes.Repeat([]byte{' '}, legacyWrongBoundary)); err == nil || err.Error() == "V3 envelope byte limit" {
		t.Fatalf("ASSERT_FR23_V3_INHERITS_V2_ENVELOPE_LIMIT: %v", err)
	}
	if _, err := validateForV3(bytes.Repeat([]byte{' '}, MaxEnvelopeBytesV2+MaxDiagnosticBytes+1)); err == nil || err.Error() != "V3 envelope byte limit" {
		t.Fatalf("ASSERT_FR23_V3_ENVELOPE_LIMIT_PLUS_ONE: %v", err)
	}
}

func TestV2RejectsDiagnosticField(t *testing.T) {
	result, root := coordinatorV2Fixture(t, nil)
	v2, err := CaptureV2(context.Background(), result, root)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(v2, &object); err != nil {
		t.Fatal(err)
	}
	object["diagnostics"] = map[string]any{}
	altered, _ := json.Marshal(object)
	if _, err := ValidateFor(altered, Family, "v2"); err == nil {
		t.Fatal("ASSERT_FR23_V2_REJECTS_DIAGNOSTICS")
	}
}
