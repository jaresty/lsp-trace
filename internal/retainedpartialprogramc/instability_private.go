package retainedpartialprogramc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"

	"lsp-trace/internal/programc"
)

const InstabilityProfile = "DERIVED_RETAINED_PARTIAL_INSTABILITY_PRIVATE_V1"
const InstabilityPolicy = "private-instability-v1;ordered-unique-seeds-2..6;one-run-per-seed;all-unordered-pairs;RuleB-exact-rational-matching;m<63;n*2^m<=1000000;overflow-safe-division-preflight;rates<=0.05;VI<=0.10;minimum-Jaccard>=0.80;one-independent-source;no-replay;all-empty-zero-occurrence=EMPTY;empty-pairs=NOT_APPLICABLE_EMPTY;failure-or-cancel=unsealed-INCOMPLETE;retain-all-declared-rows"

type InstabilityRunPrivate struct {
	ID            string
	Seed          uint64
	Status, Error string
	Partition     programc.PartitionMathPrivate
}
type InstabilityPairPrivate struct {
	Left, Right, Status string
	Metrics             programc.PureInstabilityMetrics
}
type InstabilityPrivate struct {
	retained                                                                  []byte
	Profile, PolicySHA256, RequestSHA256, Digest                              string
	ProjectionProfile, ProjectionDigest, InputSHA256, NativeGraphSHA256       string
	ClaimCeiling, Admission, SourceCompleteness, FeatureIdentity              string
	Authority                                                                 int
	Accepted, Partial, CanonicalComplete, Replay                              bool
	IndependentSources                                                        int
	Seeds                                                                     []uint64
	Runs                                                                      []InstabilityRunPrivate
	Pairs                                                                     []InstabilityPairPrivate
	ExpectedRuns, CompleteRuns, EmptyRuns, FailedRuns, NotAttemptedRuns       int
	ExpectedPairs, CompletePairs, BlockedPairs, EmptyPairs, NotAttemptedPairs int
	Outcome, Error                                                            string
}

func CloneInstabilityPrivate(in InstabilityPrivate) InstabilityPrivate {
	out := in
	out.Seeds = append([]uint64(nil), in.Seeds...)
	out.Runs = append([]InstabilityRunPrivate(nil), in.Runs...)
	for i := range out.Runs {
		out.Runs[i].Partition = programc.ClonePartitionMathPrivate(in.Runs[i].Partition)
	}
	out.Pairs = append([]InstabilityPairPrivate(nil), in.Pairs...)
	out.retained = bytes.Clone(in.retained)
	return out
}
func InstabilityPolicyBytesPrivate() []byte { return []byte(InstabilityPolicy) }
func InstabilityPolicyDigestPrivate() string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(InstabilityPolicyBytesPrivate()))
}
func instabilityDigest(v any) string {
	b, _ := json.Marshal(v)
	return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
}
func validSeeds(seeds []uint64) error {
	if len(seeds) < 2 || len(seeds) > 6 {
		return fmt.Errorf("requires 2..6 seeds")
	}
	seen := map[uint64]bool{}
	for _, s := range seeds {
		if seen[s] {
			return fmt.Errorf("duplicate seed")
		}
		seen[s] = true
	}
	return nil
}
func validatePrivateRequest(ctx context.Context, p Projection, seeds []uint64) error {
	if ctx == nil {
		return fmt.Errorf("nil context")
	}
	if err := validSeeds(seeds); err != nil {
		return err
	}
	if p.state == nil || p.state.self != p.state {
		return fmt.Errorf("sealed projection required")
	}
	return nil
}
func newPrivateAssessment(p Projection, seeds []uint64) InstabilityPrivate {
	s := p.Source()
	a := InstabilityPrivate{Profile: InstabilityProfile, PolicySHA256: InstabilityPolicyDigestPrivate(), ProjectionProfile: ProfileID, ProjectionDigest: ProfileDigest, InputSHA256: s.InputSHA256, NativeGraphSHA256: s.NativeGraphSHA256, ClaimCeiling: ClaimCeiling, Admission: Admission, SourceCompleteness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Partial: true, IndependentSources: 1, Seeds: append([]uint64(nil), seeds...), ExpectedRuns: len(seeds), ExpectedPairs: len(seeds) * (len(seeds) - 1) / 2, Outcome: "INCOMPLETE"}
	a.RequestSHA256 = instabilityDigest(struct {
		Input, Graph, Projection, Policy string
		Seeds                            []uint64
	}{s.InputSHA256, s.NativeGraphSHA256, ProfileDigest, a.PolicySHA256, a.Seeds})
	for i, s := range seeds {
		a.Runs = append(a.Runs, InstabilityRunPrivate{ID: fmt.Sprintf("run-%d-seed-%d", i, s), Seed: s, Status: "NOT_ATTEMPTED"})
	}
	for i := range seeds {
		for j := i + 1; j < len(seeds); j++ {
			a.Pairs = append(a.Pairs, InstabilityPairPrivate{Left: a.Runs[i].ID, Right: a.Runs[j].ID, Status: "NOT_ATTEMPTED"})
		}
	}
	return a
}
func finishPrivate(a InstabilityPrivate, err error) InstabilityPrivate {
	a.CompleteRuns = 0
	a.EmptyRuns = 0
	a.FailedRuns = 0
	a.NotAttemptedRuns = 0
	for _, r := range a.Runs {
		switch r.Status {
		case "COMPLETE":
			a.CompleteRuns++
		case "EMPTY":
			a.EmptyRuns++
		case "FAILED":
			a.FailedRuns++
		case "NOT_ATTEMPTED":
			a.NotAttemptedRuns++
		}
	}
	a.CompletePairs = 0
	a.BlockedPairs = 0
	a.EmptyPairs = 0
	a.NotAttemptedPairs = 0
	for _, p := range a.Pairs {
		switch p.Status {
		case "COMPLETE":
			a.CompletePairs++
		case "BLOCKED_MATCHING_RESOURCE":
			a.BlockedPairs++
		case "NOT_APPLICABLE_EMPTY":
			a.EmptyPairs++
		case "NOT_ATTEMPTED":
			a.NotAttemptedPairs++
		}
	}
	if err != nil {
		a.Outcome = "INCOMPLETE"
		a.Error = err.Error()
		a.retained = nil
	}
	a.Digest = ""
	a.Digest = instabilityDigest(a)
	return CloneInstabilityPrivate(a)
}

