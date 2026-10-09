package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type QualificationParseLimits struct{ MaxRawBytes, MaxCases, MaxMetrics, MaxCriteria, MaxReviewers, MaxStringBytes int }
type qualificationCommitment struct {
	ID         string `json:"id"`
	ByteLength int    `json:"byte_length"`
	Digest     string `json:"digest"`
}
type qualificationCase struct {
	ID, Arm, Partition, Corpus, Language, Provider, ProviderProfile, Revision string
	Required, Completed                                                       bool
	Candidates, Evaluated, Passed, Rejected, Excluded, Failed                 int
}
type qualificationAccounting struct {
	Candidates, Evaluated, Passed, Rejected, Excluded, Failed int
}
type qualificationMetric struct {
	Code        string `json:"code"`
	Numerator   int    `json:"numerator"`
	Denominator int    `json:"denominator"`
}
type qualificationCriterion struct {
	Code, MetricCode, Comparator string
	Threshold                    Rational
}
type qualificationReviewer struct{ Identity, IdentityDomain, IdentityClass, Custody, Decision string }
type qualificationDocument struct {
	Family, Version, PolicyClass, IdentityDomain string
	Policy, GroupingInput                        qualificationCommitment
	EvaluationPlan, CorpusManifest               qualificationCommitment
	Implementation, Environment, Report          qualificationCommitment
	Cases                                        []qualificationCase
	Metrics                                      []qualificationMetric
	Criteria                                     []qualificationCriterion
	Reviewers                                    []qualificationReviewer
	Accounting                                   qualificationAccounting
	RequiredReviewerClass                        string
	RequiredAcceptances                          int
	ValidFromEpoch, ValidThroughEpoch            uint64
	DecisionUnix                                 int64
}
type GroupingPolicyQualificationReceipt struct {
	canonical []byte
	doc       qualificationDocument
	outcome   string
}
type VerifiedQualification struct {
	policyID, policyDigest, inputID, inputDigest string
	policyByteLength, inputByteLength            int
	validFromEpoch, validThroughEpoch            uint64
}

func (r GroupingPolicyQualificationReceipt) CanonicalBytes() []byte {
	return append([]byte(nil), r.canonical...)
}
func (r GroupingPolicyQualificationReceipt) Outcome() string { return r.outcome }
func (v VerifiedQualification) PolicyID() string             { return v.policyID }
func (v VerifiedQualification) InputID() string              { return v.inputID }
func (v VerifiedQualification) InputDigest() string          { return v.inputDigest }
func (v VerifiedQualification) InputByteLength() int         { return v.inputByteLength }
func (v VerifiedQualification) ValidAt(epoch uint64) bool {
	return v.validFromEpoch != 0 && epoch >= v.validFromEpoch && epoch <= v.validThroughEpoch
}
func (v VerifiedQualification) Matches(policy GroupingPolicy, input AdmittedInput, epoch uint64) bool {
	return v.ValidAt(epoch) &&
		v.policyID == policy.ID() && v.policyDigest == policy.Digest() && v.policyByteLength == policy.ByteLength() &&
		v.inputID == input.ID() && v.inputDigest == input.Digest() && v.inputByteLength == input.ByteLength()
}

var requiredMetricCodes = []string{"FALSE_MERGE_RATE", "FRAGMENTATION_RATE", "STABILITY_RATE", "UNMATCHED_NO_COMMUNITY_RATE", "SHARED_NODE_EFFECT_RATE", "HIGH_DEGREE_COLLAPSE_RATE", "RESOURCE_LIMIT_COMPLIANCE", "DETERMINISTIC_REPLAY", "INPUT_PERMUTATION_STABILITY", "CROSS_SEED_INSTABILITY_CALLS_ONLY", "CROSS_SEED_INSTABILITY_REFERENCES_ONLY", "CROSS_SEED_INSTABILITY_DEFINITIONS_ONLY", "CROSS_SEED_INSTABILITY_REFERENCES_PLUS_DEFINITIONS", "CROSS_SEED_INSTABILITY_CALLS_PLUS_REFERENCES", "CROSS_SEED_INSTABILITY_CALLS_PLUS_DEFINITIONS", "CROSS_SEED_INSTABILITY_CALLS_PLUS_REFERENCES_PLUS_DEFINITIONS"}
var requiredArms = []string{"CALLS_ONLY", "REFERENCES_ONLY", "DEFINITIONS_ONLY", "REFERENCES_PLUS_DEFINITIONS", "CALLS_PLUS_REFERENCES", "CALLS_PLUS_DEFINITIONS", "CALLS_PLUS_REFERENCES_PLUS_DEFINITIONS", "ADVERSARIAL_BUILTIN", "ADVERSARIAL_IMPORT_REEXPORT", "ADVERSARIAL_GENERATED", "ADVERSARIAL_SHARED_HELPER", "ADVERSARIAL_DUPLICATE_PROVIDER", "ADVERSARIAL_ALIAS_MULTI_TARGET", "ADVERSARIAL_CROSS_LANGUAGE_REVISION"}

