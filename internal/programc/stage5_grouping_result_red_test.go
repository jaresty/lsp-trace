package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// These types are deliberately local test mechanics. They neither construct nor
// impersonate a production referencegrouping qualification or composition.
type stage5SyntheticToken struct {
	family, evidenceClass, trustClass string
	compositionDigest                 string
	validFrom, validThrough           uint64
}

type stage5LocalOccurrence struct {
	Identity, Kind, From, To, Disposition string
	Numerator, Denominator                int
}

type stage5LocalPair struct {
	OccurrenceIdentity, Kind, From, To string
	Numerator, Denominator             int
}

type stage5LocalAccounting struct {
	Input, Included, Excluded, Capped, Normalized, Downweighted, SeparateLayer, Duplicate, Terminal int
	Truncated                                                                                       bool
}

type stage5LocalCompositionSnapshot struct {
	Family, Version, InputID, InputDigest, PolicyID, PolicyDigest string
	QualificationID, QualificationDigest                          string
	Occurrences                                                   []stage5LocalOccurrence
	Pairs                                                         []stage5LocalPair
	Accounting                                                    stage5LocalAccounting
	Digest                                                        string
}

type stage5LocalResult struct {
	Family, Version, CompositionDigest, Algorithm, Profile, NumericContract string
	Seed                                                                    uint64
	Partition                                                               [][]string
	Accounting                                                              stage5LocalAccounting
	Authority                                                               int
	Accepted                                                                bool
	Completeness                                                            string
	Digest                                                                  string
}

type stage5LocalRun struct {
	Seed                    uint64 `json:"seed"`
	RunID                   string `json:"run_id"`
	Status                  string `json:"status"`
	ResultID                string `json:"result_id"`
	ResultDigest            string `json:"result_digest"`
	ResultCanonicalByteSize int    `json:"result_canonical_byte_size"`
}

type stage5LocalReadiness struct {
	Family, Version, CompositionDigest, Outcome       string
	Algorithm, Profile, NumericContract, LimitsDigest string
	SeedInventory, ResultInventory                    []string
	Runs                                              []stage5LocalRun
	Succeeded, Failed, Incomplete, Terminal           int
	PublicationAllowed, PublicationBlocked            bool
	Digest                                            string
}

type stage5LocalFixture struct {
	name             string
	occurrences      []stage5LocalOccurrence
	wantPartition    [][]string
	wantPairs        []stage5LocalPair
	wantAccounting   stage5LocalAccounting
	wantReadinessRun []stage5LocalRun
}

func stage5LocalFixtureForMode(name string) stage5LocalFixture {
	occ := map[string][]stage5LocalOccurrence{
		"calls-only": {{"call-1", "CALLS", "a", "b", "INCLUDED", 1, 1}, {"call-2", "CALLS", "b", "c", "INCLUDED", 1, 1}},
		"refs-only":  {{"ref-1", "REFERENCES_SYMBOL", "a", "b", "DOWNWEIGHTED", 1, 2}, {"ref-2", "REFERENCES_SYMBOL", "b", "b", "DOWNWEIGHTED", 1, 2}},
		"defs-only":  {{"def-1", "RESOLVES_TO_DEFINITION", "a", "b", "CAPPED", 1, 1}},
		"refs-defs":  {{"ref-1", "REFERENCES_SYMBOL", "a", "b", "DOWNWEIGHTED", 1, 2}, {"ref-2", "REFERENCES_SYMBOL", "b", "b", "DOWNWEIGHTED", 1, 2}, {"def-1", "RESOLVES_TO_DEFINITION", "a", "b", "CAPPED", 1, 1}},
		"calls-refs": {{"call-1", "CALLS", "a", "b", "INCLUDED", 1, 1}, {"call-2", "CALLS", "b", "c", "INCLUDED", 1, 1}, {"ref-1", "REFERENCES_SYMBOL", "a", "b", "DOWNWEIGHTED", 1, 2}, {"ref-2", "REFERENCES_SYMBOL", "b", "b", "DOWNWEIGHTED", 1, 2}},
		"calls-defs": {{"call-1", "CALLS", "a", "b", "INCLUDED", 1, 1}, {"call-2", "CALLS", "b", "c", "INCLUDED", 1, 1}, {"def-1", "RESOLVES_TO_DEFINITION", "a", "b", "CAPPED", 1, 1}},
		"all":        {{"call-1", "CALLS", "a", "b", "INCLUDED", 1, 1}, {"call-2", "CALLS", "b", "c", "INCLUDED", 1, 1}, {"ref-1", "REFERENCES_SYMBOL", "a", "b", "DOWNWEIGHTED", 1, 2}, {"ref-2", "REFERENCES_SYMBOL", "b", "b", "DOWNWEIGHTED", 1, 2}, {"def-1", "RESOLVES_TO_DEFINITION", "a", "b", "CAPPED", 1, 1}},
		"empty-zero": []stage5LocalOccurrence{},
	}[name]
	pairs := make([]stage5LocalPair, len(occ))
	nodes := map[string]bool{}
	a := stage5LocalAccounting{Input: len(occ), Terminal: len(occ)}
	for i, o := range occ {
		pairs[i] = stage5LocalPair{o.Identity, o.Kind, o.From, o.To, o.Numerator, o.Denominator}
		nodes[o.From], nodes[o.To] = true, true
		switch o.Disposition {
		case "INCLUDED":
			a.Included++
		case "DOWNWEIGHTED":
			a.Downweighted++
		case "CAPPED":
			a.Capped++
		}
	}
	partition := make([][]string, 0, len(nodes))
	for node := range nodes {
		partition = append(partition, []string{node})
	}
	sort.Slice(partition, func(i, j int) bool { return partition[i][0] < partition[j][0] })
	return stage5LocalFixture{name: name, occurrences: occ, wantPartition: partition, wantPairs: pairs, wantAccounting: a}
}

