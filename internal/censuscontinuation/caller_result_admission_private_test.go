package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

const (
	assertAdmissionExactReceipt = "ASSERT_CALLER_RESULT_ADMISSION_EXACT_RECEIPT"
	assertAdmissionStrictParser = "ASSERT_CALLER_RESULT_ADMISSION_STRICT_PARSER"
	assertAdmissionPrePut       = "ASSERT_CALLER_RESULT_ADMISSION_PRE_PUT"
	assertAdmissionReadback     = "ASSERT_CALLER_RESULT_ADMISSION_READBACK"
	assertAdmissionReplay       = "ASSERT_CALLER_RESULT_ADMISSION_REPLAY"
	assertAdmissionPreserves    = "ASSERT_CALLER_RESULT_ADMISSION_PRESERVES_PREDECESSORS"
)

func privateAdmissionFixture(t *testing.T, kind string) (*MemoryStore, string, privateCallerResult, []byte) {
	t.Helper()
	store, _, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	resultSelector, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, privateCallerSubmission(t, privateCallerResultObserved, kind, requests))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.Get(context.Background(), resultSelector)
	if err != nil {
		t.Fatal(err)
	}
	result, err := parsePrivateCallerResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	return store, resultSelector, result, raw
}

func privateAdmissionReidentify(w privateCallerResultAdmissionWire) []byte {
	w.AdmissionID = identityJSON(privateCallerResultAdmissionIdentityDomain, w, func(v *privateCallerResultAdmissionWire) { v.AdmissionID = "" })
	raw, _ := json.Marshal(w)
	return raw
}

func TestPrivateCallerResultAdmissionExactReceiptAllOutcomeArms(t *testing.T) {
	for _, kind := range []string{privateCallerOutcomeSuccess, privateCallerOutcomePartial, privateCallerOutcomeFailed, privateCallerOutcomeRefused, privateCallerOutcomeUnknown} {
		t.Run(kind, func(t *testing.T) {
			store, resultSelector, result, resultRaw := privateAdmissionFixture(t, kind)
			selector, err := AdmitPrivateCallerResultSelector(context.Background(), store, resultSelector)
			if err != nil {
				t.Fatalf("%s: %v", assertAdmissionExactReceipt, err)
			}
			raw, err := store.Get(context.Background(), selector)
			if err != nil {
				t.Fatal(err)
			}
			admission, err := parsePrivateCallerResultAdmission(raw)
			if err != nil {
				t.Fatalf("%s: %v", assertAdmissionExactReceipt, err)
			}
			w := admission.wire
			if w.SchemaVersion != privateCallerResultAdmissionSchema || w.Status != privateCallerResultAdmissionStatus || w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness || w.SemanticQualification != "NOT_PERFORMED" || w.CandidateGenerationAllowed || w.AutomaticProgressionAllowed {
				t.Fatalf("%s fixed values: %+v", assertAdmissionExactReceipt, w)
			}
			if w.ResultSelector != resultSelector || w.ResultDigest != resultSelector || w.ResultByteLength != uint64(len(resultRaw)) || w.ResultID != result.ID() || w.Custody != result.wire.Custody || w.Provenance != result.wire.Provenance {
				t.Fatalf("%s result binding: %+v", assertAdmissionExactReceipt, w)
			}
			if len(w.Rows) != len(result.wire.Results) || w.Rows == nil {
				t.Fatalf("%s rows: %#v", assertAdmissionExactReceipt, w.Rows)
			}
			for i, row := range w.Rows {
				want := result.wire.Results[i]
				if row.Ordinal != i || row.RecordID != want.RecordID || row.MessageID != want.MessageID || row.LineageIdentity != want.LineageIdentity {
					t.Fatalf("%s row %d: %+v", assertAdmissionExactReceipt, i, row)
				}
			}
			if digestOf(raw) != selector || admission.ID() != w.AdmissionID {
				t.Fatalf("%s identity selector=%q admission=%q", assertAdmissionExactReceipt, selector, admission.ID())
			}
		})
	}
}

