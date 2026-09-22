// Package provisionalfeaturecatalog builds correction-safe, authority-zero review catalogs.
package provisionalfeaturecatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/targetpacket"
)

const (
	SchemaVersion       = "lsp-trace.provisional-feature-catalog.v1"
	StatusProvisional   = "PROVISIONAL"
	CompletenessUnknown = "UNKNOWN"
)

type CatalogOutcome string

const (
	OutcomeComplete CatalogOutcome = "COMPLETE"
	OutcomeDegraded CatalogOutcome = "DEGRADED"
)

type TerminalOutcome string

const (
	TerminalComplete          TerminalOutcome = "COMPLETE"
	TerminalAbstained         TerminalOutcome = "ABSTAINED"
	TerminalInvalidInput      TerminalOutcome = "INVALID_INPUT"
	TerminalModelUnavailable  TerminalOutcome = "MODEL_UNAVAILABLE"
	TerminalContextLimit      TerminalOutcome = "CONTEXT_LIMIT"
	TerminalOutputInvalid     TerminalOutcome = "OUTPUT_INVALID"
	TerminalTimeout           TerminalOutcome = "TIMEOUT"
	TerminalCancelled         TerminalOutcome = "CANCELLED"
	TerminalResourceLimit     TerminalOutcome = "RESOURCE_LIMIT"
	TerminalBackendFailure    TerminalOutcome = "BACKEND_FAILURE"
	TerminalPolicyMismatch    TerminalOutcome = "POLICY_MISMATCH"
	TerminalDuplicateInput    TerminalOutcome = "DUPLICATE_INPUT"
	TerminalSourceUnavailable TerminalOutcome = "SOURCE_UNAVAILABLE"
)

type Limitation struct {
	Text string `json:"text"`
}
type Citation struct {
	Text string `json:"text"`
}
type Semantic struct {
	Verdict                string       `json:"verdict"`
	TargetRole             string       `json:"target_role"`
	TargetRoleIsModelText  bool         `json:"target_role_is_model_text"`
	NearestOutwardConsumer string       `json:"nearest_outward_consumer"`
	ConsumerNeed           string       `json:"consumer_need"`
	ProvidedBehavior       string       `json:"provided_behavior"`
	BoundaryContribution   string       `json:"boundary_contribution"`
	Limitations            []Limitation `json:"limitations"`
	Citations              []Citation   `json:"citations"`
}
type Lineage struct {
	CensusID                      string   `json:"census_id"`
	BatchID                       string   `json:"batch_id"`
	CommunityIdentity             string   `json:"community_identity"`
	ConstituentIdentity           string   `json:"constituent_identity"`
	ConstituentOrdinal            int      `json:"constituent_ordinal"`
	ExecutionBundleID             string   `json:"execution_bundle_id"`
	SeedLabel                     string   `json:"seed_label"`
	SeedAt                        string   `json:"seed_at"`
	Members                       []string `json:"members"`
	PreparedTargets               []string `json:"prepared_targets"`
	SCCMembers                    []string `json:"scc_members"`
	SelectedNode                  string   `json:"selected_node"`
	SelectedLogicalSourceID       string   `json:"selected_logical_source_id"`
	PacketID                      string   `json:"packet_id"`
	PacketCustodyGraphDigest      string   `json:"packet_custody_graph_digest"`
	PacketCustodyCaptureID        string   `json:"packet_custody_capture_id"`
	ConsumerResolution            string   `json:"consumer_resolution"`
	AlternativeID                 string   `json:"alternative_id"`
	AlternativeOrdinal            int      `json:"alternative_ordinal"`
	ConsumerRelationID            string   `json:"consumer_relation_id"`
	ConsumerOccurrenceID          string   `json:"consumer_occurrence_id"`
	ConsumerCallerID              string   `json:"consumer_caller_id"`
	ConsumerTargetID              string   `json:"consumer_target_id"`
	ConsumerCallerLogicalSourceID string   `json:"consumer_caller_logical_source_id"`
	RequestRecordID               string   `json:"request_record_id"`
	RequestMessageID              string   `json:"request_message_id"`
	RequestLineageIdentity        string   `json:"request_lineage_identity"`
	InvocationID                  string   `json:"invocation_id"`
	ResponseID                    string   `json:"response_id"`
}
type Entry struct {
	EntryID          string                    `json:"entry_id"`
	Status           string                    `json:"status"`
	Authority        int                       `json:"authority"`
	Accepted         bool                      `json:"accepted"`
	Completeness     string                    `json:"completeness"`
	DisplayLabel     string                    `json:"display_label"`
	LineageBindingID string                    `json:"lineage_binding_id"`
	Lineage          Lineage                   `json:"lineage"`
	TerminalOutcome  TerminalOutcome           `json:"terminal_outcome"`
	Semantic         Semantic                  `json:"semantic"`
	Failure          *PreparationFailureMember `json:"preparation_failure,omitempty"`
}

