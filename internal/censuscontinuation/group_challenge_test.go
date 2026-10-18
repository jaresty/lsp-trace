package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
)

func privateGroupChallengeFixture(t *testing.T) (*MemoryStore, string, privateGroupingArtifact) {
	t.Helper()
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	ids := []string{predecessor.wire.Candidates[0].CandidateID, predecessor.wire.Candidates[1].CandidateID}
	sort.Strings(ids)
	groups := []privateGroupingSubmissionGroup{
		{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: ids, Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}},
		{ClientGroupKey: "b", Kind: privateGroupingSemantic, CandidateIDs: []string{ids[0]}, Label: "b", Claim: "b", Basis: "b", RejectionConditions: []string{"b"}},
		{ClientGroupKey: "c", Kind: privateGroupingSingleton, CandidateIDs: []string{ids[1]}, RejectionConditions: []string{}},
	}
	selector, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, privateGroupingSubmission(t, groups, []privateGroupingSubmissionAlternative{{LeftClientGroupKey: "a", RightClientGroupKey: "b"}}))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, err := parsePrivateGroupingArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	return store, selector, artifact
}

func privateGroupChallengeSubmission(t *testing.T, predecessor privateGroupingArtifact, groupVerdicts map[string]string, alternativeVerdict string) []byte {
	t.Helper()
	groups := append([]privateGroupingGroup(nil), predecessor.wire.Groups...)
	sort.Slice(groups, func(i, j int) bool { return groups[i].GroupID < groups[j].GroupID })
	groupRecords := make([]privateGroupChallengeSubmissionRecord, len(groups))
	for i, g := range groups {
		verdict := groupVerdicts[g.Kind]
		if verdict == privateChallengeSplit && len(g.CandidateIDs) == 1 {
			verdict = privateChallengeRevise
		}
		proposals := [][]string{}
		if verdict == privateChallengeRevise {
			proposals = [][]string{append([]string{}, g.CandidateIDs...)}
		}
		if verdict == privateChallengeSplit {
			proposals = [][]string{{g.CandidateIDs[0]}, {g.CandidateIDs[1]}}
		}
		groupRecords[i] = privateGroupChallengeSubmissionRecord{g.GroupID, verdict, proposals}
	}
	alternatives := append([]privateGroupingAlternative(nil), predecessor.wire.Alternatives...)
	sort.Slice(alternatives, func(i, j int) bool { return alternatives[i].AlternativeID < alternatives[j].AlternativeID })
	alternativeRecords := make([]privateAlternativeChallengeSubmissionRecord, len(alternatives))
	for i, a := range alternatives {
		var refs *[]string
		if alternativeVerdict == privateChallengeRevise {
			v := []string{a.LeftGroupID, a.RightGroupID}
			sort.Strings(v)
			refs = &v
		}
		alternativeRecords[i] = privateAlternativeChallengeSubmissionRecord{a.AlternativeID, alternativeVerdict, refs}
	}
	raw, err := json.Marshal(privateGroupChallengeSubmissionWire{SchemaVersion: privateGroupChallengeSubmissionSchema, ProducerProcessRef: "producer", ChallengerProcessRef: "challenger", RecordRef: nil, GroupChallenges: groupRecords, AlternativeChallenges: alternativeRecords})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPrivateGroupChallengeRetainsExactClosedChallenge(t *testing.T) {
	store, groupingSelector, predecessor := privateGroupChallengeFixture(t)
	submission := privateGroupChallengeSubmission(t, predecessor, map[string]string{privateGroupingSemantic: privateChallengeSplit, privateGroupingSingleton: privateChallengeRetain}, privateChallengeRevise)
	selector, err := RetainPrivateGroupChallengeSelector(context.Background(), store, groupingSelector, submission)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, err := parsePrivateGroupChallengeArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	w := artifact.wire
	if w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown || w.FeatureIdentity != "UNRESOLVED" || w.AdjudicationStatus != "NOT_PERFORMED" || w.AutomaticProgressionAllowed || w.GroupingMutated {
		t.Fatalf("ceiling: %+v", w)
	}
	if w.ChallengeProvenance != "CALLER_ASSERTED_DISTINCT_PROCESS_REFS" || w.ChallengeAuthentication != "UNAUTHENTICATED" || w.ChallengeStatus != "CALLER_ASSERTED_CHALLENGE_RETAINED" || w.IndependenceStatus != "NOT_ESTABLISHED" {
		t.Fatalf("provenance: %+v", w)
	}
	if w.GroupingSelector != groupingSelector || w.GroupingDigest != groupingSelector || w.GroupingArtifactID != predecessor.ID() || w.GroupingByteLength == 0 {
		t.Fatalf("binding: %+v", w)
	}
	if w.Accounting != (privateGroupChallengeAccounting{2, 3, 1, 3, 1, 4}) {
		t.Fatalf("accounting: %+v", w.Accounting)
	}
	for _, forbidden := range []string{`"candidate_snapshots"`, `"outcome"`, `"source_refs"`, `"prepared_count"`, `"requests_count"`, `"distinct_outcome_count"`, `"supersession"`, `"revision"`, `"head"`, `"current"`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("forbidden %s", forbidden)
		}
	}
	second, err := RetainPrivateGroupChallengeSelector(context.Background(), store, groupingSelector, append([]byte{}, submission...))
	if err != nil || second != selector {
		t.Fatalf("replay %q %v", second, err)
	}
}

