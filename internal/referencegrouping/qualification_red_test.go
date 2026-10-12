package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/programcadmission"
)

// These literals model an externally issued test receipt. They deliberately do
// not reuse a production receipt family, policy class, identity domain, or
// constructor. The implementation under test may only admit this family via
// the explicit test-only verification seam.
const (
	testQualificationFamily  = "LSP_TRACE_TEST_GROUPING_QUALIFICATION_RECEIPT"
	testQualificationVersion = "test-v1"
	testPolicyClass          = "LSP_TRACE_TEST_GROUPING_POLICY"
	testIdentityDomain       = "LSP_TRACE_TEST_IDENTITY_V1"
)

type testQualificationCommitment struct {
	ID         string `json:"id"`
	ByteLength int    `json:"byte_length"`
	Digest     string `json:"digest"`
}

type testQualificationCase struct {
	ID, Arm, Partition, Corpus, Language, Provider, ProviderProfile, Revision string
	Required, Completed                                                       bool
	Candidates, Evaluated, Passed, Rejected, Excluded, Failed                 int
}

type testQualificationAccounting struct {
	Candidates, Evaluated, Passed, Rejected, Excluded, Failed int
}

type testQualificationMetric struct {
	Code        string `json:"code"`
	Numerator   int    `json:"numerator"`
	Denominator int    `json:"denominator"`
}

type testQualificationCriterion struct {
	Code, MetricCode, Comparator string
	Threshold                    Rational
}

type testQualificationReviewer struct {
	Identity, IdentityDomain, IdentityClass, Custody, Decision string
}

type testQualificationReceipt struct {
	Family, Version, PolicyClass, IdentityDomain string
	Policy, GroupingInput                        testQualificationCommitment
	EvaluationPlan, CorpusManifest               testQualificationCommitment
	Implementation, Environment, Report          testQualificationCommitment
	Cases                                        []testQualificationCase
	Metrics                                      []testQualificationMetric
	Criteria                                     []testQualificationCriterion
	Reviewers                                    []testQualificationReviewer
	Accounting                                   testQualificationAccounting
	RequiredReviewerClass                        string
	RequiredAcceptances                          int
	ValidFromEpoch, ValidThroughEpoch            uint64
	DecisionUnix                                 int64
	// Outcome is intentionally absent: admission derives it from complete
	// evidence and criteria rather than trusting issuer self-attestation.
}

func testCommitment(id, fill string, n int) testQualificationCommitment {
	return testQualificationCommitment{ID: id, ByteLength: n, Digest: "sha256:" + strings.Repeat(fill, 64)}
}

func testEvidenceCommitment(id string, raw []byte) testQualificationCommitment {
	sum := sha256.Sum256(raw)
	return testQualificationCommitment{ID: id, ByteLength: len(raw), Digest: fmt.Sprintf("sha256:%x", sum)}
}

func testQualificationReportSummary(totals testQualificationAccounting) []byte {
	raw, err := json.Marshal(struct {
		Family     string                      `json:"family"`
		Accounting testQualificationAccounting `json:"accounting"`
	}{"TEST_ONLY_QUALIFICATION_REPORT", totals})
	if err != nil {
		panic(err)
	}
	return append(raw, '\n')
}

func independentTestQualificationEvidence() map[string][]byte {
	return map[string][]byte{
		"evaluation-plan": []byte("TEST ONLY EVALUATION PLAN v1\n"),
		"corpus-manifest": []byte("TEST ONLY CORPUS MANIFEST v1\n"),
		"implementation":  []byte("TEST ONLY IMPLEMENTATION v1\n"),
		"environment":     []byte("TEST ONLY ENVIRONMENT v1\n"),
		"report":          testQualificationReportSummary(testQualificationAccounting{Candidates: 56, Evaluated: 42, Passed: 28, Rejected: 14, Excluded: 14, Failed: 0}),
	}
}

func independentTestQualificationTrustAnchors(t *testing.T, receipt testQualificationReceipt) map[string][]byte {
	t.Helper()
	if len(receipt.Reviewers) != 1 {
		t.Fatalf("test anchor requires exactly one reviewer, got %d", len(receipt.Reviewers))
	}
	canonicalReceipt := canonicalExternalTestReceipt(t, receipt)
	receiptSum := sha256.Sum256(canonicalReceipt)
	reviewer := receipt.Reviewers[0]
	record := struct {
		Domain                 string                        `json:"domain"`
		ReceiptDigest          string                        `json:"receipt_digest"`
		ReceiptByteLength      int                           `json:"receipt_byte_length"`
		ReviewerIdentity       string                        `json:"reviewer_identity"`
		ReviewerIdentityDomain string                        `json:"reviewer_identity_domain"`
		ReviewerIdentityClass  string                        `json:"reviewer_identity_class"`
		ReviewerCustody        string                        `json:"reviewer_custody"`
		Decision               string                        `json:"decision"`
		PolicyCommitment       testQualificationCommitment   `json:"policy_commitment"`
		InputCommitment        testQualificationCommitment   `json:"input_commitment"`
		EvidenceCommitments    []testQualificationCommitment `json:"evidence_commitments"`
	}{
		Domain:                 "lsp-trace:test-grouping-qualification-review-record:v1",
		ReceiptDigest:          fmt.Sprintf("sha256:%x", receiptSum),
		ReceiptByteLength:      len(canonicalReceipt),
		ReviewerIdentity:       reviewer.Identity,
		ReviewerIdentityDomain: reviewer.IdentityDomain,
		ReviewerIdentityClass:  reviewer.IdentityClass,
		ReviewerCustody:        reviewer.Custody,
		Decision:               reviewer.Decision,
		PolicyCommitment:       receipt.Policy,
		InputCommitment:        receipt.GroupingInput,
		EvidenceCommitments: []testQualificationCommitment{
			receipt.EvaluationPlan, receipt.CorpusManifest, receipt.Implementation, receipt.Environment, receipt.Report,
		},
	}
	canonicalRecord, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	recordSum := sha256.Sum256(append([]byte("lsp-trace:test-grouping-qualification-review-record-commitment:v1\n"), canonicalRecord...))
	anchor, err := json.Marshal(struct {
		Identity         string `json:"identity"`
		IdentityDomain   string `json:"identity_domain"`
		IdentityClass    string `json:"identity_class"`
		Custody          string `json:"custody"`
		Decision         string `json:"decision"`
		KeyCommitment    string `json:"key_commitment"`
		RecordCommitment string `json:"record_commitment"`
	}{
		Identity: reviewer.Identity, IdentityDomain: reviewer.IdentityDomain, IdentityClass: reviewer.IdentityClass,
		Custody: reviewer.Custody, Decision: reviewer.Decision,
		KeyCommitment: "sha256:" + strings.Repeat("a", 64), RecordCommitment: fmt.Sprintf("sha256:%x", recordSum),
	})
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{reviewer.Identity: append(anchor, '\n')}
}

