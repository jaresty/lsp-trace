package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func testOnlyGroupingPolicyRequest() GroupingPolicyRequest {
	return GroupingPolicyRequest{
		PolicyID:      "TEST_ONLY_GROUPING_POLICY_V1",
		Epoch:         7,
		EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"},
		Rules: []GroupingPolicyRule{
			{RelationKind: "CALLS", Action: "INCLUDE", RationaleCode: "TEST_ONLY_DIRECT_CALL", Weight: Rational{Numerator: 1, Denominator: 1}},
			{RelationKind: "REFERENCES_SYMBOL", Action: "CAP", RationaleCode: "TEST_ONLY_REFERENCE_HUB", Weight: Rational{Numerator: 1, Denominator: 2}, Cap: 32},
			{RelationKind: "RESOLVES_TO_DEFINITION", Action: "SEPARATE_LAYER", RationaleCode: "TEST_ONLY_DEFINITION_LAYER", Weight: Rational{Numerator: -2, Denominator: 4}, Layer: "TEST_ONLY_DEFINITIONS"},
		},
		Limits: GroupingPolicyLimits{MaxRules: 8, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1 << 20, MaxDenominator: 1 << 20, MaxCap: 32},
	}
}

func generousGroupingPolicyParseLimits() GroupingPolicyParseLimits {
	return GroupingPolicyParseLimits{MaxRawBytes: 1 << 16, MaxRules: 8, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256}
}

func mustParseExternalTestGroupingPolicy(t *testing.T) GroupingPolicy {
	t.Helper()
	artifact, err := BuildGroupingPolicy(testOnlyGroupingPolicyRequest())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := ParseGroupingPolicy(artifact.CanonicalBytes(), generousGroupingPolicyParseLimits())
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func canonicalJSONContainsFloat(t *testing.T, raw []byte) bool {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("BLOCKED_NOT_RED decode canonical policy: %v", err)
	}
	var containsFloat func(any) bool
	containsFloat = func(value any) bool {
		switch typed := value.(type) {
		case json.Number:
			return strings.ContainsAny(string(typed), ".eE")
		case []any:
			for _, item := range typed {
				if containsFloat(item) {
					return true
				}
			}
		case map[string]any:
			for _, item := range typed {
				if containsFloat(item) {
					return true
				}
			}
		}
		return false
	}
	return containsFloat(value)
}

func TestGroupingPolicyCanonicalRoundTripIdentityAndDefensiveCopies(t *testing.T) {
	artifact, err := BuildGroupingPolicy(testOnlyGroupingPolicyRequest())
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || bytes.Contains(raw, []byte{'\r'}) {
		t.Fatalf("ASSERT_GROUPING_POLICY_STRICT_LF bytes=%q", raw)
	}
	if canonicalJSONContainsFloat(t, raw) {
		t.Fatalf("ASSERT_GROUPING_POLICY_NO_FLOATS bytes=%s", raw)
	}
	sum := sha256.Sum256(raw)
	want := fmt.Sprintf("sha256:%x", sum)
	if artifact.ID() != want || artifact.Digest() != want || artifact.ByteLength() != len(raw) || artifact.DomainDigest() == "" || artifact.DomainDigest() == strings.Repeat("0", len(artifact.DomainDigest())) {
		t.Fatalf("ASSERT_GROUPING_POLICY_EXACT_ID_DOMAIN id=%q digest=%q len=%d domain=%q", artifact.ID(), artifact.Digest(), artifact.ByteLength(), artifact.DomainDigest())
	}
	parsed, err := ParseGroupingPolicy(raw, generousGroupingPolicyParseLimits())
	if err != nil || !bytes.Equal(parsed.CanonicalBytes(), raw) || parsed.ID() != artifact.ID() {
		t.Fatalf("ASSERT_GROUPING_POLICY_ROUNDTRIP parsed=%+v err=%v", parsed, err)
	}
	first, second := artifact.CanonicalBytes(), artifact.CanonicalBytes()
	first[0] ^= 0xff
	rules := artifact.Rules()
	rules[0].Action = "EXCLUDE"
	kinds := artifact.EligibleKinds()
	kinds[0] = "MUTATED"
	if bytes.Equal(first, second) || !bytes.Equal(second, artifact.CanonicalBytes()) || reflect.DeepEqual(rules, artifact.Rules()) || reflect.DeepEqual(kinds, artifact.EligibleKinds()) {
		t.Fatal("ASSERT_GROUPING_POLICY_DEFENSIVE_COPIES")
	}
}

