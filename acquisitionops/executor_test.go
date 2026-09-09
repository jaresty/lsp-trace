package acquisitionops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
	"net/url"
	"os"
	"path/filepath"
)

type forbiddenRuntime struct{ calls int }

func (r *forbiddenRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	r.calls++
	return sessionruntime.SessionMetadata{}, session.SessionNotFound
}
func (r *forbiddenRuntime) RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.calls++
	return sessionruntime.RoundTripResult{}
}
func (r *forbiddenRuntime) Records() []sessionruntime.Record { r.calls++; return nil }

const validManifest = `{"schema_version":"lsp-trace.seed-manifest.v2","coordinate_convention":"zero-based-session","root":{"id":"root","locator":{"uri":"file:///fixture/a.go","line":0,"character":0}},"required_targets":[]}`

func TestManifestClosedPreflight(t *testing.T) {
	for _, change := range []struct{ name, old, new string }{
		{"empty-symbol-with-position", `"character":0`, `"character":0,"symbol":""`},
		{"empty-language", `"character":0`, `"character":0,"language_id":""`},
		{"unknown-selector", `"character":0`, `"character":0,"name":"a"`},
		{"partial-position", `,"character":0`, ``},
		{"null-coordinate", `"character":0`, `"character":null`},
		{"duplicate-coordinate", `"character":0`, `"character":0,"character":1`},
		{"negative", `"character":0`, `"character":-1`},
		{"fraction", `"character":0`, `"character":0.5`},
		{"overflow", `"character":0`, `"character":4294967296`},
		{"URI", `file:///fixture/a.go`, `file:///fixture/../a.go`},
		{"version", `.v2`, `.v99`},
		{"root-id", `"id":"root"`, `"id":"primary"`},
	} {
		t.Run(change.name, func(t *testing.T) {
			raw := strings.Replace(validManifest, change.old, change.new, 1)
			for _, mode := range []string{"slice_v2", "incoming_v2"} {
				r := &forbiddenRuntime{}
				_, fail := NewExecutor(r).Execute(context.Background(), request(mode, `{"session_id":"s","generation":1,"seed_manifest":`+raw+`}`))
				if fail == nil || fail.Code != "INVALID_INPUT" || r.calls != 0 {
					t.Fatalf("ASSERT_PREFLIGHT_BEFORE_RUNTIME: failure=%v calls=%d", fail, r.calls)
				}
			}
		})
	}
}
func request(mode, raw string) operation.Request {
	return operation.Request{Name: operation.Name(mode), Input: json.RawMessage(raw)}
}

type v3ParityRuntime struct {
	profile runtimeprofile.Profile
	uri     string
	queries int
}

func (r *v3ParityRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, ""
}
func (r *v3ParityRuntime) Records() []sessionruntime.Record {
	return []sessionruntime.Record{{SessionID: "fixture", Profile: r.profile, Generation: 9007199254740993, State: session.Ready}}
}
func (r *v3ParityRuntime) PrepareDocument(_ context.Context, req sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	content := []byte("package fixture\n")
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": req.URI, "languageId": "go", "version": 1, "text": string(content)}})
	return sessionruntime.DocumentResult{URI: req.URI, LanguageID: "go", Supply: &sessionruntime.DocumentSupply{URI: req.URI, SessionID: req.SessionID, Generation: req.Generation, DocumentVersion: 1, Classification: graphprovenance.Supplied, Method: "textDocument/didOpen", Params: params, Content: content}}
}
func (r *v3ParityRuntime) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	if req.Method == "textDocument/prepareCallHierarchy" {
		item := lsp.CallHierarchyItem{Name: "leaf", Kind: 12, URI: r.uri, Range: lsp.Range{End: lsp.Position{Character: 4}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 4}}}
		raw, _ := json.Marshal([]lsp.CallHierarchyItem{item})
		return sessionruntime.RoundTripResult{Result: raw}
	}
	return sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}
}
func (r *v3ParityRuntime) Diagnostics(string, uint64) manageddiagnostic.QueryResult {
	r.queries++
	return manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}
}
func (r *v3ParityRuntime) SeedCustodyProvenance(string, uint64) (seedbinding.CustodyMode, bool) {
	return seedbinding.CallerAssertedLocal, true
}

