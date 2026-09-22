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

	"lsp-trace/internal/describeworker"
)

const (
	SchemaVersionV2       = "lsp-trace.provisional-feature-catalog.v2"
	ExperimentalV2Warning = "EXPERIMENTAL PROVISIONAL — semantic richness not qualified"
)

type EntryV2 struct {
	ResponseID              string                                          `json:"response_id"`
	RequestRecordID         string                                          `json:"request_record_id"`
	MessageID               string                                          `json:"message_id"`
	AttemptID               string                                          `json:"attempt_id"`
	Authority               int                                             `json:"authority"`
	Accepted                bool                                            `json:"accepted"`
	Completeness            string                                          `json:"completeness"`
	Consumer                describeworker.ConsumerIdentityV2               `json:"consumer"`
	Semantic                describeworker.SemanticResponseV2               `json:"semantic"`
	AdmissibleEvidenceBasis describeworker.EvidenceBasisV2                  `json:"admissible_evidence_basis"`
	CitationSuggestions     *describeworker.AttributedCitationSuggestionsV2 `json:"citation_suggestions,omitempty"`
}
type CatalogV2 struct {
	SchemaVersion          string                     `json:"schema_version"`
	CatalogID              string                     `json:"catalog_id"`
	Authority              int                        `json:"authority"`
	Accepted               bool                       `json:"accepted"`
	Completeness           string                     `json:"completeness"`
	OutcomeStatus          CatalogOutcome             `json:"outcome"`
	Counts                 Accounting                 `json:"accounting"`
	Entries                []EntryV2                  `json:"entries"`
	PreparationFailureList []PreparationFailureMember `json:"preparation_failures"`
}