var testCrossSeedInstabilityMetricCodes = []string{
	"CROSS_SEED_INSTABILITY_CALLS_ONLY",
	"CROSS_SEED_INSTABILITY_REFERENCES_ONLY",
	"CROSS_SEED_INSTABILITY_DEFINITIONS_ONLY",
	"CROSS_SEED_INSTABILITY_REFERENCES_PLUS_DEFINITIONS",
	"CROSS_SEED_INSTABILITY_CALLS_PLUS_REFERENCES",
	"CROSS_SEED_INSTABILITY_CALLS_PLUS_DEFINITIONS",
	"CROSS_SEED_INSTABILITY_CALLS_PLUS_REFERENCES_PLUS_DEFINITIONS",
}

func completeTestQualification(policy GroupingPolicy, input AdmittedInput) testQualificationReceipt {
	// Exact immutable synthetic pins exercise mechanics only. They are not
	// production corpus/provider names. The qualification plan leaves the real
	// CUE provider, corpus, revisions, and policy unresolved.
	const (
		testGoCorpus    = "test-only://adr0011-stage4/go-corpus@sha256:1111111111111111111111111111111111111111111111111111111111111111"
		testCueCorpus   = "test-only://adr0011-stage4/cue-corpus@sha256:2222222222222222222222222222222222222222222222222222222222222222"
		testGoProvider  = "test-only://provider/gopls@sha256:3333333333333333333333333333333333333333333333333333333333333333"
		testCueProvider = "test-only://provider/cue-lsp@sha256:4444444444444444444444444444444444444444444444444444444444444444"
		testGoRevision  = "git:5555555555555555555555555555555555555555"
		testCueRevision = "git:6666666666666666666666666666666666666666"
	)
	arms := []struct{ id, arm, partition, corpus, language, provider, profile, revision string }{
		{"calls", "CALLS_ONLY", "CALIBRATION", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"references", "REFERENCES_ONLY", "CALIBRATION", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"definitions", "DEFINITIONS_ONLY", "CALIBRATION", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"references-definitions", "REFERENCES_PLUS_DEFINITIONS", "HELD_OUT", testCueCorpus, "cue", testCueProvider, "TEST_ONLY_CUE_PROVIDER_EXACT_V1", testCueRevision},
		{"calls-references", "CALLS_PLUS_REFERENCES", "HELD_OUT", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"calls-definitions", "CALLS_PLUS_DEFINITIONS", "HELD_OUT", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"calls-references-definitions", "CALLS_PLUS_REFERENCES_PLUS_DEFINITIONS", "HELD_OUT", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"builtin-hub", "ADVERSARIAL_BUILTIN", "ADVERSARIAL", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"import-reexport", "ADVERSARIAL_IMPORT_REEXPORT", "ADVERSARIAL", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"generated-registry", "ADVERSARIAL_GENERATED", "ADVERSARIAL", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"shared-helper", "ADVERSARIAL_SHARED_HELPER", "ADVERSARIAL", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"duplicate-provider", "ADVERSARIAL_DUPLICATE_PROVIDER", "ADVERSARIAL", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"aliases-multitarget", "ADVERSARIAL_ALIAS_MULTI_TARGET", "ADVERSARIAL", testGoCorpus, "go", testGoProvider, "GO_GOPLS_EXACT_V1", testGoRevision},
		{"cross-language-revision", "ADVERSARIAL_CROSS_LANGUAGE_REVISION", "ADVERSARIAL", testCueCorpus, "cue", testCueProvider, "TEST_ONLY_CUE_PROVIDER_EXACT_V1", testCueRevision},
	}
	cases := make([]testQualificationCase, len(arms))
	for i, a := range arms {
		cases[i] = testQualificationCase{a.id, a.arm, a.partition, a.corpus, a.language, a.provider, a.profile, a.revision, true, true, 4, 3, 2, 1, 1, 0}
	}
	metrics := []testQualificationMetric{
		{"FALSE_MERGE_RATE", 0, 1}, {"FRAGMENTATION_RATE", 0, 1},
		{"STABILITY_RATE", 1, 1}, {"UNMATCHED_NO_COMMUNITY_RATE", 0, 1},
		{"SHARED_NODE_EFFECT_RATE", 0, 1}, {"HIGH_DEGREE_COLLAPSE_RATE", 0, 1},
		{"RESOURCE_LIMIT_COMPLIANCE", 1, 1}, {"DETERMINISTIC_REPLAY", 1, 1},
		{"INPUT_PERMUTATION_STABILITY", 1, 1},
	}
	// Stage4 records only identity-neutral qualification measurements. Exact
	// successor evidence identities are reserved to Stage5/7.
	for _, code := range testCrossSeedInstabilityMetricCodes {
		metrics = append(metrics, testQualificationMetric{Code: code, Numerator: 0, Denominator: 1})
	}
	criteria := make([]testQualificationCriterion, len(metrics))
	for i, m := range metrics {
		cmp, threshold := "EQ", Rational{Numerator: 0, Denominator: 1}
		if m.Code == "STABILITY_RATE" || m.Code == "RESOURCE_LIMIT_COMPLIANCE" || m.Code == "DETERMINISTIC_REPLAY" || m.Code == "INPUT_PERMUTATION_STABILITY" {
			threshold = Rational{Numerator: 1, Denominator: 1}
		}
		criteria[i] = testQualificationCriterion{"REQUIRE_" + m.Code, m.Code, cmp, threshold}
	}
	evidence := independentTestQualificationEvidence()
	return testQualificationReceipt{
		Family: testQualificationFamily, Version: testQualificationVersion,
		PolicyClass: testPolicyClass, IdentityDomain: testIdentityDomain,
		Policy:         testQualificationCommitment{ID: policy.ID(), ByteLength: policy.ByteLength(), Digest: policy.Digest()},
		GroupingInput:  testQualificationCommitment{ID: input.ID(), ByteLength: len(input.CanonicalBytes()), Digest: input.Digest()},
		EvaluationPlan: testEvidenceCommitment("evaluation-plan", evidence["evaluation-plan"]), CorpusManifest: testEvidenceCommitment("corpus-manifest", evidence["corpus-manifest"]),
		Implementation: testEvidenceCommitment("implementation", evidence["implementation"]), Environment: testEvidenceCommitment("environment", evidence["environment"]), Report: testEvidenceCommitment("report", evidence["report"]),
		Cases: cases, Metrics: metrics, Criteria: criteria,
		Accounting:            testQualificationAccounting{Candidates: 56, Evaluated: 42, Passed: 28, Rejected: 14, Excluded: 14, Failed: 0},
		Reviewers:             []testQualificationReviewer{{"reviewer-1", testIdentityDomain, "LSP_TRACE_TEST_MAINTAINER", "REVIEWER_ASSERTED", "ACCEPT"}},
		RequiredReviewerClass: "LSP_TRACE_TEST_MAINTAINER", RequiredAcceptances: 1,
		ValidFromEpoch: 7, ValidThroughEpoch: 9, DecisionUnix: 1700000000,
	}
}

