package programcadmission

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"hash"
	"sort"
)

const TypedInputVersion = "lsp-trace.private.program-c-typed-staging.v0"

type RelationKind string

const (
	TypedCalls                RelationKind = "CALLS"
	TypedReferencesSymbol     RelationKind = "REFERENCES_SYMBOL"
	TypedResolvesToDefinition RelationKind = "RESOLVES_TO_DEFINITION"
)

type MethodCandidate struct {
	Identity string
	Kind     RelationKind
	From, To string
}

type TypedInputRequest struct {
	Calls            CompositeProjectionAdmission
	Methods          MethodProjectionAdmission
	EnableMethods    bool
	Kinds            []RelationKind
	MethodCandidates []MethodCandidate
}

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
	integrity                                                 typedInputIntegrity
}

type typedInputIntegrity struct {
	seal            [sha256.Size]byte
	nodes           []string
	selectedKinds   []RelationKind
	methodsEnabled  bool
	methodClaim     string
	methodAuthority int
	methodAccepted  bool
	methodComplete  string
}

type typedInputIntegrityError struct{}

func (typedInputIntegrityError) Error() string { return "invalid private typed input integrity" }

var ErrMethodEvidenceUnadmitted = errors.New("definition/reference evidence has no admitted immutable occurrence binding")
var ErrTypedSelection = errors.New("unsupported private typed input selection")

func PrepareTypedInput(req TypedInputRequest) (TypedInput, error) {
	if len(req.MethodCandidates) != 0 {
		return TypedInput{}, ErrMethodEvidenceUnadmitted
	}
	selected := make(map[RelationKind]bool, len(req.Kinds))
	for _, kind := range req.Kinds {
		if selected[kind] {
			if req.EnableMethods {
				return TypedInput{}, ErrMethodEvidenceInvalidSelection
			}
			return TypedInput{}, ErrTypedSelection
		}
		selected[kind] = true
		if kind != TypedCalls && (!req.EnableMethods || (kind != TypedReferencesSymbol && kind != TypedResolvesToDefinition)) {
			if req.EnableMethods {
				return TypedInput{}, ErrMethodEvidenceInvalidSelection
			}
			return TypedInput{}, ErrTypedSelection
		}
	}
	if len(selected) == 0 {
		return TypedInput{}, ErrTypedSelection
	}
	if !req.EnableMethods && (len(selected) != 1 || !selected[TypedCalls]) {
		return TypedInput{}, ErrTypedSelection
	}
	needMethods := selected[TypedReferencesSymbol] || selected[TypedResolvesToDefinition]
	if needMethods && len(req.Methods.occurrences) == 0 {
		return TypedInput{}, ErrMethodEvidenceUnadmitted
	}

	out := TypedInput{Version: TypedInputVersion, Authority: Authority, SourceGraphComplete: SourceGraphComplete, PairWeights: make(map[TypedPair]int)}
	seen := make(map[string]bool)
	if selected[TypedCalls] {
		if err := appendCalls(&out, req.Calls, seen); err != nil {
			return TypedInput{}, err
		}
	}
	if needMethods {
		for _, occurrence := range req.Methods.Occurrences() {
			if !selected[occurrence.Kind] {
				continue
			}
			if seen[occurrence.Identity] {
				return TypedInput{}, ErrMethodEvidenceDuplicateIdentity
			}
			seen[occurrence.Identity] = true
			out.Occurrences = append(out.Occurrences, TypedOccurrence{Identity: occurrence.Identity, From: occurrence.From, To: occurrence.To, Kind: occurrence.Kind, Ordinal: occurrence.Ordinal})
			out.PairWeights[TypedPair{From: occurrence.From, To: occurrence.To}]++
		}
	}
	sort.Slice(out.Occurrences, func(i, j int) bool { return out.Occurrences[i].Identity < out.Occurrences[j].Identity })
	out.integrity.selectedKinds = append([]RelationKind(nil), req.Kinds...)
	sort.Slice(out.integrity.selectedKinds, func(i, j int) bool { return out.integrity.selectedKinds[i] < out.integrity.selectedKinds[j] })
	out.integrity.methodsEnabled = req.EnableMethods
	if needMethods {
		out.integrity.methodClaim = req.Methods.ClaimCeiling()
		out.integrity.methodAuthority = req.Methods.Authority()
		out.integrity.methodAccepted = req.Methods.Accepted()
		out.integrity.methodComplete = req.Methods.Completeness()
	}
	out.integrity.seal = typedInputSeal(out)
	return out, nil
}

