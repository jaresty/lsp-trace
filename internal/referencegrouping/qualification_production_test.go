package referencegrouping

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/programcadmission"
)

// CONTROLLED_CODE_VERIFICATION_NOT_ACTUAL_POLICY_QUALIFICATION. The fixture
// authority below is constructed only in this _test.go file; the real factory
// remains UNISSUED. No fixture is installed in a production registry.
type controlledQualificationResolver struct {
	artifacts map[string][]byte
	calls     int
}

func (r *controlledQualificationResolver) ResolveQualificationArtifact(role, digest string, n int) ([]byte, error) {
	r.calls++
	return r.artifacts[role], nil
}

type controlledQualificationBundle struct {
	owner    *qualificationOwner
	resolver *controlledQualificationResolver
	receipt  GroupingPolicyQualificationReceipt
	policy   GroupingPolicy
	input    AdmittedInput
	plan     qualificationPlanEvidence
	manifest qualificationManifestEvidence
	report   qualificationReportEvidence
}

func productionTestLimits() productionQualificationLimits {
	return productionQualificationLimits{Receipt: generousQualificationParseLimits(), MaxArtifactBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxTokens: 100000, MaxDepth: 16, MaxRecords: 4096}
}
func productionTestBytes(t *testing.T, v any) []byte {
	t.Helper()
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return append(raw, '\n')
}
func productionTestCommit(id string, raw []byte) qualificationCommitment {
	return qualificationCommitment{id, len(raw), receiptDigest(raw)}
}
func (b *controlledQualificationBundle) seal(t *testing.T, authorize bool) {
	t.Helper()
	d := b.receipt.doc
	raw := b.resolver.artifacts
	raw["corpus-manifest"] = productionTestBytes(t, b.manifest)
	d.CorpusManifest = productionTestCommit("corpus-manifest", raw["corpus-manifest"])
	b.plan.CorpusManifest = d.CorpusManifest
	raw["evaluation-plan"] = productionTestBytes(t, b.plan)
	d.EvaluationPlan = productionTestCommit("evaluation-plan", raw["evaluation-plan"])
	b.report.EvaluationPlan = d.EvaluationPlan
	b.report.CorpusManifest = d.CorpusManifest
	raw["report"] = productionTestBytes(t, b.report)
	d.Report = productionTestCommit("report", raw["report"])
	parsed, e := ParseGroupingPolicyQualificationReceipt(productionTestBytes(t, d), productionTestLimits().Receipt)
	if e != nil {
		t.Fatal(e)
	}
	b.receipt = parsed
	if authorize {
		// A package-private whitebox fixture, never a production hook.
		b.owner.approvals = []qualificationApproval{{Receipt: productionTestCommit("receipt", parsed.canonical), Policy: d.Policy, Input: d.GroupingInput, Evidence: [5]qualificationCommitment{d.EvaluationPlan, d.CorpusManifest, d.Implementation, d.Environment, d.Report}, RequiredClass: b.plan.RequiredReviewerClass, RequiredAcceptances: b.plan.RequiredAcceptances, Reviewers: append([]qualificationReviewer(nil), d.Reviewers...), ValidFromEpoch: d.ValidFromEpoch, ValidThroughEpoch: d.ValidThroughEpoch, FrozenUnix: b.plan.FrozenUnix, DecisionUnix: d.DecisionUnix}}
	}
}
func controlledProductionQualification(t *testing.T, kinds []programcadmission.RelationKind) *controlledQualificationBundle {
	t.Helper()
	p := stage5Policy(t, nil)
	f := referenceFixture(t, kinds)
	artifact, e := BuildInput(f.request())
	if e != nil {
		t.Fatal(e)
	}
	in, e := ParseAndAdmit(artifact.CanonicalBytes(), f.predecessors, generousParseLimits())
	if e != nil {
		t.Fatal(e)
	}
	var d qualificationDocument
	if e = json.Unmarshal(productionTestBytes(t, completeTestQualification(p, in)), &d); e != nil {
		t.Fatal(e)
	}
	d.Family = "GROUPING_POLICY_QUALIFICATION_RECEIPT"
	d.Version = "v1"
	d.PolicyClass = "GROUPING_POLICY"
	d.IdentityDomain = "lsp-trace:grouping-policy-qualification-receipt:v1"
	d.RequiredReviewerClass = "CONTROLLED_MAINTAINER"
	d.ValidFromEpoch = 107
	d.ValidThroughEpoch = 109
	d.DecisionUnix = 1800000300
	d.Reviewers = []qualificationReviewer{{"controlled-reviewer", d.IdentityDomain, d.RequiredReviewerClass, "REVIEWER_ASSERTED", "ACCEPT"}}
	for j := range d.Cases {
		c := &d.Cases[j]
		c.Corpus = strings.ReplaceAll(c.Corpus, "test-only://", "controlled-evaluation://")
		c.Provider = strings.ReplaceAll(c.Provider, "test-only://", "controlled-evaluation://")
		c.ProviderProfile = strings.ReplaceAll(c.ProviderProfile, "TEST_ONLY_CUE_PROVIDER", "CUE_CONTROLLED")
	}
	sort.Slice(d.Cases, func(i, j int) bool { return d.Cases[i].ID < d.Cases[j].ID })
	sort.Slice(d.Metrics, func(i, j int) bool { return d.Metrics[i].Code < d.Metrics[j].Code })
	sort.Slice(d.Criteria, func(i, j int) bool { return d.Criteria[i].Code < d.Criteria[j].Code })
	raw := map[string][]byte{}
	for _, role := range []string{"implementation", "environment"} {
		raw[role] = productionTestBytes(t, qualificationIdentityEvidence{"GROUPING_QUALIFICATION_" + strings.ToUpper(role), "v1", "controlled-code-verification", "not actual qualification"})
	}
	d.Implementation = productionTestCommit("implementation", raw["implementation"])
	d.Environment = productionTestCommit("environment", raw["environment"])
	resolver := &controlledQualificationResolver{artifacts: raw}
	owner, e := newQualificationOwner(productionTestLimits(), resolver)
	if e != nil {
		t.Fatal(e)
	}
	b := &controlledQualificationBundle{owner: owner, resolver: resolver, receipt: GroupingPolicyQualificationReceipt{doc: d}, policy: p, input: in}
	b.plan = qualificationPlanEvidence{Family: "GROUPING_QUALIFICATION_PLAN", Version: "v1", Policy: d.Policy, Input: d.GroupingInput, Implementation: d.Implementation, Environment: d.Environment, Criteria: d.Criteria, RequiredReviewerClass: d.RequiredReviewerClass, RequiredAcceptances: 1, FrozenUnix: 1800000100}
	b.manifest = qualificationManifestEvidence{Family: "GROUPING_QUALIFICATION_CORPUS", Version: "v1"}
	b.report = qualificationReportEvidence{Family: "GROUPING_QUALIFICATION_REPORT", Version: "v1", Policy: d.Policy, Input: d.GroupingInput, Implementation: d.Implementation, Environment: d.Environment, ExecutionUnix: 1800000200, Metrics: d.Metrics, Accounting: d.Accounting, Uncertainty: "controlled measurements only; no live corpus claim"}
	for _, c := range d.Cases {
		spec := qualificationCaseSpec{c.ID, c.Arm, c.Partition, c.Corpus, c.Language, c.Provider, c.ProviderProfile, c.Revision, []string{"a:group-1", "b:group-1"}}
		b.plan.Cases = append(b.plan.Cases, spec)
		b.manifest.Cases = append(b.manifest.Cases, spec)
		b.report.Cases = append(b.report.Cases, qualificationCaseResult{c, spec.ExpectedGrouping, append([]string(nil), spec.ExpectedGrouping...)})
	}
	for _, m := range d.Metrics {
		def := qualificationMetricDefinition{Code: m.Code, Definition: "controlled retained counter for " + m.Code, Aggregation: "SUM_EXACT_COUNTS"}
		for _, c := range d.Cases {
			if strings.HasPrefix(m.Code, "CROSS_SEED_INSTABILITY_") && c.Arm != strings.TrimPrefix(m.Code, "CROSS_SEED_INSTABILITY_") {
				continue
			}
			def.CaseIDs = append(def.CaseIDs, c.ID)
			b.report.Observations = append(b.report.Observations, qualificationObservation{c.ID, m.Code, m.Numerator, 1})
		}
		b.plan.MetricDefinitions = append(b.plan.MetricDefinitions, def)
	}
	b.seal(t, true)
	return b
}