func canonicalExternalTestReceipt(t *testing.T, r testQualificationReceipt) []byte {
	t.Helper()
	sort.Slice(r.Cases, func(i, j int) bool { return r.Cases[i].ID < r.Cases[j].ID })
	sort.Slice(r.Metrics, func(i, j int) bool { return r.Metrics[i].Code < r.Metrics[j].Code })
	sort.Slice(r.Criteria, func(i, j int) bool { return r.Criteria[i].Code < r.Criteria[j].Code })
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func testQualificationFixture(t *testing.T) (GroupingPolicy, AdmittedInput) {
	t.Helper()
	policy := mustParseExternalTestGroupingPolicy(t)
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	input, err := ParseAndAdmit(artifact.CanonicalBytes(), f.predecessors, generousParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	return policy, input
}

func TestQualificationDerivesCompleteOutcomeAndPinsExactPolicyAndInput(t *testing.T) {
	policy, input := testQualificationFixture(t)
	raw := canonicalExternalTestReceipt(t, completeTestQualification(policy, input))
	receipt, err := parseTestOnlyQualificationReceipt(raw, QualificationParseLimits{MaxRawBytes: 1 << 20, MaxCases: 32, MaxMetrics: 32, MaxCriteria: 32, MaxReviewers: 8, MaxStringBytes: 256})
	if err != nil || receipt.Outcome() != "QUALIFIED" {
		t.Fatalf("ASSERT_STAGE4_COMPLETE_DERIVED_QUALIFIED outcome=%q err=%v", receipt.Outcome(), err)
	}
	verified, err := verifyTestOnlyQualification(receipt, policy, input, 8)
	if err != nil || verified.PolicyID() != policy.ID() || verified.InputID() != input.ID() || verified.InputDigest() != input.Digest() || verified.InputByteLength() != len(input.CanonicalBytes()) {
		t.Fatalf("ASSERT_STAGE4_EXACT_POLICY_INPUT_BINDING verified=%+v err=%v", verified, err)
	}
}

func TestQualificationMatrixOmissionIsIncompleteFailureIsRejected(t *testing.T) {
	policy, input := testQualificationFixture(t)
	for name, mutate := range map[string]func(*testQualificationReceipt){
		"missing-arm":             func(r *testQualificationReceipt) { r.Cases = r.Cases[1:] },
		"missing-corpus-manifest": func(r *testQualificationReceipt) { r.CorpusManifest = testQualificationCommitment{} },
		"missing-case-corpus":     func(r *testQualificationReceipt) { r.Cases[0].Corpus = "" },
		"missing-language":        func(r *testQualificationReceipt) { r.Cases[0].Language = "" },
		"missing-provider":        func(r *testQualificationReceipt) { r.Cases[0].Provider = "" },
		"missing-revision":        func(r *testQualificationReceipt) { r.Cases[0].Revision = "" },
		"missing-cue-profile": func(r *testQualificationReceipt) {
			for i := range r.Cases {
				if r.Cases[i].Language == "cue" {
					r.Cases[i].ProviderProfile = ""
				}
			}
		},
		"missing-adversarial":  func(r *testQualificationReceipt) { r.Cases = r.Cases[:7] },
		"missing-metric":       func(r *testQualificationReceipt) { r.Metrics = r.Metrics[1:] },
		"missing-criterion":    func(r *testQualificationReceipt) { r.Criteria = r.Criteria[1:] },
		"missing-reviewer":     func(r *testQualificationReceipt) { r.Reviewers = nil },
		"case-required-false":  func(r *testQualificationReceipt) { r.Cases[0].Required = false },
		"case-completed-false": func(r *testQualificationReceipt) { r.Cases[0].Completed = false },
		"case-candidates-zero": func(r *testQualificationReceipt) { r.Cases[0].Candidates = 0 },
		"case-evaluated-zero":  func(r *testQualificationReceipt) { r.Cases[0].Evaluated = 0 },
		"case-passed-zero":     func(r *testQualificationReceipt) { r.Cases[0].Passed = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			mutate(&r)
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err != nil || receipt.Outcome() != "INCOMPLETE" {
				t.Fatalf("ASSERT_STAGE4_OMISSION_INCOMPLETE outcome=%q err=%v", receipt.Outcome(), err)
			}
		})
	}
	for _, metric := range []string{"FALSE_MERGE_RATE", "FRAGMENTATION_RATE", "STABILITY_RATE", "UNMATCHED_NO_COMMUNITY_RATE", "SHARED_NODE_EFFECT_RATE", "HIGH_DEGREE_COLLAPSE_RATE", "RESOURCE_LIMIT_COMPLIANCE", "DETERMINISTIC_REPLAY", "INPUT_PERMUTATION_STABILITY"} {
		t.Run("failed-"+metric, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			for i := range r.Metrics {
				if r.Metrics[i].Code == metric {
					if r.Metrics[i].Numerator == 0 {
						r.Metrics[i].Numerator = 1
					} else {
						r.Metrics[i].Numerator--
					}
				}
			}
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err != nil || receipt.Outcome() != "REJECTED" {
				t.Fatalf("ASSERT_STAGE4_FAILED_CRITERION_REJECTED outcome=%q err=%v", receipt.Outcome(), err)
			}
		})
	}
}

func TestQualificationRequiresPerModeCrossSeedInstability(t *testing.T) {
	policy, input := testQualificationFixture(t)
	for _, metricCode := range testCrossSeedInstabilityMetricCodes {
		t.Run(metricCode, func(t *testing.T) {
			criterionCode := "REQUIRE_" + metricCode
			for _, tc := range []struct {
				name   string
				mutate func(*testQualificationReceipt)
				want   string
			}{
				{"metric-omitted", func(r *testQualificationReceipt) {
					for i := range r.Metrics {
						if r.Metrics[i].Code == metricCode {
							r.Metrics = append(r.Metrics[:i], r.Metrics[i+1:]...)
							return
						}
					}
				}, "INCOMPLETE"},
				{"criterion-omitted", func(r *testQualificationReceipt) {
					for i := range r.Criteria {
						if r.Criteria[i].Code == criterionCode {
							r.Criteria = append(r.Criteria[:i], r.Criteria[i+1:]...)
							return
						}
					}
				}, "INCOMPLETE"},
				{"threshold-failed", func(r *testQualificationReceipt) {
					for i := range r.Metrics {
						if r.Metrics[i].Code == metricCode {
							r.Metrics[i].Numerator = 1
						}
					}
				}, "REJECTED"},
				{"aggregate-stability-substitution", func(r *testQualificationReceipt) {
					filtered := r.Metrics[:0]
					for _, metric := range r.Metrics {
						if metric.Code == "STABILITY_RATE" {
							continue
						}
						if metric.Code == metricCode {
							metric.Code = "STABILITY_RATE"
						}
						filtered = append(filtered, metric)
					}
					r.Metrics = filtered
				}, "INCOMPLETE"},
				{"input-permutation-substitution", func(r *testQualificationReceipt) {
					filtered := r.Metrics[:0]
					for _, metric := range r.Metrics {
						if metric.Code == "INPUT_PERMUTATION_STABILITY" {
							continue
						}
						if metric.Code == metricCode {
							metric.Code = "INPUT_PERMUTATION_STABILITY"
						}
						filtered = append(filtered, metric)
					}
					r.Metrics = filtered
				}, "INCOMPLETE"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					r := completeTestQualification(policy, input)
					tc.mutate(&r)
					receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
					if err != nil {
						t.Fatalf("ASSERT_STAGE4_PER_MODE_CROSS_SEED_DERIVED outcome=%s err=%v", tc.want, err)
					}
					if receipt.Outcome() != tc.want {
						t.Fatalf("ASSERT_STAGE4_PER_MODE_CROSS_SEED_%s outcome=%q", tc.want, receipt.Outcome())
					}
				})
			}
		})
	}
}

