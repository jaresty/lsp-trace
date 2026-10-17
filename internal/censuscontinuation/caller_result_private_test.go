package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	// Exact request identities are never repaired or sorted.
	for name, mutate := range map[string]func(*privateCallerResultSubmissionRow){
		"record-substitution":  func(row *privateCallerResultSubmissionRow) { row.RecordID = string(bytes.Repeat([]byte{'a'}, 64)) },
		"message-substitution": func(row *privateCallerResultSubmissionRow) { row.MessageID = string(bytes.Repeat([]byte{'b'}, 64)) },
		"lineage-substitution": func(row *privateCallerResultSubmissionRow) {
			row.LineageIdentity = string(bytes.Repeat([]byte{'c'}, 64))
		},
	} {
		requestMismatch := decoded
		requestMismatch.Results = append([]privateCallerResultSubmissionRow(nil), decoded.Results...)
		mutate(&requestMismatch.Results[0])
		cases[name], _ = json.Marshal(requestMismatch)
	}
	if len(decoded.Results) > 1 {
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

	// Use a clone without this result so cancellation demonstrably occurs after a new immutable install.
	fresh := NewMemoryStore()
	base.mu.RLock()
	for selector, raw := range base.objects {
		if selector != first {
			fresh.objects[selector] = append([]byte(nil), raw...)
		}
	}
	base.mu.RUnlock()
	ctx, cancelAfterPut := context.WithCancel(context.Background())
	post := &privateCallerCancelAfterPutStore{Store: fresh, cancel: cancelAfterPut}
	postSelector, err := RetainPrivateCallerResultSelector(ctx, post, preparedSelector, submission)
	if !errors.Is(err, context.Canceled) || postSelector != "" {
		t.Fatalf("post-install cancellation selector=%q err=%v", postSelector, err)
	}
	installed, err := fresh.Get(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parsePrivateCallerResult(installed)
	if err != nil || parsed.ID() == "" {
		t.Fatalf("installed result parse id=%q err=%v", parsed.ID(), err)
	}
	recovered, err := RetainPrivateCallerResultSelector(context.Background(), fresh, preparedSelector, submission)
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

func privateCallerRetainedFixture(t *testing.T) (*MemoryStore, string, []byte, privateCallerResult) {
	t.Helper()
	store, _, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	selector, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeSuccess, requests))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.Get(context.Background(), selector)
	if err != nil {
		t.Fatal(err)
	}
	result, err := parsePrivateCallerResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	return store, selector, raw, result
}

func privateCallerReidentify(w privateCallerResultWire) []byte {
	w.ResultID = identityJSON(privateCallerResultIdentityDomain, w, func(v *privateCallerResultWire) { v.ResultID = "" })
	raw, _ := json.Marshal(w)
	return raw
}