func TestNONTEST_QUALIFICATION_REACHES_TYPED_COMPOSITION(t *testing.T) {
	modes := [][]programcadmission.RelationKind{
		{programcadmission.TypedCalls}, {programcadmission.TypedReferencesSymbol}, {programcadmission.TypedResolvesToDefinition},
		{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition},
		{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol}, {programcadmission.TypedCalls, programcadmission.TypedResolvesToDefinition},
		{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition},
	}
	for j, kinds := range modes {
		t.Run(requiredArms[j], func(t *testing.T) {
			b := controlledProductionQualification(t, kinds)
			q, e := b.owner.Verify(b.receipt, b.policy, b.input, 108)
			if e != nil {
				t.Fatalf("NONTEST_QUALIFICATION_REACHES_TYPED_COMPOSITION: %v", e)
			}
			composed, e := ComposeQualified(b.input, b.policy, q, 108, stage5Limits())
			if e != nil {
				t.Fatalf("NONTEST_QUALIFICATION_REACHES_TYPED_COMPOSITION: %v", e)
			}
			source := b.input.Contributions()
			got := composed.Occurrences()
			if len(got) != len(source) || len(composed.PairContributions()) != len(source) || !reflect.DeepEqual(composed.Custody(), b.input.EndpointCustody()) {
				t.Fatal("NONTEST_QUALIFICATION_REACHES_TYPED_COMPOSITION: typed multiplicity/custody")
			}
			for k, c := range source {
				o := got[k]
				if o.Kind != c.Kind || o.From != c.From || o.To != c.To || o.Ordinal != c.Ordinal || o.OccurrenceIdentity != c.OccurrenceIdentity || o.SourceCustodyIdentity != b.input.CustodyIdentityForOccurrence(c.OccurrenceIdentity) || o.SourcePredecessorIdentity != b.input.PredecessorIdentityForContribution(c) {
					t.Fatal("NONTEST_QUALIFICATION_REACHES_TYPED_COMPOSITION: direction/identity")
				}
			}
			t.Log("NONTEST_QUALIFICATION_REACHES_TYPED_COMPOSITION A-pass")
		})
	}
}