type PreparationFailureMember struct {
	FailureID      string   `json:"failure_id"`
	NominationID   string   `json:"nomination_id"`
	PacketIntentID string   `json:"packet_intent_id"`
	Role           string   `json:"role"`
	Code           string   `json:"code"`
	EvidenceIDs    []string `json:"evidence_ids"`
}

type PreparationFailureInput = PreparationFailureMember
type Accounting struct {
	Total            int `json:"total"`
	Complete         int `json:"complete"`
	Abstained        int `json:"abstained"`
	InvalidInput     int `json:"invalid_input"`
	ModelUnavailable int `json:"model_unavailable"`
	ContextLimit     int `json:"context_limit"`
	OutputInvalid    int `json:"output_invalid"`
	Timeout          int `json:"timeout"`
	Cancelled        int `json:"cancelled"`
	ResourceLimit    int `json:"resource_limit"`
	BackendFailure   int `json:"backend_failure"`
	PolicyMismatch   int `json:"policy_mismatch"`
	DuplicateInput   int `json:"duplicate_input"`
	NominationTotal  int `json:"nomination_total,omitempty"`
	RequestTotal     int `json:"request_total,omitempty"`
	DescribedEntries int `json:"described_entries,omitempty"`
	PreRequestFailed int `json:"pre_request_failed,omitempty"`
}
type ActorAuthority string

const (
	AuthorityStakeholder ActorAuthority = "STAKEHOLDER"
	AuthorityReviewer    ActorAuthority = "REVIEWER"
)

type DeltaKind string

const (
	DeltaReviewState     DeltaKind = "REVIEW_STATE"
	DeltaInventoryState  DeltaKind = "INVENTORY_STATE"
	DeltaCommunityRename DeltaKind = "COMMUNITY_RENAME"
	DeltaCommunityMerge  DeltaKind = "COMMUNITY_MERGE"
)

type InventoryStateDelta struct {
	Kind DeltaKind `json:"kind"`
	From string    `json:"from"`
	To   string    `json:"to"`
}
type RebuildDisposition string

const (
	RebuildRequired RebuildDisposition = "REBUILD_REQUIRED"
	RebuildDeferred RebuildDisposition = "REBUILD_DEFERRED"
)

type Correction struct {
	CorrectionID            string                `json:"correction_id"`
	PredecessorCatalogID    string                `json:"predecessor_catalog_id"`
	PredecessorEntryIDs     []string              `json:"predecessor_entry_ids"`
	SupersedesCorrectionIDs []string              `json:"supersedes_correction_ids"`
	SupersedesEntryIDs      []string              `json:"supersedes_entry_ids"`
	ActorID                 string                `json:"actor_id"`
	ActorAuthority          ActorAuthority        `json:"actor_authority"`
	Reason                  string                `json:"reason"`
	InventoryStateDeltas    []InventoryStateDelta `json:"inventory_state_deltas"`
	AffectedIDs             []string              `json:"affected_ids"`
	RebuildDisposition      RebuildDisposition    `json:"rebuild_disposition"`
}
type CorrectionInput struct {
	SupersedesCorrectionIDs []string
	SupersedesEntryIDs      []string
	ActorID                 string
	ActorAuthority          ActorAuthority
	Reason                  string
	InventoryStateDeltas    []InventoryStateDelta
	AffectedEntryIDs        []string
	RebuildDisposition      RebuildDisposition
}
type CatalogWire struct {
	SchemaVersion        string         `json:"schema_version"`
	CatalogID            string         `json:"catalog_id"`
	PredecessorCatalogID string         `json:"predecessor_catalog_id"`
	Status               string         `json:"status"`
	Authority            int            `json:"authority"`
	Accepted             bool           `json:"accepted"`
	Completeness         string         `json:"completeness"`
	Outcome              CatalogOutcome `json:"outcome"`
	Entries              []Entry        `json:"entries"`
	Accounting           Accounting     `json:"accounting"`
	Corrections          []Correction   `json:"corrections"`
}
type Catalog struct{ wire CatalogWire }