func TestPrivateGroupChallengeVerdictAndProposalMatrix(t *testing.T) {
	store, selector, p := privateGroupChallengeFixture(t)
	valid := privateGroupChallengeSubmission(t, p, map[string]string{privateGroupingSemantic: privateChallengeRetain, privateGroupingSingleton: privateChallengeRetain}, privateChallengeRetain)
	var base privateGroupChallengeSubmissionWire
	if err := json.Unmarshal(valid, &base); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*privateGroupChallengeSubmissionWire){
		"equal-process": func(w *privateGroupChallengeSubmissionWire) { w.ChallengerProcessRef = w.ProducerProcessRef },
		"missing-group": func(w *privateGroupChallengeSubmissionWire) {
			w.GroupChallenges = w.GroupChallenges[:len(w.GroupChallenges)-1]
		},
		"duplicate-group": func(w *privateGroupChallengeSubmissionWire) {
			w.GroupChallenges[1].TargetGroupID = w.GroupChallenges[0].TargetGroupID
		},
		"unordered-group": func(w *privateGroupChallengeSubmissionWire) {
			w.GroupChallenges[0], w.GroupChallenges[1] = w.GroupChallenges[1], w.GroupChallenges[0]
		},
		"retain-proposal": func(w *privateGroupChallengeSubmissionWire) {
			w.GroupChallenges[0].ProposedCandidateGroups = [][]string{{p.wire.Groups[0].CandidateIDs[0]}}
		},
		"revise-empty": func(w *privateGroupChallengeSubmissionWire) {
			w.GroupChallenges[0].Verdict = privateChallengeRevise
			w.GroupChallenges[0].ProposedCandidateGroups = [][]string{}
		},
		"split-single": func(w *privateGroupChallengeSubmissionWire) {
			for i := range w.GroupChallenges {
				g := groupByChallengeID(p, w.GroupChallenges[i].TargetGroupID)
				if len(g.CandidateIDs) > 1 {
					w.GroupChallenges[i].Verdict = privateChallengeSplit
					w.GroupChallenges[i].ProposedCandidateGroups = [][]string{append([]string{}, g.CandidateIDs...)}
					break
				}
			}
		},
		"singleton-split": func(w *privateGroupChallengeSubmissionWire) {
			for i := range w.GroupChallenges {
				g := groupByChallengeID(p, w.GroupChallenges[i].TargetGroupID)
				if g.Kind == privateGroupingSingleton {
					w.GroupChallenges[i].Verdict = privateChallengeSplit
					w.GroupChallenges[i].ProposedCandidateGroups = [][]string{{g.CandidateIDs[0]}, {g.CandidateIDs[0]}}
				}
			}
		},
		"alternative-split": func(w *privateGroupChallengeSubmissionWire) {
			w.AlternativeChallenges[0].Verdict = privateChallengeSplit
		},
		"alternative-revise-null": func(w *privateGroupChallengeSubmissionWire) {
			w.AlternativeChallenges[0].Verdict = privateChallengeRevise
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			w := base
			w.GroupChallenges = append([]privateGroupChallengeSubmissionRecord(nil), base.GroupChallenges...)
			w.AlternativeChallenges = append([]privateAlternativeChallengeSubmissionRecord(nil), base.AlternativeChallenges...)
			mutate(&w)
			raw, _ := json.Marshal(w)
			if got, err := RetainPrivateGroupChallengeSelector(context.Background(), store, selector, raw); err == nil || got != "" {
				t.Fatalf("selector=%q err=%v", got, err)
			}
		})
	}
}
func groupByChallengeID(p privateGroupingArtifact, id string) privateGroupingGroup {
	for _, g := range p.wire.Groups {
		if g.GroupID == id {
			return g
		}
	}
	return privateGroupingGroup{}
}