func ParseGroupingPolicyQualificationReceipt(raw []byte, l QualificationParseLimits) (GroupingPolicyQualificationReceipt, error) {
	return parseQualification(raw, l, "GROUPING_POLICY_QUALIFICATION_RECEIPT", "v1", "GROUPING_POLICY", "lsp-trace:grouping-policy-qualification-receipt:v1", false)
}

func VerifyGroupingPolicyQualification(
	receipt GroupingPolicyQualificationReceipt,
	policy GroupingPolicy,
	input AdmittedInput,
	epoch uint64,
) (VerifiedQualification, error) {
	// A receipt carries assertions and commitments, not the independently held
	// bytes or reviewer trust records needed to establish production authority.
	return VerifiedQualification{}, errors.New("independent qualification evidence required")
}
func parseQualification(raw []byte, l QualificationParseLimits, fam, ver, class, domain string, strictTest bool) (GroupingPolicyQualificationReceipt, error) {
	if l.MaxRawBytes <= 0 || len(raw) > l.MaxRawBytes || len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || bytes.Contains(raw, []byte("\r")) {
		return GroupingPolicyQualificationReceipt{}, errors.New("framing")
	}
	if err := scanQualification(raw[:len(raw)-1], l); err != nil {
		return GroupingPolicyQualificationReceipt{}, err
	}
	d := qualificationDocument{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return GroupingPolicyQualificationReceipt{}, err
	}
	if err := requireEOF(dec); err != nil {
		return GroupingPolicyQualificationReceipt{}, err
	}
	if d.Family != fam || d.Version != ver || d.PolicyClass != class || d.IdentityDomain != domain {
		return GroupingPolicyQualificationReceipt{}, errors.New("receipt envelope")
	}
	if !qualificationSorted(d) {
		return GroupingPolicyQualificationReceipt{}, errors.New("order")
	}
	if !knownReviewerDecisions(d.Reviewers) {
		return GroupingPolicyQualificationReceipt{}, errors.New("reviewer decision")
	}
	canon, _ := json.Marshal(d)
	canon = append(canon, '\n')
	if !bytes.Equal(canon, raw) {
		return GroupingPolicyQualificationReceipt{}, errors.New("noncanonical")
	}
	out := deriveQualificationOutcome(d, strictTest)
	return GroupingPolicyQualificationReceipt{append([]byte(nil), raw...), d, out}, nil
}
func deriveQualificationOutcome(d qualificationDocument, strict bool) string {
	if !commitmentsPresent(d) || !completeCases(d, strict) || !completeMetrics(d) || !completeCriteria(d) || !reviewComplete(d) || !accountingComplete(d) || (strict && !exactTestReceipt(d)) || (!strict && !productionPinsCoherent(d)) {
		return "INCOMPLETE"
	}
	if reviewDeclined(d) || !metricsPass(d) {
		return "REJECTED"
	}
	return "QUALIFIED"
}
func commitmentsPresent(d qualificationDocument) bool {
	for _, c := range []qualificationCommitment{d.Policy, d.GroupingInput, d.EvaluationPlan, d.CorpusManifest, d.Implementation, d.Environment, d.Report} {
		if c.ID == "" || c.ByteLength <= 0 || !validDigest(c.Digest) {
			return false
		}
	}
	return true
}
func completeCases(d qualificationDocument, strict bool) bool {
	if len(d.Cases) != len(requiredArms) {
		return false
	}
	seen := map[string]bool{}
	for _, c := range d.Cases {
		if seen[c.Arm] || !contains(requiredArms, c.Arm) || c.ID == "" || c.Partition == "" || !c.Required || !c.Completed || c.Candidates <= 0 || c.Evaluated <= 0 || c.Passed <= 0 || c.Rejected < 0 || c.Excluded < 0 || c.Failed < 0 || c.Evaluated != c.Passed+c.Rejected || c.Candidates != c.Evaluated+c.Excluded+c.Failed {
			return false
		}
		if c.Corpus == "" || c.Language == "" || c.Provider == "" || c.ProviderProfile == "" || c.Revision == "" {
			return false
		}
		seen[c.Arm] = true
	}
	return true
}
func completeMetrics(d qualificationDocument) bool {
	if len(d.Metrics) != len(requiredMetricCodes) {
		return false
	}
	seen := map[string]bool{}
	for _, m := range d.Metrics {
		if seen[m.Code] || !contains(requiredMetricCodes, m.Code) || !reducedMetric(m) {
			return false
		}
		seen[m.Code] = true
	}
	return true
}
func completeCriteria(d qualificationDocument) bool {
	if len(d.Criteria) != len(requiredMetricCodes) {
		return false
	}
	seen := map[string]bool{}
	for _, c := range d.Criteria {
		if seen[c.Code] || c.Code != "REQUIRE_"+c.MetricCode || !contains(requiredMetricCodes, c.MetricCode) || c.Comparator != "EQ" || reduceRational(c.Threshold) != c.Threshold || c.Threshold.Denominator <= 0 {
			return false
		}
		want := 0
		if c.MetricCode == "STABILITY_RATE" || c.MetricCode == "RESOURCE_LIMIT_COMPLIANCE" || c.MetricCode == "DETERMINISTIC_REPLAY" || c.MetricCode == "INPUT_PERMUTATION_STABILITY" {
			want = 1
		}
		if c.Threshold != (Rational{want, 1}) {
			return false
		}
		seen[c.Code] = true
	}
	return true
}
func knownReviewerDecisions(reviewers []qualificationReviewer) bool {
	for _, r := range reviewers {
		if r.Decision != "" && r.Decision != "ACCEPT" && r.Decision != "DECLINE" {
			return false
		}
	}
	return true
}
func reviewComplete(d qualificationDocument) bool {
	if d.RequiredReviewerClass == "" || d.RequiredAcceptances <= 0 || len(d.Reviewers) < d.RequiredAcceptances || d.ValidFromEpoch == 0 || d.ValidThroughEpoch < d.ValidFromEpoch || d.DecisionUnix <= 0 {
		return false
	}
	seen := map[string]bool{}
	for _, r := range d.Reviewers {
		if r.Identity == "" || seen[r.Identity] || r.IdentityDomain != d.IdentityDomain || r.IdentityClass != d.RequiredReviewerClass || r.Custody != "REVIEWER_ASSERTED" || r.Decision == "" {
			return false
		}
		seen[r.Identity] = true
	}
	return true
}
func reviewDeclined(d qualificationDocument) bool {
	accept := 0
	for _, r := range d.Reviewers {
		if r.Decision == "DECLINE" {
			return true
		}
		if r.Decision == "ACCEPT" {
			accept++
		}
	}
	return accept < d.RequiredAcceptances
}
func accountingComplete(d qualificationDocument) bool {
	var sum qualificationAccounting
	for _, c := range d.Cases {
		sum.Candidates += c.Candidates
		sum.Evaluated += c.Evaluated
		sum.Passed += c.Passed
		sum.Rejected += c.Rejected
		sum.Excluded += c.Excluded
		sum.Failed += c.Failed
	}
	return sum == d.Accounting && sum.Evaluated == sum.Passed+sum.Rejected && sum.Candidates == sum.Evaluated+sum.Excluded+sum.Failed
}
func metricsPass(d qualificationDocument) bool {
	for _, m := range d.Metrics {
		want := 0
		if m.Code == "STABILITY_RATE" || m.Code == "RESOURCE_LIMIT_COMPLIANCE" || m.Code == "DETERMINISTIC_REPLAY" || m.Code == "INPUT_PERMUTATION_STABILITY" {
			want = m.Denominator
		}
		if m.Numerator != want {
			return false
		}
	}
	return true
}
func reducedMetric(m qualificationMetric) bool {
	return m.Denominator > 0 && m.Numerator >= 0 && m.Numerator <= m.Denominator && gcd(m.Numerator, m.Denominator) == 1
}
func qualificationSorted(d qualificationDocument) bool {
	return sort.SliceIsSorted(d.Cases, func(i, j int) bool { return d.Cases[i].ID < d.Cases[j].ID }) && sort.SliceIsSorted(d.Metrics, func(i, j int) bool { return d.Metrics[i].Code < d.Metrics[j].Code }) && sort.SliceIsSorted(d.Criteria, func(i, j int) bool { return d.Criteria[i].Code < d.Criteria[j].Code })
}
func validDigest(s string) bool {
	return len(s) == 71 && strings.HasPrefix(s, "sha256:") && strings.Trim(s[7:], "0123456789abcdef") == ""
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func scanQualification(raw []byte, l QualificationParseLimits) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func(json.Token, string) error
	walk = func(t json.Token, f string) error {
		switch v := t.(type) {
		case string:
			if len(v) > l.MaxStringBytes {
				return errors.New("string bound")
			}
		case json.Number:
			if !validInteger(string(v)) {
				return errors.New("number")
			}
		case json.Delim:
			switch v {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					k, e := dec.Token()
					if e != nil {
						return e
					}
					ks, ok := k.(string)
					if !ok || seen[ks] {
						return errors.New("key")
					}
					seen[ks] = true
					n, e := dec.Token()
					if e != nil {
						return e
					}
					if e = walk(n, ks); e != nil {
						return e
					}
				}
				_, e := dec.Token()
				return e
			case '[':
				limit, ok := map[string]int{"Cases": l.MaxCases, "Metrics": l.MaxMetrics, "Criteria": l.MaxCriteria, "Reviewers": l.MaxReviewers}[f]
				if !ok {
					return errors.New("array")
				}
				n := 0
				for dec.More() {
					n++
					if n > limit {
						return errors.New("array bound")
					}
					x, e := dec.Token()
					if e != nil {
						return e
					}
					if e = walk(x, ""); e != nil {
						return e
					}
				}
				_, e := dec.Token()
				return e
			}
		}
		return nil
	}
	t, e := dec.Token()
	if e != nil {
		return e
	}
	if e = walk(t, ""); e != nil {
		return e
	}
	if _, e = dec.Token(); e != io.EOF {
		return errors.New("trailing")
	}
	return nil
}
func verifyQualificationEvidence(r GroupingPolicyQualificationReceipt, evidence, trust map[string][]byte, p GroupingPolicy, in AdmittedInput, epoch uint64, strict bool) (VerifiedQualification, error) {
	if !strict {
		return VerifiedQualification{}, errors.New("production evidence verifier unavailable")
	}
	if err := verifyIndependentEvidence(r.doc, evidence); err != nil {
		return VerifiedQualification{}, err
	}
	if err := verifyQualificationReviewerAuthorization(r, trust); err != nil {
		return VerifiedQualification{}, err
	}
	return verifyQualification(r, p, in, epoch, true)
}