func requireProductionQualificationRefusal(t *testing.T, b *controlledQualificationBundle, identity, outcome string) {
	t.Helper()
	q, e := b.owner.Verify(b.receipt, b.policy, b.input, 108)
	var failure *qualificationFailure
	if e == nil || !errors.As(e, &failure) || failure.Outcome != outcome || q != (VerifiedQualification{}) {
		t.Fatalf("%s A-fail: qualification=%+v error=%v", identity, q, e)
	}
	composition, ce := ComposeQualified(b.input, b.policy, q, 108, stage5Limits())
	if ce == nil || composition.ByteLength() != 0 {
		t.Fatalf("%s A-fail: unqualified composition effects", identity)
	}
	t.Log(identity + " A-pass")
}
func productionReferenceBundle(t *testing.T) *controlledQualificationBundle {
	return controlledProductionQualification(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
}

func TestProductionQualificationIndependentAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*controlledQualificationBundle)
	}{
		{"UNISSUED_DEFAULT", func(b *controlledQualificationBundle) {
			o, e := newQualificationOwner(productionTestLimits(), b.resolver)
			if e != nil {
				t.Fatal(e)
			}
			b.owner = o
		}},
		{"ABSENT_MAINTAINER_RECORD", func(b *controlledQualificationBundle) { b.owner.approvals = nil }},
		{"AUTHORIZATION_SUBJECT_SCOPE", func(b *controlledQualificationBundle) {
			b.owner.approvals[0].Receipt.Digest = "sha256:" + strings.Repeat("b", 64)
		}},
		{"AUTHORIZATION_POLICY_SCOPE", func(b *controlledQualificationBundle) {
			b.owner.approvals[0].Policy.Digest = "sha256:" + strings.Repeat("b", 64)
		}},
		{"AUTHORIZATION_INPUT_SCOPE", func(b *controlledQualificationBundle) {
			b.owner.approvals[0].Input.Digest = "sha256:" + strings.Repeat("b", 64)
		}},
		{"AUTHORIZATION_ARTIFACT_SCOPE", func(b *controlledQualificationBundle) {
			b.owner.approvals[0].Evidence[4].Digest = "sha256:" + strings.Repeat("b", 64)
		}},
		{"AUTHORIZATION_CLASS", func(b *controlledQualificationBundle) { b.owner.approvals[0].RequiredClass = "foreign-class" }},
		{"AUTHORIZATION_COUNT", func(b *controlledQualificationBundle) { b.owner.approvals[0].RequiredAcceptances = 2 }},
		{"AUTHORIZATION_EPOCH", func(b *controlledQualificationBundle) { b.owner.approvals[0].ValidThroughEpoch++ }},
		{"AUTHORIZATION_TIMESTAMP", func(b *controlledQualificationBundle) { b.owner.approvals[0].DecisionUnix++ }},
		{"AUTHORIZATION_REVIEWER_DECLINE", func(b *controlledQualificationBundle) { b.owner.approvals[0].Reviewers[0].Decision = "DECLINE" }},
		{"EXTRA_AUTHORIZATION_NO_RELAXATION", func(b *controlledQualificationBundle) {
			a := b.owner.approvals[0]
			a.Reviewers = append([]qualificationReviewer(nil), a.Reviewers...)
			a.Reviewers[0].Decision = "DECLINE"
			b.owner.approvals = append(b.owner.approvals, a)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := productionReferenceBundle(t)
			tc.mutate(b)
			requireProductionQualificationRefusal(t, b, tc.name, "INCOMPLETE")
			if b.resolver.calls != 0 {
				t.Fatal("AUTHORIZATION_BEFORE_RESOLVER A-fail")
			}
		})
	}
}

