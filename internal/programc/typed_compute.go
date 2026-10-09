package programc

import (
	"fmt"

	"lsp-trace/internal/programcadmission"
)

type typedSelection uint8

const (
	callsOnly typedSelection = iota + 1
	methodsOnly
	mixed
)

const (
	codeTypedSelectionMismatch FailureCode = "TYPED_SELECTION_MISMATCH"
	codeTypedSelectionUnknown  FailureCode = "TYPED_SELECTION_UNKNOWN"
	codeTypedInputIntegrity    FailureCode = "TYPED_INPUT_INTEGRITY"
)

type typedProjectionInput interface {
	Nodes() []string
	Occurrences() []programcadmission.TypedOccurrence
	PairWeights() map[programcadmission.TypedPair]int
	SelectedKinds() []programcadmission.RelationKind
	CompositeSourceBinding() programcadmission.CompositeSourceBinding
	CombinedClaimCeiling() string
	RetainedCallsAdmission() programcadmission.CompositeProjectionAdmission
}

func computeTypedProjection(in typedProjectionInput, selection typedSelection, seed uint64) (Outcome, *Failure) {
	if selection != callsOnly && selection != methodsOnly && selection != mixed {
		return Outcome{}, &Failure{Code: codeTypedSelectionUnknown, Message: "unknown typed selection"}
	}
	kinds := in.SelectedKinds()
	hasCalls, hasMethods := false, false
	for _, kind := range kinds {
		switch kind {
		case programcadmission.TypedCalls:
			hasCalls = true
		case programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition:
			hasMethods = true
		default:
			return Outcome{}, &Failure{Code: codeTypedInputIntegrity, Message: "invalid private typed projection integrity"}
		}
	}
	matches := selection == callsOnly && hasCalls && !hasMethods || selection == methodsOnly && !hasCalls && hasMethods || selection == mixed && hasCalls && hasMethods
	if !matches {
		return Outcome{}, &Failure{Code: codeTypedSelectionMismatch, Message: "typed selection does not match admitted relation kinds"}
	}
	if selection == callsOnly {
		admission := in.RetainedCallsAdmission()
		if !admission.Valid() {
			return Outcome{}, &Failure{Code: codeTypedInputIntegrity, Message: "invalid private typed projection integrity"}
		}
		return ComputeComposite(admission, seed)
	}

	occurrences := append([]programcadmission.TypedOccurrence(nil), in.Occurrences()...)
	if len(occurrences) > MaxOccurrences {
		return Outcome{}, &Failure{Code: CodeResourceLimit, Message: fmt.Sprintf("occurrence cap exceeded: %d > %d", len(occurrences), MaxOccurrences)}
	}
	nodes := in.Nodes()
	p := Projection{NodeIdentities: append([]string(nil), nodes...), NodeIDs: make(map[string]int64, len(nodes)), PairWeights: make(map[Pair]float64)}
	for i, identity := range nodes {
		if identity == "" || (i > 0 && nodes[i-1] >= identity) {
			return Outcome{}, &Failure{Code: codeTypedInputIntegrity, Message: "invalid private typed projection integrity"}
		}
		p.NodeIDs[identity] = int64(i)
	}
	for _, occurrence := range occurrences {
		from, fromOK := p.NodeIDs[occurrence.From]
		to, toOK := p.NodeIDs[occurrence.To]
		if !fromOK || !toOK || occurrence.Identity == "" {
			return Outcome{}, &Failure{Code: codeTypedInputIntegrity, Message: "invalid private typed projection integrity"}
		}
		p.Occurrences = append(p.Occurrences, Occurrence{Identity: occurrence.Identity, RelationID: occurrence.RelationID, From: from, To: to, Weight: 1})
	}
	for pair, weight := range in.PairWeights() {
		from, fromOK := p.NodeIDs[pair.From]
		to, toOK := p.NodeIDs[pair.To]
		if !fromOK || !toOK || weight <= 0 {
			return Outcome{}, &Failure{Code: codeTypedInputIntegrity, Message: "invalid private typed projection integrity"}
		}
		p.PairWeights[Pair{From: from, To: to}] = float64(weight)
	}
	compositeSource := programcadmission.CompositeSourceBinding{}
	if selection == mixed {
		compositeSource = in.CompositeSourceBinding()
		p.CompositeSource = programcadmission.CloneCompositeSourceBinding(compositeSource)
	}
	out, failure := computeProjection(p, seed, SourceBinding{}, compositeSource, in.CombinedClaimCeiling())
	if failure == nil && selection == methodsOnly {
		out.CompositeSource = programcadmission.CompositeSourceBinding{}
		out.Projection.CompositeSource = programcadmission.CompositeSourceBinding{}
	}
	return out, failure
}