func (c Catalog) ID() string                   { return c.wire.CatalogID }
func (c Catalog) Outcome() CatalogOutcome      { return c.wire.Outcome }
func (c Catalog) Accounting() Accounting       { return c.wire.Accounting }
func (c Catalog) PredecessorCatalogID() string { return c.wire.PredecessorCatalogID }
func (c Catalog) Entries() []Entry             { return cloneEntries(c.wire.Entries) }
func (c Catalog) Corrections() []Correction    { return cloneCorrections(c.wire.Corrections) }
func (c Catalog) Bytes() ([]byte, error) {
	if err := Validate(c); err != nil {
		return nil, err
	}
	return json.Marshal(c.wire)
}

func Build(packets targetpacket.Result, requests []describerequest.Record, invocations []describeworker.InvocationRecord, responses []describeworker.ResponseRecord) (Catalog, error) {
	if packets.State == targetpacket.StateEmpty && len(packets.Packets) == 0 && len(requests) == 0 && len(invocations) == 0 && len(responses) == 0 {
		return finish(CatalogWire{SchemaVersion: SchemaVersion, Status: StatusProvisional, Completeness: CompletenessUnknown, Outcome: OutcomeComplete, Entries: []Entry{}, Corrections: []Correction{}})
	}
	if packets.State != targetpacket.StatePrepared {
		return Catalog{}, errors.New("validated prepared packets required")
	}
	packetByID := map[string]targetpacket.Packet{}
	for _, p := range packets.Packets {
		raw, err := targetpacket.EncodeCanonical(p)
		if err != nil {
			return Catalog{}, err
		}
		valid, err := targetpacket.Validate(raw)
		if err != nil {
			return Catalog{}, err
		}
		if _, ok := packetByID[valid.PacketID]; ok {
			return Catalog{}, errors.New("duplicate packet")
		}
		packetByID[valid.PacketID] = valid
	}
	orderedRequests := append([]describerequest.Record(nil), requests...)
	sort.Slice(orderedRequests, func(i, j int) bool {
		if orderedRequests[i].Lineage.PacketID != orderedRequests[j].Lineage.PacketID {
			return orderedRequests[i].Lineage.PacketID < orderedRequests[j].Lineage.PacketID
		}
		return orderedRequests[i].Lineage.AlternativeOrdinal < orderedRequests[j].Lineage.AlternativeOrdinal
	})
	if err := describerequest.Validate(orderedRequests); err != nil {
		return Catalog{}, err
	}
	requestByID := map[string]describerequest.Record{}
	for _, r := range orderedRequests {
		if _, ok := requestByID[r.RecordID]; ok {
			return Catalog{}, errors.New("duplicate request")
		}
		requestByID[r.RecordID] = r
	}
	invByRequest := map[string]describeworker.InvocationRecord{}
	for _, v := range invocations {
		if err := v.Validate(); err != nil {
			return Catalog{}, err
		}
		b := v.Binding()
		if _, ok := invByRequest[b.RequestRecordID]; ok {
			return Catalog{}, errors.New("duplicate invocation")
		}
		invByRequest[b.RequestRecordID] = v
	}
	respByRequest := map[string]describeworker.ResponseRecord{}
	for _, v := range responses {
		if err := v.Validate(); err != nil {
			return Catalog{}, err
		}
		b := v.Binding()
		if _, ok := respByRequest[b.RequestRecordID]; ok {
			return Catalog{}, errors.New("duplicate response")
		}
		respByRequest[b.RequestRecordID] = v
	}
	expected := 0
	for _, p := range packetByID {
		if len(p.ConsumerAlternatives) == 0 {
			expected++
		} else {
			expected += len(p.ConsumerAlternatives)
		}
	}
	successful := 0
	for _, inv := range invByRequest {
		if inv.Status() == describeworker.StatusSucceeded {
			successful++
		}
	}
	if len(requestByID) != expected || len(invByRequest) != expected || len(respByRequest) != successful {
		return Catalog{}, errors.New("join cardinality mismatch")
	}
	entries := make([]Entry, 0, expected)
	for _, r := range requestByID {
		p, ok := packetByID[r.Lineage.PacketID]
		if !ok {
			return Catalog{}, errors.New("foreign packet")
		}
		if r.Lineage.CensusID != p.CensusID || r.Lineage.ConsumerResolution != string(p.ConsumerResolution) {
			return Catalog{}, errors.New("request packet mismatch")
		}
		var alt *targetpacket.ConsumerAlternative
		if len(p.ConsumerAlternatives) == 0 {
			if r.Lineage.AlternativeID != "OUTWARD_CONSUMER_UNRESOLVED" || r.Lineage.AlternativeOrdinal != 0 {
				return Catalog{}, errors.New("unresolved alternative mismatch")
			}
		} else {
			if r.Lineage.AlternativeOrdinal < 0 || r.Lineage.AlternativeOrdinal >= len(p.ConsumerAlternatives) {
				return Catalog{}, errors.New("alternative ordinal mismatch")
			}
			a := p.ConsumerAlternatives[r.Lineage.AlternativeOrdinal]
			if a.ReconciliationID != r.Lineage.AlternativeID {
				return Catalog{}, errors.New("alternative id mismatch")
			}
			alt = &a
		}
		inv, ok := invByRequest[r.RecordID]
		if !ok {
			return Catalog{}, errors.New("missing invocation")
		}
		resp, hasResponse := respByRequest[r.RecordID]
		ib := inv.Binding()
		if ib.RequestRecordID != r.RecordID || ib.MessageID != r.Envelope.MessageID {
			return Catalog{}, errors.New("binding mismatch")
		}
		if inv.Status() == describeworker.StatusSucceeded {
			if !hasResponse {
				return Catalog{}, errors.New("missing response")
			}
			rb := resp.Binding()
			if rb.RequestRecordID != r.RecordID || rb.MessageID != r.Envelope.MessageID || inv.Response().ID() != resp.ID() {
				return Catalog{}, errors.New("binding mismatch")
			}
		} else if hasResponse || inv.Response().ID() != "" {
			return Catalog{}, errors.New("unexpected failure response")
		}
		e := makeEntry(p, r, alt, inv, resp)
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entryKey(entries[i]) < entryKey(entries[j]) })
	w := CatalogWire{SchemaVersion: SchemaVersion, Status: StatusProvisional, Completeness: CompletenessUnknown, Outcome: OutcomeComplete, Entries: entries, Corrections: []Correction{}}
	w.Accounting = account(entries)
	if w.Accounting.Total != len(entries) || w.Accounting.Complete != w.Accounting.Total {
		w.Outcome = OutcomeDegraded
	}
	return finish(w)
}