func TestProductionQualificationEvidenceAndCriteria(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutate    func(*controlledQualificationBundle)
		authorize bool
		outcome   string
	}{
		{"PLAN_CRITERIA_BINDING", func(b *controlledQualificationBundle) { b.plan.Criteria = nil }, true, "INCOMPLETE"},
		{"MISSING_RECEIPT_CRITERIA", func(b *controlledQualificationBundle) { b.receipt.doc.Criteria = nil; b.plan.Criteria = nil }, true, "INCOMPLETE"},
		{"UNSUPPORTED_CRITERION", func(b *controlledQualificationBundle) { b.receipt.doc.Criteria[0].Comparator = "LE" }, true, "INCOMPLETE"},
		{"MISSING_METRIC", func(b *controlledQualificationBundle) { b.receipt.doc.Metrics = b.receipt.doc.Metrics[1:] }, true, "INCOMPLETE"},
		{"MISSING_REQUIRED_ARM", func(b *controlledQualificationBundle) {
			c := b.receipt.doc.Cases[0]
			b.receipt.doc.Cases = b.receipt.doc.Cases[1:]
			b.receipt.doc.Accounting.Candidates -= c.Candidates
			b.receipt.doc.Accounting.Evaluated -= c.Evaluated
			b.receipt.doc.Accounting.Passed -= c.Passed
			b.receipt.doc.Accounting.Rejected -= c.Rejected
			b.receipt.doc.Accounting.Excluded -= c.Excluded
			b.receipt.doc.Accounting.Failed -= c.Failed
			b.report.Accounting = b.receipt.doc.Accounting
		}, true, "INCOMPLETE"},
		{"INCOMPLETE_REQUIRED_ARM", func(b *controlledQualificationBundle) {
			b.receipt.doc.Cases[0].Completed = false
			b.report.Cases[0].Case.Completed = false
		}, true, "INCOMPLETE"},
		{"RECEIPT_REPORT_MISMATCH", func(b *controlledQualificationBundle) { b.report.Cases[0].Case.Provider = "foreign" }, true, "INCOMPLETE"},
		{"PLAN_PARTITION_MISMATCH", func(b *controlledQualificationBundle) { b.plan.Cases[0].Partition = "HELD_OUT" }, true, "INCOMPLETE"},
		{"EXPECTED_GROUPING_MISMATCH", func(b *controlledQualificationBundle) { b.report.Cases[0].ExpectedGrouping = []string{"foreign"} }, true, "INCOMPLETE"},
		{"FOREIGN_CORPUS_RESEALED", func(b *controlledQualificationBundle) { b.manifest.Cases[0].Corpus = "foreign" }, false, "INCOMPLETE"},
		{"FOREIGN_REPORT_RESEALED", func(b *controlledQualificationBundle) { b.report.Uncertainty = "foreign report" }, false, "INCOMPLETE"},
		{"MANIFEST_PLAN_MISMATCH", func(b *controlledQualificationBundle) { b.manifest.Cases[0].Provider = "foreign" }, true, "INCOMPLETE"},
		{"REPORT_SUMMARY_MISMATCH", func(b *controlledQualificationBundle) { b.report.Accounting.Candidates++ }, true, "INCOMPLETE"},
		{"METRIC_DERIVATION_MISMATCH", func(b *controlledQualificationBundle) {
			b.report.Observations[0].Numerator = 1 - b.report.Observations[0].Numerator
		}, true, "INCOMPLETE"},
		{"METRIC_CASE_MISSING", func(b *controlledQualificationBundle) { b.report.Observations = b.report.Observations[1:] }, true, "INCOMPLETE"},
		{"METRIC_CASE_DUPLICATE", func(b *controlledQualificationBundle) {
			b.report.Observations = append(b.report.Observations, b.report.Observations[0])
		}, true, "INCOMPLETE"},
		{"METRIC_MODE_COVERAGE", func(b *controlledQualificationBundle) {
			for j, d := range b.plan.MetricDefinitions {
				if len(d.CaseIDs) > 1 {
					b.plan.MetricDefinitions[j].CaseIDs = d.CaseIDs[:1]
					return
				}
			}
			t.Fatal("multi-case metric fixture missing")
		}, true, "INCOMPLETE"},
		{"METRIC_SUM_OVERFLOW", func(b *controlledQualificationBundle) {
			for j := range b.report.Observations {
				b.report.Observations[j].Denominator = math.MaxInt
			}
		}, true, "INCOMPLETE"},
		{"CRITERIA_FROZEN_BEFORE_EXECUTION", func(b *controlledQualificationBundle) { b.report.ExecutionUnix = b.plan.FrozenUnix }, true, "INCOMPLETE"},
		{"DECISION_AFTER_EXECUTION", func(b *controlledQualificationBundle) { b.report.ExecutionUnix = b.receipt.doc.DecisionUnix + 1 }, true, "INCOMPLETE"},
		{"COMPLETED_FAILED_CRITERION", func(b *controlledQualificationBundle) {
			for j, m := range b.receipt.doc.Metrics {
				if m.Code == "FALSE_MERGE_RATE" {
					b.receipt.doc.Metrics[j].Numerator = 1
					b.report.Metrics[j].Numerator = 1
				}
			}
			for j, o := range b.report.Observations {
				if o.MetricCode == "FALSE_MERGE_RATE" {
					b.report.Observations[j].Numerator = 1
				}
			}
		}, true, "REJECTED"},
		{"EXACT_MAINTAINER_DECLINE", func(b *controlledQualificationBundle) { b.receipt.doc.Reviewers[0].Decision = "DECLINE" }, true, "REJECTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := productionReferenceBundle(t)
			tc.mutate(b)
			b.seal(t, tc.authorize)
			requireProductionQualificationRefusal(t, b, tc.name, tc.outcome)
		})
	}
}

