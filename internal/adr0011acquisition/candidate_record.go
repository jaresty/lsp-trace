package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
)

const candidateRecordRole = "REFERENCES_OCCURRENCE_CANDIDATE_V1"

var errCandidateRecord = errors.New("private references candidate not verified")

func candidateRecordDigest(b []byte) string {
	return privateDigest(append([]byte(candidateRecordRole+"\x00"), b...))
}
func candidateRecordSelector(d string) string {
	return "adr0011-references-issuance-v1-candidate-" + strings.TrimPrefix(d, "sha256:") + ".json"
}

// canonicalCandidateRecord derives all fields from independently replayed predecessors.
// It is inert: neither the record nor an occurrence ID issues T or A.
func canonicalCandidateRecord(root *publication.Root, proposal privateBodyPublication, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error)) ([]byte, error) {
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	if !replayProposalRecord(root, proposal, x, read) {
		return nil, errCandidateRecord
	}
	context, err := canonicalProposalContext(root, x, read)
	if err != nil {
		return nil, errCandidateRecord
	}
	proposed, err := read(root, proposal.selector, sourceRecordLimit)
	if err != nil {
		return nil, errCandidateRecord
	}
	var fields struct {
		Context      json.RawMessage `json:"context"`
		KnownE       int             `json:"known_e"`
		WholeResultP int             `json:"whole_result_p"`
	}
	if json.Unmarshal(proposed, &fields) != nil || !bytes.Equal(fields.Context, context) {
		return nil, errCandidateRecord
	}
	raw, ok := rawResultExpected(root, x.ResponseRead, x.ResponseInputs.Payload, x.ResponseInputs, read)
	if !ok {
		return nil, errCandidateRecord
	}
	payload, err := read(root, raw.PayloadSelector, rawResultPayloadLimit)
	if err != nil || !bytes.Equal(payload, x.Payload) || privateDigest(payload) != raw.PayloadDigest {
		return nil, errCandidateRecord
	}
	result, failure := adr0011methodresult.ParseRawReferences(payload, 1000)
	if failure != nil || len(result.Items) != fields.KnownE || len(result.Items) != fields.WholeResultP {
		return nil, errCandidateRecord
	}
	var identity struct {
		TransactionID     string          `json:"transaction_id"`
		QueryOccurrenceID string          `json:"query_occurrence_id"`
		MethodRef         json.RawMessage `json:"method_ref"`
		TargetRef         json.RawMessage `json:"target_ref"`
	}
	if json.Unmarshal(context, &identity) != nil || identity.TransactionID == "" || identity.QueryOccurrenceID == "" {
		return nil, errCandidateRecord
	}
	occurrences := make([]string, 0, len(result.Items))
	for i, item := range result.Items {
		parsedURI, uriErr := url.Parse(item.URI)
		if item.Kind != adr0011methodresult.Location || item.Ordinal != i || !validTargetIdentityString(item.URI) || uriErr != nil || !parsedURI.IsAbs() {
			return nil, errCandidateRecord
		}
		rangeJSON := targetIdentityRangeJSON(adr0011querytarget.Range{Start: adr0011querytarget.Position{Line: item.Range.Start.Line, Character: item.Range.Start.Character}, End: adr0011querytarget.Position{Line: item.Range.End.Line, Character: item.Range.End.Character}})
		id := targetIdentityHash([]byte("REFERENCES_OCCURRENCE_V1"), []byte(identity.TransactionID), []byte(identity.QueryOccurrenceID), identity.MethodRef, identity.TargetRef, []byte(strconv.Itoa(i)), targetIdentityStringJSON(item.URI), rangeJSON)
		occurrences = append(occurrences, fmt.Sprintf(`{"occurrence_id":%s,"ordinal":%d,"returned_range":%s,"returned_uri":%s}`, targetIdentityStringJSON(id), i, rangeJSON, targetIdentityStringJSON(item.URI)))
	}
	b := []byte(fmt.Sprintf(`{"context":%s,"occurrences":[%s],"proposal_ref":%s,"proposed_p":%d,"schema_version":%s}`, context, strings.Join(occurrences, ","), targetIdentityRefJSON(targetRecordRef(proposalRecordRole, proposal)), len(occurrences), targetIdentityStringJSON(candidateRecordRole)))
	if len(b) > sourceRecordLimit {
		return nil, errCandidateRecord
	}
	return b, nil
}

func replayCandidateRecord(root *publication.Root, state, proposal privateBodyPublication, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayCandidateRecordInternal(root, state, proposal, x, read, false)
}
func replayCandidateRecordInternal(root *publication.Root, state, proposal privateBodyPublication, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	b, err := read(root, state.selector, sourceRecordLimit)
	if err != nil || len(b) != state.byteCount || state.digest != candidateRecordDigest(b) || state.selector != candidateRecordSelector(state.digest) {
		return false
	}
	expected, err := canonicalCandidateRecord(root, proposal, x, read)
	return err == nil && bytes.Equal(b, expected)
}
func publishCandidateRecord(root *publication.Root, proposal privateBodyPublication, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error), trace publication.BoundFileTrace) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	b, err := canonicalCandidateRecord(root, proposal, x, read)
	if err != nil {
		return absent, errCandidateRecord
	}
	digest := candidateRecordDigest(b)
	selector := candidateRecordSelector(digest)
	precommit, uncertain := false, false
	receipt, err := publication.PublishBoundFileWithTrace(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return errCandidateRecord
		}
		return nil
	}, func(event publication.BoundFileTraceEvent) {
		if trace != nil {
			trace(event)
		}
		if !event.OK {
			switch event.Stage {
			case "OPEN_VALIDATE", "CANDIDATE", "TEMP", "WRITE_FSYNC":
				precommit = true
			case "TARGET", "HARDLINK":
				if event.Result == "INJECTED_BEFORE_INSTALL" {
					precommit = true
				} else {
					uncertain = true
				}
			}
		}
		if event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
			uncertain = true
		}
	})
	if err != nil || receipt == nil {
		if precommit && !uncertain {
			return absent, errCandidateRecord
		}
		return privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}, errCandidateRecord
	}
	state := privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, privateDigest(b), len(b)) || !replayCandidateRecordInternal(root, state, proposal, x, read, true) {
		return state, errCandidateRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
