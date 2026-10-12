package programcadmission

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"reflect"
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
	seal                       [sha256.Size]byte
	nodes                      []string
	selectedKinds              []RelationKind
	methodsEnabled             bool
	methodClaim                string
	methodAuthority            int
	methodAccepted             bool
	methodComplete             string
	callsAdmission             CompositeProjectionAdmission
	methodEvidence             []MethodOccurrenceEvidence
	methodAdmissionOccurrences []MethodOccurrenceEvidence
	methodCanonicalBytes       []byte
	methodCommitmentDigest     [sha256.Size]byte
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
	if needMethods {
		if len(req.Methods.occurrences) == 0 {
			return TypedInput{}, ErrMethodEvidenceUnadmitted
		}
		if err := validateMethodProjectionAdmission(req.Methods); err != nil {
			return TypedInput{}, err
		}
	}

	out := TypedInput{Version: TypedInputVersion, Authority: Authority, SourceGraphComplete: SourceGraphComplete, PairWeights: make(map[TypedPair]int)}
	seen := make(map[string]bool)
	if selected[TypedCalls] {
		if err := appendCalls(&out, req.Calls, seen); err != nil {
			return TypedInput{}, err
		}
		out.integrity.callsAdmission = cloneCompositeProjectionAdmission(req.Calls)
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
			out.integrity.methodEvidence = append(out.integrity.methodEvidence, occurrence)
			out.PairWeights[TypedPair{From: occurrence.From, To: occurrence.To}]++
		}
		sort.Slice(out.integrity.methodEvidence, func(i, j int) bool {
			return out.integrity.methodEvidence[i].Identity < out.integrity.methodEvidence[j].Identity
		})
		out.integrity.methodAdmissionOccurrences = req.Methods.Occurrences()
		out.integrity.methodCanonicalBytes = req.Methods.CanonicalBytes()
		copy(out.integrity.methodCommitmentDigest[:], req.Methods.CommitmentDigest())
	}
	sort.Slice(out.Occurrences, func(i, j int) bool { return out.Occurrences[i].Identity < out.Occurrences[j].Identity })
	nodes := make(map[string]bool, len(out.integrity.nodes))
	for _, id := range out.integrity.nodes {
		nodes[id] = true
	}
	for _, occurrence := range out.Occurrences {
		nodes[occurrence.From], nodes[occurrence.To] = true, true
	}
	out.integrity.nodes = out.integrity.nodes[:0]
	for id := range nodes {
		out.integrity.nodes = append(out.integrity.nodes, id)
	}
	sort.Strings(out.integrity.nodes)
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
	for i, call := range calls {
		if call.Identity == "" || call.RelationID == "" || seen[call.Identity] || (i > 0 && calls[i-1].Identity >= call.Identity) || call.Ordinal < 0 || call.From < 0 || call.To < 0 || int(call.From) >= len(ids) || int(call.To) >= len(ids) || call.Weight != 1 || call.Provider != source.Compatibility.ServerCommand || call.ProviderVersion != source.Compatibility.ServerVersion || call.Language != source.Compatibility.LanguageID || call.Identity != occurrenceIdentity(source.CompositeID, call.RelationID, call.Ordinal, call.CallSite) {
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
	if isZeroSeal(in.integrity.seal) || subtle.ConstantTimeCompare(in.integrity.seal[:], expected[:]) != 1 || validateTypedInputStructure(in) != nil {
		return typedInputIntegrityError{}
	}
	return nil
}

func validateTypedInputStructure(in TypedInput) error {
	if in.Version != TypedInputVersion || in.Authority != Authority || in.SourceGraphComplete != SourceGraphComplete || len(in.integrity.selectedKinds) == 0 {
		return typedInputIntegrityError{}
	}
	selected := make(map[RelationKind]bool, len(in.integrity.selectedKinds))
	for i, kind := range in.integrity.selectedKinds {
		if (i > 0 && in.integrity.selectedKinds[i-1] >= kind) || (kind != TypedCalls && kind != TypedReferencesSymbol && kind != TypedResolvesToDefinition) {
			return typedInputIntegrityError{}
		}
		selected[kind] = true
	}
	if (selected[TypedReferencesSymbol] || selected[TypedResolvesToDefinition]) != in.integrity.methodsEnabled {
		return typedInputIntegrityError{}
	}
	if in.integrity.methodsEnabled {
		if in.integrity.methodClaim != MethodEvidenceClaimCeiling || in.integrity.methodAuthority != 0 || in.integrity.methodAccepted || in.integrity.methodComplete != "UNKNOWN" {
			return typedInputIntegrityError{}
		}
		admission := MethodProjectionAdmission{occurrences: append([]MethodOccurrenceEvidence(nil), in.integrity.methodAdmissionOccurrences...), canonicalBytes: append([]byte(nil), in.integrity.methodCanonicalBytes...), commitmentDigest: in.integrity.methodCommitmentDigest}
		if validateMethodProjectionAdmission(admission) != nil {
			return typedInputIntegrityError{}
		}
	} else if len(in.integrity.methodEvidence) != 0 || len(in.integrity.methodAdmissionOccurrences) != 0 || len(in.integrity.methodCanonicalBytes) != 0 || !isZeroMethodCommitment(in.integrity.methodCommitmentDigest) {
		return typedInputIntegrityError{}
	}
	nodes := make(map[string]bool)
	var retainedCalls []TypedOccurrence
	if selected[TypedCalls] {
		retained := in.integrity.callsAdmission
		if len(retained.canonicalBytes) == 0 {
			return typedInputIntegrityError{}
		}
		readmitted, err := Admit(retained.canonicalBytes)
		if err != nil || !reflect.DeepEqual(retained, readmitted.Admission) {
			return typedInputIntegrityError{}
		}
		expectedCalls, err := PrepareTypedInput(TypedInputRequest{Calls: readmitted.Admission, Kinds: []RelationKind{TypedCalls}})
		if err != nil || in.CompositeID != expectedCalls.CompositeID || in.CompositeOutputSHA256 != expectedCalls.CompositeOutputSHA256 || in.ClaimCeiling != expectedCalls.ClaimCeiling {
			return typedInputIntegrityError{}
		}
		retainedCalls = expectedCalls.Occurrences
		for _, id := range expectedCalls.integrity.nodes {
			nodes[id] = true
		}
	} else if in.CompositeID != "" || in.CompositeOutputSHA256 != "" || in.ClaimCeiling != "" || in.integrity.callsAdmission.Valid() {
		return typedInputIntegrityError{}
	}
	seen := make(map[string]bool, len(in.Occurrences))
	pairs := make(map[TypedPair]int)
	observedCalls, observedMethods := 0, 0
	for i, occurrence := range in.Occurrences {
		if occurrence.Identity == "" || occurrence.From == "" || occurrence.To == "" || seen[occurrence.Identity] || !selected[occurrence.Kind] || occurrence.Ordinal < 0 || (i > 0 && in.Occurrences[i-1].Identity >= occurrence.Identity) {
			return typedInputIntegrityError{}
		}
		if occurrence.Kind == TypedCalls && occurrence.RelationID == "" || occurrence.Kind != TypedCalls && occurrence.RelationID != "" {
			return typedInputIntegrityError{}
		}
		if occurrence.Kind == TypedCalls {
			if observedCalls >= len(retainedCalls) || occurrence != retainedCalls[observedCalls] {
				return typedInputIntegrityError{}
			}
			observedCalls++
		} else {
			if observedMethods >= len(in.integrity.methodEvidence) {
				return typedInputIntegrityError{}
			}
			method := in.integrity.methodEvidence[observedMethods]
			if method.Identity != occurrence.Identity || method.Kind != occurrence.Kind || method.From != occurrence.From || method.To != occurrence.To || method.Ordinal != occurrence.Ordinal {
				return typedInputIntegrityError{}
			}
			observedMethods++
		}
		seen[occurrence.Identity], nodes[occurrence.From], nodes[occurrence.To] = true, true, true
		pairs[TypedPair{From: occurrence.From, To: occurrence.To}]++
	}
	if observedCalls != len(retainedCalls) || observedMethods != len(in.integrity.methodEvidence) {
		return typedInputIntegrityError{}
	}
	canonicalNodes := make([]string, 0, len(nodes))
	for id := range nodes {
		canonicalNodes = append(canonicalNodes, id)
	}
	sort.Strings(canonicalNodes)
	if !equalStrings(canonicalNodes, in.integrity.nodes) || len(pairs) != len(in.PairWeights) {
		return typedInputIntegrityError{}
	}
	for pair, weight := range pairs {
		if weight <= 0 || in.PairWeights[pair] != weight {
			return typedInputIntegrityError{}
		}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cloneCompositeProjectionAdmission(in CompositeProjectionAdmission) CompositeProjectionAdmission {
	return CompositeProjectionAdmission{nodeIDs: in.NodeIdentities(), occurrences: in.Occurrences(), claimCeiling: in.claimCeiling, source: in.SourceBinding(), canonicalBytes: cloneBytes(in.canonicalBytes), valid: in.valid}
}

func (in TypedInput) retainedCallsAdmission() CompositeProjectionAdmission {
	return cloneCompositeProjectionAdmission(in.integrity.callsAdmission)
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
	writeSealBytes(h, in.integrity.methodCanonicalBytes)
	writeSealBytes(h, in.integrity.methodCommitmentDigest[:])
	writeSealUint64(h, uint64(len(in.integrity.methodAdmissionOccurrences)))
	for _, occurrence := range in.integrity.methodAdmissionOccurrences {
		writeMethodEvidenceSeal(h, occurrence)
	}
	writeSealUint64(h, uint64(len(in.integrity.methodEvidence)))
	for _, occurrence := range in.integrity.methodEvidence {
		writeMethodEvidenceSeal(h, occurrence)
	}
	calls := in.integrity.callsAdmission
	writeSealBool(h, calls.valid)
	writeSealStrings(h, calls.nodeIDs)
	writeSealString(h, calls.claimCeiling)
	writeSealUint64(h, uint64(len(calls.occurrences)))
	for _, occurrence := range calls.occurrences {
		writeSealString(h, occurrence.Identity)
		writeSealString(h, occurrence.RelationID)
		writeSealString(h, occurrence.Provider)
		writeSealString(h, occurrence.ProviderVersion)
		writeSealString(h, occurrence.Language)
		writeSealInt(h, int(occurrence.From))
		writeSealInt(h, int(occurrence.To))
		writeSealInt(h, occurrence.Ordinal)
		writeSealString(h, fmt.Sprint(occurrence.CallSite))
		writeSealString(h, fmt.Sprint(occurrence.Weight))
	}
	writeSealString(h, fmt.Sprintf("%#v", calls.source))
	writeSealBytes(h, calls.canonicalBytes)
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

func writeMethodEvidenceSeal(h hash.Hash, occurrence MethodOccurrenceEvidence) {
	writeSealString(h, occurrence.Identity)
	writeSealString(h, occurrence.From)
	writeSealString(h, occurrence.To)
	writeSealString(h, string(occurrence.Kind))
	writeSealInt(h, occurrence.Ordinal)
	writeSealString(h, occurrence.SourceRole)
	writeSealString(h, occurrence.TargetRole)
	writeSealString(h, occurrence.Method)
	writeSealString(h, occurrence.QuerySelector)
	writeSealString(h, occurrence.QueryDigest)
	writeSealInt(h, occurrence.QueryLength)
	writeSealString(h, occurrence.EvidenceSelector)
	writeSealString(h, occurrence.EvidenceDigest)
	writeSealInt(h, occurrence.EvidenceLength)
	writeSealString(h, occurrence.Provider)
	writeSealString(h, occurrence.ProviderVersion)
	writeSealString(h, occurrence.SessionID)
	writeSealUint64(h, occurrence.Generation)
	writeSealString(h, occurrence.Revision)
	writeSealString(h, occurrence.Custody)
	writeSealString(h, occurrence.PositionEncoding)
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

func writeSealBytes(h hash.Hash, value []byte) {
	writeSealBool(h, value != nil)
	writeSealUint64(h, uint64(len(value)))
	_, _ = h.Write(value)
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
