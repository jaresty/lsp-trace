package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
)

func inspectFixture(t *testing.T) ([]byte, string) {
	t.Helper()
	caller := graph.NewNode(graph.Item{Name: "caller", URI: "file:///w/caller.go"})
	callee := graph.NewNode(graph.Item{Name: "callee", URI: "file:///w/callee.go"})
	edges := graph.MergeEdge(nil, graph.Edge{CallerNodeID: caller.ID, CalleeNodeID: callee.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}}})
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV3,
		Invocation:    graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "chosen", At: "callee.go:1:1"}, {Label: "other", At: "other.go:1:1"}}},
		Nodes:         []graph.Node{caller, callee},
		Edges:         edges,
		Diagnostics:   []graph.Diagnostic{{Phase: "traverse", NodeID: callee.ID, Message: "on reached node"}, {Phase: "server", Message: "global only"}},
		Seeds: []graph.SeedResult{
			{Label: "chosen", Requested: graph.Target{URI: callee.URI, Line: 1, Column: 1}, PreparedTargetIDs: []string{callee.ID}, ReachedNodeIDs: []string{caller.ID, callee.ID}, ReachedRelationIDs: []string{edges[0].RelationID}, ReachedEdges: edges},
			{Label: "other", Requested: graph.Target{URI: "file:///w/other.go", Line: 1, Column: 1}, Failure: &graph.SeedFailure{Phase: "prepare", Message: "no item"}},
		},
		Summary: graph.Summary{Complete: true},
	}
	data, err := marshalResult(result, false)
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(artifact, data, 0600); err != nil {
		t.Fatal(err)
	}
	selector := filepath.Join(t.TempDir(), "selector.json")
	if err := publishBundle(selector, data); err != nil {
		t.Fatal(err)
	}
	return data, artifact + "\x00" + selector
}

func TestInspectSeedProjectsSameReadOnlyEvidenceFromArtifactAndSelector(t *testing.T) {
	original, paths := inspectFixture(t)
	artifact, selector, _ := strings.Cut(paths, "\x00")
	var outputs []string
	for _, input := range []string{artifact, selector} {
		stdout, stderr, code := captureRun(t, []string{"inspect", input, "--seed", "chosen", "--json"})
		if code != 0 || stderr != "" {
			t.Fatalf("ASSERT_INSPECT_ACCEPTS_ARTIFACT_OR_SELECTOR: input=%s code=%d stderr=%q", input, code, stderr)
		}
		var got struct {
			ProjectionKind   string `json:"projection_kind"`
			Authority        string `json:"authority"`
			ArtifactIdentity struct {
				SemanticCommitmentDigest   string `json:"semantic_commitment_digest"`
				ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
			} `json:"artifact_identity"`
			PreparationStatus string                 `json:"preparation_status"`
			Seed              graph.SeedResult       `json:"seed"`
			SeedMemberships   []graph.SeedMembership `json:"seed_memberships"`
			Nodes             []graph.Node           `json:"nodes"`
			Relations         []graph.Edge           `json:"relations"`
			Global            struct {
				Summary     map[string]any     `json:"summary"`
				Diagnostics []graph.Diagnostic `json:"diagnostics"`
			} `json:"global"`
			DiagnosticsOnReachedNodes struct {
				Authority   string             `json:"authority"`
				Diagnostics []graph.Diagnostic `json:"diagnostics"`
			} `json:"diagnostics_on_reached_nodes"`
		}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("ASSERT_INSPECT_JSON: %v stdout=%q", err, stdout)
		}
		if got.ProjectionKind != "SEED_INSPECTION" || got.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || got.PreparationStatus != "SUCCEEDED" || got.Seed.Label != "chosen" || strings.Contains(stdout, `"schema_version"`) || len(got.SeedMemberships) != 4 || len(got.Nodes) != 2 || len(got.Relations) != 1 || len(got.Global.Diagnostics) != 2 || len(got.DiagnosticsOnReachedNodes.Diagnostics) != 1 || got.DiagnosticsOnReachedNodes.Authority != "TOOL_DERIVED_NODE_CORRELATION" || got.ArtifactIdentity.SemanticCommitmentDigest == "" || got.ArtifactIdentity.ExactSerializedBytesDigest != graph.ExactBytesDigest(original) {
			t.Fatalf("ASSERT_INSPECT_EXACT_NATIVE_PROJECTION: %#v", got)
		}
		outputs = append(outputs, stdout)
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("ASSERT_INSPECT_SELECTOR_ARTIFACT_EQUIVALENCE: artifact=%s selector=%s", outputs[0], outputs[1])
	}
	after, err := os.ReadFile(artifact)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("ASSERT_INSPECT_READ_ONLY: err=%v equal=%v", err, bytes.Equal(after, original))
	}
}