func TestQualificationRejectsInvalidCrossSeedInstabilityRationals(t *testing.T) {
	policy, input := testQualificationFixture(t)
	for _, metricCode := range testCrossSeedInstabilityMetricCodes {
		for _, tc := range []struct {
			name   string
			mutate func(*testQualificationReceipt)
		}{
			{"zero-denominator", func(r *testQualificationReceipt) {
				for i := range r.Metrics {
					if r.Metrics[i].Code == metricCode {
						r.Metrics[i].Denominator = 0
					}
				}
			}},
			{"negative-denominator", func(r *testQualificationReceipt) {
				for i := range r.Metrics {
					if r.Metrics[i].Code == metricCode {
						r.Metrics[i].Denominator = -1
					}
				}
			}},
			{"nonreduced", func(r *testQualificationReceipt) {
				for i := range r.Metrics {
					if r.Metrics[i].Code == metricCode {
						r.Metrics[i].Numerator, r.Metrics[i].Denominator = 2, 4
					}
				}
			}},
			{"criterion-zero-denominator", func(r *testQualificationReceipt) {
				for i := range r.Criteria {
					if r.Criteria[i].MetricCode == metricCode {
						r.Criteria[i].Threshold.Denominator = 0
					}
				}
			}},
			{"criterion-negative-denominator", func(r *testQualificationReceipt) {
				for i := range r.Criteria {
					if r.Criteria[i].MetricCode == metricCode {
						r.Criteria[i].Threshold.Denominator = -1
					}
				}
			}},
			{"criterion-nonreduced", func(r *testQualificationReceipt) {
				for i := range r.Criteria {
					if r.Criteria[i].MetricCode == metricCode {
						r.Criteria[i].Threshold.Numerator, r.Criteria[i].Threshold.Denominator = 2, 4
					}
				}
			}},
		} {
			t.Run(metricCode+"-"+tc.name, func(t *testing.T) {
				r := completeTestQualification(policy, input)
				tc.mutate(&r)
				if receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits()); err == nil && receipt.Outcome() == "QUALIFIED" {
					t.Fatal("ASSERT_STAGE4_CROSS_SEED_RATIONAL_INVALID")
				}
			})
		}
	}
}

func TestQualificationRejectsSubstitutionStalenessReplayAndDigestMismatch(t *testing.T) {
	policy, input := testQualificationFixture(t)
	base := completeTestQualification(policy, input)
	for name, mutate := range map[string]func(*testQualificationReceipt){
		"policy-id":             func(r *testQualificationReceipt) { r.Policy.ID += "-other" },
		"policy-length":         func(r *testQualificationReceipt) { r.Policy.ByteLength++ },
		"policy-digest":         func(r *testQualificationReceipt) { r.Policy.Digest = "sha256:" + strings.Repeat("9", 64) },
		"input-id":              func(r *testQualificationReceipt) { r.GroupingInput.ID += "-other" },
		"input-length":          func(r *testQualificationReceipt) { r.GroupingInput.ByteLength++ },
		"input-digest":          func(r *testQualificationReceipt) { r.GroupingInput.Digest = "sha256:" + strings.Repeat("8", 64) },
		"wrong-family":          func(r *testQualificationReceipt) { r.Family = "GROUPING_POLICY_QUALIFICATION_RECEIPT" },
		"wrong-policy-class":    func(r *testQualificationReceipt) { r.PolicyClass = "PRODUCTION" },
		"wrong-identity-domain": func(r *testQualificationReceipt) { r.IdentityDomain = "PRODUCTION" },
	} {
		t.Run(name, func(t *testing.T) {
			r := base
			mutate(&r)
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err == nil {
				_, err = verifyTestOnlyQualification(receipt, policy, input, 8)
			}
			if err == nil {
				t.Fatal("ASSERT_STAGE4_SUBSTITUTION_REJECTED")
			}
		})
	}
	receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, base), generousQualificationParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := verifyTestOnlyQualification(receipt, policy, input, 10); err == nil || !reflect.DeepEqual(got, VerifiedQualification{}) {
		t.Fatalf("ASSERT_STAGE4_STALE_REJECTED got=%+v err=%v", got, err)
	}
	recomputed, err := recomputeTestOnlyQualification(receipt)
	if err != nil || recomputed.Outcome() != receipt.Outcome() || !bytes.Equal(recomputed.CanonicalBytes(), receipt.CanonicalBytes()) {
		t.Fatalf("ASSERT_STAGE4_RECOMPUTE_PARITY err=%v", err)
	}
	original := receipt.CanonicalBytes()
	sum := sha256.Sum256(original)
	expectedDigest := fmt.Sprintf("sha256:%x", sum)
	replayedReceipt := base
	replayedReceipt.DecisionUnix++
	replayed := canonicalExternalTestReceipt(t, replayedReceipt)
	if _, err := admitTestOnlyQualificationReceipt(replayed, len(original), expectedDigest, generousQualificationParseLimits()); err == nil {
		t.Fatal("ASSERT_STAGE4_REPLAY_DIGEST_MISMATCH_REJECTED")
	}
}

