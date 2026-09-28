package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const eventsRole = "REFERENCES_EVALUATOR_EVENTS_V1"

var errEventsRecord = errors.New("private references events not verified")

type eventsEntry struct {
	Sequence            int    `json:"sequence"`
	Kind                string `json:"kind"`
	Ordinal             *int   `json:"ordinal"`
	TerminalDisposition string `json:"terminal_disposition"`
}
type eventsRecord struct {
	SchemaVersion   string                `json:"schema_version"`
	TransactionID   string                `json:"transaction_id"`
	RequestKey      string                `json:"request_key"`
	InvocationID    string                `json:"invocation_id"`
	ResponseReadRef targetRecordRefFields `json:"response_read_ref"`
	RawResultRef    targetRecordRefFields `json:"raw_result_ref"`
	Events          []eventsEntry         `json:"events"`
}
type attachedJournalRecord struct {
	Owner struct {
		Transaction  string
		RequestKey   string
		Invocation   string
		ResponseRead string
		RawSelector  string
		RawDigest    string
	} `json:"owner"`
	Events []struct {
		Kind    string `json:"kind"`
		Ordinal int    `json:"ordinal"`
	} `json:"events"`
}

func eventsDigest(b []byte) string { return privateDigest(append([]byte(eventsRole+"\x00"), b...)) }
func eventsSelector(d string) string {
	return "adr0011-references-issuance-v1-events-" + strings.TrimPrefix(d, "sha256:") + ".json"
}
func canonicalEvents(r eventsRecord) ([]byte, error) {
	if r.SchemaVersion != eventsRole || !validPrivateDigest(r.TransactionID) || !validTargetIdentityString(r.RequestKey) || !validTargetIdentityString(r.InvocationID) || r.ResponseReadRef.SchemaVersion != responseReadRole || r.RawResultRef.SchemaVersion != rawResultRole || !validPrivateDigest(r.ResponseReadRef.Digest) || !validPrivateDigest(r.RawResultRef.Digest) || r.ResponseReadRef.Selector != responseReadSelector(r.ResponseReadRef.Digest) || r.RawResultRef.Selector != rawResultSelector(r.RawResultRef.Digest) || len(r.Events) < 2 || len(r.Events) > 2002 {
		return nil, errEventsRecord
	}
	encoded := make([]string, 0, len(r.Events))
	for i, e := range r.Events {
		if e.Sequence != i {
			return nil, errEventsRecord
		}
		ordinal := "null"
		switch e.Kind {
		case "QUERY_BEGIN":
			if i != 0 || e.Ordinal != nil || e.TerminalDisposition != "NONE" {
				return nil, errEventsRecord
			}
		case "ELEMENT_BEGIN":
			if e.Ordinal == nil || *e.Ordinal < 0 || *e.Ordinal >= 1000 || e.TerminalDisposition != "NONE" {
				return nil, errEventsRecord
			}
			ordinal = fmt.Sprint(*e.Ordinal)
		case "ELEMENT_TERMINAL":
			if e.Ordinal == nil || *e.Ordinal < 0 || *e.Ordinal >= 1000 || e.TerminalDisposition != "VALID_PENDING_ADMISSION" {
				return nil, errEventsRecord
			}
			ordinal = fmt.Sprint(*e.Ordinal)
		case "QUERY_TERMINAL":
			if i != len(r.Events)-1 || e.Ordinal != nil || (e.TerminalDisposition != "EMPTY" && e.TerminalDisposition != "ITEMS") {
				return nil, errEventsRecord
			}
		default:
			return nil, errEventsRecord
		}
		encoded = append(encoded, fmt.Sprintf(`{"kind":%s,"ordinal":%s,"sequence":%d,"terminal_disposition":%s}`, targetIdentityStringJSON(e.Kind), ordinal, i, targetIdentityStringJSON(e.TerminalDisposition)))
	}
	f := func(s string) string { return string(targetIdentityStringJSON(s)) }
	b := []byte(fmt.Sprintf(`{"events":[%s],"invocation_id":%s,"raw_result_ref":%s,"request_key":%s,"response_read_ref":%s,"schema_version":%s,"transaction_id":%s}`, strings.Join(encoded, ","), f(r.InvocationID), targetIdentityRefJSON(targetResultRef{r.RawResultRef.SchemaVersion, r.RawResultRef.Selector, r.RawResultRef.Digest}), f(r.RequestKey), targetIdentityRefJSON(targetResultRef{r.ResponseReadRef.SchemaVersion, r.ResponseReadRef.Selector, r.ResponseReadRef.Digest}), f(r.SchemaVersion), f(r.TransactionID)))
	if len(b) > sourceRecordLimit {
		return nil, errEventsRecord
	}
	return b, nil
}