func TestInspectAllSeedsIsDeterministicEquivalentAndReadOnly(t *testing.T) {
	artifactBytes, paths := inspectFixture(t)
	artifact, selector, _ := strings.Cut(paths, "\x00")
	selectorBefore, err := os.ReadFile(selector)
	if err != nil {
		t.Fatal(err)
	}
	var outputs []string
	for _, input := range []string{artifact, selector, selector} {
		stdout, stderr, code := captureRun(t, []string{"inspect", input, "--all-seeds", "--json"})
		if code != 0 || stderr != "" {
			t.Fatalf("ASSERT_INSPECT_ALL_SEEDS_ACCEPTS_ARTIFACT_OR_SELECTOR: input=%s code=%d stderr=%q", input, code, stderr)
		}
		outputs = append(outputs, stdout)
	}
	if outputs[0] != outputs[1] || outputs[1] != outputs[2] {
		t.Fatalf("ASSERT_INSPECT_ALL_SEEDS_DETERMINISTIC_EQUIVALENCE: artifact=%s selector=%s repeated=%s", outputs[0], outputs[1], outputs[2])
	}
	artifactAfter, artifactErr := os.ReadFile(artifact)
	selectorAfter, selectorErr := os.ReadFile(selector)
	if artifactErr != nil || selectorErr != nil || !bytes.Equal(artifactAfter, artifactBytes) || !bytes.Equal(selectorAfter, selectorBefore) {
		t.Fatalf("ASSERT_INSPECT_ALL_SEEDS_READ_ONLY: artifact_err=%v selector_err=%v artifact_equal=%v selector_equal=%v", artifactErr, selectorErr, bytes.Equal(artifactAfter, artifactBytes), bytes.Equal(selectorAfter, selectorBefore))
	}
}

