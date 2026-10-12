package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"lsp-trace/internal/programcadmission"
)

const (
	inputFamily  = "REFERENCE_GROUPING_INPUT"
	inputVersion = "v1"
	claimCeiling = "STRUCTURAL_OCCURRENCE_AND_CUSTODY_COMMITMENTS_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;NO_PRODUCER_AUTHENTICATION;NO_POLICY_QUALIFICATION;NO_FEATURE_ACCEPTANCE"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type PredecessorRecord struct {
	Role, Selector, Schema string
	Bytes                  []byte
	ClaimedDigest          string
}

type EndpointCustody struct {
	Endpoint, OccurrenceIdentity, EndpointRole, PredecessorSelector, Custody string
}

type Contribution struct {
	Kind, From, To, OccurrenceIdentity                                            string
	Ordinal                                                                       int
	MethodEvidenceSelector, QuerySelector, EvidenceSelector                       string
	MappingReceiptSelector, AcquisitionReceiptSelector, RetainedCompositeSelector string
	Action                                                                        string
	Weight                                                                        int
}

type CandidateOutcome struct{ OccurrenceIdentity, Disposition string }

type ResourceLimits struct {
	MaxArtifacts, MaxNodes, MaxOccurrences, MaxContributions, MaxCanonicalBytes, MaxStringBytes int
}

type ParseLimits struct {
	MaxRawBytes, MaxArtifacts, MaxNodes, MaxOccurrences, MaxContributions, MaxCanonicalBytes, MaxStringBytes int
}

type Accounting struct {
	Candidates, Included, Excluded, Duplicates int
	Terminal, Truncated                        bool
}

type InputRequest struct {
	TypedInput                  programcadmission.TypedInput
	Predecessors                []PredecessorRecord
	OccurrenceLedgerBytes       []byte
	EndpointCustody             []EndpointCustody
	OccurrenceAdmissionPolicyID string
	CompositionPolicyID         string
	PrivacyPolicy               string
	EligibleKinds               []programcadmission.RelationKind
	Limits                      ResourceLimits
	CandidateOutcomes           []CandidateOutcome
	Accounting                  Accounting
}

type predecessorJSON struct {
	Role       string `json:"role"`
	Selector   string `json:"selector"`
	Schema     string `json:"schema"`
	ByteLength int    `json:"byte_length"`
	Digest     string `json:"digest"`
}
type custodyJSON struct {
	Endpoint            string `json:"endpoint"`
	OccurrenceIdentity  string `json:"occurrence_identity"`
	EndpointRole        string `json:"endpoint_role"`
	PredecessorSelector string `json:"predecessor_selector"`
	Custody             string `json:"custody"`
}
type contributionJSON struct {
	Kind                       string `json:"kind"`
	From                       string `json:"from"`
	To                         string `json:"to"`
	Ordinal                    int    `json:"ordinal"`
	OccurrenceIdentity         string `json:"occurrence_identity"`
	MethodEvidenceSelector     string `json:"method_evidence_selector,omitempty"`
	QuerySelector              string `json:"query_selector,omitempty"`
	EvidenceSelector           string `json:"evidence_selector,omitempty"`
	MappingReceiptSelector     string `json:"mapping_receipt_selector,omitempty"`
	AcquisitionReceiptSelector string `json:"acquisition_receipt_selector,omitempty"`
	RetainedCompositeSelector  string `json:"retained_composite_selector,omitempty"`
}
type outcomeJSON struct {
	OccurrenceIdentity string `json:"occurrence_identity"`
	Disposition        string `json:"disposition"`
}
type limitsJSON struct {
	MaxArtifacts      int `json:"max_artifacts"`
	MaxNodes          int `json:"max_nodes"`
	MaxOccurrences    int `json:"max_occurrences"`
	MaxContributions  int `json:"max_contributions"`
	MaxCanonicalBytes int `json:"max_canonical_bytes"`
	MaxStringBytes    int `json:"max_string_bytes"`
}
type accountingJSON struct {
	Candidates int  `json:"candidates"`
	Included   int  `json:"included"`
	Excluded   int  `json:"excluded"`
	Duplicates int  `json:"duplicates"`
	Terminal   bool `json:"terminal"`
	Truncated  bool `json:"truncated"`
}
type document struct {
	Family                      string             `json:"family"`
	Version                     string             `json:"version"`
	ClaimCeiling                string             `json:"claim_ceiling"`
	Authority                   int                `json:"authority"`
	Accepted                    bool               `json:"accepted"`
	Completeness                string             `json:"completeness"`
	OccurrenceAdmissionPolicyID string             `json:"occurrence_admission_policy_id"`
	CompositionPolicyID         string             `json:"composition_policy_id"`
	PrivacyPolicy               string             `json:"privacy_policy"`
	EligibleKinds               []string           `json:"eligible_kinds"`
	ResourceLimits              limitsJSON         `json:"resource_limits"`
	Predecessors                []predecessorJSON  `json:"predecessors"`
	EndpointCustody             []custodyJSON      `json:"endpoint_custody"`
	Contributions               []contributionJSON `json:"contributions"`
	CandidateOutcomes           []outcomeJSON      `json:"candidate_outcomes"`
	Accounting                  accountingJSON     `json:"accounting"`
}

type Artifact struct {
	canonical []byte
	digest    string
	doc       document
}
type AdmittedInput struct{ artifact Artifact }

func (a Artifact) CanonicalBytes() []byte             { return append([]byte(nil), a.canonical...) }
func (a Artifact) ID() string                         { return a.digest }
func (a Artifact) Digest() string                     { return a.digest }
func (a Artifact) Family() string                     { return a.doc.Family }
func (a Artifact) Version() string                    { return a.doc.Version }
func (a Artifact) ClaimCeiling() string               { return a.doc.ClaimCeiling }
func (a Artifact) Authority() int                     { return a.doc.Authority }
func (a Artifact) Accepted() bool                     { return a.doc.Accepted }
func (a Artifact) Completeness() string               { return a.doc.Completeness }
func (a Artifact) Contributions() []Contribution      { return publicContributions(a.doc.Contributions) }
func (a Artifact) EndpointCustody() []EndpointCustody { return publicCustody(a.doc.EndpointCustody) }
func (a Artifact) CandidateOutcomes() []CandidateOutcome {
	return publicOutcomes(a.doc.CandidateOutcomes)
}
func (a Artifact) Accounting() Accounting  { return publicAccounting(a.doc.Accounting) }
func (a Artifact) EligibleKinds() []string { return append([]string(nil), a.doc.EligibleKinds...) }
func (a Artifact) Predecessors() []PredecessorRecord {
	out := make([]PredecessorRecord, len(a.doc.Predecessors))
	for i, p := range a.doc.Predecessors {
		out[i] = PredecessorRecord{Role: p.Role, Selector: p.Selector, Schema: p.Schema}
	}
	return out
}
func (a AdmittedInput) CanonicalBytes() []byte                { return a.artifact.CanonicalBytes() }
func (a AdmittedInput) ID() string                            { return a.artifact.ID() }
func (a AdmittedInput) Digest() string                        { return a.artifact.Digest() }
func (a AdmittedInput) ByteLength() int                       { return len(a.artifact.canonical) }
func (a AdmittedInput) Artifact() Artifact                    { return cloneArtifact(a.artifact) }
func (a AdmittedInput) Contributions() []Contribution         { return a.artifact.Contributions() }
func (a AdmittedInput) EndpointCustody() []EndpointCustody    { return a.artifact.EndpointCustody() }
func (a AdmittedInput) CandidateOutcomes() []CandidateOutcome { return a.artifact.CandidateOutcomes() }
func (a AdmittedInput) Accounting() Accounting                { return a.artifact.Accounting() }
func (a AdmittedInput) PredecessorIdentityForContribution(c Contribution) string {
	switch c.Kind {
	case string(programcadmission.TypedCalls):
		return c.AcquisitionReceiptSelector
	case string(programcadmission.TypedReferencesSymbol), string(programcadmission.TypedResolvesToDefinition):
		return c.MethodEvidenceSelector
	default:
		return ""
	}
}
func (a AdmittedInput) CustodyIdentityForOccurrence(identity string) string {
	var rows []EndpointCustody
	for _, c := range a.artifact.EndpointCustody() {
		if c.OccurrenceIdentity == identity {
			rows = append(rows, c)
		}
	}
	if len(rows) == 0 {
		return ""
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Endpoint != rows[j].Endpoint {
			return rows[i].Endpoint < rows[j].Endpoint
		}
		if rows[i].EndpointRole != rows[j].EndpointRole {
			return rows[i].EndpointRole < rows[j].EndpointRole
		}
		if rows[i].PredecessorSelector != rows[j].PredecessorSelector {
			return rows[i].PredecessorSelector < rows[j].PredecessorSelector
		}
		return rows[i].Custody < rows[j].Custody
	})
	h := sha256.New()
	h.Write([]byte("lsp-trace:reference-grouping-occurrence-custody:v1\x00"))
	for _, c := range rows {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\n", c.Endpoint, c.OccurrenceIdentity, c.EndpointRole, c.PredecessorSelector, c.Custody)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}
func (a AdmittedInput) MatchesTypedInput(in programcadmission.TypedInput) bool {
	projection, err := programcadmission.PreparePrivateLeidenProjection(in)
	if err != nil {
		return false
	}
	kinds := projection.SelectedKinds()
	eligible := make(map[string]bool, len(a.artifact.doc.EligibleKinds))
	for _, kind := range a.artifact.doc.EligibleKinds {
		eligible[kind] = true
	}
	for _, kind := range kinds {
		if !eligible[string(kind)] {
			return false
		}
	}
	occurrences := projection.Occurrences()
	if len(occurrences) != len(a.artifact.doc.CandidateOutcomes) {
		return false
	}
	byIdentity := make(map[string]programcadmission.TypedOccurrence, len(occurrences))
	for _, occurrence := range occurrences {
		if occurrence.Identity == "" || byIdentity[occurrence.Identity].Identity != "" {
			return false
		}
		byIdentity[occurrence.Identity] = occurrence
	}
	for _, outcome := range a.artifact.doc.CandidateOutcomes {
		if byIdentity[outcome.OccurrenceIdentity].Identity == "" {
			return false
		}
	}
	for _, contribution := range a.artifact.doc.Contributions {
		occurrence, ok := byIdentity[contribution.OccurrenceIdentity]
		if !ok || occurrence.From != contribution.From || occurrence.To != contribution.To ||
			string(occurrence.Kind) != contribution.Kind || occurrence.Ordinal != contribution.Ordinal {
			return false
		}
	}
	return true
}

func BuildInput(req InputRequest) (Artifact, error) {
	projection, err := programcadmission.PreparePrivateLeidenProjection(req.TypedInput)
	if err != nil {
		return Artifact{}, err
	}
	occurrences := projection.Occurrences()
	if err := validateRequest(req, occurrences); err != nil {
		return Artifact{}, err
	}

	preds := make([]predecessorJSON, len(req.Predecessors))
	selectorByRole := make(map[string]string, len(req.Predecessors))
	roleBySelector := make(map[string]string, len(req.Predecessors))
	for i, p := range req.Predecessors {
		wantSelector, wantSchema, ok := predecessorContract(p.Role)
		if !ok || p.Selector != wantSelector || p.Schema != wantSchema || len(p.Bytes) == 0 || (p.ClaimedDigest != "" && (!digestPattern.MatchString(p.ClaimedDigest) || p.ClaimedDigest == "sha256:"+strings.Repeat("0", 64))) {
			return Artifact{}, errors.New("invalid predecessor")
		}
		if selectorByRole[p.Role] != "" || roleBySelector[p.Selector] != "" {
			return Artifact{}, errors.New("duplicate predecessor identity")
		}
		h := sha256.Sum256(p.Bytes)
		preds[i] = predecessorJSON{p.Role, p.Selector, p.Schema, len(p.Bytes), fmt.Sprintf("sha256:%x", h)}
		selectorByRole[p.Role] = p.Selector
		roleBySelector[p.Selector] = p.Role
	}
	sort.Slice(preds, func(i, j int) bool {
		if preds[i].Role != preds[j].Role {
			return preds[i].Role < preds[j].Role
		}
		return preds[i].Selector < preds[j].Selector
	})
	ledger := predecessorByRole(req.Predecessors, "OCCURRENCE_LEDGER")
	if ledger == nil {
		return Artifact{}, errors.New("missing occurrence ledger")
	}
	canonicalLedger, err := canonicalOccurrenceLedger(occurrences)
	if err != nil || !bytes.Equal(ledger.Bytes, canonicalLedger) {
		return Artifact{}, errors.New("invalid occurrence ledger")
	}
	if len(req.OccurrenceLedgerBytes) != 0 && !bytes.Equal(req.OccurrenceLedgerBytes, ledger.Bytes) {
		return Artifact{}, errors.New("mismatched occurrence ledger")
	}
	if err := validateBoundPredecessors(req.Predecessors, projection.MethodEvidence(), projection.MethodCanonicalBytes(), projection.CompositeSourceBinding(), projection.RetainedCallsCanonicalBytes()); err != nil {
		return Artifact{}, err
	}

	methodByIdentity := map[string]programcadmission.MethodOccurrenceEvidence{}
	for _, m := range projection.MethodEvidence() {
		methodByIdentity[m.Identity] = m
	}
	contribs := make([]contributionJSON, len(occurrences))
	for i, o := range occurrences {
		c := contributionJSON{Kind: string(o.Kind), From: o.From, To: o.To, Ordinal: o.Ordinal, OccurrenceIdentity: o.Identity}
		switch o.Kind {
		case programcadmission.TypedCalls:
			c.AcquisitionReceiptSelector = selectorByRole["ACQUISITION_RECEIPT"]
			c.RetainedCompositeSelector = selectorByRole["RETAINED_COMPOSITE"]
			if selectorByRole["ACQUISITION_RECEIPT"] == "" || c.RetainedCompositeSelector == "" {
				return Artifact{}, errors.New("missing calls predecessor")
			}
		case programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition:
			m, ok := methodByIdentity[o.Identity]
			if !ok || m.Kind != o.Kind || m.From != o.From || m.To != o.To || m.Ordinal != o.Ordinal {
				return Artifact{}, errors.New("method occurrence mismatch")
			}
			c.MethodEvidenceSelector, c.QuerySelector, c.EvidenceSelector = m.EvidenceSelector, m.QuerySelector, m.EvidenceSelector
			c.MappingReceiptSelector = selectorByRole["METHOD_MAPPING_RECEIPT"]
			if c.MethodEvidenceSelector != selectorByRole["METHOD_EVIDENCE"] || c.QuerySelector != selectorByRole["METHOD_QUERY"] || c.MappingReceiptSelector == "" || c.MappingReceiptSelector == c.EvidenceSelector {
				return Artifact{}, errors.New("method predecessor mismatch")
			}
		default:
			return Artifact{}, errors.New("unsupported contribution kind")
		}
		contribs[i] = c
	}
	sort.Slice(contribs, func(i, j int) bool {
		return contribs[i].OccurrenceIdentity < contribs[j].OccurrenceIdentity
	})

	if err := validateRequiredPredecessors(selectorByRole, occurrences); err != nil {
		return Artifact{}, err
	}
	custody, err := expandCustody(req.EndpointCustody, occurrences, roleBySelector)
	if err != nil {
		return Artifact{}, err
	}
	outcomes := make([]outcomeJSON, len(req.CandidateOutcomes))
	for i, o := range req.CandidateOutcomes {
		outcomes[i] = outcomeJSON{o.OccurrenceIdentity, o.Disposition}
	}
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].OccurrenceIdentity < outcomes[j].OccurrenceIdentity })
	kinds := make([]string, len(req.EligibleKinds))
	for i, k := range req.EligibleKinds {
		kinds[i] = string(k)
	}
	sort.Strings(kinds)
	doc := document{inputFamily, inputVersion, claimCeiling, 0, false, "UNKNOWN", req.OccurrenceAdmissionPolicyID, req.CompositionPolicyID, req.PrivacyPolicy, kinds, limitsJSON{req.Limits.MaxArtifacts, req.Limits.MaxNodes, req.Limits.MaxOccurrences, req.Limits.MaxContributions, req.Limits.MaxCanonicalBytes, req.Limits.MaxStringBytes}, preds, custody, contribs, outcomes, accountingJSON{req.Accounting.Candidates, req.Accounting.Included, req.Accounting.Excluded, req.Accounting.Duplicates, req.Accounting.Terminal, req.Accounting.Truncated}}
	if err := validateDocument(doc); err != nil {
		return Artifact{}, err
	}
	return artifactFromDocument(doc, req.Limits.MaxCanonicalBytes)
}

