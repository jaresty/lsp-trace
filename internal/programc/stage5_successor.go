package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
)

type SuccessorAccounting struct {
	Input, Included, Excluded, Capped, Normalized, Downweighted, SeparateLayer, Duplicate, Terminal int
	Truncated                                                                                       bool
}

type SuccessorGroupingResult struct {
	Family, Version, CompositionDigest, Algorithm, Profile, NumericContract string
	Seed                                                                    uint64
	Partition                                                               [][]string
	Accounting                                                              SuccessorAccounting
	Authority                                                               int
	Accepted                                                                bool
	Completeness                                                            string
	Digest                                                                  string
}

type GroupingReadinessRun struct {
	Seed                    uint64 `json:"seed"`
	RunID                   string `json:"run_id"`
	Status                  string `json:"status"`
	ResultID                string `json:"result_id"`
	ResultDigest            string `json:"result_digest"`
	ResultCanonicalByteSize int    `json:"result_canonical_byte_size"`
}

type GroupingReadiness struct {
	Family, Version, CompositionDigest, Outcome       string
	Algorithm, Profile, NumericContract, LimitsDigest string
	SeedInventory, ResultInventory                    []string
	Runs                                              []GroupingReadinessRun
	Succeeded, Failed, Incomplete, Terminal           int
	PublicationAllowed, PublicationBlocked            bool
	Digest                                            string
}

type successorSnapshotWire struct {
	Family, Version, InputID, InputDigest, PolicyID, PolicyDigest string
	QualificationID, QualificationDigest                          string
	Occurrences                                                   []struct {
		Identity, Kind, From, To, Disposition string
		Numerator, Denominator                int
	}
	Pairs []struct {
		OccurrenceIdentity, Kind, From, To string
		Numerator, Denominator             int
	}
	Accounting SuccessorAccounting
	Digest     string
}

func successorCanonical(v any) []byte { b, _ := json.Marshal(v); return append(b, '\n') }
func successorDigest(b []byte) string { h := sha256.Sum256(b); return fmt.Sprintf("sha256:%x", h) }
func successorGCD(a, b int) int {
	if a < 0 {
		a = -a
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}
func validateSuccessorResultBytes(raw []byte, snapshot successorSnapshotWire, seed uint64) error {
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || bytes.Contains(raw, []byte{'\r'}) {
		return errors.New("result framing")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r SuccessorGroupingResult
	if err := dec.Decode(&r); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("result trailing")
	}
	if !bytes.Equal(raw, successorCanonical(r)) || r.Family != "REFERENCE_GROUPING_RESULT" || r.Version != "v1" || r.CompositionDigest != snapshot.Digest || r.Seed != seed || r.Algorithm != "TEST_ONLY_SUCCESSOR_MECHANICS" || r.Profile != "REFERENCE_GROUPING_V1" || r.NumericContract != "EXACT_REDUCED_INTEGER_RATIONAL_V1" || r.Authority != 0 || r.Accepted || r.Completeness != "UNKNOWN" {
		return errors.New("result binding")
	}
	want := r
	want.Digest = ""
	if r.Digest != successorDigest(successorCanonical(want)) {
		return errors.New("result digest")
	}
	if !reflect.DeepEqual(r.Partition, successorPartition(snapshot)) || r.Accounting != snapshot.Accounting {
		return errors.New("result semantics")
	}
	return nil
}

func decodeSuccessorSnapshot[A any](in A) (successorSnapshotWire, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return successorSnapshotWire{}, err
	}
	var s successorSnapshotWire
	if err := json.Unmarshal(raw, &s); err != nil {
		return successorSnapshotWire{}, err
	}
	want := s
	want.Digest = ""
	if s.Family != "TEST_ONLY_STAGE5_COMPOSITION" || s.Version != "v1" || s.Digest != successorDigest(successorCanonical(want)) {
		return successorSnapshotWire{}, errors.New("invalid composition snapshot")
	}
	if s.Accounting.Input != len(s.Occurrences) || s.Accounting.Terminal != len(s.Occurrences) || len(s.Pairs) != len(s.Occurrences) || s.Accounting.Excluded != 0 || s.Accounting.SeparateLayer != 0 || s.Accounting.Duplicate != 0 || s.Accounting.Truncated {
		return successorSnapshotWire{}, errors.New("invalid composition accounting")
	}
	var derived SuccessorAccounting
	derived.Input, derived.Terminal = len(s.Occurrences), len(s.Occurrences)
	for i := range s.Occurrences {
		o, p := s.Occurrences[i], s.Pairs[i]
		if o.Identity == "" || o.Identity != p.OccurrenceIdentity || o.Kind != p.Kind || o.From != p.From || o.To != p.To || o.Numerator != p.Numerator || o.Denominator != p.Denominator || p.Numerator < 0 || p.Denominator <= 0 || successorGCD(p.Numerator, p.Denominator) != 1 {
			return successorSnapshotWire{}, errors.New("invalid composition occurrence")
		}
		switch o.Kind {
		case "CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION":
		default:
			return successorSnapshotWire{}, errors.New("invalid relation kind")
		}
		switch o.Disposition {
		case "INCLUDED":
			derived.Included++
		case "DOWNWEIGHTED":
			derived.Downweighted++
		case "CAPPED":
			derived.Capped++
		case "NORMALIZED":
			derived.Normalized++
		default:
			return successorSnapshotWire{}, errors.New("invalid disposition")
		}
	}
	if derived != s.Accounting {
		return successorSnapshotWire{}, errors.New("invalid derived accounting")
	}
	return s, nil
}

