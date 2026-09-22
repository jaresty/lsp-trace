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
	Planned   int             `json:"-"`
	Documents LimitAccounting `json:"documents"`
	Bytes     LimitAccounting `json:"bytes"`
	Work      LimitAccounting `json:"work"`
}

type DocumentPreparationFailure string

const (
	DocumentPreparationContextCancelled      DocumentPreparationFailure = "CONTEXT_CANCELLED"
	DocumentPreparationContextDeadline       DocumentPreparationFailure = "CONTEXT_DEADLINE"
	DocumentPreparationStaleGeneration       DocumentPreparationFailure = "STALE_GENERATION"
	DocumentPreparationLifecycleConflict     DocumentPreparationFailure = "LIFECYCLE_CONFLICT"
	DocumentPreparationResourceExhausted     DocumentPreparationFailure = "RESOURCE_EXHAUSTED"
	DocumentPreparationSessionPoisoned       DocumentPreparationFailure = "SESSION_POISONED"
	DocumentPreparationSupplyUnavailable     DocumentPreparationFailure = "DOCUMENT_SUPPLY_UNAVAILABLE"
	DocumentPreparationLanguageIDUnavailable DocumentPreparationFailure = "LANGUAGE_ID_UNAVAILABLE"
	DocumentPreparationURIUnavailable        DocumentPreparationFailure = "DOCUMENT_URI_UNAVAILABLE"
	DocumentPreparationOutsideWorkspace      DocumentPreparationFailure = "DOCUMENT_OUTSIDE_WORKSPACE"
	DocumentPreparationSourceUnavailable     DocumentPreparationFailure = "DOCUMENT_SOURCE_UNAVAILABLE"
	DocumentPreparationSupplyMissing         DocumentPreparationFailure = "SUPPLY_MISSING"
	DocumentPreparationUnknown               DocumentPreparationFailure = "UNKNOWN_DOCUMENT_PREPARATION_FAILURE"
)

type TerminalFailure struct {
	URI            string                     `json:"-"`
	Failure        session.Failure            `json:"-"`
	Dimension      DocumentPreparationFailure `json:"-"`
	FailingOrdinal int                        `json:"-"`
}

func normalizeDocumentPreparationFailure(failure session.Failure) DocumentPreparationFailure {
	switch failure {
	case session.RequestCancelled:
		return DocumentPreparationContextCancelled
	case session.RequestTimeout:
		return DocumentPreparationContextDeadline
	case session.StaleGeneration:
		return DocumentPreparationStaleGeneration
	case session.LifecycleConflict:
		return DocumentPreparationLifecycleConflict
	case session.ResourceExhausted:
		return DocumentPreparationResourceExhausted
	case session.SessionPoisoned:
		return DocumentPreparationSessionPoisoned
	case sessionruntime.DocumentSupplyUnavailable:
		return DocumentPreparationSupplyUnavailable
	case sessionruntime.DocumentURIUnavailable:
		return DocumentPreparationURIUnavailable
	case sessionruntime.DocumentOutsideWorkspace:
		return DocumentPreparationOutsideWorkspace
	case sessionruntime.DocumentSourceUnavailable:
		return DocumentPreparationSourceUnavailable
	case sessionruntime.LanguageIDUnavailable:
		return DocumentPreparationLanguageIDUnavailable
	default:
		return DocumentPreparationUnknown
	}
}

type PreparationResult struct {
	Status                PreparationStatus                `json:"status"`
	Failure               *TerminalFailure                 `json:"failure,omitempty"`
	Accounting            PreparationAccounting            `json:"accounting"`
	Supplies              []*sessionruntime.DocumentSupply `json:"-"`
	ResourceLimitField    string                           `json:"-"`
	ResourceLimitObserved int                              `json:"-"`
	ResourceLimitValue    int                              `json:"-"`
}

func Prepare(ctx context.Context, preparer DocumentPreparer, sessionID string, generation uint64, languageID string, plan []string, limits PreparationLimits) PreparationResult {
	uris := uniqueURIs(plan)
	result := PreparationResult{Status: PreparationComplete}
	result.Accounting.Planned = len(uris)
	result.Accounting.Documents = LimitAccounting{Observed: len(uris), Limit: limits.MaxDocuments}
	result.Accounting.Bytes = LimitAccounting{Limit: limits.MaxBytes}
	result.Accounting.Work = LimitAccounting{Observed: len(uris), Limit: limits.MaxWork}
	for _, bound := range []struct {
		field string
		limit int
	}{
		{field: "documents", limit: limits.MaxDocuments},
		{field: "messages", limit: limits.MaxMessages},
		{field: "work", limit: limits.MaxWork},
	} {
		if len(uris) > bound.limit {
			result.Status = PreparationResourceLimit
			result.ResourceLimitField = bound.field
			result.ResourceLimitObserved = len(uris)
			result.ResourceLimitValue = bound.limit
			return result
		}
	}

	supplies := make([]*sessionruntime.DocumentSupply, 0, len(uris))
	for ordinal, uri := range uris {
		result.Accounting.Attempted++
		prepared := preparer.PrepareDocument(ctx, sessionruntime.DocumentRequest{
			SessionID: sessionID, Generation: generation, URI: uri,
			LanguageID: languageID, CaptureSupply: true,
		})
		if prepared.Failure != "" {
			result.Status = PreparationFailed
			result.Failure = &TerminalFailure{URI: uri, Failure: prepared.Failure, Dimension: normalizeDocumentPreparationFailure(prepared.Failure), FailingOrdinal: ordinal}
			return result
		}
		if prepared.Supply == nil {
			result.Status = PreparationFailed
			result.Failure = &TerminalFailure{URI: uri, Failure: sessionruntime.DocumentSupplyUnavailable, Dimension: DocumentPreparationSupplyMissing, FailingOrdinal: ordinal}
			return result
		}
		result.Accounting.Succeeded++
		if len(prepared.Supply.Content) > limits.MaxDocumentBytes {
			result.Status = PreparationResourceLimit
			result.ResourceLimitField = "document_bytes"
			result.ResourceLimitObserved = len(prepared.Supply.Content)
			result.ResourceLimitValue = limits.MaxDocumentBytes
			return result
		}
		result.Accounting.Bytes.Observed += len(prepared.Supply.Content)
		if result.Accounting.Bytes.Observed > limits.MaxBytes {
			result.Status = PreparationResourceLimit
			result.ResourceLimitField = "bytes"
			result.ResourceLimitObserved = result.Accounting.Bytes.Observed
			result.ResourceLimitValue = limits.MaxBytes
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