func TestPrivateCallerResultParserRejectsRetainedMutations(t *testing.T) {
	_, _, raw, result := privateCallerRetainedFixture(t)
	mutations := map[string]func(*privateCallerResultWire){
		"record-id-syntax":  func(w *privateCallerResultWire) { w.Results[0].RecordID = "not-an-identity" },
		"message-id-syntax": func(w *privateCallerResultWire) { w.Results[0].MessageID = "not-an-identity" },
		"lineage-id-syntax": func(w *privateCallerResultWire) { w.Results[0].LineageIdentity = "not-an-identity" },
		"null-outcome":      func(w *privateCallerResultWire) { w.Results[0].Outcome = nil },
		"outcome-digest": func(w *privateCallerResultWire) {
			var arm privateCallerSemanticOutcome
			_ = json.Unmarshal(w.Results[0].Outcome, &arm)
			arm.RawSemanticSHA256 = digestOf([]byte("wrong"))
			w.Results[0].Outcome, _ = json.Marshal(arm)
		},
		"outcome-length": func(w *privateCallerResultWire) {
			var arm privateCallerSemanticOutcome
			_ = json.Unmarshal(w.Results[0].Outcome, &arm)
			arm.RawSemanticByteLength++
			w.Results[0].Outcome, _ = json.Marshal(arm)
		},
		"canonical-semantic": func(w *privateCallerResultWire) {
			var arm privateCallerSemanticOutcome
			_ = json.Unmarshal(w.Results[0].Outcome, &arm)
			// Keep the SUCCESS/COMPLETE semantic payload valid while making its
			// byte representation noncanonical. Recompute every enclosing digest
			// so canonical-byte validation is the sole failing invariant.
			arm.CanonicalSemantic = append([]byte(" "), arm.CanonicalSemantic...)
			arm.RawSemanticSHA256 = digestOf(arm.CanonicalSemantic)
			arm.RawSemanticByteLength = uint64(len(arm.CanonicalSemantic))
			w.Results[0].Outcome, _ = json.Marshal(arm)
		},
	}
	if len(result.wire.Results) > 1 {
		mutations["duplicate-record"] = func(w *privateCallerResultWire) { w.Results[1].RecordID = w.Results[0].RecordID }
		mutations["duplicate-message"] = func(w *privateCallerResultWire) { w.Results[1].MessageID = w.Results[0].MessageID }
		mutations["duplicate-lineage"] = func(w *privateCallerResultWire) { w.Results[1].LineageIdentity = w.Results[0].LineageIdentity }
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			wire := result.wire
			wire.Results = append([]privateCallerResultRow(nil), result.wire.Results...)
			mutate(&wire)
			if _, err := parsePrivateCallerResult(privateCallerReidentify(wire)); err == nil {
				t.Fatal("mutated retained artifact accepted")
			}
		})
	}
	identity := result.wire
	identity.ResultID = digestOf([]byte("wrong result identity"))
	identityRaw, _ := json.Marshal(identity)
	if _, err := parsePrivateCallerResult(identityRaw); err == nil {
		t.Fatal("result identity mutation accepted")
	}
	for name, bad := range map[string][]byte{
		"unknown":      bytes.Replace(raw, []byte(`"status":`), []byte(`"unknown":0,"status":`), 1),
		"duplicate":    bytes.Replace(raw, []byte(`"status":`), []byte(`"status":"CALLER_RESULT_RETAINED","status":`), 1),
		"trailing":     append(append([]byte(nil), raw...), []byte(`{}`)...),
		"null-results": bytes.Replace(raw, mustJSON(result.wire.Results), []byte("null"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parsePrivateCallerResult(bad); err == nil {
				t.Fatal("invalid retained JSON accepted")
			}
		})
	}
}

func mustJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }

func privateCallerPreparedRaw(w privateHandoffPreparedWire) []byte {
	w.PreparedID = identityJSON("lsp-trace:census-handoff-prepared:private:v1", w, func(v *privateHandoffPreparedWire) { v.PreparedID = "" })
	raw, _ := json.Marshal(w)
	return raw
}

func TestPrivateCallerResultRevalidatesEveryPreparedBindingBeforePut(t *testing.T) {
	base, _, prepared, _, requests := privateCallerPreparedFixture(t)
	submission := privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeSuccess, requests)
	mutations := map[string]func(*privateHandoffPreparedWire){
		"checkpoint-selector": func(w *privateHandoffPreparedWire) {
			w.CheckpointSelector = digestOf([]byte("other checkpoint selector"))
		},
		"checkpoint-id":     func(w *privateHandoffPreparedWire) { w.CheckpointID = digestOf([]byte("other checkpoint id")) },
		"chain-identity":    func(w *privateHandoffPreparedWire) { w.ChainIdentity = digestOf([]byte("other chain")) },
		"requests-selector": func(w *privateHandoffPreparedWire) { w.RequestsSelector = digestOf([]byte("other requests selector")) },
		"requests-digest":   func(w *privateHandoffPreparedWire) { w.RequestsDigest = digestOf([]byte("other requests digest")) },
		"requests-length":   func(w *privateHandoffPreparedWire) { w.RequestsByteLength++ },
		"requests-count":    func(w *privateHandoffPreparedWire) { w.RequestsCount++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			wire := prepared.wire
			mutate(&wire)
			raw := privateCallerPreparedRaw(wire)
			selector, err := base.Put(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			store := &privateHandoffObservingStore{Store: base}
			if _, err := RetainPrivateCallerResultSelector(context.Background(), store, selector, submission); err == nil || store.puts != 0 {
				t.Fatalf("err=%v puts=%d", err, store.puts)
			}
		})
	}
}

