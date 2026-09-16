package liveprojection

import (
	"context"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type DocumentPreparer interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
}

type PreparationStatus string

const (
	PreparationComplete      PreparationStatus = "COMPLETE"
	PreparationResourceLimit PreparationStatus = "RESOURCE_LIMIT"
	PreparationFailed        PreparationStatus = "FAILED"
)

type PreparationLimits struct {
	MaxDocuments     int `json:"max_documents"`
	MaxBytes         int `json:"max_bytes"`
	MaxDocumentBytes int `json:"max_document_bytes"`
	MaxMessages      int `json:"max_messages"`
	MaxWork          int `json:"max_work"`
}

type LimitAccounting struct {
	Observed int `json:"observed"`
	Limit    int `json:"limit"`
}

type PreparationAccounting struct {
	Attempted int             `json:"attempted"`
	Succeeded int             `json:"succeeded"`
	Documents LimitAccounting `json:"documents"`
	Bytes     LimitAccounting `json:"bytes"`
	Work      LimitAccounting `json:"work"`
}

type TerminalFailure struct {
	URI     string          `json:"uri"`
	Failure session.Failure `json:"failure"`
}

type PreparationResult struct {
	Status     PreparationStatus                `json:"status"`
	Failure    *TerminalFailure                 `json:"failure,omitempty"`
	Accounting PreparationAccounting            `json:"accounting"`
	Supplies   []*sessionruntime.DocumentSupply `json:"-"`
}

func Prepare(ctx context.Context, preparer DocumentPreparer, sessionID string, generation uint64, languageID string, plan []string, limits PreparationLimits) PreparationResult {
	uris := uniqueURIs(plan)
	result := PreparationResult{Status: PreparationComplete}
	result.Accounting.Documents = LimitAccounting{Observed: len(uris), Limit: limits.MaxDocuments}
	result.Accounting.Bytes = LimitAccounting{Limit: limits.MaxBytes}
	result.Accounting.Work = LimitAccounting{Observed: len(uris), Limit: limits.MaxWork}
	if len(uris) > limits.MaxDocuments || len(uris) > limits.MaxMessages || len(uris) > limits.MaxWork {
		result.Status = PreparationResourceLimit
		return result
	}

	supplies := make([]*sessionruntime.DocumentSupply, 0, len(uris))
	for _, uri := range uris {
		result.Accounting.Attempted++
		prepared := preparer.PrepareDocument(ctx, sessionruntime.DocumentRequest{
			SessionID: sessionID, Generation: generation, URI: uri,
			LanguageID: languageID, CaptureSupply: true,
		})
		if prepared.Failure != "" {
			result.Status = PreparationFailed
			result.Failure = &TerminalFailure{URI: uri, Failure: prepared.Failure}
			return result
		}
		if prepared.Supply == nil {
			result.Status = PreparationFailed
			result.Failure = &TerminalFailure{URI: uri, Failure: sessionruntime.DocumentSupplyUnavailable}
			return result
		}
		result.Accounting.Succeeded++
		if len(prepared.Supply.Content) > limits.MaxDocumentBytes {
			result.Status = PreparationResourceLimit
			return result
		}
		result.Accounting.Bytes.Observed += len(prepared.Supply.Content)
		if result.Accounting.Bytes.Observed > limits.MaxBytes {
			result.Status = PreparationResourceLimit
			return result
		}
		supplies = append(supplies, cloneSupply(prepared.Supply))
	}
	result.Supplies = supplies
	return result
}

func uniqueURIs(plan []string) []string {
	uris := make([]string, 0, len(plan))
	seen := make(map[string]struct{}, len(plan))
	for _, uri := range plan {
		if uri == "" {
			continue
		}
		if _, ok := seen[uri]; ok {
			continue
		}
		seen[uri] = struct{}{}
		uris = append(uris, uri)
	}
	return uris
}

func cloneSupply(supply *sessionruntime.DocumentSupply) *sessionruntime.DocumentSupply {
	cloned := *supply
	cloned.Content = append([]byte(nil), supply.Content...)
	cloned.Params = append([]byte(nil), supply.Params...)
	return &cloned
}
