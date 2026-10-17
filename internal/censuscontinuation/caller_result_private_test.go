package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
)

func privateCallerSuccessSemantic(verdict string, limitations []string) json.RawMessage {
	value := describeworker.SemanticResponseV2{
		Verdict:              verdict,
		TargetRole:           "explains the bounded target",
		ConsumerNeed:         describeworker.ConsumerNeedV2{Status: describeworker.ConsumerUnresolvedV2},
		ProvidedBehavior:     describeworker.ProvidedBehaviorV2{Value: "provides bounded behavior"},
		BoundaryContribution: "contributes within the supplied boundary",
		Limitations:          limitations,
	}
	raw, _ := json.Marshal(value)
	return raw
}

func privateCallerOutcome(t *testing.T, kind string) json.RawMessage {
	t.Helper()
	var value any
	switch kind {
	case privateCallerOutcomeSuccess:
		value = privateCallerSemanticSubmission{Kind: kind, Semantic: privateCallerSuccessSemantic("COMPLETE", []string{})}
	case privateCallerOutcomePartial:
		value = privateCallerSemanticSubmission{Kind: kind, Semantic: privateCallerSuccessSemantic("ABSTAINED", []string{"caller could not resolve beyond supplied evidence"})}
	case privateCallerOutcomeFailed:
		value = privateCallerFailureOutcome{Kind: kind, Failure: privateCallerFailure{Category: "CALLER_FAILURE", Code: "EVALUATION_FAILED", Detail: "caller evaluation failed", Retryable: true}}
	case privateCallerOutcomeRefused:
		value = privateCallerFailureOutcome{Kind: kind, Failure: privateCallerFailure{Category: "POLICY_REFUSAL", Code: "POLICY_DENIED", Detail: "caller policy denied evaluation"}}
	case privateCallerOutcomeUnknown:
		value = privateCallerUnknownOutcome{Kind: kind, Ambiguities: []privateCallerAmbiguity{{Code: "MULTIPLE_READINGS", Subject: "target role", Options: []string{"reading-a", "reading-b"}}}}
	default:
		t.Fatalf("unknown fixture outcome %q", kind)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func privateCallerPreparedFixture(t *testing.T) (*MemoryStore, Result, privateHandoffPrepared, string, []describerequest.Record) {
	t.Helper()
	store, paused, chain := privateHandoffFixture(t)
	prepared, selector, err := preparePrivateHandoff(context.Background(), store, paused.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	ref := privateRequestsRef(t, chain[len(chain)-1])
	raw, err := store.Get(context.Background(), ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	requests, err := describerequest.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return store, paused, prepared, selector, requests
}

func privateCallerSubmission(t *testing.T, provenance, kind string, requests []describerequest.Record) []byte {
	t.Helper()
	rows := make([]privateCallerResultSubmissionRow, len(requests))
	for i, request := range requests {
		rows[i] = privateCallerResultSubmissionRow{RequestOrdinal: i, RecordID: request.RecordID, MessageID: request.Envelope.MessageID, LineageIdentity: request.Lineage.LineageIdentity, Outcome: privateCallerOutcome(t, kind)}
	}
	raw, err := json.Marshal(privateCallerResultSubmissionWire{SchemaVersion: privateCallerResultSubmissionSchema, Provenance: provenance, Results: rows})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPrivateCallerResultRetainsEveryOutcomeArm(t *testing.T) {
	store, _, prepared, preparedSelector, requests := privateCallerPreparedFixture(t)
	for _, kind := range []string{privateCallerOutcomeSuccess, privateCallerOutcomePartial, privateCallerOutcomeFailed, privateCallerOutcomeRefused, privateCallerOutcomeUnknown} {
		t.Run(kind, func(t *testing.T) {
			provenance := privateCallerResultObserved
			if kind == privateCallerOutcomeUnknown {
				provenance = privateCallerResultSynthetic
			}
			result, selector, err := retainPrivateCallerResult(context.Background(), store, preparedSelector, privateCallerSubmission(t, provenance, kind, requests))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := store.Get(context.Background(), selector)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := parsePrivateCallerResult(raw)
			w := result.wire
			if err != nil || parsed.ID() != result.ID() || digestOf(raw) != selector || w.PreparedSelector != preparedSelector || w.PreparedID != prepared.ID() || w.PreparedDigest != preparedSelector || w.PreparedByteLength == 0 || w.RequestsCount != len(requests) || w.Provenance != provenance || w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness || w.Custody != privateCallerResultCustody {
				t.Fatalf("result=%+v selector=%q parsed=%q err=%v", w, selector, parsed.ID(), err)
			}
			for i, row := range w.Results {
				if row.RequestOrdinal != i || row.RecordID != requests[i].RecordID || row.MessageID != requests[i].Envelope.MessageID || row.LineageIdentity != requests[i].Lineage.LineageIdentity {
					t.Fatalf("row binding %d: %+v", i, row)
				}
			}
		})
	}
}

func TestPrivateCallerResultRejectsInvalidSubmissionBeforePut(t *testing.T) {
	base, _, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	valid := privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeSuccess, requests)
	var decoded privateCallerResultSubmissionWire
	if err := json.Unmarshal(valid, &decoded); err != nil {
		t.Fatal(err)
	}
	invalidArm := func(raw string) []byte {
		copy := decoded
		copy.Results = append([]privateCallerResultSubmissionRow(nil), decoded.Results...)
		copy.Results[0].Outcome = json.RawMessage(raw)
		out, _ := json.Marshal(copy)
		return out
	}
	partialComplete, _ := json.Marshal(privateCallerSemanticSubmission{Kind: privateCallerOutcomePartial, Semantic: privateCallerSuccessSemantic("COMPLETE", []string{})})

	cases := map[string][]byte{
		"successful-empty":  invalidArm(`{"kind":"SUCCESSFUL_EMPTY"}`),
		"cross-arm":         invalidArm(`{"kind":"FAILED","failure":{"category":"CALLER_FAILURE","code":"x","detail":"x","retryable":false},"semantic":{}}`),
		"null-options":      invalidArm(`{"kind":"UNKNOWN","ambiguities":[{"code":"x","subject":"x","options":null}]}`),
		"duplicate-options": invalidArm(`{"kind":"UNKNOWN","ambiguities":[{"code":"x","subject":"x","options":["a","a"]}]}`),
		"partial-complete":  invalidArm(string(partialComplete)),
		"noncanonical":      append([]byte(" "), valid...),
		"unknown-member":    bytes.Replace(valid, []byte(`"provenance":`), []byte(`"extra":0,"provenance":`), 1),
		"duplicate-member":  bytes.Replace(valid, []byte(`"provenance":`), []byte(`"provenance":"CALLER_OBSERVED","provenance":`), 1),
	}
	// A nil results array is structurally distinct from an empty array.
	nilRaw, _ := json.Marshal(privateCallerResultSubmissionWire{SchemaVersion: privateCallerResultSubmissionSchema, Provenance: privateCallerResultObserved, Results: nil})
	cases["null-results"] = nilRaw
	// Exact request identity is never repaired or sorted.
	requestMismatch := decoded
	requestMismatch.Results = append([]privateCallerResultSubmissionRow(nil), decoded.Results...)
	requestMismatch.Results[0].RecordID = string(bytes.Repeat([]byte{'a'}, 64))
	cases["request-substitution"], _ = json.Marshal(requestMismatch)
	if len(requestMismatch.Results) > 1 {
		reordered := decoded
		reordered.Results = append([]privateCallerResultSubmissionRow(nil), decoded.Results...)
		reordered.Results[0], reordered.Results[1] = reordered.Results[1], reordered.Results[0]
		cases["reordered"], _ = json.Marshal(reordered)
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			store := &privateHandoffObservingStore{Store: base}
			if _, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, raw); err == nil || store.puts != 0 {
				t.Fatalf("err=%v puts=%d", err, store.puts)
			}
		})
	}
	store := &privateHandoffObservingStore{Store: base}
	if _, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, make([]byte, privateCallerResultMaxBytes+1)); err == nil || store.puts != 0 {
		t.Fatalf("oversize err=%v puts=%d", err, store.puts)
	}
}