func TestQualificationRejectsEveryRequiredValueSubstitutionAndDuplicate(t *testing.T) {
	policy, input := testQualificationFixture(t)
	base := completeTestQualification(policy, input)
	assertNotQualified := func(t *testing.T, r testQualificationReceipt) {
		t.Helper()
		receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
		if err == nil && receipt.Outcome() == "QUALIFIED" {
			t.Fatal("ASSERT_STAGE4_VALUE_SET_NOT_COUNT_ONLY")
		}
	}
	for i := range base.Cases {
		for _, field := range []string{"id", "arm", "partition", "corpus", "language", "provider", "provider-profile", "revision", "required", "completed", "candidates", "evaluated", "passed"} {
			t.Run(fmt.Sprintf("case-%02d-substitute-%s", i, field), func(t *testing.T) {
				r := completeTestQualification(policy, input)
				switch field {
				case "id":
					r.Cases[i].ID += "-substituted"
				case "arm":
					r.Cases[i].Arm = "SUBSTITUTED_ARM"
				case "partition":
					r.Cases[i].Partition = "SUBSTITUTED_PARTITION"
				case "corpus":
					r.Cases[i].Corpus = "test-only://substituted@sha256:" + strings.Repeat("a", 64)
				case "language":
					r.Cases[i].Language = "substituted-language"
				case "provider":
					r.Cases[i].Provider = "test-only://substituted-provider@sha256:" + strings.Repeat("b", 64)
				case "provider-profile":
					r.Cases[i].ProviderProfile = "SUBSTITUTED_PROFILE"
				case "revision":
					r.Cases[i].Revision = "git:" + strings.Repeat("c", 40)
				case "required":
					r.Cases[i].Required = !r.Cases[i].Required
				case "completed":
					r.Cases[i].Completed = !r.Cases[i].Completed
				case "candidates":
					r.Cases[i].Candidates++
				case "evaluated":
					r.Cases[i].Evaluated++
				case "passed":
					r.Cases[i].Passed++
				}
				assertNotQualified(t, r)
			})
		}
		t.Run(fmt.Sprintf("case-%02d-duplicate", i), func(t *testing.T) {
			r := completeTestQualification(policy, input)
			r.Cases = append(r.Cases, r.Cases[i])
			assertNotQualified(t, r)
		})
	}
	metricFields := []struct {
		name       string
		substitute func(*testQualificationMetric)
	}{
		{"code", func(m *testQualificationMetric) { m.Code = "SUBSTITUTED_METRIC" }},
		{"numerator", func(m *testQualificationMetric) { m.Numerator++ }},
		{"denominator", func(m *testQualificationMetric) { m.Denominator++ }},
	}
	for i := range base.Metrics {
		for _, field := range metricFields {
			t.Run("metric-"+base.Metrics[i].Code+"-substitute-"+field.name, func(t *testing.T) {
				r := completeTestQualification(policy, input)
				field.substitute(&r.Metrics[i])
				assertNotQualified(t, r)
			})
		}
		t.Run("metric-"+base.Metrics[i].Code+"-duplicate", func(t *testing.T) {
			r := completeTestQualification(policy, input)
			r.Metrics = append(r.Metrics, r.Metrics[i])
			assertNotQualified(t, r)
		})
	}
	criterionFields := []struct {
		name       string
		substitute func(*testQualificationCriterion)
	}{
		{"code", func(c *testQualificationCriterion) { c.Code = "SUBSTITUTED_CRITERION" }},
		{"metric-code", func(c *testQualificationCriterion) { c.MetricCode = "SUBSTITUTED_METRIC" }},
		{"comparator", func(c *testQualificationCriterion) { c.Comparator = "SUBSTITUTED_COMPARATOR" }},
		{"threshold-numerator", func(c *testQualificationCriterion) { c.Threshold.Numerator++ }},
		{"threshold-denominator", func(c *testQualificationCriterion) { c.Threshold.Denominator++ }},
	}
	for i := range base.Criteria {
		for _, field := range criterionFields {
			t.Run("criterion-"+base.Criteria[i].Code+"-substitute-"+field.name, func(t *testing.T) {
				r := completeTestQualification(policy, input)
				field.substitute(&r.Criteria[i])
				assertNotQualified(t, r)
			})
		}
		t.Run("criterion-"+base.Criteria[i].Code+"-duplicate", func(t *testing.T) {
			r := completeTestQualification(policy, input)
			r.Criteria = append(r.Criteria, r.Criteria[i])
			assertNotQualified(t, r)
		})
	}
	reviewerFields := []struct {
		name       string
		substitute func(*testQualificationReviewer)
	}{
		{"identity", func(r *testQualificationReviewer) { r.Identity = "substituted-reviewer" }},
		{"identity-domain", func(r *testQualificationReviewer) { r.IdentityDomain = "SUBSTITUTED_DOMAIN" }},
		{"identity-class", func(r *testQualificationReviewer) { r.IdentityClass = "SUBSTITUTED_CLASS" }},
		{"custody", func(r *testQualificationReviewer) { r.Custody = "SUBSTITUTED_CUSTODY" }},
		{"decision", func(r *testQualificationReviewer) { r.Decision = "REJECT" }},
	}
	for _, field := range reviewerFields {
		t.Run("reviewer-substitute-"+field.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			field.substitute(&r.Reviewers[0])
			assertNotQualified(t, r)
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*testQualificationReceipt)
	}{
		{"required-reviewer-class-substitute", func(r *testQualificationReceipt) { r.RequiredReviewerClass = "SUBSTITUTED_CLASS" }},
		{"required-acceptances-substitute", func(r *testQualificationReceipt) { r.RequiredAcceptances++ }},
		{"required-reviewer-duplicate", func(r *testQualificationReceipt) { r.Reviewers = append(r.Reviewers, r.Reviewers[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			tc.mutate(&r)
			assertNotQualified(t, r)
		})
	}
}

func TestQualificationRejectsEveryReceiptAndEvidenceCommitmentFieldMutation(t *testing.T) {
	policy, input := testQualificationFixture(t)
	commitments := []struct {
		name             string
		selectCommitment func(*testQualificationReceipt) *testQualificationCommitment
	}{
		{"policy", func(r *testQualificationReceipt) *testQualificationCommitment { return &r.Policy }},
		{"grouping-input", func(r *testQualificationReceipt) *testQualificationCommitment { return &r.GroupingInput }},
		{"evaluation-plan", func(r *testQualificationReceipt) *testQualificationCommitment { return &r.EvaluationPlan }},
		{"corpus-manifest", func(r *testQualificationReceipt) *testQualificationCommitment { return &r.CorpusManifest }},
		{"implementation", func(r *testQualificationReceipt) *testQualificationCommitment { return &r.Implementation }},
		{"environment", func(r *testQualificationReceipt) *testQualificationCommitment { return &r.Environment }},
		{"report", func(r *testQualificationReceipt) *testQualificationCommitment { return &r.Report }},
	}
	for _, commitment := range commitments {
		for _, field := range []string{"id", "byte-length", "digest"} {
			t.Run(commitment.name+"-"+field, func(t *testing.T) {
				r := completeTestQualification(policy, input)
				c := commitment.selectCommitment(&r)
				switch field {
				case "id":
					c.ID += "-substituted"
				case "byte-length":
					c.ByteLength++
				case "digest":
					c.Digest = "sha256:" + strings.Repeat("7", 64)
				}
				receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
				if err == nil {
					_, err = verifyTestOnlyQualification(receipt, policy, input, 8)
				}
				if err == nil {
					t.Fatalf("ASSERT_STAGE4_COMMITMENT_FIELD_REJECTED commitment=%s field=%s", commitment.name, field)
				}
			})
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*testQualificationReceipt)
	}{
		{"version", func(r *testQualificationReceipt) { r.Version = "test-v2" }},
		{"valid-from-epoch", func(r *testQualificationReceipt) { r.ValidFromEpoch++ }},
		{"valid-through-epoch", func(r *testQualificationReceipt) { r.ValidThroughEpoch-- }},
		{"decision-unix", func(r *testQualificationReceipt) { r.DecisionUnix++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			tc.mutate(&r)
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err == nil {
				_, err = verifyTestOnlyQualification(receipt, policy, input, 8)
			}
			if err == nil {
				t.Fatalf("ASSERT_STAGE4_RECEIPT_FIELD_REJECTED field=%s", tc.name)
			}
		})
	}
}

func productionShapedTestQualification(policy GroupingPolicy, input AdmittedInput) testQualificationReceipt {
	r := completeTestQualification(policy, input)
	r.Family = "GROUPING_POLICY_QUALIFICATION_RECEIPT"
	r.Version = "v1"
	r.PolicyClass = "GROUPING_POLICY"
	r.IdentityDomain = "lsp-trace:grouping-policy-qualification-receipt:v1"
	for i := range r.Reviewers {
		r.Reviewers[i].IdentityDomain = r.IdentityDomain
	}
	return r
}

func TestSyntheticPackageIsTestOnlyAndCannotCreateProductionQualification(t *testing.T) {
	policy, input := testQualificationFixture(t)
	raw := canonicalExternalTestReceipt(t, completeTestQualification(policy, input))
	testReceipt, err := parseTestOnlyQualificationReceipt(raw, generousQualificationParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := verifyTestOnlyQualificationWithEvidence(testReceipt, independentTestQualificationEvidence(), independentTestQualificationTrustAnchors(t, completeTestQualification(policy, input)), policy, input, 8); err != nil || !got.Matches(policy, input, 8) {
		t.Fatalf("ASSERT_EXPLICIT_TEST_ONLY_EVIDENCE_AND_TRUST_ADMISSION got=%+v err=%v", got, err)
	}
	if got, err := ParseGroupingPolicyQualificationReceipt(raw, generousQualificationParseLimits()); err == nil || !reflect.DeepEqual(got, GroupingPolicyQualificationReceipt{}) {
		t.Fatalf("ASSERT_PRODUCTION_PARSER_REJECTS_SAME_SYNTHETIC_PACKAGE got=%+v err=%v", got, err)
	}
	productionShaped := productionShapedTestQualification(policy, input)
	productionReceipt, err := ParseGroupingPolicyQualificationReceipt(canonicalExternalTestReceipt(t, productionShaped), generousQualificationParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := VerifyGroupingPolicyQualification(productionReceipt, policy, input, 8); err == nil || !reflect.DeepEqual(got, VerifiedQualification{}) {
		t.Fatalf("ASSERT_SYNTHETIC_INPUTS_NEVER_YIELD_PRODUCTION_VERIFIED_QUALIFICATION got=%+v err=%v", got, err)
	}
}

func TestQualificationDecisionClassificationDeclineMissingAndUnknown(t *testing.T) {
	policy, input := testQualificationFixture(t)
	for _, tc := range []struct {
		name, decision, want string
		wantError            bool
	}{
		{"decline-complete", "DECLINE", "REJECTED", false},
		{"missing", "", "INCOMPLETE", false},
		{"unknown", "MAYBE", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			r.Reviewers[0].Decision = tc.decision
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if tc.wantError {
				if err == nil {
					t.Fatalf("ASSERT_REVIEWER_UNKNOWN_DECISION_ERROR outcome=%q", receipt.Outcome())
				}
				return
			}
			if err != nil || receipt.Outcome() != tc.want {
				t.Fatalf("ASSERT_REVIEWER_DECISION_CLASSIFICATION want=%q got=%q err=%v", tc.want, receipt.Outcome(), err)
			}
		})
	}
}

func TestQualificationExactCaseAndAggregateAccounting(t *testing.T) {
	policy, input := testQualificationFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(*testQualificationReceipt)
	}{
		{"candidates-unreconciled", func(r *testQualificationReceipt) { r.Cases[0].Candidates++ }},
		{"evaluated-plus-excluded-failed-unreconciled", func(r *testQualificationReceipt) { r.Cases[0].Excluded = 2 }},
		{"failed-unreconciled", func(r *testQualificationReceipt) { r.Cases[0].Failed = 1 }},
		{"passed-exceeds-evaluated", func(r *testQualificationReceipt) { r.Cases[0].Passed = r.Cases[0].Evaluated + 1 }},
		{"aggregate-denominator-substitution", func(r *testQualificationReceipt) { r.Metrics[0].Denominator += r.Cases[0].Evaluated }},
		{"aggregate-numerator-substitution", func(r *testQualificationReceipt) { r.Metrics[0].Numerator++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			tc.mutate(&r)
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err == nil && receipt.Outcome() == "QUALIFIED" {
				t.Fatal("ASSERT_EXACT_CASE_AND_AGGREGATE_ACCOUNTING")
			}
		})
	}
}

func TestProductionQualificationWithoutExplicitImmutablePinsIsIncomplete(t *testing.T) {
	policy, input := testQualificationFixture(t)
	r := completeTestQualification(policy, input)
	r.Family = "GROUPING_POLICY_QUALIFICATION_RECEIPT"
	r.Version = "v1"
	r.PolicyClass = "GROUPING_POLICY"
	r.IdentityDomain = "lsp-trace:grouping-policy-qualification-receipt:v1"
	for i := range r.Cases {
		r.Cases[i].Corpus, r.Cases[i].Provider, r.Cases[i].ProviderProfile, r.Cases[i].Revision = "", "", "", ""
	}
	receipt, err := ParseGroupingPolicyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome() != "INCOMPLETE" {
		t.Fatalf("ASSERT_STAGE4_PRODUCTION_UNRESOLVED_PINS_INCOMPLETE outcome=%q", receipt.Outcome())
	}
}

func TestProductionVerifierCannotAcceptTestOnlyReceiptFamily(t *testing.T) {
	policy, input := testQualificationFixture(t)
	raw := canonicalExternalTestReceipt(t, completeTestQualification(policy, input))
	if got, err := ParseGroupingPolicyQualificationReceipt(raw, generousQualificationParseLimits()); err == nil || !reflect.DeepEqual(got, GroupingPolicyQualificationReceipt{}) {
		t.Fatalf("ASSERT_STAGE4_PRODUCTION_VERIFIER_REJECTS_TEST_FAMILY got=%+v err=%v", got, err)
	}
}

func TestTestOnlyQualificationRequiresIndependentEvidenceAndReviewerTrust(t *testing.T) {
	policy, input := testQualificationFixture(t)
	r := completeTestQualification(policy, input)
	raw := canonicalExternalTestReceipt(t, r)
	receipt, err := parseTestOnlyQualificationReceipt(raw, generousQualificationParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := any(receipt).(VerifiedQualification); ok {
		t.Fatal("ASSERT_PARSE_RECEIPT_CAN_NEVER_YIELD_VERIFIED_QUALIFICATION")
	}
	evidence := independentTestQualificationEvidence()
	anchors := independentTestQualificationTrustAnchors(t, r)
	verified, err := verifyTestOnlyQualificationWithEvidence(receipt, evidence, anchors, policy, input, 8)
	if err != nil || !verified.Matches(policy, input, 8) {
		t.Fatalf("ASSERT_EXPLICIT_TEST_ONLY_EVIDENCE_AND_TRUST_ADMITTED verified=%+v err=%v", verified, err)
	}

	clone := func(in map[string][]byte) map[string][]byte {
		out := make(map[string][]byte, len(in))
		for k, v := range in {
			out[k] = append([]byte(nil), v...)
		}
		return out
	}
	for name, mutate := range map[string]func(map[string][]byte, map[string][]byte){
		"missing-evaluation-plan":    func(e, _ map[string][]byte) { delete(e, "evaluation-plan") },
		"missing-corpus-manifest":    func(e, _ map[string][]byte) { delete(e, "corpus-manifest") },
		"missing-implementation":     func(e, _ map[string][]byte) { delete(e, "implementation") },
		"missing-environment":        func(e, _ map[string][]byte) { delete(e, "environment") },
		"missing-report":             func(e, _ map[string][]byte) { delete(e, "report") },
		"evidence-byte-substitution": func(e, _ map[string][]byte) { e["report"][0] ^= 0xff },
		"missing-anchor":             func(_ map[string][]byte, a map[string][]byte) { delete(a, "reviewer-1") },
		"anchor-identity-substitution": func(_ map[string][]byte, a map[string][]byte) {
			a["reviewer-1"] = bytes.Replace(a["reviewer-1"], []byte("reviewer-1"), []byte("reviewer-2"), 1)
		},
		"anchor-domain-substitution": func(_ map[string][]byte, a map[string][]byte) {
			a["reviewer-1"] = bytes.Replace(a["reviewer-1"], []byte(testIdentityDomain), []byte("UNTRUSTED_TEST_DOMAIN"), 1)
		},
		"anchor-class-substitution": func(_ map[string][]byte, a map[string][]byte) {
			a["reviewer-1"] = bytes.Replace(a["reviewer-1"], []byte("LSP_TRACE_TEST_MAINTAINER"), []byte("UNTRUSTED_REVIEWER"), 1)
		},
		"anchor-key-commitment-substitution": func(_ map[string][]byte, a map[string][]byte) {
			a["reviewer-1"] = bytes.Replace(a["reviewer-1"], []byte(strings.Repeat("a", 64)), []byte(strings.Repeat("c", 64)), 1)
		},
		"anchor-record-commitment-substitution": func(_ map[string][]byte, a map[string][]byte) {
			anchor := append([]byte(nil), a["reviewer-1"]...)
			prefix := []byte(`"record_commitment":"sha256:`)
			at := bytes.Index(anchor, prefix)
			if at < 0 {
				panic("BLOCKED_NOT_RED record commitment absent")
			}
			anchor[at+len(prefix)] = map[byte]byte{'0': '1', '1': '0'}[anchor[at+len(prefix)]]
			if anchor[at+len(prefix)] == 0 {
				anchor[at+len(prefix)] = '0'
			}
			a["reviewer-1"] = anchor
		},
	} {
		t.Run(name, func(t *testing.T) {
			e, a := clone(evidence), clone(anchors)
			mutate(e, a)
			if got, err := verifyTestOnlyQualificationWithEvidence(receipt, e, a, policy, input, 8); err == nil || !reflect.DeepEqual(got, VerifiedQualification{}) {
				t.Fatalf("ASSERT_INDEPENDENT_EVIDENCE_OR_TRUST_SUBSTITUTION_REJECTED got=%+v err=%v", got, err)
			}
		})
	}
	if got, err := verifyTestOnlyQualificationWithEvidence(receipt, evidence, anchors, policy, input, 10); err == nil || !reflect.DeepEqual(got, VerifiedQualification{}) {
		t.Fatalf("ASSERT_TEST_ONLY_INDEPENDENT_VERIFIER_STALE_REJECTED got=%+v err=%v", got, err)
	}
	sum := sha256.Sum256(raw)
	if got, err := admitTestOnlyQualificationWithEvidence(raw, len(raw), fmt.Sprintf("sha256:%x", sum), evidence, anchors, policy, input, 8, generousQualificationParseLimits()); err != nil || !got.Matches(policy, input, 8) {
		t.Fatalf("ASSERT_TEST_ONLY_RECEIPT_ADMITTER_EXACT_REPLAY_PIN got=%+v err=%v", got, err)
	}
	if got, err := admitTestOnlyQualificationWithEvidence(raw, len(raw)+1, fmt.Sprintf("sha256:%x", sum), evidence, anchors, policy, input, 8, generousQualificationParseLimits()); err == nil || !reflect.DeepEqual(got, VerifiedQualification{}) {
		t.Fatalf("ASSERT_TEST_ONLY_RECEIPT_REPLAY_LENGTH_REJECTED got=%+v err=%v", got, err)
	}
	if got, err := admitTestOnlyQualificationWithEvidence(raw, len(raw), "sha256:"+strings.Repeat("f", 64), evidence, anchors, policy, input, 8, generousQualificationParseLimits()); err == nil || !reflect.DeepEqual(got, VerifiedQualification{}) {
		t.Fatalf("ASSERT_TEST_ONLY_RECEIPT_REPLAY_DIGEST_REJECTED got=%+v err=%v", got, err)
	}
}

func TestQualificationRecordCommitmentIsExactDecisionAuthorization(t *testing.T) {
	policy, input := testQualificationFixture(t)
	base := completeTestQualification(policy, input)
	verify := func(t *testing.T, r testQualificationReceipt, anchors map[string][]byte, wantOutcome string, wantAccepted bool) {
		t.Helper()
		receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
		if err == nil && wantOutcome != "" && receipt.Outcome() != wantOutcome {
			t.Fatalf("ASSERT_RECORD_COMMITMENT_OUTCOME want=%q got=%q", wantOutcome, receipt.Outcome())
		}
		if err == nil {
			_, err = verifyTestOnlyQualificationWithEvidence(receipt, independentTestQualificationEvidence(), anchors, policy, input, 8)
		}
		if wantAccepted && err != nil {
			t.Fatalf("ASSERT_EXACT_RECORD_COMMITMENT_ACCEPTED outcome=%q err=%v", wantOutcome, err)
		}
		if !wantAccepted && err == nil {
			t.Fatalf("ASSERT_INEXACT_RECORD_COMMITMENT_REJECTED outcome=%q", wantOutcome)
		}
	}

	t.Run("exact-accept-anchor", func(t *testing.T) {
		verify(t, base, independentTestQualificationTrustAnchors(t, base), "QUALIFIED", true)
	})
	for _, tc := range []struct {
		name   string
		mutate func(*testQualificationReceipt)
	}{
		{"decision-retains-accept-anchor", func(r *testQualificationReceipt) { r.Reviewers[0].Decision = "DECLINE" }},
		{"reviewer-identity-retains-anchor", func(r *testQualificationReceipt) { r.Reviewers[0].Identity = "reviewer-2" }},
		{"reviewer-domain-retains-anchor", func(r *testQualificationReceipt) { r.Reviewers[0].IdentityDomain = "SUBSTITUTED_DOMAIN" }},
		{"reviewer-class-retains-anchor", func(r *testQualificationReceipt) { r.Reviewers[0].IdentityClass = "SUBSTITUTED_CLASS" }},
		{"reviewer-custody-retains-anchor", func(r *testQualificationReceipt) { r.Reviewers[0].Custody = "SUBSTITUTED_CUSTODY" }},
		{"policy-commitment-retains-anchor", func(r *testQualificationReceipt) { r.Policy.Digest = "sha256:" + strings.Repeat("7", 64) }},
		{"input-commitment-retains-anchor", func(r *testQualificationReceipt) { r.GroupingInput.Digest = "sha256:" + strings.Repeat("8", 64) }},
		{"evidence-commitment-retains-anchor", func(r *testQualificationReceipt) { r.Report.Digest = "sha256:" + strings.Repeat("9", 64) }},
		{"receipt-bytes-retain-anchor", func(r *testQualificationReceipt) { r.DecisionUnix++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			anchors := independentTestQualificationTrustAnchors(t, r)
			tc.mutate(&r)
			wantOutcome := ""
			if r.Reviewers[0].Decision == "DECLINE" {
				wantOutcome = "REJECTED"
			}
			verify(t, r, anchors, wantOutcome, false)
		})
	}

	t.Run("key-commitment-remains-required", func(t *testing.T) {
		anchors := independentTestQualificationTrustAnchors(t, base)
		anchors["reviewer-1"] = bytes.Replace(anchors["reviewer-1"], []byte(strings.Repeat("a", 64)), []byte(strings.Repeat("c", 64)), 1)
		verify(t, base, anchors, "QUALIFIED", false)
	})
	t.Run("exact-decline-anchor-authorizes-rejection-without-qualification", func(t *testing.T) {
		r := completeTestQualification(policy, input)
		r.Reviewers[0].Decision = "DECLINE"
		receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
		if err != nil || receipt.Outcome() != "REJECTED" {
			t.Fatalf("ASSERT_DECLINE_DERIVES_REJECTED outcome=%q err=%v", receipt.Outcome(), err)
		}
		anchors := independentTestQualificationTrustAnchors(t, r)
		if err := verifyQualificationReviewerAuthorization(receipt, anchors); err != nil {
			t.Fatalf("ASSERT_EXACT_DECLINE_RECORD_AUTHORIZED err=%v", err)
		}
		if got, err := verifyTestOnlyQualificationWithEvidence(receipt, independentTestQualificationEvidence(), anchors, policy, input, 8); err == nil || !reflect.DeepEqual(got, VerifiedQualification{}) {
			t.Fatalf("ASSERT_DECLINE_CANNOT_YIELD_VERIFIED_QUALIFICATION got=%+v err=%v", got, err)
		}
	})
	t.Run("fabricated-accept-cannot-reuse-decline-anchor", func(t *testing.T) {
		decline := completeTestQualification(policy, input)
		decline.Reviewers[0].Decision = "DECLINE"
		anchors := independentTestQualificationTrustAnchors(t, decline)
		fabricatedAccept := decline
		fabricatedAccept.Reviewers = append([]testQualificationReviewer(nil), decline.Reviewers...)
		fabricatedAccept.Reviewers[0].Decision = "ACCEPT"
		verify(t, fabricatedAccept, anchors, "QUALIFIED", false)
	})
}

func TestQualificationReportSummaryAndReceiptAccountingReconcileIndependently(t *testing.T) {
	policy, input := testQualificationFixture(t)
	fields := []struct {
		name   string
		mutate func(*testQualificationAccounting)
	}{
		{"candidates", func(a *testQualificationAccounting) { a.Candidates++ }},
		{"evaluated", func(a *testQualificationAccounting) { a.Evaluated++ }},
		{"passed", func(a *testQualificationAccounting) { a.Passed++ }},
		{"rejected", func(a *testQualificationAccounting) { a.Rejected++ }},
		{"excluded", func(a *testQualificationAccounting) { a.Excluded++ }},
		{"failed", func(a *testQualificationAccounting) { a.Failed++ }},
	}
	for _, field := range fields {
		t.Run("report-mutated-receipt-aggregate-fixed-"+field.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			evidence := independentTestQualificationEvidence()
			reportAccounting := r.Accounting
			field.mutate(&reportAccounting)
			evidence["report"] = testQualificationReportSummary(reportAccounting)
			r.Report = testEvidenceCommitment("report", evidence["report"])
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err != nil {
				t.Fatalf("ASSERT_RECOMMITTED_REPORT_SUMMARY_MISMATCH_PARSE_SUCCESS field=%s err=%v", field.name, err)
			}
			if receipt.Outcome() != "INCOMPLETE" {
				t.Fatalf("ASSERT_RECOMMITTED_REPORT_SUMMARY_MISMATCH_INCOMPLETE field=%s outcome=%q", field.name, receipt.Outcome())
			}
		})
		t.Run("receipt-aggregate-mutated-report-fixed-"+field.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			field.mutate(&r.Accounting)
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err != nil {
				t.Fatalf("ASSERT_FIXED_REPORT_RECEIPT_AGGREGATE_MISMATCH_PARSE_SUCCESS field=%s err=%v", field.name, err)
			}
			if receipt.Outcome() != "INCOMPLETE" {
				t.Fatalf("ASSERT_FIXED_REPORT_RECEIPT_AGGREGATE_MISMATCH_INCOMPLETE field=%s outcome=%q", field.name, receipt.Outcome())
			}
		})
	}
}

func TestQualificationExactAccountingEquationsAndReportSummary(t *testing.T) {
	policy, input := testQualificationFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(*testQualificationReceipt)
	}{
		{"case-candidates", func(r *testQualificationReceipt) { r.Cases[0].Candidates++ }},
		{"case-evaluated", func(r *testQualificationReceipt) { r.Cases[0].Evaluated++ }},
		{"case-passed", func(r *testQualificationReceipt) { r.Cases[0].Passed++ }},
		{"case-rejected", func(r *testQualificationReceipt) { r.Cases[0].Rejected++ }},
		{"case-excluded", func(r *testQualificationReceipt) { r.Cases[0].Excluded++ }},
		{"case-failed", func(r *testQualificationReceipt) { r.Cases[0].Failed++ }},
		{"aggregate-candidates", func(r *testQualificationReceipt) { r.Accounting.Candidates++ }},
		{"aggregate-evaluated", func(r *testQualificationReceipt) { r.Accounting.Evaluated++ }},
		{"aggregate-passed", func(r *testQualificationReceipt) { r.Accounting.Passed++ }},
		{"aggregate-rejected", func(r *testQualificationReceipt) { r.Accounting.Rejected++ }},
		{"aggregate-excluded", func(r *testQualificationReceipt) { r.Accounting.Excluded++ }},
		{"aggregate-failed", func(r *testQualificationReceipt) { r.Accounting.Failed++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := completeTestQualification(policy, input)
			tc.mutate(&r)
			receipt, err := parseTestOnlyQualificationReceipt(canonicalExternalTestReceipt(t, r), generousQualificationParseLimits())
			if err != nil {
				t.Fatalf("ASSERT_CASE_AND_REPORT_ACCOUNTING_MISMATCH_PARSE_SUCCESS err=%v", err)
			}
			if receipt.Outcome() != "INCOMPLETE" {
				t.Fatalf("ASSERT_CASE_AND_REPORT_ACCOUNTING_MISMATCH_INCOMPLETE outcome=%q", receipt.Outcome())
			}
		})
	}
	// Rational metrics are asserted as exact reduced values separately. Their
	// denominators are not raw case/report totals and must not be equated to them.
	r := completeTestQualification(policy, input)
	for _, metric := range r.Metrics {
		if metric.Denominator != 1 || (metric.Numerator != 0 && metric.Numerator != 1) {
			t.Fatalf("ASSERT_EXACT_REDUCED_METRIC_VALUE metric=%+v", metric)
		}
	}
}

func generousQualificationParseLimits() QualificationParseLimits {
	return QualificationParseLimits{MaxRawBytes: 1 << 20, MaxCases: 32, MaxMetrics: 32, MaxCriteria: 32, MaxReviewers: 8, MaxStringBytes: 256}
}