func successorPartition(s successorSnapshotWire) [][]string {
	nodes := map[string]bool{}
	for _, p := range s.Pairs {
		nodes[p.From], nodes[p.To] = true, true
	}
	partition := make([][]string, 0, len(nodes))
	for n := range nodes {
		partition = append(partition, []string{n})
	}
	sort.Slice(partition, func(i, j int) bool { return partition[i][0] < partition[j][0] })
	return partition
}

func computeQualifiedGroupingResult[A, T any](snapshot A, token T, verify func(T) bool, seed uint64) (SuccessorGroupingResult, error) {
	if verify == nil || !verify(token) {
		return SuccessorGroupingResult{}, errors.New("qualification unavailable")
	}
	s, err := decodeSuccessorSnapshot(snapshot)
	if err != nil {
		return SuccessorGroupingResult{}, err
	}
	partition := successorPartition(s)
	r := SuccessorGroupingResult{
		Family: "REFERENCE_GROUPING_RESULT", Version: "v1", CompositionDigest: s.Digest,
		Algorithm: "TEST_ONLY_SUCCESSOR_MECHANICS", Profile: "REFERENCE_GROUPING_V1", NumericContract: "EXACT_REDUCED_INTEGER_RATIONAL_V1",
		Seed: seed, Partition: partition, Accounting: s.Accounting, Authority: 0, Accepted: false, Completeness: "UNKNOWN",
	}
	r.Digest = successorDigest(successorCanonical(successorResultWithoutDigest(r)))
	return r, nil
}
func successorResultWithoutDigest(r SuccessorGroupingResult) SuccessorGroupingResult {
	r.Digest = ""
	return r
}

func replayQualifiedGroupingResult[A, T any](snapshot A, token T, verify func(T) bool, prior SuccessorGroupingResult) (SuccessorGroupingResult, error) {
	r, err := computeQualifiedGroupingResult(snapshot, token, verify, prior.Seed)
	if err != nil || !reflect.DeepEqual(r, prior) {
		return SuccessorGroupingResult{}, errors.New("successor replay mismatch")
	}
	return r, nil
}

func computeGroupingReadiness[A, T any](snapshot A, token T, verify func(T) bool, runs []GroupingReadinessRun, results map[string][]byte) (GroupingReadiness, error) {
	if verify == nil || !verify(token) {
		return GroupingReadiness{}, errors.New("qualification unavailable")
	}
	s, err := decodeSuccessorSnapshot(snapshot)
	if err != nil {
		return GroupingReadiness{}, err
	}
	r := GroupingReadiness{Family: "REFERENCE_GROUPING_READINESS", Version: "v1", CompositionDigest: s.Digest, Algorithm: "TEST_ONLY_SUCCESSOR_MECHANICS", Profile: "REFERENCE_GROUPING_V1", NumericContract: "EXACT_REDUCED_INTEGER_RATIONAL_V1", LimitsDigest: successorDigest([]byte("STAGE5_READINESS_LIMITS_V1")), Runs: append([]GroupingReadinessRun(nil), runs...)}
	if err := deriveReadiness(&r, s, results); err != nil {
		return GroupingReadiness{}, err
	}
	wantDigest := r
	wantDigest.Digest = ""
	r.Digest = successorDigest(successorCanonical(wantDigest))
	return r, nil
}

