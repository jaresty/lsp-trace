package sourceprojection

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/sessionruntime"
)

func validLiveSupply() (*sessionruntime.DocumentSupply, LiveExpectation) {
	uri := "file:///fixture/src/source.ts"
	supply := &sessionruntime.DocumentSupply{Classification: "LSP_SUPPLIED", SessionID: "fixture-live-session", Generation: 1, URI: uri, DocumentVersion: 7, Method: "textDocument/didOpen", Content: []byte("function target() {\n  \"😀\"; target();\n}\n")}
	return supply, LiveExpectation{SessionID: supply.SessionID, Generation: supply.Generation, URI: uri, PositionEncoding: "utf-16"}
}

func TestResolveLiveExactSupplyIdentity(t *testing.T) {
	supply, expected := validLiveSupply()
	got, err := ResolveLive(supply, expected)
	if err != nil {
		t.Fatalf("ASSERT_LIVE_RESOLVER_EXACT_SUPPLY: %v", err)
	}
	if got.Binding.Custody != "LIVE" || got.Binding.SessionID != expected.SessionID || got.Binding.Generation != expected.Generation || got.Binding.URI != expected.URI || got.Binding.DocumentVersion != 7 || got.Binding.PositionEncoding != "utf-16" || got.Binding.SourceDigest != "sha256:b2f615ccc7e60a8994f75a1e5cae0a73e0e7f34967a71288eabaad7840d121d5" || got.Binding.SourceByteLength != 42 {
		t.Fatalf("ASSERT_LIVE_RESOLVER_BINDING: %+v", got.Binding)
	}
	if got.PhysicalProjectionID != "sha256:5ad8d5aeb675ed66ff0d6f407a99edcdfbe340c21ee85dbd3caa3ae798273521" {
		t.Fatalf("ASSERT_LIVE_RESOLVER_PHYSICAL_ID: %s", got.PhysicalProjectionID)
	}
	if got.Source.LogicalSourceID != expected.URI || got.Source.Digest != got.Binding.SourceDigest || !got.Source.Available || string(got.Source.Bytes) != string(supply.Content) {
		t.Fatalf("ASSERT_LIVE_RESOLVER_SOURCE: %+v", got.Source)
	}
	supply.Content[0] = 'X'
	if got.Source.Bytes[0] != 'f' {
		t.Fatal("ASSERT_LIVE_RESOLVER_OWNS_BYTES")
	}
}

func TestAssembleLiveSchemaAndNeutrality(t *testing.T) {
	supply, expected := validLiveSupply()
	resolved, err := ResolveLive(supply, expected)
	if err != nil {
		t.Fatal(err)
	}
	candidates := fixtureCandidates()
	for i := range candidates {
		candidates[i].LogicalSourceID = expected.URI
	}
	core, err := Project(candidates, map[string]Source{expected.URI: resolved.Source}, Policy{PolicyID: "public", BodyRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	const policyID = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	got, err := AssembleLive(resolved, policyID, core)
	if err != nil {
		t.Fatalf("ASSERT_LIVE_ASSEMBLY_SCHEMA: %v", err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := mcpcontract.ValidateJSON(mcpcontract.SourceProjectionResultV1ID, raw); err != nil {
		t.Fatalf("ASSERT_LIVE_ASSEMBLY_SCHEMA: %v\n%s", err, raw)
	}
	if got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.GraphFactsAdded != 0 || got.CustodyMode != "LIVE" || got.PhysicalProjectionID != resolved.PhysicalProjectionID || got.RequestPolicyID != policyID || got.Accounting != core.Accounting {
		t.Fatalf("ASSERT_LIVE_ASSEMBLY_NEUTRAL_EXACT: %+v", got)
	}
}

func TestAssembleLiveResponseLimitFailsClosed(t *testing.T) {
	supply, expected := validLiveSupply()
	resolved, err := ResolveLive(supply, expected)
	if err != nil {
		t.Fatal(err)
	}
	core := Result{Status: "SUCCESSFUL_EMPTY", Units: []Unit{}, Citations: []Citation{}, EmittedSpans: []Span{}, Omissions: []Omission{}, PrivacySummary: PrivacySummary{PolicyID: "public"}}
	got, err := AssembleLiveBounded(resolved, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", core, 1)
	if err == nil || got.PhysicalProjectionID != "" {
		t.Fatalf("ASSERT_LIVE_ASSEMBLY_RESPONSE_LIMIT: result=%+v err=%v", got, err)
	}
}

func TestResolveLiveRejectsIdentityAndContentMutation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*sessionruntime.DocumentSupply, *LiveExpectation)
	}{
		{"nil", nil},
		{"classification", func(s *sessionruntime.DocumentSupply, _ *LiveExpectation) { s.Classification = "CALLER_ASSERTED" }},
		{"session", func(s *sessionruntime.DocumentSupply, _ *LiveExpectation) { s.SessionID = "other" }},
		{"generation", func(s *sessionruntime.DocumentSupply, _ *LiveExpectation) { s.Generation = 2 }},
		{"uri", func(s *sessionruntime.DocumentSupply, _ *LiveExpectation) { s.URI = "file:///other.go" }},
		{"version", func(s *sessionruntime.DocumentSupply, _ *LiveExpectation) { s.DocumentVersion = 0 }},
		{"encoding", func(_ *sessionruntime.DocumentSupply, e *LiveExpectation) { e.PositionEncoding = "utf-8" }},
		{"invalid-utf8", func(s *sessionruntime.DocumentSupply, _ *LiveExpectation) { s.Content = []byte{0xff} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			supply, expected := validLiveSupply()
			if tc.mutate == nil {
				supply = nil
			} else {
				tc.mutate(supply, &expected)
			}
			got, err := ResolveLive(supply, expected)
			if err == nil || got.PhysicalProjectionID != "" || got.Source.Bytes != nil {
				t.Fatalf("ASSERT_LIVE_RESOLVER_FAIL_CLOSED_%s: result=%+v err=%v", tc.name, got, err)
			}
		})
	}
}