func TestGroupingPolicyClosedVocabularyCanonicalOrderAndRationals(t *testing.T) {
	base := testOnlyGroupingPolicyRequest()
	for name, mutate := range map[string]func(*GroupingPolicyRequest){
		"unknown-kind":         func(r *GroupingPolicyRequest) { r.Rules[0].RelationKind = "UNKNOWN" },
		"unknown-action":       func(r *GroupingPolicyRequest) { r.Rules[0].Action = "MERGE" },
		"unknown-rationale":    func(r *GroupingPolicyRequest) { r.Rules[0].RationaleCode = "UNREGISTERED" },
		"duplicate-kind":       func(r *GroupingPolicyRequest) { r.EligibleKinds[1] = r.EligibleKinds[0] },
		"ineligible-rule":      func(r *GroupingPolicyRequest) { r.EligibleKinds = []string{"CALLS"} },
		"zero-denominator":     func(r *GroupingPolicyRequest) { r.Rules[0].Weight.Denominator = 0 },
		"negative-denominator": func(r *GroupingPolicyRequest) { r.Rules[0].Weight.Denominator = -1 },
		"overflow-numerator":   func(r *GroupingPolicyRequest) { r.Rules[0].Weight.Numerator = math.MinInt64 },
		"overflow-denominator": func(r *GroupingPolicyRequest) { r.Rules[0].Weight.Denominator = math.MaxInt64 },
		"zero-id":              func(r *GroupingPolicyRequest) { r.PolicyID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			req := base
			req.EligibleKinds = append([]string(nil), base.EligibleKinds...)
			req.Rules = append([]GroupingPolicyRule(nil), base.Rules...)
			mutate(&req)
			if got, err := BuildGroupingPolicy(req); err == nil || !reflect.DeepEqual(got, GroupingPolicy{}) {
				t.Fatalf("ASSERT_GROUPING_POLICY_REJECT_%s got=%+v err=%v", name, got, err)
			}
		})
	}
	artifact, err := BuildGroupingPolicy(base)
	if err != nil {
		t.Fatal(err)
	}
	got := artifact.Rules()
	if got[2].Weight != (Rational{Numerator: -1, Denominator: 2}) {
		t.Fatalf("ASSERT_GROUPING_POLICY_REDUCED_RATIONAL got=%+v", got[2].Weight)
	}
	permuted := base
	permuted.Rules = []GroupingPolicyRule{base.Rules[2], base.Rules[0], base.Rules[1]}
	permuted.EligibleKinds = []string{base.EligibleKinds[2], base.EligibleKinds[0], base.EligibleKinds[1]}
	again, err := BuildGroupingPolicy(permuted)
	if err != nil || !bytes.Equal(artifact.CanonicalBytes(), again.CanonicalBytes()) {
		t.Fatalf("ASSERT_GROUPING_POLICY_CANONICAL_ARRAY_ORDER err=%v", err)
	}
}

func TestGroupingPolicyActionsAreExplicitAndNonOverlapping(t *testing.T) {
	artifact, err := BuildGroupingPolicy(testOnlyGroupingPolicyRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range artifact.Rules() {
		switch rule.Action {
		case "INCLUDE":
			if rule.Cap != 0 || rule.Normalization != "" || rule.Layer != "" {
				t.Fatalf("ASSERT_INCLUDE_EXCLUSIVE %+v", rule)
			}
		case "EXCLUDE":
			if rule.Weight.Numerator != 0 {
				t.Fatalf("ASSERT_EXCLUDE_ZERO_WEIGHT %+v", rule)
			}
		case "CAP":
			if rule.Cap <= 0 || rule.Normalization != "" || rule.Layer != "" {
				t.Fatalf("ASSERT_CAP_EXPLICIT %+v", rule)
			}
		case "NORMALIZE":
			if rule.Normalization == "" || rule.Cap != 0 || rule.Layer != "" {
				t.Fatalf("ASSERT_NORMALIZE_EXPLICIT %+v", rule)
			}
		case "DOWNWEIGHT":
			if rule.Weight.Denominator <= 0 || rule.Weight.Numerator == 0 {
				t.Fatalf("ASSERT_DOWNWEIGHT_EXPLICIT %+v", rule)
			}
		case "SEPARATE_LAYER":
			if rule.Layer == "" || rule.Cap != 0 || rule.Normalization != "" {
				t.Fatalf("ASSERT_SEPARATE_LAYER_EXPLICIT %+v", rule)
			}
		default:
			t.Fatalf("ASSERT_CLOSED_POLICY_ACTION action=%q", rule.Action)
		}
	}
}

func TestGroupingPolicyRequiresExplicitPositiveMaxCapIndependentOfRules(t *testing.T) {
	base := testOnlyGroupingPolicyRequest()
	for _, tc := range []struct {
		name   string
		maxCap int
		noCAP  bool
	}{
		{name: "omitted-with-cap-rule", maxCap: 0},
		{name: "zero-without-cap-rule", maxCap: 0, noCAP: true},
		{name: "negative-with-cap-rule", maxCap: -1},
		{name: "negative-without-cap-rule", maxCap: -1, noCAP: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.EligibleKinds = append([]string(nil), base.EligibleKinds...)
			req.Rules = append([]GroupingPolicyRule(nil), base.Rules...)
			req.Limits.MaxCap = tc.maxCap
			if tc.noCAP {
				req.Rules[1] = GroupingPolicyRule{RelationKind: "REFERENCES_SYMBOL", Action: "INCLUDE", RationaleCode: "TEST_ONLY_REFERENCE_HUB", Weight: Rational{Numerator: 1, Denominator: 1}}
			}
			if got, err := BuildGroupingPolicy(req); err == nil || !reflect.DeepEqual(got, GroupingPolicy{}) {
				t.Fatalf("ASSERT_GROUPING_POLICY_EXPLICIT_POSITIVE_MAX_CAP_REQUIRED max_cap=%d no_cap_rule=%t got=%+v err=%v", tc.maxCap, tc.noCAP, got, err)
			}
		})
	}
}

