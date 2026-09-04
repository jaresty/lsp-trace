package execution

import (
	"errors"
	"os"

	"lsp-trace/internal/source"
)

const manifestBridgePerturbEnv = "LSP_TRACE_MANIFEST_BRIDGE_PERTURB"

// AssembleDecisionManifest joins canonical manifest receipts to explicit caller
// decisions and delegates all closed-manifest semantics to source.AssembleManifest.
func AssembleDecisionManifest(receipts []source.ManifestReceipt, decisions []source.ManifestDecision) ([]byte, error) {
	perturb := os.Getenv(manifestBridgePerturbEnv)
	if perturb == "ASSERT_BRIDGE_EXACT_ACCOUNTING" {
		seen := make(map[string]bool, len(decisions))
		for _, decision := range decisions {
			seen[decision.ReceiptID] = true
		}
		for _, receipt := range receipts {
			if !seen[receipt.ID] {
				decisions = append(decisions, source.ManifestDecision{ReceiptID: receipt.ID, State: source.ManifestInclude})
			}
		}
	}

	manifest, err := source.AssembleManifest(receipts, decisions)
	if err != nil {
		var manifestErr *source.ManifestError
		if perturb == "ASSERT_BRIDGE_CLOSED_ASSEMBLY_ERROR" && errors.As(err, &manifestErr) && manifestErr.Code == source.ManifestErrParentClosure {
			return []byte("{}\n"), nil
		}
		return nil, err
	}
	if perturb == "ASSERT_BRIDGE_CANONICAL_DELEGATION_BYTES" {
		return append(manifest, ' '), nil
	}
	return manifest, nil
}