func BuildWithPreparationFailures(nominationTotal int, failures []PreparationFailureInput, packets targetpacket.Result, requests []describerequest.Record, invocations []describeworker.InvocationRecord, responses []describeworker.ResponseRecord) (Catalog, error) {
	failedNominations := map[string]bool{}
	for _, failure := range failures {
		failedNominations[failure.NominationID] = true
	}
	if nominationTotal < 0 || nominationTotal != len(packets.Packets)+len(failedNominations) {
		return Catalog{}, errors.New("nomination accounting mismatch")
	}
	base, err := Build(packets, requests, invocations, responses)
	if err != nil {
		return Catalog{}, err
	}
	w := base.wire
	seen := map[string]bool{}
	for _, f := range failures {
		if !validFailureInput(f) || seen[f.FailureID] {
			return Catalog{}, errors.New("preparation failure mismatch")
		}
		seen[f.FailureID] = true
		member := f
		member.EvidenceIDs = append([]string(nil), f.EvidenceIDs...)
		sort.Strings(member.EvidenceIDs)
		e := Entry{Status: StatusProvisional, Completeness: CompletenessUnknown, DisplayLabel: "nomination " + f.NominationID + " / SOURCE_UNAVAILABLE", TerminalOutcome: TerminalSourceUnavailable, Semantic: Semantic{Limitations: []Limitation{}, Citations: []Citation{}}, Failure: &member}
		e.LineageBindingID = hash("lsp-trace.provisional-feature-lineage.v1", e.Lineage)
		e.EntryID = entryID(e)
		w.Entries = append(w.Entries, e)
	}
	sort.Slice(w.Entries, func(i, j int) bool { return entryKey(w.Entries[i]) < entryKey(w.Entries[j]) })
	w.Accounting = account(w.Entries)
	w.Accounting.NominationTotal = nominationTotal
	w.Accounting.RequestTotal = len(requests)
	w.Accounting.DescribedEntries = len(requests)
	w.Accounting.PreRequestFailed = len(failures)
	if len(failures) > 0 {
		w.Outcome = OutcomeDegraded
	}
	return finish(w)
}

