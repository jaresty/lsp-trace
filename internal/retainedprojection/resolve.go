package retainedprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceobject"
)

const (
	CodeInvalidPlan              Code = "INVALID_PLAN"
	CodeReturnedIdentityMismatch Code = "RETURNED_IDENTITY_MISMATCH"
	CodeResolvedLengthMismatch   Code = "RESOLVED_LENGTH_MISMATCH"
	CodeResolvedDigestMismatch   Code = "RESOLVED_DIGEST_MISMATCH"
	CodeResolveDistinctLimit     Code = "RESOLVE_DISTINCT_OBJECT_LIMIT"
	CodeResolveSourceBytesLimit  Code = "RESOLVE_UNIQUE_SOURCE_BYTES_LIMIT"
	CodeResolveSelectionLimit    Code = "RESOLVE_LOGICAL_SELECTION_LIMIT"
)

type Lookup interface {
	Get(sourceobject.Identity) (sourceobject.Object, error)
}

type ResolveLimits struct {
	MaxDistinctObjects   uint64
	MaxUniqueSourceBytes uint64
	MaxLogicalSelections uint64
}

type ResolvedSelection struct {
	Selection Selection
	Bytes     []byte
}

type ResolveResult struct {
	Selections []ResolvedSelection
}

func Resolve(plan Plan, lookup Lookup, limits ResolveLimits) (ResolveResult, error) {
	zero := ResolveResult{}
	if err := validateResolvePlan(plan, lookup, limits, true); err != nil {
		return zero, err
	}
	if uint64(len(plan.Selections)) > limits.MaxLogicalSelections {
		return zero, fail(CodeResolveSelectionLimit, Key{}, "logical selection limit exceeded")
	}

	identities := make([]sourceobject.Identity, 0, len(plan.Selections))
	seen := make(map[sourceobject.Identity]struct{}, len(plan.Selections))
	var uniqueBytes uint64
	for _, selection := range plan.Selections {
		id := selection.Source
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		identities = append(identities, id)
		if uint64(len(identities)) > limits.MaxDistinctObjects {
			return zero, fail(CodeResolveDistinctLimit, selection.Key, "distinct object limit exceeded")
		}
		if id.ByteLength > math.MaxUint64-uniqueBytes || uniqueBytes+id.ByteLength > limits.MaxUniqueSourceBytes {
			return zero, fail(CodeResolveSourceBytesLimit, selection.Key, "unique source byte limit exceeded")
		}
		uniqueBytes += id.ByteLength
	}

	cache := make(map[sourceobject.Identity][]byte, len(identities))
	for _, requested := range identities {
		object, err := lookup.Get(requested)
		if err != nil {
			return zero, err
		}
		if object.Identity != requested {
			return zero, fail(CodeReturnedIdentityMismatch, Key{}, "lookup returned a different identity")
		}
		if uint64(len(object.Bytes)) != requested.ByteLength {
			return zero, fail(CodeResolvedLengthMismatch, Key{}, "resolved bytes differ from identity length")
		}
		sum := sha256.Sum256(object.Bytes)
		if "sha256:"+hex.EncodeToString(sum[:]) != requested.Digest {
			return zero, fail(CodeResolvedDigestMismatch, Key{}, "resolved bytes differ from identity digest")
		}
		cache[requested] = append([]byte(nil), object.Bytes...)
	}

	selections := make([]ResolvedSelection, len(plan.Selections))
	for i, selection := range plan.Selections {
		selections[i] = ResolvedSelection{
			Selection: cloneSelection(selection),
			Bytes:     append([]byte(nil), cache[selection.Source]...),
		}
	}
	return ResolveResult{Selections: selections}, nil
}

func ResolveMetadata(plan Plan, limits ResolveLimits) (ResolveResult, error) {
	zero := ResolveResult{}
	if err := validateResolvePlan(plan, nil, limits, false); err != nil {
		return zero, err
	}
	if uint64(len(plan.Selections)) > limits.MaxLogicalSelections {
		return zero, fail(CodeResolveSelectionLimit, Key{}, "logical selection limit exceeded")
	}
	seen := make(map[sourceobject.Identity]struct{}, len(plan.Selections))
	for _, selection := range plan.Selections {
		seen[selection.Source] = struct{}{}
		if uint64(len(seen)) > limits.MaxDistinctObjects {
			return zero, fail(CodeResolveDistinctLimit, selection.Key, "distinct object limit exceeded")
		}
	}
	selections := make([]ResolvedSelection, len(plan.Selections))
	for i, selection := range plan.Selections {
		selections[i] = ResolvedSelection{Selection: cloneSelection(selection)}
	}
	return ResolveResult{Selections: selections}, nil
}

func validateResolvePlan(plan Plan, lookup Lookup, limits ResolveLimits, requireLookup bool) error {
	if (requireLookup && lookup == nil) || limits.MaxDistinctObjects == 0 || limits.MaxUniqueSourceBytes == 0 || limits.MaxLogicalSelections == 0 {
		return fail(CodeInvalidPlan, Key{}, "required lookup and positive resolver limits required")
	}
	if plan.Ordering != Ordering || len(plan.Selections) == 0 || plan.Target == (Key{}) || plan.Selections[0].Key != plan.Target {
		return fail(CodeInvalidPlan, Key{}, "canonical non-empty plan required")
	}
	keys := make(map[Key]struct{}, len(plan.Selections))
	for i, selection := range plan.Selections {
		if selection.Ordinal != i || selection.Key.GraphSubjectID == "" || selection.Key.LogicalSourceID == "" || !canonicalResolveDigest(selection.Source.Digest) {
			return fail(CodeInvalidPlan, selection.Key, "invalid selection")
		}
		if _, duplicate := keys[selection.Key]; duplicate {
			return fail(CodeInvalidPlan, selection.Key, "duplicate selection key")
		}
		keys[selection.Key] = struct{}{}
		if i > 1 && !lessKey(plan.Selections[i-1].Key, selection.Key) {
			return fail(CodeInvalidPlan, selection.Key, "additional selection keys are not strictly ordered")
		}
		wantRole := "ADDITIONAL"
		if i == 0 {
			wantRole = "TARGET"
		}
		if selection.Role != wantRole {
			return fail(CodeInvalidPlan, selection.Key, "invalid selection role")
		}
	}
	return nil
}

func cloneSelection(selection Selection) Selection {
	cloned := selection
	cloned.EvidenceRanges = append([]EvidenceRange(nil), selection.EvidenceRanges...)
	cloned.ItemRange = cloneRange(selection.ItemRange)
	cloned.SelectionRange = cloneRange(selection.SelectionRange)
	cloned.CallSiteRange = cloneRange(selection.CallSiteRange)
	return cloned
}

func cloneRange(value *graph.Range) *graph.Range {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func canonicalResolveDigest(digest string) bool {
	if len(digest) != 71 || !strings.HasPrefix(digest, "sha256:") || strings.ToLower(digest) != digest {
		return false
	}
	_, err := hex.DecodeString(digest[7:])
	return err == nil
}
