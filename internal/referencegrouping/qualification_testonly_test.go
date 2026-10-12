package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func parseTestOnlyQualificationReceipt(raw []byte, l QualificationParseLimits) (GroupingPolicyQualificationReceipt, error) {
	return parseQualification(raw, l, testQualificationFamily, testQualificationVersion, testPolicyClass, testIdentityDomain, true)
}
func verifyTestOnlyQualification(r GroupingPolicyQualificationReceipt, p GroupingPolicy, in AdmittedInput, epoch uint64) (VerifiedQualification, error) {
	baseline := completeTestQualification(p, in)
	sort.Slice(baseline.Cases, func(i, j int) bool { return baseline.Cases[i].ID < baseline.Cases[j].ID })
	sort.Slice(baseline.Metrics, func(i, j int) bool { return baseline.Metrics[i].Code < baseline.Metrics[j].Code })
	sort.Slice(baseline.Criteria, func(i, j int) bool { return baseline.Criteria[i].Code < baseline.Criteria[j].Code })
	raw, err := json.Marshal(baseline)
	if err != nil {
		return VerifiedQualification{}, err
	}
	baselineReceipt, err := parseTestOnlyQualificationReceipt(append(raw, '\n'), generousQualificationParseLimits())
	if err != nil {
		return VerifiedQualification{}, err
	}
	return verifyTestOnlyQualificationWithEvidence(r, independentTestQualificationEvidence(), independentTestQualificationTrustAnchorsForParsed(baselineReceipt), p, in, epoch)
}
func independentTestQualificationTrustAnchorsForParsed(receipt GroupingPolicyQualificationReceipt) map[string][]byte {
	if len(receipt.doc.Reviewers) != 1 {
		return nil
	}
	reviewer := receipt.doc.Reviewers[0]
	receiptSum := sha256.Sum256(receipt.canonical)
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
		Domain: "lsp-trace:test-grouping-qualification-review-record:v1", ReceiptDigest: fmt.Sprintf("sha256:%x", receiptSum), ReceiptByteLength: len(receipt.canonical),
		ReviewerIdentity: reviewer.Identity, ReviewerIdentityDomain: reviewer.IdentityDomain, ReviewerIdentityClass: reviewer.IdentityClass, ReviewerCustody: reviewer.Custody, Decision: reviewer.Decision,
		PolicyCommitment: receipt.doc.Policy, InputCommitment: receipt.doc.GroupingInput,
		EvidenceCommitments: []qualificationCommitment{receipt.doc.EvaluationPlan, receipt.doc.CorpusManifest, receipt.doc.Implementation, receipt.doc.Environment, receipt.doc.Report},
	}
	canonicalRecord, err := json.Marshal(record)
	if err != nil {
		return nil
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
	}{reviewer.Identity, reviewer.IdentityDomain, reviewer.IdentityClass, reviewer.Custody, reviewer.Decision, "sha256:" + strings.Repeat("a", 64), fmt.Sprintf("sha256:%x", recordSum)})
	if err != nil {
		return nil
	}
	return map[string][]byte{reviewer.Identity: append(anchor, '\n')}
}
func verifyTestOnlyQualificationWithEvidence(r GroupingPolicyQualificationReceipt, evidence, trust map[string][]byte, p GroupingPolicy, in AdmittedInput, epoch uint64) (VerifiedQualification, error) {
	// verifyQualificationEvidence is the intended production API. This test-only
	// adapter deliberately remains a compile RED until production supplies it.
	return verifyQualificationEvidence(r, evidence, trust, p, in, epoch, true)
}
func admitTestOnlyQualificationWithEvidence(raw []byte, wantLength int, wantDigest string, evidence, trust map[string][]byte, p GroupingPolicy, in AdmittedInput, epoch uint64, l QualificationParseLimits) (VerifiedQualification, error) {
	if len(raw) != wantLength || receiptDigest(raw) != wantDigest {
		return VerifiedQualification{}, errors.New("receipt commitment mismatch")
	}
	r, err := parseTestOnlyQualificationReceipt(raw, l)
	if err != nil {
		return VerifiedQualification{}, err
	}
	return verifyTestOnlyQualificationWithEvidence(r, evidence, trust, p, in, epoch)
}
func recomputeTestOnlyQualification(r GroupingPolicyQualificationReceipt) (GroupingPolicyQualificationReceipt, error) {
	if len(r.canonical) == 0 {
		return GroupingPolicyQualificationReceipt{}, errors.New("empty receipt")
	}
	out, err := parseTestOnlyQualificationReceipt(r.canonical, generousQualificationParseLimits())
	if err != nil {
		return GroupingPolicyQualificationReceipt{}, err
	}
	if !bytes.Equal(out.canonical, r.canonical) {
		return GroupingPolicyQualificationReceipt{}, errors.New("recompute mismatch")
	}
	return out, nil
}
func admitTestOnlyQualificationReceipt(raw []byte, wantLength int, wantDigest string, l QualificationParseLimits) (GroupingPolicyQualificationReceipt, error) {
	if len(raw) != wantLength || receiptDigest(raw) != wantDigest {
		return GroupingPolicyQualificationReceipt{}, errors.New("receipt commitment mismatch")
	}
	return parseTestOnlyQualificationReceipt(raw, l)
}
