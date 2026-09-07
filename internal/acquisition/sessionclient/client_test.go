package sessionclient_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisition/sessionclient"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// Existing clients remain assignable without a production import cycle. The
// managed adapter below additionally preserves structural wire shape and supply.
var _ acquisition.Client = (*incomingops.SessionClient)(nil)

type fakeRuntime struct {
	calls     []sessionruntime.RoundTripRequest
	documents []sessionruntime.DocumentRequest
	failure   session.Failure
}

func (f *fakeRuntime) RoundTrip(_ context.Context, r sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	f.calls = append(f.calls, r)
	if f.failure != "" {
		return sessionruntime.RoundTripResult{Failure: f.failure}
	}
	if r.Method == "textDocument/prepareCallHierarchy" {
		i := lsp.CallHierarchyItem{Name: "a", Kind: 12, URI: "file:///a.go", Range: lsp.Range{End: lsp.Position{Character: 10}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}, Data: json.RawMessage(`{"value":7}`)}
		raw, _ := json.Marshal([]lsp.CallHierarchyItem{i})
		return sessionruntime.RoundTripResult{Result: raw}
	}
	return sessionruntime.RoundTripResult{Result: json.RawMessage("null")}
}
func (f *fakeRuntime) PrepareDocument(_ context.Context, r sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	f.documents = append(f.documents, r)
	return sessionruntime.DocumentResult{URI: r.URI, LanguageID: r.LanguageID, Supply: &sessionruntime.DocumentSupply{URI: r.URI, SessionID: r.SessionID, Generation: r.Generation, DocumentVersion: 1, Classification: "SUPPLIED", Method: "textDocument/didOpen", Params: json.RawMessage(`{}`)}}
}
func req() acquisition.Request {
	zero := uint32(0)
	return acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "ctx", SessionID: "session", Generation: 7, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: "file:///a.go", Line: &zero, Character: &zero, LanguageID: "go"}, DownDepth: 1, UpDepth: 1}, Limits: acquisition.Limits{MaxNodes: 10, MaxRequests: 10, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 7}}
}
func TestManagedAdapterGenerationLimitsSupply(t *testing.T) {
	f := &fakeRuntime{}
	r := req()
	got, e := acquisition.Acquire(context.Background(), sessionclient.New(f), r)
	if e != nil {
		t.Fatal(e)
	}
	if len(f.calls) != 3 || len(f.documents) != 1 || len(got.Supplies) != 1 || got.Usage.Requests != 4 {
		t.Fatal("real adapter retains supply and wire attempt accounting")
	}
	for _, call := range f.calls {
		if call.Generation != 7 || call.SessionID != "session" || call.MaxMessages != 7 || call.MaxBytes > 1<<20 || call.Deadline.IsZero() {
			t.Fatal("exact managed context and limits")
		}
	}
	if !f.documents[0].CaptureSupply || f.documents[0].LanguageID != "go" {
		t.Fatal("optional supply receives declared language/context")
	}
}
func TestManagedRestartFailureAccounted(t *testing.T) {
	f := &fakeRuntime{failure: session.Failure("SESSION_GENERATION_MISMATCH")}
	got, e := acquisition.Acquire(context.Background(), sessionclient.New(f), req())
	if e != nil {
		t.Fatal(e)
	}
	if got.Targets[0].Resolution.Status != acquisition.ResolutionFailed || got.Requests[1].Reason != "SESSION_GENERATION_MISMATCH" {
		t.Fatal("runtime validates generations; coordinator retains exact failure")
	}
}