func TestPrivateGroupChallengeArtifactWideWorkBound(t *testing.T) {
	bounds := privateGroupChallengeFrozenBounds()
	if bounds.MaxWork != privateGroupChallengeMaxWork {
		t.Fatalf("MaxWork=%d want=%d", bounds.MaxWork, privateGroupChallengeMaxWork)
	}
	atLimit := [][][]string{{make([]string, bounds.MaxWork-5)}, {{}}}
	if err := validatePrivateGroupChallengeWork(2, atLimit, 1, nil, bounds.MaxWork); err != nil {
		t.Fatalf("at limit: %v", err)
	}
	firstHalf := bounds.MaxWork / 2
	plusOne := [][][]string{{make([]string, firstHalf)}, {make([]string, bounds.MaxWork-3-firstHalf)}}
	if err := validatePrivateGroupChallengeWork(2, plusOne, 0, nil, bounds.MaxWork); err == nil || err.Error() != "private group challenge work limit" {
		t.Fatalf("distributed plus one: %v", err)
	}
	if err := validatePrivateGroupChallengeWork(math.MaxInt, nil, 1, nil, math.MaxInt); err == nil || err.Error() != "private group challenge work limit" {
		t.Fatalf("overflow-safe oversized count: %v", err)
	}

	store, selector, predecessor := privateGroupChallengeFixture(t)
	raw := privateGroupChallengeSubmission(t, predecessor, map[string]string{privateGroupingSemantic: privateChallengeRetain, privateGroupingSingleton: privateChallengeRetain}, privateChallengeRetain)
	var submission privateGroupChallengeSubmissionWire
	if err := json.Unmarshal(raw, &submission); err != nil {
		t.Fatal(err)
	}
	for i := range submission.GroupChallenges {
		submission.GroupChallenges[i].ProposedCandidateGroups = make([][]string, 6_000)
	}
	raw, err := json.Marshal(submission)
	if err != nil {
		t.Fatal(err)
	}
	fault := &privateGroupChallengeFaultStore{Store: store}
	if got, err := RetainPrivateGroupChallengeSelector(context.Background(), fault, selector, raw); err == nil || err.Error() != "private group challenge work limit" || got != "" || fault.puts != 0 {
		t.Fatalf("distributed rejection selector=%q err=%v puts=%d", got, err, fault.puts)
	}
}

func TestPrivateGroupChallengeProfileDigestIncludesMaxWork(t *testing.T) {
	const want = "sha256:686743b48acb3bbc29ae9e66a83b24194deff07b42a50970968bbe66533c7b2e"
	if got := privateGroupChallengeComputedProfileDigest(); got != want {
		t.Fatalf("profile digest=%s want=%s", got, want)
	}
	bounds := privateGroupChallengeFrozenBounds()
	bounds.MaxWork++
	if got := identityJSON(privateGroupChallengeProfileDomain, bounds, func(*privateGroupChallengeBounds) {}); got == privateGroupChallengeComputedProfileDigest() {
		t.Fatal("MaxWork mutation did not change profile digest")
	}
}