// ComputeInstabilityPrivate is closed to arbitrary producers. Context cancellation
// is cooperative between operations, not a hard CPU/heap guarantee. Failure
// returns detached, unsealed INCOMPLETE accounting plus the original error.
func ComputeInstabilityPrivate(ctx context.Context, p Projection, seeds []uint64) (InstabilityPrivate, error) {
	return computeInstabilityPrivate(ctx, p, seeds, ComputeProjection)
}

// Unexported dependency parameter supports deterministic fault tests only. There
// is no public callback, source-admission factory, retry, or mutable global hook.
func computeInstabilityPrivate(ctx context.Context, p Projection, seeds []uint64, compute func(Projection, uint64) (Outcome, *programc.Failure)) (InstabilityPrivate, error) {
	if err := validatePrivateRequest(ctx, p, seeds); err != nil {
		return InstabilityPrivate{}, err
	}
	a := newPrivateAssessment(p, seeds)
	fail := func(err error) (InstabilityPrivate, error) { return finishPrivate(a, err), err }
	for i, s := range a.Seeds {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		o, f := compute(p, s)
		if f != nil {
			a.Runs[i].Status = "FAILED"
			a.Runs[i].Error = f.Error()
			return fail(f)
		}
		a.Runs[i].Partition = programc.ClonePartitionMathPrivate(o.Math)
		a.Runs[i].Status = o.Math.Outcome
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
	}
	a, err := assessPrivate(ctx, p, a.Seeds, a.Runs)
	if err != nil {
		return a, err
	}
	a.retained, _ = json.Marshal(a)
	return a, nil
}
func assessPrivate(ctx context.Context, p Projection, seeds []uint64, runs []InstabilityRunPrivate) (InstabilityPrivate, error) {
	if err := validatePrivateRequest(ctx, p, seeds); err != nil {
		return InstabilityPrivate{}, err
	}
	a := newPrivateAssessment(p, seeds)
	fail := func(err error) (InstabilityPrivate, error) { return finishPrivate(a, err), err }
	if len(runs) != len(seeds) {
		return fail(fmt.Errorf("missing run"))
	}
	empty := len(p.state.math.Occurrences) == 0
	for i, r := range runs {
		want := "COMPLETE"
		if empty {
			want = "EMPTY"
		}
		if r.ID != a.Runs[i].ID || r.Seed != seeds[i] || r.Status != want || r.Error != "" || r.Partition.Seed != r.Seed || r.Partition.Outcome != want || r.Partition.Algorithm == "" || r.Partition.Resolution != 1 || r.Partition.LogicalDigest == "" {
			return fail(fmt.Errorf("run binding"))
		}
		if i > 0 && r.Partition.Algorithm != runs[0].Partition.Algorithm {
			return fail(fmt.Errorf("mixed algorithm"))
		}
		if err := validatePrivatePartition(p.state.math.NodeIdentities, r.Partition.Communities); err != nil {
			return fail(err)
		}
		if r.Partition.LogicalDigest != programc.PartitionDigestMathPrivate(r.Partition.Communities) {
			return fail(fmt.Errorf("partition logical digest"))
		}
		r.Partition = programc.ClonePartitionMathPrivate(r.Partition)
		a.Runs[i] = r
	}
	a.Outcome = "STABLE"
	if empty {
		a.Outcome = "EMPTY"
	}
	k := 0
	for i := range runs {
		for j := i + 1; j < len(runs); j++ {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			if empty {
				a.Pairs[k].Status = "NOT_APPLICABLE_EMPTY"
			} else {
				m, err := programc.ComparePartitionsMathPrivate(p.state.math.NodeIdentities, runs[i].Partition.Communities, runs[j].Partition.Communities)
				if err != nil {
					return fail(err)
				}
				a.Pairs[k].Metrics = m
				a.Pairs[k].Status = "COMPLETE"
				if m.Outcome == "BLOCKED_MATCHING_RESOURCE" {
					a.Pairs[k].Status = m.Outcome
				} else if m.Outcome == "UNSTABLE" {
					a.Outcome = "UNSTABLE"
				}
			}
			k++
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
		}
	}
	for _, p := range a.Pairs {
		if p.Status == "BLOCKED_MATCHING_RESOURCE" {
			a.Outcome = "RESOURCE_INCOMPLETE"
			break
		}
	}
	return finishPrivate(a, nil), nil
}

