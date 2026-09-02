package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
)

var requiredFilterSupports = []string{"EXACT_REFERENCE_INTERSECTION", "EXACT_REFERENCE_DIFFERENCE"}
var requiredFilterExclusions = []string{"SHARED_FEATURE_PURPOSE", "DISTINCT_FEATURE_PURPOSE", "FEATURE_IDENTITY", "WORKFLOW_IDENTITY", "MERGE_OR_SPLIT_DISPOSITION", "INDEPENDENT_OBSERVATION", "EVIDENTIARY_SUPPORT", "CONFIDENCE", "COVERAGE", "RUNTIME_BEHAVIOR", "ACCEPTANCE"}

type semanticPartition struct {
	Shared    []json.RawMessage `json:"shared"`
	LeftOnly  []json.RawMessage `json:"left_only"`
	RightOnly []json.RawMessage `json:"right_only"`
}
type semanticAccounting struct {
	LeftReferenceCount      int `json:"left_reference_count"`
	RightReferenceCount     int `json:"right_reference_count"`
	SharedReferenceCount    int `json:"shared_reference_count"`
	LeftOnlyReferenceCount  int `json:"left_only_reference_count"`
	RightOnlyReferenceCount int `json:"right_only_reference_count"`
	PairUniverseCount       int `json:"pair_universe_count"`
}

// ValidateFilter validates cross-field filter.v1 accounting, partition, seed, and claim-ceiling invariants.
func ValidateFilter(data []byte) error {
	var p struct {
		Operands struct {
			Left  string `json:"left_seed_label"`
			Right string `json:"right_seed_label"`
		} `json:"operands"`
		Seeds []struct {
			Label, State string
			Failure      json.RawMessage `json:"failure"`
		} `json:"seeds"`
		Partitions map[string]semanticPartition `json:"partitions"`
		Accounting struct {
			RequestedSeedCount                   int                `json:"requested_seed_count"`
			SuccessfulSeedCount                  int                `json:"successful_seed_count"`
			FailedSeedCount                      int                `json:"failed_seed_count"`
			SuccessfulSeedWithMembershipCount    int                `json:"successful_seed_with_membership_count"`
			SuccessfulSeedWithoutMembershipCount int                `json:"successful_seed_without_membership_count"`
			Nodes                                semanticAccounting `json:"nodes"`
			Calls                                semanticAccounting `json:"call_relations"`
			Dispatch                             semanticAccounting `json:"dispatch_relationships"`
			Siblings                             semanticAccounting `json:"sibling_candidates"`
			Diagnostics                          semanticAccounting `json:"diagnostic_correlations"`
		} `json:"accounting"`
		ClaimCeiling struct {
			Supports       []string `json:"supports"`
			DoesNotSupport []string `json:"does_not_support"`
		} `json:"claim_ceiling"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if len(p.Seeds) != 2 || p.Operands.Left == p.Operands.Right || p.Seeds[0].Label != p.Operands.Left || p.Seeds[1].Label != p.Operands.Right {
		return fmt.Errorf("operands and selected seed order do not reconcile")
	}
	for _, seed := range p.Seeds {
		failed := seed.State == "FAILED"
		isNull := string(seed.Failure) == "null"
		if failed == isNull {
			return fmt.Errorf("seed %q state and failure do not reconcile", seed.Label)
		}
	}
	if p.Accounting.SuccessfulSeedCount+p.Accounting.FailedSeedCount != p.Accounting.RequestedSeedCount || p.Accounting.SuccessfulSeedWithMembershipCount+p.Accounting.SuccessfulSeedWithoutMembershipCount != p.Accounting.SuccessfulSeedCount {
		return fmt.Errorf("common seed accounting does not reconcile")
	}
	if !reflect.DeepEqual(p.ClaimCeiling.Supports, requiredFilterSupports) || !reflect.DeepEqual(p.ClaimCeiling.DoesNotSupport, requiredFilterExclusions) {
		return fmt.Errorf("claim ceiling does not match filter.v1")
	}
	checks := []struct {
		name string
		p    semanticPartition
		a    semanticAccounting
	}{{"nodes", p.Partitions["nodes"], p.Accounting.Nodes}, {"call_relations", p.Partitions["call_relations"], p.Accounting.Calls}, {"dispatch_relationships", p.Partitions["dispatch_relationships"], p.Accounting.Dispatch}, {"sibling_candidates", p.Partitions["sibling_candidates"], p.Accounting.Siblings}, {"diagnostic_correlations", p.Partitions["diagnostic_correlations"], p.Accounting.Diagnostics}}
	leftTotal, rightTotal := 0, 0
	for _, c := range checks {
		if hasPartitionDuplicates(c.p) {
			return fmt.Errorf("duplicate or overlapping filter reference in %s", c.name)
		}
		s, l, r := len(c.p.Shared), len(c.p.LeftOnly), len(c.p.RightOnly)
		if c.a.SharedReferenceCount != s || c.a.LeftOnlyReferenceCount != l || c.a.RightOnlyReferenceCount != r || c.a.LeftReferenceCount != s+l || c.a.RightReferenceCount != s+r || c.a.PairUniverseCount != s+l+r {
			return fmt.Errorf("invalid filter accounting for %s", c.name)
		}
		leftTotal += c.a.LeftReferenceCount
		rightTotal += c.a.RightReferenceCount
	}
	for i, total := range []int{leftTotal, rightTotal} {
		want := "SUCCESSFUL_WITH_EVIDENCE"
		if p.Seeds[i].State == "FAILED" {
			want = "FAILED"
		} else if total == 0 {
			want = "SUCCESSFUL_EMPTY"
		}
		if p.Seeds[i].State != want || (want == "FAILED" && total != 0) {
			return fmt.Errorf("seed %q state and reference counts do not reconcile", p.Seeds[i].Label)
		}
	}
	return nil
}

func hasPartitionDuplicates(p semanticPartition) bool {
	seen := map[string]bool{}
	for _, group := range [][]json.RawMessage{p.Shared, p.LeftOnly, p.RightOnly} {
		for _, v := range group {
			key := string(v)
			if seen[key] {
				return true
			}
			seen[key] = true
		}
	}
	return false
}