func TestProductionQualificationExactArtifactBytes(t *testing.T) {
	b := productionReferenceBundle(t)
	// A valid canonical identity artifact with equal byte length, but not the
	// independently authorized bytes. No other format/semantic error masks this.
	b.resolver.artifacts["implementation"] = []byte(strings.Replace(string(b.resolver.artifacts["implementation"]), "controlled-code-verification", "substitute-code-verification", 1))
	requireProductionQualificationRefusal(t, b, "EXACT_ARTIFACT_COMMITMENT", "INCOMPLETE")
}

func TestProductionQualificationBoundsBeforeResolver(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*controlledQualificationBundle)
	}{
		{"TOTAL_BUDGET_BEFORE_RESOLVER", func(b *controlledQualificationBundle) {
			d := b.receipt.doc
			b.owner.limits.MaxTotalBytes = len(b.receipt.canonical) + b.policy.ByteLength() + b.input.ByteLength() + d.EvaluationPlan.ByteLength + d.CorpusManifest.ByteLength + d.Implementation.ByteLength + d.Environment.ByteLength + d.Report.ByteLength - 1
		}},
		{"ARTIFACT_LENGTH_BEFORE_RESOLVER", func(b *controlledQualificationBundle) { b.receipt.doc.Report.ByteLength = math.MaxInt }},
		{"RECEIPT_LENGTH_BEFORE_PARSE", func(b *controlledQualificationBundle) {
			b.owner.limits.Receipt.MaxRawBytes = len(b.receipt.canonical) - 1
		}},
		{"ZERO_LIMIT_BEFORE_RESOLVER", func(b *controlledQualificationBundle) { b.owner.limits.MaxArtifactBytes = 0 }},
		{"CHECKED_ACCOUNTING_OVERFLOW", func(b *controlledQualificationBundle) {
			b.receipt.doc.Cases[0].Passed = math.MaxInt
			b.receipt.doc.Cases[0].Rejected = 1
		}},
		{"TOKEN_BOUND_BEFORE_PARSE", func(b *controlledQualificationBundle) { b.owner.limits.MaxTokens = 1 }},
		{"DEPTH_BOUND_BEFORE_PARSE", func(b *controlledQualificationBundle) { b.owner.limits.MaxDepth = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := productionReferenceBundle(t)
			tc.mutate(b)
			requireProductionQualificationRefusal(t, b, tc.name, "INCOMPLETE")
			if b.resolver.calls != 0 {
				t.Fatalf("%s A-fail: resolver calls=%d", tc.name, b.resolver.calls)
			}
		})
	}
}