func verifyIndependentEvidence(d qualificationDocument, evidence map[string][]byte) error {
	for name, commitment := range map[string]qualificationCommitment{
		"evaluation-plan": d.EvaluationPlan, "corpus-manifest": d.CorpusManifest,
		"implementation": d.Implementation, "environment": d.Environment, "report": d.Report,
	} {
		raw, ok := evidence[name]
		if !ok || commitment.ID != name || commitment.ByteLength != len(raw) || commitment.Digest != receiptDigest(raw) {
			return errors.New("independent evidence commitment mismatch")
		}
	}
	var report struct {
		Family     string                  `json:"family"`
		Accounting qualificationAccounting `json:"accounting"`
	}
	dec := json.NewDecoder(bytes.NewReader(evidence["report"]))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil || requireEOF(dec) != nil || report.Family != "TEST_ONLY_QUALIFICATION_REPORT" || report.Accounting != d.Accounting {
		return errors.New("qualification report mismatch")
	}
	return nil
}

func verifyQualificationReviewerAuthorization(receipt GroupingPolicyQualificationReceipt, trust map[string][]byte) error {
	type anchor struct {
		Identity         string `json:"identity"`
		IdentityDomain   string `json:"identity_domain"`
		IdentityClass    string `json:"identity_class"`
		Custody          string `json:"custody"`
		Decision         string `json:"decision"`
		KeyCommitment    string `json:"key_commitment"`
		RecordCommitment string `json:"record_commitment"`
	}
	receiptSum := sha256.Sum256(receipt.canonical)
	for _, reviewer := range receipt.doc.Reviewers {
		raw, ok := trust[reviewer.Identity]
		if !ok {
			return errors.New("reviewer trust missing")
		}
		var a anchor
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil || requireEOF(dec) != nil || a.Identity != reviewer.Identity || a.IdentityDomain != reviewer.IdentityDomain || a.IdentityClass != reviewer.IdentityClass || a.Custody != reviewer.Custody || a.Decision != reviewer.Decision || a.KeyCommitment != "sha256:"+strings.Repeat("a", 64) {
			return errors.New("reviewer trust mismatch")
		}
		record := struct {
			Domain                 string                    `json:"domain"`
			ReceiptDigest          string                    `json:"receipt_digest"`
			ReceiptByteLength      int                       `json:"receipt_byte_length"`
			ReviewerIdentity       string                    `json:"reviewer_identity"`
			ReviewerIdentityDomain string                    `json:"reviewer_identity_domain"`
			ReviewerIdentityClass  string                    `json:"reviewer_identity_class"`
			ReviewerCustody        string                    `json:"reviewer_custody"`
			Decision               string                    `json:"decision"`
			PolicyCommitment       qualificationCommitment   `json:"policy_commitment"`
			InputCommitment        qualificationCommitment   `json:"input_commitment"`
			EvidenceCommitments    []qualificationCommitment `json:"evidence_commitments"`
		}{
			"lsp-trace:test-grouping-qualification-review-record:v1", fmt.Sprintf("sha256:%x", receiptSum), len(receipt.canonical),
			reviewer.Identity, reviewer.IdentityDomain, reviewer.IdentityClass, reviewer.Custody, reviewer.Decision,
			receipt.doc.Policy, receipt.doc.GroupingInput,
			[]qualificationCommitment{receipt.doc.EvaluationPlan, receipt.doc.CorpusManifest, receipt.doc.Implementation, receipt.doc.Environment, receipt.doc.Report},
		}
		canonicalRecord, err := json.Marshal(record)
		if err != nil {
			return err
		}
		recordSum := sha256.Sum256(append([]byte("lsp-trace:test-grouping-qualification-review-record-commitment:v1\n"), canonicalRecord...))
		if a.RecordCommitment != fmt.Sprintf("sha256:%x", recordSum) {
			return errors.New("reviewer trust mismatch")
		}
	}
	return nil
}

