package graphprovenance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/schema"
)

func TestV2AdmissionTamperGuards(t *testing.T) {
	r, root := coordinatorV2Fixture(t, nil)
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*EvidenceV2){
		"missing_requested_row": func(e *EvidenceV2) { e.Acquisition.Targets = e.Acquisition.Targets[:1] },
		"request_owner":         func(e *EvidenceV2) { e.Acquisition.Requests[0].TargetID = "alias" },
		"request_response":      func(e *EvidenceV2) { e.Acquisition.Requests[0].Response = json.RawMessage(`[]`) },
		"census_deletion":       func(e *EvidenceV2) { e.Bindings = e.Bindings[1:] },
		"source_hash":           func(e *EvidenceV2) { e.Captures[0].ID = "sha256:" + strings.Repeat("0", 64) },
		"source_content":        func(e *EvidenceV2) { e.Captures[0].Content = []byte("changed") },
		"capture_omission":      func(e *EvidenceV2) { e.Captures = []Receipt{} },
		"graph_digest":          func(e *EvidenceV2) { e.GraphDigest = "sha256:" + strings.Repeat("0", 64) },
		"usage":                 func(e *EvidenceV2) { e.Acquisition.Usage.Requests++ },
		"connection_work":       func(e *EvidenceV2) { e.Acquisition.Targets[1].Connection.Work = 0 },
		"generation":            func(e *EvidenceV2) { e.Acquisition.Request.Context.Generation++ },
		"ceiling":               func(e *EvidenceV2) { e.AnalyzedVersion = "VERIFIED" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var e EvidenceV2
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			if err := ValidateV2(e); err != nil {
				t.Fatal("positive control:", err)
			}
			mutate(&e)
			wrong, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateFor(wrong, Family, "v2"); err == nil {
				t.Fatalf("ASSERT_V2_REJECT_%s: accepted tamper", name)
			}
			t.Logf("ASSERT_V2_REJECT_%s: PASS", name)
		})
	}
	bad := r
	bad.Targets = bad.Targets[:1]
	if _, err := CaptureV2(context.Background(), bad, root); err == nil {
		t.Fatal("ASSERT_V2_CAPTURE_REJECT_MISSING_ROW")
	}
	for _, version := range []string{"v1", Version, "", "v3"} {
		if _, err := ValidateFor(raw, Family, version); err == nil {
			t.Fatalf("ASSERT_V2_EXACT_VERSION: accepted %q", version)
		}
	}
	if _, err := schema.ValidateFor(raw, Family, "v2"); err == nil {
		t.Fatal("ASSERT_V2_CORE_REQUIRES_COMPOSITION")
	}
	if _, err := schema.BytesFor(Family, "v2"); err != nil {
		t.Fatal(err)
	}
}

