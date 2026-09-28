package adr0011acquisition

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"lsp-trace/internal/publication"
)

const proposalRecordRole = "REFERENCES_EVALUATION_PROPOSAL_V1"

var errProposalRecord = errors.New("private references proposal not verified")

func proposalRecordDigest(b []byte) string {
	return privateDigest(append([]byte(proposalRecordRole+"\x00"), b...))
}
func proposalRecordSelector(d string) string {
	return "adr0011-references-issuance-v1-proposal-" + strings.TrimPrefix(d, "sha256:") + ".json"
}

// canonicalProposalRecord never accepts a claimant-provided context or proposal.
// Its fields are projections of independently replayed predecessors.
func canonicalProposalRecord(root *publication.Root, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error)) ([]byte, error) {
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	context, err := canonicalProposalContext(root, x, read)
	if err != nil {
		return nil, errProposalRecord
	}
	r := x.ResponseInputs
	response, ok := responseReadExpected(root, r, read)
	if !ok || !replayRawResult(root, x.RawResult, x.ResponseRead, r.Payload, r, read) ||
		!replayScannerRecord(root, x.Scanner, x.ResponseRead, x.RawResult, r.Payload, r, read, nil) {
		return nil, errProposalRecord
	}
	raw, ok := rawResultExpected(root, x.ResponseRead, r.Payload, r, read)
	if !ok || raw.PayloadByteLength < 1 || raw.PayloadByteLength > rawResultPayloadLimit {
		return nil, errProposalRecord
	}
	payload, err := read(root, raw.PayloadSelector, rawResultPayloadLimit)
	if err != nil || !bytes.Equal(payload, x.Payload) || raw.PayloadDigest != privateDigest(payload) || raw.PayloadByteLength != len(payload) {
		return nil, errProposalRecord
	}
	scan := scanRawTopLevel(payload)
	if scan.form != "NULL" && scan.form != "ARRAY" || scan.form != x.ScannerObservation.form || scan.knownE != x.ScannerObservation.knownE || scan.workUnits != x.ScannerObservation.workUnits {
		return nil, errProposalRecord
	}
	events, ok := eventsExpected(root, x.ResponseRead, x.RawResult, x.ScannerObservation, r, response.TransactionID, payload, x.Journal, read)
	if !ok || !replayEvents(root, x.Events, x.ResponseRead, x.RawResult, x.ScannerObservation, r, response.TransactionID, payload, x.Journal, read) || !validProposalEvaluation(x.Evaluation, r, x.ScannerObservation, payload) ||
		len(events.Events) != 2+2*scan.knownE {
		return nil, errProposalRecord
	}
	outcome, disposition := "COMPLETE_EMPTY", "EMPTY"
	if scan.knownE > 0 {
		outcome, disposition = "COMPLETE", "ITEMS"
	}
	if events.Events[len(events.Events)-1].TerminalDisposition != disposition || x.Evaluation.Outcome != outcome || x.Evaluation.Disposition != disposition {
		return nil, errProposalRecord
	}
	field := func(s string) string { return string(targetIdentityStringJSON(s)) }
	ref := func(role string, p privateBodyPublication) string {
		return string(targetIdentityRefJSON(targetRecordRef(role, p)))
	}
	// ASCII object keys are in JCS UTF-16 order; the shared string encoder enforces NFC.
	b := []byte(fmt.Sprintf(`{"context":%s,"declared_n":1,"evaluator_event_ref":%s,"known_e":%d,"observed_b":1,"observed_e_b":%d,"observed_e_t":%d,"proposed_disposition":%s,"proposed_outcome":%s,"raw_result_byte_length":%d,"raw_result_digest":%s,"raw_result_presence":"PRESENT","raw_result_ref":%s,"scanner_observation_ref":%s,"schema_version":%s,"top_level_form":%s,"whole_result_p":%d}`,
		context, ref(eventsRole, x.Events), scan.knownE, scan.knownE, scan.knownE, field(disposition), field(outcome), len(payload), field(privateDigest(payload)), ref(rawResultRole, x.RawResult), ref(scannerRecordRole, x.Scanner), field(proposalRecordRole), field(scan.form), scan.knownE))
	if len(b) > sourceRecordLimit {
		return nil, errProposalRecord
	}
	return b, nil
}

func replayProposalRecord(root *publication.Root, state privateBodyPublication, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayProposalRecordInternal(root, state, x, read, false)
}
func replayProposalRecordInternal(root *publication.Root, state privateBodyPublication, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	b, err := read(root, state.selector, sourceRecordLimit)
	if err != nil || len(b) != state.byteCount || state.digest != proposalRecordDigest(b) || state.selector != proposalRecordSelector(state.digest) {
		return false
	}
	expected, err := canonicalProposalRecord(root, x, read)
	return err == nil && bytes.Equal(b, expected)
}

func publishProposalRecord(root *publication.Root, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error), trace publication.BoundFileTrace) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	b, err := canonicalProposalRecord(root, x, read)
	if err != nil {
		return absent, errProposalRecord
	}
	digest := proposalRecordDigest(b)
	selector := proposalRecordSelector(digest)
	precommit, uncertain := false, false
	receipt, err := publication.PublishBoundFileWithTrace(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return errProposalRecord
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
			return absent, errProposalRecord
		}
		return privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}, errProposalRecord
	}
	state := privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, privateDigest(b), len(b)) || !replayProposalRecordInternal(root, state, x, read, true) {
		return state, errProposalRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