// eventsExpected consumes independently supplied final journal bytes; it does not
// synthesize transitions from returned parser items.
func eventsExpected(root *publication.Root, responseRead, rawResult privateBodyPublication, scanner rawScannerObservation, x responseReadInputs, transactionID string, payload, journal []byte, read func(*publication.Root, string, int64) ([]byte, error)) (eventsRecord, bool) {
	var zero eventsRecord
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	if root == nil || responseRead.stage != "VERIFIED" || rawResult.stage != "VERIFIED" || !replayResponseRead(root, responseRead, x, read) || !replayRawResult(root, rawResult, responseRead, x.Payload, x, read) {
		return zero, false
	}
	response, e := responseReadExpected(root, x, read)
	if !e || transactionID != response.TransactionID {
		return zero, false
	}
	raw, e := rawResultExpected(root, responseRead, x.Payload, x, read)
	if !e {
		return zero, false
	}
	retained, err := read(root, raw.PayloadSelector, rawResultPayloadLimit)
	if err != nil || !bytes.Equal(retained, payload) || raw.PayloadDigest != privateDigest(payload) || raw.PayloadByteLength != len(payload) {
		return zero, false
	}
	actual := scanRawTopLevel(payload)
	if actual.form == "MALFORMED" || actual.form != scanner.form || actual.knownE != scanner.knownE || actual.workUnits != scanner.workUnits || scanner.selector != x.Payload.selector || scanner.digest != x.Payload.digest || scanner.byteLength != len(payload) || actual.knownE > 1000 {
		return zero, false
	}
	identity := adr0011methodresult.PrivateTransitionIdentity{Transaction: transactionID, RequestKey: response.RequestKey, Invocation: response.InvocationID, ResponseRead: responseRead.selector, RawSelector: rawResult.selector, RawDigest: raw.PayloadDigest}
	if adr0011methodresult.ReplayPrivateAttachedJournal(root, payload, x.Pair.Key, identity, journal) != nil {
		return zero, false
	}
	var attached attachedJournalRecord
	if strictjson.RejectDuplicates(journal) != nil || json.Unmarshal(journal, &attached) != nil || attached.Owner.Transaction != identity.Transaction || attached.Owner.RequestKey != identity.RequestKey || attached.Owner.Invocation != identity.Invocation || attached.Owner.ResponseRead != identity.ResponseRead || attached.Owner.RawSelector != identity.RawSelector || attached.Owner.RawDigest != identity.RawDigest || len(attached.Events) != 2+2*scanner.knownE {
		return zero, false
	}
	events := make([]eventsEntry, len(attached.Events))
	for i, v := range attached.Events {
		entry := eventsEntry{Sequence: i, TerminalDisposition: "NONE"}
		switch v.Kind {
		case "QUERY_BEGIN":
			if i != 0 || v.Ordinal != -1 {
				return zero, false
			}
			entry.Kind = "QUERY_BEGIN"
		case "ELEMENT_BEGIN":
			if i < 1 || i >= len(events)-1 || i%2 != 1 || v.Ordinal != (i-1)/2 {
				return zero, false
			}
			n := v.Ordinal
			entry.Kind = "ELEMENT_BEGIN"
			entry.Ordinal = &n
		case "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION":
			if i < 2 || i >= len(events)-1 || i%2 != 0 || v.Ordinal != (i-2)/2 {
				return zero, false
			}
			n := v.Ordinal
			entry.Kind = "ELEMENT_TERMINAL"
			entry.Ordinal = &n
			entry.TerminalDisposition = "VALID_PENDING_ADMISSION"
		case "QUERY_TERMINAL_EMPTY":
			if i != len(events)-1 || scanner.knownE != 0 || v.Ordinal != -1 {
				return zero, false
			}
			entry.Kind = "QUERY_TERMINAL"
			entry.TerminalDisposition = "EMPTY"
		case "QUERY_TERMINAL_ITEMS":
			if i != len(events)-1 || scanner.knownE == 0 || v.Ordinal != -1 {
				return zero, false
			}
			entry.Kind = "QUERY_TERMINAL"
			entry.TerminalDisposition = "ITEMS"
		default:
			return zero, false
		}
		events[i] = entry
	}
	return eventsRecord{eventsRole, transactionID, response.RequestKey, response.InvocationID, targetRecordRefFields{responseRead.selector, responseRead.digest, responseReadRole}, targetRecordRefFields{rawResult.selector, rawResult.digest, rawResultRole}, events}, true
}
func replayEvents(root *publication.Root, state, responseRead, rawResult privateBodyPublication, scanner rawScannerObservation, x responseReadInputs, transactionID string, payload, journal []byte, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayEventsInternal(root, state, responseRead, rawResult, scanner, x, transactionID, payload, journal, read, false)
}
func replayEventsInternal(root *publication.Root, state, responseRead, rawResult privateBodyPublication, scanner rawScannerObservation, x responseReadInputs, transactionID string, payload, journal []byte, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	b, e := read(root, state.selector, sourceRecordLimit)
	if e != nil || len(b) != state.byteCount || state.digest != eventsDigest(b) || state.selector != eventsSelector(state.digest) || strictjson.RejectDuplicates(b) != nil {
		return false
	}
	var record eventsRecord
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&record) != nil {
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return false
	}
	expected, ok := eventsExpected(root, responseRead, rawResult, scanner, x, transactionID, payload, journal, read)
	if !ok {
		return false
	}
	canonical, e := canonicalEvents(expected)
	return e == nil && bytes.Equal(b, canonical)
}
func publishEvents(root *publication.Root, responseRead, rawResult privateBodyPublication, scanner rawScannerObservation, evaluation *adr0011methodresult.ReferenceEvaluation, x responseReadInputs, transactionID string, payload, journal []byte, read func(*publication.Root, string, int64) ([]byte, error), trace publication.BoundFileTrace) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if evaluation == nil || evaluation.Key != x.Pair.Key || evaluation.RawDigest != privateDigest(payload) || evaluation.E != scanner.knownE || evaluation.P != scanner.knownE || evaluation.EB != scanner.knownE || evaluation.ET != scanner.knownE || evaluation.N != 1 || evaluation.B != 1 || evaluation.T != 0 || evaluation.A != 0 || evaluation.FailureOrdinal != -1 || len(evaluation.Events) != 2*scanner.knownE || len(evaluation.Items) != scanner.knownE {
		return absent, errEventsRecord
	}
	for i, event := range evaluation.Events {
		transition := "BEGIN"
		if i%2 != 0 {
			transition = "TERMINAL"
		}
		if event.Ordinal != i/2 || event.Transition != transition {
			return absent, errEventsRecord
		}
	}
	disposition, outcome := "ITEMS", "COMPLETE"
	if scanner.knownE == 0 {
		disposition, outcome = "EMPTY", "COMPLETE_EMPTY"
	}
	if evaluation.Disposition != disposition || evaluation.Outcome != outcome {
		return absent, errEventsRecord
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	record, ok := eventsExpected(root, responseRead, rawResult, scanner, x, transactionID, payload, journal, read)
	if !ok {
		return absent, errEventsRecord
	}
	b, e := canonicalEvents(record)
	if e != nil {
		return absent, errEventsRecord
	}
	digest := eventsDigest(b)
	selector := eventsSelector(digest)
	precommit, uncertain := false, false
	receipt, e := publication.PublishBoundFileWithTrace(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return errEventsRecord
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
	if e != nil || receipt == nil {
		if precommit && !uncertain {
			return absent, errEventsRecord
		}
		return privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}, errEventsRecord
	}
	state := privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, privateDigest(b), len(b)) || !replayEventsInternal(root, state, responseRead, rawResult, scanner, x, transactionID, payload, journal, read, true) {
		return state, errEventsRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