func validFailureInput(f PreparationFailureInput) bool {
	if f.FailureID == "" || f.NominationID == "" || f.PacketIntentID == "" || (f.Role != "TARGET" && f.Role != "CALLER") || f.Code != "EXACT_ENDPOINT_SOURCE_UNAVAILABLE" || len(f.EvidenceIDs) == 0 {
		return false
	}
	for _, id := range f.EvidenceIDs {
		if id == "" {
			return false
		}
	}
	return true
}

func makeEntry(p targetpacket.Packet, r describerequest.Record, a *targetpacket.ConsumerAlternative, inv describeworker.InvocationRecord, resp describeworker.ResponseRecord) Entry {
	l := Lineage{CensusID: p.Lineage.CensusID, BatchID: p.Lineage.BatchID, CommunityIdentity: p.Lineage.CommunityIdentity, ConstituentIdentity: p.Lineage.ConstituentIdentity, ConstituentOrdinal: p.Lineage.ConstituentOrdinal, ExecutionBundleID: p.Lineage.ExecutionBundleID, SeedLabel: p.Lineage.SeedLabel, SeedAt: p.Lineage.SeedAt, Members: append([]string(nil), p.Lineage.Members...), PreparedTargets: append([]string(nil), p.Lineage.PreparedTargets...), SCCMembers: append([]string(nil), p.Lineage.SCCMembers...), SelectedNode: p.Lineage.SelectedNode, SelectedLogicalSourceID: p.Lineage.SelectedLogicalSourceID, PacketID: p.PacketID, PacketCustodyGraphDigest: p.Custody.GraphDigest, PacketCustodyCaptureID: p.Custody.CaptureID, ConsumerResolution: r.Lineage.ConsumerResolution, AlternativeID: r.Lineage.AlternativeID, AlternativeOrdinal: r.Lineage.AlternativeOrdinal, RequestRecordID: r.RecordID, RequestMessageID: r.Envelope.MessageID, RequestLineageIdentity: r.Lineage.LineageIdentity, InvocationID: inv.ID(), ResponseID: resp.ID()}
	if a != nil {
		l.ConsumerRelationID = a.RelationID
		l.ConsumerOccurrenceID = a.OccurrenceID
		l.ConsumerCallerID = a.CallerID
		l.ConsumerTargetID = a.TargetID
		l.ConsumerCallerLogicalSourceID = a.CallerLogicalSourceID
	}
	s := resp.Response()
	sem := Semantic{Limitations: []Limitation{}, Citations: []Citation{}}
	if resp.ID() != "" {
		sem = Semantic{Verdict: s.Verdict, TargetRole: s.TargetRole, TargetRoleIsModelText: true, NearestOutwardConsumer: s.NearestOutwardConsumer, ConsumerNeed: s.ConsumerNeed, ProvidedBehavior: s.ProvidedBehavior, BoundaryContribution: s.BoundaryContribution, Limitations: make([]Limitation, len(s.Limitations)), Citations: make([]Citation, len(s.Citations))}
		for i, v := range s.Limitations {
			sem.Limitations[i] = Limitation{v}
		}
		for i, v := range s.Citations {
			sem.Citations[i] = Citation{v}
		}
	}
	e := Entry{Status: StatusProvisional, Completeness: CompletenessUnknown, DisplayLabel: "community " + l.CommunityIdentity + " / packet " + l.PacketID + " / alternative " + l.AlternativeID, Lineage: l, TerminalOutcome: terminal(inv.Status(), s.Verdict), Semantic: sem}
	e.LineageBindingID = hash("lsp-trace.provisional-feature-lineage.v1", l)
	e.EntryID = entryID(e)
	return e
}
func terminal(s describeworker.TerminalStatus, verdict string) TerminalOutcome {
	switch s {
	case describeworker.StatusSucceeded:
		if verdict == "ABSTAINED" {
			return TerminalAbstained
		}
		return TerminalComplete
	case describeworker.StatusCancelled:
		return TerminalCancelled
	case describeworker.StatusTimeout:
		return TerminalTimeout
	case describeworker.StatusResourceLimit:
		return TerminalResourceLimit
	case describeworker.StatusBackendFailure:
		return TerminalBackendFailure
	case describeworker.StatusOutputInvalid:
		return TerminalOutputInvalid
	case describeworker.StatusModelUnavailable:
		return TerminalModelUnavailable
	case describeworker.StatusPolicyMismatch:
		return TerminalPolicyMismatch
	}
	return TerminalInvalidInput
}
func account(es []Entry) Accounting {
	a := Accounting{Total: len(es)}
	for _, e := range es {
		switch e.TerminalOutcome {
		case TerminalComplete:
			a.Complete++
		case TerminalAbstained:
			a.Abstained++
		case TerminalInvalidInput:
			a.InvalidInput++
		case TerminalModelUnavailable:
			a.ModelUnavailable++
		case TerminalContextLimit:
			a.ContextLimit++
		case TerminalOutputInvalid:
			a.OutputInvalid++
		case TerminalTimeout:
			a.Timeout++
		case TerminalCancelled:
			a.Cancelled++
		case TerminalResourceLimit:
			a.ResourceLimit++
		case TerminalBackendFailure:
			a.BackendFailure++
		case TerminalPolicyMismatch:
			a.PolicyMismatch++
		case TerminalDuplicateInput:
			a.DuplicateInput++
		case TerminalSourceUnavailable:
			// Counted separately as a pre-request failure by the additive builder.
		}
	}
	return a
}
func (a Accounting) sum() int {
	return a.Complete + a.Abstained + a.InvalidInput + a.ModelUnavailable + a.ContextLimit + a.OutputInvalid + a.Timeout + a.Cancelled + a.ResourceLimit + a.BackendFailure + a.PolicyMismatch + a.DuplicateInput + a.PreRequestFailed
}
func entryKey(e Entry) string {
	if e.Failure != nil {
		return "0\x00" + e.Failure.NominationID + "\x00" + e.Failure.FailureID
	}
	return "1\x00" + strings.Join([]string{e.Lineage.CommunityIdentity, e.Lineage.PacketID, fmt.Sprintf("%020d", e.Lineage.AlternativeOrdinal), e.Lineage.AlternativeID, e.Lineage.RequestRecordID, e.Lineage.InvocationID, e.Lineage.ResponseID}, "\x00")
}
func entryID(e Entry) string {
	e.EntryID = ""
	return hash("lsp-trace.provisional-feature-entry.v1", e)
}
func catalogID(w CatalogWire) string {
	w.CatalogID = ""
	return hash("lsp-trace.provisional-feature-catalog.v1", w)
}
func correctionID(c Correction) string {
	c.CorrectionID = ""
	return hash("lsp-trace.provisional-feature-correction.v1", c)
}
func hash(domain string, v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(append([]byte(domain+"\x00"), b...))
	return "sha256:" + hex.EncodeToString(s[:])
}
func finish(w CatalogWire) (Catalog, error) {
	w.CatalogID = catalogID(w)
	c := Catalog{wire: w}
	return c, Validate(c)
}

