package qualificationpolicy

import (
	"bytes"
	"strings"
	"testing"
)

func validFixture() (Policy, Evidence, Trust) {
	tuple := Tuple{Language: "typescript", Provider: "typescript-language-server", Version: "4.3.3", Operation: "incoming"}
	p := Policy{SchemaVersion: SchemaID, PolicyID: "incoming-ts", PolicyVersion: "1", AuthorityID: "release-authority", Tuple: tuple,
		RequiredPositive: []ObservationRequirement{{ID: "direct", Expected: "edge"}}, RequiredNegative: []ObservationRequirement{{ID: "unsupported", Expected: "blocked"}},
		MinimumRealServerRuns: 1, MinimumRepetitions: 1, PassPredicates: []string{"direct"}, FailurePredicates: []string{"crash"}, CustodyReceipt: "sha256:policy"}
	e := Evidence{PolicyID: p.PolicyID, PolicyVersion: p.PolicyVersion, Tuple: tuple, Status: "PASS", IssuerID: "evidence-producer",
		Observations: []Observation{{ID: "direct", Actual: "edge", Passed: true, RealServer: true, RunID: "run-1"}, {ID: "unsupported", Actual: "blocked", Passed: true, RealServer: true, RunID: "run-1"}}}
	trust := Trust{AuthorityID: p.AuthorityID, CurrentPolicyVersion: p.PolicyVersion, TrustedCustodyReceipt: p.CustodyReceipt}
	return p, e, trust
}

func TestQualificationPolicyDecisionBoundary(t *testing.T) {
	p, e, trust := validFixture()
	if got := Validate(p, e, trust); got.Status != "PASS" || got.Blocked || len(got.Reasons) != 0 {
		t.Fatalf("ASSERT_VALID_EXACT_PASS: got %#v", got)
	}
	tests := []struct {
		name, assertion string
		mutate          func(*Policy, *Evidence, *Trust)
		wantStatus      string
		wantBlocked     bool
	}{
		{"authority", "ASSERT_UNTRUSTED_AUTHORITY_REJECTED", func(p *Policy, _ *Evidence, _ *Trust) { p.AuthorityID = "producer" }, "NOT_QUALIFIED", false},
		{"custody", "ASSERT_UNTRUSTED_CUSTODY_REJECTED", func(p *Policy, _ *Evidence, _ *Trust) { p.CustodyReceipt = "sha256:self" }, "NOT_QUALIFIED", false},
		{"stale", "ASSERT_STALE_POLICY_REJECTED", func(_ *Policy, e *Evidence, _ *Trust) { e.PolicyVersion = "0" }, "NOT_QUALIFIED", false},
		{"tuple", "ASSERT_EXACT_TUPLE_REQUIRED", func(_ *Policy, e *Evidence, _ *Trust) { e.Tuple.Version = "other" }, "NOT_QUALIFIED", false},
		{"positive", "ASSERT_POSITIVE_OBSERVATION_REQUIRED", func(_ *Policy, e *Evidence, _ *Trust) { e.Observations[0].Passed = false }, "FAIL", false},
		{"negative", "ASSERT_NEGATIVE_OBSERVATION_REQUIRED", func(_ *Policy, e *Evidence, _ *Trust) { e.Observations = e.Observations[:1] }, "NOT_QUALIFIED", false},
		{"real-server", "ASSERT_REAL_SERVER_MINIMUM_REQUIRED", func(_ *Policy, e *Evidence, _ *Trust) {
			for i := range e.Observations {
				e.Observations[i].RealServer = false
			}
		}, "NOT_QUALIFIED", false},
		{"repetition", "ASSERT_REPETITION_MINIMUM_REQUIRED", func(p *Policy, _ *Evidence, _ *Trust) { p.MinimumRepetitions = 2 }, "NOT_QUALIFIED", false},
		{"failure", "ASSERT_FAILURE_PREDICATE_FORCES_FAIL", func(_ *Policy, e *Evidence, _ *Trust) {
			e.Observations = append(e.Observations, Observation{ID: "crash", Actual: "seen", Passed: true, RealServer: true, RunID: "run-1"})
		}, "FAIL", false},
		{"self-issued", "ASSERT_SELF_ISSUED_PASS_REJECTED", func(p *Policy, e *Evidence, _ *Trust) { e.IssuerID = p.AuthorityID }, "NOT_QUALIFIED", false},
		{"blocked", "ASSERT_BLOCKED_PRESERVED_NON_PASS", func(_ *Policy, e *Evidence, _ *Trust) { e.Status = "BLOCKED" }, "BLOCKED", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p2, e2, trust2 := validFixture()
			tc.mutate(&p2, &e2, &trust2)
			got := Validate(p2, e2, trust2)
			if got.Status != tc.wantStatus || got.Blocked != tc.wantBlocked || len(got.Reasons) == 0 {
				t.Fatalf("%s: got %#v, want status=%s blocked=%v with reason", tc.assertion, got, tc.wantStatus, tc.wantBlocked)
			}
		})
	}
}

func TestCanonicalBytesStableWithoutMutatingInput(t *testing.T) {
	p, _, _ := validFixture()
	p.RequiredPositive = append(p.RequiredPositive, ObservationRequirement{ID: "a", Expected: "first"})
	before := append([]ObservationRequirement(nil), p.RequiredPositive...)
	a, err := CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("ASSERT_POLICY_CANONICAL_BYTES_STABLE: %q != %q", a, b)
	}
	if strings.Index(string(a), `"id":"a"`) > strings.Index(string(a), `"id":"direct"`) {
		t.Fatalf("ASSERT_POLICY_COLLECTIONS_CANONICALLY_SORTED: %s", a)
	}
	for i := range before {
		if before[i] != p.RequiredPositive[i] {
			t.Fatal("ASSERT_CANONICALIZATION_DOES_NOT_MUTATE_INPUT")
		}
	}
}

func TestParseRejectsOpenOrMultipleDocuments(t *testing.T) {
	p, _, _ := validFixture()
	raw, err := CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(append(append([]byte(nil), raw...), []byte(` {}`)...)); err == nil {
		t.Fatal("ASSERT_TRAILING_JSON_REJECTED")
	}
	open := bytes.Replace(raw, []byte(`"policy_id":"incoming-ts"`), []byte(`"unknown":true,"policy_id":"incoming-ts"`), 1)
	if _, err := Parse(open); err == nil {
		t.Fatal("ASSERT_UNKNOWN_POLICY_FIELD_REJECTED")
	}
}