func stage5LocalSnapshot(f stage5LocalFixture) stage5LocalCompositionSnapshot {
	s := stage5LocalCompositionSnapshot{
		Family: "TEST_ONLY_STAGE5_COMPOSITION", Version: "v1",
		InputID: "test-input-" + f.name, InputDigest: stage5Digest([]byte("input:" + f.name)),
		PolicyID: "test-policy-" + f.name, PolicyDigest: stage5Digest([]byte("policy:" + f.name)),
		QualificationID: "test-qualification-" + f.name, QualificationDigest: stage5Digest([]byte("qualification:" + f.name)),
		Occurrences: make([]stage5LocalOccurrence, len(f.occurrences)), Pairs: make([]stage5LocalPair, len(f.wantPairs)), Accounting: f.wantAccounting,
	}
	copy(s.Occurrences, f.occurrences)
	copy(s.Pairs, f.wantPairs)
	s.Digest = stage5Digest(stage5Canonical(s.withoutDigest()))
	return s
}

func (s stage5LocalCompositionSnapshot) withoutDigest() stage5LocalCompositionSnapshot {
	s.Digest = ""
	return s
}
func (r stage5LocalResult) withoutDigest() stage5LocalResult       { r.Digest = ""; return r }
func (r stage5LocalReadiness) withoutDigest() stage5LocalReadiness { r.Digest = ""; return r }

func stage5TokenFor(s stage5LocalCompositionSnapshot, epoch uint64) stage5SyntheticToken {
	return stage5SyntheticToken{"LSP_TRACE_TEST_STAGE5_MECHANICS_TOKEN", "TEST_ONLY_STRUCTURAL_EVIDENCE", "TEST_ONLY_TRUST_ANCHOR", s.Digest, epoch, epoch + 2}
}

func stage5VerifyToken(want stage5SyntheticToken, epoch uint64) func(stage5SyntheticToken) bool {
	return func(got stage5SyntheticToken) bool {
		return reflect.DeepEqual(got, want) && got.family == "LSP_TRACE_TEST_STAGE5_MECHANICS_TOKEN" && got.evidenceClass == "TEST_ONLY_STRUCTURAL_EVIDENCE" && got.trustClass == "TEST_ONLY_TRUST_ANCHOR" && epoch >= got.validFrom && epoch <= got.validThrough
	}
}

func stage5Canonical(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return append(raw, '\n')
}
func stage5Digest(raw []byte) string { sum := sha256.Sum256(raw); return fmt.Sprintf("sha256:%x", sum) }