func Validate(c Catalog) error {
	w := c.wire
	if w.SchemaVersion != SchemaVersion || w.Status != StatusProvisional || w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown || w.CatalogID != catalogID(w) {
		return errors.New("catalog envelope mismatch")
	}
	seen := map[string]bool{}
	for i, e := range w.Entries {
		baseValid := e.Status == StatusProvisional && e.Authority == 0 && !e.Accepted && e.Completeness == CompletenessUnknown && e.EntryID == entryID(e) && e.LineageBindingID == hash("lsp-trace.provisional-feature-lineage.v1", e.Lineage)
		failureValid := e.Failure != nil && e.TerminalOutcome == TerminalSourceUnavailable && validFailureInput(*e.Failure) && e.Lineage.ResponseID == "" && e.Semantic.Verdict == "" && !e.Semantic.TargetRoleIsModelText
		describedValid := e.Failure == nil && e.Lineage.PacketID != "" && e.Lineage.RequestRecordID != "" && e.Lineage.InvocationID != "" && (e.TerminalOutcome != TerminalComplete || e.Lineage.ResponseID != "") && (e.Lineage.ResponseID != "" || (e.Semantic.Verdict == "" && !e.Semantic.TargetRoleIsModelText))
		if !baseValid || (!failureValid && !describedValid) {
			return errors.New("entry integrity mismatch")
		}
		if seen[e.EntryID] {
			return errors.New("duplicate entry")
		}
		seen[e.EntryID] = true
		if i > 0 && entryKey(w.Entries[i-1]) >= entryKey(e) {
			return errors.New("entry order mismatch")
		}
	}
	expectedAccounting := account(w.Entries)
	expectedAccounting.NominationTotal, expectedAccounting.RequestTotal, expectedAccounting.DescribedEntries, expectedAccounting.PreRequestFailed = w.Accounting.NominationTotal, w.Accounting.RequestTotal, w.Accounting.DescribedEntries, w.Accounting.PreRequestFailed
	if w.Accounting != expectedAccounting || w.Accounting.Total != len(w.Entries) || w.Accounting.sum() != w.Accounting.Total || (w.Accounting.NominationTotal != 0 && (w.Accounting.NominationTotal != w.Accounting.RequestTotal+w.Accounting.PreRequestFailed || w.Accounting.DescribedEntries != w.Accounting.RequestTotal || w.Accounting.NominationTotal != len(w.Entries))) {
		return errors.New("accounting mismatch")
	}
	balanced := w.Accounting.sum() == len(w.Entries)
	complete := balanced && w.Accounting.Complete == w.Accounting.Total
	if complete && w.Outcome != OutcomeComplete || !complete && w.Outcome != OutcomeDegraded {
		return errors.New("outcome mismatch")
	}
	correctionSeen := map[string]bool{}
	for _, k := range w.Corrections {
		if k.CorrectionID != correctionID(k) || k.PredecessorCatalogID == "" || k.ActorID == "" || k.Reason == "" || len(k.InventoryStateDeltas) == 0 || len(k.AffectedIDs) == 0 {
			return errors.New("correction mismatch")
		}
		if correctionSeen[k.CorrectionID] {
			return errors.New("duplicate correction")
		}
		correctionSeen[k.CorrectionID] = true
		for _, d := range k.InventoryStateDeltas {
			if d.Kind == DeltaCommunityRename || d.Kind == DeltaCommunityMerge || d.From == "" || d.To == "" {
				return errors.New("semantic correction forbidden")
			}
		}
	}
	return nil
}