// Validate partition membership without matching; EMPTY must not encounter the
// exponential matcher and cannot turn absent occurrence evidence into STABLE.
func validatePrivatePartition(nodes []string, communities []programc.Community) error {
	universe := map[string]bool{}
	for _, n := range nodes {
		universe[n] = true
	}
	seen := map[string]bool{}
	for i, c := range communities {
		if !slices.IsSorted(c.Members) || (i > 0 && slices.Compare(communities[i-1].Members, c.Members) >= 0) {
			return fmt.Errorf("noncanonical partition")
		}
		if len(c.Members) == 0 {
			return fmt.Errorf("empty community")
		}
		for _, n := range c.Members {
			if !universe[n] || seen[n] {
				return fmt.Errorf("partition membership")
			}
			seen[n] = true
		}
	}
	if len(seen) != len(nodes) {
		return fmt.Errorf("partition coverage")
	}
	return nil
}

// ValidateInstabilityPrivate recomputes metrics from retained sealed partitions,
// never reruns Leiden. Unsealed INCOMPLETE diagnostics are not metric reports.
func ValidateInstabilityPrivate(ctx context.Context, p Projection, expected InstabilityPrivate, raw []byte) error {
	if ctx == nil {
		return fmt.Errorf("nil context")
	}
	if len(raw) > 16<<20 {
		return fmt.Errorf("artifact byte cap")
	}
	if expected.Outcome == "INCOMPLETE" || len(expected.retained) == 0 || len(expected.Runs) > 6 || len(expected.Pairs) > 15 {
		return fmt.Errorf("sealed completed assessment required")
	}
	supplied, err := marshalInstabilityPrivateBounded(p, expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(supplied, expected.retained) {
		return fmt.Errorf("sealed completed assessment required")
	}
	a, err := assessPrivate(ctx, p, expected.Seeds, expected.Runs)
	if err != nil {
		return err
	}
	want, _ := json.Marshal(a)
	if !bytes.Equal(raw, want) {
		return fmt.Errorf("private instability canonical mismatch")
	}
	return nil
}
