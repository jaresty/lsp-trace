package execution

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
)

const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func validSpec(t *testing.T) (RequestSpec, func()) {
	t.Helper()
	dir := t.TempDir()
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	spec := RequestSpec{
		RequestID:   "request-1",
		Receipts:    []source.ManifestReceipt{{ID: "root", Path: "main.go", Digest: digest}},
		Decisions:   []source.ManifestDecision{{ReceiptID: "root", State: source.ManifestInclude}},
		Publication: PublicationDestination{Root: root, Selector: "results/request-1.json"},
	}
	return spec, func() { _ = root.Close() }
}

func requireCode(t *testing.T, err error, code ValidationCode) {
	t.Helper()
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Code != code {
		t.Fatalf("ASSERT_CANONICAL_VALIDATION_%s: got %v", code, err)
	}
}

func TestRequestRejectsNonCanonicalIdentity(t *testing.T) {
	spec, closeRoot := validSpec(t)
	defer closeRoot()
	for _, id := range []string{"", " request-1", "request-1 ", "request\x00one"} {
		spec.RequestID = id
		_, err := NewRequest(spec)
		requireCode(t, err, ValidationInvalidIdentity)
	}
}

func TestRequestRequiresExplicitExactManifestDecisions(t *testing.T) {
	spec, closeRoot := validSpec(t)
	defer closeRoot()

	t.Run("omission", func(t *testing.T) {
		candidate := spec
		candidate.Decisions = nil
		_, err := NewRequest(candidate)
		requireCode(t, err, ValidationManifestAccounting)
	})
	t.Run("unknown", func(t *testing.T) {
		candidate := spec
		candidate.Decisions = append(candidate.Decisions, source.ManifestDecision{ReceiptID: "unknown", State: source.ManifestExclude, Reason: "not selected"})
		_, err := NewRequest(candidate)
		requireCode(t, err, ValidationManifestAccounting)
	})
	t.Run("duplicate", func(t *testing.T) {
		candidate := spec
		candidate.Decisions = append(candidate.Decisions, candidate.Decisions[0])
		_, err := NewRequest(candidate)
		requireCode(t, err, ValidationManifestDuplicate)
	})
	t.Run("malformed", func(t *testing.T) {
		candidate := spec
		candidate.Decisions[0].State = "MAYBE"
		_, err := NewRequest(candidate)
		requireCode(t, err, ValidationManifestInvalid)
	})
}

func TestRequestRejectsNonCanonicalPublicationDestination(t *testing.T) {
	spec, closeRoot := validSpec(t)
	defer closeRoot()
	for _, selector := range []string{"", ".", "a/../b", "./result.json", "/tmp/result.json", "../result.json", `a\\b`} {
		spec.Publication.Selector = selector
		_, err := NewRequest(spec)
		requireCode(t, err, ValidationPublicationDestination)
	}

	candidate := spec
	candidate.Publication = PublicationDestination{Selector: "result.json"}
	_, err := NewRequest(candidate)
	requireCode(t, err, ValidationPublicationDestination)
}

func TestRequestAllowsAbsentTrustAndCopiesPresentTrust(t *testing.T) {
	spec, closeRoot := validSpec(t)
	defer closeRoot()
	request, err := NewRequest(spec)
	if err != nil || request.Trust() != nil {
		t.Fatalf("ASSERT_OPTIONAL_TRUST_ABSENT: request=%v err=%v", request, err)
	}

	spec.Trust = &schema.TrustAdmissionRequest{
		Receipt:        []byte("receipt"),
		Context:        schema.TrustAuthenticationContext{ProvisionedReceiptIDs: map[string]struct{}{"trust-1": {}}},
		GitAttestation: &schema.GitAttestationEvidence{EvidenceType: schema.GitCommitAttestation, CommitIdentity: "commit-1"},
	}
	request, err = NewRequest(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Trust.Receipt[0] = 'X'
	delete(spec.Trust.Context.ProvisionedReceiptIDs, "trust-1")
	spec.Trust.GitAttestation.CommitIdentity = "changed"
	got := request.Trust()
	if string(got.Receipt) != "receipt" || len(got.Context.ProvisionedReceiptIDs) != 1 || got.GitAttestation.CommitIdentity != "commit-1" {
		t.Fatalf("ASSERT_DEFENSIVE_COPY_TRUST_INGRESS: %+v", got)
	}
	got.Receipt[0] = 'Y'
	delete(got.Context.ProvisionedReceiptIDs, "trust-1")
	got.GitAttestation.CommitIdentity = "changed-again"
	again := request.Trust()
	if string(again.Receipt) != "receipt" || len(again.Context.ProvisionedReceiptIDs) != 1 || again.GitAttestation.CommitIdentity != "commit-1" {
		t.Fatalf("ASSERT_DEFENSIVE_COPY_TRUST_EGRESS: %+v", again)
	}
}

func TestRequestDefensivelyCopiesManifestInputsAndOutputs(t *testing.T) {
	spec, closeRoot := validSpec(t)
	defer closeRoot()
	request, err := NewRequest(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Receipts[0].Path = "changed.go"
	spec.Decisions[0].State = source.ManifestExclude
	receipts, decisions := request.ManifestInputs()
	if receipts[0].Path != "main.go" || decisions[0].State != source.ManifestInclude {
		t.Fatalf("ASSERT_DEFENSIVE_COPY_MANIFEST_INGRESS: receipts=%+v decisions=%+v", receipts, decisions)
	}
	receipts[0].Path = "changed-again.go"
	decisions[0].State = source.ManifestExclude
	receipts, decisions = request.ManifestInputs()
	if receipts[0].Path != "main.go" || decisions[0].State != source.ManifestInclude {
		t.Fatalf("ASSERT_DEFENSIVE_COPY_MANIFEST_EGRESS: receipts=%+v decisions=%+v", receipts, decisions)
	}
}

func TestRequestPreservesPublicationDestinationWithoutExecutingIt(t *testing.T) {
	spec, closeRoot := validSpec(t)
	defer closeRoot()
	request, err := NewRequest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if request.RequestID() != "request-1" || request.Publication() != spec.Publication {
		t.Fatalf("ASSERT_REQUEST_DATA_PRESERVED: id=%q publication=%+v", request.RequestID(), request.Publication())
	}
	if _, err := os.Stat(filepath.Join(spec.Publication.Root.Path(), spec.Publication.Selector)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ASSERT_NO_STAGE_EXECUTION: publication target unexpectedly exists: %v", err)
	}
}
