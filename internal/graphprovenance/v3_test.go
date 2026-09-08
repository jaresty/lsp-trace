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
