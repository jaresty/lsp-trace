package programcrepresentative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"lsp-trace/internal/strictjson"
)

// CheckedQualificationCalculationV2 is an opaque mechanical calculation, not a
// production qualification capability. Its inputs may come from an untrusted
// adapter; only the Census owner can establish private inventory provenance.
type CheckedQualificationCalculationV2 struct {
	state *checkedQualificationCalculationStateV2
}
type checkedQualificationCalculationStateV2 struct {
	constituents       []ConstituentV2
	mechanicsHandles   map[string]MechanicsV2
	subject            ArtifactBindingDTO
	subjectDTO         QualificationSubjectV2
	artifacts          map[ArtifactBindingDTO][]byte
	envelope           ArtifactBindingDTO
	terminalGeneration uint64
	mechanics          map[string][]byte
	custody            map[string]CustodyIdentityDTO
}

// MultiSeedResultV2 is an immutable canonical value. Calculated records consistency
// with a checked calculation; it does not establish production qualification.
type MultiSeedResultV2 struct {
	dto        ResultV2
	canonical  []byte
	calculated *checkedQualificationCalculationStateV2
}

func (r MultiSeedResultV2) Identity() string { return r.dto.Identity }
func (r MultiSeedResultV2) Calculated() bool { return r.calculated != nil }
func (r MultiSeedResultV2) CanonicalBytes() ([]byte, error) {
	if _, err := CanonicalSizeResultV2(r.dto); err != nil {
		return nil, err
	}
	if uint64(len(r.canonical)) > MaxArtifactCanonicalBytesV2 {
		return nil, fmt.Errorf("multiseed result cap")
	}
	return append([]byte{}, r.canonical...), nil
}
func (r MultiSeedResultV2) DTO() (ResultV2, error) {
	if _, err := CanonicalSizeResultV2(r.dto); err != nil {
		return ResultV2{}, err
	}
	return cloneResultV2(r.dto), nil
}
func cloneResultV2(in ResultV2) ResultV2 {
	out := in
	out.Composition.Mechanics = cloneMechanicsDTOv2(in.Composition.Mechanics)
	out.Grouping.SelectionContext = cloneSelectionContextV2(in.Grouping.SelectionContext)
	out.GroupResults = make([]GroupResultV2, len(in.GroupResults))
	for i, g := range in.GroupResults {
		out.GroupResults[i] = g
		out.GroupResults[i].Record.SCCMembers = append([]string{}, g.Record.SCCMembers...)
		out.GroupResults[i].Record.IncomingPredecessors = append([]PredecessorV2{}, g.Record.IncomingPredecessors...)
	}
	return out
}