func TestStage5AllEightModesReachOnlyLocalSuccessorMechanicsGate(t *testing.T) {
	for _, name := range []string{"calls-only", "refs-only", "defs-only", "refs-defs", "calls-refs", "calls-defs", "all", "empty-zero"} {
		t.Run(name, func(t *testing.T) {
			fixture := stage5LocalFixtureForMode(name)
			snapshot := stage5LocalSnapshot(fixture)
			token := stage5TokenFor(snapshot, 8)
			result, err := computeQualifiedGroupingResult(snapshot, token, stage5VerifyToken(token, 8), 71)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Partition, fixture.wantPartition) || !reflect.DeepEqual(snapshot.Pairs, fixture.wantPairs) || snapshot.Accounting != fixture.wantAccounting || result.Accounting != fixture.wantAccounting {
				t.Fatalf("ASSERT_STAGE5_EXACT_MODE_TABLE mode=%s partition=%+v pairs=%+v accounting=%+v", name, result.Partition, snapshot.Pairs, result.Accounting)
			}
			run := stage5LocalRun{Seed: 71, RunID: "run-71", Status: "SUCCEEDED", ResultID: "result-71", ResultDigest: result.Digest, ResultCanonicalByteSize: len(stage5Canonical(result))}
			readiness, err := computeGroupingReadiness(snapshot, token, stage5VerifyToken(token, 8), []stage5LocalRun{run})
			if err != nil || !readiness.PublicationAllowed || readiness.Outcome != "READY" || readiness.Succeeded != 1 || readiness.Terminal != 1 || !reflect.DeepEqual(readiness.Runs, []stage5LocalRun{run}) {
				t.Fatalf("ASSERT_STAGE5_EXACT_READINESS_TABLE mode=%s readiness=%+v err=%v", name, readiness, err)
			}
			replayed, err := replayQualifiedGroupingResult(snapshot, token, stage5VerifyToken(token, 8), result)
			if err != nil || !reflect.DeepEqual(replayed, result) || !bytes.Equal(stage5Canonical(replayed), stage5Canonical(result)) {
				t.Fatalf("ASSERT_STAGE5_EXACT_REPLAY mode=%s err=%v", name, err)
			}
		})
	}
}

func TestStage5LocalTokenAndCompositionMutationInventoryAtomicRefusal(t *testing.T) {
	fixture := stage5LocalFixtureForMode("all")
	base := stage5LocalSnapshot(fixture)
	token := stage5TokenFor(base, 8)
	digest := func(n byte) string { return "sha256:" + strings.Repeat(string(n), 64) }
	compositionMutations := map[string]func(*stage5LocalCompositionSnapshot){
		"family": func(x *stage5LocalCompositionSnapshot) { x.Family += "-mutated" }, "version": func(x *stage5LocalCompositionSnapshot) { x.Version += "-mutated" },
		"input-id": func(x *stage5LocalCompositionSnapshot) { x.InputID += "-mutated" }, "input-digest": func(x *stage5LocalCompositionSnapshot) { x.InputDigest = digest('1') },
		"policy-id": func(x *stage5LocalCompositionSnapshot) { x.PolicyID += "-mutated" }, "policy-digest": func(x *stage5LocalCompositionSnapshot) { x.PolicyDigest = digest('2') },
		"qualification-id": func(x *stage5LocalCompositionSnapshot) { x.QualificationID += "-mutated" }, "qualification-digest": func(x *stage5LocalCompositionSnapshot) { x.QualificationDigest = digest('3') },
		"occurrences": func(x *stage5LocalCompositionSnapshot) { x.Occurrences[0].From += "-mutated" }, "pairs": func(x *stage5LocalCompositionSnapshot) { x.Pairs[0].To += "-mutated" },
		"accounting": func(x *stage5LocalCompositionSnapshot) { x.Accounting.Terminal++ }, "digest": func(x *stage5LocalCompositionSnapshot) { x.Digest = digest('4') },
	}
	for name, mutate := range compositionMutations {
		t.Run("composition-"+name, func(t *testing.T) {
			got := base
			got.Occurrences = append([]stage5LocalOccurrence(nil), base.Occurrences...)
			got.Pairs = append([]stage5LocalPair(nil), base.Pairs...)
			mutate(&got)
			if result, err := computeQualifiedGroupingResult(got, token, stage5VerifyToken(token, 8), 71); err == nil || !reflect.DeepEqual(result, stage5LocalResult{}) {
				t.Fatalf("ASSERT_STAGE5_COMPOSITION_MUTATION_REFUSED result=%+v err=%v", result, err)
			}
		})
	}
	tokenMutations := map[string]func(*stage5SyntheticToken){"family": func(x *stage5SyntheticToken) { x.family += "-mutated" }, "evidence": func(x *stage5SyntheticToken) { x.evidenceClass += "-mutated" }, "trust": func(x *stage5SyntheticToken) { x.trustClass += "-mutated" }, "composition": func(x *stage5SyntheticToken) { x.compositionDigest = digest('5') }, "valid-from": func(x *stage5SyntheticToken) { x.validFrom++ }, "valid-through": func(x *stage5SyntheticToken) { x.validThrough-- }}
	for name, mutate := range tokenMutations {
		t.Run("token-"+name, func(t *testing.T) {
			got := token
			mutate(&got)
			if result, err := computeQualifiedGroupingResult(base, got, stage5VerifyToken(token, 8), 71); err == nil || !reflect.DeepEqual(result, stage5LocalResult{}) {
				t.Fatalf("ASSERT_STAGE5_TOKEN_MUTATION_REFUSED result=%+v err=%v", result, err)
			}
		})
	}
	if result, err := computeQualifiedGroupingResult(base, token, stage5VerifyToken(token, 11), 71); err == nil || !reflect.DeepEqual(result, stage5LocalResult{}) {
		t.Fatalf("ASSERT_STAGE5_STALE_TOKEN_REFUSED result=%+v err=%v", result, err)
	}
}