type productionRecordBoundRow struct{ A, B, C int }
type productionRecordBoundFixture struct {
	Rows   []productionRecordBoundRow
	Nested [][]int
	IDs    []string
}

// Helper-level proof isolates structural preflight from receipt authorization,
// required-case inventories and token/byte/depth limits.
func TestProductionQualificationRecordBoundBeforeDecode(t *testing.T) {
	limits := productionTestLimits()
	limits.MaxRecords = 2
	raw := productionTestBytes(t, productionRecordBoundFixture{Rows: []productionRecordBoundRow{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}})
	var destination productionRecordBoundFixture
	err := parseProductionQualificationArtifact(raw, &destination, limits)
	var failure *qualificationFailure
	if !errors.As(err, &failure) || failure.Reason != "record bound" || !reflect.DeepEqual(destination, productionRecordBoundFixture{}) {
		t.Fatalf("RECORD_BOUND_BEFORE_TYPED_DECODE A-fail: error=%v destination=%+v", err, destination)
	}
	t.Log("RECORD_BOUND_BEFORE_TYPED_DECODE A-pass")
}

func TestProductionQualificationRecordBoundCollectionEdges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value productionRecordBoundFixture
		over  bool
	}{
		{"exact-N-objects-not-property-keys", productionRecordBoundFixture{Rows: []productionRecordBoundRow{{1, 2, 3}, {4, 5, 6}}}, false},
		{"nested-exact-N-per-array-not-aggregate", productionRecordBoundFixture{Nested: [][]int{{1, 2}, {3, 4}}}, false},
		{"nested-inner-N-plus-one", productionRecordBoundFixture{Nested: [][]int{{1, 2, 3}}}, true},
		{"nested-outer-N-plus-one", productionRecordBoundFixture{Nested: [][]int{{}, {}, {}}}, true},
		{"scalar-N-plus-one", productionRecordBoundFixture{IDs: []string{"a", "b", "c"}}, true},
		{"empty-collections", productionRecordBoundFixture{Rows: []productionRecordBoundRow{}, Nested: [][]int{}, IDs: []string{}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limits := productionTestLimits()
			limits.MaxRecords = 2
			raw := productionTestBytes(t, tc.value)
			var destination productionRecordBoundFixture
			err := parseProductionQualificationArtifact(raw, &destination, limits)
			if tc.over {
				var failure *qualificationFailure
				if !errors.As(err, &failure) || failure.Reason != "record bound" || !reflect.DeepEqual(destination, productionRecordBoundFixture{}) {
					t.Fatalf("RECORD_COLLECTION_EDGES A-fail: error=%v destination=%+v", err, destination)
				}
			} else if err != nil || !reflect.DeepEqual(destination, tc.value) {
				t.Fatalf("RECORD_COLLECTION_EDGES A-fail: error=%v destination=%+v", err, destination)
			}
			t.Log("RECORD_COLLECTION_EDGES A-pass")
		})
	}
}