func TestFR23CanonicalPublicSurfaceByteParity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "fr23-parity", Workspace: root, Profile: "fake", EnvironmentReference: "hermetic"})
	if err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	manifest := Manifest{SchemaVersion: ManifestVersion, CoordinateConvention: "zero-based-session", Root: Target{ID: "root", Locator: lspLocator(uri)}, RequiredTargets: []Target{}}
	typed, _ := json.Marshal(Input{SessionID: "fixture", Generation: 9007199254740993, SeedManifest: manifest})
	manifestRaw, _ := json.Marshal(manifest)
	mcpRaw := []byte(`{"session_id":"fixture","generation":9007199254740993,"seed_manifest":` + string(manifestRaw) + `}`)
	run := func(name operation.Name, input []byte) ([]byte, *v3ParityRuntime) {
		runtime := &v3ParityRuntime{profile: runtimeprofile.Resolve(selector), uri: uri}
		got, failure := NewExecutor(runtime).Execute(context.Background(), operation.Request{Name: name, Input: input})
		if failure != nil {
			t.Fatalf("ASSERT_FR23_PUBLIC_PARITY_EXECUTION/%s: %v", name, failure)
		}
		receipt, ok := got.CustodyReceipt.(*CustodyReceipt)
		if !ok || receipt.Provenance != seedbinding.CallerAssertedLocal || receipt.Authenticated {
			t.Fatalf("ASSERT_CALLER_ASSERTED_LOCAL_PUBLIC_RECEIPT_NO_PROMOTION/%s: %#v", name, got.CustodyReceipt)
		}
		return got.Artifact, runtime
	}
	cliV3, cliRuntime := run(SliceV3, typed)
	mcpV3, mcpRuntime := run(SliceV3, mcpRaw)
	if !bytes.Equal(cliV3, mcpV3) || cliRuntime.queries != 1 || mcpRuntime.queries != 1 {
		t.Fatalf("ASSERT_FR23_V3_CANONICAL_BYTES: equal=%v queries=%d/%d", bytes.Equal(cliV3, mcpV3), cliRuntime.queries, mcpRuntime.queries)
	}
	if _, err := graphprovenance.ValidateFor(cliV3, graphprovenance.Family, "v3"); err != nil {
		t.Fatal(err)
	}
	cliV2, _ := run(Slice, typed)
	mcpV2, _ := run(Slice, mcpRaw)
	if !bytes.Equal(cliV2, mcpV2) {
		t.Fatal("ASSERT_FR23_V2_DEFAULT_BYTES_FROZEN")
	}
}

func TestManagedV5ExplicitOutputAndNoDowngrade(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "v5", Workspace: root, Profile: "fake", EnvironmentReference: "hermetic"})
	if err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	manifest := Manifest{SchemaVersion: ManifestVersion, CoordinateConvention: "zero-based-session", Root: Target{ID: "root", Locator: lspLocator(uri)}, RequiredTargets: []Target{}, Expansion: Expansion{TopmostSiblings: true}}
	runtime := &v3ParityRuntime{profile: runtimeprofile.Resolve(selector), uri: uri}
	input, _ := json.Marshal(Input{SessionID: "fixture", Generation: 9007199254740993, SeedManifest: manifest, OutputVersion: graphprovenance.VersionV5})
	got, failure := NewExecutor(runtime).Execute(context.Background(), operation.Request{Name: SliceV3, Input: input})
	if failure != nil {
		t.Fatalf("ASSERT_MANAGED_V5_SUCCESS: %v", failure)
	}
	if _, err := graphprovenance.ValidateFor(got.Artifact, graphprovenance.Family, "v5"); err != nil {
		t.Fatalf("ASSERT_MANAGED_V5_VALID: %v", err)
	}
	manifest.Expansion.TopmostSiblings = false
	bad, _ := json.Marshal(Input{SessionID: "fixture", Generation: 9007199254740993, SeedManifest: manifest, OutputVersion: graphprovenance.VersionV5})
	before := runtime.queries
	if _, failure := NewExecutor(runtime).Execute(context.Background(), operation.Request{Name: SliceV3, Input: bad}); failure == nil || failure.Code != operation.FailureInvalidInput || runtime.queries != before {
		t.Fatalf("ASSERT_MANAGED_V5_EXPANSION_REQUIRED_BEFORE_ACQUISITION: failure=%v", failure)
	}
}

func lspLocator(uri string) acquisition.Locator {
	zero := uint32(0)
	return acquisition.Locator{URI: uri, Line: &zero, Character: &zero, LanguageID: "go"}
}

func TestManifestDefaultsAndBounds(t *testing.T) {
	m, err := DecodeManifest([]byte(validManifest), Slice)
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.Request(Slice)
	if err != nil || r.Root.DownDepth != 2 || r.Root.UpDepth != 2 || r.Limits.MaxRequests != 1000 {
		t.Fatal("ASSERT_EXPLICIT_DEFAULTS")
	}
	zero := 0
	m.Root.DownDepth = &zero
	m.Root.UpDepth = &zero
	m.Limits.MaxNodes = &zero
	m.Limits.MaxRequests = &zero
	m.Limits.MaxEvidenceBytes = &zero
	m.Limits.MaxPathWork = &zero
	r, err = m.Request(Incoming)
	if err != nil || r.Root.DownDepth != 0 || r.Root.UpDepth != 0 || r.Limits.MaxRequests != 0 || r.Limits.MaxNodes != 0 || r.Limits.MaxPathWork != 0 || r.Limits.MaxEvidenceBytes != 0 {
		t.Fatal("ASSERT_ZERO_NOT_DEFAULT")
	}
	for i := 0; i < 63; i++ {
		target := m.Root
		target.ID = fmt.Sprintf("target-%d", i)
		m.RequiredTargets = append(m.RequiredTargets, target)
	}
	if _, err := m.Request(Slice); err != nil {
		t.Fatal("ASSERT_64_TOTAL_ACCEPTED", err)
	}
	m.RequiredTargets = append(m.RequiredTargets, m.RequiredTargets[0])
	if _, err := m.Request(Slice); err == nil {
		t.Fatal("ASSERT_65_TOTAL_REJECTED")
	}
	m.RequiredTargets = m.RequiredTargets[:63]
	m.RequiredTargets[1].ID = m.RequiredTargets[0].ID
	if _, err := m.Request(Slice); err == nil {
		t.Fatal("ASSERT_DUPLICATE_IDS_REJECTED")
	}
}
