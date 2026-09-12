package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func foundationRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func assertFoundationDependency(t *testing.T, mod, sum string) {
	t.Helper()
	const version = "gonum.org/v1/gonum v0.17.1-0.20260426204603-69ca49f456a7"
	const xsysVersion = "golang.org/x/sys v0.36.0"
	checks := map[string]bool{
		"ASSERT_FOUNDATION_GONUM_EXACT_VERSION": strings.Contains(mod, version),
		"ASSERT_FOUNDATION_GONUM_MODULE_SUM":    strings.Contains(sum, version+" h1:V43GU8qUQ/EYbEPGzTP0PANTR6RiJgEJdU/vgsFhQiU="),
		"ASSERT_FOUNDATION_GONUM_GOMOD_SUM":     strings.Contains(sum, version+"/go.mod h1:El3tOrEuMpv2UdMrbNlKEh9vd86bmQ6vqIcDwxEOc1E="),
		"ASSERT_FOUNDATION_GONUM_NO_REPLACE":    !strings.Contains(mod, "replace gonum.org/v1/gonum"),
		"ASSERT_FOUNDATION_X_SYS_EXACT_VERSION": strings.Contains(mod, xsysVersion),
		"ASSERT_FOUNDATION_X_SYS_MODULE_SUM":    strings.Contains(sum, xsysVersion+" h1:KVRy2GtZBrk1cBYA7MKu5bEZFxQk4NIDV6RLVcC8o0k="),
		"ASSERT_FOUNDATION_X_SYS_GOMOD_SUM":     strings.Contains(sum, xsysVersion+"/go.mod h1:OgkHotnGiDImocRcuBABYBEXf8A9a87e/uXjp9XT3ks="),
		"ASSERT_FOUNDATION_X_SYS_NO_REPLACE":    !strings.Contains(mod, "replace golang.org/x/sys"),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("%s", name)
		}
	}
}

func assertFoundationNotices(t *testing.T, license, notices, goreleaser string) {
	t.Helper()
	checks := map[string]bool{
		"ASSERT_FOUNDATION_ROOT_MIT_PRESERVED":         strings.Contains(license, "MIT License"),
		"ASSERT_FOUNDATION_GONUM_ATTRIBUTION":          strings.Contains(notices, "Copyright ©2013 The Gonum Authors. All rights reserved."),
		"ASSERT_FOUNDATION_GONUM_BSD_CONDITIONS":       strings.Contains(notices, "Redistributions of source code must retain") || strings.Contains(notices, "Redistributions of source code") || strings.Contains(notices, "Redistribution and use in source and binary forms"),
		"ASSERT_FOUNDATION_GONUM_NO_ENDORSEMENT":       strings.Contains(notices, "Neither the name of the Gonum project"),
		"ASSERT_FOUNDATION_GONUM_DISCLAIMER":           strings.Contains(notices, "THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS \"AS IS\""),
		"ASSERT_FOUNDATION_X_SYS_HEADING":              strings.Contains(notices, "golang.org/x/sys v0.36.0\n=========================="),
		"ASSERT_FOUNDATION_X_SYS_COPYRIGHT":            strings.Contains(notices, "golang.org/x/sys v0.36.0\n==========================\n\nCopyright 2009 The Go Authors."),
		"ASSERT_FOUNDATION_X_SYS_BINARY_NOTICE":        strings.Contains(notices, "Redistributions in binary form must reproduce the above copyright"),
		"ASSERT_FOUNDATION_X_SYS_DISCLAIMER":           strings.Count(notices, "THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS") >= 3,
		"ASSERT_FOUNDATION_ARCHIVE_LICENSE":            strings.Contains(goreleaser, "- LICENSE"),
		"ASSERT_FOUNDATION_ARCHIVE_NOTICES":            strings.Contains(goreleaser, "- THIRD_PARTY_NOTICES"),
		"ASSERT_FOUNDATION_ARCHIVE_NO_PROVIDER_ASSETS": !strings.Contains(goreleaser, "providers/"),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("%s", name)
		}
	}
}

func TestFoundationDependencyAndNoticeContracts(t *testing.T) {
	root := foundationRoot(t)
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	assertFoundationDependency(t, read("go.mod"), read("go.sum"))
	assertFoundationNotices(t, read("LICENSE"), read("THIRD_PARTY_NOTICES"), read(".goreleaser.yaml"))
}

func validIdentity() map[string]any {
	d := "sha256:" + strings.Repeat("a", 64)
	return map[string]any{"graph_provenance_v5_sha256": d, "admitted_graph_sha256": d, "projection_sha256": d, "projection_policy_id": "p", "algorithm_name": "leiden", "algorithm_version": "v", "parameters_canonical_sha256": d, "seed": 1, "resource_policy_sha256": d}
}