func TestProductionQualificationReportRecordBoundEndToEnd(t *testing.T) {
	b := productionReferenceBundle(t)
	// Receipt metrics/criteria (16), cases (14), and all other collections fit.
	// Only the report observation collection exceeds this limit.
	b.owner.limits.MaxRecords = 32
	q, err := b.owner.Verify(b.receipt, b.policy, b.input, 108)
	var failure *qualificationFailure
	if !errors.As(err, &failure) || failure.Reason != "record bound" || q != (VerifiedQualification{}) || b.resolver.calls != 5 {
		t.Fatalf("REPORT_RECORD_BOUND_OWNER_PATH A-fail: q=%+v error=%v calls=%d", q, err, b.resolver.calls)
	}
	t.Log("REPORT_RECORD_BOUND_OWNER_PATH A-pass")
}

func TestProductionQualificationSerializedAccountingOverflow(t *testing.T) {
	b := productionReferenceBundle(t)
	n := math.MaxInt / 2
	var wrapped qualificationAccounting
	for j := range b.receipt.doc.Cases {
		c := &b.receipt.doc.Cases[j]
		c.Candidates, c.Evaluated, c.Passed, c.Rejected, c.Excluded, c.Failed = n, n, n, 0, 0, 0
		b.report.Cases[j].Case = *c
		// Deliberately model the frozen helper's wrapping arithmetic in the
		// counterexample, not in the new verifier.
		wrapped.Candidates += n
		wrapped.Evaluated += n
		wrapped.Passed += n
	}
	b.receipt.doc.Accounting = wrapped
	b.report.Accounting = wrapped
	b.seal(t, true)
	if b.receipt.Outcome() != "QUALIFIED" {
		t.Fatal("serialized wrapping counterexample precondition")
	}
	requireProductionQualificationRefusal(t, b, "SERIALIZED_ACCOUNTING_OVERFLOW", "INCOMPLETE")
	if b.resolver.calls != 0 {
		t.Fatal("SERIALIZED_ACCOUNTING_OVERFLOW A-fail: resolver invoked")
	}
}

func TestProductionQualificationCurrentValidityAndCopies(t *testing.T) {
	b := productionReferenceBundle(t)
	for _, epoch := range []uint64{0, 106, 110, math.MaxUint64} {
		q, e := b.owner.Verify(b.receipt, b.policy, b.input, epoch)
		if e == nil || q != (VerifiedQualification{}) {
			t.Fatal("CURRENT_VALIDITY A-fail")
		}
	}
	for _, epoch := range []uint64{107, 108, 109} {
		q, e := b.owner.Verify(b.receipt, b.policy, b.input, epoch)
		if e != nil || !q.Matches(b.policy, b.input, epoch) {
			t.Fatalf("VALID_RANGE_EDGES A-fail: %v", e)
		}
	}
	q, e := b.owner.Verify(b.receipt, b.policy, b.input, 108)
	if e != nil {
		t.Fatal(e)
	}
	bytes := b.receipt.CanonicalBytes()
	bytes[0] ^= 1
	inBytes := b.input.CanonicalBytes()
	inBytes[0] ^= 1
	if !q.Matches(b.policy, b.input, 108) || q.Matches(b.policy, b.input, 110) {
		t.Fatal("DEFENSIVE_OPAQUE_MATCHES A-fail")
	}
	other := controlledProductionQualification(t, []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition})
	if q.Matches(other.policy, other.input, 108) {
		t.Fatal("WRONG_INPUT_MATCH A-fail")
	}
	b.input = other.input
	requireProductionQualificationRefusal(t, b, "WRONG_INPUT", "INCOMPLETE")
	if len(issuedQualificationApprovals()) != 0 {
		t.Fatal("PRODUCTION_AUTHORIZATIONS_UNISSUED A-fail")
	}
	t.Log("CURRENT_VALIDITY_AND_DEFENSIVE_COPIES A-pass")
}

