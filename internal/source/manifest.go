package source

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

type ManifestDecisionState string

const (
	ManifestInclude    ManifestDecisionState = "INCLUDE"
	ManifestExclude    ManifestDecisionState = "EXCLUDE"
	manifestPerturbEnv                       = "LSP_TRACE_MANIFEST_PERTURB"
)

type ManifestReceipt struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	ParentID string `json:"parent_id,omitempty"`
	Digest   string `json:"digest"`
}

type ManifestDecision struct {
	ReceiptID string                `json:"receipt_id"`
	State     ManifestDecisionState `json:"state"`
	Reason    string                `json:"reason,omitempty"`
}

type ManifestErrorCode string

const (
	ManifestErrInvalid       ManifestErrorCode = "INVALID"
	ManifestErrAccounting    ManifestErrorCode = "ACCOUNTING"
	ManifestErrDuplicate     ManifestErrorCode = "DUPLICATE"
	ManifestErrParentClosure ManifestErrorCode = "PARENT_CLOSURE"
	ManifestErrCycle         ManifestErrorCode = "CYCLE"
)

type ManifestError struct {
	Code   ManifestErrorCode
	Detail string
}

func (e *ManifestError) Error() string { return string(e.Code) + ": " + e.Detail }

type SourceManifest struct {
	SchemaVersion string             `json:"schema_version"`
	Included      []ManifestReceipt  `json:"included"`
	Excluded      []ManifestDecision `json:"excluded"`
}

var manifestDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func manifestError(code ManifestErrorCode, format string, args ...any) error {
	return &ManifestError{Code: code, Detail: fmt.Sprintf(format, args...)}
}

func AssembleManifest(receipts []ManifestReceipt, decisions []ManifestDecision) ([]byte, error) {
	perturb := os.Getenv(manifestPerturbEnv)
	byID := make(map[string]ManifestReceipt, len(receipts))
	paths := make(map[string]string, len(receipts))
	for _, receipt := range receipts {
		if (receipt.ID == "" || receipt.Path == "" || path.IsAbs(receipt.Path) || path.Clean(receipt.Path) != receipt.Path || strings.HasPrefix(receipt.Path, "../") || !manifestDigest.MatchString(receipt.Digest)) && perturb != "ASSERT_MANIFEST_CLOSED_VALIDATION" {
			return nil, manifestError(ManifestErrInvalid, "malformed receipt %q", receipt.ID)
		}
		if _, exists := byID[receipt.ID]; exists && perturb != "ASSERT_MANIFEST_DUPLICATE_REJECTION" {
			return nil, manifestError(ManifestErrDuplicate, "duplicate receipt id %q", receipt.ID)
		}
		if prior, exists := paths[receipt.Path]; exists && perturb != "ASSERT_MANIFEST_DUPLICATE_REJECTION" {
			return nil, manifestError(ManifestErrDuplicate, "duplicate receipt path %q (%s, %s)", receipt.Path, prior, receipt.ID)
		}
		byID[receipt.ID] = receipt
		paths[receipt.Path] = receipt.ID
	}

	decisionByID := make(map[string]ManifestDecision, len(decisions))
	for _, decision := range decisions {
		if decision.ReceiptID == "" || (decision.State != ManifestInclude && decision.State != ManifestExclude) || (decision.State == ManifestExclude && decision.Reason == "") || (decision.State == ManifestInclude && decision.Reason != "") {
			return nil, manifestError(ManifestErrInvalid, "malformed decision for %q", decision.ReceiptID)
		}
		if _, exists := byID[decision.ReceiptID]; !exists {
			return nil, manifestError(ManifestErrAccounting, "decision references unknown receipt %q", decision.ReceiptID)
		}
		if _, exists := decisionByID[decision.ReceiptID]; exists && perturb != "ASSERT_MANIFEST_EXACT_ACCOUNTING_OVERLAP" {
			return nil, manifestError(ManifestErrDuplicate, "duplicate or overlapping decision for %q", decision.ReceiptID)
		}
		decisionByID[decision.ReceiptID] = decision
	}
	if len(decisionByID) != len(byID) && perturb != "ASSERT_MANIFEST_EXACT_ACCOUNTING_OMISSION" {
		return nil, manifestError(ManifestErrAccounting, "decisions account for %d of %d receipts", len(decisionByID), len(byID))
	}

	included := make([]ManifestReceipt, 0, len(receipts))
	excluded := make([]ManifestDecision, 0, len(receipts))
	for id, receipt := range byID {
		decision, exists := decisionByID[id]
		if !exists {
			continue
		}
		if decision.State == ManifestInclude {
			included = append(included, receipt)
		} else {
			excluded = append(excluded, decision)
		}
	}
	includedIDs := make(map[string]bool, len(included))
	for _, receipt := range included {
		includedIDs[receipt.ID] = true
	}
	if perturb != "ASSERT_MANIFEST_PARENT_CLOSURE" {
		for _, receipt := range included {
			if receipt.ParentID != "" && !includedIDs[receipt.ParentID] {
				return nil, manifestError(ManifestErrParentClosure, "included receipt %q lacks included parent %q", receipt.ID, receipt.ParentID)
			}
		}
	}
	if perturb != "ASSERT_MANIFEST_CYCLE_REJECTION" {
		state := make(map[string]uint8, len(included))
		var visit func(string) error
		visit = func(id string) error {
			if state[id] == 1 {
				return manifestError(ManifestErrCycle, "parent cycle at %q", id)
			}
			if state[id] == 2 {
				return nil
			}
			state[id] = 1
			if parent := byID[id].ParentID; parent != "" && includedIDs[parent] {
				if err := visit(parent); err != nil {
					return err
				}
			}
			state[id] = 2
			return nil
		}
		for id := range includedIDs {
			if err := visit(id); err != nil {
				return nil, err
			}
		}
	}
	if perturb != "ASSERT_MANIFEST_CANONICAL_ORDER_REORDERED_INPUT" {
		sort.Slice(included, func(i, j int) bool { return included[i].Path < included[j].Path })
		sort.Slice(excluded, func(i, j int) bool {
			return byID[excluded[i].ReceiptID].Path < byID[excluded[j].ReceiptID].Path
		})
	}
	manifest := SourceManifest{SchemaVersion: "lsp-trace.source-custody-manifest.v1", Included: included, Excluded: excluded}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, manifestError(ManifestErrInvalid, "encode manifest: %v", err)
	}
	return append(encoded, '\n'), nil
}