func deriveReadiness(r *GroupingReadiness, snapshot successorSnapshotWire, results map[string][]byte) error {
	if len(r.Runs) == 0 {
		return errors.New("empty run inventory")
	}
	r.SeedInventory = nil
	r.ResultInventory = nil
	r.Succeeded = 0
	r.Failed = 0
	r.Incomplete = 0
	r.Terminal = 0
	last := uint64(0)
	for i, run := range r.Runs {
		if run.RunID != fmt.Sprintf("run-%d", run.Seed) || (i > 0 && run.Seed <= last) {
			return errors.New("noncanonical run inventory")
		}
		last = run.Seed
		r.SeedInventory = append(r.SeedInventory, fmt.Sprintf("seed-%d", run.Seed))
		r.Terminal++
		switch run.Status {
		case "SUCCEEDED":
			raw, ok := results[run.ResultID]
			var admitted SuccessorGroupingResult
			decodeErr := json.Unmarshal(bytes.TrimSuffix(raw, []byte{'\n'}), &admitted)
			if run.ResultID != fmt.Sprintf("result-%d", run.Seed) || !ok || decodeErr != nil || run.ResultDigest != admitted.Digest || run.ResultCanonicalByteSize != len(raw) || validateSuccessorResultBytes(raw, snapshot, run.Seed) != nil {
				return errors.New("invalid successful run")
			}
			r.Succeeded++
			r.ResultInventory = append(r.ResultInventory, fmt.Sprintf("%s\x00%s\x00%d", run.ResultID, run.ResultDigest, run.ResultCanonicalByteSize))
		case "FAILED":
			if run.ResultID != "" || run.ResultDigest != "" || run.ResultCanonicalByteSize != 0 {
				return errors.New("failed run has result")
			}
			r.Failed++
		case "INCOMPLETE":
			if run.ResultID != "" || run.ResultDigest != "" || run.ResultCanonicalByteSize != 0 {
				return errors.New("incomplete run has result")
			}
			r.Incomplete++
		default:
			return errors.New("unknown run status")
		}
	}
	if r.Failed == 0 && r.Incomplete == 0 {
		r.Outcome = "READY"
		r.PublicationAllowed = true
		r.PublicationBlocked = false
	} else {
		r.Outcome = "BLOCKED"
		r.PublicationAllowed = false
		r.PublicationBlocked = true
	}
	return nil
}

func parseAndAdmitGroupingReadiness[A any](raw []byte, snapshot A, results map[string][]byte, maxRaw, maxRuns int) (GroupingReadiness, error) {
	if len(raw) == 0 || len(raw) > maxRaw || raw[len(raw)-1] != '\n' || (len(raw) > 1 && raw[len(raw)-2] == '\n') || bytes.Contains(raw, []byte{'\r'}) {
		return GroupingReadiness{}, errors.New("readiness framing")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r GroupingReadiness
	if err := dec.Decode(&r); err != nil {
		return GroupingReadiness{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return GroupingReadiness{}, errors.New("readiness trailing")
	}
	if len(r.Runs) > maxRuns {
		return GroupingReadiness{}, errors.New("run limit")
	}
	canon := successorCanonical(r)
	if !bytes.Equal(canon, raw) {
		return GroupingReadiness{}, errors.New("noncanonical readiness")
	}
	s, err := decodeSuccessorSnapshot(snapshot)
	if err != nil || r.Family != "REFERENCE_GROUPING_READINESS" || r.Version != "v1" || r.CompositionDigest != s.Digest || r.Algorithm != "TEST_ONLY_SUCCESSOR_MECHANICS" || r.Profile != "REFERENCE_GROUPING_V1" || r.NumericContract != "EXACT_REDUCED_INTEGER_RATIONAL_V1" || r.LimitsDigest != successorDigest([]byte("STAGE5_READINESS_LIMITS_V1")) {
		return GroupingReadiness{}, errors.New("readiness binding")
	}
	want := r
	want.Digest = ""
	if r.Digest != successorDigest(successorCanonical(want)) {
		return GroupingReadiness{}, errors.New("readiness digest")
	}
	derived := r
	digest := derived.Digest
	derived.Digest = ""
	if err := deriveReadiness(&derived, s, results); err != nil {
		return GroupingReadiness{}, err
	}
	derived.Digest = digest
	if !reflect.DeepEqual(derived, r) {
		return GroupingReadiness{}, errors.New("readiness accounting")
	}
	return r, nil
}

func replayGroupingReadiness[A, T any](snapshot A, token T, verify func(T) bool, prior GroupingReadiness, results map[string][]byte) (GroupingReadiness, error) {
	if verify == nil || !verify(token) {
		return GroupingReadiness{}, errors.New("qualification unavailable")
	}
	r, err := parseAndAdmitGroupingReadiness(successorCanonical(prior), snapshot, results, 1<<20, len(prior.Runs))
	if err != nil {
		return GroupingReadiness{}, err
	}
	return r, nil
}
