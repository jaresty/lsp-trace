package execution

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"lsp-trace/internal/source"
)

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func bridgeReceipts() []source.ManifestReceipt {
	return []source.ManifestReceipt{
		{ID: "root", Path: "root", Digest: digestA},
		{ID: "child", Path: "root/child.go", ParentID: "root", Digest: digestB},
	}
}

func bridgeDecisions() []source.ManifestDecision {
	return []source.ManifestDecision{
		{ReceiptID: "root", State: source.ManifestInclude},
		{ReceiptID: "child", State: source.ManifestInclude},
	}
}

func TestAssembleDecisionManifestGuards(t *testing.T) {
	t.Run("ASSERT_BRIDGE_EXACT_ACCOUNTING", func(t *testing.T) {
		cases := []struct {
			name      string
			receipts  []source.ManifestReceipt
			decisions []source.ManifestDecision
		}{
			{name: "omitted", receipts: bridgeReceipts(), decisions: bridgeDecisions()[:1]},
			{name: "duplicate", receipts: bridgeReceipts(), decisions: append(bridgeDecisions(), source.ManifestDecision{ReceiptID: "child", State: source.ManifestExclude, Reason: "duplicate"})},
			{name: "unknown", receipts: bridgeReceipts(), decisions: append(bridgeDecisions(), source.ManifestDecision{ReceiptID: "unknown", State: source.ManifestExclude, Reason: "unknown"})},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := AssembleDecisionManifest(tc.receipts, tc.decisions); err == nil {
					t.Fatalf("ASSERT_BRIDGE_EXACT_ACCOUNTING result=PASS want=FAIL case=%s", tc.name)
				}
			})
		}
	})

	t.Run("ASSERT_BRIDGE_CANONICAL_DELEGATION_BYTES", func(t *testing.T) {
		want, err := source.AssembleManifest(bridgeReceipts(), bridgeDecisions())
		if err != nil {
			t.Fatal(err)
		}
		got, err := AssembleDecisionManifest(bridgeReceipts(), bridgeDecisions())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("ASSERT_BRIDGE_CANONICAL_DELEGATION_BYTES result=FAIL got=%q want=%q", got, want)
		}
	})

	t.Run("ASSERT_BRIDGE_CLOSED_ASSEMBLY_ERROR", func(t *testing.T) {
		receipts := bridgeReceipts()
		receipts[1].ParentID = "missing"
		_, err := AssembleDecisionManifest(receipts, bridgeDecisions())
		var manifestErr *source.ManifestError
		if !errors.As(err, &manifestErr) || manifestErr.Code != source.ManifestErrParentClosure {
			t.Fatalf("ASSERT_BRIDGE_CLOSED_ASSEMBLY_ERROR result=FAIL err=%v", err)
		}
	})
}

func TestAssembleDecisionManifestPerturbationWitness(t *testing.T) {
	perturb := os.Getenv(manifestBridgePerturbEnv)
	if perturb == "" {
		t.Skip("set " + manifestBridgePerturbEnv + " to exercise one present-but-wrong state")
	}
	results := map[string]bool{}

	_, omissionErr := AssembleDecisionManifest(bridgeReceipts(), bridgeDecisions()[:1])
	results["ASSERT_BRIDGE_EXACT_ACCOUNTING"] = omissionErr != nil

	want, directErr := source.AssembleManifest(bridgeReceipts(), bridgeDecisions())
	got, bridgeErr := AssembleDecisionManifest(bridgeReceipts(), bridgeDecisions())
	results["ASSERT_BRIDGE_CANONICAL_DELEGATION_BYTES"] = directErr == nil && bridgeErr == nil && bytes.Equal(got, want)

	receipts := bridgeReceipts()
	receipts[1].ParentID = "missing"
	_, closedErr := AssembleDecisionManifest(receipts, bridgeDecisions())
	var manifestErr *source.ManifestError
	results["ASSERT_BRIDGE_CLOSED_ASSEMBLY_ERROR"] = errors.As(closedErr, &manifestErr) && manifestErr.Code == source.ManifestErrParentClosure

	for name, passed := range results {
		wantPass := name != perturb
		if passed != wantPass {
			t.Fatalf("%s result=%s want=%s perturb=%s", name, passFail(passed), passFail(wantPass), perturb)
		}
		t.Logf("%s result=%s perturb=%s", name, passFail(passed), perturb)
	}
}

func passFail(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}