func verifyQualification(r GroupingPolicyQualificationReceipt, p GroupingPolicy, in AdmittedInput, epoch uint64, strict bool) (VerifiedQualification, error) {
	if r.outcome != "QUALIFIED" || epoch < r.doc.ValidFromEpoch || epoch > r.doc.ValidThroughEpoch {
		return VerifiedQualification{}, errors.New("not qualified")
	}
	if r.doc.Policy.ID != p.ID() || r.doc.Policy.Digest != p.Digest() || r.doc.Policy.ByteLength != p.ByteLength() || r.doc.GroupingInput.ID != in.ID() || r.doc.GroupingInput.Digest != in.Digest() || r.doc.GroupingInput.ByteLength != len(in.CanonicalBytes()) {
		return VerifiedQualification{}, errors.New("binding")
	}
	if strict && !exactTestReceipt(r.doc) {
		return VerifiedQualification{}, errors.New("test receipt substitution")
	}
	return VerifiedQualification{
		policyID: p.ID(), policyDigest: p.Digest(), policyByteLength: p.ByteLength(),
		inputID: in.ID(), inputDigest: in.Digest(), inputByteLength: in.ByteLength(),
		validFromEpoch: r.doc.ValidFromEpoch, validThroughEpoch: r.doc.ValidThroughEpoch,
	}, nil
}
func exactTestReceipt(d qualificationDocument) bool {
	if d.Version != "test-v1" || d.ValidFromEpoch != 7 || d.ValidThroughEpoch != 9 || d.DecisionUnix != 1700000000 || d.RequiredReviewerClass != "LSP_TRACE_TEST_MAINTAINER" || d.RequiredAcceptances != 1 {
		return false
	}
	evidence := map[string][]byte{
		"evaluation-plan": []byte("TEST ONLY EVALUATION PLAN v1\n"),
		"corpus-manifest": []byte("TEST ONLY CORPUS MANIFEST v1\n"),
		"implementation":  []byte("TEST ONLY IMPLEMENTATION v1\n"),
		"environment":     []byte("TEST ONLY ENVIRONMENT v1\n"),
	}
	report, _ := json.Marshal(struct {
		Family     string                  `json:"family"`
		Accounting qualificationAccounting `json:"accounting"`
	}{"TEST_ONLY_QUALIFICATION_REPORT", qualificationAccounting{56, 42, 28, 14, 14, 0}})
	evidence["report"] = append(report, '\n')
	commitment := func(id string) qualificationCommitment {
		raw := evidence[id]
		return qualificationCommitment{id, len(raw), receiptDigest(raw)}
	}
	want := []qualificationCommitment{commitment("evaluation-plan"), commitment("corpus-manifest"), commitment("implementation"), commitment("environment"), commitment("report")}
	got := []qualificationCommitment{d.EvaluationPlan, d.CorpusManifest, d.Implementation, d.Environment, d.Report}
	if fmt.Sprint(want) != fmt.Sprint(got) {
		return false
	}
	if len(d.Reviewers) != 1 {
		return false
	}
	reviewer := d.Reviewers[0]
	if reviewer.Identity != "reviewer-1" || reviewer.IdentityDomain != d.IdentityDomain || reviewer.IdentityClass != "LSP_TRACE_TEST_MAINTAINER" || reviewer.Custody != "REVIEWER_ASSERTED" || (reviewer.Decision != "ACCEPT" && reviewer.Decision != "DECLINE") {
		return false
	}
	return exactTestValues(d)
}
func productionPinsCoherent(d qualificationDocument) bool {
	type pins struct{ corpus, provider, revision string }
	byProfile := map[string]pins{}
	for _, c := range d.Cases {
		key := c.Language + "\x00" + c.ProviderProfile
		p := pins{c.Corpus, c.Provider, c.Revision}
		if prior, ok := byProfile[key]; ok && prior != p {
			return false
		}
		byProfile[key] = p
	}
	return true
}
func exactTestValues(d qualificationDocument) bool {
	const (
		goCorpus    = "test-only://adr0011-stage4/go-corpus@sha256:1111111111111111111111111111111111111111111111111111111111111111"
		cueCorpus   = "test-only://adr0011-stage4/cue-corpus@sha256:2222222222222222222222222222222222222222222222222222222222222222"
		goProvider  = "test-only://provider/gopls@sha256:3333333333333333333333333333333333333333333333333333333333333333"
		cueProvider = "test-only://provider/cue-lsp@sha256:4444444444444444444444444444444444444444444444444444444444444444"
		goRevision  = "git:5555555555555555555555555555555555555555"
		cueRevision = "git:6666666666666666666666666666666666666666"
	)
	goCase := func(id, arm, partition string) qualificationCase {
		return qualificationCase{id, arm, partition, goCorpus, "go", goProvider, "GO_GOPLS_EXACT_V1", goRevision, true, true, 4, 3, 2, 1, 1, 0}
	}
	cueCase := func(id, arm, partition string) qualificationCase {
		return qualificationCase{id, arm, partition, cueCorpus, "cue", cueProvider, "TEST_ONLY_CUE_PROVIDER_EXACT_V1", cueRevision, true, true, 4, 3, 2, 1, 1, 0}
	}
	expectedCases := map[string]qualificationCase{
		"CALLS_ONLY":                             goCase("calls", "CALLS_ONLY", "CALIBRATION"),
		"REFERENCES_ONLY":                        goCase("references", "REFERENCES_ONLY", "CALIBRATION"),
		"DEFINITIONS_ONLY":                       goCase("definitions", "DEFINITIONS_ONLY", "CALIBRATION"),
		"REFERENCES_PLUS_DEFINITIONS":            cueCase("references-definitions", "REFERENCES_PLUS_DEFINITIONS", "HELD_OUT"),
		"CALLS_PLUS_REFERENCES":                  goCase("calls-references", "CALLS_PLUS_REFERENCES", "HELD_OUT"),
		"CALLS_PLUS_DEFINITIONS":                 goCase("calls-definitions", "CALLS_PLUS_DEFINITIONS", "HELD_OUT"),
		"CALLS_PLUS_REFERENCES_PLUS_DEFINITIONS": goCase("calls-references-definitions", "CALLS_PLUS_REFERENCES_PLUS_DEFINITIONS", "HELD_OUT"),
		"ADVERSARIAL_BUILTIN":                    goCase("builtin-hub", "ADVERSARIAL_BUILTIN", "ADVERSARIAL"),
		"ADVERSARIAL_IMPORT_REEXPORT":            goCase("import-reexport", "ADVERSARIAL_IMPORT_REEXPORT", "ADVERSARIAL"),
		"ADVERSARIAL_GENERATED":                  goCase("generated-registry", "ADVERSARIAL_GENERATED", "ADVERSARIAL"),
		"ADVERSARIAL_SHARED_HELPER":              goCase("shared-helper", "ADVERSARIAL_SHARED_HELPER", "ADVERSARIAL"),
		"ADVERSARIAL_DUPLICATE_PROVIDER":         goCase("duplicate-provider", "ADVERSARIAL_DUPLICATE_PROVIDER", "ADVERSARIAL"),
		"ADVERSARIAL_ALIAS_MULTI_TARGET":         goCase("aliases-multitarget", "ADVERSARIAL_ALIAS_MULTI_TARGET", "ADVERSARIAL"),
		"ADVERSARIAL_CROSS_LANGUAGE_REVISION":    cueCase("cross-language-revision", "ADVERSARIAL_CROSS_LANGUAGE_REVISION", "ADVERSARIAL"),
	}
	if len(d.Cases) != len(expectedCases) || d.Accounting != (qualificationAccounting{Candidates: 56, Evaluated: 42, Passed: 28, Rejected: 14, Excluded: 14, Failed: 0}) {
		return false
	}
	for _, c := range d.Cases {
		if expected, ok := expectedCases[c.Arm]; !ok || c != expected {
			return false
		}
	}
	expectedMetrics := map[string]qualificationMetric{}
	for _, code := range requiredMetricCodes {
		numerator := 0
		if code == "STABILITY_RATE" || code == "RESOURCE_LIMIT_COMPLIANCE" || code == "DETERMINISTIC_REPLAY" || code == "INPUT_PERMUTATION_STABILITY" {
			numerator = 1
		}
		expectedMetrics[code] = qualificationMetric{code, numerator, 1}
	}
	if len(d.Metrics) != len(expectedMetrics) || len(d.Criteria) != len(expectedMetrics) {
		return false
	}
	for _, m := range d.Metrics {
		if expected, ok := expectedMetrics[m.Code]; !ok || m.Denominator != expected.Denominator {
			return false
		}
	}
	for _, c := range d.Criteria {
		metric, ok := expectedMetrics[c.MetricCode]
		if !ok || c != (qualificationCriterion{"REQUIRE_" + c.MetricCode, c.MetricCode, "EQ", Rational{metric.Numerator, 1}}) {
			return false
		}
	}
	return true
}
func receiptDigest(raw []byte) string { h := sha256.Sum256(raw); return fmt.Sprintf("sha256:%x", h) }
