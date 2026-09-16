package liveprojection

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"lsp-trace/internal/sourceprojection"
	"lsp-trace/sessionruntime"
)

const (
	assertComposeExactResolution = "ASSERT_COMPOSE_EXACT_LIVE_RESOLUTION"
	assertComposeAllOrNone       = "ASSERT_COMPOSE_ALL_OR_NONE"
	assertComposeDeterministic   = "ASSERT_COMPOSE_DETERMINISTIC_MULTI_SOURCE"
	assertComposeLimits          = "ASSERT_COMPOSE_INDEPENDENT_PROJECTION_LIMITS"
	assertComposeRanges          = "ASSERT_COMPOSE_PRESERVES_RANGE_ROLES"
	assertComposeProvenance      = "ASSERT_COMPOSE_SERVER_REPORTED_PROVENANCE"
	assertComposeNeutral         = "ASSERT_COMPOSE_GRAPH_NEUTRAL"
	assertComposePrivate         = "ASSERT_COMPOSE_RAW_SUPPLIES_PRIVATE"
)

func compositionFixture() (PreparationResult, []sourceprojection.Candidate, sourceprojection.Policy) {
	const (
		target = "file:///workspace/target.go"
		caller = "file:///workspace/caller.go"
	)
	supplies := []*sessionruntime.DocumentSupply{
		{Classification: "LSP_SUPPLIED", SessionID: "session", Generation: 4, URI: target, DocumentVersion: 1, Method: "textDocument/didOpen", Content: []byte("target()\n")},
		{Classification: "LSP_SUPPLIED", SessionID: "session", Generation: 4, URI: caller, DocumentVersion: 2, Method: "textDocument/didChange", Content: []byte("caller()\n")},
	}
	prepared := PreparationResult{Status: PreparationComplete, Supplies: supplies}
	candidates := []sourceprojection.Candidate{
		{UnitID: "endpoint", CitationID: "citation-endpoint", Role: "ENDPOINT", GraphSubjectID: "node-target", LogicalSourceID: target, Range: projectionRange(0, 0, 0, 6), EvidenceRange: projectionRange(0, 1, 0, 5), ItemRange: projectionRange(0, 0, 0, 8), SelectionRange: projectionRange(0, 1, 0, 5), PositionEncoding: "utf-16", PrivacyClassification: "PUBLIC"},
		{UnitID: "relation", CitationID: "citation-relation", Role: "RELATION", GraphSubjectID: "edge-caller-target", OccurrenceID: "occurrence", LogicalSourceID: caller, Range: projectionRange(0, 0, 0, 6), EvidenceRange: projectionRange(0, 0, 0, 6), PositionEncoding: "utf-16", RelationProvenance: "SERVER_REPORTED", PrivacyClassification: "PUBLIC"},
	}
	policy := sourceprojection.Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 1024, MaxRanges: 8, MaxObjects: 8, MaxWork: 8, EnforceLimits: true}
	return prepared, candidates, policy
}

func projectionRange(sl, sc, el, ec uint32) sourceprojection.Range {
	return sourceprojection.Range{Start: sourceprojection.Position{Line: sl, Character: sc}, End: sourceprojection.Position{Line: el, Character: ec}}
}

func TestComposeResolvesEverySupplyAndProjectsDeterministically(t *testing.T) {
	t.Log(assertComposeExactResolution, assertComposeDeterministic, assertComposeRanges, assertComposeProvenance)
	prepared, candidates, policy := compositionFixture()
	original := append([]sourceprojection.Candidate(nil), candidates...)
	got := Compose(prepared, "session", 4, candidates, policy)
	if got.Status != CompositionComplete || len(got.Bindings) != 2 || len(got.Resolutions) != 2 {
		t.Fatalf("%s: result=%+v", assertComposeExactResolution, got)
	}
	if got.Bindings[0].URI != "file:///workspace/caller.go" || got.Bindings[1].URI != "file:///workspace/target.go" || len(got.PhysicalProjectionIDs) != 2 {
		t.Fatalf("%s: bindings=%+v ids=%q", assertComposeDeterministic, got.Bindings, got.PhysicalProjectionIDs)
	}
	if !reflect.DeepEqual(candidates, original) {
		t.Fatalf("%s: candidates mutated\n got=%+v\nwant=%+v", assertComposeRanges, candidates, original)
	}
	units := append([]sourceprojection.Unit(nil), got.Projection.Units...)
	slices.SortFunc(units, func(a, b sourceprojection.Unit) int { return strings.Compare(a.UnitID, b.UnitID) })
	if len(units) != 2 || units[0].Range != candidates[0].Range {
		t.Fatalf("%s: units=%+v", assertComposeRanges, units)
	}
	if units[1].RelationProvenance != "SERVER_REPORTED" {
		t.Fatalf("%s: units=%+v", assertComposeProvenance, units)
	}

	permutedPrepared := prepared
	permutedPrepared.Supplies = []*sessionruntime.DocumentSupply{prepared.Supplies[1], prepared.Supplies[0]}
	permutedCandidates := []sourceprojection.Candidate{candidates[1], candidates[0]}
	again := Compose(permutedPrepared, "session", 4, permutedCandidates, policy)
	if !reflect.DeepEqual(got, again) {
		t.Fatalf("%s: first=%+v second=%+v", assertComposeDeterministic, got, again)
	}
}

func TestComposeResolutionFailureIsAllOrNone(t *testing.T) {
	t.Log(assertComposeAllOrNone)
	prepared, candidates, policy := compositionFixture()
	prepared.Supplies[0].Generation++
	got := Compose(prepared, "session", 4, candidates, policy)
	if got.Status != CompositionFailed || got.Failure == nil || got.Failure.URI != "file:///workspace/target.go" || len(got.Resolutions) != 0 || len(got.Projection.Units) != 0 {
		t.Fatalf("%s: result=%+v", assertComposeAllOrNone, got)
	}
}

func TestComposeUsesIndependentProjectionLimits(t *testing.T) {
	t.Log(assertComposeLimits)
	prepared, candidates, policy := compositionFixture()
	policy.MaxWork = 1
	got := Compose(prepared, "session", 4, candidates, policy)
	if got.Status != CompositionFailed || got.Failure == nil || !strings.Contains(got.Failure.Cause, "projection work limit") || len(got.Resolutions) != 0 {
		t.Fatalf("%s: result=%+v", assertComposeLimits, got)
	}
}

func TestComposeIsGraphNeutralAndRawSupplyPrivate(t *testing.T) {
	t.Log(assertComposeNeutral, assertComposePrivate)
	prepared, candidates, policy := compositionFixture()
	got := Compose(prepared, "session", 4, candidates, policy)
	if got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.GraphFactsAdded != 0 {
		t.Fatalf("%s: result=%+v", assertComposeNeutral, got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	for _, forbidden := range []string{"target()", "caller()", "LSP_SUPPLIED", "DocumentVersion", "Method", "Content", "Params", "resolutions"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("%s: found=%q json=%s", assertComposePrivate, forbidden, encoded)
		}
	}
}