func TestProjectSeedInspectionNormalizesEmptyCollections(t *testing.T) {
	data, err := json.Marshal(inspectBundle{
		SchemaVersion: graph.SchemaVersionV3,
		Seeds:         []graph.SeedResult{{Label: "empty"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	projection, err := projectSeedInspection(data, "empty")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}

	for _, field := range []string{
		`"prepared_target_ids":[]`,
		`"reached_node_ids":[]`,
		`"diagnostics":[]`,
		`"seed_memberships":[]`,
		`"nodes":[]`,
		`"relations":[]`,
		`"terminals":[]`,
		`"frontier":[]`,
	} {
		if !bytes.Contains(encoded, []byte(field)) {
			t.Errorf("ASSERT_INSPECT_NORMALIZED_EMPTY_COLLECTION: missing %s in %s", field, encoded)
		}
	}
}

func TestInspectProjectsFailedSeedWithoutFabricatingMembership(t *testing.T) {
	_, paths := inspectFixture(t)
	artifact, _, _ := strings.Cut(paths, "\x00")
	stdout, stderr, code := captureRun(t, []string{"inspect", artifact, "--seed", "other"})
	var got struct {
		ProjectionKind    string           `json:"projection_kind"`
		Authority         string           `json:"authority"`
		PreparationStatus string           `json:"preparation_status"`
		Seed              graph.SeedResult `json:"seed"`
		Nodes             []graph.Node     `json:"nodes"`
		Relations         []graph.Edge     `json:"relations"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("ASSERT_INSPECT_FAILED_SEED_JSON: %v code=%d stdout=%q stderr=%q", err, code, stdout, stderr)
	}
	if code != 0 || stderr != "" || got.ProjectionKind != "SEED_INSPECTION" || got.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || got.PreparationStatus != "FAILED" || got.Seed.Failure == nil || got.Seed.Failure.Phase != "prepare" || got.Seed.Failure.Message != "no item" || len(got.Nodes) != 0 || len(got.Relations) != 0 {
		t.Fatalf("ASSERT_INSPECT_FAILED_SEED_STATE_WITHOUT_MEMBERSHIP: code=%d stderr=%q projection=%#v", code, stderr, got)
	}
}

func TestInspectCLICompatibilityModesAndUsage(t *testing.T) {
	_, paths := inspectFixture(t)
	artifact, _, _ := strings.Cut(paths, "\x00")

	t.Run("legacy seed mode remains accepted", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"inspect", artifact, "--seed", "chosen", "--json"})
		if code != 0 || stderr != "" || !json.Valid([]byte(stdout)) {
			t.Fatalf("ASSERT_INSPECT_LEGACY_SEED_MODE_ACCEPTED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})

	t.Run("all-seeds projects copied records and normalized references", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"inspect", artifact, "--all-seeds", "--json"})
		var got struct {
			InspectionSchemaVersion string `json:"inspection_schema_version"`
			ProjectionKind          string `json:"projection_kind"`
			Authority               string `json:"authority"`
			Records                 struct {
				Nodes       []graph.Node       `json:"nodes"`
				Relations   []graph.Edge       `json:"call_relations"`
				Diagnostics []graph.Diagnostic `json:"diagnostics"`
			} `json:"records"`
			Seeds []struct {
				PreparationStatus string                 `json:"preparation_status"`
				Seed              graph.SeedResult       `json:"seed"`
				SeedMemberships   []graph.SeedMembership `json:"seed_memberships"`
				DiagnosticIndexes []int                  `json:"correlated_diagnostic_indexes"`
			} `json:"seeds"`
			Accounting struct {
				RequestedSeeds       int `json:"requested_seed_count"`
				SuccessfulSeeds      int `json:"successful_seed_count"`
				FailedSeeds          int `json:"failed_seed_count"`
				NodeRecords          int `json:"global_node_record_count"`
				CallRelationRecords  int `json:"global_call_relation_record_count"`
				MembershipRecords    int `json:"seed_membership_record_count"`
				CorrelatedReferences int `json:"seed_correlated_diagnostic_reference_count"`
			} `json:"accounting"`
		}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("ASSERT_INSPECT_ALL_SEEDS_JSON: %v code=%d stdout=%q stderr=%q", err, code, stdout, stderr)
		}
		if code != 0 || stderr != "" || got.InspectionSchemaVersion != "lsp-trace.inspect.v1" || got.ProjectionKind != "ALL_SEEDS" || got.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || len(got.Seeds) != 2 || got.Seeds[0].Seed.Label != "chosen" || got.Seeds[0].PreparationStatus != "SUCCEEDED" || len(got.Seeds[0].SeedMemberships) != 4 || len(got.Seeds[0].DiagnosticIndexes) != 1 || got.Seeds[1].Seed.Label != "other" || got.Seeds[1].PreparationStatus != "FAILED" || len(got.Seeds[1].SeedMemberships) != 0 || len(got.Records.Nodes) != 2 || len(got.Records.Relations) != 1 || len(got.Records.Diagnostics) != 2 || got.Accounting.RequestedSeeds != 2 || got.Accounting.SuccessfulSeeds != 1 || got.Accounting.FailedSeeds != 1 || got.Accounting.NodeRecords != 2 || got.Accounting.CallRelationRecords != 1 || got.Accounting.MembershipRecords != 4 || got.Accounting.CorrelatedReferences != 1 {
			t.Fatalf("ASSERT_INSPECT_ALL_SEEDS_NORMALIZED_ACCOUNTED: code=%d stderr=%q projection=%#v", code, stderr, got)
		}
	})

	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "mode required", args: []string{"inspect", artifact}},
		{name: "modes exclusive", args: []string{"inspect", artifact, "--seed", "chosen", "--all-seeds"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := captureRun(t, tc.args)
			if code != 1 || stdout != "" || !strings.Contains(stderr, "usage: lsp-trace inspect SELECTOR_OR_ARTIFACT (--seed LABEL | --all-seeds) [--json]") {
				t.Fatalf("ASSERT_INSPECT_MODE_USAGE: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestValidateAllSeedAccountingRejectsEveryMutatedCount(t *testing.T) {
	data, _ := inspectFixture(t)
	valid, err := projectAllSeedInspection(data)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		field  string
		mutate func(*inspectAllProjection)
	}{
		{"requested_seed_count", func(p *inspectAllProjection) { p.Accounting.RequestedSeedCount++ }},
		{"successful_seed_count", func(p *inspectAllProjection) { p.Accounting.SuccessfulSeedCount++ }},
		{"failed_seed_count", func(p *inspectAllProjection) { p.Accounting.FailedSeedCount++ }},
		{"successful_seed_with_membership_count", func(p *inspectAllProjection) { p.Accounting.SuccessfulSeedWithMembershipCount++ }},
		{"successful_seed_without_membership_count", func(p *inspectAllProjection) { p.Accounting.SuccessfulSeedWithoutMembershipCount++ }},
		{"global_node_record_count", func(p *inspectAllProjection) { p.Accounting.GlobalNodeRecordCount++ }},
		{"global_call_relation_record_count", func(p *inspectAllProjection) { p.Accounting.GlobalCallRelationRecordCount++ }},
		{"global_dispatch_relationship_record_count", func(p *inspectAllProjection) { p.Accounting.GlobalDispatchRelationshipRecordCount++ }},
		{"global_sibling_candidate_record_count", func(p *inspectAllProjection) { p.Accounting.GlobalSiblingCandidateRecordCount++ }},
		{"global_diagnostic_record_count", func(p *inspectAllProjection) { p.Accounting.GlobalDiagnosticRecordCount++ }},
		{"global_terminal_record_count", func(p *inspectAllProjection) { p.Accounting.GlobalTerminalRecordCount++ }},
		{"global_frontier_record_count", func(p *inspectAllProjection) { p.Accounting.GlobalFrontierRecordCount++ }},
		{"seed_membership_record_count", func(p *inspectAllProjection) { p.Accounting.SeedMembershipRecordCount++ }},
		{"seed_node_reference_count", func(p *inspectAllProjection) { p.Accounting.SeedNodeReferenceCount++ }},
		{"seed_call_relation_reference_count", func(p *inspectAllProjection) { p.Accounting.SeedCallRelationReferenceCount++ }},
		{"seed_discovery_nomination_reference_count", func(p *inspectAllProjection) { p.Accounting.SeedDiscoveryNominationReferenceCount++ }},
		{"seed_correlated_diagnostic_reference_count", func(p *inspectAllProjection) { p.Accounting.SeedCorrelatedDiagnosticReferenceCount++ }},
	}
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			projection := valid
			test.mutate(&projection)
			if err := validateAllSeedAccounting(projection); err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("ASSERT_INSPECT_ACCOUNTING_RECONCILES_%s: %v", test.field, err)
			}
		})
	}
}

func TestProjectAllSeedInspectionRequiresExactlyOneResultPerInvocationSeed(t *testing.T) {
	data, _ := inspectFixture(t)
	var bundle inspectBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		label   string
		results []graph.SeedResult
		count   int
	}{
		{"missing", "other", bundle.Seeds[:1], 0},
		{"duplicate", "chosen", append(bundle.Seeds, bundle.Seeds[0]), 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := bundle
			mutated.Seeds = test.results
			encoded, err := json.Marshal(mutated)
			if err != nil {
				t.Fatal(err)
			}
			_, err = projectAllSeedInspection(encoded)
			want := fmt.Sprintf(`seed label %q has %d results; expected exactly one`, test.label, test.count)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("ASSERT_INSPECT_ALL_SEEDS_EXACT_RESULT_CARDINALITY: got=%v want=%q", err, want)
			}
		})
	}
}

func TestInspectRejectsUnknownSeedAndTamperedSelector(t *testing.T) {
	_, paths := inspectFixture(t)
	artifact, selector, _ := strings.Cut(paths, "\x00")
	stdout, stderr, code := captureRun(t, []string{"inspect", artifact, "--seed", "missing"})
	if code != 1 || stdout != "" || !strings.Contains(stderr, `inspect: seed label "missing" not found`) {
		t.Fatalf("ASSERT_INSPECT_UNKNOWN_SEED_FAILS_CLOSED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	selected, err := selectedGenerationFile(selector, generationArtifactName)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selected, append(data, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = captureRun(t, []string{"inspect", selector, "--seed", "chosen"})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "exact-byte integrity mismatch") {
		t.Fatalf("ASSERT_INSPECT_SELECTOR_CUSTODY_FAILS_CLOSED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
