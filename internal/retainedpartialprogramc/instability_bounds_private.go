package retainedpartialprogramc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

const maxInstabilityArtifactPrivate = 16 << 20

// preflightInstabilityPrivate bounds the exported, caller-mutable view BEFORE
// encoding, cloning, membership maps or partition comparisons. The allowance is
// a conservative JSON upper bound: six bytes per input string byte plus fixed
// key/scalar/container overhead. Subtraction prevents arithmetic overflow.
func preflightInstabilityPrivate(p Projection, a InstabilityPrivate) error {
	if p.state == nil || p.state.self != p.state || len(a.retained) == 0 || len(a.retained) > maxInstabilityArtifactPrivate || len(a.Seeds) < 2 || len(a.Seeds) > 6 || len(a.Runs) != len(a.Seeds) || len(a.Pairs) != len(a.Seeds)*(len(a.Seeds)-1)/2 {
		return fmt.Errorf("private assessment bounded seal/shape")
	}
	n := len(p.state.math.NodeIdentities)
	if n == 0 || n > 10000 {
		return fmt.Errorf("private assessment node bound")
	}
	remaining := maxInstabilityArtifactPrivate
	take := func(size int) bool {
		if size < 0 || size > remaining {
			return false
		}
		remaining -= size
		return true
	}
	text := func(s string, cap int) bool {
		if len(s) > cap || len(s) > remaining/6 {
			return false
		}
		remaining -= len(s) * 6
		return utf8.ValidString(s) && take(64)
	}
	if !take(4096) {
		return fmt.Errorf("private assessment encoded bound")
	}
	for _, s := range []string{a.Profile, a.PolicySHA256, a.RequestSHA256, a.Digest, a.ProjectionProfile, a.ProjectionDigest, a.InputSHA256, a.NativeGraphSHA256, a.ClaimCeiling, a.Admission, a.SourceCompleteness, a.FeatureIdentity, a.Outcome, a.Error} {
		if !text(s, 4096) {
			return fmt.Errorf("private assessment string bound")
		}
	}
	for _, r := range a.Runs {
		if len(r.Partition.Communities) > n {
			return fmt.Errorf("private assessment community bound")
		}
		if !take(2048) {
			return fmt.Errorf("private assessment encoded bound")
		}
		for _, s := range []string{r.ID, r.Status, r.Error, r.Partition.Outcome, r.Partition.Algorithm, r.Partition.LogicalDigest} {
			if !text(s, 1024) {
				return fmt.Errorf("private assessment run string bound")
			}
		}
		members := 0
		for _, c := range r.Partition.Communities {
			if len(c.Members) == 0 || len(c.Members) > n-members {
				return fmt.Errorf("private assessment member bound")
			}
			members += len(c.Members)
			if !take(128) {
				return fmt.Errorf("private assessment encoded bound")
			}
			for _, member := range c.Members {
				if !text(member, 1024) {
					return fmt.Errorf("private assessment member string/encoded bound")
				}
			}
		}
		if members != n {
			return fmt.Errorf("private assessment partition count")
		}
	}
	for _, pair := range a.Pairs {
		if !take(2048) {
			return fmt.Errorf("private assessment encoded bound")
		}
		for _, s := range []string{pair.Left, pair.Right, pair.Status, pair.Metrics.Outcome, pair.Metrics.Guard} {
			if !text(s, 1024) {
				return fmt.Errorf("private assessment pair string bound")
			}
		}
	}
	return nil
}

func marshalInstabilityPrivateBounded(p Projection, a InstabilityPrivate) ([]byte, error) {
	if err := preflightInstabilityPrivate(p, a); err != nil {
		return nil, err
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	if len(b) > maxInstabilityArtifactPrivate {
		return nil, fmt.Errorf("private assessment encoded cap")
	}
	return b, nil
}

// ValidatedInstabilityBytesPrivate compares the bounded caller-visible view to
// its retained seal and validates the actual partitions/metrics, without Leiden.
// Returned bytes are detached and preserve the historical canonical encoding.
// Callers must not mutate inputs concurrently with validation/preparation.
func ValidatedInstabilityBytesPrivate(ctx context.Context, p Projection, a InstabilityPrivate) ([]byte, error) {
	if err := ValidateInstabilityPrivate(ctx, p, a, a.retained); err != nil {
		return nil, err
	}
	return bytes.Clone(a.retained), nil
}
