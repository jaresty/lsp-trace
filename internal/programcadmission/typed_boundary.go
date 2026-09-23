package programcadmission

import (
	"errors"
	"sort"
)

// TypedInputVersion is a private staging contract, not an ADR 0011 grouping
// policy, public selector, or authorization to run Leiden on mixed evidence.
const TypedInputVersion = "lsp-trace.private.program-c-typed-staging.v0"

type RelationKind string

const (
	TypedCalls                RelationKind = "CALLS"
	TypedReferencesSymbol     RelationKind = "REFERENCES_SYMBOL"
	TypedResolvesToDefinition RelationKind = "RESOLVES_TO_DEFINITION"
)

// MethodCandidate is deliberately untrusted. Its presence can only reject a
// request until an independent immutable D/R admission contract is implemented.
type MethodCandidate struct {
	Identity string
	Kind     RelationKind
	From, To string
}

type TypedInputRequest struct {
	Calls            CompositeProjectionAdmission
	Kinds            []RelationKind // explicit; no implicit selection in this private API
	MethodCandidates []MethodCandidate
}

// Ordinal is the original server-reported call-site index within its validated
// relation, not the position in the identity-sorted admission slice.
type TypedOccurrence struct {
	Identity, RelationID, From, To string
	Kind                           RelationKind
	Ordinal                        int
}

type TypedPair struct{ From, To string }

type TypedInput struct {
	Version, CompositeID, CompositeOutputSHA256, ClaimCeiling string
	Authority                                                 int
	SourceGraphComplete                                       string
	Occurrences                                               []TypedOccurrence
	PairWeights                                               map[TypedPair]int
}

var ErrMethodEvidenceUnadmitted = errors.New("definition/reference evidence has no admitted immutable occurrence binding")
var ErrTypedSelection = errors.New("unsupported private typed input selection")

// PrepareTypedInput is a CALLS-only compatibility staging seam. It never takes
// candidate D/R records as evidence and never calls Leiden. A future successor
// must independently verify method receipts, query targets, member ledgers and
// occurrence custody, then pin and qualify a new composition policy/version.
func PrepareTypedInput(req TypedInputRequest) (TypedInput, error) {
	if len(req.MethodCandidates) != 0 {
		return TypedInput{}, ErrMethodEvidenceUnadmitted
	}
	if len(req.Kinds) != 1 || req.Kinds[0] != TypedCalls {
		return TypedInput{}, ErrTypedSelection
	}
	if !req.Calls.Valid() || req.Calls.ClaimCeiling() == "" {
		return TypedInput{}, errors.New("validated CALLS composite admission required")
	}
	source := req.Calls.SourceBinding()
	if source.CompositeID == "" || source.CompositeOutputSHA256 == "" || source.ClaimCeiling != req.Calls.ClaimCeiling() || source.Authority != 0 || source.SourceGraphComplete != SourceGraphComplete || source.Completeness.WholeWorkspace {
		return TypedInput{}, errors.New("invalid CALLS composite source binding")
	}
	ids := req.Calls.NodeIdentities()
	if len(ids) > maxNodes {
		return TypedInput{}, errors.New("typed node cap exceeded")
	}
	for i, id := range ids {
		if id == "" || (i > 0 && ids[i-1] >= id) {
			return TypedInput{}, errors.New("noncanonical admitted nodes")
		}
	}
	calls := req.Calls.Occurrences()
	if len(calls) > maxOccurrences {
		return TypedInput{}, errors.New("typed occurrence cap exceeded")
	}
	out := TypedInput{Version: TypedInputVersion, CompositeID: source.CompositeID, CompositeOutputSHA256: source.CompositeOutputSHA256, ClaimCeiling: source.ClaimCeiling, Authority: Authority, SourceGraphComplete: SourceGraphComplete, PairWeights: make(map[TypedPair]int), Occurrences: make([]TypedOccurrence, 0, len(calls))}
	seen := make(map[string]bool, len(calls))
	for _, call := range calls {
		if call.Identity == "" || call.RelationID == "" || seen[call.Identity] || call.Ordinal < 0 || call.From < 0 || call.To < 0 || int(call.From) >= len(ids) || int(call.To) >= len(ids) || call.Weight != 1 {
			return TypedInput{}, errors.New("invalid admitted CALLS occurrence")
		}
		seen[call.Identity] = true
		from, to := ids[call.From], ids[call.To]
		out.Occurrences = append(out.Occurrences, TypedOccurrence{Identity: call.Identity, RelationID: call.RelationID, From: from, To: to, Kind: TypedCalls, Ordinal: call.Ordinal})
		out.PairWeights[TypedPair{From: from, To: to}]++
	}
	sort.Slice(out.Occurrences, func(i, j int) bool { return out.Occurrences[i].Identity < out.Occurrences[j].Identity })
	return out, nil
}
