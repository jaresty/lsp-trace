package programcadmission

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestCanonicalMethodEvidenceBytesMatchesEncodingJSONForNilOccurrences(t *testing.T) {
	in := validInternalMethodEvidenceEnvelope()
	in.Occurrences = nil
	want, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalMethodEvidenceBytes(in, len(want))
	if err != nil || string(got) != string(want) {
		t.Fatalf("ASSERT_METHOD_CANONICAL_NIL_SLICE_PARITY got=%q want=%q err=%v", got, want, err)
	}
}

func TestCanonicalMethodEvidenceBytesAcceptsExactLimitAndRejectsOverflowAtomically(t *testing.T) {
	in := validInternalMethodEvidenceEnvelope()
	in.Occurrences[0].Identity = "escape-\"-\\-\n-\t-世界"
	in.Occurrences[0].From = "file:///workspace/\"quoted\"/\\source.go#使用"
	in.Occurrences[0].To = "file:///workspace/\"quoted\"/\\target.go#定義"
	in.Occurrences[0].QuerySelector = "queries/\"quoted\"/\\query-\n.json"
	in.Occurrences[0].EvidenceSelector = "evidence/\"quoted\"/\\evidence-\t.json"

	canonical, err := canonicalMethodEvidenceBytes(in, math.MaxInt)
	if err != nil {
		t.Fatalf("BLOCKED_NOT_RED canonical fixture: %v", err)
	}
	if raw := methodPreflightInputBytesForTest(in); raw > MaxMethodCommitmentInputBytes {
		t.Fatalf("BLOCKED_NOT_RED escaping fixture exceeds raw preflight cap: raw=%d cap=%d", raw, MaxMethodCommitmentInputBytes)
	}
	if len(canonical) <= methodPreflightInputBytesForTest(in) {
		t.Fatalf("BLOCKED_NOT_RED escaping fixture did not expand under JSON encoding: canonical=%d raw=%d", len(canonical), methodPreflightInputBytesForTest(in))
	}

	exact, err := canonicalMethodEvidenceBytes(in, len(canonical))
	if err != nil || string(exact) != string(canonical) {
		t.Fatalf("ASSERT_METHOD_CANONICAL_EXACT_BOUND_ACCEPTED bytes=%d err=%v", len(canonical), err)
	}
	over, err := canonicalMethodEvidenceBytes(in, len(canonical)-1)
	if !errors.Is(err, ErrMethodEvidenceResourceBound) || len(over) != 0 {
		t.Fatalf("ASSERT_METHOD_CANONICAL_OVER_BOUND_ATOMIC bytes=%d err=%v", len(over), err)
	}
}

func TestAddBoundedSaturatesWithoutIntegerOverflow(t *testing.T) {
	if got, exceeded := addBounded(40, 2, 42); got != 42 || exceeded {
		t.Fatalf("ASSERT_METHOD_PREFLIGHT_ADD_EXACT_BOUND got=%d exceeded=%t", got, exceeded)
	}
	if got, exceeded := addBounded(math.MaxInt-2, 10, math.MaxInt-1); got != math.MaxInt || !exceeded {
		t.Fatalf("ASSERT_METHOD_PREFLIGHT_ADD_SATURATES got=%d exceeded=%t", got, exceeded)
	}
}

func methodPreflightInputBytesForTest(in UntrustedMethodEvidenceEnvelope) int {
	total := len(in.Family) + len(in.Version) + len(in.ClaimCeiling) + len(in.Completeness) +
		len(in.Provider) + len(in.ProviderVersion) + len(in.SessionID) + len(in.Revision) +
		len(in.Custody) + len(in.PositionEncoding)
	for _, o := range in.Occurrences {
		total += len(o.Identity) + len(o.From) + len(o.To) + len(string(o.Kind)) + len(o.SourceRole) + len(o.TargetRole) +
			len(o.Method) + len(o.QuerySelector) + len(o.QueryDigest) + len(o.EvidenceSelector) +
			len(o.EvidenceDigest) + len(o.Provider) + len(o.ProviderVersion) + len(o.SessionID) +
			len(o.Revision) + len(o.Custody) + len(o.PositionEncoding)
	}
	return total
}

func validInternalMethodEvidenceEnvelope() UntrustedMethodEvidenceEnvelope {
	occurrence := MethodOccurrenceEvidence{
		Identity:         "reference-0",
		Kind:             TypedReferencesSymbol,
		Ordinal:          0,
		From:             "file:///workspace/use.go#reference",
		To:               "file:///workspace/decl.go#symbol",
		SourceRole:       "REFERENCING_OCCURRENCE",
		TargetRole:       "REFERENCED_SYMBOL",
		Method:           "textDocument/references",
		QuerySelector:    "queries/reference-0.json",
		QueryDigest:      "sha256:" + strings.Repeat("1", 64),
		QueryLength:      17,
		EvidenceSelector: "evidence/reference-0.json",
		EvidenceDigest:   "sha256:" + strings.Repeat("2", 64),
		EvidenceLength:   29,
		Provider:         "gopls",
		ProviderVersion:  "v0.17.1",
		SessionID:        "session-1",
		Generation:       7,
		Revision:         "commit-1",
		Custody:          "CALLER_ASSERTED",
		PositionEncoding: "utf-16",
	}
	return UntrustedMethodEvidenceEnvelope{
		Family:           MethodEvidenceFamily,
		Version:          MethodEvidenceFamily,
		ClaimCeiling:     MethodEvidenceClaimCeiling,
		Completeness:     "UNKNOWN",
		Provider:         occurrence.Provider,
		ProviderVersion:  occurrence.ProviderVersion,
		SessionID:        occurrence.SessionID,
		Generation:       occurrence.Generation,
		Revision:         occurrence.Revision,
		Custody:          occurrence.Custody,
		PositionEncoding: occurrence.PositionEncoding,
		Occurrences:      []MethodOccurrenceEvidence{occurrence},
	}
}