func TestPrivateCallerResultOversizedEncodedArtifactDoesNotWrite(t *testing.T) {
	store := &privateHandoffObservingStore{Store: NewMemoryStore()}
	wire := privateCallerResultWire{SchemaVersion: privateCallerResultSchema, Status: privateCallerResultStatus, Completeness: ContinuationCompleteness, Custody: privateCallerResultCustody, Provenance: privateCallerResultObserved, PreparedSelector: digestOf([]byte("p")), PreparedID: digestOf([]byte("pi")), PreparedDigest: digestOf([]byte("p")), PreparedByteLength: 1, CheckpointSelector: digestOf([]byte("c")), CheckpointID: digestOf([]byte("ci")), ChainIdentity: digestOf([]byte("chain")), RequestsSelector: digestOf([]byte("r")), RequestsDigest: digestOf([]byte("r")), RequestsByteLength: 1}
	outcome, _ := json.Marshal(privateCallerUnknownOutcome{Kind: privateCallerOutcomeUnknown, Ambiguities: []privateCallerAmbiguity{{Code: "x", Subject: strings.Repeat("s", privateCallerResultMaxTextBytes), Options: []string{"x"}}}})
	const rows = 16500
	wire.RequestsCount = rows
	wire.Results = make([]privateCallerResultRow, rows)
	for i := range wire.Results {
		id := func(domain string) string {
			return strings.TrimPrefix(digestOf([]byte(domain+string(rune(i)))), "sha256:")
		}
		wire.Results[i] = privateCallerResultRow{RequestOrdinal: i, RecordID: id("record"), MessageID: id("message"), LineageIdentity: id("lineage"), Outcome: outcome}
	}
	wire.ResultID = identityJSON(privateCallerResultIdentityDomain, wire, func(v *privateCallerResultWire) { v.ResultID = "" })
	if _, err := persistPrivateCallerResult(context.Background(), store, privateCallerResult{wire: wire}); err == nil || store.puts != 0 {
		t.Fatalf("err=%v puts=%d", err, store.puts)
	}
}

func TestPrivateCallerResultConcurrentDistinctSubmissionsCoexist(t *testing.T) {
	store, _, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	submissions := [][]byte{
		privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeSuccess, requests),
		privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeUnknown, requests),
	}
	type result struct {
		selector string
		err      error
	}
	results := make(chan result, len(submissions))
	var wg sync.WaitGroup
	for _, submission := range submissions {
		submission := append([]byte(nil), submission...)
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, submission)
			results <- result{selector, err}
		}()
	}
	wg.Wait()
	close(results)
	seen := map[string]bool{}
	for got := range results {
		if got.err != nil || !validDigest(got.selector) || seen[got.selector] {
			t.Fatalf("result=%+v seen=%v", got, seen)
		}
		seen[got.selector] = true
		raw, err := store.Get(context.Background(), got.selector)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parsePrivateCallerResult(raw); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("selectors=%v", seen)
	}
}

func privateCallerChainIdentity(chain []Checkpoint) string {
	ids := make([]string, len(chain))
	for i := range chain {
		ids[i] = chain[i].ID()
	}
	return identityJSON("lsp-trace:census-handoff-prepared-chain:private:v1", ids, func(*[]string) {})
}

