package source

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestAssembleManifestGuards(t *testing.T) {
	t.Run("ASSERT_MANIFEST_EXACT_ACCOUNTING_OMISSION", func(t *testing.T) {
		_, err := AssembleManifest(baseReceipts(), []ManifestDecision{{ReceiptID: "root", State: ManifestInclude}, {ReceiptID: "child", State: ManifestInclude}})
		assertCode(t, err, ManifestErrAccounting)
	})
	t.Run("ASSERT_MANIFEST_EXACT_ACCOUNTING_OVERLAP", func(t *testing.T) {
		_, err := AssembleManifest(baseReceipts(), []ManifestDecision{{ReceiptID: "root", State: ManifestInclude}, {ReceiptID: "child", State: ManifestInclude}, {ReceiptID: "excluded", State: ManifestExclude, Reason: "policy"}, {ReceiptID: "excluded", State: ManifestInclude}})
		assertCode(t, err, ManifestErrDuplicate)
	})
	t.Run("ASSERT_MANIFEST_PARENT_CLOSURE", func(t *testing.T) {
		_, err := AssembleManifest(baseReceipts(), []ManifestDecision{{ReceiptID: "root", State: ManifestExclude, Reason: "policy"}, {ReceiptID: "child", State: ManifestInclude}, {ReceiptID: "excluded", State: ManifestExclude, Reason: "policy"}})
		assertCode(t, err, ManifestErrParentClosure)
	})
	t.Run("ASSERT_MANIFEST_CYCLE_REJECTION", func(t *testing.T) {
		receipts := []ManifestReceipt{{ID: "a", Path: "a", ParentID: "b", Digest: digestA}, {ID: "b", Path: "b", ParentID: "a", Digest: digestB}}
		_, err := AssembleManifest(receipts, []ManifestDecision{{ReceiptID: "a", State: ManifestInclude}, {ReceiptID: "b", State: ManifestInclude}})
		assertCode(t, err, ManifestErrCycle)
	})
	t.Run("ASSERT_MANIFEST_DUPLICATE_REJECTION", func(t *testing.T) {
		receipts := append(baseReceipts(), ManifestReceipt{ID: "other", Path: "child/file.go", Digest: digestB})
		decisions := []ManifestDecision{{ReceiptID: "root", State: ManifestInclude}, {ReceiptID: "child", State: ManifestInclude}, {ReceiptID: "excluded", State: ManifestExclude, Reason: "policy"}, {ReceiptID: "other", State: ManifestExclude, Reason: "duplicate path"}}
		_, err := AssembleManifest(receipts, decisions)
		assertCode(t, err, ManifestErrDuplicate)
	})
	t.Run("ASSERT_MANIFEST_CLOSED_VALIDATION", func(t *testing.T) {
		receipts := baseReceipts()
		receipts[1].Path = "/absolute/file.go"
		_, err := AssembleManifest(receipts, baseDecisions())
		assertCode(t, err, ManifestErrInvalid)
	})
	t.Run("ASSERT_MANIFEST_CANONICAL_ORDER_REORDERED_INPUT", func(t *testing.T) {
		receipts := baseReceipts()
		decisions := baseDecisions()
		first, err := AssembleManifest(receipts, decisions)
		if err != nil {
			t.Fatal(err)
		}
		reverseReceipts(receipts)
		reverseDecisions(decisions)
		second, err := AssembleManifest(receipts, decisions)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("canonical bytes changed after reordered inputs:\n%s\n%s", first, second)
		}
		if len(first) == 0 || first[len(first)-1] != '\n' {
			t.Fatalf("canonical bytes are not newline terminated: %q", first)
		}
	})
}

func TestManifestPerturbationIsAssertionSpecific(t *testing.T) {
	perturb := os.Getenv(manifestPerturbEnv)
	if perturb == "" {
		t.Skip("perturbation control")
	}
	results := map[string]bool{}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"ASSERT_MANIFEST_EXACT_ACCOUNTING_OMISSION", func() error {
			_, err := AssembleManifest(baseReceipts(), []ManifestDecision{{ReceiptID: "root", State: ManifestInclude}, {ReceiptID: "child", State: ManifestInclude}})
			return err
		}},
		{"ASSERT_MANIFEST_EXACT_ACCOUNTING_OVERLAP", func() error {
			_, err := AssembleManifest(baseReceipts(), append(baseDecisions(), ManifestDecision{ReceiptID: "excluded", State: ManifestInclude}))
			return err
		}},
		{"ASSERT_MANIFEST_PARENT_CLOSURE", func() error {
			_, err := AssembleManifest(baseReceipts(), []ManifestDecision{{ReceiptID: "root", State: ManifestExclude, Reason: "x"}, {ReceiptID: "child", State: ManifestInclude}, {ReceiptID: "excluded", State: ManifestExclude, Reason: "x"}})
			return err
		}},
		{"ASSERT_MANIFEST_CYCLE_REJECTION", func() error {
			_, err := AssembleManifest([]ManifestReceipt{{ID: "a", Path: "a", ParentID: "b", Digest: digestA}, {ID: "b", Path: "b", ParentID: "a", Digest: digestB}}, []ManifestDecision{{ReceiptID: "a", State: ManifestInclude}, {ReceiptID: "b", State: ManifestInclude}})
			return err
		}},
		{"ASSERT_MANIFEST_DUPLICATE_REJECTION", func() error {
			_, err := AssembleManifest([]ManifestReceipt{{ID: "a", Path: "same", Digest: digestA}, {ID: "b", Path: "same", Digest: digestB}}, []ManifestDecision{{ReceiptID: "a", State: ManifestInclude}, {ReceiptID: "b", State: ManifestExclude, Reason: "x"}})
			return err
		}},
		{"ASSERT_MANIFEST_CLOSED_VALIDATION", func() error {
			receipts := baseReceipts()
			receipts[1].Path = "/absolute/file.go"
			_, err := AssembleManifest(receipts, baseDecisions())
			return err
		}},
	} {
		results[tc.name] = tc.run() != nil
	}
	if perturb == "ASSERT_MANIFEST_CANONICAL_ORDER_REORDERED_INPUT" {
		r := baseReceipts()
		d := baseDecisions()
		first, _ := AssembleManifest(r, d)
		reverseReceipts(r)
		reverseDecisions(d)
		second, _ := AssembleManifest(r, d)
		results[perturb] = bytes.Equal(first, second)
	}
	for name, passed := range results {
		want := name != perturb
		if passed != want {
			t.Fatalf("ASSERTION_RESULT %s got_pass=%t want_pass=%t perturb=%s", name, passed, want, perturb)
		}
		t.Logf("ASSERTION_RESULT %s PASS", name)
	}
}

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func baseReceipts() []ManifestReceipt {
	return []ManifestReceipt{{ID: "root", Path: "root", Digest: digestA}, {ID: "child", Path: "child/file.go", ParentID: "root", Digest: digestB}, {ID: "excluded", Path: "notes.txt", Digest: digestC}}
}
func baseDecisions() []ManifestDecision {
	return []ManifestDecision{{ReceiptID: "root", State: ManifestInclude}, {ReceiptID: "child", State: ManifestInclude}, {ReceiptID: "excluded", State: ManifestExclude, Reason: "policy"}}
}
func reverseReceipts(v []ManifestReceipt) {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
}
func reverseDecisions(v []ManifestDecision) {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
}
func assertCode(t *testing.T, err error, code ManifestErrorCode) {
	t.Helper()
	var target *ManifestError
	if !errors.As(err, &target) || target.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}