func TestPrivateCallerResultAdmissionStrictParserMutations(t *testing.T) {
	store, resultSelector, _, _ := privateAdmissionFixture(t, privateCallerOutcomeSuccess)
	selector, err := AdmitPrivateCallerResultSelector(context.Background(), store, resultSelector)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	admission, _ := parsePrivateCallerResultAdmission(raw)
	mutations := map[string]func(*privateCallerResultAdmissionWire){
		"schema":              func(w *privateCallerResultAdmissionWire) { w.SchemaVersion += "x" },
		"status":              func(w *privateCallerResultAdmissionWire) { w.Status += "x" },
		"authority":           func(w *privateCallerResultAdmissionWire) { w.Authority = 1 },
		"accepted":            func(w *privateCallerResultAdmissionWire) { w.Accepted = true },
		"completeness":        func(w *privateCallerResultAdmissionWire) { w.Completeness = "COMPLETE" },
		"qualification":       func(w *privateCallerResultAdmissionWire) { w.SemanticQualification = "PERFORMED" },
		"candidate":           func(w *privateCallerResultAdmissionWire) { w.CandidateGenerationAllowed = true },
		"progression":         func(w *privateCallerResultAdmissionWire) { w.AutomaticProgressionAllowed = true },
		"result-selector":     func(w *privateCallerResultAdmissionWire) { w.ResultSelector = digestOf([]byte("x")) },
		"result-digest":       func(w *privateCallerResultAdmissionWire) { w.ResultDigest = digestOf([]byte("x")) },
		"prepared-selector":   func(w *privateCallerResultAdmissionWire) { w.PreparedSelector = digestOf([]byte("x")) },
		"prepared-digest":     func(w *privateCallerResultAdmissionWire) { w.PreparedDigest = digestOf([]byte("x")) },
		"checkpoint-selector": func(w *privateCallerResultAdmissionWire) { w.CheckpointSelector = digestOf([]byte("x")) },
		"checkpoint-digest":   func(w *privateCallerResultAdmissionWire) { w.CheckpointDigest = digestOf([]byte("x")) },
		"requests-selector":   func(w *privateCallerResultAdmissionWire) { w.RequestsSelector = digestOf([]byte("x")) },
		"requests-digest":     func(w *privateCallerResultAdmissionWire) { w.RequestsDigest = digestOf([]byte("x")) },
		"requests-count":      func(w *privateCallerResultAdmissionWire) { w.RequestsCount++ },
		"row":                 func(w *privateCallerResultAdmissionWire) { w.Rows[0].RecordID = "not-an-identity" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			w := admission.wire
			w.Rows = append([]privateCallerResultAdmissionRow(nil), w.Rows...)
			mutate(&w)
			if _, err := parsePrivateCallerResultAdmission(privateAdmissionReidentify(w)); err == nil {
				t.Fatalf("%s mutation %s accepted", assertAdmissionStrictParser, name)
			}
		})
	}
	badID := admission.wire
	badID.AdmissionID = digestOf([]byte("wrong"))
	badIDRaw, _ := json.Marshal(badID)
	for name, candidate := range map[string][]byte{
		"id":           badIDRaw,
		"unknown":      bytes.Replace(raw, []byte(`"status":`), []byte(`"unknown":0,"status":`), 1),
		"duplicate":    bytes.Replace(raw, []byte(`"status":`), []byte(`"status":"CALLER_RESULT_MECHANICALLY_ADMITTED","status":`), 1),
		"trailing":     append(append([]byte(nil), raw...), []byte(`{}`)...),
		"noncanonical": append([]byte(" "), raw...),
		"null-rows":    bytes.Replace(raw, mustJSON(admission.wire.Rows), []byte("null"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parsePrivateCallerResultAdmission(candidate); err == nil {
				t.Fatalf("%s invalid %s accepted", assertAdmissionStrictParser, name)
			}
		})
	}
}

type privateAdmissionFaultStore struct {
	Store
	mu              sync.Mutex
	puts            int
	wrongPut        bool
	missingReadback bool
	mutateReadback  bool
	cancel          context.CancelFunc
	last            string
	changeSelector  string
	changeAfter     int
	gets            map[string]int
}