func appendCalls(out *TypedInput, admission CompositeProjectionAdmission, seen map[string]bool) error {
	if !admission.Valid() || admission.ClaimCeiling() == "" {
		return errors.New("validated CALLS composite admission required")
	}
	source := admission.SourceBinding()
	if source.CompositeID == "" || source.CompositeOutputSHA256 == "" || source.ClaimCeiling != admission.ClaimCeiling() || source.Authority != 0 || source.SourceGraphComplete != SourceGraphComplete || source.Completeness.WholeWorkspace {
		return errors.New("invalid CALLS composite source binding")
	}
	ids := admission.NodeIdentities()
	if len(ids) > maxNodes {
		return errors.New("typed node cap exceeded")
	}
	for i, id := range ids {
		if id == "" || (i > 0 && ids[i-1] >= id) {
			return errors.New("noncanonical admitted nodes")
		}
	}
	calls := admission.Occurrences()
	if len(calls) > maxOccurrences {
		return errors.New("typed occurrence cap exceeded")
	}
	out.CompositeID, out.CompositeOutputSHA256, out.ClaimCeiling = source.CompositeID, source.CompositeOutputSHA256, source.ClaimCeiling
	out.integrity.nodes = append([]string(nil), ids...)
	out.Occurrences = make([]TypedOccurrence, 0, len(calls))
	for _, call := range calls {
		if call.Identity == "" || call.RelationID == "" || seen[call.Identity] || call.Ordinal < 0 || call.From < 0 || call.To < 0 || int(call.From) >= len(ids) || int(call.To) >= len(ids) || call.Weight != 1 {
			return errors.New("invalid admitted CALLS occurrence")
		}
		seen[call.Identity] = true
		from, to := ids[call.From], ids[call.To]
		out.Occurrences = append(out.Occurrences, TypedOccurrence{Identity: call.Identity, RelationID: call.RelationID, From: from, To: to, Kind: TypedCalls, Ordinal: call.Ordinal})
		out.PairWeights[TypedPair{From: from, To: to}]++
	}
	return nil
}

func validateTypedInputIntegrity(in TypedInput) error {
	expected := typedInputSeal(in)
	if isZeroSeal(in.integrity.seal) || subtle.ConstantTimeCompare(in.integrity.seal[:], expected[:]) != 1 {
		return typedInputIntegrityError{}
	}
	return nil
}

func DirectedWalkCandidates(in TypedInput) ([]MethodCandidate, error) {
	if err := validateTypedInputIntegrity(in); err != nil {
		return nil, err
	}
	out := make([]MethodCandidate, len(in.Occurrences))
	for i, occurrence := range in.Occurrences {
		if occurrence.Identity == "" || occurrence.From == "" || occurrence.To == "" || (occurrence.Kind != TypedCalls && occurrence.Kind != TypedReferencesSymbol && occurrence.Kind != TypedResolvesToDefinition) {
			return nil, errors.New("invalid typed occurrence")
		}
		out[i] = MethodCandidate{Identity: occurrence.Identity, Kind: occurrence.Kind, From: occurrence.From, To: occurrence.To}
	}
	return out, nil
}

func isZeroSeal(seal [sha256.Size]byte) bool {
	var zero [sha256.Size]byte
	return subtle.ConstantTimeCompare(seal[:], zero[:]) == 1
}

func typedInputSeal(in TypedInput) [sha256.Size]byte {
	h := sha256.New()
	writeSealString(h, in.Version)
	writeSealString(h, in.CompositeID)
	writeSealString(h, in.CompositeOutputSHA256)
	writeSealString(h, in.ClaimCeiling)
	writeSealInt(h, in.Authority)
	writeSealString(h, in.SourceGraphComplete)
	writeSealStrings(h, in.integrity.nodes)
	writeSealBool(h, in.integrity.methodsEnabled)
	writeSealString(h, in.integrity.methodClaim)
	writeSealInt(h, in.integrity.methodAuthority)
	writeSealBool(h, in.integrity.methodAccepted)
	writeSealString(h, in.integrity.methodComplete)
	writeSealUint64(h, uint64(len(in.integrity.selectedKinds)))
	for _, kind := range in.integrity.selectedKinds {
		writeSealString(h, string(kind))
	}
	writeSealUint64(h, uint64(len(in.Occurrences)))
	for _, occurrence := range in.Occurrences {
		writeSealString(h, occurrence.Identity)
		writeSealString(h, occurrence.RelationID)
		writeSealString(h, occurrence.From)
		writeSealString(h, occurrence.To)
		writeSealString(h, string(occurrence.Kind))
		writeSealInt(h, occurrence.Ordinal)
	}
	pairs := make([]TypedPair, 0, len(in.PairWeights))
	for pair := range in.PairWeights {
		pairs = append(pairs, pair)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].From != pairs[j].From {
			return pairs[i].From < pairs[j].From
		}
		return pairs[i].To < pairs[j].To
	})
	writeSealUint64(h, uint64(len(pairs)))
	for _, pair := range pairs {
		writeSealString(h, pair.From)
		writeSealString(h, pair.To)
		writeSealInt(h, in.PairWeights[pair])
	}
	var seal [sha256.Size]byte
	copy(seal[:], h.Sum(nil))
	return seal
}

func writeSealStrings(h hash.Hash, values []string) {
	writeSealUint64(h, uint64(len(values)))
	for _, value := range values {
		writeSealString(h, value)
	}
}

func writeSealString(h hash.Hash, value string) {
	writeSealUint64(h, uint64(len(value)))
	_, _ = h.Write([]byte(value))
}

func writeSealInt(h hash.Hash, value int) { writeSealUint64(h, uint64(int64(value))) }

func writeSealBool(h hash.Hash, value bool) {
	if value {
		_, _ = h.Write([]byte{1})
		return
	}
	_, _ = h.Write([]byte{0})
}

func writeSealUint64(h hash.Hash, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = h.Write(encoded[:])
}
