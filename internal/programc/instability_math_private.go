package programc

import (
	"fmt"
	"sort"
)

// PartitionDigestMathPrivate is the existing canonical logical partition digest,
// with no source/admission semantics. Callers validate canonical membership first.
func PartitionDigestMathPrivate(communities []Community) string { return logicalDigest(communities) }

// PureInstabilityMetrics carries numerical results only, never admission or source claims.
type PureInstabilityMetrics struct {
	Outcome, Guard                                                                   string
	Smaller, Larger                                                                  int
	Work, States                                                                     uint64
	WorkRepresentable                                                                bool
	Reassigned, NodeDenominator, Unmatched, CommunityDenominator, MatchedDenominator int
	Minimum, Median, Mean, VI                                                        float64
}

// privateMatchingGuard checks width and division BEFORE multiplying. Blocked work
// is zero only when unrepresentable; never multiply unless division proves safety.
func privateMatchingGuard(n, m int, limit uint64) (string, uint64) {
	if m >= 63 {
		return "MATCHING_STATE_WIDTH", 0
	}
	if n <= 0 || m <= 0 {
		return "MATCHING_EMPTY", 0
	}
	states := uint64(1) << uint(m)
	if !privateMatchingWorkFits(uint64(n), states, limit) {
		if uint64(n) <= ^uint64(0)/states {
			return "MATCHING_WORK_LIMIT", uint64(n) * states
		}
		return "MATCHING_WORK_LIMIT", 0
	}
	return "", uint64(n) * states
}
func privateMatchingWorkFits(n, states, limit uint64) bool {
	return n > 0 && states > 0 && n <= limit/states
}

// ComparePartitionsMathPrivate reuses the frozen exact matching and canonical VI
// kernels. No ordinary Outcome, receipt, artifact, or validator is constructed.
func ComparePartitionsMathPrivate(nodes []string, left, right []Community) (PureInstabilityMetrics, error) {
	var out PureInstabilityMetrics
	if f := enforceCaps(len(nodes), 0); f != nil {
		return out, f
	}
	if len(nodes) == 0 {
		return out, fmt.Errorf("empty node universe")
	}
	universe := map[string]bool{}
	for i, id := range nodes {
		if id == "" || (i > 0 && nodes[i-1] >= id) {
			return out, fmt.Errorf("noncanonical nodes")
		}
		universe[id] = true
	}
	canonical := func(in []Community) ([]Community, error) {
		if len(in) == 0 || len(in) > len(nodes) {
			return nil, fmt.Errorf("partition community count")
		}
		out := make([]Community, len(in))
		seen := map[string]bool{}
		total := 0
		for i, c := range in {
			if len(c.Members) > len(nodes)-total {
				return nil, fmt.Errorf("partition member count")
			}
			total += len(c.Members)
			if len(c.Members) == 0 {
				return nil, fmt.Errorf("empty community")
			}
			out[i].Members = append([]string(nil), c.Members...)
			sort.Strings(out[i].Members)
			for _, id := range out[i].Members {
				if !universe[id] || seen[id] {
					return nil, fmt.Errorf("partition universe")
				}
				seen[id] = true
			}
		}
		if len(seen) != len(nodes) {
			return nil, fmt.Errorf("partition coverage")
		}
		sort.Slice(out, func(i, j int) bool { return compareStrings(out[i].Members, out[j].Members) < 0 })
		return out, nil
	}
	var err error
	left, err = canonical(left)
	if err != nil {
		return out, err
	}
	right, err = canonical(right)
	if err != nil {
		return out, err
	}
	out.Smaller, out.Larger = len(left), len(right)
	if out.Smaller > out.Larger {
		out.Smaller, out.Larger = out.Larger, out.Smaller
	}
	out.Outcome = "BLOCKED_MATCHING_RESOURCE"
	out.Guard, out.Work = privateMatchingGuard(out.Smaller, out.Larger, uint64(instabilityMaxMatchingWork))
	if out.Larger > 0 && out.Larger < 63 {
		out.States = uint64(1) << uint(out.Larger)
		out.WorkRepresentable = uint64(out.Smaller) <= ^uint64(0)/out.States
	}
	if out.Guard != "" {
		return out, nil
	}
	pairs, err := maximumMatching(left, right)
	if err != nil {
		return out, err
	}
	vals := make([]float64, len(pairs))
	for i, p := range pairs {
		vals[i] = p.score
		if p.l == nil || p.r == nil {
			out.Unmatched++
		}
	}
	sort.Float64s(vals)
	out.MatchedDenominator = len(vals)
	out.Minimum = vals[0]
	out.Median = median(vals)
	for _, v := range vals {
		out.Mean += v
	}
	out.Mean /= float64(len(vals))
	out.CommunityDenominator = len(left) + len(right)
	// Preserve Rule B's score-descending, lexical tie ordering before reassignment.
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score != pairs[j].score {
			return pairs[i].score > pairs[j].score
		}
		if c := compareStrings(pairs[i].l, pairs[j].l); c != 0 {
			return c < 0
		}
		return compareStrings(pairs[i].r, pairs[j].r) < 0
	})
	out.Reassigned, out.NodeDenominator = nodeReassignment(pairs)
	out.VI, _ = variationInformationCanonical(left, right)
	out.Outcome = "STABLE"
	if rate(out.Reassigned, out.NodeDenominator) > 0.05 || rate(out.Unmatched, out.CommunityDenominator) > 0.05 || out.VI > 0.10 || out.Minimum < 0.80 {
		out.Outcome = "UNSTABLE"
	}
	return out, nil
}