func TestProductionQualificationWrongPolicyAndLegacySeams(t *testing.T) {
	b := productionReferenceBundle(t)
	b.policy = stage5Policy(t, map[string]GroupingPolicyRule{"REFERENCES_SYMBOL": {Action: "EXCLUDE", RationaleCode: "CONTROLLED_EXCLUSION", Weight: Rational{0, 1}}})
	requireProductionQualificationRefusal(t, b, "WRONG_POLICY", "INCOMPLETE")
	b = productionReferenceBundle(t)
	if q, e := VerifyGroupingPolicyQualification(b.receipt, b.policy, b.input, 108); e == nil || q != (VerifiedQualification{}) || e.Error() != "independent qualification evidence required" {
		t.Fatal("LEGACY_NO_EVIDENCE_API_UNCHANGED A-fail")
	}
	if q, e := verifyQualificationEvidence(b.receipt, b.resolver.artifacts, nil, b.policy, b.input, 108, false); e == nil || q != (VerifiedQualification{}) || e.Error() != "production evidence verifier unavailable" {
		t.Fatal("LEGACY_STRICT_FALSE_UNCHANGED A-fail")
	}
	t.Log("WRONG_POLICY_AND_LEGACY_SEAMS A-pass")
}

func TestProductionQualificationStrictArtifactParser(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]byte) []byte
	}{
		{"DUPLICATE_ARTIFACT_FIELD", func(raw []byte) []byte {
			return []byte(strings.Replace(string(raw), `{"Family":`, `{"Family":"GROUPING_QUALIFICATION_REPORT","Family":`, 1))
		}},
		{"UNKNOWN_ARTIFACT_FIELD", func(raw []byte) []byte {
			return []byte(strings.Replace(string(raw), `{"Family":`, `{"unknown":0,"Family":`, 1))
		}},
		{"ARTIFACT_NO_LF", func(raw []byte) []byte { return raw[:len(raw)-1] }},
		{"ARTIFACT_EXTRA_LF", func(raw []byte) []byte { return append(raw, '\n') }},
		{"ARTIFACT_MISSING_BYTES", func(raw []byte) []byte { return nil }},
		{"TEST_ONLY_REPORT_RESEALED", func(raw []byte) []byte { return []byte(`{"family":"TEST_ONLY_QUALIFICATION_REPORT"}` + "\n") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := productionReferenceBundle(t)
			raw := tc.change(append([]byte(nil), b.resolver.artifacts["report"]...))
			b.resolver.artifacts["report"] = raw
			// Reseal the assertion and exact independent whitebox decision, so strict
			// interpretation (not a stale authorization) must detect malformed evidence.
			d := b.receipt.doc
			d.Report = productionTestCommit("report", raw)
			r, e := ParseGroupingPolicyQualificationReceipt(productionTestBytes(t, d), productionTestLimits().Receipt)
			if e != nil {
				t.Fatal(e)
			}
			b.receipt = r
			b.owner.approvals[0].Receipt = productionTestCommit("receipt", r.canonical)
			b.owner.approvals[0].Evidence[4] = d.Report
			requireProductionQualificationRefusal(t, b, tc.name, "INCOMPLETE")
		})
	}
}

func TestProductionQualificationTestOnlyEvidenceRefused(t *testing.T) {
	b := productionReferenceBundle(t)
	raw := independentTestQualificationEvidence()
	for role, value := range raw {
		t.Run(role, func(t *testing.T) {
			c := productionReferenceBundle(t)
			c.resolver.artifacts[role] = value
			requireProductionQualificationRefusal(t, c, "TEST_ONLY_EVIDENCE_REFUSED", "INCOMPLETE")
		})
	}
	b.resolver.artifacts["implementation"] = productionTestBytes(t, map[string]string{"KeyCommitment": "sha256:" + strings.Repeat("a", 64)})
	b.receipt.doc.Implementation = productionTestCommit("implementation", b.resolver.artifacts["implementation"])
	b.plan.Implementation = b.receipt.doc.Implementation
	b.report.Implementation = b.receipt.doc.Implementation
	b.seal(t, true)
	requireProductionQualificationRefusal(t, b, "TEST_ONLY_AAAA_ANCHOR_REFUSED", "INCOMPLETE")
}