func privateCallerStorePreparedForCheckpoint(t *testing.T, store *MemoryStore, template privateHandoffPrepared, selector string, checkpointID string, chainIdentity string) string {
	t.Helper()
	wire := template.wire
	wire.CheckpointSelector = selector
	wire.CheckpointID = checkpointID
	wire.ChainIdentity = chainIdentity
	raw := privateCallerPreparedRaw(wire)
	preparedSelector, err := store.Put(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	return preparedSelector
}

func TestPrivateCallerResultRejectsCheckpointStageStatusAndMembershipBeforePut(t *testing.T) {
	store, _, prepared, _, requests := privateCallerPreparedFixture(t)
	submission := privateCallerSubmission(t, privateCallerResultObserved, privateCallerOutcomeSuccess, requests)
	chain, err := VerifyChain(context.Background(), store, prepared.wire.CheckpointSelector)
	if err != nil || len(chain) < 2 {
		t.Fatalf("chain=%d err=%v", len(chain), err)
	}
	last := chain[len(chain)-1]

	// A valid earlier-stage chain remains ineligible for result retention.
	priorSelector := last.PriorID()
	prior := chain[len(chain)-2]
	preparedSelector := privateCallerStorePreparedForCheckpoint(t, store, prepared, priorSelector, prior.ID(), privateCallerChainIdentity(chain[:len(chain)-1]))
	observed := &privateHandoffObservingStore{Store: store}
	if _, err := RetainPrivateCallerResultSelector(context.Background(), observed, preparedSelector, submission); err == nil || observed.puts != 0 {
		t.Fatalf("stage err=%v puts=%d", err, observed.puts)
	}

	// A valid chain ending at REQUESTS_RENDERED but with COMPLETE status is also ineligible.
	complete, err := NewCheckpoint(CheckpointInput{PriorCheckpointID: last.PriorID(), Stage: last.Stage(), CensusID: last.CensusID(), HandoffID: last.HandoffID(), ContractID: last.ContractID(), ProfileID: last.ProfileID(), Artifacts: last.Artifacts(), Status: StatusComplete})
	if err != nil {
		t.Fatal(err)
	}
	completeRaw, err := complete.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	completeSelector, err := store.Put(context.Background(), completeRaw)
	if err != nil {
		t.Fatal(err)
	}
	completeChain := append(append([]Checkpoint(nil), chain[:len(chain)-1]...), complete)
	preparedSelector = privateCallerStorePreparedForCheckpoint(t, store, prepared, completeSelector, complete.ID(), privateCallerChainIdentity(completeChain))
	observed = &privateHandoffObservingStore{Store: store}
	if _, err := RetainPrivateCallerResultSelector(context.Background(), observed, preparedSelector, submission); err == nil || observed.puts != 0 {
		t.Fatalf("status err=%v puts=%d", err, observed.puts)
	}

	for name, mutate := range map[string]func(*checkpointWire){
		"missing-requests-artifact":   func(w *checkpointWire) { w.Artifacts = append([]ArtifactRef(nil), w.Artifacts[:len(w.Artifacts)-1]...) },
		"duplicate-requests-artifact": func(w *checkpointWire) { w.Artifacts = append(w.Artifacts, w.Artifacts[len(w.Artifacts)-1]) },
	} {
		t.Run(name, func(t *testing.T) {
			wire := last.wire
			wire.Artifacts = append([]ArtifactRef(nil), last.wire.Artifacts...)
			mutate(&wire)
			wire.CheckpointID = identityJSON("lsp-trace:census-continuation-checkpoint:v1", wire, func(v *checkpointWire) { v.CheckpointID = "" })
			raw, _ := json.Marshal(wire)
			selector, err := store.Put(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			mutatedChain := append(append([]Checkpoint(nil), chain[:len(chain)-1]...), Checkpoint{wire: wire})
			preparedSelector := privateCallerStorePreparedForCheckpoint(t, store, prepared, selector, wire.CheckpointID, privateCallerChainIdentity(mutatedChain))
			if _, err := VerifyChain(context.Background(), store, selector); err == nil || !strings.Contains(err.Error(), "checkpoint artifact membership mismatch") {
				t.Fatalf("VerifyChain err=%v", err)
			}
			observed := &privateHandoffObservingStore{Store: store}
			if _, err := RetainPrivateCallerResultSelector(context.Background(), observed, preparedSelector, submission); err == nil || observed.puts != 0 {
				t.Fatalf("err=%v puts=%d", err, observed.puts)
			}
		})
	}
}
