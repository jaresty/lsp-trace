package seedexport

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const (
	testSession    = "session-7"
	testInvocation = "invocation-7"
	testRevision   = "commit-abc"
	testSource     = "caller-source"
	testManifest   = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func fixture(t *testing.T) ValidatedCarrier {
	t.Helper()
	graph, err := json.Marshal(map[string]any{
		"schema_version": "lsp-trace.graph.v5",
		"invocation": map[string]any{
			"provenance": map[string]any{"invocation_id": testInvocation, "source": testSource, "source_revision": testRevision},
			"seeds": []any{
				map[string]any{"label": "beta", "at": "file:///workspace/b.go:4:2"},
				map[string]any{"label": "alpha", "at": "file:///workspace/a.go#Run"},
			},
		},
		"nodes": []any{map[string]any{"id": "must-not-be-a-seed", "name": "fallback"}},
		"edges": []any{map[string]any{"kind": "CALLS"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	graphSum := sha256.Sum256(graph)
	envelope := map[string]any{
		"schema_version":  "lsp-trace.graph-provenance.v5",
		"session_id":      testSession,
		"generation":      7,
		"graph_v5":        base64.StdEncoding.EncodeToString(graph),
		"graph_v5_sha256": fmt.Sprintf("sha256:%x", graphSum),
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	envelopeSum := sha256.Sum256(raw)
	line, character := uint32(3), uint32(1)
	return ValidatedCarrier{
		GraphProvenanceV5: raw,
		Targets: []Target{
			{Ordinal: 1, ID: "target-b", Label: "beta", Locator: Locator{Kind: LocatorPosition, Value: "file:///workspace/b.go", PathSemantics: PathSemanticsURI, Position: &Position{Line: line, Character: character}}, DownDepth: 3, UpDepth: 1},
			{Ordinal: 0, ID: "target-a", Label: "alpha", Locator: Locator{Kind: LocatorSymbol, Value: "file:///workspace/a.go", PathSemantics: PathSemanticsURI, Symbol: "Run", LanguageID: "go"}, DownDepth: 2, UpDepth: 4},
		},
		Binding: Provenance{SessionID: testSession, Generation: 7, InvocationID: testInvocation, Source: testSource, Revision: testRevision, EnvelopeSHA256: fmt.Sprintf("sha256:%x", envelopeSum), GraphV5SHA256: fmt.Sprintf("sha256:%x", graphSum), ManifestSHA256: testManifest, ManifestSessionID: testSession, ManifestGeneration: 7, ManifestInvocationID: testInvocation},
	}
}

func TestExportV5ReportsPreciseMissingCarrierWithoutNodeFallback(t *testing.T) {
	c := fixture(t)
	got := ExportV5(c.GraphProvenanceV5)
	if got.Outcome != OutcomeNotAvailable || got.Model != nil || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "ORIGINAL_REQUEST_CARRIER_MISSING" {
		t.Fatalf("ASSERT_RAW_V5_NOT_AVAILABLE_NO_NODE_FALLBACK: %#v", got)
	}
}

func TestExportValidatedPreservesOriginalTargetsAndCanonicalOrder(t *testing.T) {
	got := ExportValidated(fixture(t))
	if got.Outcome != OutcomeAvailable || got.Model == nil {
		t.Fatalf("ASSERT_VALIDATED_CARRIER_AVAILABLE: %#v", got)
	}
	if ids := []string{got.Model.Targets[0].ID, got.Model.Targets[1].ID}; !reflect.DeepEqual(ids, []string{"target-a", "target-b"}) {
		t.Fatalf("ASSERT_CANONICAL_ORDINAL_ORDER: %v", ids)
	}
	if got.Model.Targets[0].Locator.Kind != LocatorSymbol || got.Model.Targets[0].Locator.Symbol != "Run" || got.Model.Targets[1].Locator.Kind != LocatorPosition || *got.Model.Targets[1].Locator.Position != (Position{Line: 3, Character: 1}) {
		t.Fatalf("ASSERT_EXACT_LOCATOR_KIND_AND_ZERO_BASED_COORDINATES: %#v", got.Model.Targets)
	}
	if got.Model.Targets[0].DownDepth != 2 || got.Model.Targets[0].UpDepth != 4 || got.Model.Provenance.CustodyClaimed || got.Model.BodyCompleteness != BodyCompletenessUnassessed {
		t.Fatalf("ASSERT_BOUNDS_NONCUSTODIAL_PROVENANCE_BODY_UNASSESSED: %#v", got.Model)
	}
}

func TestExportValidatedRejectsCarrierMutations(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ValidatedCarrier)
		outcome Outcome
		code    string
	}{
		{"absent targets", func(c *ValidatedCarrier) { c.Targets = nil }, OutcomeNotAvailable, "ORIGINAL_TARGETS_ABSENT"},
		{"duplicate targets", func(c *ValidatedCarrier) { c.Targets[1].ID = c.Targets[0].ID }, OutcomeInvalid, "DUPLICATE_TARGET"},
		{"duplicate labels", func(c *ValidatedCarrier) { c.Targets[1].Label = c.Targets[0].Label }, OutcomeInvalid, "DUPLICATE_TARGET_LABEL"},
		{"duplicate ordinals", func(c *ValidatedCarrier) { c.Targets[1].Ordinal = c.Targets[0].Ordinal }, OutcomeInvalid, "DUPLICATE_TARGET_ORDINAL"},
		{"invocation binding", func(c *ValidatedCarrier) { c.Binding.InvocationID = "other" }, OutcomeInvalid, "MANIFEST_BINDING_MISMATCH"},
		{"manifest binding absent", func(c *ValidatedCarrier) { c.Binding.ManifestSHA256 = "" }, OutcomeNotAvailable, "MANIFEST_BINDING_MISSING"},
		{"manifest binding inconsistent", func(c *ValidatedCarrier) { c.Binding.ManifestInvocationID = "other" }, OutcomeInvalid, "MANIFEST_BINDING_MISMATCH"},
		{"malformed position", func(c *ValidatedCarrier) { c.Targets[0].Locator.Position = nil }, OutcomeInvalid, "MALFORMED_LOCATOR"},
		{"mixed locator", func(c *ValidatedCarrier) { c.Targets[0].Locator.Symbol = "also-symbol" }, OutcomeInvalid, "MALFORMED_LOCATOR"},
		{"malformed depth", func(c *ValidatedCarrier) { c.Targets[0].DownDepth = 65 }, OutcomeInvalid, "MALFORMED_BOUNDS"},
		{"provenance mismatch", func(c *ValidatedCarrier) {
			c.Binding.GraphV5SHA256 = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}, OutcomeInvalid, "PROVENANCE_MISMATCH"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := fixture(t)
			tc.mutate(&c)
			got := ExportValidated(c)
			if got.Outcome != tc.outcome || got.Model != nil || len(got.Diagnostics) == 0 || got.Diagnostics[0].Code != tc.code {
				t.Fatalf("ASSERT_MUTATION_FAILS_CLOSED_%s: %#v", strings.ToUpper(strings.ReplaceAll(tc.name, " ", "_")), got)
			}
		})
	}
}

func TestExportValidatedIsDeterministicAndDiagnosticsAreBoundedSafe(t *testing.T) {
	c := fixture(t)
	first, second := ExportValidated(c), ExportValidated(c)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("ASSERT_DETERMINISTIC_MODEL: %#v != %#v", first, second)
	}
	c.Binding.Source = strings.Repeat("/private/source/body/secret", 1000)
	got := ExportValidated(c)
	encoded, _ := json.Marshal(got.Diagnostics)
	if len(got.Diagnostics) > 8 || len(encoded) > 2048 || strings.Contains(string(encoded), "/private/") || strings.Contains(string(encoded), "secret") {
		t.Fatalf("ASSERT_BOUNDED_PRIVATE_SAFE_DIAGNOSTICS: %s", encoded)
	}
}