// FinalizeMultiSeedV2 actually performs composition -> grouping -> derived
// qualification -> all local records -> outer result. D is never multiplied.
func FinalizeMultiSeedV2(m MechanicsV2, q CheckedQualificationCalculationV2) (MultiSeedResultV2, error) {
	fail := func(err error) (MultiSeedResultV2, error) { return MultiSeedResultV2{}, err }
	if q.state == nil || q.state.terminalGeneration == 0 || !validDigest(q.state.envelope.Identity) {
		return fail(fmt.Errorf("v2 checked calculation unavailable"))
	}
	if _, err := CanonicalSizeMechanicsIdentityDTOv2(m.dto); err != nil {
		return fail(err)
	}
	if !bytes.Equal(q.state.mechanics[m.Identity()], m.canonical) || !m.Trusted() {
		return fail(fmt.Errorf("v2 qualification mechanics context mismatch"))
	}
	custody, ok := q.state.custody[m.dto.CustodyIdentity]
	if !ok || custody.Identity != m.dto.CustodyIdentity {
		return fail(fmt.Errorf("v2 qualification custody mismatch"))
	}
	_, policy, _, err := BuildStage6PolicyArtifactV2()
	if err != nil {
		return fail(err)
	}
	_, limits, _, err := BuildStage6LimitsArtifactV2()
	if err != nil {
		return fail(err)
	}
	c := CompositionV2{SchemaVersion: CompositionSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Mechanics: m.dto, Custody: custody, Policy: policy, QualificationEnvelope: q.state.envelope}
	n, err := CanonicalSizeCompositionV2(c)
	pre, err := marshalSizedV2(c, n, err)
	if err != nil {
		return fail(err)
	}
	c.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-composition:v2", pre)
	n, err = CanonicalSizeCompositionV2(c)
	raw, err := marshalSizedV2(c, n, err)
	if err != nil {
		return fail(err)
	}
	cb := multiSeedBindingV2(c.SchemaVersion, c.Identity, raw)
	g := GroupingV2{SchemaVersion: GroupingSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Composition: cb, Limits: limits, Algorithm: "gonum.org/v1/gonum/graph/community.Leiden", NumericContract: "EXACT_INTEGER_DIRECTED_GRAPH_V1", SelectionContext: m.dto.SelectionContext}
	n, err = CanonicalSizeGroupingV2(g)
	pre, err = marshalSizedV2(g, n, err)
	if err != nil {
		return fail(err)
	}
	g.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-grouping:v2", pre)
	n, err = CanonicalSizeGroupingV2(g)
	raw, err = marshalSizedV2(g, n, err)
	if err != nil {
		return fail(err)
	}
	gb := multiSeedBindingV2(g.SchemaVersion, g.Identity, raw)
	d := DerivedQualificationV2{SchemaVersion: DerivedQualificationSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Composition: cb, Grouping: gb, Envelope: q.state.envelope, ValidFrom: q.state.terminalGeneration, ValidThrough: q.state.terminalGeneration}
	n, err = CanonicalSizeDerivedQualificationV2(d)
	pre, err = marshalSizedV2(d, n, err)
	if err != nil {
		return fail(err)
	}
	d.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-derived-qualification:v2", pre)
	records, accounting, err := computeGroupRecordsV2(m)
	if err != nil {
		return fail(err)
	}
	r := ResultV2{SchemaVersion: ResultSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Composition: c, Grouping: g, Qualification: d, Terminal: "GROUPS_TERMINAL_COMPLETE", GroupResults: records, GroupAccounting: accounting}
	n, err = CanonicalSizeResultV2(r)
	pre, err = marshalSizedV2(r, n, err)
	if err != nil {
		return fail(err)
	}
	r.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-result:v2", pre)
	n, err = CanonicalSizeResultV2(r)
	raw, err = marshalSizedV2(r, n, err)
	if err != nil {
		return fail(err)
	}
	return MultiSeedResultV2{dto: cloneResultV2(r), canonical: raw, calculated: q.state}, nil
}

// ParseMultiSeedResultV2 validates exact wire identity and nested semantics;
// it does not replace expected-context reconstruction or the review verifier.
func ParseMultiSeedResultV2(raw []byte) (MultiSeedResultV2, error) {
	fail := func(err error) (MultiSeedResultV2, error) { return MultiSeedResultV2{}, err }
	if uint64(len(raw)) > MaxArtifactCanonicalBytesV2 || !utf8.Valid(raw) {
		return fail(fmt.Errorf("invalid v2 result size/UTF-8"))
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return fail(err)
	}
	var dto ResultV2
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return fail(err)
	}
	n, err := CanonicalSizeResultV2(dto)
	canonical, err := marshalSizedV2(dto, n, err)
	if err != nil || !bytes.Equal(raw, canonical) {
		return fail(fmt.Errorf("noncanonical v2 result"))
	}
	if err := validateResultV2(dto); err != nil {
		return fail(err)
	}
	return MultiSeedResultV2{dto: cloneResultV2(dto), canonical: append([]byte{}, raw...)}, nil
}

