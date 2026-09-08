package graphprovenance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
)

func TestV2SourceOutcomesAndBudget(t *testing.T) {
	for _, status := range []string{"MISSING", "NONREGULAR", "OUTSIDE_SCOPE", "BYTE_LIMIT_EXCEEDED", "CANCELLED"} {
		t.Run(status, func(t *testing.T) {
			r, root := coordinatorV2Fixture(t, nil)
			ctx := context.Background()
			path := filepath.Join(root, "a.go")
			switch status {
			case "MISSING":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "NONREGULAR":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "OUTSIDE_SCOPE":
				root = t.TempDir()
			case "BYTE_LIMIT_EXCEEDED":
				if err := os.WriteFile(path, make([]byte, MaxFileBytes+1), 0600); err != nil {
					t.Fatal(err)
				}
			case "CANCELLED":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			raw, err := CaptureV2(ctx, r, root)
			if err != nil {
				t.Fatal(err)
			}
			var e EvidenceV2
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			if len(e.Captures) != 1 || e.Captures[0].Status != status || e.Captures[0].Content != nil {
				t.Fatalf("ASSERT_V2_SOURCE_%s: %+v", status, e.Captures)
			}
			if _, err := ValidateFor(raw, Family, "v2"); err != nil {
				t.Fatal(err)
			}
			t.Logf("ASSERT_V2_SOURCE_%s: PASS", status)
		})
	}
	r, root := coordinatorV2Fixture(t, func(r *acquisition.Request) {
		r.RequiredTargets = nil
		for i := 0; i < 6; i++ {
			target := r.Root
			target.ID = fmt.Sprintf("missing-%d", i)
			target.Locator.URI = r.Root.Locator.URI + fmt.Sprintf("-missing-%d", i)
			r.RequiredTargets = append(r.RequiredTargets, target)
		}
	})
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e.CaptureBudget.ChargedBytes != MaxTotalBytes {
		t.Fatal("ASSERT_V2_CAPTURE_BOUNDARY_CHARGE")
	}
	found := false
	for _, receipt := range e.Captures {
		if receipt.Status == "BUDGET_EXCEEDED" {
			found = true
		}
	}
	if !found {
		t.Fatal("ASSERT_V2_CAPTURE_BUDGET_CLASSIFICATION")
	}
	e.CaptureBudget.ChargedBytes = 0
	bad, _ := json.Marshal(e)
	if _, err := ValidateFor(bad, Family, "v2"); err == nil {
		t.Fatal("ASSERT_V2_CAPTURE_BUDGET_TAMPER")
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	delete(doc, "capture_budget")
	bad, _ = json.Marshal(doc)
	if _, err := ValidateFor(bad, Family, "v2"); err == nil {
		t.Fatal("ASSERT_V2_CAPTURE_BUDGET_REQUIRED")
	}
	t.Log("ASSERT_V2_CAPTURE_BOUNDARY_CHARGE: PASS; ASSERT_V2_CAPTURE_BUDGET_CLASSIFICATION: PASS; ASSERT_V2_CAPTURE_BUDGET_TAMPER: PASS; ASSERT_V2_CAPTURE_BUDGET_REQUIRED: PASS")
}

func TestV2GraphMismatchAndReplayLocator(t *testing.T) {
	r, root := coordinatorV2Fixture(t, nil)
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range e.Bindings {
		if b.Attribution == "SOURCE" && b.URI == r.Request.Root.Locator.URI && strings.HasPrefix(b.Pointer, "/graph/replay_input_manifest/") {
			found = true
		}
	}
	if !found {
		t.Fatal("ASSERT_V2_REPLAY_SOURCE_LOCATOR")
	}
	other, _ := coordinatorV2Fixture(t, nil)
	e.Acquisition.Graph = other.Graph
	if _, err := json.Marshal(e.Acquisition.Graph); err != nil {
		t.Fatal("independently valid native graph fixture:", err)
	}
	if err := ValidateV2(e); err == nil {
		t.Fatal("ASSERT_V2_TYPED_GRAPH_MISMATCH")
	}
	if _, err := json.Marshal(e); err == nil {
		t.Fatal("ASSERT_V2_MARSHAL_MUST_NOT_REPLACE_TYPED_GRAPH")
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	var descriptor map[string]json.RawMessage
	_ = json.Unmarshal(doc["acquisition"], &descriptor)
	descriptor["graph"] = json.RawMessage(`{}`)
	doc["acquisition"], _ = json.Marshal(descriptor)
	bad, _ := json.Marshal(doc)
	if _, err := ValidateFor(bad, Family, "v2"); err == nil {
		t.Fatal("ASSERT_V2_SERIALIZED_GRAPH_MISMATCH")
	}
	t.Log("ASSERT_V2_REPLAY_SOURCE_LOCATOR: PASS; ASSERT_V2_TYPED_GRAPH_MISMATCH: PASS; ASSERT_V2_MARSHAL_MUST_NOT_REPLACE_TYPED_GRAPH: PASS; ASSERT_V2_SERIALIZED_GRAPH_MISMATCH: PASS")
}