func TestGroupingPolicyRequestedCAPCannotDefineOrRaisePolicyBound(t *testing.T) {
	base := testOnlyGroupingPolicyRequest()
	for _, tc := range []struct {
		name         string
		policyMaxCap int
		requestedCAP int
	}{
		{name: "cannot-define-omitted-bound", policyMaxCap: 0, requestedCAP: 32},
		{name: "cannot-raise-explicit-bound", policyMaxCap: 16, requestedCAP: 32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.Rules = append([]GroupingPolicyRule(nil), base.Rules...)
			req.Limits.MaxCap = tc.policyMaxCap
			req.Rules[1].Cap = tc.requestedCAP
			if got, err := BuildGroupingPolicy(req); err == nil || !reflect.DeepEqual(got, GroupingPolicy{}) {
				t.Fatalf("ASSERT_REQUESTED_CAP_CANNOT_DEFINE_OR_RAISE_BOUND policy_max_cap=%d requested_cap=%d got=%+v err=%v", tc.policyMaxCap, tc.requestedCAP, got, err)
			}
		})
	}
}

func TestGroupingPolicyRulesEnforceExclusiveFieldsAndExplicitBounds(t *testing.T) {
	limitsType := reflect.TypeOf(GroupingPolicyLimits{})
	maxCapField, ok := limitsType.FieldByName("MaxCap")
	if !ok || maxCapField.Type.Kind() != reflect.Int {
		t.Fatal("ASSERT_GROUPING_POLICY_LIMITS_DECLARE_INTEGER_MAX_CAP")
	}
	base := testOnlyGroupingPolicyRequest()
	type fieldMutation struct {
		name string
		set  func(*GroupingPolicyRule)
	}
	irrelevant := []fieldMutation{
		{"weight", func(r *GroupingPolicyRule) { r.Weight = Rational{Numerator: 1, Denominator: 2} }},
		{"cap", func(r *GroupingPolicyRule) { r.Cap = 1 }},
		{"normalization", func(r *GroupingPolicyRule) { r.Normalization = "TEST_ONLY_UNIT" }},
		{"layer", func(r *GroupingPolicyRule) { r.Layer = "TEST_ONLY_LAYER" }},
	}
	valid := map[string]GroupingPolicyRule{
		"INCLUDE":        {Action: "INCLUDE", Weight: Rational{1, 1}},
		"EXCLUDE":        {Action: "EXCLUDE", Weight: Rational{0, 1}},
		"DOWNWEIGHT":     {Action: "DOWNWEIGHT", Weight: Rational{1, 2}},
		"CAP":            {Action: "CAP", Weight: Rational{1, 1}, Cap: 32},
		"NORMALIZE":      {Action: "NORMALIZE", Weight: Rational{1, 1}, Normalization: "TEST_ONLY_UNIT"},
		"SEPARATE_LAYER": {Action: "SEPARATE_LAYER", Weight: Rational{1, 1}, Layer: "TEST_ONLY_LAYER"},
	}
	allowedField := map[string]string{"DOWNWEIGHT": "weight", "CAP": "cap", "NORMALIZE": "normalization", "SEPARATE_LAYER": "layer"}
	for _, action := range []string{"INCLUDE", "EXCLUDE", "DOWNWEIGHT", "CAP", "NORMALIZE", "SEPARATE_LAYER"} {
		t.Run(action+"-valid", func(t *testing.T) {
			assertPolicyRuleBuild(t, base, valid[action], true)
		})
		for _, mutation := range irrelevant {
			if allowedField[action] == mutation.name {
				continue
			}
			t.Run(action+"-rejects-irrelevant-"+mutation.name, func(t *testing.T) {
				rule := valid[action]
				mutation.set(&rule)
				assertPolicyRuleBuild(t, base, rule, false)
			})
		}
	}
	for _, tc := range []struct {
		name   string
		weight Rational
		valid  bool
	}{
		{"positive-fraction", Rational{1, 2}, true},
		{"positive-one", Rational{1, 1}, true},
		{"zero", Rational{0, 1}, false},
		{"negative", Rational{-1, 2}, false},
		{"over-one", Rational{2, 1}, false},
	} {
		t.Run("DOWNWEIGHT-"+tc.name, func(t *testing.T) {
			rule := valid["DOWNWEIGHT"]
			rule.Weight = tc.weight
			assertPolicyRuleBuild(t, base, rule, tc.valid)
		})
	}
	for _, tc := range []struct {
		name  string
		cap   int
		valid bool
	}{{"zero", 0, false}, {"at-max", 32, true}, {"over-max", 33, false}} {
		t.Run("CAP-"+tc.name, func(t *testing.T) {
			req := base
			limits := reflect.ValueOf(&req.Limits).Elem().FieldByName("MaxCap")
			limits.SetInt(32)
			rule := valid["CAP"]
			rule.Cap = tc.cap
			assertPolicyRuleBuild(t, req, rule, tc.valid)
		})
	}
}