func TestCommunitySchemasStrictBoundedAndRegistered(t *testing.T) {
	d := "sha256:" + strings.Repeat("a", 64)
	community := map[string]any{"schema_version": "lsp-trace.community.v1", "input_identity": validIdentity(), "outcome": "EMPTY", "communities": []any{}, "accounting": map[string]any{"admitted_node_count": 0, "accounted_node_count": 0, "unaccounted_node_count": 0}, "claim_ceiling": "Communities are structural observations under the exact bound Graph Provenance V5 input and projection. They do not establish feature, product, ownership, service, organizational, business-boundary, semantic-community, producer-authentication, permission, or production-authority claims."}
	boundaryPolicy := map[string]any{"id": "program-c-a-07-boundary-accounting/v2", "conductance": "cut_weight(C)/(min(directed_out_volume(C),directed_out_volume(V\\C)))", "conductance_zero_outcome": "UNAVAILABLE:zero minimum directed out-volume", "pagerank": map[string]any{"alpha": .85, "tolerance": 1e-9, "max_iterations": 1000, "max_work": 1000000, "arithmetic": "Go-binary64-explicit-rounding-sequential/v1", "score": "positive-binary64-node-score/v1", "work": "evaluation-3n+m/v1", "convergence": "stationary-L1-inclusive/v1"}, "pagerank_selection": "required-top-k-inclusive-ties-then-crossing/v1", "hub_selection": "required-top-k-inclusive-ties-then-crossing/v1", "hub_score": "weighted-directed-in-plus-out-occurrence-sum/v1", "weak_critical_semantics": "weak-undirected-occurrence-multigraph-bridge-articulation/v1", "crossing_witness_semantics": "direct-community-pair-minimum-occurrence-identity/v1"}
	boundary := map[string]any{"schema_version": "lsp-trace.community-boundary.v1", "outcome": "EMPTY", "bindings": map[string]any{"source_sha256": d, "session_id": "s", "generation": 1, "profile_id": "calls-v1", "profile_sha256": d, "algorithm": "leiden", "algorithm_version": "v", "partition_sha256": d, "policy_sha256": "sha256:f6d68b67ff8117c573f795b95697accd85e345998228cb3ce0b59864c967d8d7", "seed": 1}, "request": map[string]any{"pagerank_top_k": 1, "hub_top_k": 1}, "policy": boundaryPolicy, "rank_policies": []any{"initial-p/v1", "dangling-p/v1", "stationary-L1-inclusive/v1", "lexical-nodes-occurrence-ID/v1", "Go-binary64-explicit-rounding-sequential/v1", "positive-binary64-node-score/v1", "evaluation-3n+m/v1"}, "pagerank": map[string]any{"status": "COMPLETE", "reason": "", "iterations": 0, "work": 0, "residual": 0, "residual_iteration": 0}, "accounting": map[string]any{"admitted_relation_occurrence_count": 0, "accounted_relation_occurrence_count": 0, "unaccounted_relation_occurrence_count": 0, "intra_relation_occurrence_count": 0, "crossing_relation_occurrence_count": 0, "admitted_node_count": 1, "accounted_node_count": 1, "unaccounted_node_count": 0}, "communities": []any{}, "high_centrality_crossing_nodes": []any{}, "hub_crossing_nodes": []any{}, "bridges": []any{}, "articulation_points": []any{}, "crossing_witnesses": []any{}, "claim_ceiling": "All measures and witnesses are structural observations under the exact bound projection. They do not establish feature, product, ownership, service, organizational, or business-boundary identity or explanation.", "digest": d}
	comparison := validIdentity()
	delete(comparison, "graph_provenance_v5_sha256")
	delete(comparison, "seed")
	instabilityPolicy := map[string]any{"version": "program-c-a-08-instability/v1", "matching": "maximum-total-exact-jaccard-one-to-one-with-explicit-unmatched-sentinels/v1", "tie_break": "lexicographic-left-member-set-then-right-member-set/v1", "pair_order": "descending-score-then-left-member-set-then-right-member-set/v1", "missing_nodes": "explicit-singleton-missing-side-bins/v1", "arithmetic": "Go-binary64-log2-canonical-node-order/v1", "max_runs": 300, "max_pairs": 44850, "max_matching_work": 1000000}
	instability := map[string]any{"schema_version": "lsp-trace.community-instability.v1", "comparison_identity": comparison, "outcome": "INCOMPLETE", "thresholds": map[string]any{"node_reassignment_rate_max": 0.05, "unmatched_community_rate_max": 0.05, "variation_of_information_bits_max": 0.1, "pairwise_jaccard_minimum_min": 0.8}, "policy_version": "program-c-a-08-instability/v1", "policy": instabilityPolicy, "policy_sha256": "sha256:b44cef6e72f2e808202961b94f91a45d97e8bcaded8e0e91ebf6cda64a5bd67c", "run_accounting": map[string]any{"declared_seed_count": 2, "declared_run_count": 6, "completed_run_count": 0, "failed_run_count": 0, "incomplete_run_count": 6, "expected_pair_count": 15, "completed_pair_count": 0, "blocked_pair_count": 15}, "pairwise_comparisons": []any{}, "claim_ceiling": "STABLE means only that the declared metrics met the declared thresholds for the exact bound graph, projection, implementation, parameters, seeds, and completed runs. It is label-independent, structural, and not a universal stability or semantic-community claim.", "structural_claim_ceiling": "Structural output does not establish feature identity, ownership, architecture, runtime execution, whole-source completeness, producer authentication, permission, production authority, or stability beyond the exact compared runs."}
	cases := []struct {
		family string
		doc    map[string]any
	}{{FamilyCommunity, community}, {FamilyCommunityBoundary, boundary}, {FamilyCommunityInstability, instability}}
	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			b, _ := json.Marshal(tc.doc)
			if _, err := ValidateStructure(b, tc.family, "v1"); err != nil {
				t.Fatalf("ASSERT_FOUNDATION_SCHEMA_GREEN_%s: %v", tc.family, err)
			}
			wrong := map[string]any{}
			for k, v := range tc.doc {
				wrong[k] = v
			}
			wrong["unexpected"] = true
			b, _ = json.Marshal(wrong)
			if _, err := ValidateStructure(b, tc.family, "v1"); err == nil {
				t.Fatalf("ASSERT_FOUNDATION_SCHEMA_ADDITIONAL_PROPERTIES_%s", tc.family)
			}
		})
	}
}
