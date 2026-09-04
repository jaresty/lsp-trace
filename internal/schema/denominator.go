package schema

import (
	"encoding/json"
	"fmt"
)

type sourceDenominatorV1 struct {
	Scope struct {
		IndependentlyBounded bool `json:"independently_bounded"`
	} `json:"scope"`
	ScopePolicy struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	} `json:"scope_policy"`
	Members []struct {
		MemberID string `json:"member_id"`
	} `json:"members"`
	MembershipCommitment *struct {
		MemberIDs        []string `json:"member_ids"`
		EqualityVerified bool     `json:"equality_verified"`
	} `json:"membership_commitment"`
	Exclusions []struct {
		MemberID string `json:"member_id"`
	} `json:"exclusions"`
	CoveredMembers []struct {
		MemberID string `json:"member_id"`
	} `json:"covered_members"`
	Counts struct {
		Universe int `json:"universe"`
		Members  int `json:"members"`
		Excluded int `json:"excluded"`
		Covered  int `json:"covered"`
	} `json:"counts"`
	ExtractorQualification struct {
		ScopePolicyID      string `json:"scope_policy_id"`
		ScopePolicyVersion string `json:"scope_policy_version"`
		Status             string `json:"status"`
	} `json:"extractor_qualification"`
}

// ValidateSourceDenominator verifies the cross-field proof invariants that JSON
// Schema cannot express: independently bounded scope, exact qualification,
// canonical identity sets, counts, and exhaustive coverage-or-exclusion.
func ValidateSourceDenominator(data []byte) error {
	var d sourceDenominatorV1
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("decode denominator: %w", err)
	}
	if !d.Scope.IndependentlyBounded {
		return fmt.Errorf("scope universe must be independently bounded")
	}
	q := d.ExtractorQualification
	if q.Status != "PASS" {
		return fmt.Errorf("extractor qualification must resolve to exact-match PASS")
	}
	if q.ScopePolicyID != d.ScopePolicy.ID || q.ScopePolicyVersion != d.ScopePolicy.Version {
		return fmt.Errorf("qualification scope policy must exactly match denominator scope policy")
	}

	members := make(map[string]struct{}, len(d.Members))
	memberIDs := make([]string, 0, len(d.Members))
	for _, member := range d.Members {
		memberIDs = append(memberIDs, member.MemberID)
	}
	if d.MembershipCommitment != nil {
		if !d.MembershipCommitment.EqualityVerified {
			return fmt.Errorf("membership commitment must have independently verified equality")
		}
		memberIDs = append(memberIDs, d.MembershipCommitment.MemberIDs...)
	}
	for _, memberID := range memberIDs {
		if _, exists := members[memberID]; exists {
			return fmt.Errorf("duplicate denominator member %q", memberID)
		}
		members[memberID] = struct{}{}
	}
	excluded := make(map[string]struct{}, len(d.Exclusions))
	for _, exclusion := range d.Exclusions {
		if _, exists := members[exclusion.MemberID]; !exists {
			return fmt.Errorf("exclusion member %q is not in denominator members", exclusion.MemberID)
		}
		if _, exists := excluded[exclusion.MemberID]; exists {
			return fmt.Errorf("duplicate exclusion member %q", exclusion.MemberID)
		}
		excluded[exclusion.MemberID] = struct{}{}
	}
	covered := make(map[string]struct{}, len(d.CoveredMembers))
	for _, coverage := range d.CoveredMembers {
		if _, exists := members[coverage.MemberID]; !exists {
			return fmt.Errorf("covered member %q is not in denominator members", coverage.MemberID)
		}
		if _, exists := covered[coverage.MemberID]; exists {
			return fmt.Errorf("duplicate covered member %q", coverage.MemberID)
		}
		if _, exists := excluded[coverage.MemberID]; exists {
			return fmt.Errorf("member %q is both covered and excluded", coverage.MemberID)
		}
		covered[coverage.MemberID] = struct{}{}
	}
	if d.Counts.Universe != len(members) {
		return fmt.Errorf("universe count %d does not match unique members %d", d.Counts.Universe, len(members))
	}
	if d.Counts.Members != len(memberIDs) {
		return fmt.Errorf("members count %d does not match members %d", d.Counts.Members, len(memberIDs))
	}
	if d.Counts.Excluded != len(excluded) || d.Counts.Covered != len(covered) {
		return fmt.Errorf("coverage counts do not match exclusions and covered members")
	}
	if len(excluded)+len(covered) != len(members) {
		return fmt.Errorf("members must be exhausted by covered members and typed exclusions")
	}
	return nil
}