func (s *privateAdmissionFaultStore) Put(ctx context.Context, raw []byte) (string, error) {
	s.mu.Lock()
	s.puts++
	s.mu.Unlock()
	selector, err := s.Store.Put(ctx, raw)
	s.last = selector
	if s.cancel != nil {
		s.cancel()
	}
	if s.wrongPut {
		return digestOf([]byte("wrong selector")), err
	}
	return selector, err
}
func (s *privateAdmissionFaultStore) Get(ctx context.Context, selector string) ([]byte, error) {
	s.mu.Lock()
	if s.gets == nil {
		s.gets = map[string]int{}
	}
	s.gets[selector]++
	n := s.gets[selector]
	s.mu.Unlock()
	if selector == s.last && s.missingReadback {
		return nil, errors.New("missing")
	}
	raw, err := s.Store.Get(ctx, selector)
	if err != nil {
		return nil, err
	}
	if selector == s.last && s.mutateReadback {
		raw[len(raw)-1] ^= 1
	}
	if selector == s.changeSelector && n > s.changeAfter {
		raw = append([]byte(nil), raw...)
		raw[len(raw)-1] ^= 1
	}
	return raw, nil
}

func TestPrivateCallerResultAdmissionFailsBeforePutOnObservedInconsistency(t *testing.T) {
	base, resultSelector, _, _ := privateAdmissionFixture(t, privateCallerOutcomeSuccess)
	store := &privateAdmissionFaultStore{Store: base, changeSelector: resultSelector, changeAfter: 1}
	if selector, err := AdmitPrivateCallerResultSelector(context.Background(), store, resultSelector); err == nil || selector != "" || store.puts != 0 {
		t.Fatalf("%s selector=%q err=%v puts=%d", assertAdmissionPrePut, selector, err, store.puts)
	}
}