func (c CatalogV2) ID() string              { return c.CatalogID }
func (c CatalogV2) Outcome() CatalogOutcome { return c.OutcomeStatus }
func (c CatalogV2) Accounting() Accounting  { return c.Counts }
func (c CatalogV2) PreparationFailures() []PreparationFailureMember {
	return clonePreparationFailures(c.PreparationFailureList)
}
func (c CatalogV2) Bytes() ([]byte, error) {
	if err := ValidateV2(c); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

// BuildV2 copies validated response fields. It does not infer, merge, or reinterpret semantics.
func BuildV2(responses []describeworker.ResponseRecordV2) (CatalogV2, error) {
	return BuildV2WithPreparationFailures(len(responses), nil, responses)
}

func BuildV2WithPreparationFailures(nominationTotal int, failures []PreparationFailureInput, responses []describeworker.ResponseRecordV2) (CatalogV2, error) {
	catalog := CatalogV2{SchemaVersion: SchemaVersionV2, Completeness: CompletenessUnknown, OutcomeStatus: OutcomeComplete, Entries: []EntryV2{}, PreparationFailureList: []PreparationFailureMember{}}
	failedNominations := map[string]bool{}
	for _, failure := range failures {
		if failure.FailureID == "" || failure.NominationID == "" || failure.PacketIntentID == "" || failure.Role == "" || failure.Code == "" || len(failure.EvidenceIDs) == 0 || failedNominations[failure.NominationID] {
			return CatalogV2{}, errors.New("invalid preparation failure v2 catalog input")
		}
		failedNominations[failure.NominationID] = true
		copy := failure
		copy.EvidenceIDs = append([]string(nil), failure.EvidenceIDs...)
		sort.Strings(copy.EvidenceIDs)
		catalog.PreparationFailureList = append(catalog.PreparationFailureList, copy)
	}
	if nominationTotal < 0 || nominationTotal != len(responses)+len(failedNominations) {
		return CatalogV2{}, errors.New("invalid nomination accounting v2 catalog input")
	}
	seen := map[string]bool{}
	for _, response := range responses {
		if err := response.Validate(); err != nil || seen[response.ID()] {
			return CatalogV2{}, errors.New("invalid response v2 catalog input")
		}
		seen[response.ID()] = true
		host := response.Host()
		suggestions := response.CitationSuggestions()
		entry := EntryV2{ResponseID: response.ID(), RequestRecordID: host.RequestRecordID, MessageID: host.MessageID, AttemptID: host.AttemptID, Completeness: CompletenessUnknown, Consumer: response.Consumer(), Semantic: response.Semantic(), AdmissibleEvidenceBasis: response.AdmissibleEvidenceBasis()}
		if suggestions.Label != "" {
			entry.CitationSuggestions = &suggestions
		}
		catalog.Entries = append(catalog.Entries, entry)
	}
	sort.Slice(catalog.Entries, func(i, j int) bool { return catalog.Entries[i].ResponseID < catalog.Entries[j].ResponseID })
	sort.Slice(catalog.PreparationFailureList, func(i, j int) bool {
		if catalog.PreparationFailureList[i].NominationID != catalog.PreparationFailureList[j].NominationID {
			return catalog.PreparationFailureList[i].NominationID < catalog.PreparationFailureList[j].NominationID
		}
		return catalog.PreparationFailureList[i].FailureID < catalog.PreparationFailureList[j].FailureID
	})
	catalog.Counts = Accounting{Total: len(responses) + len(failures), Complete: len(responses), NominationTotal: nominationTotal, RequestTotal: len(responses), DescribedEntries: len(responses), PreRequestFailed: len(failures)}
	if len(failures) != 0 {
		catalog.OutcomeStatus = OutcomeDegraded
	}
	catalog.CatalogID = catalogV2ID(catalog)
	return catalog, ValidateV2(catalog)
}
func ValidateV2(c CatalogV2) error {
	if c.SchemaVersion != SchemaVersionV2 || c.CatalogID != catalogV2ID(c) || c.Authority != 0 || c.Accepted || c.Completeness != CompletenessUnknown || (c.OutcomeStatus != OutcomeComplete && c.OutcomeStatus != OutcomeDegraded) || c.Entries == nil || c.PreparationFailureList == nil {
		return errors.New("catalog v2 envelope mismatch")
	}
	for i, e := range c.Entries {
		if e.ResponseID == "" || e.RequestRecordID == "" || e.MessageID == "" || e.AttemptID == "" || e.Authority != 0 || e.Accepted || e.Completeness != CompletenessUnknown || e.AdmissibleEvidenceBasis.Label != "admissible_evidence_basis" {
			return errors.New("catalog v2 entry mismatch")
		}
		if i > 0 && c.Entries[i-1].ResponseID >= e.ResponseID {
			return errors.New("catalog v2 order mismatch")
		}
		if e.CitationSuggestions != nil && e.CitationSuggestions.Label != "MODEL_ATTRIBUTED_NON_AUTHORITATIVE" {
			return errors.New("catalog v2 suggestion mismatch")
		}
	}
	if c.Counts.Total != len(c.Entries)+len(c.PreparationFailureList) || c.Counts.Complete != len(c.Entries) || c.Counts.PreRequestFailed != len(c.PreparationFailureList) || c.Counts.DescribedEntries != len(c.Entries) || c.Counts.RequestTotal != len(c.Entries) || c.Counts.NominationTotal != len(c.Entries)+len(c.PreparationFailureList) || c.Counts.sum() != c.Counts.Total || (c.OutcomeStatus == OutcomeComplete) != (len(c.PreparationFailureList) == 0) {
		return errors.New("catalog v2 accounting mismatch")
	}
	for i, failure := range c.PreparationFailureList {
		if failure.FailureID == "" || failure.NominationID == "" || failure.PacketIntentID == "" || failure.Role == "" || failure.Code == "" || len(failure.EvidenceIDs) == 0 || (i > 0 && c.PreparationFailureList[i-1].NominationID >= failure.NominationID) {
			return errors.New("catalog v2 preparation failure mismatch")
		}
	}
	return nil
}
func catalogV2ID(c CatalogV2) string {
	c.CatalogID = ""
	raw, _ := json.Marshal(c)
	sum := sha256.Sum256(append([]byte("lsp-trace.provisional-feature-catalog.v2\x00"), raw...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ParseV2(raw []byte) (CatalogV2, error) {
	if len(raw) == 0 || !bytes.Equal(raw, bytes.TrimSpace(raw)) || rejectDuplicate(raw) != nil {
		return CatalogV2{}, errors.New("invalid catalog v2")
	}
	var catalog CatalogV2
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return CatalogV2{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return CatalogV2{}, errors.New("invalid catalog v2")
	}
	canonical, err := json.Marshal(catalog)
	if err != nil || !bytes.Equal(raw, canonical) {
		return CatalogV2{}, errors.New("noncanonical catalog v2")
	}
	return catalog, ValidateV2(catalog)
}

func clonePreparationFailures(in []PreparationFailureMember) []PreparationFailureMember {
	out := append([]PreparationFailureMember(nil), in...)
	for i := range out {
		out[i].EvidenceIDs = append([]string(nil), out[i].EvidenceIDs...)
	}
	return out
}

func RenderReviewV2(c CatalogV2) (string, error) {
	if err := ValidateV2(c); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintln(&b, ExperimentalV2Warning)
	fmt.Fprintf(&b, "Catalog %s · Schema %s · Authority 0 · Accepted false · Completeness UNKNOWN\n", c.CatalogID, c.SchemaVersion)
	for _, e := range c.Entries {
		fmt.Fprintf(&b, "\n- Response %s · Request %s · Consumer %d/%s (%s)\n", e.ResponseID, e.RequestRecordID, e.Consumer.AlternativeOrdinal, e.Consumer.AlternativeID, e.Consumer.Resolution)
		fmt.Fprintf(&b, "  Model target_role: %s\n", safeText(e.Semantic.TargetRole))
		fmt.Fprintf(&b, "  Model consumer_need: %s\n", safeText(e.Semantic.ConsumerNeed.Value))
		fmt.Fprintf(&b, "  Model provided_behavior: %s\n", safeText(e.Semantic.ProvidedBehavior.Value))
		fmt.Fprintf(&b, "  Model boundary_contribution: %s\n", safeText(e.Semantic.BoundaryContribution))
		fmt.Fprintf(&b, "  admissible_evidence_basis: target_role=%v consumer_need=%v provided_behavior=%v boundary_contribution=%v limitations=%v (admissibility only; not proof)\n", e.AdmissibleEvidenceBasis.TargetRole, e.AdmissibleEvidenceBasis.ConsumerNeed, e.AdmissibleEvidenceBasis.ProvidedBehavior, e.AdmissibleEvidenceBasis.BoundaryContribution, e.AdmissibleEvidenceBasis.Limitations)
		if e.CitationSuggestions != nil {
			fmt.Fprintf(&b, "  Model citation_suggestions (%s; not support): %+v\n", e.CitationSuggestions.Label, e.CitationSuggestions.Suggestions)
		}
	}
	return b.String(), nil
}