func assertPolicyRuleBuild(t *testing.T, base GroupingPolicyRequest, rule GroupingPolicyRule, valid bool) {
	t.Helper()
	req := base
	req.EligibleKinds = append([]string(nil), base.EligibleKinds...)
	req.Rules = append([]GroupingPolicyRule(nil), base.Rules...)
	rule.RelationKind, rule.RationaleCode = "CALLS", "TEST_ONLY_DIRECT_CALL"
	req.Rules[0] = rule
	got, err := BuildGroupingPolicy(req)
	if valid && err != nil {
		t.Fatalf("ASSERT_POLICY_ACTION_VALID action=%s rule=%+v err=%v", rule.Action, rule, err)
	}
	if !valid && (err == nil || !reflect.DeepEqual(got, GroupingPolicy{})) {
		t.Fatalf("ASSERT_POLICY_ACTION_FORBIDDEN_FIELD_REJECTED action=%s rule=%+v got=%+v err=%v", rule.Action, rule, got, err)
	}
}

func TestGroupingPolicyStrictParserMutationCorpus(t *testing.T) {
	artifact, err := BuildGroupingPolicy(testOnlyGroupingPolicyRequest())
	if err != nil {
		t.Fatal(err)
	}
	raw := artifact.CanonicalBytes()
	mutations := map[string][]byte{
		"unknown":   bytes.Replace(raw, []byte(`{"family":`), []byte(`{"unknown":true,"family":`), 1),
		"duplicate": bytes.Replace(raw, []byte(`{"family":`), []byte(`{"family":"GROUPING_POLICY","family":`), 1),
		"no-lf":     append([]byte(nil), raw[:len(raw)-1]...),
		"extra-lf":  append(append([]byte(nil), raw...), '\n'),
		"crlf":      append(append([]byte(nil), raw[:len(raw)-1]...), '\r', '\n'),
		"float":     bytes.Replace(raw, []byte(`"denominator":1`), []byte(`"denominator":1.0`), 1),
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if got, err := ParseGroupingPolicy(mutated, generousGroupingPolicyParseLimits()); err == nil || !reflect.DeepEqual(got, GroupingPolicy{}) {
				t.Fatalf("ASSERT_GROUPING_POLICY_STRICT_%s got=%+v err=%v", name, got, err)
			}
		})
	}
	tiny := generousGroupingPolicyParseLimits()
	tiny.MaxRawBytes = len(raw) - 1
	if got, err := ParseGroupingPolicy(raw, tiny); err == nil || !reflect.DeepEqual(got, GroupingPolicy{}) {
		t.Fatalf("ASSERT_GROUPING_POLICY_RAW_BOUND got=%+v err=%v", got, err)
	}
}