func TestPrivateCallerResultRejectsPreparedAndRequestTamperBeforePut(t *testing.T) {
	base, _, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	submission := privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeSuccess, requests)
	preparedRaw, err := base.Get(context.Background(), preparedSelector)
	if err != nil {
		t.Fatal(err)
	}
	badPrepared := append([]byte(nil), preparedRaw...)
	badPrepared[len(badPrepared)-1] ^= 1
	store := &privateHandoffObservingStore{Store: base, overrides: map[string][]byte{preparedSelector: badPrepared}}
	if _, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, submission); err == nil || store.puts != 0 {
		t.Fatalf("prepared tamper err=%v puts=%d", err, store.puts)
	}

	prepared, err := parsePrivateHandoffPrepared(preparedRaw)
	if err != nil {
		t.Fatal(err)
	}
	requestsRaw, err := base.Get(context.Background(), prepared.wire.RequestsSelector)
	if err != nil {
		t.Fatal(err)
	}
	badRequests := append([]byte(nil), requestsRaw...)
	badRequests[len(badRequests)-1] ^= 1
	store = &privateHandoffObservingStore{Store: base, overrides: map[string][]byte{prepared.wire.RequestsSelector: badRequests}}
	if _, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, submission); err == nil || store.puts != 0 {
		t.Fatalf("request tamper err=%v puts=%d", err, store.puts)
	}
}

type privateCallerReadbackTamperStore struct {
	Store
	last string
}