func Parse(raw []byte) (Catalog, error) {
	if err := rejectDuplicate(raw); err != nil {
		return Catalog{}, err
	}
	var w CatalogWire
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&w); err != nil {
		return Catalog{}, err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return Catalog{}, errors.New("trailing json")
	}
	canonical, _ := json.Marshal(w)
	if !bytes.Equal(raw, canonical) {
		return Catalog{}, errors.New("noncanonical json")
	}
	c := Catalog{wire: w}
	return c, Validate(c)
}
func rejectDuplicate(raw []byte) error { d := json.NewDecoder(bytes.NewReader(raw)); return scan(d) }
func scan(d *json.Decoder) error {
	tok, err := d.Token()
	if err != nil {
		return err
	}
	x, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if x == '{' {
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s := k.(string)
			if seen[s] {
				return errors.New("duplicate json key")
			}
			seen[s] = true
			if e = scan(d); e != nil {
				return e
			}
		}
		_, err = d.Token()
		return err
	}
	if x == '[' {
		for d.More() {
			if err = scan(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return errors.New("invalid json")
}

func BuildSuccessor(prior Catalog, inputs []CorrectionInput) (Catalog, error) {
	if err := Validate(prior); err != nil {
		return Catalog{}, err
	}
	w := prior.wire
	w.PredecessorCatalogID = prior.ID()
	w.Entries = cloneEntries(w.Entries)
	w.Corrections = cloneCorrections(w.Corrections)
	entryIDs := map[string]bool{}
	for _, e := range w.Entries {
		entryIDs[e.EntryID] = true
	}
	corrIDs := map[string]bool{}
	for _, k := range w.Corrections {
		corrIDs[k.CorrectionID] = true
	}
	for _, in := range inputs {
		if in.ActorID == "" || in.Reason == "" || len(in.InventoryStateDeltas) == 0 || len(in.AffectedEntryIDs) == 0 || in.RebuildDisposition == "" || (in.ActorAuthority != AuthorityStakeholder && in.ActorAuthority != AuthorityReviewer) {
			return Catalog{}, errors.New("invalid correction")
		}
		for _, id := range in.AffectedEntryIDs {
			if !entryIDs[id] {
				return Catalog{}, errors.New("foreign affected entry")
			}
		}
		for _, id := range in.SupersedesCorrectionIDs {
			if !corrIDs[id] {
				return Catalog{}, errors.New("foreign superseded correction")
			}
		}
		for _, d := range in.InventoryStateDeltas {
			if d.Kind == DeltaCommunityRename || d.Kind == DeltaCommunityMerge || d.From == "" || d.To == "" {
				return Catalog{}, errors.New("semantic equivalence correction forbidden")
			}
		}
		sup := append([]string(nil), in.SupersedesEntryIDs...)
		if len(sup) == 0 {
			sup = append([]string(nil), in.AffectedEntryIDs...)
		}
		for _, id := range sup {
			if !entryIDs[id] {
				return Catalog{}, errors.New("foreign superseded entry")
			}
		}
		sort.Strings(sup)
		aff := append([]string(nil), in.AffectedEntryIDs...)
		sort.Strings(aff)
		k := Correction{PredecessorCatalogID: prior.ID(), PredecessorEntryIDs: entryIDList(prior.wire.Entries), SupersedesCorrectionIDs: append([]string(nil), in.SupersedesCorrectionIDs...), SupersedesEntryIDs: sup, ActorID: in.ActorID, ActorAuthority: in.ActorAuthority, Reason: in.Reason, InventoryStateDeltas: append([]InventoryStateDelta(nil), in.InventoryStateDeltas...), AffectedIDs: aff, RebuildDisposition: in.RebuildDisposition}
		sort.Strings(k.SupersedesCorrectionIDs)
		k.CorrectionID = correctionID(k)
		if corrIDs[k.CorrectionID] {
			return Catalog{}, errors.New("duplicate correction")
		}
		corrIDs[k.CorrectionID] = true
		w.Corrections = append(w.Corrections, k)
	}
	return finish(w)
}
func entryIDList(es []Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.EntryID
	}
	sort.Strings(out)
	return out
}
func cloneEntries(in []Entry) []Entry {
	out := make([]Entry, len(in))
	for i, e := range in {
		out[i] = e
		out[i].Lineage.Members = append([]string(nil), e.Lineage.Members...)
		out[i].Lineage.PreparedTargets = append([]string(nil), e.Lineage.PreparedTargets...)
		out[i].Lineage.SCCMembers = append([]string(nil), e.Lineage.SCCMembers...)
		out[i].Semantic.Limitations = append([]Limitation(nil), e.Semantic.Limitations...)
		out[i].Semantic.Citations = append([]Citation(nil), e.Semantic.Citations...)
	}
	return out
}
func cloneCorrections(in []Correction) []Correction {
	out := make([]Correction, len(in))
	for i, k := range in {
		out[i] = k
		out[i].PredecessorEntryIDs = append([]string(nil), k.PredecessorEntryIDs...)
		out[i].SupersedesCorrectionIDs = append([]string(nil), k.SupersedesCorrectionIDs...)
		out[i].SupersedesEntryIDs = append([]string(nil), k.SupersedesEntryIDs...)
		out[i].InventoryStateDeltas = append([]InventoryStateDelta(nil), k.InventoryStateDeltas...)
		out[i].AffectedIDs = append([]string(nil), k.AffectedIDs...)
	}
	return out
}