func TestPrivateGroupChallengeStrictWireBoundsAndForbiddenFields(t *testing.T) {
	store, selector, p := privateGroupChallengeFixture(t)
	valid := privateGroupChallengeSubmission(t, p, map[string]string{privateGroupingSemantic: privateChallengeRetain, privateGroupingSingleton: privateChallengeRetain}, privateChallengeRetain)
	cases := map[string][]byte{"nil": nil, "null": []byte("null"), "whitespace": append([]byte(" "), valid...), "trailing": append(append([]byte{}, valid...), byte('\n')), "unknown": bytes.Replace(valid, []byte(`"group_challenges":`), []byte(`"source_refs":[],"group_challenges":`), 1), "duplicate": bytes.Replace(valid, []byte(`"schema_version":`), []byte(`"schema_version":"x","schema_version":`), 1), "two-values": append(append([]byte{}, valid...), []byte("{}")...), "oversize": make([]byte, privateGroupChallengeMaxBytes+1)}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := RetainPrivateGroupChallengeSelector(context.Background(), store, selector, raw); err == nil || got != "" {
				t.Fatalf("selector=%q err=%v", got, err)
			}
		})
	}
	var w privateGroupChallengeSubmissionWire
	_ = json.Unmarshal(valid, &w)
	w.ProducerProcessRef = strings.Repeat("x", privateGroupChallengeMaxRefBytes+1)
	raw, _ := json.Marshal(w)
	if _, err := RetainPrivateGroupChallengeSelector(context.Background(), store, selector, raw); err == nil {
		t.Fatal("ref plus one accepted")
	}
}

type privateGroupChallengeFaultStore struct {
	Store
	mu                                          sync.Mutex
	puts                                        int
	gets                                        map[string]int
	last                                        string
	wrongPut, mutateReadback, changePredecessor bool
	cancel                                      context.CancelFunc
}

func (s *privateGroupChallengeFaultStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gets == nil {
		s.gets = map[string]int{}
	}
	s.gets[key]++
	raw, err := s.Store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if s.changePredecessor && s.gets[key] == 2 {
		return append(raw, ' '), nil
	}
	if s.mutateReadback && key == s.last && s.puts > 0 {
		return append(raw, ' '), nil
	}
	return raw, nil
}
func (s *privateGroupChallengeFaultStore) Put(ctx context.Context, raw []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts++
	key, err := s.Store.Put(ctx, raw)
	s.last = key
	if s.cancel != nil {
		s.cancel()
	}
	if s.wrongPut {
		return digestOf([]byte("wrong")), err
	}
	return key, err
}

func TestPrivateGroupChallengeCancellationMutationReadbackAndConcurrency(t *testing.T) {
	store, selector, p := privateGroupChallengeFixture(t)
	submission := privateGroupChallengeSubmission(t, p, map[string]string{privateGroupingSemantic: privateChallengeRetain, privateGroupingSingleton: privateChallengeRetain}, privateChallengeRetain)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fault := &privateGroupChallengeFaultStore{Store: store}
	if got, err := RetainPrivateGroupChallengeSelector(ctx, fault, selector, submission); !errors.Is(err, context.Canceled) || got != "" || fault.puts != 0 {
		t.Fatalf("pre cancel %q %v", got, err)
	}
	for name, configure := range map[string]func(*privateGroupChallengeFaultStore){"wrong-selector": func(s *privateGroupChallengeFaultStore) { s.wrongPut = true }, "readback": func(s *privateGroupChallengeFaultStore) { s.mutateReadback = true }, "predecessor-change": func(s *privateGroupChallengeFaultStore) { s.changePredecessor = true }} {
		t.Run(name, func(t *testing.T) {
			f := &privateGroupChallengeFaultStore{Store: store}
			configure(f)
			if got, err := RetainPrivateGroupChallengeSelector(context.Background(), f, selector, submission); err == nil || got != "" {
				t.Fatalf("selector=%q err=%v", got, err)
			}
		})
	}
	fresh := NewMemoryStore()
	groupingRaw, _ := store.Get(context.Background(), selector)
	_, _ = fresh.Put(context.Background(), groupingRaw)
	postCtx, postCancel := context.WithCancel(context.Background())
	post := &privateGroupChallengeFaultStore{Store: fresh, cancel: postCancel}
	if got, err := RetainPrivateGroupChallengeSelector(postCtx, post, selector, submission); !errors.Is(err, context.Canceled) || got != "" || post.puts != 1 {
		t.Fatalf("post cancel %q %v puts=%d", got, err, post.puts)
	}
	const n = 8
	results := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, e := RetainPrivateGroupChallengeSelector(context.Background(), store, selector, submission)
			results <- s
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	want := ""
	for got := range results {
		if want == "" {
			want = got
		} else if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}