func ParseInput(raw []byte, limits ParseLimits) (Artifact, error) {
	if limits.MaxRawBytes <= 0 || len(raw) > limits.MaxRawBytes || len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || bytes.Contains(raw, []byte("\r")) {
		return Artifact{}, errors.New("invalid raw framing")
	}
	if err := scanJSON(raw[:len(raw)-1], limits); err != nil {
		return Artifact{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc document
	if err := dec.Decode(&doc); err != nil {
		return Artifact{}, err
	}
	if err := requireEOF(dec); err != nil {
		return Artifact{}, err
	}
	if err := validateDocument(doc); err != nil {
		return Artifact{}, err
	}
	if !arraysCanonical(doc) {
		return Artifact{}, errors.New("noncanonical array order")
	}
	if len(doc.Predecessors) > limits.MaxArtifacts || uniqueCustodyEndpoints(doc.EndpointCustody) > limits.MaxNodes || len(doc.Contributions) > limits.MaxContributions || len(doc.CandidateOutcomes) > limits.MaxOccurrences {
		return Artifact{}, errors.New("resource bound exceeded")
	}
	canonicalLimit := limits.MaxCanonicalBytes
	if doc.ResourceLimits.MaxCanonicalBytes < canonicalLimit {
		canonicalLimit = doc.ResourceLimits.MaxCanonicalBytes
	}
	a, err := artifactFromDocument(doc, canonicalLimit)
	if err != nil {
		return Artifact{}, err
	}
	if !bytes.Equal(a.canonical, raw) {
		return Artifact{}, errors.New("noncanonical input")
	}
	return a, nil
}

func ParseAndAdmit(raw []byte, predecessors []PredecessorRecord, limits ParseLimits) (AdmittedInput, error) {
	a, err := ParseInput(raw, limits)
	if err != nil {
		return AdmittedInput{}, err
	}
	return AdmitInput(a, predecessors)
}
func AdmitInput(a Artifact, supplied []PredecessorRecord) (AdmittedInput, error) {
	if len(a.canonical) == 0 || len(supplied) != len(a.doc.Predecessors) {
		return AdmittedInput{}, errors.New("predecessor set mismatch")
	}
	if !arraysCanonical(a.doc) {
		return AdmittedInput{}, errors.New("noncanonical array order")
	}
	rebuilt, err := artifactFromDocument(a.doc, len(a.canonical))
	if err != nil || !bytes.Equal(rebuilt.canonical, a.canonical) || rebuilt.digest != a.digest {
		return AdmittedInput{}, errors.New("noncanonical artifact")
	}
	bySelector := make(map[string]PredecessorRecord, len(supplied))
	for _, p := range supplied {
		if _, ok := bySelector[p.Selector]; ok {
			return AdmittedInput{}, errors.New("duplicate supplied predecessor")
		}
		bySelector[p.Selector] = p
	}
	for _, want := range a.doc.Predecessors {
		p, ok := bySelector[want.Selector]
		if !ok || p.Role != want.Role || p.Schema != want.Schema || len(p.Bytes) != want.ByteLength {
			return AdmittedInput{}, errors.New("predecessor mismatch")
		}
		h := sha256.Sum256(p.Bytes)
		if fmt.Sprintf("sha256:%x", h) != want.Digest {
			return AdmittedInput{}, errors.New("predecessor digest mismatch")
		}
	}
	if err := validateOccurrenceLedger(a.doc); err != nil {
		return AdmittedInput{}, err
	}
	if err := validateSuppliedOccurrenceLedger(a.doc, bySelector["ledgers/occurrences.json"].Bytes); err != nil {
		return AdmittedInput{}, err
	}
	if err := replaySuppliedPredecessors(a.doc, bySelector); err != nil {
		return AdmittedInput{}, err
	}
	return AdmittedInput{artifact: cloneArtifact(a)}, nil
}

func validateRequest(req InputRequest, occurrences []programcadmission.TypedOccurrence) error {
	nodes := make(map[string]struct{}, 2*len(occurrences))
	maxString := 0
	observe := func(values ...string) {
		for _, value := range values {
			if len(value) > maxString {
				maxString = len(value)
			}
		}
	}
	observe(req.OccurrenceAdmissionPolicyID, req.CompositionPolicyID, req.PrivacyPolicy)
	for _, occurrence := range occurrences {
		nodes[occurrence.From], nodes[occurrence.To] = struct{}{}, struct{}{}
		observe(occurrence.Identity, occurrence.RelationID, occurrence.From, occurrence.To, string(occurrence.Kind))
	}
	for _, predecessor := range req.Predecessors {
		observe(predecessor.Role, predecessor.Selector, predecessor.Schema, predecessor.ClaimedDigest)
	}
	for _, custody := range req.EndpointCustody {
		observe(custody.Endpoint, custody.OccurrenceIdentity, custody.EndpointRole, custody.PredecessorSelector, custody.Custody)
	}
	if len(req.Predecessors) > req.Limits.MaxArtifacts || len(nodes) > req.Limits.MaxNodes || len(occurrences) > req.Limits.MaxOccurrences || len(occurrences) > req.Limits.MaxContributions || maxString > req.Limits.MaxStringBytes {
		return errors.New("resource bound exceeded")
	}
	if req.Accounting.Candidates != len(occurrences) || req.Accounting.Included != len(occurrences) || req.Accounting.Excluded != 0 || req.Accounting.Duplicates != 0 || !req.Accounting.Terminal || req.Accounting.Truncated {
		return errors.New("invalid accounting")
	}
	return nil
}
func predecessorContract(role string) (string, string, bool) {
	contracts := map[string][2]string{
		"OCCURRENCE_LEDGER": {"ledgers/occurrences.json", "lsp-trace.private.program-c-occurrence-ledger.v1"}, "ACQUISITION_RECEIPT": {"calls/acquisition.json", "lsp-trace.private.program-c-acquisition-receipt.v1"}, "RETAINED_COMPOSITE": {"calls/composite.json", "lsp-trace.private.program-c-composite.v1"}, "METHOD_QUERY": {"methods/query.json", "lsp-trace.private.program-c-method-query.v0"}, "METHOD_EVIDENCE": {"methods/evidence.json", programcadmission.MethodEvidenceFamily}, "METHOD_MAPPING_RECEIPT": {"methods/mapping-receipt.json", "lsp-trace.private.program-c-method-mapping-receipt.v1"},
	}
	contract, ok := contracts[role]
	return contract[0], contract[1], ok
}
func validateRequiredPredecessors(selectors map[string]string, occurrences []programcadmission.TypedOccurrence) error {
	required := map[string]bool{"OCCURRENCE_LEDGER": true}
	for _, occurrence := range occurrences {
		if occurrence.Kind == programcadmission.TypedCalls {
			required["ACQUISITION_RECEIPT"], required["RETAINED_COMPOSITE"] = true, true
		} else {
			required["METHOD_QUERY"], required["METHOD_EVIDENCE"], required["METHOD_MAPPING_RECEIPT"] = true, true, true
		}
	}
	if len(selectors) != len(required) {
		return errors.New("predecessor set mismatch")
	}
	for role := range required {
		if selectors[role] == "" {
			return errors.New("missing predecessor role")
		}
	}
	return nil
}
func validateDocument(d document) error {
	if d.Family != inputFamily || d.Version != inputVersion || d.ClaimCeiling != claimCeiling || d.Authority != 0 || d.Accepted || d.Completeness != "UNKNOWN" {
		return errors.New("invalid claim envelope")
	}
	if len(d.EligibleKinds) != 3 || d.EligibleKinds[0] != "CALLS" || d.EligibleKinds[1] != "REFERENCES_SYMBOL" || d.EligibleKinds[2] != "RESOLVES_TO_DEFINITION" {
		return errors.New("invalid eligible kinds")
	}
	if d.OccurrenceAdmissionPolicyID == "" || d.CompositionPolicyID == "" || d.PrivacyPolicy == "" {
		return errors.New("missing policy identity")
	}
	if d.ResourceLimits.MaxArtifacts < 0 || d.ResourceLimits.MaxNodes < 0 || d.ResourceLimits.MaxOccurrences < 0 || d.ResourceLimits.MaxContributions < 0 || d.ResourceLimits.MaxCanonicalBytes < 0 || d.ResourceLimits.MaxStringBytes < 0 {
		return errors.New("negative limit")
	}
	if len(d.Predecessors) > d.ResourceLimits.MaxArtifacts || uniqueCustodyEndpoints(d.EndpointCustody) > d.ResourceLimits.MaxNodes || len(d.Contributions) > d.ResourceLimits.MaxContributions || len(d.CandidateOutcomes) > d.ResourceLimits.MaxOccurrences || maxDocumentStringBytes(d) > d.ResourceLimits.MaxStringBytes {
		return errors.New("embedded resource bound exceeded")
	}
	if d.Accounting.Candidates != len(d.Contributions) || d.Accounting.Included != len(d.Contributions) || d.Accounting.Excluded != 0 || d.Accounting.Duplicates != 0 || !d.Accounting.Terminal || d.Accounting.Truncated {
		return errors.New("invalid accounting")
	}
	if err := validateOccurrenceLedger(d); err != nil {
		return err
	}
	return validateSelectorFamilies(d)
}
func validateSelectorFamilies(d document) error {
	roles := make(map[string]string, len(d.Predecessors))
	for _, predecessor := range d.Predecessors {
		wantSelector, wantSchema, ok := predecessorContract(predecessor.Role)
		if !ok || predecessor.Selector != wantSelector || predecessor.Schema != wantSchema {
			return errors.New("predecessor contract mismatch")
		}
		if predecessor.Selector == "" || roles[predecessor.Selector] != "" {
			return errors.New("duplicate or empty predecessor selector")
		}
		roles[predecessor.Selector] = predecessor.Role
	}
	require := func(selector, role string) bool { return selector != "" && roles[selector] == role }
	for _, contribution := range d.Contributions {
		switch contribution.Kind {
		case "CALLS":
			if !require(contribution.AcquisitionReceiptSelector, "ACQUISITION_RECEIPT") || !require(contribution.RetainedCompositeSelector, "RETAINED_COMPOSITE") {
				return errors.New("calls predecessor family mismatch")
			}
		case "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION":
			if !require(contribution.QuerySelector, "METHOD_QUERY") || !require(contribution.MethodEvidenceSelector, "METHOD_EVIDENCE") || !require(contribution.EvidenceSelector, "METHOD_EVIDENCE") || !require(contribution.MappingReceiptSelector, "METHOD_MAPPING_RECEIPT") {
				return errors.New("method predecessor family mismatch")
			}
		}
	}
	for _, custody := range d.EndpointCustody {
		role := "METHOD_EVIDENCE"
		if custody.EndpointRole == "CALLER_OCCURRENCE" || custody.EndpointRole == "CALLEE_TARGET" {
			role = "ACQUISITION_RECEIPT"
		}
		if !require(custody.PredecessorSelector, role) {
			return errors.New("custody predecessor family mismatch")
		}
	}
	return nil
}
func validateOccurrenceLedger(d document) error {
	seen := map[string]bool{}
	for _, c := range d.Contributions {
		if c.OccurrenceIdentity == "" || seen[c.OccurrenceIdentity] {
			return errors.New("duplicate contribution")
		}
		seen[c.OccurrenceIdentity] = true
		if c.From == "" || c.To == "" || c.Ordinal < 0 {
			return errors.New("invalid contribution")
		}
		switch c.Kind {
		case "CALLS":
			if c.AcquisitionReceiptSelector == "" || c.RetainedCompositeSelector == "" || c.MethodEvidenceSelector != "" || c.QuerySelector != "" || c.EvidenceSelector != "" || c.MappingReceiptSelector != "" {
				return errors.New("invalid calls binding")
			}
		case "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION":
			if c.MethodEvidenceSelector == "" || c.QuerySelector == "" || c.EvidenceSelector == "" || c.MappingReceiptSelector == "" || c.AcquisitionReceiptSelector != "" || c.RetainedCompositeSelector != "" {
				return errors.New("invalid method binding")
			}
		default:
			return errors.New("invalid kind")
		}
	}
	out := map[string]bool{}
	for _, o := range d.CandidateOutcomes {
		if o.Disposition != "ADMITTED" || !seen[o.OccurrenceIdentity] || out[o.OccurrenceIdentity] {
			return errors.New("invalid candidate outcome")
		}
		out[o.OccurrenceIdentity] = true
	}
	if len(out) != len(seen) {
		return errors.New("outcome mismatch")
	}
	expectedCustody := make(map[string]string, 2*len(d.Contributions))
	for _, contribution := range d.Contributions {
		sourceRole, targetRole := "", ""
		switch contribution.Kind {
		case "CALLS":
			sourceRole, targetRole = "CALLER_OCCURRENCE", "CALLEE_TARGET"
		case "REFERENCES_SYMBOL":
			sourceRole, targetRole = "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
		case "RESOLVES_TO_DEFINITION":
			sourceRole, targetRole = "QUERY_OCCURRENCE", "DEFINITION_TARGET"
		}
		expectedCustody[contribution.OccurrenceIdentity+"\x00"+sourceRole] = contribution.From
		expectedCustody[contribution.OccurrenceIdentity+"\x00"+targetRole] = contribution.To
	}
	custody := map[string]int{}
	for _, c := range d.EndpointCustody {
		if !seen[c.OccurrenceIdentity] || c.Endpoint == "" || c.PredecessorSelector == "" || c.Custody != "CALLER_ASSERTED" {
			return errors.New("invalid custody")
		}
		key := c.OccurrenceIdentity + "\x00" + c.EndpointRole
		if custody[key] > 0 {
			return errors.New("duplicate custody")
		}
		if expectedCustody[key] != c.Endpoint {
			return errors.New("custody endpoint or role mismatch")
		}
		custody[key]++
		if c.EndpointRole != "REFERENCING_OCCURRENCE" && c.EndpointRole != "REFERENCED_SYMBOL" && c.EndpointRole != "QUERY_OCCURRENCE" && c.EndpointRole != "DEFINITION_TARGET" && c.EndpointRole != "CALLER_OCCURRENCE" && c.EndpointRole != "CALLEE_TARGET" {
			return errors.New("invalid endpoint role")
		}
	}
	if len(d.EndpointCustody) != 2*len(seen) {
		return errors.New("custody occurrence mismatch")
	}
	return nil
}
func expandCustody(base []EndpointCustody, occurrences []programcadmission.TypedOccurrence, roleBySelector map[string]string) ([]custodyJSON, error) {
	type custodyKey struct{ endpoint, predecessorRole string }
	byEndpointRole := map[custodyKey]EndpointCustody{}
	for _, c := range base {
		role := roleBySelector[c.PredecessorSelector]
		if c.Endpoint == "" || c.OccurrenceIdentity != "" || c.EndpointRole != "" || c.PredecessorSelector == "" || c.Custody != "CALLER_ASSERTED" || (role != "METHOD_EVIDENCE" && role != "ACQUISITION_RECEIPT") {
			return nil, errors.New("invalid endpoint custody")
		}
		key := custodyKey{c.Endpoint, role}
		if _, exists := byEndpointRole[key]; exists {
			return nil, errors.New("duplicate endpoint custody")
		}
		byEndpointRole[key] = c
	}
	out := make([]custodyJSON, 0, len(occurrences)*2)
	for _, o := range occurrences {
		predecessorRole, sr, tr := "", "", ""
		switch o.Kind {
		case programcadmission.TypedCalls:
			predecessorRole, sr, tr = "ACQUISITION_RECEIPT", "CALLER_OCCURRENCE", "CALLEE_TARGET"
		case programcadmission.TypedReferencesSymbol:
			predecessorRole, sr, tr = "METHOD_EVIDENCE", "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
		case programcadmission.TypedResolvesToDefinition:
			predecessorRole, sr, tr = "METHOD_EVIDENCE", "QUERY_OCCURRENCE", "DEFINITION_TARGET"
		default:
			return nil, errors.New("invalid occurrence kind")
		}
		from, ok1 := byEndpointRole[custodyKey{o.From, predecessorRole}]
		to, ok2 := byEndpointRole[custodyKey{o.To, predecessorRole}]
		if !ok1 || !ok2 {
			return nil, errors.New("missing endpoint custody")
		}
		identity := o.Identity
		out = append(out, custodyJSON{o.From, identity, sr, from.PredecessorSelector, from.Custody}, custodyJSON{o.To, identity, tr, to.PredecessorSelector, to.Custody})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Endpoint != out[j].Endpoint {
			return out[i].Endpoint < out[j].Endpoint
		}
		if out[i].OccurrenceIdentity != out[j].OccurrenceIdentity {
			return out[i].OccurrenceIdentity < out[j].OccurrenceIdentity
		}
		return out[i].EndpointRole < out[j].EndpointRole
	})
	return out, nil
}
func artifactFromDocument(d document, max int) (Artifact, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(d); err != nil {
		return Artifact{}, err
	}
	if max <= 0 || b.Len() > max {
		return Artifact{}, errors.New("canonical byte bound exceeded")
	}
	h := sha256.Sum256(b.Bytes())
	return Artifact{canonical: append([]byte(nil), b.Bytes()...), digest: fmt.Sprintf("sha256:%x", h), doc: d}, nil
}
func scanJSON(raw []byte, limits ParseLimits) error {
	if limits.MaxArtifacts < 0 || limits.MaxNodes < 0 || limits.MaxOccurrences < 0 || limits.MaxContributions < 0 || limits.MaxNodes > int(^uint(0)>>1)/2 {
		return errors.New("invalid array limits")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func(json.Token, string) error
	walk = func(tok json.Token, field string) error {
		switch v := tok.(type) {
		case string:
			if len(v) > limits.MaxStringBytes {
				return errors.New("string bound exceeded")
			}
		case json.Number:
			s := string(v)
			if !validInteger(s) {
				return errors.New("invalid numeric form")
			}
			if _, err := strconv.ParseUint(s, 10, 63); err != nil {
				return errors.New("integer overflow")
			}
		case json.Delim:
			switch v {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					kt, err := dec.Token()
					if err != nil {
						return err
					}
					k, ok := kt.(string)
					if !ok || len(k) > limits.MaxStringBytes || seen[k] {
						return errors.New("duplicate or invalid key")
					}
					seen[k] = true
					t, err := dec.Token()
					if err != nil {
						return err
					}
					if err = walk(t, k); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			case '[':
				limit, ok := arrayLimit(field, limits)
				if !ok {
					return errors.New("unexpected array")
				}
				n := 0
				for dec.More() {
					n++
					if n > limit {
						return errors.New("array bound exceeded")
					}
					t, err := dec.Token()
					if err != nil {
						return err
					}
					if err = walk(t, ""); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			}
		}
		return nil
	}
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if err = walk(t, ""); err != nil {
		return err
	}
	if _, err = dec.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func arrayLimit(field string, l ParseLimits) (int, bool) {
	switch field {
	case "eligible_kinds":
		return 3, true
	case "predecessors":
		return l.MaxArtifacts, true
	case "endpoint_custody":
		return 2 * l.MaxNodes, true
	case "contributions":
		return l.MaxContributions, true
	case "candidate_outcomes":
		return l.MaxOccurrences, true
	default:
		return 0, false
	}
}

type occurrenceLedgerRow struct {
	Identity   string `json:"identity"`
	Kind       string `json:"kind"`
	From       string `json:"from"`
	To         string `json:"to"`
	Ordinal    int    `json:"ordinal"`
	RelationID string `json:"relation_id"`
}

type occurrenceLedgerDocument struct {
	Domain      string                `json:"domain"`
	Version     string                `json:"version"`
	Claim       string                `json:"claim"`
	Occurrences []occurrenceLedgerRow `json:"occurrences"`
}

func canonicalOccurrenceLedger(occurrences []programcadmission.TypedOccurrence) ([]byte, error) {
	rows := make([]occurrenceLedgerRow, len(occurrences))
	for i, occurrence := range occurrences {
		rows[i] = occurrenceLedgerRow{occurrence.Identity, string(occurrence.Kind), occurrence.From, occurrence.To, occurrence.Ordinal, occurrence.RelationID}
	}
	raw, err := json.Marshal(occurrenceLedgerDocument{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func validateSuppliedOccurrenceLedger(d document, raw []byte) error {
	var ledger occurrenceLedgerDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ledger); err != nil || requireEOF(decoder) != nil {
		return errors.New("invalid supplied occurrence ledger")
	}
	encoded, err := json.Marshal(ledger)
	if err != nil || !bytes.Equal(append(encoded, '\n'), raw) || ledger.Domain != "PROGRAM_C_TYPED_OCCURRENCES" || ledger.Version != "v1" || ledger.Claim != "ADMITTED_TYPED_OCCURRENCES_EXACT" || len(ledger.Occurrences) != len(d.Contributions) {
		return errors.New("invalid supplied occurrence ledger")
	}
	for i, contribution := range d.Contributions {
		row := ledger.Occurrences[i]
		if row.Identity != contribution.OccurrenceIdentity || row.Kind != contribution.Kind || row.From != contribution.From || row.To != contribution.To || row.Ordinal != contribution.Ordinal {
			return errors.New("occurrence ledger contribution mismatch")
		}
		if (row.Kind == "CALLS") != (row.RelationID != "") {
			return errors.New("occurrence ledger relation identity mismatch")
		}
	}
	return nil
}

func validateBoundPredecessors(predecessors []PredecessorRecord, methods []programcadmission.MethodOccurrenceEvidence, methodCanonical []byte, source programcadmission.CompositeSourceBinding, retained []byte) error {
	byRole := make(map[string]PredecessorRecord, len(predecessors))
	for _, predecessor := range predecessors {
		byRole[predecessor.Role] = predecessor
	}
	if len(retained) != 0 {
		composite := byRole["RETAINED_COMPOSITE"]
		if !bytes.Equal(composite.Bytes, retained) {
			return errors.New("retained composite bytes mismatch")
		}
		binding, err := json.Marshal(source)
		if err != nil {
			return err
		}
		binding = append(binding, '\n')
		if !bytes.Equal(byRole["ACQUISITION_RECEIPT"].Bytes, binding) {
			return errors.New("acquisition source binding mismatch")
		}
	}
	if len(methods) != 0 && !bytes.Equal(byRole["METHOD_EVIDENCE"].Bytes, methodCanonical) {
		return errors.New("method canonical bytes mismatch")
	}
	for _, method := range methods {
		query := byRole["METHOD_QUERY"]
		evidence := byRole["METHOD_EVIDENCE"]
		queryDigest := sha256.Sum256(query.Bytes)
		if method.QuerySelector != query.Selector || method.QueryLength != len(query.Bytes) || method.QueryDigest != fmt.Sprintf("sha256:%x", queryDigest) || method.EvidenceSelector != evidence.Selector {
			return errors.New("method predecessor bytes mismatch")
		}
	}
	if len(methods) != 0 {
		mappings, err := parseMethodMappingReceipt(byRole["METHOD_MAPPING_RECEIPT"].Bytes, len(methods))
		if err != nil {
			return err
		}
		byIdentity := make(map[string]programcadmission.MethodOccurrenceEvidence, len(methods))
		for _, method := range methods {
			byIdentity[method.Identity] = method
		}
		if len(mappings) != len(byIdentity) {
			return errors.New("method mapping receipt count mismatch")
		}
		for _, mapping := range mappings {
			method, ok := byIdentity[mapping.Identity]
			if !ok || string(method.Kind) != mapping.Kind || method.From != mapping.From || method.To != mapping.To || method.Ordinal != mapping.Ordinal || method.QuerySelector != mapping.QuerySelector || method.EvidenceSelector != mapping.EvidenceSelector {
				return errors.New("method mapping receipt mismatch")
			}
		}
	}
	return nil
}

type admittedMethodOccurrenceMapping struct {
	Identity         string `json:"identity"`
	Kind             string `json:"kind"`
	From             string `json:"from"`
	To               string `json:"to"`
	Ordinal          int    `json:"ordinal"`
	QuerySelector    string `json:"query_selector"`
	EvidenceSelector string `json:"evidence_selector"`
}

type admittedMethodMappingReceipt struct {
	Family   string                            `json:"family"`
	Version  string                            `json:"version"`
	Mappings []admittedMethodOccurrenceMapping `json:"mappings"`
}

func replaySuppliedPredecessors(d document, supplied map[string]PredecessorRecord) error {
	replayed := make([]programcadmission.TypedOccurrence, 0, len(d.Contributions))
	if composite, ok := supplied["calls/composite.json"]; ok {
		admitted, err := programcadmission.Admit(composite.Bytes)
		if err != nil {
			return errors.New("invalid retained composite")
		}
		binding, err := json.Marshal(admitted.Admission.SourceBinding())
		if err != nil {
			return err
		}
		if receipt, exists := supplied["calls/acquisition.json"]; !exists || !bytes.Equal(receipt.Bytes, append(binding, '\n')) {
			return errors.New("acquisition source binding mismatch")
		}
		nodes := admitted.Admission.NodeIdentities()
		for _, occurrence := range admitted.Admission.Occurrences() {
			if occurrence.From < 0 || occurrence.To < 0 || int(occurrence.From) >= len(nodes) || int(occurrence.To) >= len(nodes) {
				return errors.New("invalid replayed calls occurrence")
			}
			replayed = append(replayed, programcadmission.TypedOccurrence{Identity: occurrence.Identity, RelationID: occurrence.RelationID, From: nodes[occurrence.From], To: nodes[occurrence.To], Kind: programcadmission.TypedCalls, Ordinal: occurrence.Ordinal})
		}
	}
	if evidence, ok := supplied["methods/evidence.json"]; ok {
		methods, err := admitCanonicalMethodEvidence(evidence.Bytes)
		if err != nil {
			return err
		}
		receipt, exists := supplied["methods/mapping-receipt.json"]
		if !exists {
			return errors.New("missing method mapping receipt")
		}
		mappings, err := parseMethodMappingReceipt(receipt.Bytes, len(d.Contributions))
		if err != nil {
			return err
		}
		byIdentity := make(map[string]programcadmission.MethodOccurrenceEvidence, len(methods.Occurrences()))
		for _, occurrence := range methods.Occurrences() {
			byIdentity[occurrence.Identity] = occurrence
		}
		query := supplied["methods/query.json"]
		queryDigest := sha256.Sum256(query.Bytes)
		for _, mapping := range mappings {
			occurrence, exists := byIdentity[mapping.Identity]
			if !exists || string(occurrence.Kind) != mapping.Kind || occurrence.From != mapping.From || occurrence.To != mapping.To || occurrence.Ordinal != mapping.Ordinal || occurrence.QuerySelector != mapping.QuerySelector || occurrence.EvidenceSelector != mapping.EvidenceSelector || occurrence.QuerySelector != query.Selector || occurrence.QueryLength != len(query.Bytes) || occurrence.QueryDigest != fmt.Sprintf("sha256:%x", queryDigest) || occurrence.EvidenceSelector != evidence.Selector {
				return errors.New("method mapping receipt mismatch")
			}
			replayed = append(replayed, programcadmission.TypedOccurrence{Identity: occurrence.Identity, From: occurrence.From, To: occurrence.To, Kind: occurrence.Kind, Ordinal: occurrence.Ordinal})
		}
	}
	sort.Slice(replayed, func(i, j int) bool { return replayed[i].Identity < replayed[j].Identity })
	if len(replayed) != len(d.Contributions) {
		return errors.New("replayed occurrence count mismatch")
	}
	for i, occurrence := range replayed {
		contribution := d.Contributions[i]
		if occurrence.Identity != contribution.OccurrenceIdentity || string(occurrence.Kind) != contribution.Kind || occurrence.From != contribution.From || occurrence.To != contribution.To || occurrence.Ordinal != contribution.Ordinal {
			return errors.New("replayed contribution mismatch")
		}
	}
	ledger, err := canonicalOccurrenceLedger(replayed)
	if err != nil || !bytes.Equal(ledger, supplied["ledgers/occurrences.json"].Bytes) {
		return errors.New("replayed occurrence ledger mismatch")
	}
	expectedCustody, err := custodyFromReplayed(d.Contributions)
	if err != nil || !reflect.DeepEqual(expectedCustody, d.EndpointCustody) {
		return errors.New("replayed custody mismatch")
	}
	return nil
}

func admitCanonicalMethodEvidence(raw []byte) (programcadmission.MethodProjectionAdmission, error) {
	if len(raw) == 0 || len(raw) > programcadmission.MaxMethodCanonicalBytes {
		return programcadmission.MethodProjectionAdmission{}, errors.New("invalid method canonical bytes")
	}
	var envelope programcadmission.UntrustedMethodEvidenceEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || requireEOF(decoder) != nil {
		return programcadmission.MethodProjectionAdmission{}, errors.New("invalid method canonical bytes")
	}
	admitted, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil || !bytes.Equal(admitted.CanonicalBytes(), raw) {
		return programcadmission.MethodProjectionAdmission{}, errors.New("invalid method canonical bytes")
	}
	return admitted, nil
}

func parseMethodMappingReceipt(raw []byte, limit int) ([]admittedMethodOccurrenceMapping, error) {
	if len(raw) == 0 || limit < 0 {
		return nil, errors.New("invalid method mapping receipt")
	}
	var receipt admittedMethodMappingReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil || requireEOF(decoder) != nil || receipt.Family != "METHOD_MAPPING_RECEIPT" || receipt.Version != "v1" || len(receipt.Mappings) > limit {
		return nil, errors.New("invalid method mapping receipt")
	}
	for i, mapping := range receipt.Mappings {
		if mapping.Identity == "" || mapping.From == "" || mapping.To == "" || mapping.Ordinal < 0 || mapping.QuerySelector == "" || mapping.EvidenceSelector == "" || (mapping.Kind != string(programcadmission.TypedReferencesSymbol) && mapping.Kind != string(programcadmission.TypedResolvesToDefinition)) || (i > 0 && receipt.Mappings[i-1].Identity >= mapping.Identity) {
			return nil, errors.New("invalid method mapping receipt")
		}
	}
	canonical, err := json.Marshal(receipt)
	if err != nil || !bytes.Equal(append(canonical, '\n'), raw) {
		return nil, errors.New("noncanonical method mapping receipt")
	}
	return receipt.Mappings, nil
}

func custodyFromReplayed(contributions []contributionJSON) ([]custodyJSON, error) {
	out := make([]custodyJSON, 0, 2*len(contributions))
	for _, contribution := range contributions {
		selector, sourceRole, targetRole := "", "", ""
		switch contribution.Kind {
		case string(programcadmission.TypedCalls):
			selector, sourceRole, targetRole = contribution.AcquisitionReceiptSelector, "CALLER_OCCURRENCE", "CALLEE_TARGET"
		case string(programcadmission.TypedReferencesSymbol):
			selector, sourceRole, targetRole = contribution.MethodEvidenceSelector, "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
		case string(programcadmission.TypedResolvesToDefinition):
			selector, sourceRole, targetRole = contribution.MethodEvidenceSelector, "QUERY_OCCURRENCE", "DEFINITION_TARGET"
		default:
			return nil, errors.New("invalid replayed contribution kind")
		}
		out = append(out, custodyJSON{contribution.From, contribution.OccurrenceIdentity, sourceRole, selector, "CALLER_ASSERTED"}, custodyJSON{contribution.To, contribution.OccurrenceIdentity, targetRole, selector, "CALLER_ASSERTED"})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Endpoint != out[j].Endpoint {
			return out[i].Endpoint < out[j].Endpoint
		}
		if out[i].OccurrenceIdentity != out[j].OccurrenceIdentity {
			return out[i].OccurrenceIdentity < out[j].OccurrenceIdentity
		}
		return out[i].EndpointRole < out[j].EndpointRole
	})
	return out, nil
}

func arraysCanonical(d document) bool {
	for i := 1; i < len(d.Predecessors); i++ {
		a, b := d.Predecessors[i-1], d.Predecessors[i]
		if a.Role > b.Role || (a.Role == b.Role && a.Selector >= b.Selector) {
			return false
		}
	}
	for i := 1; i < len(d.EndpointCustody); i++ {
		a, b := d.EndpointCustody[i-1], d.EndpointCustody[i]
		if a.Endpoint > b.Endpoint || (a.Endpoint == b.Endpoint && (a.OccurrenceIdentity > b.OccurrenceIdentity || (a.OccurrenceIdentity == b.OccurrenceIdentity && a.EndpointRole >= b.EndpointRole))) {
			return false
		}
	}
	for i := 1; i < len(d.Contributions); i++ {
		a, b := d.Contributions[i-1], d.Contributions[i]
		if contributionLess(b, a) || !contributionLess(a, b) {
			return false
		}
	}
	for i := 1; i < len(d.CandidateOutcomes); i++ {
		if d.CandidateOutcomes[i-1].OccurrenceIdentity >= d.CandidateOutcomes[i].OccurrenceIdentity {
			return false
		}
	}
	return true
}

func contributionLess(a, b contributionJSON) bool {
	return a.OccurrenceIdentity < b.OccurrenceIdentity
}

func maxDocumentStringBytes(doc document) int {
	max := 0
	var visit func(reflect.Value)
	visit = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.String:
			if value.Len() > max {
				max = value.Len()
			}
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				visit(value.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				visit(value.Index(i))
			}
		}
	}
	visit(reflect.ValueOf(doc))
	return max
}

func uniqueCustodyEndpoints(custody []custodyJSON) int {
	endpoints := make(map[string]struct{}, len(custody))
	for _, entry := range custody {
		endpoints[entry.Endpoint] = struct{}{}
	}
	return len(endpoints)
}

func validInteger(s string) bool {
	if s == "0" {
		return true
	}
	if len(s) == 0 || s[0] < '1' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
func requireEOF(dec *json.Decoder) error {
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func predecessorByRole(ps []PredecessorRecord, role string) *PredecessorRecord {
	for i := range ps {
		if ps[i].Role == role {
			return &ps[i]
		}
	}
	return nil
}
func cloneArtifact(a Artifact) Artifact {
	a.canonical = append([]byte(nil), a.canonical...)
	a.doc.Predecessors = append([]predecessorJSON(nil), a.doc.Predecessors...)
	a.doc.EndpointCustody = append([]custodyJSON(nil), a.doc.EndpointCustody...)
	a.doc.Contributions = append([]contributionJSON(nil), a.doc.Contributions...)
	a.doc.CandidateOutcomes = append([]outcomeJSON(nil), a.doc.CandidateOutcomes...)
	a.doc.EligibleKinds = append([]string(nil), a.doc.EligibleKinds...)
	return a
}
func publicContributions(in []contributionJSON) []Contribution {
	out := make([]Contribution, len(in))
	for i, c := range in {
		out[i] = Contribution{Kind: c.Kind, From: c.From, To: c.To, Ordinal: c.Ordinal, OccurrenceIdentity: c.OccurrenceIdentity, MethodEvidenceSelector: c.MethodEvidenceSelector, QuerySelector: c.QuerySelector, EvidenceSelector: c.EvidenceSelector, MappingReceiptSelector: c.MappingReceiptSelector, AcquisitionReceiptSelector: c.AcquisitionReceiptSelector, RetainedCompositeSelector: c.RetainedCompositeSelector}
	}
	return out
}
func publicCustody(in []custodyJSON) []EndpointCustody {
	out := make([]EndpointCustody, len(in))
	for i, c := range in {
		out[i] = EndpointCustody{c.Endpoint, c.OccurrenceIdentity, c.EndpointRole, c.PredecessorSelector, c.Custody}
	}
	return out
}
func publicOutcomes(in []outcomeJSON) []CandidateOutcome {
	out := make([]CandidateOutcome, len(in))
	for i, o := range in {
		out[i] = CandidateOutcome{o.OccurrenceIdentity, o.Disposition}
	}
	return out
}
func publicAccounting(a accountingJSON) Accounting {
	return Accounting{a.Candidates, a.Included, a.Excluded, a.Duplicates, a.Terminal, a.Truncated}
}
