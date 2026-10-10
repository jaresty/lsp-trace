package referencegrouping

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/programcadmission"
)

// Stage5 tests use only Stage4's structurally test-only receipt, independent
// evidence, and reviewer trust. They do not construct production qualification.
func stage5VerifiedFixture(t *testing.T, kinds []programcadmission.RelationKind, policy GroupingPolicy) (AdmittedInput, VerifiedQualification) {
	t.Helper()
	f := referenceFixture(t, kinds)
	artifact, err := BuildInput(f.request())
	if err != nil {
		t.Fatal(err)
	}
	input, err := ParseAndAdmit(artifact.CanonicalBytes(), f.predecessors, generousParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	r := completeTestQualification(policy, input)
	raw := canonicalExternalTestReceipt(t, r)
	receipt, err := parseTestOnlyQualificationReceipt(raw, generousQualificationParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifyTestOnlyQualificationWithEvidence(receipt, independentTestQualificationEvidence(), independentTestQualificationTrustAnchors(t, r), policy, input, 8)
	if err != nil {
		t.Fatal(err)
	}
	return input, verified
}

func stage5Policy(t *testing.T, actions map[string]GroupingPolicyRule) GroupingPolicy {
	t.Helper()
	return stage5PolicyWithNumericLimits(t, actions, 1<<20, 1<<20)
}

func stage5PolicyWithNumericLimits(t *testing.T, actions map[string]GroupingPolicyRule, maxNumerator, maxDenominator int) GroupingPolicy {
	t.Helper()
	rules := make([]GroupingPolicyRule, 0, 3)
	for _, kind := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
		rule, ok := actions[kind]
		if !ok {
			rule = GroupingPolicyRule{RelationKind: kind, Action: "INCLUDE", RationaleCode: "TEST_ONLY_INCLUDE", Weight: Rational{1, 1}}
		}
		rule.RelationKind = kind
		rules = append(rules, rule)
	}
	p, err := BuildGroupingPolicy(GroupingPolicyRequest{
		PolicyID: "TEST_ONLY_STAGE5_POLICY", Epoch: 8,
		EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules,
		Limits: GroupingPolicyLimits{MaxRules: 8, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: maxNumerator, MaxDenominator: maxDenominator, MaxCap: 32},
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func stage5Limits() CompositionLimits {
	return CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20}
}

func TestStage5ClosedActionsHaveExactDispositionPairAndAccountingTables(t *testing.T) {
	type expectedAccounting struct {
		Included, Excluded, Capped, Normalized, Downweighted, SeparateLayer, Duplicate, Terminal int
		Truncated                                                                                bool
	}
	type actionCase struct {
		rule              GroupingPolicyRule
		dispositions      []string
		pairSourceIndexes []int
		pairWeights       []Rational
		accounting        expectedAccounting
		wantLayer         string
		wantNormalization string
		wantCapBefore     int
		wantCapAfter      int
	}
	cases := []actionCase{
		{rule: GroupingPolicyRule{Action: "INCLUDE", RationaleCode: "TEST_ONLY_INCLUDE", Weight: Rational{1, 1}}, dispositions: []string{"INCLUDED", "INCLUDED"}, pairSourceIndexes: []int{0, 1}, pairWeights: []Rational{{1, 1}, {1, 1}}, accounting: expectedAccounting{Included: 2, Terminal: 2}},
		{rule: GroupingPolicyRule{Action: "EXCLUDE", RationaleCode: "TEST_ONLY_EXCLUDE", Weight: Rational{0, 1}}, dispositions: []string{"EXCLUDED", "EXCLUDED"}, accounting: expectedAccounting{Excluded: 2, Terminal: 2}},
		{rule: GroupingPolicyRule{Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_DOWNWEIGHT", Weight: Rational{1, 2}}, dispositions: []string{"DOWNWEIGHTED", "DOWNWEIGHTED"}, pairSourceIndexes: []int{0, 1}, pairWeights: []Rational{{1, 2}, {1, 2}}, accounting: expectedAccounting{Downweighted: 2, Terminal: 2}},
		// Multiplicity two exceeds cap one: the first canonical occurrence is CAPPED
		// and paired; the second is EXCLUDED and has no pair contribution.
		{rule: GroupingPolicyRule{Action: "CAP", RationaleCode: "TEST_ONLY_CAP", Weight: Rational{1, 1}, Cap: 1}, dispositions: []string{"CAPPED", "EXCLUDED"}, pairSourceIndexes: []int{0}, pairWeights: []Rational{{1, 1}}, accounting: expectedAccounting{Capped: 1, Excluded: 1, Terminal: 2}, wantCapBefore: 2, wantCapAfter: 1},
		{rule: GroupingPolicyRule{Action: "NORMALIZE", RationaleCode: "TEST_ONLY_NORMALIZE", Weight: Rational{1, 1}, Normalization: "BY_KIND_MULTIPLICITY"}, dispositions: []string{"NORMALIZED", "NORMALIZED"}, pairSourceIndexes: []int{0, 1}, pairWeights: []Rational{{1, 2}, {1, 2}}, accounting: expectedAccounting{Normalized: 2, Terminal: 2}, wantNormalization: "BY_KIND_MULTIPLICITY"},
		{rule: GroupingPolicyRule{Action: "SEPARATE_LAYER", RationaleCode: "TEST_ONLY_LAYER", Weight: Rational{1, 1}, Layer: "DEFINITIONS"}, dispositions: []string{"SEPARATE_LAYER", "SEPARATE_LAYER"}, accounting: expectedAccounting{SeparateLayer: 2, Terminal: 2}, wantLayer: "DEFINITIONS"},
	}
	for _, tc := range cases {
		t.Run(tc.rule.Action, func(t *testing.T) {
			policy := stage5Policy(t, map[string]GroupingPolicyRule{"REFERENCES_SYMBOL": tc.rule})
			input, verified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, policy)
			receipt, err := ComposeQualified(input, policy, verified, 8, stage5Limits())
			if err != nil {
				t.Fatal(err)
			}
			got, source := receipt.Occurrences(), input.Contributions()
			if len(got) != 2 || len(source) != 2 {
				t.Fatalf("BLOCKED_NOT_RED fixture got=%d source=%d", len(got), len(source))
			}
			for i := range source {
				wantCustody := input.CustodyIdentityForOccurrence(source[i].OccurrenceIdentity)
				wantPredecessor := input.PredecessorIdentityForContribution(source[i])
				if got[i].OccurrenceIdentity != source[i].OccurrenceIdentity || got[i].Kind != source[i].Kind || got[i].From != source[i].From || got[i].To != source[i].To || got[i].Ordinal != source[i].Ordinal || got[i].Action != tc.rule.Action || got[i].RationaleCode != tc.rule.RationaleCode || got[i].Layer != tc.wantLayer || got[i].NormalizationBasis != tc.wantNormalization || got[i].CapBefore != tc.wantCapBefore || got[i].CapAfter != tc.wantCapAfter || got[i].SourcePredecessorIdentity != wantPredecessor || got[i].SourceCustodyIdentity != wantCustody || got[i].TerminalDisposition != tc.dispositions[i] {
					t.Fatalf("ASSERT_STAGE5_EXACT_ACTION_RECORD action=%s row=%d got=%+v source=%+v predecessor=%q custody=%q", tc.rule.Action, i, got[i], source[i], wantPredecessor, wantCustody)
				}
			}
			pairs := receipt.PairContributions()
			if len(pairs) != len(tc.pairWeights) {
				t.Fatalf("ASSERT_STAGE5_EXACT_PAIR_TABLE action=%s got=%+v want=%+v", tc.rule.Action, pairs, tc.pairWeights)
			}
			for i, weight := range tc.pairWeights {
				sourceIndex := tc.pairSourceIndexes[i]
				if pairs[i].OccurrenceIdentity != source[sourceIndex].OccurrenceIdentity || pairs[i].Kind != source[sourceIndex].Kind || pairs[i].From != source[sourceIndex].From || pairs[i].To != source[sourceIndex].To || pairs[i].Weight != weight || pairs[i].RationaleCode != tc.rule.RationaleCode || pairs[i].Layer != "" || pairs[i].NormalizationBasis != tc.wantNormalization || pairs[i].CapBefore != tc.wantCapBefore || pairs[i].CapAfter != tc.wantCapAfter || pairs[i].SourcePredecessorIdentity != input.PredecessorIdentityForContribution(source[sourceIndex]) || pairs[i].SourceCustodyIdentity != input.CustodyIdentityForOccurrence(source[sourceIndex].OccurrenceIdentity) {
					t.Fatalf("ASSERT_STAGE5_EXACT_BASE_PAIR_RECORD action=%s row=%d got=%+v", tc.rule.Action, i, pairs[i])
				}
			}
			separate := receipt.SeparateLayerContributions()
			if tc.rule.Action == "SEPARATE_LAYER" {
				if len(pairs) != 0 || len(separate) != len(source) {
					t.Fatalf("ASSERT_STAGE5_SEPARATE_LAYER_ABSENT_FROM_BASE_PAIRS base=%+v separate=%+v", pairs, separate)
				}
				for i := range separate {
					if separate[i].Layer != tc.wantLayer || separate[i].RationaleCode != tc.rule.RationaleCode || separate[i].Weight != (Rational{1, 1}) || separate[i].SourcePredecessorIdentity != input.PredecessorIdentityForContribution(source[i]) || separate[i].SourceCustodyIdentity != input.CustodyIdentityForOccurrence(source[i].OccurrenceIdentity) {
						t.Fatalf("ASSERT_STAGE5_EXACT_SEPARATE_COLLECTION_ROW row=%d got=%+v", i, separate[i])
					}
				}
			} else if len(separate) != 0 {
				t.Fatalf("ASSERT_STAGE5_NON_LAYER_HAS_EMPTY_SEPARATE_COLLECTION action=%s got=%+v", tc.rule.Action, separate)
			}
			a := receipt.Accounting()
			gotAccounting := expectedAccounting{a.Included, a.Excluded, a.Capped, a.Normalized, a.Downweighted, a.SeparateLayer, a.Duplicate, a.Terminal, a.Truncated}
			if gotAccounting != tc.accounting {
				t.Fatalf("ASSERT_STAGE5_EXACT_ACCOUNTING action=%s got=%+v want=%+v", tc.rule.Action, gotAccounting, tc.accounting)
			}
		})
	}
}

func TestStage5CompositionReceiptCanonicalBindingsAccountingAndDefensiveCopies(t *testing.T) {
	policy := stage5Policy(t, map[string]GroupingPolicyRule{
		"CALLS":                  {Action: "INCLUDE", RationaleCode: "TEST_ONLY_CALLS", Weight: Rational{1, 1}},
		"REFERENCES_SYMBOL":      {Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_REFS", Weight: Rational{1, 2}},
		"RESOLVES_TO_DEFINITION": {Action: "SEPARATE_LAYER", RationaleCode: "TEST_ONLY_DEFS", Weight: Rational{1, 1}, Layer: "DEFINITIONS"},
	})
	input, verified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, policy)
	receipt, err := ComposeQualified(input, policy, verified, 8, stage5Limits())
	if err != nil {
		t.Fatal(err)
	}
	bindings := receipt.Bindings()
	if bindings.InputID != input.ID() || bindings.InputDigest != input.Digest() || bindings.InputByteLength != input.ByteLength() || bindings.PolicyID != policy.ID() || bindings.PolicyDigest != policy.Digest() || bindings.PolicyByteLength != policy.ByteLength() || bindings.QualificationReceiptCommitment.ID == "" || bindings.QualificationReceiptCommitment.Digest == "" || bindings.QualificationReceiptCommitment.ByteLength <= 0 {
		t.Fatalf("ASSERT_STAGE5_EXACT_THREE_WAY_BINDING %+v", bindings)
	}
	a := receipt.Accounting()
	if a.Included+a.Excluded+a.Capped+a.Normalized+a.Downweighted+a.SeparateLayer+a.Duplicate != a.Terminal || a.Terminal != len(input.Contributions()) || a.Truncated || a.Duplicate != 0 {
		t.Fatalf("ASSERT_STAGE5_BALANCED_ACCOUNTING %+v", a)
	}
	raw := receipt.CanonicalBytes()
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || bytes.Contains(raw, []byte{'\r'}) {
		t.Fatalf("ASSERT_STAGE5_CANONICAL_LF bytes=%q", raw)
	}
	if err := rejectStage5DecimalOrExponentNumbers(raw); err != nil {
		t.Fatalf("ASSERT_STAGE5_CANONICAL_INTEGER_NUMERIC_TOKENS: %v", err)
	}
	parsed, err := ParseCompositionReceipt(raw, stage5Limits())
	if err != nil || parsed.ID() != receipt.ID() || !bytes.Equal(parsed.CanonicalBytes(), raw) {
		t.Fatalf("ASSERT_STAGE5_RECEIPT_ROUNDTRIP err=%v", err)
	}
	first, second := receipt.CanonicalBytes(), receipt.CanonicalBytes()
	first[0] ^= 0xff
	occ := receipt.Occurrences()
	occ[0].From = "mutated"
	pairs := receipt.PairContributions()
	pairs[0].From = "mutated"
	if bytes.Equal(first, second) || !bytes.Equal(second, receipt.CanonicalBytes()) || receipt.Occurrences()[0].From == "mutated" || receipt.PairContributions()[0].From == "mutated" {
		t.Fatal("ASSERT_STAGE5_RECEIPT_DEFENSIVE_COPIES")
	}
}

func TestStage5CompositionReceiptStrictParserAndAtomicZero(t *testing.T) {
	policy := stage5Policy(t, nil)
	input, verified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, policy)
	receipt, err := ComposeQualified(input, policy, verified, 8, stage5Limits())
	if err != nil {
		t.Fatal(err)
	}
	raw := receipt.CanonicalBytes()
	mutations := map[string][]byte{
		"unknown":   bytes.Replace(raw, []byte(`{"family":`), []byte(`{"unknown":true,"family":`), 1),
		"duplicate": bytes.Replace(raw, []byte(`{"family":`), []byte(`{"family":"COMPOSITION_RECEIPT","family":`), 1),
		"no-lf":     append([]byte(nil), raw[:len(raw)-1]...), "extra-lf": append(append([]byte(nil), raw...), '\n'),
		"crlf": append(append([]byte(nil), raw[:len(raw)-1]...), '\r', '\n'), "trailing": append(append([]byte(nil), raw...), []byte("{}\n")...),
	}
	for name, bad := range mutations {
		t.Run(name, func(t *testing.T) {
			if got, err := ParseCompositionReceipt(bad, stage5Limits()); err == nil || !reflect.DeepEqual(got, CompositionReceipt{}) {
				t.Fatalf("ASSERT_STAGE5_STRICT_%s got=%+v err=%v", name, got, err)
			}
		})
	}
	for _, cut := range []func(*CompositionLimits){func(l *CompositionLimits) { l.MaxRawBytes = len(raw) - 1 }, func(l *CompositionLimits) { l.MaxOccurrences = 0 }, func(l *CompositionLimits) { l.MaxPairContributions = 0 }, func(l *CompositionLimits) { l.MaxCanonicalBytes = len(raw) - 1 }, func(l *CompositionLimits) { l.MaxStringBytes = 1 }} {
		limits := stage5Limits()
		cut(&limits)
		if got, err := ParseCompositionReceipt(raw, limits); err == nil || !reflect.DeepEqual(got, CompositionReceipt{}) {
			t.Fatalf("ASSERT_STAGE5_LIMIT_ATOMIC_ZERO got=%+v err=%v", got, err)
		}
	}
}

func TestStage5RationalPairContributionsCanonicalAndOverflowSafe(t *testing.T) {
	policy := stage5Policy(t, map[string]GroupingPolicyRule{"REFERENCES_SYMBOL": {Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_HALF", Weight: Rational{2, 4}}})
	input, verified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, policy)
	receipt, err := ComposeQualified(input, policy, verified, 8, stage5Limits())
	if err != nil {
		t.Fatal(err)
	}
	pairs := receipt.PairContributions()
	if !sort.SliceIsSorted(pairs, func(i, j int) bool { return pairs[i].CanonicalKey() < pairs[j].CanonicalKey() }) {
		t.Fatal("ASSERT_STAGE5_PAIR_CANONICAL_ORDER")
	}
	for _, pair := range pairs {
		if pair.Weight.Denominator <= 0 || pair.Weight.Numerator < 0 || gcd(pair.Weight.Numerator, pair.Weight.Denominator) != 1 || pair.Kind != "REFERENCES_SYMBOL" || pair.OccurrenceIdentity == "" {
			t.Fatalf("ASSERT_STAGE5_REDUCED_NONNEGATIVE_POLICY_RATIONAL %+v", pair)
		}
	}
	if receipt.PairWeightDigest() == "" {
		t.Fatal("ASSERT_STAGE5_PAIR_WEIGHT_DIGEST_PRESENT")
	}
	if err := rejectStage5DecimalOrExponentNumbers(receipt.CanonicalBytes()); err != nil {
		t.Fatalf("ASSERT_STAGE5_PAIR_DIGEST_INTEGER_NUMERIC_TOKENS: %v", err)
	}

	// The accepted Stage3 source rows are an independent oracle for direction,
	// occurrence multiplicity, self-loop retention, and cross-kind non-deduplication.
	mixedPolicy := stage5Policy(t, nil)
	mixedInput, mixedVerified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, mixedPolicy)
	mixed, err := ComposeQualified(mixedInput, mixedPolicy, mixedVerified, 8, stage5Limits())
	if err != nil {
		t.Fatal(err)
	}
	source, gotPairs := mixedInput.Contributions(), mixed.PairContributions()
	if len(source) != 3 || len(gotPairs) != 3 {
		t.Fatalf("ASSERT_STAGE5_EXACT_PAIR_MULTIPLICITY got=%d source=%d", len(gotPairs), len(source))
	}
	for i := range source {
		if gotPairs[i].OccurrenceIdentity != source[i].OccurrenceIdentity || gotPairs[i].Kind != source[i].Kind || gotPairs[i].From != source[i].From || gotPairs[i].To != source[i].To {
			t.Fatalf("ASSERT_STAGE5_PAIR_DIRECTION_AND_OCCURRENCE row=%d got=%+v source=%+v", i, gotPairs[i], source[i])
		}
	}
	if gotPairs[1].OccurrenceIdentity != "reference-loop-1" || gotPairs[1].From != gotPairs[1].To {
		t.Fatalf("ASSERT_STAGE5_EXACT_SELF_LOOP_ROW %+v", gotPairs[1])
	}
	if gotPairs[0].From != gotPairs[2].From || gotPairs[0].To != gotPairs[2].To || gotPairs[0].Kind == gotPairs[2].Kind || gotPairs[0].OccurrenceIdentity == gotPairs[2].OccurrenceIdentity {
		t.Fatalf("ASSERT_STAGE5_EXACT_CROSS_KIND_SAME_ENDPOINT_ROWS ref=%+v def=%+v", gotPairs[0], gotPairs[2])
	}

	t.Run("zero-and-negative-denominator-policy-admission", func(t *testing.T) {
		for _, denominator := range []int{0, -1} {
			_, err := BuildGroupingPolicy(GroupingPolicyRequest{
				PolicyID: "TEST_ONLY_STAGE5_INVALID_DENOMINATOR", Epoch: 8,
				EligibleKinds: []string{"CALLS"},
				Rules:         []GroupingPolicyRule{{RelationKind: "CALLS", Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_INVALID", Weight: Rational{1, denominator}}},
				Limits:        GroupingPolicyLimits{MaxRules: 8, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: math.MaxInt, MaxDenominator: math.MaxInt, MaxCap: 32},
			})
			if err == nil {
				t.Fatalf("ASSERT_STAGE5_INVALID_DENOMINATOR_REJECTED_AT_POLICY_ADMISSION denominator=%d", denominator)
			}
		}
	})

	t.Run("lcm-overflow-from-valid-reduced-denominators", func(t *testing.T) {
		overflowPolicy := stage5PolicyWithNumericLimits(t, map[string]GroupingPolicyRule{
			"CALLS":                  {Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_D1", Weight: Rational{1, 2147483629}},
			"REFERENCES_SYMBOL":      {Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_D2", Weight: Rational{1, 2147483587}},
			"RESOLVES_TO_DEFINITION": {Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_D3", Weight: Rational{1, 2147483579}},
		}, math.MaxInt, math.MaxInt)
		overflowInput, overflowVerified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, overflowPolicy)
		limits := stage5Limits()
		limits.MaxDenominator = math.MaxInt
		if got, err := ComposeQualified(overflowInput, overflowPolicy, overflowVerified, 8, limits); err == nil || !reflect.DeepEqual(got, CompositionReceipt{}) {
			t.Fatalf("ASSERT_STAGE5_LCM_OVERFLOW_FROM_VALID_INPUT_ATOMIC_ZERO got=%+v err=%v", got, err)
		}
	})

	t.Run("numerator-multiplicity-accumulator-overflow", func(t *testing.T) {
		overflowPolicy := stage5PolicyWithNumericLimits(t, map[string]GroupingPolicyRule{
			"REFERENCES_SYMBOL": {Action: "INCLUDE", RationaleCode: "TEST_ONLY_NEAR_MAX", Weight: Rational{math.MaxInt, 1}},
		}, math.MaxInt, math.MaxInt)
		overflowInput, overflowVerified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, overflowPolicy)
		limits := stage5Limits()
		limits.MaxAccumulator = math.MaxInt
		if got, err := ComposeQualified(overflowInput, overflowPolicy, overflowVerified, 8, limits); err == nil || !reflect.DeepEqual(got, CompositionReceipt{}) {
			t.Fatalf("ASSERT_STAGE5_NUMERATOR_MULTIPLICITY_OVERFLOW_ATOMIC_ZERO got=%+v err=%v", got, err)
		}
	})
}

func TestStage5EightModeFixturesDeterministicAccountingCustodyAndStaleness(t *testing.T) {
	fixtures := []struct {
		name                 string
		kinds                []programcadmission.RelationKind
		occurrences, custody int
	}{
		{"calls-only", []programcadmission.RelationKind{programcadmission.TypedCalls}, 2, 2},
		{"refs-only", []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, 2, 2},
		{"defs-only", []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition}, 1, 2},
		{"refs-defs", []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, 3, 2},
		{"calls-refs", []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol}, 4, 4},
		{"calls-defs", []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedResolvesToDefinition}, 3, 4},
		{"all", []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, 5, 4},
		{"empty-zero", nil, 0, 0},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			policy := stage5Policy(t, nil)
			input, verified := stage5VerifiedFixture(t, fixture.kinds, policy)
			a, err := ComposeQualified(input, policy, verified, 8, stage5Limits())
			if err != nil {
				t.Fatal(err)
			}
			b, err := ComposeQualified(input, policy, verified, 8, stage5Limits())
			if err != nil || a.ID() != b.ID() || !bytes.Equal(a.CanonicalBytes(), b.CanonicalBytes()) {
				t.Fatalf("ASSERT_STAGE5_FIXTURE_DETERMINISTIC err=%v", err)
			}
			accounting := a.Accounting()
			if len(input.Contributions()) != fixture.occurrences || len(a.Occurrences()) != fixture.occurrences || len(a.PairContributions()) != fixture.occurrences || accounting.Included != fixture.occurrences || accounting.Terminal != fixture.occurrences || accounting.Excluded != 0 || accounting.Capped != 0 || accounting.Normalized != 0 || accounting.Downweighted != 0 || accounting.SeparateLayer != 0 || accounting.Duplicate != 0 || accounting.Truncated {
				t.Fatalf("ASSERT_STAGE5_EXACT_EIGHT_MODE_ACCOUNTING mode=%s accounting=%+v occurrences=%d pairs=%d", fixture.name, accounting, len(a.Occurrences()), len(a.PairContributions()))
			}
			if len(a.Custody()) != fixture.custody || !reflect.DeepEqual(a.Custody(), input.EndpointCustody()) {
				t.Fatalf("ASSERT_STAGE5_EXACT_EIGHT_MODE_CUSTODY mode=%s got=%+v want=%+v", fixture.name, a.Custody(), input.EndpointCustody())
			}
			if got, err := ComposeQualified(input, policy, verified, 10, stage5Limits()); err == nil || !reflect.DeepEqual(got, CompositionReceipt{}) {
				t.Fatalf("ASSERT_STAGE5_STALE_QUALIFICATION_ATOMIC_ZERO got=%+v err=%v", got, err)
			}
		})
	}
}

func TestStage5CompositionJSONContainsOnlyCanonicalIntegerRationals(t *testing.T) {
	policy := stage5Policy(t, nil)
	input, verified := stage5VerifiedFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls}, policy)
	receipt, err := ComposeQualified(input, policy, verified, 8, stage5Limits())
	if err != nil {
		t.Fatal(err)
	}
	if err := rejectStage5DecimalOrExponentNumbers(receipt.CanonicalBytes()); err != nil {
		t.Fatalf("ASSERT_STAGE5_NO_DECIMAL_OR_EXPONENT_NUMERIC_TOKEN: %v", err)
	}
}

func rejectStage5DecimalOrExponentNumbers(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("multiple JSON values")
	}
	var inspect func(any) error
	inspect = func(v any) error {
		switch x := v.(type) {
		case json.Number:
			if strings.ContainsAny(string(x), ".eE") {
				return fmt.Errorf("non-integer numeric token %q", x)
			}
		case []any:
			for _, item := range x {
				if err := inspect(item); err != nil {
					return err
				}
			}
		case map[string]any:
			for _, item := range x {
				if err := inspect(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return inspect(value)
}
