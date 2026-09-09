package acquisitionops

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type seedOrderRuntime struct {
	admission seedbinding.Status
	calls     []string
	starts    int
	workspace string
}

func (r *seedOrderRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return sessionruntime.SessionMetadata{PositionEncoding: "utf-8", CallHierarchySupport: true}, ""
}
func (r *seedOrderRuntime) Records() []sessionruntime.Record {
	v, _ := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: r.workspace, Profile: "fake", EnvironmentReference: "test"})
	return []sessionruntime.Record{{SessionID: "s", Generation: 1, Profile: runtimeprofile.Resolve(v)}}
}
func (r *seedOrderRuntime) SeedBindingRequested(string, uint64) bool { return true }
func (r *seedOrderRuntime) AdmitSeedBinding(context.Context, string, uint64, time.Time, int, int64) seedbinding.ValidationResult {
	r.calls = append(r.calls, "textDocument/documentSymbol")
	return seedbinding.ValidationResult{Status: r.admission}
}
func (r *seedOrderRuntime) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.calls = append(r.calls, req.Method)
	item := `[{"name":"GetSchoolFilterModel","kind":12,"uri":"file:///workspace/VariableApiController.cs","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`
	return sessionruntime.RoundTripResult{Result: json.RawMessage(item)}
}
func (r *seedOrderRuntime) PrepareDocument(_ context.Context, req sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	return sessionruntime.DocumentResult{URI: req.URI, LanguageID: req.LanguageID, Version: 1}
}

func TestSemanticSeedAdmissionPrecedesPrepareAndBlocksMismatch(t *testing.T) {
	workspace := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: workspace + "/VariableApiController.cs"}).String()
	manifest := Manifest{SchemaVersion: ManifestVersion, CoordinateConvention: "zero-based-session", Root: Target{ID: "root", Locator: structLocator(uri)}, RequiredTargets: []Target{}, Limits: Limits{MaxNodes: intp(10), MaxRequests: intp(10)}}
	input, _ := json.Marshal(Input{SessionID: "s", Generation: 1, SeedManifest: manifest})
	for _, tc := range []struct {
		name        string
		status      seedbinding.Status
		wantFailure bool
	}{{"match", seedbinding.Match, false}, {"mismatch", seedbinding.Mismatch, true}} {
		t.Run(tc.name, func(t *testing.T) {
			r := &seedOrderRuntime{admission: tc.status, starts: 1, workspace: workspace}
			_, failure := NewExecutor(r).Execute(context.Background(), operation.Request{Name: Slice, Input: input})
			joined := strings.Join(r.calls, ",")
			if tc.wantFailure {
				if failure == nil || failure.Code != seedbinding.BindingMismatch || joined != "textDocument/documentSymbol" {
					t.Fatalf("ASSERT_SEED_MISMATCH_NO_PREPARE: failure=%v calls=%s", failure, joined)
				}
			} else if failure != nil || !strings.HasPrefix(joined, "textDocument/documentSymbol,textDocument/prepareCallHierarchy") {
				t.Fatalf("ASSERT_DOCUMENT_SYMBOL_BEFORE_PREPARE: failure=%v calls=%s", failure, joined)
			}
			if r.starts != 1 {
				t.Fatalf("ASSERT_PROVIDER_STARTED_ONCE: %d", r.starts)
			}
		})
	}
}

func intp(v int) *int { return &v }
func structLocator(uri string) acquisition.Locator {
	zero := uint32(0)
	return acquisition.Locator{URI: uri, Line: &zero, Character: &zero}
}