func TestStage5ReadinessMalformedLimitsAndMutationCorpus(t *testing.T) {
	snapshot := stage5LocalSnapshot(stage5LocalFixtureForMode("refs-only"))
	token := stage5TokenFor(snapshot, 8)
	goodResult, _ := computeQualifiedGroupingResult(snapshot, token, stage5VerifyToken(token, 8), 3)
	good := []stage5LocalRun{
		{Seed: 3, RunID: "run-3", Status: "SUCCEEDED", ResultID: "result-3", ResultDigest: goodResult.Digest, ResultCanonicalByteSize: len(stage5Canonical(goodResult))},
		{Seed: 5, RunID: "run-5", Status: "FAILED"},
		{Seed: 7, RunID: "run-7", Status: "INCOMPLETE"},
	}
	readiness, err := computeGroupingReadiness(snapshot, token, stage5VerifyToken(token, 8), good)
	if err != nil || readiness.PublicationAllowed || readiness.Succeeded != 1 || readiness.Failed != 1 || readiness.Incomplete != 1 || readiness.Terminal != 3 {
		t.Fatalf("ASSERT_STAGE5_READINESS_ACCOUNTING %+v err=%v", readiness, err)
	}
	badRuns := map[string][]stage5LocalRun{
		"empty":                  nil,
		"duplicate-seed":         {{Seed: 3, RunID: "run-a", Status: "FAILED"}, {Seed: 3, RunID: "run-b", Status: "INCOMPLETE"}},
		"descending":             {{Seed: 5, RunID: "run-5", Status: "FAILED"}, {Seed: 3, RunID: "run-3", Status: "INCOMPLETE"}},
		"unknown-status":         {{Seed: 3, RunID: "run-3", Status: "UNKNOWN"}},
		"success-missing-digest": {{Seed: 3, RunID: "run-3", Status: "SUCCEEDED", ResultID: "result-3", ResultCanonicalByteSize: 1}},
		"success-zero-bytes":     {{Seed: 3, RunID: "run-3", Status: "SUCCEEDED", ResultID: "result-3", ResultDigest: goodResult.Digest}},
		"failed-result":          {{Seed: 3, RunID: "run-3", Status: "FAILED", ResultID: "result-3", ResultDigest: goodResult.Digest, ResultCanonicalByteSize: 1}},
		"incomplete-result":      {{Seed: 3, RunID: "run-3", Status: "INCOMPLETE", ResultID: "result-3", ResultDigest: goodResult.Digest, ResultCanonicalByteSize: 1}},
	}
	for name, runs := range badRuns {
		t.Run(name, func(t *testing.T) {
			got, err := computeGroupingReadiness(snapshot, token, stage5VerifyToken(token, 8), runs)
			if err == nil || !reflect.DeepEqual(got, stage5LocalReadiness{}) {
				t.Fatalf("ASSERT_STAGE5_READINESS_MALFORMED_ATOMIC_ZERO got=%+v err=%v", got, err)
			}
		})
	}
	raw := stage5Canonical(readiness)
	for name, bad := range map[string][]byte{"unknown": bytes.Replace(raw, []byte(`{"Family":`), []byte(`{"Unknown":true,"Family":`), 1), "duplicate": bytes.Replace(raw, []byte(`{"Family":`), []byte(`{"Family":"x","Family":`), 1), "no-lf": raw[:len(raw)-1], "extra-lf": append(append([]byte(nil), raw...), '\n'), "crlf": append(append([]byte(nil), raw[:len(raw)-1]...), '\r', '\n')} {
		t.Run("json-"+name, func(t *testing.T) {
			if got, err := parseAndAdmitGroupingReadiness(bad, len(raw), 16); err == nil || !reflect.DeepEqual(got, stage5LocalReadiness{}) {
				t.Fatal("ASSERT_STAGE5_READINESS_STRICT_MALFORMED_REFUSED")
			}
		})
	}
	if got, err := parseAndAdmitGroupingReadiness(raw, len(raw)-1, 16); err == nil || !reflect.DeepEqual(got, stage5LocalReadiness{}) {
		t.Fatal("ASSERT_STAGE5_READINESS_RAW_LIMIT")
	}
	if got, err := parseAndAdmitGroupingReadiness(raw, len(raw), 2); err == nil || !reflect.DeepEqual(got, stage5LocalReadiness{}) {
		t.Fatal("ASSERT_STAGE5_READINESS_RUN_LIMIT")
	}
	mutations := map[string]func(*stage5LocalReadiness){
		"family":              func(x *stage5LocalReadiness) { x.Family += "-mutated" },
		"version":             func(x *stage5LocalReadiness) { x.Version += "-mutated" },
		"composition-digest":  func(x *stage5LocalReadiness) { x.CompositionDigest = stage5Digest([]byte("mutated-composition")) },
		"seed":                func(x *stage5LocalReadiness) { x.Runs[0].Seed++ },
		"run-id":              func(x *stage5LocalReadiness) { x.Runs[0].RunID += "-mutated" },
		"result-id":           func(x *stage5LocalReadiness) { x.Runs[0].ResultID += "-mutated" },
		"result-digest":       func(x *stage5LocalReadiness) { x.Runs[0].ResultDigest = stage5Digest([]byte("mutated")) },
		"result-length":       func(x *stage5LocalReadiness) { x.Runs[0].ResultCanonicalByteSize++ },
		"status":              func(x *stage5LocalReadiness) { x.Runs[0].Status = "INCOMPLETE" },
		"succeeded":           func(x *stage5LocalReadiness) { x.Succeeded++ },
		"failed":              func(x *stage5LocalReadiness) { x.Failed++ },
		"incomplete":          func(x *stage5LocalReadiness) { x.Incomplete++ },
		"terminal":            func(x *stage5LocalReadiness) { x.Terminal++ },
		"outcome":             func(x *stage5LocalReadiness) { x.Outcome += "-mutated" },
		"publication-allowed": func(x *stage5LocalReadiness) { x.PublicationAllowed = !x.PublicationAllowed },
		"publication-blocked": func(x *stage5LocalReadiness) { x.PublicationBlocked = !x.PublicationBlocked },
		"algorithm":           func(x *stage5LocalReadiness) { x.Algorithm += "-mutated" },
		"profile":             func(x *stage5LocalReadiness) { x.Profile += "-mutated" },
		"numeric":             func(x *stage5LocalReadiness) { x.NumericContract += "-mutated" },
		"limits":              func(x *stage5LocalReadiness) { x.LimitsDigest += "-mutated" },
		"seed-inventory":      func(x *stage5LocalReadiness) { x.SeedInventory = append(x.SeedInventory, "seed-mutated") },
		"result-inventory":    func(x *stage5LocalReadiness) { x.ResultInventory = append(x.ResultInventory, "result-mutated") },
		"inventory-order":     func(x *stage5LocalReadiness) { x.Runs[0], x.Runs[1] = x.Runs[1], x.Runs[0] },
		"inventory-duplicate": func(x *stage5LocalReadiness) { x.Runs = append(x.Runs, x.Runs[0]) },
		"inventory-add": func(x *stage5LocalReadiness) {
			x.Runs = append(x.Runs, stage5LocalRun{Seed: 11, RunID: "run-11", Status: "FAILED"})
		},
		"inventory-remove": func(x *stage5LocalReadiness) { x.Runs = x.Runs[:len(x.Runs)-1] },
	}
	corruptDigest := readiness
	corruptDigest.Digest = stage5Digest([]byte("corrupted-without-recomputation"))
	if got, err := parseAndAdmitGroupingReadiness(stage5Canonical(corruptDigest), 1<<20, 16); err == nil || !reflect.DeepEqual(got, stage5LocalReadiness{}) {
		t.Fatalf("ASSERT_STAGE5_READINESS_CORRUPTED_DIGEST_REFUSED got=%+v err=%v", got, err)
	}
	for name, mutate := range mutations {
		t.Run("mutation-"+name, func(t *testing.T) {
			mutated := readiness
			mutated.Runs = append([]stage5LocalRun(nil), readiness.Runs...)
			mutated.SeedInventory = append([]string(nil), readiness.SeedInventory...)
			mutated.ResultInventory = append([]string(nil), readiness.ResultInventory...)
			mutate(&mutated)
			mutated.Digest = stage5Digest(stage5Canonical(mutated.withoutDigest()))
			got, err := parseAndAdmitGroupingReadiness(stage5Canonical(mutated), 1<<20, 16)
			if err == nil || !reflect.DeepEqual(got, stage5LocalReadiness{}) {
				t.Fatalf("ASSERT_STAGE5_READINESS_MUTATION_PARSE_ADMIT_ATOMIC_ZERO mutation=%s got=%+v err=%v", name, got, err)
			}
			if replayed, replayErr := replayGroupingReadiness(snapshot, token, stage5VerifyToken(token, 8), mutated); replayErr == nil || !reflect.DeepEqual(replayed, stage5LocalReadiness{}) {
				t.Fatalf("ASSERT_STAGE5_READINESS_MUTATION_REPLAY_ATOMIC_ZERO mutation=%s got=%+v err=%v", name, replayed, replayErr)
			}
		})
	}
}

