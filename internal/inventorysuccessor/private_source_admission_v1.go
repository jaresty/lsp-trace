package inventorysuccessor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

// PrivateSourceAdmission cannot be constructed from a lower candidate or public
// unit. Its mint remains inside the producer package, downstream of completion.
type PrivateSourceAdmission struct{ state *privateSourceAdmissionV1 }
type privateSourceAdmissionV1 struct {
	intent    targetpacket.PrivateSourceIntent
	unit      targetpacket.PrivateSourceUnit
	originals PrivateProducerOriginals
}

func (a PrivateSourceAdmission) Valid() bool { return a.state != nil }
func (a PrivateSourceAdmission) Unit() (targetpacket.PrivateSourceUnit, error) {
	if a.state == nil {
		return targetpacket.PrivateSourceUnit{}, targetpacket.ErrPrivateSource
	}
	return a.state.unit, nil
}
func (a PrivateSourceAdmission) Matches(i targetpacket.PrivateSourceIntent) bool {
	return a.state != nil && a.state.intent == i
}
func (a PrivateSourceAdmission) Bound() int {
	if a.state == nil {
		return 0
	}
	u := a.state.unit
	return len(u.Body)*6 + 6*(len(u.URI)+len(u.Revision)+len(u.Privacy)+len(u.Workspace)+len(u.SessionID)+len(u.DocumentVersion)+len(u.PositionEncoding)+len(u.SymbolID)) + 16384
}
func mintPrivateSourceAdmission(i targetpacket.PrivateSourceIntent, o PrivateProducerOriginals, c targetpacket.PrivateSourceCandidate) (PrivateSourceAdmission, error) {
	if o.state == nil || !c.Matches(i, o.candidate) {
		return PrivateSourceAdmission{}, targetpacket.ErrPrivateSource
	}
	u, err := c.Unit()
	if err != nil || u.ClaimCeiling != targetpacket.PrivateSourceCandidateCeiling {
		return PrivateSourceAdmission{}, targetpacket.ErrPrivateSource
	}
	u.ClaimCeiling = targetpacket.PrivateSourceCeiling
	raw, err := json.Marshal(u)
	if err != nil {
		return PrivateSourceAdmission{}, err
	}
	u.ID = fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	return PrivateSourceAdmission{&privateSourceAdmissionV1{i, u, o}}, nil
}
func AdmitPrivateEndpointSource(i targetpacket.PrivateSourceIntent, o PrivateProducerOriginals, definitions []sessionruntime.PrivateGroupingSourceEvidence, objects []sourceobject.Object, l targetpacket.PrivateSourceLimits) (PrivateSourceAdmission, string) {
	if o.state == nil {
		return PrivateSourceAdmission{}, "NO_OPAQUE_PRODUCER_ORIGINALS"
	}
	c, gap := targetpacket.CalculatePrivateEndpointSource(i, o.candidate, definitions, objects, l)
	if !c.Valid() {
		return PrivateSourceAdmission{}, gap
	}
	a, err := mintPrivateSourceAdmission(i, o, c)
	if err != nil {
		return PrivateSourceAdmission{}, err.Error()
	}
	return a, ""
}
func RebindPrivateSourceAdmission(i targetpacket.PrivateSourceIntent, a PrivateSourceAdmission) (PrivateSourceAdmission, error) {
	if a.state == nil {
		return PrivateSourceAdmission{}, targetpacket.ErrPrivateSource
	}
	old := a.state.intent
	if i.InputID != old.InputID || i.Endpoint != old.Endpoint || i.BindingState != old.BindingState || i.Requirement != old.Requirement || i.ClaimCeiling != old.ClaimCeiling {
		return PrivateSourceAdmission{}, targetpacket.ErrPrivateSource
	}
	s := *a.state
	s.intent = i
	return PrivateSourceAdmission{&s}, nil
}
