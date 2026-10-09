package programc

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"hash"
	"reflect"
	"sort"

	"lsp-trace/internal/programcadmission"
)

// TypedComputation is an internal opaque carrier for a validated typed
// computation and the exact source state required to reproduce it.
type TypedComputation struct {
	cachedOutcome    Outcome
	source           programcadmission.TypedInput
	seed             uint64
	occurrences      []programcadmission.TypedOccurrence
	methodEvidence   []programcadmission.MethodOccurrenceEvidence
	selectedKinds    []programcadmission.RelationKind
	pairWeights      map[programcadmission.TypedPair]int
	methodCommitment []byte
	seal             [sha256.Size]byte
}

func ComputeTyped(in programcadmission.TypedInput, seed uint64) (TypedComputation, *Failure) {
	projection, err := programcadmission.PreparePrivateLeidenProjection(in)
	if err != nil {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	selection, ok := inferTypedSelection(projection.SelectedKinds())
	if !ok {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	outcome, failure := computeTypedProjection(projection, selection, seed)
	if failure != nil {
		return TypedComputation{}, failure
	}
	out := TypedComputation{
		cachedOutcome:    CloneOutcome(outcome),
		source:           cloneTypedInputPublic(in),
		seed:             seed,
		occurrences:      projection.Occurrences(),
		methodEvidence:   projection.MethodEvidence(),
		selectedKinds:    projection.SelectedKinds(),
		pairWeights:      projection.PairWeights(),
		methodCommitment: projection.MethodCommitment(),
	}
	if len(out.methodEvidence) == 0 {
		out.methodCommitment = nil
	}
	out.seal = typedComputationSeal(out)
	return out, nil
}

func RecomputeTyped(in TypedComputation) (TypedComputation, *Failure) {
	if !validTypedComputation(in) {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	projection, err := programcadmission.PreparePrivateLeidenProjection(in.source)
	if err != nil || !reflect.DeepEqual(projection.Occurrences(), in.occurrences) ||
		!reflect.DeepEqual(projection.MethodEvidence(), in.methodEvidence) ||
		!reflect.DeepEqual(projection.SelectedKinds(), in.selectedKinds) ||
		!reflect.DeepEqual(projection.PairWeights(), in.pairWeights) ||
		!reflect.DeepEqual(normalizeMethodCommitment(projection.MethodEvidence(), projection.MethodCommitment()), in.methodCommitment) {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	selection, ok := inferTypedSelection(in.selectedKinds)
	if !ok {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	outcome, failure := computeTypedProjection(projection, selection, in.seed)
	if failure != nil {
		return TypedComputation{}, failure
	}
	out := cloneTypedComputation(in)
	out.cachedOutcome = CloneOutcome(outcome)
	return out, nil
}

func (in TypedComputation) Outcome() Outcome { return CloneOutcome(in.cachedOutcome) }
func (in TypedComputation) Occurrences() []programcadmission.TypedOccurrence {
	return append([]programcadmission.TypedOccurrence(nil), in.occurrences...)
}
func (in TypedComputation) MethodEvidence() []programcadmission.MethodOccurrenceEvidence {
	return append([]programcadmission.MethodOccurrenceEvidence(nil), in.methodEvidence...)
}
func (in TypedComputation) SelectedKinds() []programcadmission.RelationKind {
	return append([]programcadmission.RelationKind(nil), in.selectedKinds...)
}
func (in TypedComputation) PairWeights() map[programcadmission.TypedPair]int {
	out := make(map[programcadmission.TypedPair]int, len(in.pairWeights))
	for pair, weight := range in.pairWeights {
		out[pair] = weight
	}
	return out
}
func (in TypedComputation) MethodCommitment() []byte {
	return append([]byte(nil), in.methodCommitment...)
}

func inferTypedSelection(kinds []programcadmission.RelationKind) (typedSelection, bool) {
	hasCalls, hasMethods := false, false
	for _, kind := range kinds {
		switch kind {
		case programcadmission.TypedCalls:
			hasCalls = true
		case programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition:
			hasMethods = true
		default:
			return 0, false
		}
	}
	switch {
	case hasCalls && hasMethods:
		return mixed, true
	case hasCalls:
		return callsOnly, true
	case hasMethods:
		return methodsOnly, true
	default:
		return 0, false
	}
}

func validTypedComputation(in TypedComputation) bool {
	if isZeroTypedComputationSeal(in.seal) {
		return false
	}
	want := typedComputationSeal(in)
	return subtle.ConstantTimeCompare(in.seal[:], want[:]) == 1
}

func typedComputationSeal(in TypedComputation) [sha256.Size]byte {
	h := sha256.New()
	writeTypedUint64(h, in.seed)
	writeTypedString(h, in.source.Version)
	writeTypedString(h, in.source.CompositeID)
	writeTypedString(h, in.source.CompositeOutputSHA256)
	writeTypedString(h, in.source.ClaimCeiling)
	writeTypedInt(h, in.source.Authority)
	writeTypedString(h, in.source.SourceGraphComplete)
	writeTypedOccurrences(h, in.source.Occurrences)
	writeTypedPairs(h, in.source.PairWeights)
	writeTypedOccurrences(h, in.occurrences)
	writeTypedUint64(h, uint64(len(in.methodEvidence)))
	for _, occurrence := range in.methodEvidence {
		writeTypedMethodEvidence(h, occurrence)
	}
	writeTypedUint64(h, uint64(len(in.selectedKinds)))
	for _, kind := range in.selectedKinds {
		writeTypedString(h, string(kind))
	}
	writeTypedPairs(h, in.pairWeights)
	writeTypedBytes(h, in.methodCommitment)
	var seal [sha256.Size]byte
	copy(seal[:], h.Sum(nil))
	return seal
}

func cloneTypedComputation(in TypedComputation) TypedComputation {
	out := in
	out.cachedOutcome = CloneOutcome(in.cachedOutcome)
	out.source = cloneTypedInputPublic(in.source)
	out.occurrences = append([]programcadmission.TypedOccurrence(nil), in.occurrences...)
	out.methodEvidence = append([]programcadmission.MethodOccurrenceEvidence(nil), in.methodEvidence...)
	out.selectedKinds = append([]programcadmission.RelationKind(nil), in.selectedKinds...)
	out.pairWeights = cloneTypedPairs(in.pairWeights)
	out.methodCommitment = append([]byte(nil), in.methodCommitment...)
	return out
}

func cloneTypedInputPublic(in programcadmission.TypedInput) programcadmission.TypedInput {
	out := in
	out.Occurrences = append([]programcadmission.TypedOccurrence(nil), in.Occurrences...)
	out.PairWeights = cloneTypedPairs(in.PairWeights)
	return out
}

func cloneTypedPairs(in map[programcadmission.TypedPair]int) map[programcadmission.TypedPair]int {
	out := make(map[programcadmission.TypedPair]int, len(in))
	for pair, weight := range in {
		out[pair] = weight
	}
	return out
}

func normalizeMethodCommitment(methods []programcadmission.MethodOccurrenceEvidence, commitment []byte) []byte {
	if len(methods) == 0 {
		return nil
	}
	return commitment
}

func typedComputationIntegrityFailure() *Failure {
	return &Failure{Code: codeTypedInputIntegrity, Message: "TYPED_INPUT_INTEGRITY"}
}

func isZeroTypedComputationSeal(in [sha256.Size]byte) bool {
	var zero [sha256.Size]byte
	return subtle.ConstantTimeCompare(in[:], zero[:]) == 1
}

func writeTypedOccurrences(h hash.Hash, occurrences []programcadmission.TypedOccurrence) {
	writeTypedUint64(h, uint64(len(occurrences)))
	for _, occurrence := range occurrences {
		writeTypedString(h, occurrence.Identity)
		writeTypedString(h, occurrence.RelationID)
		writeTypedString(h, occurrence.From)
		writeTypedString(h, occurrence.To)
		writeTypedString(h, string(occurrence.Kind))
		writeTypedInt(h, occurrence.Ordinal)
	}
}

func writeTypedPairs(h hash.Hash, pairs map[programcadmission.TypedPair]int) {
	keys := make([]programcadmission.TypedPair, 0, len(pairs))
	for pair := range pairs {
		keys = append(keys, pair)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].From != keys[j].From {
			return keys[i].From < keys[j].From
		}
		return keys[i].To < keys[j].To
	})
	writeTypedUint64(h, uint64(len(keys)))
	for _, pair := range keys {
		writeTypedString(h, pair.From)
		writeTypedString(h, pair.To)
		writeTypedInt(h, pairs[pair])
	}
}

func writeTypedMethodEvidence(h hash.Hash, occurrence programcadmission.MethodOccurrenceEvidence) {
	writeTypedString(h, occurrence.Identity)
	writeTypedString(h, occurrence.From)
	writeTypedString(h, occurrence.To)
	writeTypedString(h, string(occurrence.Kind))
	writeTypedInt(h, occurrence.Ordinal)
	writeTypedString(h, occurrence.SourceRole)
	writeTypedString(h, occurrence.TargetRole)
	writeTypedString(h, occurrence.Method)
	writeTypedString(h, occurrence.QuerySelector)
	writeTypedString(h, occurrence.QueryDigest)
	writeTypedInt(h, occurrence.QueryLength)
	writeTypedString(h, occurrence.EvidenceSelector)
	writeTypedString(h, occurrence.EvidenceDigest)
	writeTypedInt(h, occurrence.EvidenceLength)
	writeTypedString(h, occurrence.Provider)
	writeTypedString(h, occurrence.ProviderVersion)
	writeTypedString(h, occurrence.SessionID)
	writeTypedUint64(h, occurrence.Generation)
	writeTypedString(h, occurrence.Revision)
	writeTypedString(h, occurrence.Custody)
	writeTypedString(h, occurrence.PositionEncoding)
}

func writeTypedString(h hash.Hash, value string) {
	writeTypedUint64(h, uint64(len(value)))
	_, _ = h.Write([]byte(value))
}
func writeTypedBytes(h hash.Hash, value []byte) {
	writeTypedUint64(h, uint64(len(value)))
	_, _ = h.Write(value)
}
func writeTypedInt(h hash.Hash, value int) { writeTypedUint64(h, uint64(int64(value))) }
func writeTypedUint64(h hash.Hash, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = h.Write(encoded[:])
}
