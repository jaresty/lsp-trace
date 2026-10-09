package programcadmission

const privateTypedClaimCeiling = "PRIVATE_PROGRAM_C_TYPED_RELATION_QUALIFICATION;STRUCTURAL_PARTITION_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;FEATURE_IDENTITY_UNRESOLVED;NO_PRODUCER_AUTHENTICATION;NO_EXECUTION_CLAIM"

// privateLeidenProjection is an opaque, validated internal bridge. Its
// accessors always return defensive copies.
type privateLeidenProjection struct {
	nodes            []string
	occurrences      []TypedOccurrence
	pairWeights      map[TypedPair]int
	selectedKinds    []RelationKind
	compositeSource  CompositeSourceBinding
	calls            CompositeProjectionAdmission
	methodClaim      string
	methodAuthority  int
	methodAccepted   bool
	methodComplete   string
	methodEvidence   []MethodOccurrenceEvidence
	methodCommitment []byte
	claimCeiling     string
}

func PreparePrivateLeidenProjection(in TypedInput) (privateLeidenProjection, error) {
	if err := validateTypedInputIntegrity(in); err != nil {
		return privateLeidenProjection{}, err
	}
	out := privateLeidenProjection{
		nodes:            append([]string(nil), in.integrity.nodes...),
		occurrences:      append([]TypedOccurrence(nil), in.Occurrences...),
		pairWeights:      make(map[TypedPair]int, len(in.PairWeights)),
		selectedKinds:    append([]RelationKind(nil), in.integrity.selectedKinds...),
		calls:            cloneCompositeProjectionAdmission(in.integrity.callsAdmission),
		methodClaim:      in.integrity.methodClaim,
		methodAuthority:  in.integrity.methodAuthority,
		methodAccepted:   in.integrity.methodAccepted,
		methodComplete:   in.integrity.methodComplete,
		methodEvidence:   append([]MethodOccurrenceEvidence(nil), in.integrity.methodEvidence...),
		methodCommitment: append([]byte(nil), in.integrity.methodCommitmentDigest[:]...),
	}
	for pair, weight := range in.PairWeights {
		out.pairWeights[pair] = weight
	}
	if out.calls.Valid() {
		out.compositeSource = out.calls.SourceBinding()
	}
	if len(out.selectedKinds) == 1 && out.selectedKinds[0] == TypedCalls {
		out.claimCeiling = out.calls.ClaimCeiling()
	} else {
		out.claimCeiling = privateTypedClaimCeiling
	}
	return out, nil
}

func (p privateLeidenProjection) Nodes() []string { return append([]string(nil), p.nodes...) }
func (p privateLeidenProjection) Occurrences() []TypedOccurrence {
	return append([]TypedOccurrence(nil), p.occurrences...)
}
func (p privateLeidenProjection) PairWeights() map[TypedPair]int {
	out := make(map[TypedPair]int, len(p.pairWeights))
	for pair, weight := range p.pairWeights {
		out[pair] = weight
	}
	return out
}
func (p privateLeidenProjection) SelectedKinds() []RelationKind {
	return append([]RelationKind(nil), p.selectedKinds...)
}
func (p privateLeidenProjection) CompositeSourceBinding() CompositeSourceBinding {
	if !p.calls.Valid() {
		return CompositeSourceBinding{}
	}
	return CloneCompositeSourceBinding(p.compositeSource)
}
func (p privateLeidenProjection) MethodClaimCeiling() string { return p.methodClaim }
func (p privateLeidenProjection) MethodAuthority() int       { return p.methodAuthority }
func (p privateLeidenProjection) MethodAccepted() bool       { return p.methodAccepted }
func (p privateLeidenProjection) MethodCompleteness() string { return p.methodComplete }
func (p privateLeidenProjection) MethodEvidence() []MethodOccurrenceEvidence {
	return append([]MethodOccurrenceEvidence(nil), p.methodEvidence...)
}
func (p privateLeidenProjection) MethodCommitment() []byte {
	return append([]byte(nil), p.methodCommitment...)
}
func (p privateLeidenProjection) CombinedClaimCeiling() string { return p.claimCeiling }
func (p privateLeidenProjection) RetainedCallsAdmission() CompositeProjectionAdmission {
	return cloneCompositeProjectionAdmission(p.calls)
}
func (p privateLeidenProjection) retainedCallsAdmission() CompositeProjectionAdmission {
	return p.RetainedCallsAdmission()
}
