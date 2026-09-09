package operation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/normativeanalytics"
	"lsp-trace/internal/publication"
)

func analyticsGraphBytes(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(struct {
		SchemaVersion string   `json:"schema_version"`
		BuildRevision string   `json:"build_revision"`
		Nodes         []string `json:"nodes"`
		Edges         []any    `json:"edges"`
	}{normativeanalytics.RetainedGraphSchema, "r1", []string{"a", "b"}, []any{}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func analyticsRequest(t *testing.T, name Name, op normativeanalytics.Operation, input any, max int64, root *publication.Root) Request {
	t.Helper()
	b, err := json.Marshal(map[string]any{"input": input, "operation": op, "filter": []string{"CALLS"}, "max_work": max})
	if err != nil {
		t.Fatal(err)
	}
	return Request{Name: name, Input: b, PublicationRoot: root}
}

func TestBoundedAnalyticsV2InputParityAndVerifiedSelector(t *testing.T) {
	raw := analyticsGraphBytes(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "graph.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sum := sha256.Sum256(raw)
	selector := map[string]any{"selector": "graph.json", "artifact_schema_id": "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json", "artifact_digest": "sha256:" + hex.EncodeToString(sum[:]), "artifact_byte_length": len(raw)}
	for _, tc := range []struct {
		name Name
		op   normativeanalytics.Operation
	}{
		{BoundedRetainedAnalysisV2, normativeanalytics.Analysis}, {BoundedRetainedMetricsV2, normativeanalytics.Metrics}, {BoundedRetainedRankingV2, normativeanalytics.Ranking},
	} {
		for _, max := range []int64{1, 10000} {
			inline, fail := BoundedRetainedAnalyticsV2Handler(context.Background(), analyticsRequest(t, tc.name, tc.op, string(raw), max, nil))
			if fail != nil {
				t.Fatal(fail)
			}
			object, fail := BoundedRetainedAnalyticsV2Handler(context.Background(), analyticsRequest(t, tc.name, tc.op, json.RawMessage(raw), max, nil))
			if fail != nil {
				t.Fatal(fail)
			}
			args := map[string]any{"publication_selector": selector, "operation": tc.op, "filter": []string{"CALLS"}, "max_work": max}
			b, _ := json.Marshal(args)
			selected, fail := BoundedRetainedAnalyticsV2Handler(context.Background(), Request{Name: tc.name, Input: b, PublicationRoot: root})
			if fail != nil {
				t.Fatal(fail)
			}
			if string(inline.Artifact) != string(object.Artifact) {
				t.Fatal("ASSERT_ANALYTICS_INLINE_RAW_BYTE_PARITY")
			}
			var local, verified map[string]any
			json.Unmarshal(inline.Artifact, &local)
			json.Unmarshal(selected.Artifact, &verified)
			if local["claim_level"] != normativeanalytics.ClaimUnverifiedLocal || verified["claim_level"] != normativeanalytics.ClaimVerifiedProvenance || verified["provenance"] == nil {
				t.Fatal("ASSERT_ANALYTICS_CLAIM_PROVENANCE")
			}
		}
	}
}

func TestBoundedAnalyticsV2RejectsUnsafeOrUnverifiedSelectors(t *testing.T) {
	raw := analyticsGraphBytes(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "graph.json"), raw, 0600)
	root, _ := publication.OpenRoot(dir)
	defer root.Close()
	sum := sha256.Sum256(raw)
	base := map[string]any{"selector": "graph.json", "artifact_schema_id": "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json", "artifact_digest": "sha256:" + hex.EncodeToString(sum[:]), "artifact_byte_length": len(raw)}
	for _, mutate := range []func(map[string]any){
		func(x map[string]any) { x["selector"] = "../graph.json" }, func(x map[string]any) { x["selector"] = filepath.Join(dir, "graph.json") }, func(x map[string]any) { x["selector"] = "missing.json" }, func(x map[string]any) { x["artifact_schema_id"] = "wrong-family" }, func(x map[string]any) { x["artifact_digest"] = "sha256:" + string(make([]byte, 64)) }, func(x map[string]any) { x["artifact_byte_length"] = 1 },
	} {
		x := map[string]any{}
		for k, v := range base {
			x[k] = v
		}
		mutate(x)
		b, _ := json.Marshal(map[string]any{"publication_selector": x, "operation": "ANALYSIS", "filter": []string{"CALLS"}, "max_work": 100})
		if _, fail := BoundedRetainedAnalyticsV2Handler(context.Background(), Request{Name: BoundedRetainedAnalysisV2, Input: b, PublicationRoot: root}); fail == nil {
			t.Fatal("ASSERT_ANALYTICS_SELECTOR_REJECTED")
		}
	}
	if err := os.Symlink("graph.json", filepath.Join(dir, "link.json")); err == nil {
		x := map[string]any{}
		for k, v := range base {
			x[k] = v
		}
		x["selector"] = "link.json"
		b, _ := json.Marshal(map[string]any{"publication_selector": x, "operation": "ANALYSIS", "filter": []string{"CALLS"}, "max_work": 100})
		if _, fail := BoundedRetainedAnalyticsV2Handler(context.Background(), Request{Name: BoundedRetainedAnalysisV2, Input: b, PublicationRoot: root}); fail == nil {
			t.Fatal("ASSERT_ANALYTICS_SYMLINK_REJECTED")
		}
	}
}
