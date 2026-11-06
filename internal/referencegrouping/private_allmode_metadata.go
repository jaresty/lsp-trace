package referencegrouping

import "errors"

// AllModeMetadataPrivate reads bounded opaque metadata, never Artifact(), payload
// parsing, replay, source binding, or qualification. EligibleKinds is a copy.
type AllModeMetadataPrivate struct {
	Input                                CompositionCommitment
	EligibleKinds                        []string
	Contributions, Custody, Predecessors int
}

func ReadAllModeMetadataPrivate(in AdmittedInput) (AllModeMetadataPrivate, error) {
	a := in.artifact
	d := a.doc
	if len(a.canonical) == 0 || a.digest == "" || len(a.canonical) > EvaluationMaxBytesPrivate || len(d.EligibleKinds) == 0 || len(d.EligibleKinds) > 3 || len(d.Contributions) > EvaluationMaxOccurrencesPrivate || len(d.EndpointCustody) > 2*EvaluationMaxOccurrencesPrivate || len(d.Predecessors) > EvaluationMaxOccurrencesPrivate {
		return AllModeMetadataPrivate{}, errors.New("private allmode opaque input bound")
	}
	for _, k := range d.EligibleKinds {
		if k != "CALLS" && k != "REFERENCES_SYMBOL" && k != "RESOLVES_TO_DEFINITION" {
			return AllModeMetadataPrivate{}, errors.New("private allmode kind")
		}
	}
	return AllModeMetadataPrivate{CompositionCommitment{a.digest, a.digest, len(a.canonical)}, append([]string(nil), d.EligibleKinds...), len(d.Contributions), len(d.EndpointCustody), len(d.Predecessors)}, nil
}

// ReadAllModePolicyMetadataPrivate exposes only the three bounded rule kinds
// not explicitly excluded. Input EligibleKinds is the fixed admission universe,
// not a mode declaration; empty projections must retain the policy distinction.
func ReadAllModePolicyMetadataPrivate(p GroupingPolicy) (CompositionCommitment, []string, error) {
	if len(p.canonical) == 0 || len(p.canonical) > EvaluationMaxBytesPrivate || len(p.doc.Rules) != 3 || len(p.doc.EligibleKinds) != 3 {
		return CompositionCommitment{}, nil, errors.New("private allmode policy bound")
	}
	kinds := make([]string, 0, 3)
	for _, r := range p.doc.Rules {
		if r.Action != "EXCLUDE" {
			kinds = append(kinds, r.RelationKind)
		}
	}
	return CompositionCommitment{p.ID(), p.Digest(), p.ByteLength()}, kinds, nil
}

// ProjectionCountsPrivate permits a consumer to charge/check aggregate bounds
// before requesting defensive copies of the projection collections.
func ProjectionCountsPrivate(p EvaluationProjectionPrivate) (nodes, occurrences, custody int, err error) {
	if !p.Valid() || len(p.canonical) > EvaluationMaxBytesPrivate || len(p.doc.Nodes) > EvaluationMaxNodesPrivate || len(p.doc.Occurrences) > EvaluationMaxOccurrencesPrivate || len(p.doc.Custody) > 2*EvaluationMaxOccurrencesPrivate {
		return 0, 0, 0, errors.New("private allmode projection bound")
	}
	return len(p.doc.Nodes), len(p.doc.Occurrences), len(p.doc.Custody), nil
}