func (s *privateCallerReadbackTamperStore) Put(ctx context.Context, raw []byte) (string, error) {
	selector, err := s.Store.Put(ctx, raw)
	s.last = selector
	return selector, err
}
func (s *privateCallerReadbackTamperStore) Get(ctx context.Context, selector string) ([]byte, error) {
	raw, err := s.Store.Get(ctx, selector)
	if err == nil && selector == s.last && s.last != "" {
		raw = append([]byte(nil), raw...)
		raw[len(raw)-1] ^= 1
	}
	return raw, err
}

type privateCallerCancelAfterPutStore struct {
	Store
	cancel context.CancelFunc
}

func (s *privateCallerCancelAfterPutStore) Put(ctx context.Context, raw []byte) (string, error) {
	selector, err := s.Store.Put(ctx, raw)
	s.cancel()
	return selector, err
}

func TestPrivateCallerResultReadbackReplayConcurrencyAndCancellation(t *testing.T) {
	base, _, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	submission := privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeSuccess, requests)

	tamper := &privateCallerReadbackTamperStore{Store: base}
	if _, err := RetainPrivateCallerResultSelector(context.Background(), tamper, preparedSelector, submission); err == nil {
		t.Fatal("tampered readback accepted")
	}

	first, err := RetainPrivateCallerResultSelector(context.Background(), base, preparedSelector, submission)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RetainPrivateCallerResultSelector(context.Background(), base, preparedSelector, append([]byte(nil), submission...))
	if err != nil || first != second {
		t.Fatalf("replay first=%q second=%q err=%v", first, second, err)
	}

	selectors := make(chan string, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector, err := RetainPrivateCallerResultSelector(context.Background(), base, preparedSelector, submission)
			selectors <- selector
			errs <- err
		}()
	}
	wg.Wait()
	close(selectors)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for selector := range selectors {
		if selector != first {
			t.Fatalf("concurrent selector=%q want=%q", selector, first)
		}
	}

	different := privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeUnknown, requests)
	differentSelector, err := RetainPrivateCallerResultSelector(context.Background(), base, preparedSelector, different)
	if err != nil || differentSelector == first {
		t.Fatalf("distinct selector=%q first=%q err=%v", differentSelector, first, err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	store := &privateHandoffObservingStore{Store: base}
	if _, err := RetainPrivateCallerResultSelector(cancelled, store, preparedSelector, submission); !errors.Is(err, context.Canceled) || store.puts != 0 {
		t.Fatalf("pre-cancel err=%v puts=%d", err, store.puts)
	}

	ctx, cancelAfterPut := context.WithCancel(context.Background())
	post := &privateCallerCancelAfterPutStore{Store: base, cancel: cancelAfterPut}
	if _, err := RetainPrivateCallerResultSelector(ctx, post, preparedSelector, submission); !errors.Is(err, context.Canceled) {
		t.Fatalf("post-install cancellation err=%v", err)
	}
	recovered, err := RetainPrivateCallerResultSelector(context.Background(), base, preparedSelector, submission)
	if err != nil || recovered != first {
		t.Fatalf("recovery selector=%q want=%q err=%v", recovered, first, err)
	}
}

func TestPrivateCallerResultPreservesPublicAndCheckpointContracts(t *testing.T) {
	store, paused, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	beforePaused, _ := json.Marshal(paused)
	beforeStages := append([]Stage(nil), stageOrder...)
	beforeKinds := append([]string(nil), expectedArtifactKinds(StageRequestsRendered)...)
	preparedBefore, err := store.Get(context.Background(), preparedSelector)
	if err != nil {
		t.Fatal(err)
	}
	submission := privateCallerSubmission(t, privateCallerResultSynthetic, privateCallerOutcomeUnknown, requests)
	inputAlias := append([]byte(nil), submission...)
	selector, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, inputAlias)
	if err != nil {
		t.Fatal(err)
	}
	for i := range inputAlias {
		inputAlias[i] = 'x'
	}
	retained, err := store.Get(context.Background(), selector)
	if err != nil {
		t.Fatal(err)
	}
	retained[0] ^= 1
	retainedAgain, err := store.Get(context.Background(), selector)
	if err != nil {
		t.Fatal(err)
	}
	if retainedAgain[0] == retained[0] {
		t.Fatal("store output alias mutated retained result")
	}
	afterPaused, _ := json.Marshal(paused)
	preparedAfter, _ := store.Get(context.Background(), preparedSelector)
	if !bytes.Equal(beforePaused, afterPaused) || !bytes.Equal(preparedBefore, preparedAfter) || !equalStages(beforeStages, stageOrder) || !equalStrings(beforeKinds, expectedArtifactKinds(StageRequestsRendered)) || paused.Status != StatusPaused {
		t.Fatalf("public/checkpoint contract changed")
	}
}

func equalStages(a, b []Stage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