func TestV2IndependentRoundtripAndIdentity(t *testing.T) {
	r, root := coordinatorV2Fixture(t, nil)
	native, _ := json.Marshal(r.Graph)
	original, _ := json.Marshal(r)
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	restored, _ := json.Marshal(e.Acquisition)
	if !bytes.Equal(original, restored) {
		t.Fatalf("ASSERT_V2_RESULT_ROUNDTRIP:\n%s\n%s", original, restored)
	}
	if !bytes.Equal(native, e.GraphBytes) {
		t.Fatal("ASSERT_V2_NATIVE_BYTES")
	}
	sum := sha256.Sum256(append([]byte("lsp-trace.graph-provenance.v2:graph\x00"), native...))
	if e.GraphDigest != fmt.Sprintf("sha256:%x", sum) {
		t.Fatal("ASSERT_V2_GRAPH_PREIMAGE")
	}
	receipt := e.Captures[0]
	id := receipt.ID
	receipt.ID = ""
	b, _ := json.Marshal(receipt)
	sum = sha256.Sum256(append([]byte("lsp-trace.graph-provenance.v2:receipt\x00"), b...))
	if id != fmt.Sprintf("sha256:%x", sum) {
		t.Fatal("ASSERT_V2_RECEIPT_PREIMAGE")
	}
	if !bytes.Contains(restored, []byte("9007199254740993")) {
		t.Fatal("ASSERT_V2_NUMBER_PRECISION")
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFor(pretty.Bytes(), Family, VersionV2); err != nil {
		t.Fatal("ASSERT_V2_FORMAT_STABILITY:", err)
	}
	t.Log("ASSERT_V2_RESULT_ROUNDTRIP: PASS; ASSERT_V2_GRAPH_PREIMAGE: PASS; ASSERT_V2_RECEIPT_PREIMAGE: PASS; ASSERT_V2_NUMBER_PRECISION: PASS; ASSERT_V2_FORMAT_STABILITY: PASS")
}

func TestV2EmptyRootPartialAndExplicitLimits(t *testing.T) {
	cases := map[string]func(*acquisition.Request){
		"root_ambiguous": func(r *acquisition.Request) {
			r.Root.Locator.Line = nil
			r.Root.Locator.Character = nil
			r.Root.Locator.Symbol = "Ambiguous"
		},
		"virtual_root": func(r *acquisition.Request) { r.Root.Locator.URI = "untitled:///virtual.go" },
		"root_empty_required_survives": func(r *acquisition.Request) {
			r.Root.Locator.Line = nil
			r.Root.Locator.Character = nil
			r.Root.Locator.Symbol = "Absent"
		},
		"entirely_empty": func(r *acquisition.Request) {
			r.Root.Locator.Line = nil
			r.Root.Locator.Character = nil
			r.Root.Locator.Symbol = "Absent"
			r.RequiredTargets = nil
		},
		"partial":                     func(r *acquisition.Request) { r.Limits.MaxRequests = 1 },
		"zero_budget":                 func(r *acquisition.Request) { r.Limits.MaxNodes = 0; r.Limits.MaxEvidenceBytes = 0 },
		"above_future_retained_limit": func(r *acquisition.Request) { r.Limits.MaxNodes = 4097 },
		"incoming":                    func(r *acquisition.Request) { r.Mode = acquisition.Incoming },
	}
	for name, configure := range cases {
		t.Run(name, func(t *testing.T) {
			r, root := coordinatorV2Fixture(t, configure)
			raw, err := CaptureV2(context.Background(), r, root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateFor(raw, Family, "v2"); err != nil {
				t.Fatal(err)
			}
			var e EvidenceV2
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			if len(e.Acquisition.Targets) != 1+len(r.Request.RequiredTargets) {
				t.Fatal("target denominator")
			}
			if e.Acquisition.AcquisitionComplete != r.AcquisitionComplete {
				t.Fatal("partiality")
			}
			if name == "root_ambiguous" && e.Acquisition.Targets[0].Resolution.Status != acquisition.Ambiguous {
				t.Fatal("ambiguous root lost")
			}
			if name == "virtual_root" {
				found := false
				for _, capture := range e.Captures {
					if capture.URI == "untitled:///virtual.go" && capture.Status == "VIRTUAL_URI" {
						found = true
					}
				}
				if !found {
					t.Fatal("virtual source classification lost")
				}
			}
			if name == "root_empty_required_survives" {
				found := false
				for _, b := range e.Bindings {
					if b.Pointer == "/acquisition/request/root/locator/symbol" && b.URI == r.Request.Root.Locator.URI {
						found = true
					}
				}
				if !found {
					t.Fatal("missing unsuccessful requested symbol source reference")
				}
			}
			t.Logf("ASSERT_V2_BOUNDARY_%s: PASS", name)
		})
	}
}

func TestV2KnownCarrierPreflight(t *testing.T) {
	r, root := coordinatorV2Fixture(t, nil)
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	e.Acquisition.Requests[0].Response = json.RawMessage(strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65))
	bad, _ := json.Marshal(e)
	bad = append([]byte(`{"schema_version":"duplicate",`), bad[1:]...)
	if _, err := ValidateFor(bad, Family, "v2"); err == nil || !strings.Contains(err.Error(), "depth limit") {
		t.Fatalf("ASSERT_V2_DEPTH_BEFORE_DUPLICATE: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc, "acquisition")
	bad, _ = json.Marshal(doc)
	if _, err := ValidateFor(bad, Family, "v2"); err == nil {
		t.Fatal("ASSERT_V2_ACCOUNTING_REQUIRED")
	}
	// The field called data is opaque JSON, even when it resembles URI metadata.
	if err := preflightV2([]byte(`{"data":`+strings.Repeat("[", 100)+`"file:///not-a-source"`+strings.Repeat("]", 100)+`}`), MaxEnvelopeBytesV2); err != nil {
		t.Fatal("ASSERT_V2_OPAQUE_DATA:", err)
	}
	t.Log("ASSERT_V2_DEPTH_BEFORE_DUPLICATE: PASS; ASSERT_V2_ACCOUNTING_REQUIRED: PASS; ASSERT_V2_OPAQUE_DATA: PASS")
}
