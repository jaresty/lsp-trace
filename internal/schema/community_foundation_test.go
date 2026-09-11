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
	checks := map[string]bool{
		"ASSERT_FOUNDATION_GONUM_EXACT_VERSION": strings.Contains(mod, version),
		"ASSERT_FOUNDATION_GONUM_MODULE_SUM":    strings.Contains(sum, version+" h1:V43GU8qUQ/EYbEPGzTP0PANTR6RiJgEJdU/vgsFhQiU="),
		"ASSERT_FOUNDATION_GONUM_GOMOD_SUM":     strings.Contains(sum, version+"/go.mod h1:El3tOrEuMpv2UdMrbNlKEh9vd86bmQ6vqIcDwxEOc1E="),
		"ASSERT_FOUNDATION_GONUM_NO_REPLACE":    !strings.Contains(mod, "replace gonum.org/v1/gonum"),
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
	boundaryID := validIdentity()
	delete(boundaryID, "graph_provenance_v5_sha256")
	boundaryID["community_artifact_sha256"] = d
	boundary := map[string]any{"schema_version": "lsp-trace.community-boundary.v1", "input_identity": boundaryID, "outcome": "EMPTY", "accounting": map[string]any{"admitted_relation_occurrence_count": 0, "accounted_relation_occurrence_count": 0, "unaccounted_relation_occurrence_count": 0, "intra_relation_occurrence_count": 0, "crossing_relation_occurrence_count": 0, "admitted_node_count": 0, "accounted_node_count": 0, "unaccounted_node_count": 0}, "communities": []any{}, "claim_ceiling": "All measures and witnesses are structural observations under the exact bound projection. They do not establish feature, product, ownership, service, organizational, or business-boundary identity or explanation."}
	comparison := validIdentity()
	delete(comparison, "graph_provenance_v5_sha256")
	delete(comparison, "seed")
	comparison["left_community_artifact_sha256"] = d
	comparison["right_community_artifact_sha256"] = d
	instability := map[string]any{"schema_version": "lsp-trace.community-instability.v1", "comparison_identity": comparison, "outcome": "INCOMPLETE", "thresholds": map[string]any{"node_reassignment_rate_max": 0.05, "unmatched_community_rate_max": 0.05, "variation_of_information_bits_max": 0.1, "pairwise_jaccard_minimum_min": 0.8}, "run_accounting": map[string]any{"declared_seed_count": 2, "declared_run_count": 6, "completed_run_count": 0, "failed_run_count": 0, "incomplete_run_count": 6, "expected_pair_count": 15, "completed_pair_count": 0, "blocked_pair_count": 15}, "pairwise_comparisons": []any{}, "claim_ceiling": "STABLE means only that the declared metrics met the declared thresholds for the exact bound graph, projection, implementation, parameters, seeds, and completed runs. It is label-independent, structural, and not a universal stability or semantic-community claim."}
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
