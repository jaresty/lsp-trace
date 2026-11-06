package targetpacket

import (
	"errors"
	rg "lsp-trace/internal/referencegrouping"
)

// PrivateTypedEndpoint is an occurrence endpoint, NOT a source binding or a
// CALLS call site. Ordinals are local to the original typed input namespace.
type PrivateTypedEndpoint struct {
	Node, Kind, Occurrence, Role, Predecessor, Custody, Disposition string
	Ordinal                                                         int
}

type PrivateNomination struct {
	Status, ClaimCeiling, RejectionCondition                      string
	ID, Mode, RunID, InputID, CaseID, PolicyID, ResultID, Lineage string
	Seed                                                          uint64
	Slot, Community                                               int
	Members                                                       []string
	Endpoints                                                     []PrivateTypedEndpoint
	Gap                                                           string
}

type PrivateSourceIntent struct {
	NominationID, Mode, RunID, InputID, CaseID, PolicyID, ResultID, Lineage string
	Seed                                                                    uint64
	Slot, Community                                                         int
	Endpoint                                                                PrivateTypedEndpoint
	BindingState, Requirement, ClaimCeiling, Gap                            string
}

// BuildPrivateSourceIntents is preparation only. There is intentionally no
// Lookup, capture, projection, Packet, Describe, or worker dependency here.
// A positive intent construction never means that admitted source exists.
func privateEndpointRole(kind, role string) bool {
	switch kind {
	case "CALLS":
		return role == "CALLER_OCCURRENCE" || role == "CALLEE_TARGET"
	case "REFERENCES_SYMBOL":
		return role == "REFERENCING_OCCURRENCE" || role == "REFERENCED_SYMBOL"
	case "RESOLVES_TO_DEFINITION":
		return role == "QUERY_OCCURRENCE" || role == "DEFINITION_TARGET"
	}
	return false
}

func BuildPrivateSourceIntents(n PrivateNomination, maxEndpoints int) ([]PrivateSourceIntent, error) {
	if maxEndpoints <= 0 || len(n.Endpoints) > maxEndpoints || len(n.Endpoints) > 2*rg.EvaluationMaxOccurrencesPrivate+rg.EvaluationMaxNodesPrivate || len(n.Members) > rg.EvaluationMaxNodesPrivate || n.ID == "" || n.Mode == "" || n.RunID == "" || n.Lineage == "" {
		return nil, errors.New("private source intent bound or binding")
	}
	for _, s := range []string{n.ID, n.Mode, n.RunID, n.InputID, n.CaseID, n.PolicyID, n.ResultID, n.Lineage, n.Gap, n.Status, n.ClaimCeiling, n.RejectionCondition} {
		if len(s) > rg.EvaluationMaxStringPrivate {
			return nil, errors.New("private nomination string bound")
		}
	}
	members := make(map[string]bool, len(n.Members))
	for _, m := range n.Members {
		if m == "" || len(m) > rg.EvaluationMaxStringPrivate || members[m] {
			return nil, errors.New("private nomination member")
		}
		members[m] = true
	}
	count := len(n.Endpoints)
	if count == 0 {
		count = 1
	}
	out := make([]PrivateSourceIntent, 0, count)
	add := func(e PrivateTypedEndpoint, gap string) {
		out = append(out, PrivateSourceIntent{NominationID: n.ID, Mode: n.Mode, RunID: n.RunID, InputID: n.InputID, CaseID: n.CaseID, PolicyID: n.PolicyID, ResultID: n.ResultID, Lineage: n.Lineage, Seed: n.Seed, Slot: n.Slot, Community: n.Community, Endpoint: e, BindingState: "UNRESOLVED", Requirement: "REQUIRES_ADMITTED_SOURCE_BINDING_AND_FULL_DEFINITION", ClaimCeiling: rg.EvaluationCeilingPrivate, Gap: gap})
	}
	for _, e := range n.Endpoints {
		if !members[e.Node] || e.Ordinal < 0 || (e.Kind != "CALLS" && e.Kind != "REFERENCES_SYMBOL" && e.Kind != "RESOLVES_TO_DEFINITION") || !privateEndpointRole(e.Kind, e.Role) || e.Occurrence == "" {
			return nil, errors.New("private typed endpoint mismatch")
		}
		for _, s := range []string{e.Node, e.Kind, e.Occurrence, e.Role, e.Predecessor, e.Custody, e.Disposition} {
			if len(s) > rg.EvaluationMaxStringPrivate {
				return nil, errors.New("private endpoint string bound")
			}
		}
		add(e, "NO_ADMITTED_SOURCE_BINDING")
	}
	if len(out) == 0 {
		gap := n.Gap
		if gap == "" {
			gap = "NO_OCCURRENCE_ENDPOINT_SOURCE_BINDING"
		}
		add(PrivateTypedEndpoint{}, gap)
	}
	return out, nil
}
