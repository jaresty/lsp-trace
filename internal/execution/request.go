// Package execution defines transport- and stage-independent execution request data.
package execution

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
)

type ValidationCode string

const (
	ValidationInvalidIdentity        ValidationCode = "INVALID_IDENTITY"
	ValidationManifestInvalid        ValidationCode = "MANIFEST_INVALID"
	ValidationManifestDuplicate      ValidationCode = "MANIFEST_DUPLICATE"
	ValidationManifestAccounting     ValidationCode = "MANIFEST_ACCOUNTING"
	ValidationPublicationDestination ValidationCode = "PUBLICATION_DESTINATION_INVALID"
)

type ValidationError struct {
	Code   ValidationCode
	Detail string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Detail) }

type PublicationDestination struct {
	Root     *publication.Root
	Selector string
}

type RequestSpec struct {
	RequestID   string
	Receipts    []source.ManifestReceipt
	Decisions   []source.ManifestDecision
	Trust       *schema.TrustAdmissionRequest
	Publication PublicationDestination
}

type Request struct {
	requestID   string
	receipts    []source.ManifestReceipt
	decisions   []source.ManifestDecision
	trust       *schema.TrustAdmissionRequest
	publication PublicationDestination
}

var receiptDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validationError(code ValidationCode, format string, args ...any) error {
	return &ValidationError{Code: code, Detail: fmt.Sprintf(format, args...)}
}

func NewRequest(spec RequestSpec) (*Request, error) {
	if spec.RequestID == "" || strings.TrimSpace(spec.RequestID) != spec.RequestID || strings.IndexByte(spec.RequestID, 0) >= 0 {
		return nil, validationError(ValidationInvalidIdentity, "request id is not canonical")
	}
	if err := validateManifestInputs(spec.Receipts, spec.Decisions); err != nil {
		return nil, err
	}
	if err := validatePublicationDestination(spec.Publication); err != nil {
		return nil, err
	}
	return &Request{
		requestID:   spec.RequestID,
		receipts:    append([]source.ManifestReceipt(nil), spec.Receipts...),
		decisions:   append([]source.ManifestDecision(nil), spec.Decisions...),
		trust:       copyTrust(spec.Trust),
		publication: spec.Publication,
	}, nil
}

func validateManifestInputs(receipts []source.ManifestReceipt, decisions []source.ManifestDecision) error {
	byID := make(map[string]struct{}, len(receipts))
	byPath := make(map[string]struct{}, len(receipts))
	for _, receipt := range receipts {
		if receipt.ID == "" || receipt.Path == "" || path.IsAbs(receipt.Path) || path.Clean(receipt.Path) != receipt.Path || strings.HasPrefix(receipt.Path, "../") || !receiptDigest.MatchString(receipt.Digest) {
			return validationError(ValidationManifestInvalid, "malformed receipt %q", receipt.ID)
		}
		if _, exists := byID[receipt.ID]; exists {
			return validationError(ValidationManifestDuplicate, "duplicate receipt id %q", receipt.ID)
		}
		if _, exists := byPath[receipt.Path]; exists {
			return validationError(ValidationManifestDuplicate, "duplicate receipt path %q", receipt.Path)
		}
		byID[receipt.ID] = struct{}{}
		byPath[receipt.Path] = struct{}{}
	}

	accounted := make(map[string]struct{}, len(decisions))
	for _, decision := range decisions {
		if decision.ReceiptID == "" || (decision.State != source.ManifestInclude && decision.State != source.ManifestExclude) || (decision.State == source.ManifestExclude && decision.Reason == "") || (decision.State == source.ManifestInclude && decision.Reason != "") {
			return validationError(ValidationManifestInvalid, "malformed decision for %q", decision.ReceiptID)
		}
		if _, exists := byID[decision.ReceiptID]; !exists {
			return validationError(ValidationManifestAccounting, "decision references unknown receipt %q", decision.ReceiptID)
		}
		if _, exists := accounted[decision.ReceiptID]; exists {
			return validationError(ValidationManifestDuplicate, "duplicate decision for %q", decision.ReceiptID)
		}
		accounted[decision.ReceiptID] = struct{}{}
	}
	if len(accounted) != len(byID) {
		return validationError(ValidationManifestAccounting, "decisions account for %d of %d receipts", len(accounted), len(byID))
	}
	return nil
}

func validatePublicationDestination(destination PublicationDestination) error {
	selector := destination.Selector
	if destination.Root == nil || selector == "" || selector == "." || strings.IndexByte(selector, 0) >= 0 || strings.Contains(selector, `\`) || filepath.IsAbs(selector) || !filepath.IsLocal(selector) || path.Clean(selector) != selector {
		return validationError(ValidationPublicationDestination, "publication destination is not canonical")
	}
	return nil
}

func copyTrust(input *schema.TrustAdmissionRequest) *schema.TrustAdmissionRequest {
	if input == nil {
		return nil
	}
	copied := *input
	copied.Receipt = append([]byte(nil), input.Receipt...)
	copied.Context.ProvisionedReceiptIDs = make(map[string]struct{}, len(input.Context.ProvisionedReceiptIDs))
	for id := range input.Context.ProvisionedReceiptIDs {
		copied.Context.ProvisionedReceiptIDs[id] = struct{}{}
	}
	if input.GitAttestation != nil {
		attestation := *input.GitAttestation
		copied.GitAttestation = &attestation
	}
	return &copied
}

func (r *Request) RequestID() string { return r.requestID }

func (r *Request) ManifestInputs() ([]source.ManifestReceipt, []source.ManifestDecision) {
	return append([]source.ManifestReceipt(nil), r.receipts...), append([]source.ManifestDecision(nil), r.decisions...)
}

func (r *Request) Trust() *schema.TrustAdmissionRequest { return copyTrust(r.trust) }

func (r *Request) Publication() PublicationDestination { return r.publication }
