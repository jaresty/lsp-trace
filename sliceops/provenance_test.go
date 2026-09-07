package sliceops

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type provenanceRuntime struct {
	*fakeRuntime
	root     string
	captured bool
}

func (f *provenanceRuntime) Records() []sessionruntime.Record {
	v, _ := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: f.root, Profile: "go", EnvironmentReference: "local"})
	return []sessionruntime.Record{{SessionID: "s", Generation: 1, Profile: runtimeprofile.Resolve(v)}}
}
func (f *provenanceRuntime) PrepareDocument(_ context.Context, r sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	f.captured = r.CaptureSupply
	return sessionruntime.DocumentResult{URI: r.URI, LanguageID: "go", Version: 1}
}
func TestGraphProvenanceInputIsExactAndCompositionRejected(t *testing.T) {
	base := bytes.TrimSuffix(validInput(), []byte("}"))
	for _, suffix := range []string{`,"Graph_provenance":true}`, `,"graph_provenance":true,"graph_provenance":true}`, `,"graph_provenance":null}`, `,"graph_provenance":true,"relations":["CALLS"]}`, `,"graph_provenance":true,"adapters":"auto"}`} {
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}}
		_, failed := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: append(append([]byte(nil), base...), []byte(suffix)...)})
		if failed == nil || len(f.calls) != 0 {
			t.Fatalf("ASSERT_EXACT_PROVENANCE_INPUT_NO_TRAVERSAL: %s", suffix)
		}
	}
}

func TestManagedGraphProvenanceConsumer(t *testing.T) {
	f := &provenanceRuntime{root: t.TempDir(), fakeRuntime: &fakeRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, results: map[string]sessionruntime.RoundTripResult{
		"textDocument/prepareCallHierarchy:": {Result: json.RawMessage(`[]`)},
	}}}
	var input map[string]any
	_ = json.Unmarshal(validInput(), &input)
	input["graph_provenance"] = true
	raw, _ := json.Marshal(input)
	result, failure := NewExecutor(f).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: raw})
	if failure != nil {
		t.Fatalf("ASSERT_GRAPH_PROVENANCE_CONSUMER: %v", failure)
	}
	var envelope map[string]any
	if json.Unmarshal(result.Artifact, &envelope) != nil || envelope["schema_version"] != "lsp-trace.graph-provenance.v1" || !f.captured {
		t.Fatalf("ASSERT_GRAPH_PROVENANCE_CONSUMER: capture=%v artifact=%s", f.captured, result.Artifact)
	}
}