func TestPrivateCallerResultAdmissionPutReadbackCancellationReplayAndAlias(t *testing.T) {
	base, resultSelector, _, _ := privateAdmissionFixture(t, privateCallerOutcomeSuccess)
	for name, configure := range map[string]func(*privateAdmissionFaultStore){
		"wrong-put":        func(s *privateAdmissionFaultStore) { s.wrongPut = true },
		"missing-readback": func(s *privateAdmissionFaultStore) { s.missingReadback = true },
		"mutated-readback": func(s *privateAdmissionFaultStore) { s.mutateReadback = true },
	} {
		t.Run(name, func(t *testing.T) {
			s := &privateAdmissionFaultStore{Store: base}
			configure(s)
			if selector, err := AdmitPrivateCallerResultSelector(context.Background(), s, resultSelector); err == nil || selector != "" {
				t.Fatalf("%s %s selector=%q err=%v", assertAdmissionReadback, name, selector, err)
			}
		})
	}
	first, err := AdmitPrivateCallerResultSelector(context.Background(), base, resultSelector)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AdmitPrivateCallerResultSelector(context.Background(), base, resultSelector)
	if err != nil || second != first {
		t.Fatalf("%s replay %q %q %v", assertAdmissionReplay, first, second, err)
	}
	var wg sync.WaitGroup
	out := make(chan string, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, e := AdmitPrivateCallerResultSelector(context.Background(), base, resultSelector)
			out <- s
			errs <- e
		}()
	}
	wg.Wait()
	close(out)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	for s := range out {
		if s != first {
			t.Fatalf("%s concurrent=%q want=%q", assertAdmissionReplay, s, first)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pre := &privateAdmissionFaultStore{Store: base}
	if s, e := AdmitPrivateCallerResultSelector(ctx, pre, resultSelector); !errors.Is(e, context.Canceled) || s != "" || pre.puts != 0 {
		t.Fatalf("pre cancel %q %v %d", s, e, pre.puts)
	}
	fresh := NewMemoryStore()
	base.mu.RLock()
	for k, v := range base.objects {
		if k != first {
			fresh.objects[k] = append([]byte(nil), v...)
		}
	}
	base.mu.RUnlock()
	postCtx, postCancel := context.WithCancel(context.Background())
	post := &privateAdmissionFaultStore{Store: fresh, cancel: postCancel}
	if s, e := AdmitPrivateCallerResultSelector(postCtx, post, resultSelector); !errors.Is(e, context.Canceled) || s != "" {
		t.Fatalf("post cancel %q %v", s, e)
	}
	recovered, err := AdmitPrivateCallerResultSelector(context.Background(), fresh, resultSelector)
	if err != nil || recovered != first {
		t.Fatalf("recovery %q %v want %q", recovered, err, first)
	}
	raw, _ := fresh.Get(context.Background(), first)
	raw[0] ^= 1
	again, _ := fresh.Get(context.Background(), first)
	if raw[0] == again[0] {
		t.Fatal("alias mutated store")
	}
}

func TestPrivateCallerResultAdmissionPreservesPredecessorBytes(t *testing.T) {
	store, resultSelector, result, resultRaw := privateAdmissionFixture(t, privateCallerOutcomeUnknown)
	selectors := []string{resultSelector, result.wire.PreparedSelector, result.wire.CheckpointSelector, result.wire.RequestsSelector}
	before := map[string][]byte{}
	for _, selector := range selectors {
		before[selector], _ = store.Get(context.Background(), selector)
	}
	if _, err := AdmitPrivateCallerResultSelector(context.Background(), store, resultSelector); err != nil {
		t.Fatal(err)
	}
	for _, selector := range selectors {
		after, _ := store.Get(context.Background(), selector)
		if !bytes.Equal(before[selector], after) {
			t.Fatalf("%s changed %s", assertAdmissionPreserves, selector)
		}
	}
	if !bytes.Equal(before[resultSelector], resultRaw) {
		t.Fatalf("%s result snapshot", assertAdmissionPreserves)
	}
}

func TestPrivateCallerResultAdmissionAllowsZeroRequestsWithEmptyRows(t *testing.T) {
	store, resultSelector, _, _ := privateAdmissionFixture(t, privateCallerOutcomeSuccess)
	selector, err := AdmitPrivateCallerResultSelector(context.Background(), store, resultSelector)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	admission, err := parsePrivateCallerResultAdmission(raw)
	if err != nil {
		t.Fatal(err)
	}
	wire := admission.wire
	wire.RequestsCount = 0
	wire.Rows = []privateCallerResultAdmissionRow{}
	zeroRaw := privateAdmissionReidentify(wire)
	zero, err := parsePrivateCallerResultAdmission(zeroRaw)
	if err != nil || zero.wire.RequestsCount != 0 || zero.wire.Rows == nil || len(zero.wire.Rows) != 0 || !bytes.Contains(zeroRaw, []byte(`"rows":[]`)) {
		t.Fatalf("zero receipt=%s admission=%+v err=%v", zeroRaw, zero.wire, err)
	}
}

func TestPrivateCallerResultAdmissionConcurrentDistinctResultsCoexist(t *testing.T) {
	store, firstResult, _, _ := privateAdmissionFixture(t, privateCallerOutcomeSuccess)
	_, secondResult, _, _ := privateAdmissionFixture(t, privateCallerOutcomeUnknown)
	results := make(chan string, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, resultSelector := range []string{firstResult, secondResult} {
		resultSelector := resultSelector
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector, err := AdmitPrivateCallerResultSelector(context.Background(), store, resultSelector)
			results <- selector
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	seen := map[string]bool{}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for selector := range results {
		if !validDigest(selector) || seen[selector] {
			t.Fatalf("distinct selector=%q seen=%v", selector, seen)
		}
		seen[selector] = true
	}
	if len(seen) != 2 {
		t.Fatalf("coexisting=%v", seen)
	}
}

func TestPrivateCallerResultAdmissionSizeBoundaryHelper(t *testing.T) {
	if err := validatePrivateCallerResultAdmissionSize(make([]byte, privateCallerResultAdmissionMaxBytes)); err != nil {
		t.Fatalf("boundary rejected: %v", err)
	}
	if err := validatePrivateCallerResultAdmissionSize(make([]byte, privateCallerResultAdmissionMaxBytes+1)); err == nil {
		t.Fatal("boundary+1 accepted")
	}
}