func validateResultV2(r ResultV2) error {
	if r.SchemaVersion != ResultSchemaVersionV2 || r.Profile != ProfileCallsOnlyFrozenCensusV1 || r.Terminal != "GROUPS_TERMINAL_COMPLETE" || r.Authority != 0 || r.Accepted || !validDigest(r.Identity) {
		return fmt.Errorf("invalid v2 result envelope")
	}
	// Each independently hashed object blanks only its own root identity.
	c, g, d := r.Composition, r.Grouping, r.Qualification
	if c.SchemaVersion != CompositionSchemaVersionV2 || g.SchemaVersion != GroupingSchemaVersionV2 || d.SchemaVersion != DerivedQualificationSchemaVersionV2 || c.Profile != r.Profile || g.Profile != r.Profile || d.Profile != r.Profile || c.Authority != 0 || g.Authority != 0 || d.Authority != 0 || c.Accepted || g.Accepted || d.Accepted {
		return fmt.Errorf("mixed/invalid v2 result versions")
	}
	if err := validateMechanicsEnvelopeV2(c.Mechanics); err != nil {
		return err
	}
	cc := c
	cc.Identity = ""
	n, err := CanonicalSizeCompositionV2(cc)
	pre, err := marshalSizedV2(cc, n, err)
	if err != nil || c.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-composition:v2", pre) {
		return fmt.Errorf("v2 composition identity mismatch")
	}
	n, err = CanonicalSizeCompositionV2(c)
	raw, err := marshalSizedV2(c, n, err)
	if err != nil {
		return err
	}
	cb := multiSeedBindingV2(c.SchemaVersion, c.Identity, raw)
	gc := g
	gc.Identity = ""
	n, err = CanonicalSizeGroupingV2(gc)
	pre, err = marshalSizedV2(gc, n, err)
	if err != nil || g.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-grouping:v2", pre) {
		return fmt.Errorf("v2 grouping identity mismatch")
	}
	n, err = CanonicalSizeGroupingV2(g)
	raw, err = marshalSizedV2(g, n, err)
	if err != nil {
		return err
	}
	gb := multiSeedBindingV2(g.SchemaVersion, g.Identity, raw)
	dc := d
	dc.Identity = ""
	n, err = CanonicalSizeDerivedQualificationV2(dc)
	pre, err = marshalSizedV2(dc, n, err)
	if err != nil || d.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-derived-qualification:v2", pre) {
		return fmt.Errorf("v2 derived qualification identity mismatch")
	}
	rc := r
	rc.Identity = ""
	n, err = CanonicalSizeResultV2(rc)
	pre, err = marshalSizedV2(rc, n, err)
	if err != nil || r.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-result:v2", pre) {
		return fmt.Errorf("v2 result identity mismatch")
	}
	_, pb, _, err := BuildStage6PolicyArtifactV2()
	if err != nil {
		return err
	}
	_, lb, _, err := BuildStage6LimitsArtifactV2()
	if err != nil {
		return err
	}
	custody := c.Custody
	custody.Identity = ""
	expectedCustody, err := hashIdentity(custodyDomain, custody)
	if err != nil || c.Custody.SchemaVersion != CustodySchemaVersion || expectedCustody != c.Custody.Identity || c.Custody.Identity != c.Mechanics.CustodyIdentity || c.Policy != pb || g.Limits != lb || g.Composition != cb || g.Algorithm != "gonum.org/v1/gonum/graph/community.Leiden" || g.NumericContract != "EXACT_INTEGER_DIRECTED_GRAPH_V1" || !equalJSON(g.SelectionContext, c.Mechanics.SelectionContext) || d.Composition != cb || d.Grouping != gb || d.Envelope != c.QualificationEnvelope || d.ValidFrom == 0 || d.ValidFrom != d.ValidThrough {
		return fmt.Errorf("v2 result contextual binding mismatch")
	}
	if !validDigest(d.Envelope.Identity) || !validDigest(d.Envelope.SHA256) || d.Envelope.ByteLength == 0 || d.Envelope.ByteLength > MaxArtifactCanonicalBytesV2 || d.Envelope.SchemaVersion != "lsp-trace.program-c-representative-qualification.private.v2" {
		return fmt.Errorf("invalid v2 qualification binding")
	}
	records, a, err := groupRecordsFromAdmittedDTOv2(c.Mechanics)
	if err != nil || !equalJSON(records, r.GroupResults) || a != r.GroupAccounting {
		return fmt.Errorf("v2 group result recomputation mismatch")
	}
	return nil
}