const stage5PlatformNeutralGolden = "{\"Family\":\"REFERENCE_GROUPING_RESULT\",\"Version\":\"v1\",\"CompositionDigest\":\"sha256:28bf3d91d35507d459948ec363e18f3931f6bb8e54f3c1815feee15994319ad4\",\"Algorithm\":\"TEST_ONLY_SUCCESSOR_MECHANICS\",\"Profile\":\"REFERENCE_GROUPING_V1\",\"NumericContract\":\"EXACT_REDUCED_INTEGER_RATIONAL_V1\",\"Seed\":83,\"Partition\":[[\"a\"],[\"b\"],[\"c\"]],\"Accounting\":{\"Input\":2,\"Included\":2,\"Excluded\":0,\"Capped\":0,\"Normalized\":0,\"Downweighted\":0,\"SeparateLayer\":0,\"Duplicate\":0,\"Terminal\":2,\"Truncated\":false},\"Authority\":0,\"Accepted\":false,\"Completeness\":\"UNKNOWN\",\"Digest\":\"sha256:b7edecb9e32736747714db6d6058473454de810b34041b6402cee2b7342550ee\"}\n"

func TestStage5PlatformNeutralPinnedGoldenAndConcurrencyOracle(t *testing.T) {
	snapshot := stage5LocalSnapshot(stage5LocalFixtureForMode("calls-only"))
	token := stage5TokenFor(snapshot, 8)
	want := []byte(stage5PlatformNeutralGolden)
	const workers = 32
	got := make([]stage5LocalResult, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range got {
		i := i
		go func() {
			defer wg.Done()
			<-start
			got[i], errs[i] = computeQualifiedGroupingResult(snapshot, token, stage5VerifyToken(token, 8), 83)
		}()
	}
	close(start)
	wg.Wait()
	for i := range got {
		raw := stage5Canonical(got[i])
		if errs[i] != nil || !bytes.Equal(raw, want) || stage5Digest(raw) != "sha256:87eea9e54879cf8160879b799aed2b89bfecb74c0af7240ad1789fb880abfb36" {
			t.Fatalf("ASSERT_STAGE5_PLATFORM_NEUTRAL_CONCURRENT_GOLDEN i=%d digest=%s err=%v\n got=%s\nwant=%s", i, stage5Digest(raw), errs[i], raw, want)
		}
	}
}
